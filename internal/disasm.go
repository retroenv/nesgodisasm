// Package disasm provides an NES program disassembler.
package disasm

import (
	"fmt"
	"hash/crc32"
	"io"
	"strings"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/assembler"
	"github.com/retroenv/nesgodisasm/internal/consts"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/nesgodisasm/internal/writer"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/nes/codedatalog"
	"github.com/retroenv/retrogolib/log"
)

type FileWriterConstructor func(app *program.Program, options options.Disassembler,
	mainWriter io.Writer, newBankWriter assembler.NewBankWriter) writer.AssemblerWriter

var _ arch.Disasm = &Disasm{}

// Disasm implements a disassembler.
type Disasm struct {
	arch    arch.Architecture
	logger  *log.Logger
	options options.Disassembler

	pc uint16 // program counter

	cart                  *cartridge.Cartridge
	fileWriterConstructor FileWriterConstructor
	handlers              program.Handlers

	codeBaseAddress     uint16 // codebase address of the cartridge, it is not always 0x8000
	vectorsStartAddress uint16

	constants     *consts.Consts
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
func New(ar arch.Architecture, logger *log.Logger, cart *cartridge.Cartridge,
	options options.Disassembler, fileWriterConstructor FileWriterConstructor) (*Disasm, error) {

	dis := &Disasm{
		arch:                        ar,
		logger:                      logger,
		options:                     options,
		cart:                        cart,
		fileWriterConstructor:       fileWriterConstructor,
		variables:                   map[uint16]*variable{},
		usedVariables:               map[uint16]struct{}{},
		jumpEngineCallersAdded:      map[uint16]*jumpEngineCaller{},
		jumpEngines:                 map[uint16]struct{}{},
		branchDestinations:          map[uint16]struct{}{},
		offsetsToParseAdded:         map[uint16]struct{}{},
		offsetsParsed:               map[uint16]struct{}{},
		functionReturnsToParseAdded: map[uint16]struct{}{},
	}

	var err error
	dis.constants, err = consts.New(ar)
	if err != nil {
		return nil, fmt.Errorf("creating constants: %w", err)
	}

	dis.initializeBanks(cart.PRG)
	dis.mapper, err = newMapper(dis.banks, len(cart.PRG))
	if err != nil {
		return nil, fmt.Errorf("creating mapper: %w", err)
	}
	if err := dis.arch.Initialize(dis); err != nil {
		return nil, fmt.Errorf("initializing architecture: %w", err)
	}

	if options.CodeDataLog != nil {
		if err = dis.loadCodeDataLog(); err != nil {
			return nil, err
		}
	}

	return dis, nil
}

// Process disassembles the cartridge.
func (dis *Disasm) Process(mainWriter io.Writer, newBankWriter assembler.NewBankWriter) (*program.Program, error) {
	if err := dis.followExecutionFlow(); err != nil {
		return nil, err
	}

	dis.processData()
	if err := dis.processVariables(); err != nil {
		return nil, err
	}
	dis.constants.ProcessConstants()
	dis.processJumpDestinations()

	app, err := dis.convertToProgram()
	if err != nil {
		return nil, err
	}
	fileWriter := dis.fileWriterConstructor(app, dis.options, mainWriter, newBankWriter)
	if err = fileWriter.Write(); err != nil {
		return nil, fmt.Errorf("writing app to file: %w", err)
	}
	return app, nil
}

// Cart returns the loaded cartridge.
func (dis *Disasm) Cart() *cartridge.Cartridge {
	return dis.cart
}

func (dis *Disasm) ProgramCounter() uint16 {
	return dis.pc
}

func (dis *Disasm) Logger() *log.Logger {
	return dis.logger
}

func (dis *Disasm) OffsetInfo(address uint16) arch.Offset {
	return dis.mapper.offsetInfo(address)
}

func (dis *Disasm) SetHandlers(handlers program.Handlers) {
	dis.handlers = handlers
}

func (dis *Disasm) SetCodeBaseAddress(address uint16) {
	dis.codeBaseAddress = address

	dis.logger.Debug("Code base address",
		log.String("address", fmt.Sprintf("0x%04X", dis.codeBaseAddress)))
}

func (dis *Disasm) SetVectorsStartAddress(address uint16) {
	dis.vectorsStartAddress = address
}

func (dis *Disasm) Options() options.Disassembler {
	return dis.options
}

// Constants returns the constants manager.
func (dis *Disasm) Constants() arch.ConstantManager {
	return dis.constants
}

// converts the internal disassembly representation to a program type that will be used by
// the chosen assembler output instance to generate the asm file.
func (dis *Disasm) convertToProgram() (*program.Program, error) {
	app := program.New(dis.cart)
	app.CodeBaseAddress = dis.codeBaseAddress
	app.VectorsStartAddress = dis.vectorsStartAddress
	app.Handlers = dis.handlers

	for bnkIndex, bnk := range dis.banks {
		prgBank := program.NewPRGBank(len(bnk.offsets))

		for i := range len(bnk.offsets) {
			offsetInfo := bnk.offsets[i]
			programOffsetInfo, err := dis.getProgramOffset(dis.codeBaseAddress+uint16(i), offsetInfo)
			if err != nil {
				return nil, err
			}

			prgBank.Offsets[i] = programOffsetInfo
		}

		dis.constants.SetBankConstants(bnkIndex, prgBank)

		for address := range bnk.usedVariables {
			varInfo := bnk.variables[address]
			prgBank.Variables[varInfo.name] = address
		}

		setBankName(prgBank, bnkIndex, len(dis.banks))
		setBankVectors(bnk, prgBank)

		app.PRG = append(app.PRG, prgBank)
	}

	dis.constants.SetProgramConstants(app)

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
			dis.AddAddressToParse(dis.codeBaseAddress+uint16(index), 0, 0, nil, false)
		}
		if flags&codedatalog.SubEntryPoint != 0 {
			bank0.offsets[index].SetType(program.CallDestination)
		}
	}

	return nil
}

func (dis *Disasm) getProgramOffset(address uint16, offsetInfo offset) (program.Offset, error) {
	programOffset := offsetInfo.Offset
	programOffset.Address = address

	if offsetInfo.branchingTo != "" {
		programOffset.Code = fmt.Sprintf("%s %s", offsetInfo.Code(), offsetInfo.branchingTo)
	}

	if offsetInfo.IsType(program.CodeOffset | program.CodeAsData | program.FunctionReference) {
		if len(programOffset.Data) == 0 && programOffset.Label == "" {
			return programOffset, nil
		}

		if offsetInfo.IsType(program.FunctionReference) {
			programOffset.Code = ".word " + offsetInfo.branchingTo
		}

		if err := dis.setComment(address, &programOffset); err != nil {
			return program.Offset{}, err
		}
	} else {
		programOffset.SetType(program.DataOffset)
	}

	return programOffset, nil
}

func (dis *Disasm) setComment(address uint16, programOffset *program.Offset) error {
	var comments []string

	if dis.options.OffsetComments {
		programOffset.HasAddressComment = true
		comments = []string{fmt.Sprintf("$%04X", address)}
	}

	if dis.options.HexComments {
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

	for _, b := range offset.Data {
		if _, err := fmt.Fprintf(buf, "%02X ", b); err != nil {
			return "", fmt.Errorf("writing hex comment: %w", err)
		}
	}

	comment := strings.TrimRight(buf.String(), " ")
	return comment, nil
}
