// Package arch contains types and functions used for multi architecture support.
// It acts as a bridge between the disassembler and the architecture specific code.
package arch

// Architecture contains architecture specific information.
type Architecture interface {
	// Constants returns the constants translation map.
	Constants() (map[uint16]Constant, error)
	// GetAddressingParam returns the address of the param if it references an address.
	GetAddressingParam(param any) (uint16, bool)
	// HandleDisambiguousInstructions translates disambiguous instructions into data bytes as it
	// has multiple opcodes for the same addressing mode which can result in different
	// bytes being assembled and make the resulting ROM not matching the original.
	HandleDisambiguousInstructions(dis Disasm, address uint16, offsetInfo *Offset) bool
	// Initialize the architecture.
	Initialize(dis Disasm) error
	// IsAddressingIndexed returns if the opcode is using indexed addressing.
	IsAddressingIndexed(opcode Opcode) bool
	// LastCodeAddress returns the last possible address of code.
	// This is used in systems where the last address is reserved for
	// the interrupt vector table.
	LastCodeAddress() uint16
	// ProcessOffset processes an offset and returns if the offset was processed and an error if any.
	ProcessOffset(dis Disasm, address uint16, offsetInfo *Offset) (bool, error)
	// ProcessVariableUsage processes the variable usage of an offset.
	ProcessVariableUsage(offsetInfo *Offset, reference string) error
	// ReadOpParam reads the parameter of an opcode.
	ReadOpParam(dis Disasm, addressing int, address uint16) (any, []byte, error)
}

// Constant represents a constant translation from a read and write operation to a name.
// This is used to replace the parameter of an instruction by a constant name.
type Constant struct {
	Address uint16

	Read  string
	Write string
}
