// Package disasm provides an NES program disassembler.
package disasm

import (
	"fmt"
	"hash/crc32"
	"io"
	"strings"

	"github.com/retroenv/nesgodisasm/internal/assembler"
	"github.com/retroenv/nesgodisasm/internal/assembler/asm6"
	"github.com/retroenv/nesgodisasm/internal/assembler/ca65"
	"github.com/retroenv/nesgodisasm/internal/assembler/nesasm"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/nesgodisasm/internal/writer"
	"github.com/retroenv/retrogolib/arch/nes"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/nes/codedatalog"
	"github.com/retroenv/retrogolib/arch/nes/parameter"
	"github.com/retroenv/retrogolib/cpu"
	"github.com/retroenv/retrogolib/log"
)

const irqStartAddress = 0xfffa

type fileWriterConstructor func(app *program.Program, options *options.Disassembler,
	mainWriter io.Writer, newBankWriter assembler.NewBankWriter) writer.AssemblerWriter

// offset defines the content of an offset in a program that can represent data or code.
type offset struct {
	program.Offset

	opcode cpu.Opcode // opcode that the byte at this offset represents

	branchFrom  []bankReference // list of all addresses that branch to this offset
	branchingTo string          // label to jump to if instruction branches
	context     uint16          // function or interrupt context that the offset is part of
}

// Disasm implements a NES disassembler.
type Disasm struct {
	logger  *log.Logger
	options *options.Disassembler

	pc                      uint16 // program counter
	converter               parameter.Converter
	fileWriterConstructor   fileWriterConstructor
	cart                    *cartridge.Cartridge
	handlers                program.Handlers
	noUnofficialInstruction bool // enable for assemblers that do not support unofficial opcodes

	codeBaseAddress uint16 // codebase address of the cartridge, as it can be different from 0x8000

	constants     map[uint16]constTranslation
	usedConstants map[uint16]constTranslation
	variables     map[uint16]*variable
	usedVariables map[uint16]struct{}

	jumpEngines            map[uint16]struct{} // set of all jump engine functions addresses
	jumpEngineCallers      []*jumpEngineCaller // jump engine caller tables to process
	jumpEngineCallersAdded map[uint16]*jumpEngineCaller
	branchDestinations     map[uint16]struct{} // set of all addresses that are branched to

	// TODO handle bank switch
	offsetsToParse      []uint16
	offsetsToParseAdded map[uint16]struct{}
	offsetsParsed       map[uint16]struct{}

	functionReturnsToParse      []uint16
	functionReturnsToParseAdded map[uint16]struct{}

	banks  []*bank
	mapper *mapper
}

