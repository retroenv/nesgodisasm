// Package chip8 provides CHIP-8 instruction wrapper implementation.
package chip8

import (
	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/chip8"
)

// Compile-time check to ensure Instruction implements arch.Instruction.
var _ arch.Instruction = (*Instruction)(nil)

// Instruction represents a CHIP-8 instruction wrapper that implements arch.Instruction.
// It provides a bridge between the retrogolib CHIP-8 instruction definitions
// and the disassembler's architecture interface.
type Instruction struct {
	ins *chip8.Instruction
}

// IsCall returns true if the instruction is a call instruction.
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
// CHIP-8 has a well-defined instruction set with no unofficial opcodes.
func (i Instruction) Unofficial() bool {
	return false
}
