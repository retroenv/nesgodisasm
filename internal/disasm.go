// Package disasm provides an NES program disassembler.
package disasm

import (
	"fmt"
	"io"
	"strings"

	"github.com/retroenv/nesgodisasm/internal/ca65"
	"github.com/retroenv/nesgodisasm/internal/disasmoptions"
	"github.com/retroenv/nesgodisasm/internal/program"
	. "github.com/retroenv/retrogolib/nes/addressing"
	"github.com/retroenv/retrogolib/nes/cartridge"
	"github.com/retroenv/retrogolib/nes/codedatalog"
	"github.com/retroenv/retrogolib/nes/cpu"
	"github.com/retroenv/retrogolib/nes/parameter"
)

type fileWriter interface {
	Write(options *disasmoptions.Options, app *program.Program, writer io.Writer) error
}

// offset defines the content of an offset in a program that can represent data or code.
type offset struct {
	program.Offset

	opcode cpu.Opcode // opcode that the byte at this offset represents

	JumpFrom  []uint16 // list of all addresses that jump to this offset
	JumpingTo string   // label to jump to if instruction branches
}

// Disasm implements a NES disassembler.
type Disasm struct {
	options *disasmoptions.Options

	pc         uint16 // program counter
	converter  parameter.Converter
	fileWriter fileWriter
	cart       *cartridge.Cartridge
	handlers   program.Handlers

	codeBaseAddress uint16
	constants       map[uint16]constTranslation
	usedConstants   map[uint16]constTranslation
	variables       map[uint16]*variable
	usedVariables   map[uint16]struct{}

	jumpTargets map[uint16]struct{} // jumpTargets is a set of all addresses that branched to
	offsets     []offset

	targetsToParse []uint16
	targetsAdded   map[uint16]struct{}
}

// New creates a new NES disassembler that creates output compatible with the chosen assembler.
func New(cart *cartridge.Cartridge, options *disasmoptions.Options) (*Disasm, error) {
	dis := &Disasm{
		options:         options,
		cart:            cart,
		codeBaseAddress: uint16(0x10000 - len(cart.PRG)),
		variables:       map[uint16]*variable{},
		usedVariables:   map[uint16]struct{}{},
		usedConstants:   map[uint16]constTranslation{},
		offsets:         make([]offset, len(cart.PRG)),
		jumpTargets:     map[uint16]struct{}{},
		targetsAdded:    map[uint16]struct{}{},
		handlers: program.Handlers{
			NMI:   "0",
			Reset: "Reset",
			IRQ:   "0",
		},
	}

	var err error
	dis.constants, err = buildConstMap()
	if err != nil {
		return nil, err
	}

	if options.CodeDataLog != nil {
		if err := dis.loadCodeDataLog(); err != nil {
			return nil, err
		}
	}

	if err = dis.initializeCompatibleMode(options.Assembler); err != nil {
		return nil, fmt.Errorf("initializing compatible mode: %w", err)
	}

	dis.initializeIrqHandlers()
	return dis, nil
}

// Process disassembles the cartridge.
func (dis *Disasm) Process(writer io.Writer) error {
	if err := dis.followExecutionFlow(); err != nil {
		return err
	}

	dis.processData()
	if err := dis.processVariables(); err != nil {
		return err
	}
	dis.processJumpTargets()

	app, err := dis.convertToProgram()
	if err != nil {
		return err
	}
	return dis.fileWriter.Write(dis.options, app, writer)
}

// initializeCompatibleMode sets the chosen assembler specific instances to be used to output
// compatible code.
func (dis *Disasm) initializeCompatibleMode(assembler string) error {
	switch strings.ToLower(assembler) {
	case "ca65":
		dis.converter = parameter.Ca65Converter{}
		dis.fileWriter = ca65.FileWriter{}

	default:
		return fmt.Errorf("unsupported assembler '%s'", assembler)
	}
	return nil
}