// New creates a new NES disassembler that creates output compatible with the chosen assembler.
func New(logger *log.Logger, cart *cartridge.Cartridge, options *options.Disassembler) (*Disasm, error) {
	dis := &Disasm{
		logger:                      logger,
		options:                     options,
		cart:                        cart,
		variables:                   map[uint16]*variable{},
		usedVariables:               map[uint16]struct{}{},
		usedConstants:               map[uint16]constTranslation{},
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

	dis.initializeBanks(cart.PRG)
	dis.mapper, err = newMapper(dis.banks, len(cart.PRG))
	if err != nil {
		return nil, fmt.Errorf("creating mapper: %w", err)
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
func (dis *Disasm) Process(mainWriter io.Writer, newBankWriter assembler.NewBankWriter) error {
	if err := dis.followExecutionFlow(); err != nil {
		return err
	}

	dis.processData()
	if err := dis.processVariables(); err != nil {
		return err
	}
	dis.processConstants()
	dis.processJumpDestinations()

	app, err := dis.convertToProgram()
	if err != nil {
		return err
	}
	fileWriter := dis.fileWriterConstructor(app, dis.options, mainWriter, newBankWriter)
	if err = fileWriter.Write(); err != nil {
		return fmt.Errorf("writing app to file: %w", err)
	}
	return nil
}

func (dis *Disasm) initializeBanks(prg []byte) {
	for i := 0; i < len(prg); {
		size := len(prg) - i
		if size > 0x8000 {
			size = 0x8000
		}

		b := prg[i : i+size]
		bnk := newBank(b)
		dis.banks = append(dis.banks, bnk)
		i += size
	}
}

// initializeCompatibleMode sets the chosen assembler specific instances to be used to output
// compatible code.
func (dis *Disasm) initializeCompatibleMode(assemblerName string) error {
	var paramCfg parameter.Config

	switch strings.ToLower(assemblerName) {
	case assembler.Asm6:
		dis.fileWriterConstructor = asm6.New
		paramCfg = asm6.ParamConfig

	case assembler.Ca65:
		dis.fileWriterConstructor = ca65.New
		paramCfg = ca65.ParamConfig

	case assembler.Nesasm:
		dis.fileWriterConstructor = nesasm.New
		paramCfg = nesasm.ParamConfig
		dis.noUnofficialInstruction = true

	default:
		return fmt.Errorf("unsupported assembler '%s'", assemblerName)
	}

	dis.converter = parameter.New(paramCfg)
	return nil
}

// initializeIrqHandlers reads the 3 IRQ handler addresses and adds them to the addresses to be
// followed for execution flow. Multiple handler can point to the same address.
func (dis *Disasm) initializeIrqHandlers() {
	nmi := dis.readMemoryWord(irqStartAddress)
	if nmi != 0 {
		dis.logger.Debug("NMI handler", log.String("address", fmt.Sprintf("0x%04X", nmi)))
		offsetInfo := dis.mapper.offsetInfo(nmi)
		offsetInfo.Label = "NMI"
		offsetInfo.SetType(program.CallDestination)
		dis.handlers.NMI = "NMI"
	}

	reset := dis.readMemoryWord(irqStartAddress + 2)
	dis.logger.Debug("Reset handler", log.String("address", fmt.Sprintf("0x%04X", reset)))
	offsetInfo := dis.mapper.offsetInfo(reset)
	if offsetInfo != nil {
		if offsetInfo.Label != "" {
			dis.handlers.NMI = "Reset"
		}
		offsetInfo.Label = "Reset"
		offsetInfo.SetType(program.CallDestination)
	}

	irq := dis.readMemoryWord(irqStartAddress + 4)
	if irq != 0 {
		dis.logger.Debug("IRQ handler", log.String("address", fmt.Sprintf("0x%04X", irq)))
		offsetInfo = dis.mapper.offsetInfo(irq)
		if offsetInfo.Label == "" {
			offsetInfo.Label = "IRQ"
			dis.handlers.IRQ = "IRQ"
		} else {
			dis.handlers.IRQ = offsetInfo.Label
		}
		offsetInfo.SetType(program.CallDestination)
	}

	if nmi == reset {
		dis.handlers.NMI = dis.handlers.Reset
	}
	if irq == reset {
		dis.handlers.IRQ = dis.handlers.Reset
	}

	dis.calculateCodeBaseAddress(reset)

	// add IRQ handlers to be parsed after the code base address has been calculated
	dis.addAddressToParse(nmi, nmi, 0, nil, false)
	dis.addAddressToParse(reset, reset, 0, nil, false)
	dis.addAddressToParse(irq, irq, 0, nil, false)
}

// calculateCodeBaseAddress calculates the code base address that is assumed by the code.
// If the code size is only 0x4000 it will be mirror-mapped into the 0x8000 byte of RAM starting at
// 0x8000. The handlers can be set to any of the 2 mirrors as base, based on this the code base
// address is calculated. This ensures that jsr instructions will result in the same opcode, as it
// is based on the code base address.
func (dis *Disasm) calculateCodeBaseAddress(resetHandler uint16) {
	halfPrg := len(dis.cart.PRG) % 0x8000
	dis.codeBaseAddress = uint16(0x8000 + halfPrg)

	// fix up calculated code base address for half sized PRG ROMs that have a different
	// code base address configured in the assembler, like "M.U.S.C.L.E."
	if resetHandler < dis.codeBaseAddress {
		dis.codeBaseAddress = nes.CodeBaseAddress
	}
	dis.logger.Debug("Code base address",
		log.String("address", fmt.Sprintf("0x%04X", dis.codeBaseAddress)))
}

// CodeBaseAddress returns the calculated code base address.
func (dis *Disasm) CodeBaseAddress() uint16 {
	return dis.codeBaseAddress
}

// converts the internal disassembly representation to a program type that will be used by
// the chosen assembler output instance to generate the asm file.
func (dis *Disasm) convertToProgram() (*program.Program, error) {
	app := program.New(dis.cart)
	app.CodeBaseAddress = dis.codeBaseAddress
	app.Handlers = dis.handlers

	for _, bnk := range dis.banks {
		prgBank := program.NewPRGBank(len(bnk.offsets))

		for i := 0; i < len(bnk.offsets); i++ {
			offsetInfo := bnk.offsets[i]
			programOffsetInfo, err := getProgramOffset(dis.codeBaseAddress+uint16(i), offsetInfo, dis.options)
			if err != nil {
				return nil, err
			}

			prgBank.PRG[i] = programOffsetInfo
		}

		for address := range bnk.usedConstants {
			constantInfo := bnk.constants[address]
			if constantInfo.Read != "" {
				prgBank.Constants[constantInfo.Read] = address
			}
			if constantInfo.Write != "" {
				prgBank.Constants[constantInfo.Write] = address
			}
		}
		for address := range bnk.usedVariables {
			varInfo := bnk.variables[address]
			prgBank.Variables[varInfo.name] = address
		}

		app.PRG = append(app.PRG, prgBank)
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

func (dis *Disasm) loadCodeDataLog() error {
	prgFlags, err := codedatalog.LoadFile(dis.cart, dis.options.CodeDataLog)
	if err != nil {
		return fmt.Errorf("loading code/data log file: %w", err)
	}

	// TODO handle banks
	bank0 := dis.banks[0]
	for index, flags := range prgFlags {
		if index > len(bank0.offsets) {
			return nil
		}

		if flags&codedatalog.Code != 0 {
			dis.addAddressToParse(dis.codeBaseAddress+uint16(index), 0, 0, nil, false)
		}
		if flags&codedatalog.SubEntryPoint != 0 {
			bank0.offsets[index].SetType(program.CallDestination)
		}
	}

	return nil
}

func getProgramOffset(address uint16, offsetInfo offset, options *options.Disassembler) (program.Offset, error) {
	programOffset := offsetInfo.Offset
	programOffset.Address = address

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

		if err := setComment(address, &programOffset, options); err != nil {
			return program.Offset{}, err
		}
	} else {
		programOffset.SetType(program.DataOffset)
	}

	return programOffset, nil
}

func setComment(address uint16, programOffset *program.Offset, options *options.Disassembler) error {
	var comments []string

	if options.OffsetComments {
		programOffset.HasAddressComment = true
		comments = []string{fmt.Sprintf("$%04X", address)}
	}

	if options.HexComments {
		hexCodeComment, err := hexCodeComment(programOffset)
		if err != nil {
			return err
		}
		comments = append(comments, hexCodeComment)
	}

	if programOffset.Comment != "" {
		comments = append(comments, programOffset.Comment)
	}
	programOffset.Comment = strings.Join(comments, "  ")
	return nil
}

func hexCodeComment(offset *program.Offset) (string, error) {
	buf := &strings.Builder{}

	for _, b := range offset.OpcodeBytes {
		if _, err := fmt.Fprintf(buf, "%02X ", b); err != nil {
			return "", fmt.Errorf("writing hex comment: %w", err)
		}
	}

	comment := strings.TrimRight(buf.String(), " ")
	return comment, nil
}
