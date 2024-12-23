package arch

import (
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/log"
)

// Disasm represents a disassembler.
type Disasm interface {
	JumpEngine

	// AddAddressToParse adds an address to the list to be processed if the address has not been processed yet.
	AddAddressToParse(address, context, from uint16, currentInstruction Instruction, isABranchDestination bool)
	// AddVariableReference adds a variable reference if the opcode is accessing
	// the given address directly by reading or writing.
	AddVariableReference(addressReference, usageAddress uint16, opcode Opcode, forceVariableUsage bool)
	// Cart returns the loaded cartridge.
	Cart() *cartridge.Cartridge
	// ChangeAddressRangeToCodeAsData sets a range of code address to code as
	// data types. It combines all data bytes that are not split by a label.
	ChangeAddressRangeToCodeAsData(address uint16, data []byte)
	// Constants returns the constants manager.
	Constants() ConstantManager
	// Logger returns the logger.
	Logger() *log.Logger
	// OffsetInfo returns the offset information for the given address.
	OffsetInfo(address uint16) Offset
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

type ConstantManager interface {
	// ReplaceParameter replaces the parameter of an instruction by a constant name
	// if the address of the instruction is found in the constants map.
	ReplaceParameter(address uint16, opcode Opcode, paramAsString string) (string, bool)
}

// JumpEngine contains jump engine related helper.
type JumpEngine interface {
	// AddJumpEngine adds a jump engine function address to the list of jump engines.
	AddJumpEngine(address uint16)
	// GetContextDataReferences parse all instructions of the function context until the jump
	// and returns data references that could point to the function table.
	GetContextDataReferences(offsets []Offset, addresses []uint16) ([]uint16, error)
	// GetFunctionTableReference detects a jump engine function context and its function table.
	GetFunctionTableReference(context uint16, dataReferences []uint16)
	// HandleJumpEngineDestination processes a newly detected jump engine destination.
	HandleJumpEngineDestination(caller, destination uint16) error
	// HandleJumpEngineCallers processes all callers of a newly detected jump engine function.
	HandleJumpEngineCallers(context uint16) error
	// JumpContextInfo builds the list of instructions of the current function context.
	JumpContextInfo(jumpAddress uint16, offsetInfo Offset) ([]Offset, []uint16)
}
