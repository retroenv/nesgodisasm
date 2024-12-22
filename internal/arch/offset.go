package arch

import "github.com/retroenv/nesgodisasm/internal/program"

// Offset represents an offset in the disassembled code.
type Offset interface {
	// ClearType clears the offset type.
	ClearType(offsetType program.OffsetType)
	// Code returns the code string of the offset.
	Code() string
	// Comment returns the comment of the offset.
	Comment() string
	// Context returns the context of the offset.
	Context() uint16
	// Data returns the opcode bytes of the offset.
	Data() []byte
	// Instruction returns the instruction of the offset.
	Instruction() Instruction
	// IsNil returns true if the offset is nil.
	IsNil() bool
	// IsType returns true if the offset type is the given type.
	IsType(typ program.OffsetType) bool
	// Label returns the label of the offset, can be empty.
	Label() string
	// Opcode returns the opcode of the offset.
	Opcode() Opcode
	// SetCode sets the code string of the offset.
	SetCode(string)
	// SetComment sets the comment of the offset.
	SetComment(string)
	// SetData sets the opcode bytes of the offset.
	SetData([]byte)
	// SetLabel sets the label of the offset.
	SetLabel(string)
	// SetOpcode sets the opcode of the offset.
	SetOpcode(Opcode)
	// SetType sets the offset type.
	SetType(offsetType program.OffsetType)
}
