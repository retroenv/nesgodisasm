package program

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestOffset_IsType(t *testing.T) {
	offset := &Offset{}

	offset.SetType(CodeOffset)
	assert.True(t, offset.IsType(CodeOffset))
	assert.False(t, offset.IsType(DataOffset))

	offset.SetType(DataOffset)
	assert.True(t, offset.IsType(CodeOffset))
	assert.True(t, offset.IsType(DataOffset))
}

func TestOffset_SetType(t *testing.T) {
	offset := &Offset{}

	assert.False(t, offset.IsType(CodeOffset))
	offset.SetType(CodeOffset)
	assert.True(t, offset.IsType(CodeOffset))

	offset.SetType(DataOffset)
	assert.True(t, offset.IsType(CodeOffset))
	assert.True(t, offset.IsType(DataOffset))
}

func TestOffset_ClearType(t *testing.T) {
	offset := &Offset{}
	offset.SetType(CodeOffset)
	offset.SetType(DataOffset)

	assert.True(t, offset.IsType(CodeOffset))
	assert.True(t, offset.IsType(DataOffset))

	offset.ClearType(CodeOffset)
	assert.False(t, offset.IsType(CodeOffset))
	assert.True(t, offset.IsType(DataOffset))

	offset.ClearType(DataOffset)
	assert.False(t, offset.IsType(CodeOffset))
	assert.False(t, offset.IsType(DataOffset))
}

func TestOffset_HexCodeComment(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "single byte",
			data:     []byte{0xA9},
			expected: "A9",
		},
		{
			name:     "two bytes",
			data:     []byte{0xA9, 0xFF},
			expected: "A9 FF",
		},
		{
			name:     "three bytes",
			data:     []byte{0x4C, 0x00, 0x80},
			expected: "4C 00 80",
		},
		{
			name:     "empty data",
			data:     []byte{},
			expected: "",
		},
		{
			name:     "multiple bytes with different values",
			data:     []byte{0x00, 0x01, 0xFE, 0xFF},
			expected: "00 01 FE FF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := &Offset{
				Data: tt.data,
			}
			comment, err := offset.HexCodeComment()
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, comment)
		})
	}
}
