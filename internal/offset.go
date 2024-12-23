package disasm

import (
	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
)

var _ arch.Offset = &offset{}

// offset defines the content of an offset in a program that can represent data or code.
type offset struct {
	program.Offset

	opcode arch.Opcode // opcode this offset represents

	branchFrom  []bankReference // list of all addresses that branch to this offset
	branchingTo string          // label to jump to if instruction branches
	context     uint16          // function or interrupt context that the offset is part of
}

func (o *offset) Label() string {
	return o.Offset.Label
}

func (o *offset) SetLabel(s string) {
	o.Offset.Label = s
}

func (o *offset) Code() string {
	return o.Offset.Code
}

func (o *offset) Comment() string {
	return o.Offset.Comment
}

func (o *offset) Opcode() arch.Opcode {
	return o.opcode
}

func (o *offset) Data() []byte {
	return o.Offset.Data
}

func (o *offset) SetCode(s string) {
	o.Offset.Code = s
}

func (o *offset) SetComment(s string) {
	o.Offset.Comment = s
}

func (o *offset) SetOpcode(opcode arch.Opcode) {
	o.opcode = opcode
}

func (o *offset) SetData(bytes []byte) {
	o.Offset.Data = bytes
}

func (o *offset) Instruction() arch.Instruction {
	return o.opcode.Instruction()
}

func (o *offset) Context() uint16 {
	return o.context
}

func (o *offset) IsNil() bool {
	return o == nil
}

func (o *offset) Type() program.OffsetType {
	return o.Offset.Type
}

func (o *offset) SetLabelComment(s string) {
	o.Offset.LabelComment = s
}

func (o *offset) BranchFrom() []uint16 {
	branches := make([]uint16, 0, len(o.branchFrom))
	for _, ref := range o.branchFrom {
		branches = append(branches, ref.address)
	}
	return branches
}
