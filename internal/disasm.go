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
	"github.com/retroenv/retrogolib/log"
	"github.com/retroenv/retrogolib/nes/addressing"
	"github.com/retroenv/retrogolib/nes/cartridge"
	"github.com/retroenv/retrogolib/nes/codedatalog"
	"github.com/retroenv/retrogolib/nes/cpu"
	"github.com/retroenv/retrogolib/nes/parameter"
)

const irqStartAddress = 0xfffa

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
	logger  *log.Logger
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

	offsetsToParse      []uint16
	offsetsToParseAdded map[uint16]struct{}
	offsetsParsed       map[uint16]struct{}

	functionReturnsToParse      []uint16
	functionReturnsToParseAdded map[uint16]struct{}
}

// New creates a new NES disassembler that creates output compatible with the chosen assembler.
func New(cart *cartridge.Cartridge, options *disasmoptions.Options) (*Disasm, error) {
	dis := &Disasm{
		logger:                      options.Logger,
		options:                     options,
		cart:                        cart,
		variables:                   map[uint16]*variable{},
		usedVariables:               map[uint16]struct{}{},
		usedConstants:               map[uint16]constTranslation{},
		offsets:                     make([]offset, len(cart.PRG)),
		jumpEngineCallersAdded:      map[uint16]*jumpEngineCaller{},
		jumpEngines:                 map[uint16]struct{}{},
		branchDestinations:          map[uint16]struct{}{},
		offsetsToParseAdded:         map[uint16]struct{}{},
		offsetsParsed:               map[uint16]struct{}{},
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

	if err = dis.initializeCompatibleMode(options.Assembler); err != nil {
		return nil, fmt.Errorf("initializing compatible mode: %w", err)
	}

	dis.initializeIrqHandlers()

	if options.CodeDataLog != nil {
		if err = dis.loadCodeDataLog(); err != nil {
			return nil, err
		}
	}

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
	if err = dis.fileWriter.Write(dis.options, app, writer); err != nil {
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

// initializeIrqHandlers reads the 3 IRQ handler addresses and adds them to the addresses to be
// followed for execution flow. Multiple handler can point to the same address.
func (dis *Disasm) initializeIrqHandlers() {
	nmi := dis.readMemoryWord(irqStartAddress)
	if nmi != 0 {
		dis.logger.Debug("NMI handler", log.String("address", fmt.Sprintf("0x%04X", nmi)))
		dis.addAddressToParse(nmi, nmi, 0, nil, false)
		index := dis.addressToIndex(nmi)
		dis.offsets[index].Label = "NMI"
		dis.offsets[index].SetType(program.CallDestination)
		dis.handlers.NMI = "NMI"
	}

	reset := dis.readMemoryWord(irqStartAddress + 2)
	dis.logger.Debug("Reset handler", log.String("address", fmt.Sprintf("0x%04X", reset)))
	dis.addAddressToParse(reset, reset, 0, nil, false)
	index := dis.addressToIndex(reset)
	if dis.offsets[index].Label != "" {
		dis.handlers.NMI = "Reset"
	}
	dis.offsets[index].Label = "Reset"
	dis.offsets[index].SetType(program.CallDestination)

	irq := dis.readMemoryWord(irqStartAddress + 4)
	if irq != 0 {
		dis.logger.Debug("IRQ handler", log.String("address", fmt.Sprintf("0x%04X", irq)))
		dis.addAddressToParse(irq, irq, 0, nil, false)
		index = dis.addressToIndex(irq)
		if dis.offsets[index].Label == "" {
			dis.offsets[index].Label = "IRQ"
			dis.handlers.IRQ = "IRQ"
		} else {
			dis.handlers.IRQ = dis.offsets[index].Label
		}
		dis.offsets[index].SetType(program.CallDestination)
	}

	if nmi == reset {
		dis.handlers.NMI = dis.handlers.Reset
	}
	if irq == reset {
		dis.handlers.IRQ = dis.handlers.Reset
	}

	dis.calculateCodeBaseAddress(reset)
}

// calculateCodeBaseAddress calculates the code base address that is assumed by the code.
// If the code size is only 0x4000 it will be mirror-mapped into the 0x8000 byte of RAM starting at
// 0x8000. The handlers can be set to any of the 2 mirrors as base, based on this the code base
// address is calculated. This ensures that jsr instructions will result in the same opcode, as it
// is based on the code base address.
func (dis *Disasm) calculateCodeBaseAddress(resetHandler uint16) {
	dis.codeBaseAddress = uint16(0x10000 - len(dis.cart.PRG))
	if resetHandler < dis.codeBaseAddress {
		dis.codeBaseAddress = addressing.CodeBaseAddress
	}
}

// CodeBaseAddress returns the calculated code base address.
func (dis *Disasm) CodeBaseAddress() uint16 {
	return dis.codeBaseAddress
}

// converts the internal disassembly representation to a program type that will be used by
// the chosen assembler output instance to generate the asm file.
func (dis *Disasm) convertToProgram() (*program.Program, error) {
	app := program.New(dis.cart)
	app.Handlers = dis.handlers

	for i := 0; i < len(dis.offsets); i++ {
		offsetInfo := dis.offsets[i]
		programOffsetInfo, err := getProgramOffset(offsetInfo, dis.codeBaseAddress+uint16(i), dis.options)
		if err != nil {
			return nil, err
		}

		app.PRG[i] = programOffsetInfo
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

// addressToIndex converts an address if the code base address to an index into the PRG array.
func (dis *Disasm) addressToIndex(address uint16) uint16 {
	index := address % uint16(len(dis.cart.PRG))
	return index
}

func (dis *Disasm) loadCodeDataLog() error {
	prgFlags, err := codedatalog.LoadFile(dis.cart, dis.options.CodeDataLog)
	if err != nil {
		return fmt.Errorf("loading code/data log file: %w", err)
	}

	for index, flags := range prgFlags {
		if flags&codedatalog.Code != 0 {
			dis.addAddressToParse(dis.codeBaseAddress+uint16(index), 0, 0, nil, false)
		}
		if flags&codedatalog.SubEntryPoint != 0 {
			dis.offsets[index].SetType(program.CallDestination)
		}
	}

	return nil
}

func getProgramOffset(offsetInfo offset, address uint16, options *disasmoptions.Options) (program.Offset, error) {
	programOffset := offsetInfo.Offset
	if offsetInfo.branchingTo != "" {
		programOffset.Code = fmt.Sprintf("%s %s", offsetInfo.Code, offsetInfo.branchingTo)
	}

	if offsetInfo.IsType(program.CodeOffset | program.CodeAsData | program.FunctionReference) {
		if len(programOffset.OpcodeBytes) == 0 && programOffset.Label == "" {
			return programOffset, nil
		}

		if offsetInfo.IsType(program.FunctionReference) {
			programOffset.Code = fmt.Sprintf(".word %s", offsetInfo.branchingTo)
		}

		if options.OffsetComments {
			setOffsetComment(&programOffset, address)
		}
		if options.HexComments {
			if err := setHexCodeComment(&programOffset); err != nil {
				return program.Offset{}, err
			}
		}
	} else {
		programOffset.SetType(program.DataOffset)
	}

	return programOffset, nil
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
