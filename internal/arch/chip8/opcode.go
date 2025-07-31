package chip8

import (
	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/chip8"
)

const opcodeSize = 2

var _ arch.Opcode = &Opcode{}

type Opcode struct {
	op chip8.Opcode
}

func (o Opcode) Addressing() int {
	return int(o.op.Info.Mask)
}

func (o Opcode) Instruction() arch.Instruction {
	return Instruction{ins: o.op.Instruction}
}

func (o Opcode) ReadsMemory() bool {
	if o.op.Instruction == nil {
		return false
	}
	// CHIP-8 instructions that read from memory based on instruction name
	switch o.op.Instruction.Name {
	case chip8.Ld.Name: // Load instructions may read from memory depending on addressing
		return true
	case chip8.Drw.Name: // Draw instruction reads sprite data from memory
		return true
	default:
		return false
	}
}

func (o Opcode) WritesMemory() bool {
	if o.op.Instruction == nil {
		return false
	}
	// CHIP-8 instructions that write to memory based on instruction name
	switch o.op.Instruction.Name {
	case chip8.Ld.Name: // Load instructions may write to memory depending on addressing
		return true
	case chip8.Drw.Name: // Draw instruction writes to display memory
		return true
	default:
		return false
	}
}

func (o Opcode) ReadWritesMemory() bool {
	// CHIP-8 instructions that both read and write memory
	// Most CHIP-8 instructions either read OR write, but not both in the same operation
	return false
}
