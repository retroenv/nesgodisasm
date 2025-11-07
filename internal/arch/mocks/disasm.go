// Package mocks provides mock implementations of arch interfaces for testing.
package mocks

import (
	"github.com/retroenv/retrodisasm/internal/arch"
	"github.com/retroenv/retrodisasm/internal/options"
	"github.com/retroenv/retrodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/log"
)

// Disasm is a minimal mock implementation of arch.Disasm for testing.
type Disasm struct {
	logger   *log.Logger
	mapper   arch.Mapper
	codeBase uint16
	Memory   []byte
}

// NewDisasm creates a new mock Disasm with the given logger and code base address.
func NewDisasm(logger *log.Logger, mapper arch.Mapper, codeBase uint16, memorySize int) *Disasm {
	return &Disasm{
		logger:   logger,
		mapper:   mapper,
		codeBase: codeBase,
		Memory:   make([]byte, memorySize),
	}
}

func (m *Disasm) AddAddressToParse(address, context, from uint16, currentInstruction arch.Instruction, isABranchDestination bool) {
}

func (m *Disasm) Cart() *cartridge.Cartridge {
	return nil
}

func (m *Disasm) ChangeAddressRangeToCodeAsData(address uint16, data []byte) {
}

func (m *Disasm) CodeBaseAddress() uint16 {
	return m.codeBase
}

func (m *Disasm) Constants() arch.ConstantManager {
	return nil
}

func (m *Disasm) DeleteFunctionReturnToParse(uint16) {
}

func (m *Disasm) JumpEngine() arch.JumpEngine {
	return nil
}

func (m *Disasm) Logger() *log.Logger {
	return m.logger
}

func (m *Disasm) Mapper() arch.Mapper {
	return m.mapper
}

func (m *Disasm) Options() options.Disassembler {
	return options.Disassembler{}
}

func (m *Disasm) ProgramCounter() uint16 {
	return 0
}

func (m *Disasm) ReadMemory(address uint16) (byte, error) {
	if int(address) >= len(m.Memory) {
		return 0, nil
	}
	return m.Memory[address], nil
}

func (m *Disasm) ReadMemoryWord(address uint16) (uint16, error) {
	low, err := m.ReadMemory(address)
	if err != nil {
		return 0, err
	}
	high, err := m.ReadMemory(address + 1)
	if err != nil {
		return 0, err
	}
	return uint16(low) | uint16(high)<<8, nil
}

func (m *Disasm) SetCodeBaseAddress(uint16) {
}

func (m *Disasm) SetHandlers(program.Handlers) {
}

func (m *Disasm) SetVectorsStartAddress(uint16) {
}

func (m *Disasm) Variables() arch.VariableManager {
	return nil
}
