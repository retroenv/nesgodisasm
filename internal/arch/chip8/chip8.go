// Package chip8 provides CHIP-8 architecture specific disassembler implementation.
// CHIP-8 is an interpreted programming language from the 1970s designed for simple games.
// This package handles disassembly of CHIP-8 ROM files into assembly code.
package chip8

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/chip8"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/parameter"
)

// CHIP-8 memory layout constants.
const (
	// ProgramStart is the memory address where CHIP-8 programs begin execution.
	// CHIP-8 programs are loaded at address 0x200 in the virtual machine's memory space,
	// but stored starting at offset 0x0 in ROM files.
	ProgramStart = 0x200

	// MaxAddress is the highest valid address in CHIP-8 memory space (4KB total).
	MaxAddress = 0xFFF

	// LastCodeAddress is the typical end of user program space,
	// before the interpreter and system area.
	LastCodeAddress = 0xEFF
)

// Compile-time check to ensure Chip8 implements arch.Architecture.
var _ arch.Architecture = (*Chip8)(nil)

// New returns a new CHIP-8 architecture configuration.
func New(converter parameter.Converter) *Chip8 {
	return &Chip8{
		converter: converter,
	}
}

// Chip8 implements the arch.Architecture interface for CHIP-8 processors.
// CHIP-8 is an interpreted programming language with 4KB of memory,
// 16 general-purpose 8-bit registers, and a simple instruction set.
type Chip8 struct {
	converter parameter.Converter
}

// Constants returns architecture-specific constants for CHIP-8.
// CHIP-8 doesn't have hardware constants like the NES, so returns empty map.
func (c *Chip8) Constants() (map[uint16]arch.Constant, error) {
	return map[uint16]arch.Constant{}, nil
}

// GetAddressingParam extracts addressing parameters from CHIP-8 instructions.
// CHIP-8 uses direct addressing with 12-bit addresses embedded in opcodes.
func (c *Chip8) GetAddressingParam(param any) (uint16, bool) {
	switch p := param.(type) {
	case uint16:
		return p, true
	case int:
		// Validate address range for CHIP-8 (12-bit addresses)
		if p >= 0 && p <= MaxAddress {
			return uint16(p), true
		}
	}
	return 0, false
}

// HandleDisambiguousInstructions handles instructions that could be interpreted multiple ways.
// CHIP-8 has a simple instruction set with no ambiguous opcodes.
func (c *Chip8) HandleDisambiguousInstructions(dis arch.Disasm, address uint16, offsetInfo *arch.Offset) bool {
	return false
}

// Initialize sets up the disassembler for CHIP-8 ROM analysis.
// CHIP-8 programs are stored starting at ROM offset 0 but execute at memory address 0x200.
func (c *Chip8) Initialize(dis arch.Disasm) error {
	dis.AddAddressToParse(0, 0, 0, nil, false)
	return nil
}

// IsAddressingIndexed determines if an opcode uses indexed addressing.
// CHIP-8 uses register-based addressing rather than indexed addressing.
func (c *Chip8) IsAddressingIndexed(opcode arch.Opcode) bool {
	return false
}

// LastCodeAddress returns the highest valid code address for CHIP-8.
// CHIP-8 memory is 4KB (0x000-0xFFF) but interpreter occupies 0x000-0x1FF,
// so user programs typically end before 0xF00 to avoid interpreter area.
func (c *Chip8) LastCodeAddress() uint16 {
	return LastCodeAddress
}

