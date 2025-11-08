// Package m6502 provides a 6502 architecture specific disassembler code.
package m6502

import (
	"errors"
	"fmt"

	"github.com/retroenv/retrodisasm/internal/consts"
	"github.com/retroenv/retrodisasm/internal/instruction"
	"github.com/retroenv/retrodisasm/internal/jumpengine"
	"github.com/retroenv/retrodisasm/internal/offset"
	"github.com/retroenv/retrodisasm/internal/options"
	"github.com/retroenv/retrodisasm/internal/program"
	"github.com/retroenv/retrodisasm/internal/vars"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/parameter"
	"github.com/retroenv/retrogolib/log"
)

// Dependencies contains the dependencies needed by Arch6502.
type Dependencies struct {
	Disasm     disasm
	Mapper     offset.Mapper
	JumpEngine *jumpengine.JumpEngine
	Vars       *vars.Vars
	Consts     *consts.Consts
}

// disasm defines the minimal interface needed from the disassembler.
type disasm interface {
	// AddAddressToParse adds an address to the list to be processed.
	AddAddressToParse(address, context, from uint16, currentInstruction instruction.Instruction, isABranchDestination bool)
	// Cart returns the loaded cartridge.
	Cart() *cartridge.Cartridge
	// ChangeAddressRangeToCodeAsData sets a range of code address to code as data types.
	ChangeAddressRangeToCodeAsData(address uint16, data []byte)
	// Options returns the disassembler options.
	Options() options.Disassembler
	// ProgramCounter returns the current program counter of the execution tracer.
	ProgramCounter() uint16
	// ReadMemory reads a byte from the memory at the given address.
	ReadMemory(address uint16) (byte, error)
	// ReadMemoryWord reads a word from the memory at the given address.
	ReadMemoryWord(address uint16) (uint16, error)
	// SetCodeBaseAddress sets the code base address.
	SetCodeBaseAddress(address uint16)
	// SetHandlers sets the program vector handlers.
	SetHandlers(handlers program.Handlers)
	// SetVectorsStartAddress sets the start address of the vectors.
	SetVectorsStartAddress(address uint16)
}

// New returns a new 6502 architecture configuration.
func New(logger *log.Logger, converter parameter.Converter) *Arch6502 {
	return &Arch6502{
		converter: converter,
		logger:    logger,
	}
}

type Arch6502 struct {
	converter       parameter.Converter
	dis             disasm
	jumpEngine      *jumpengine.JumpEngine
	logger          *log.Logger
	mapper          offset.Mapper
	vars            *vars.Vars
	consts          *consts.Consts
	codeBaseAddress uint16
}

// InjectDependencies sets the required dependencies for this architecture.
func (ar *Arch6502) InjectDependencies(deps Dependencies) {
	ar.dis = deps.Disasm
	ar.mapper = deps.Mapper
	ar.jumpEngine = deps.JumpEngine
	ar.vars = deps.Vars
	ar.consts = deps.Consts
}

// SetCodeBaseAddress sets the code base address for this architecture.
func (ar *Arch6502) SetCodeBaseAddress(address uint16) {
	ar.codeBaseAddress = address
}

// LastCodeAddress returns the last possible address of code.
// This is used in systems where the last address is reserved for
// the interrupt vector table.
func (ar *Arch6502) LastCodeAddress() uint16 {
	return m6502.InterruptVectorStartAddress
}

func (ar *Arch6502) ProcessOffset(address uint16, offsetInfo *offset.Offset) (bool, error) {
	inspectCode, err := ar.initializeOffsetInfo(offsetInfo)
	if err != nil {
		return false, err
	}
	if !inspectCode {
		return false, nil
	}

	op := offsetInfo.Opcode
	instruction := op.Instruction()
	name := instruction.Name()
	pc := ar.dis.ProgramCounter()

	if op.Addressing() == int(m6502.ImpliedAddressing) {
		offsetInfo.Code = name
	} else {
		params, err := ar.processParamInstruction(pc, offsetInfo)
		if err != nil {
			if errors.Is(err, errInstructionOverlapsIRQHandlers) {
				ar.handleInstructionIRQOverlap(address, offsetInfo)
				return true, nil
			}
			return false, err
		}
		offsetInfo.Code = fmt.Sprintf("%s %s", name, params)
	}

	if _, ok := m6502.NotExecutingFollowingOpcodeInstructions[name]; ok {
		if err := ar.checkForJumpEngineJmp(pc, offsetInfo); err != nil {
			return false, err
		}
	} else {
		opcodeLength := uint16(len(offsetInfo.Data))
		followingOpcodeAddress := pc + opcodeLength
		ar.dis.AddAddressToParse(followingOpcodeAddress, offsetInfo.Context, address, instruction, false)
		if err := ar.checkForJumpEngineCall(pc, offsetInfo); err != nil {
			return false, err
		}
	}

	return true, nil
}

// BankWindowSize returns the bank window size.
func (ar *Arch6502) BankWindowSize(_ *cartridge.Cartridge) int {
	return 0x2000 // TODO calculate dynamically
}
