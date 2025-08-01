package chip8

import (
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/parameter"
	"github.com/retroenv/retrogolib/assert"
)

func TestNew(t *testing.T) {
	converter := parameter.New(parameter.Config{})
	arch := New(converter)

	assert.NotNil(t, arch)
	assert.Equal(t, converter, arch.converter)
}

func TestChip8_Constants(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	constants, err := arch.Constants()
	assert.NoError(t, err)
	assert.Empty(t, constants)
}

func TestChip8_GetAddressingParam(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	tests := []struct {
		name     string
		param    any
		expected uint16
		valid    bool
	}{
		{"valid uint16", uint16(0x200), 0x200, true},
		{"valid int within range", int(0x300), 0x300, true},
		{"invalid int too large", int(0x1000), 0, false},
		{"invalid negative int", int(-1), 0, false},
		{"invalid string", "invalid", 0, false},
		{"boundary max valid", int(MaxAddress), MaxAddress, true},
		{"boundary invalid", int(MaxAddress + 1), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, valid := arch.GetAddressingParam(tt.param)
			assert.Equal(t, tt.expected, addr)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestChip8_HandleDisambiguousInstructions(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	// CHIP-8 has no disambiguous instructions
	result := arch.HandleDisambiguousInstructions(nil, 0x200, nil)
	assert.False(t, result)
}

func TestChip8_IsAddressingIndexed(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	// CHIP-8 doesn't use indexed addressing
	result := arch.IsAddressingIndexed(nil)
	assert.False(t, result)
}

func TestChip8_LastCodeAddress(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	addr := arch.LastCodeAddress()
	assert.Equal(t, LastCodeAddress, addr)
}

func TestChip8_ProcessVariableUsage(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	// CHIP-8 has simple addressing, no complex variable usage
	err := arch.ProcessVariableUsage(nil, "test")
	assert.NoError(t, err)
}

func TestChip8_ReadOpParam(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	// CHIP-8 opcodes are self-contained, no additional parameters
	param, data, err := arch.ReadOpParam(nil, 0, 0x200)
	assert.NoError(t, err)
	assert.Nil(t, param)
	assert.Nil(t, data)
}

func TestChip8_BankWindowSize(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	// CHIP-8 doesn't use banking
	size := arch.BankWindowSize(nil)
	assert.Equal(t, 0, size)
}

func TestChip8_isJumpInstruction(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	tests := []struct {
		name      string
		instrName string
		expected  bool
	}{
		{"jump instruction", "jp", true},
		{"call instruction", "call", false},
		{"load instruction", "ld", false},
		{"return instruction", "ret", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := arch.isJumpInstruction(tt.instrName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChip8_isCallInstruction(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	tests := []struct {
		name      string
		instrName string
		expected  bool
	}{
		{"call instruction", "call", true},
		{"jump instruction", "jp", false},
		{"load instruction", "ld", false},
		{"return instruction", "ret", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := arch.isCallInstruction(tt.instrName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChip8_isReturnInstruction(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	tests := []struct {
		name      string
		instrName string
		expected  bool
	}{
		{"return instruction", "ret", true},
		{"call instruction", "call", false},
		{"jump instruction", "jp", false},
		{"load instruction", "ld", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := arch.isReturnInstruction(tt.instrName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChip8_isDataReferenceInstruction(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	tests := []struct {
		name      string
		instrName string
		data      []byte
		expected  bool
	}{
		{"LD I instruction", "ld", []byte{0xA2, 0x00}, true},
		{"LD V instruction", "ld", []byte{0x62, 0x00}, false},
		{"non-LD instruction", "jp", []byte{0x12, 0x00}, false},
		{"insufficient data", "ld", []byte{0xA2}, false},
		{"empty data", "ld", []byte{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := arch.isDataReferenceInstruction(tt.instrName, tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChip8_extractJumpTarget(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	tests := []struct {
		name     string
		data     []byte
		expected uint16
	}{
		{"JP instruction", []byte{0x12, 0x34}, 0x34},     // 0x234 - ProgramStart = 0x34
		{"CALL instruction", []byte{0x22, 0x34}, 0x34},   // 0x234 - ProgramStart = 0x34
		{"JP to low address", []byte{0x11, 0x50}, 0x150}, // 0x150 < ProgramStart, return as-is
		{"non-jump instruction", []byte{0x62, 0x34}, 0},
		{"insufficient data", []byte{0x12}, 0},
		{"empty data", []byte{}, 0},
		{"boundary max target", []byte{0x1F, 0xFF}, 0xDFF}, // MaxAddress -> MaxAddress - ProgramStart = 0xDFF
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := arch.extractJumpTarget(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChip8_extractDataReference(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	tests := []struct {
		name     string
		data     []byte
		expected uint16
	}{
		{"LD I instruction", []byte{0xA2, 0x34}, 0x34},     // 0x234 - ProgramStart = 0x34
		{"LD I to low address", []byte{0xA1, 0x50}, 0x150}, // 0x150 < ProgramStart, return as-is
		{"non-LD I instruction", []byte{0x62, 0x34}, 0},
		{"insufficient data", []byte{0xA2}, 0},
		{"empty data", []byte{}, 0},
		{"boundary max target", []byte{0xAF, 0xFF}, 0xDFF}, // MaxAddress -> MaxAddress - ProgramStart = 0xDFF
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := arch.extractDataReference(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChip8_formatInstruction(t *testing.T) {
	arch := New(parameter.New(parameter.Config{}))

	tests := []struct {
		name      string
		instrName string
		opcode    uint16
		expected  string
	}{
		{"CLS instruction", "cls", 0x00E0, ""},
		{"RET instruction", "ret", 0x00EE, ""},
		{"JP instruction", "jp", 0x1234, "$234"},
		{"JP V0 instruction", "jp", 0xB234, "V0, $234"},
		{"CALL instruction", "call", 0x2234, "$234"},
		{"SE Vx, byte", "se", 0x3234, "V2, $34"},
		{"SE Vx, Vy", "se", 0x5230, "V2, V3"},
		{"LD Vx, byte", "ld", 0x6234, "V2, $34"},
		{"LD Vx, Vy", "ld", 0x8230, "V2, V3"},
		{"LD I, addr", "ld", 0xA234, "I, $234"},
		{"ADD Vx, byte", "add", 0x7234, "V2, $34"},
		{"ADD Vx, Vy", "add", 0x8234, "V2, V3"},
		{"OR Vx, Vy", "or", 0x8231, "V2, V3"},
		{"SHR Vx", "shr", 0x8236, "V2"},
		{"RND Vx, byte", "rnd", 0xC234, "V2, $34"},
		{"DRW Vx, Vy, n", "drw", 0xD235, "V2, V3, $5"},
		{"SKP Vx", "skp", 0xE29E, "V2"},
		{"unknown instruction", "unknown", 0x0000, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := arch.formatInstruction(tt.instrName, tt.opcode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmark critical path operations
func BenchmarkChip8_GetAddressingParam(b *testing.B) {
	arch := New(parameter.New(parameter.Config{}))

	b.Run("uint16", func(b *testing.B) {
		for range b.N {
			_, _ = arch.GetAddressingParam(uint16(0x200))
		}
	})

	b.Run("int", func(b *testing.B) {
		for range b.N {
			_, _ = arch.GetAddressingParam(int(0x200))
		}
	})
}

func BenchmarkChip8_extractJumpTarget(b *testing.B) {
	arch := New(parameter.New(parameter.Config{}))
	data := []byte{0x12, 0x34}

	b.ResetTimer()
	for range b.N {
		_ = arch.extractJumpTarget(data)
	}
}

func BenchmarkChip8_formatInstruction(b *testing.B) {
	arch := New(parameter.New(parameter.Config{}))

	b.Run("simple", func(b *testing.B) {
		for range b.N {
			_ = arch.formatInstruction("cls", 0x00E0)
		}
	})

	b.Run("complex", func(b *testing.B) {
		for range b.N {
			_ = arch.formatInstruction("drw", 0xD235)
		}
	})
}
