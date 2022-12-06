package program

// OffsetType defines the type of a program offset.
type OffsetType uint8

// addressing modes.
const (
	UnknownOffset OffsetType = 0
	CodeOffset    OffsetType = 1 << iota
	DataOffset
	CodeAsData        // for branches into instructions and unofficial instructions
	CallDestination   // opcode is the destination of a jsr call, indicating a subroutine
	FunctionReference // reference to a function
)

// IsType returns whether the offset is of given type.
func (o *Offset) IsType(typ OffsetType) bool {
	ret := o.Type&typ != 0
	return ret
}

// SetType sets the type of the offset.
func (o *Offset) SetType(typ OffsetType) {
	o.Type |= typ
}

// ClearType unsets the type of the offset.
func (o *Offset) ClearType(typ OffsetType) {
	mask := ^(typ)
	o.Type &= mask
}
