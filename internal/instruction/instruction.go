// Package instruction contains fundamental types for CPU instructions and opcodes.
package instruction

// Instruction represents a CPU instruction.
type Instruction interface {
	// IsCall returns true if the instruction is a call.
	IsCall() bool
	// IsNil returns true if the instruction is nil.
	IsNil() bool
	// Name returns the instruction name.
	Name() string
	// Unofficial returns true if the instruction is not official.
	Unofficial() bool
}
