package m6502

import (
	"github.com/retroenv/retrodisasm/internal/offset"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/log"
)

// complementaryBranches defines pairs of branch instructions that test opposite conditions of the same flag.
// If both instructions in a pair appear consecutively, they create an unconditional branch pattern.
var complementaryBranches = map[string]string{
	m6502.Beq.Name: m6502.Bne.Name, // Zero flag: equal vs not equal
	m6502.Bne.Name: m6502.Beq.Name,
	m6502.Bcc.Name: m6502.Bcs.Name, // Carry flag: clear vs set
	m6502.Bcs.Name: m6502.Bcc.Name,
	m6502.Bpl.Name: m6502.Bmi.Name, // Negative flag: plus vs minus
	m6502.Bmi.Name: m6502.Bpl.Name,
	m6502.Bvc.Name: m6502.Bvs.Name, // Overflow flag: clear vs set
	m6502.Bvs.Name: m6502.Bvc.Name,
}

// ComplementaryBranchPair represents a detected complementary branch sequence.
type ComplementaryBranchPair struct {
	FirstAddress  uint16
	SecondAddress uint16
	FirstBranch   string
	SecondBranch  string
}

// DetectComplementaryBranchSequence checks if the current branch instruction is followed by
// its complementary branch instruction, which would create an unconditional branch pattern.
// Records the pair for later processing and returns true if detected.
func (ar *Arch6502) DetectComplementaryBranchSequence(address uint16, offsetInfo *offset.DisasmOffset) bool {
	instruction := offsetInfo.Opcode.Instruction()

	// Check if this is a conditional branch instruction
	complementary, isConditionalBranch := complementaryBranches[instruction.Name()]
	if !isConditionalBranch {
		return false
	}

	// Read the next instruction
	nextAddress := address + uint16(len(offsetInfo.Data))
	nextByte, err := ar.dis.ReadMemory(nextAddress)
	if err != nil {
		return false
	}

	// Check if the next byte is a valid opcode
	nextOpcode := m6502.Opcodes[nextByte]
	if nextOpcode.Instruction == nil {
		return false
	}

	// Check if the next instruction is the complementary branch
	if nextOpcode.Instruction.Name != complementary {
		return false
	}

	// We found a complementary branch sequence - record it for post-processing
	ar.logger.Debug("Detected complementary branch sequence",
		log.Hex("first_address", address),
		log.String("first_branch", instruction.Name()),
		log.Hex("second_address", nextAddress),
		log.String("second_branch", nextOpcode.Instruction.Name))

	return true
}
