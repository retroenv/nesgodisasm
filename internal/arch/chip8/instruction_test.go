package chip8

import (
	"testing"

	"github.com/retroenv/retrogolib/arch/cpu/chip8"
	"github.com/retroenv/retrogolib/assert"
)

func TestInstruction_IsCall(t *testing.T) {
	tests := []struct {
		name     string
		ins      *chip8.Instruction
		expected bool
	}{
		{"call instruction", chip8.Call, true},
		{"jump instruction", chip8.Jp, false},
		{"load instruction", chip8.Ld, false},
		{"nil instruction", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := Instruction{ins: tt.ins}
			result := instr.IsCall()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInstruction_IsNil(t *testing.T) {
	tests := []struct {
		name     string
		ins      *chip8.Instruction
		expected bool
	}{
		{"nil instruction", nil, true},
		{"valid instruction", chip8.Jp, false},
		{"call instruction", chip8.Call, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := Instruction{ins: tt.ins}
			result := instr.IsNil()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInstruction_Name(t *testing.T) {
	tests := []struct {
		name     string
		ins      *chip8.Instruction
		expected string
	}{
		{"nil instruction", nil, ""},
		{"jump instruction", chip8.Jp, chip8.Jp.Name},
		{"call instruction", chip8.Call, chip8.Call.Name},
		{"load instruction", chip8.Ld, chip8.Ld.Name},
		{"clear instruction", chip8.Cls, chip8.Cls.Name},
		{"return instruction", chip8.Ret, chip8.Ret.Name},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := Instruction{ins: tt.ins}
			result := instr.Name()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInstruction_Unofficial(t *testing.T) {
	tests := []struct {
		name string
		ins  *chip8.Instruction
	}{
		{"nil instruction", nil},
		{"jump instruction", chip8.Jp},
		{"call instruction", chip8.Call},
		{"load instruction", chip8.Ld},
		{"clear instruction", chip8.Cls},
		{"return instruction", chip8.Ret},
		{"draw instruction", chip8.Drw},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := Instruction{ins: tt.ins}
			// CHIP-8 has no unofficial instructions
			result := instr.Unofficial()
			assert.False(t, result)
		})
	}
}

// Test instruction wrapper creation
func TestInstructionWrapper(t *testing.T) {
	// Test with various CHIP-8 instructions
	instructions := []*chip8.Instruction{
		chip8.Cls,
		chip8.Ret,
		chip8.Jp,
		chip8.Call,
		chip8.Se,
		chip8.Sne,
		chip8.Ld,
		chip8.Add,
		chip8.Or,
		chip8.And,
		chip8.Xor,
		chip8.Sub,
		chip8.Subn,
		chip8.Shr,
		chip8.Shl,
		chip8.Rnd,
		chip8.Drw,
		chip8.Skp,
		chip8.Sknp,
	}

	for _, ins := range instructions {
		t.Run(ins.Name, func(t *testing.T) {
			instr := Instruction{ins: ins}

			// Test basic properties
			assert.False(t, instr.IsNil())
			assert.Equal(t, ins.Name, instr.Name())
			assert.False(t, instr.Unofficial())

			// Test call detection (only CALL should return true)
			expectedIsCall := ins == chip8.Call
			assert.Equal(t, expectedIsCall, instr.IsCall())
		})
	}
}
