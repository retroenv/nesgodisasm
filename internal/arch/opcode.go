package arch

// Opcode represents an opcode.
type Opcode interface {
	// Addressing returns the addressing mode of the opcode.
	Addressing() int
	// Instruction returns the instruction of the opcode.
	Instruction() Instruction
	// ReadsMemory returns true if the opcode reads memory.
	ReadsMemory() bool
	// ReadWritesMemory returns true if the opcode reads and writes memory.
	ReadWritesMemory() bool
	// WritesMemory returns true if the opcode writes memory.
	WritesMemory() bool
}