// ProcessOffset processes a CHIP-8 instruction at the given address.
// It parses the instruction, formats it for assembly output, and handles
// control flow analysis for jumps, calls, and data references.
func (c *Chip8) ProcessOffset(dis arch.Disasm, address uint16, offsetInfo *arch.Offset) (bool, error) {
	inspectCode, err := initializeOffsetInfo(dis, offsetInfo)
	if err != nil {
		return false, err
	}
	if !inspectCode {
		return false, nil
	}

	op := offsetInfo.Opcode
	instruction := op.Instruction()
	name := instruction.Name()
	pc := dis.ProgramCounter()

	// Process CHIP-8 instruction parameters
	opcodeBytes := offsetInfo.Data
	if len(opcodeBytes) >= 2 {
		opcode := uint16(opcodeBytes[0])<<8 | uint16(opcodeBytes[1])

		// Format instruction based on CHIP-8 patterns
		if params := c.formatInstruction(name, opcode); params != "" {
			offsetInfo.Code = fmt.Sprintf("%s %s", name, params)
		} else {
			offsetInfo.Code = name
		}
	} else {
		offsetInfo.Code = name
	}

	// Handle control flow for CHIP-8
	// Process control flow based on instruction type
	switch {
	case c.isJumpInstruction(name):
		if target := c.extractJumpTarget(offsetInfo.Data); target != 0 {
			dis.AddAddressToParse(target, offsetInfo.Context, address, instruction, true)
		}
	case c.isCallInstruction(name):
		if target := c.extractJumpTarget(offsetInfo.Data); target != 0 {
			dis.AddAddressToParse(target, offsetInfo.Context, address, instruction, true)
		}
		// Continue execution after call
		nextAddr := pc + uint16(len(offsetInfo.Data))
		dis.AddAddressToParse(nextAddr, offsetInfo.Context, address, instruction, false)
	case c.isDataReferenceInstruction(name, offsetInfo.Data):
		if target := c.extractDataReference(offsetInfo.Data); target != 0 {
			dis.AddAddressToParse(target, offsetInfo.Context, address, instruction, true)
		}
		// Continue to next instruction
		nextAddr := pc + uint16(len(offsetInfo.Data))
		dis.AddAddressToParse(nextAddr, offsetInfo.Context, address, instruction, false)
	case !c.isReturnInstruction(name):
		// Continue to next instruction for non-terminating instructions
		nextAddr := pc + uint16(len(offsetInfo.Data))
		dis.AddAddressToParse(nextAddr, offsetInfo.Context, address, instruction, false)
	}

	return true, nil
}

// ProcessVariableUsage processes variable usage patterns in CHIP-8 instructions.
// CHIP-8 uses simple direct addressing and register operations without complex variable patterns.
func (c *Chip8) ProcessVariableUsage(offsetInfo *arch.Offset, reference string) error {
	return nil
}

// ReadOpParam reads additional operation parameters for CHIP-8 instructions.
// CHIP-8 opcodes are 2 bytes with all parameters embedded, so no additional reads needed.
func (c *Chip8) ReadOpParam(dis arch.Disasm, addressing int, address uint16) (any, []byte, error) {
	return nil, nil, nil
}

// BankWindowSize returns the bank window size.
// CHIP-8 doesn't use banking, return 0 for single bank mapping.
func (c *Chip8) BankWindowSize(_ *cartridge.Cartridge) int {
	return 0
}

// formatInstruction formats a CHIP-8 instruction with its parameters.
// Returns the formatted parameter string for the given instruction.
func (c *Chip8) formatInstruction(name string, opcode uint16) string {
	switch name {
	case chip8.Cls.Name, chip8.Ret.Name:
		return "" // No parameters
	case chip8.Jp.Name:
		return c.formatJumpInstruction(opcode)
	case chip8.Call.Name:
		return fmt.Sprintf("$%03X", opcode&0x0FFF)
	case chip8.Se.Name, chip8.Sne.Name:
		return c.formatCompareInstruction(opcode)
	case chip8.Ld.Name:
		return c.formatLoadInstruction(opcode)
	case chip8.Add.Name:
		return c.formatAddInstruction(opcode)
	case chip8.Or.Name, chip8.And.Name, chip8.Xor.Name, chip8.Sub.Name, chip8.Subn.Name:
		return c.formatBinaryInstruction(opcode)
	case chip8.Shr.Name, chip8.Shl.Name:
		return c.formatShiftInstruction(opcode)
	case chip8.Rnd.Name:
		return c.formatRandomInstruction(opcode)
	case chip8.Drw.Name:
		return c.formatDrawInstruction(opcode)
	case chip8.Skp.Name, chip8.Sknp.Name:
		return c.formatSkipInstruction(opcode)
	}
	return ""
}

// formatJumpInstruction formats jump instructions (JP addr, JP V0+addr).
func (c *Chip8) formatJumpInstruction(opcode uint16) string {
	if opcode&0xF000 == 0x1000 {
		return fmt.Sprintf("$%03X", opcode&0x0FFF)
	}
	if opcode&0xF000 == 0xB000 {
		return fmt.Sprintf("V0, $%03X", opcode&0x0FFF)
	}
	return ""
}

// formatCompareInstruction formats comparison instructions (SE, SNE).
func (c *Chip8) formatCompareInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	// SE/SNE instructions:
	// 3XNN: SE Vx, byte
	// 4XNN: SNE Vx, byte
	// 5XY0: SE Vx, Vy
	// 9XY0: SNE Vx, Vy
	switch opcode & 0xF000 {
	case 0x3000, 0x4000:
		// SE/SNE Vx, byte
		return fmt.Sprintf("V%X, $%02X", x, opcode&0x00FF)
	case 0x5000, 0x9000:
		// SE/SNE Vx, Vy
		y := (opcode & 0x00F0) >> 4
		return fmt.Sprintf("V%X, V%X", x, y)
	}
	return ""
}

