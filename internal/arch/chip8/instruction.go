package chip8

import (
	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/chip8"
)

var _ arch.Instruction = &Instruction{}

// Instruction represents a CHIP-8 instruction.
type Instruction struct {
	ins *chip8.Instruction
}

// IsCall returns true if the instruction is a call.
func (i Instruction) IsCall() bool {
	return i.ins == chip8.Call
}

// IsNil returns true if the instruction is nil.
func (i Instruction) IsNil() bool {
	return i.ins == nil
}

// Name returns the instruction name.
func (i Instruction) Name() string {
	if i.ins == nil {
		return ""
	}
	return i.ins.Name
}

// Unofficial returns true if the instruction is not official.
func (i Instruction) Unofficial() bool {
	return false // CHIP-8 instructions are all "official"
}
