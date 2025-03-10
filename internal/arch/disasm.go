package arch

import (
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/log"
)

// Disasm represents a disassembler.
type Disasm interface {
	// AddAddressToParse adds an address to the list to be processed if the address has not been processed yet.
	AddAddressToParse(address, context, from uint16, currentInstruction Instruction, isABranchDestination bool)
	// Cart returns the loaded cartridge.
	Cart() *cartridge.Cartridge
	// ChangeAddressRangeToCodeAsData sets a range of code address to code as
	// data types. It combines all data bytes that are not split by a label.
	ChangeAddressRangeToCodeAsData(address uint16, data []byte)
	// CodeBaseAddress returns the code base address.
	CodeBaseAddress() uint16
	// Constants returns the constants manager.
	Constants() ConstantManager
	// DeleteFunctionReturnToParse deletes a function return address from the list of addresses to parse.
	DeleteFunctionReturnToParse(address uint16)
	// JumpEngine returns the jump engine.
	JumpEngine() JumpEngine
	// Logger returns the logger.
	Logger() *log.Logger
	// Mapper returns the mapper.
	Mapper() Mapper
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
	// Variables returns the variable manager.
	Variables() VariableManager
}
