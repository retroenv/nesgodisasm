package arch

import "github.com/retroenv/retrodisasm/internal/program"

// Offset defines the content of an offset in a program that can represent data or code.
type Offset struct {
	program.Offset

	Opcode Opcode // opcode this offset represents

	BranchFrom  []BankReference // list of all addresses that branch to this offset
	BranchingTo string          // label to jump to if instruction branches
	Context     uint16          // function or interrupt context that the offset is part of
}
