package chip8

import (
	"github.com/retroenv/retrodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/chip8"
)

// opcodeSize is the size of CHIP-8 instructions in bytes.
const opcodeSize = 2

// Compile-time check to ensure Opcode implements arch.Opcode.
var _ arch.Opcode = (*Opcode)(nil)

// Opcode represents a CHIP-8 instruction opcode with addressing and behavior information.
type Opcode struct {
	op chip8.Opcode
}

// Addressing returns the addressing mode mask for this CHIP-8 opcode.
func (o Opcode) Addressing() int {
	return int(o.op.Info.Mask)
}

// Instruction returns the instruction associated with this opcode.
func (o Opcode) Instruction() arch.Instruction {
	return Instruction{ins: o.op.Instruction}
}

// ReadsMemory returns true if this CHIP-8 instruction reads from memory.
func (o Opcode) ReadsMemory() bool {
	if o.op.Instruction == nil {
		return false
	}
	return chip8.MemoryReadInstructions.Contains(o.op.Instruction.Name)
}

// WritesMemory returns true if this CHIP-8 instruction writes to memory.
func (o Opcode) WritesMemory() bool {
	if o.op.Instruction == nil {
		return false
	}
	return chip8.MemoryWriteInstructions.Contains(o.op.Instruction.Name)
}

// ReadWritesMemory returns true if this CHIP-8 instruction both reads and writes memory.
// Most CHIP-8 instructions either read OR write, but not both in the same operation.
func (o Opcode) ReadWritesMemory() bool {
	return false
}
