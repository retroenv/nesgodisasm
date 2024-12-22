package m6502

import (
	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
)

var _ arch.Instruction = &Instruction{}

// Instruction represents a 6502 CPU instruction.
type Instruction struct {
	ins *m6502.Instruction
}

// IsCall returns true if the instruction is a call.
func (i Instruction) IsCall() bool {
	return i.ins.Name == m6502.Jsr.Name
}

// IsNil returns true if the instruction is nil.
func (i Instruction) IsNil() bool {
	return i.ins == nil
}

// Name returns the instruction name.
func (i Instruction) Name() string {
	return i.ins.Name
}

// Unofficial returns true if the instruction is not official.
func (i Instruction) Unofficial() bool {
	return i.ins.Unofficial
}
