// Package disasm implements a multi retro system disassembler
package disasm

import (
	"fmt"
	"hash/crc32"
	"io"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/assembler"
	"github.com/retroenv/nesgodisasm/internal/consts"
	"github.com/retroenv/nesgodisasm/internal/jumpengine"
	"github.com/retroenv/nesgodisasm/internal/mapper"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/nesgodisasm/internal/vars"
	"github.com/retroenv/nesgodisasm/internal/writer"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/codedatalog"
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

	constants  arch.ConstantManager
	jumpEngine arch.JumpEngine
	vars       arch.VariableManager

	branchDestinations map[uint16]struct{} // set of all addresses that are branched to

	// TODO handle bank switch
	offsetsToParse      []uint16
	offsetsToParseAdded map[uint16]struct{}
	offsetsParsed       map[uint16]struct{}

	functionReturnsToParse      []uint16
	functionReturnsToParseAdded map[uint16]struct{}

	mapper *mapper.Mapper
}

// New creates a new disassembler that uses the passed architecture to implement system
// specific disassembly logic.
func New(ar arch.Architecture, logger *log.Logger, cart *cartridge.Cartridge,
	options options.Disassembler, fileWriterConstructor FileWriterConstructor) (*Disasm, error) {

	dis := &Disasm{
		arch:                        ar,
		logger:                      logger,
		options:                     options,
		cart:                        cart,
		vars:                        vars.New(ar),
		fileWriterConstructor:       fileWriterConstructor,
		branchDestinations:          map[uint16]struct{}{},
		offsetsToParseAdded:         map[uint16]struct{}{},
		offsetsParsed:               map[uint16]struct{}{},
		functionReturnsToParseAdded: map[uint16]struct{}{},
		jumpEngine:                  jumpengine.New(ar),
	}

	var err error
	dis.constants, err = consts.New(ar)
	if err != nil {
		return nil, fmt.Errorf("creating constants: %w", err)
	}

	dis.mapper, err = mapper.New(ar, dis, cart)
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

	dis.mapper.ProcessData()
	if err := dis.vars.Process(dis); err != nil {
		return nil, fmt.Errorf("processing variables: %w", err)
	}
	dis.constants.Process()
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

func (dis *Disasm) SetHandlers(handlers program.Handlers) {
	dis.handlers = handlers
}

func (dis *Disasm) CodeBaseAddress() uint16 {
	return dis.codeBaseAddress
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

// Variables returns the variable manager.
func (dis *Disasm) Variables() arch.VariableManager {
	return dis.vars
}

// JumpEngine returns the jump engine.
func (dis *Disasm) JumpEngine() arch.JumpEngine {
	return dis.jumpEngine
}

// Mapper returns the mapper.
func (dis *Disasm) Mapper() arch.Mapper {
	return dis.mapper
}

// ReadMemory delegates to the architecture-specific implementation.
func (dis *Disasm) ReadMemory(address uint16) (byte, error) {
	value, err := dis.arch.ReadMemory(dis, address)
	if err != nil {
		return 0, fmt.Errorf("reading memory at address %04x: %w", address, err)
	}
	return value, nil
}

// ReadMemoryWord reads a word from memory using the architecture-specific ReadMemory method.
func (dis *Disasm) ReadMemoryWord(address uint16) (uint16, error) {
	b, err := dis.ReadMemory(address)
	if err != nil {
		return 0, err
	}
	low := uint16(b)

	b, err = dis.ReadMemory(address + 1)
	if err != nil {
		return 0, err
	}

	high := uint16(b)
	return (high << 8) | low, nil
}

// converts the internal disassembly representation to a program type that will be used by
// the chosen assembler output instance to generate the asm file.
func (dis *Disasm) convertToProgram() (*program.Program, error) {
	app := program.New(dis.cart)
	app.CodeBaseAddress = dis.codeBaseAddress
	app.VectorsStartAddress = dis.vectorsStartAddress
	app.Handlers = dis.handlers

	if err := dis.mapper.SetProgramBanks(dis, app); err != nil {
		return nil, fmt.Errorf("setting program banks: %w", err)
	}

	dis.constants.SetToProgram(app)
	dis.vars.SetToProgram(app)

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

	dis.mapper.ApplyCodeDataLog(dis, prgFlags)
	return nil
}