// initializeIrqHandlers reads the 3 handler addresses and adds them to the addresses to be
// followed for execution flow.
func (dis *Disasm) initializeIrqHandlers() {
	nmi := dis.readMemoryWord(0xFFFA)
	if nmi != 0 {
		dis.addTarget(nmi, nil, false)
		offset := dis.addressToOffset(nmi)
		dis.offsets[offset].Label = "NMI"
		dis.offsets[offset].SetType(program.CallTarget)
		dis.handlers.NMI = "NMI"
	}

	reset := dis.readMemoryWord(0xFFFC)
	dis.addTarget(reset, nil, false)
	offset := dis.addressToOffset(reset)
	dis.offsets[offset].Label = "Reset"
	dis.offsets[offset].SetType(program.CallTarget)

	irq := dis.readMemoryWord(0xFFFE)
	if irq != 0 {
		dis.addTarget(irq, nil, false)
		offset = dis.addressToOffset(irq)
		dis.offsets[offset].Label = "IRQ"
		dis.offsets[offset].SetType(program.CallTarget)
		dis.handlers.IRQ = "IRQ"
	}
}

// converts the internal disasm type representation to a program type that will be used by
// the chosen assembler output instance to generate the asm file.
func (dis *Disasm) convertToProgram() (*program.Program, error) {
	app := program.New(dis.cart)
	app.Handlers = dis.handlers

	for i := 0; i < len(dis.offsets); i++ {
		res := dis.offsets[i]
		offset := res.Offset

		if res.JumpingTo != "" {
			offset.Code = fmt.Sprintf("%s %s", res.Code, res.JumpingTo)
		}

		if res.IsType(program.CodeOffset | program.CodeAsData) {
			if len(offset.OpcodeBytes) == 0 && offset.Label == "" {
				continue
			}
			if dis.options.OffsetComments {
				setOffsetComment(&offset, dis.codeBaseAddress+uint16(i))
			}
			if dis.options.HexComments {
				if err := setHexCodeComment(&offset); err != nil {
					return nil, err
				}
			}
		} else {
			offset.SetType(program.DataOffset)
		}

		app.PRG[i] = offset
	}

	for address := range dis.usedConstants {
		constantInfo := dis.constants[address]
		if constantInfo.Read != "" {
			app.Constants[constantInfo.Read] = address
		}
		if constantInfo.Write != "" {
			app.Constants[constantInfo.Write] = address
		}
	}
	for address := range dis.usedVariables {
		varInfo := dis.variables[address]
		app.Variables[varInfo.name] = address
	}

	return app, nil
}

func (dis *Disasm) addressToOffset(address uint16) uint16 {
	offset := address - CodeBaseAddress
	offset %= uint16(len(dis.cart.PRG))
	return offset
}

func setHexCodeComment(offset *program.Offset) error {
	buf := &strings.Builder{}

	for _, b := range offset.OpcodeBytes {
		if _, err := fmt.Fprintf(buf, "%02X ", b); err != nil {
			return err
		}
	}

	comment := strings.TrimRight(buf.String(), " ")
	if offset.Comment == "" {
		offset.Comment = comment
	} else {
		offset.Comment = fmt.Sprintf("%s %s", offset.Comment, comment)
	}

	return nil
}

func setOffsetComment(offset *program.Offset, address uint16) {
	if offset.Comment == "" {
		offset.Comment = fmt.Sprintf("$%04X", address)
	} else {
		offset.Comment = fmt.Sprintf("$%04X %s", address, offset.Comment)
	}
}

func (dis *Disasm) loadCodeDataLog() error {
	prgFlags, err := codedatalog.LoadFile(dis.cart, dis.options.CodeDataLog)
	if err != nil {
		return fmt.Errorf("loading code/data log file: %w", err)
	}

	for offset, flags := range prgFlags {
		if flags&codedatalog.Code != 0 {
			dis.addTarget(dis.codeBaseAddress+uint16(offset), nil, false)
		}
		if flags&codedatalog.SubEntryPoint != 0 {
			dis.offsets[offset].SetType(program.CallTarget)
		}
	}

	return nil
}