// formatLoadInstruction formats load instructions (LD Vx, byte/Vy/I).
func (c *Chip8) formatLoadInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	switch opcode & 0xF000 {
	case 0x6000:
		return fmt.Sprintf("V%X, $%02X", x, opcode&0x00FF)
	case 0x8000:
		y := (opcode & 0x00F0) >> 4
		return fmt.Sprintf("V%X, V%X", x, y)
	case 0xA000:
		return fmt.Sprintf("I, $%03X", opcode&0x0FFF)
	}
	return ""
}

// formatAddInstruction formats add instructions (ADD Vx, byte/Vy).
func (c *Chip8) formatAddInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	if opcode&0xF000 == 0x7000 {
		return fmt.Sprintf("V%X, $%02X", x, opcode&0x00FF)
	}
	if opcode&0xF000 == 0x8000 {
		y := (opcode & 0x00F0) >> 4
		return fmt.Sprintf("V%X, V%X", x, y)
	}
	return ""
}

// formatBinaryInstruction formats binary operation instructions (OR, AND, XOR, SUB, SUBN).
func (c *Chip8) formatBinaryInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	y := (opcode & 0x00F0) >> 4
	return fmt.Sprintf("V%X, V%X", x, y)
}

// formatShiftInstruction formats shift instructions (SHR, SHL).
func (c *Chip8) formatShiftInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	return fmt.Sprintf("V%X", x)
}

// formatRandomInstruction formats random number instructions (RND).
func (c *Chip8) formatRandomInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	return fmt.Sprintf("V%X, $%02X", x, opcode&0x00FF)
}

// formatDrawInstruction formats draw instructions (DRW).
func (c *Chip8) formatDrawInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	y := (opcode & 0x00F0) >> 4
	n := opcode & 0x000F
	return fmt.Sprintf("V%X, V%X, $%X", x, y, n)
}

// formatSkipInstruction formats skip instructions (SKP, SKNP).
func (c *Chip8) formatSkipInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	return fmt.Sprintf("V%X", x)
}

// isJumpInstruction returns true if the instruction is a jump.
func (c *Chip8) isJumpInstruction(name string) bool {
	return name == chip8.Jp.Name
}

// isCallInstruction returns true if the instruction is a call.
func (c *Chip8) isCallInstruction(name string) bool {
	return name == chip8.Call.Name
}

// isReturnInstruction returns true if the instruction is a return.
func (c *Chip8) isReturnInstruction(name string) bool {
	return name == chip8.Ret.Name
}

// isDataReferenceInstruction returns true if the instruction references data (LD I, addr).
func (c *Chip8) isDataReferenceInstruction(name string, data []byte) bool {
	if name != chip8.Ld.Name || len(data) < 2 {
		return false
	}
	opcode := uint16(data[0])<<8 | uint16(data[1])
	// Only ld I, address instructions (0xAXXX)
	return (opcode & 0xF000) == 0xA000
}

// extractJumpTarget extracts the jump target address from instruction bytes.
func (c *Chip8) extractJumpTarget(data []byte) uint16 {
	if len(data) < 2 {
		return 0
	}
	opcode := uint16(data[0])<<8 | uint16(data[1])

	// For JP and CALL instructions, target is in lower 12 bits
	if (opcode&0xF000) == 0x1000 || (opcode&0xF000) == 0x2000 {
		target := opcode & 0x0FFF
		// Validate target address is within CHIP-8 address space
		if target > MaxAddress {
			return 0
		}
		// Convert from CHIP-8 memory space to ROM offset
		if target >= ProgramStart {
			return target - ProgramStart
		}
		return target
	}

	return 0
}

// extractDataReference extracts the data reference address from LD I, addr instruction.
func (c *Chip8) extractDataReference(data []byte) uint16 {
	if len(data) < 2 {
		return 0
	}
	opcode := uint16(data[0])<<8 | uint16(data[1])

	// For ld I, address instruction (0xAXXX), address is in lower 12 bits
	if (opcode & 0xF000) == 0xA000 {
		target := opcode & 0x0FFF
		// Validate target address is within CHIP-8 address space
		if target > MaxAddress {
			return 0
		}
		// Convert from CHIP-8 memory space to ROM offset
		if target >= ProgramStart {
			return target - ProgramStart
		}
		return target
	}

	return 0
}

// ReadMemory reads a byte from memory using CHIP-8-specific memory mapping.
// CHIP-8 programs use a 4KB address space starting at 0x000.
func (c *Chip8) ReadMemory(dis arch.Disasm, address uint16) (byte, error) {
	value := dis.Mapper().ReadMemory(address)
	return value, nil
}
