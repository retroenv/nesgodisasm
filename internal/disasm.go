// Package disasm provides an NES program disassembler.
package disasm

import (
	"fmt"
	"hash/crc32"
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

	branchFrom  []uint16 // list of all addresses that branch to this offset
	branchingTo string   // label to jump to if instruction branches
	context     uint16   // function or interrupt context that the offset is part of
}

// Disasm implements a NES disassembler.
type Disasm struct {
	options *disasmoptions.Options

	pc         uint16 // program counter
	converter  parameter.Converter
	fileWriter fileWriter
	cart       *cartridge.Cartridge
	handlers   program.Handlers

	codeBaseAddress uint16 // codebase address of the cartridge, as it can be different from 0x8000
	constants       map[uint16]constTranslation
	usedConstants   map[uint16]constTranslation
	variables       map[uint16]*variable
	usedVariables   map[uint16]struct{}

	jumpEngines            map[uint16]struct{} // set of all jump engine functions addresses
	jumpEngineCallers      []*jumpEngineCaller // jump engine caller tables to process
	jumpEngineCallersAdded map[uint16]*jumpEngineCaller
	branchDestinations     map[uint16]struct{} // set of all addresses that are branched to
	offsets                []offset

	offsetsToParse              []uint16
	offsetsToParseAdded         map[uint16]struct{}
	functionReturnsToParse      []uint16
	functionReturnsToParseAdded map[uint16]struct{}
}

// New creates a new NES disassembler that creates output compatible with the chosen assembler.
func New(cart *cartridge.Cartridge, options *disasmoptions.Options) (*Disasm, error) {
	dis := &Disasm{
		options:                     options,
		cart:                        cart,
		codeBaseAddress:             uint16(0x10000 - len(cart.PRG)),
		variables:                   map[uint16]*variable{},
		usedVariables:               map[uint16]struct{}{},
		usedConstants:               map[uint16]constTranslation{},
		offsets:                     make([]offset, len(cart.PRG)),
		jumpEngineCallersAdded:      map[uint16]*jumpEngineCaller{},
		jumpEngines:                 map[uint16]struct{}{},
		branchDestinations:          map[uint16]struct{}{},
		offsetsToParseAdded:         map[uint16]struct{}{},
		functionReturnsToParseAdded: map[uint16]struct{}{},
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
	dis.processJumpDestinations()

	app, err := dis.convertToProgram()
	if err != nil {
		return err
	}
	if err := dis.fileWriter.Write(dis.options, app, writer); err != nil {
		return fmt.Errorf("writing app to file: %w", err)
	}
	return nil
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
		dis.addAddressToParse(nmi, nmi, 0, nil, false)
		offset := dis.addressToOffset(nmi)
		dis.offsets[offset].Label = "NMI"
		dis.offsets[offset].SetType(program.CallDestination)
		dis.handlers.NMI = "NMI"
	}

	reset := dis.readMemoryWord(0xFFFC)
	dis.addAddressToParse(reset, reset, 0, nil, false)
	offset := dis.addressToOffset(reset)
	dis.offsets[offset].Label = "Reset"
	dis.offsets[offset].SetType(program.CallDestination)

	irq := dis.readMemoryWord(0xFFFE)
	if irq != 0 {
		dis.addAddressToParse(irq, irq, 0, nil, false)
		offset = dis.addressToOffset(irq)
		dis.offsets[offset].Label = "IRQ"
		dis.offsets[offset].SetType(program.CallDestination)
		dis.handlers.IRQ = "IRQ"
	}
}

// converts the internal disassembly representation to a program type that will be used by
// the chosen assembler output instance to generate the asm file.
func (dis *Disasm) convertToProgram() (*program.Program, error) {
	app := program.New(dis.cart)
	app.Handlers = dis.handlers

	for i := 0; i < len(dis.offsets); i++ {
		res := dis.offsets[i]
		offset, err := getProgramOffset(res, dis.codeBaseAddress+uint16(i), dis.options)
		if err != nil {
			return nil, err
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

	crc32q := crc32.MakeTable(crc32.IEEE)
	app.Checksums.PRG = crc32.Checksum(dis.cart.PRG, crc32q)
	app.Checksums.CHR = crc32.Checksum(dis.cart.CHR, crc32q)
	app.Checksums.Overall = crc32.Checksum(append(dis.cart.PRG, dis.cart.CHR...), crc32q)

	return app, nil
}

func (dis *Disasm) addressToOffset(address uint16) uint16 {
	offset := address - CodeBaseAddress
	offset %= uint16(len(dis.cart.PRG))
	return offset
}

func (dis *Disasm) loadCodeDataLog() error {
	prgFlags, err := codedatalog.LoadFile(dis.cart, dis.options.CodeDataLog)
	if err != nil {
		return fmt.Errorf("loading code/data log file: %w", err)
	}

	for offset, flags := range prgFlags {
		if flags&codedatalog.Code != 0 {
			dis.addAddressToParse(dis.codeBaseAddress+uint16(offset), 0, 0, nil, false)
		}
		if flags&codedatalog.SubEntryPoint != 0 {
			dis.offsets[offset].SetType(program.CallDestination)
		}
	}

	return nil
}

func getProgramOffset(res offset, address uint16, options *disasmoptions.Options) (program.Offset, error) {
	offset := res.Offset
	if res.branchingTo != "" {
		offset.Code = fmt.Sprintf("%s %s", res.Code, res.branchingTo)
	}

	if res.IsType(program.CodeOffset | program.CodeAsData | program.FunctionReference) {
		if len(offset.OpcodeBytes) == 0 && offset.Label == "" {
			return offset, nil
		}

		if res.IsType(program.FunctionReference) {
			offset.Code = fmt.Sprintf(".word %s", res.branchingTo)
		}

		if options.OffsetComments {
			setOffsetComment(&offset, address)
		}
		if options.HexComments {
			if err := setHexCodeComment(&offset); err != nil {
				return program.Offset{}, err
			}
		}
	} else {
		offset.SetType(program.DataOffset)
	}

	return offset, nil
}

func setHexCodeComment(offset *program.Offset) error {
	buf := &strings.Builder{}

	for _, b := range offset.OpcodeBytes {
		if _, err := fmt.Fprintf(buf, "%02X ", b); err != nil {
			return fmt.Errorf("writing hex comment: %w", err)
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
