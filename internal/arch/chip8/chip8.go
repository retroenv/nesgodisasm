// Package chip8 provides a CHIP-8 architecture specific disassembler code.
package chip8

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/chip8"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/parameter"
)

var _ arch.Architecture = &Chip8{}

// New returns a new CHIP-8 architecture configuration.
func New(converter parameter.Converter) *Chip8 {
	return &Chip8{
		converter: converter,
	}
}

type Chip8 struct {
	converter parameter.Converter
}

func (c *Chip8) Constants() (map[uint16]arch.Constant, error) {
	return map[uint16]arch.Constant{}, nil
}

func (c *Chip8) GetAddressingParam(param any) (uint16, bool) {
	// CHIP-8 addressing parameters are typically direct addresses
	switch p := param.(type) {
	case uint16:
		return p, true
	case int:
		if p >= 0 && p <= 0xFFFF {
			return uint16(p), true
		}
	}
	return 0, false
}

func (c *Chip8) HandleDisambiguousInstructions(dis arch.Disasm, address uint16, offsetInfo *arch.Offset) bool {
	// CHIP-8 doesn't have disambiguous instructions like 6502
	return false
}

func (c *Chip8) Initialize(dis arch.Disasm) error {
	// CHIP-8 programs are loaded starting at address 0 in the ROM file
	// but execute starting at 0x200 in CHIP-8 memory space
	// For disassembly purposes, we start at 0 which maps to the ROM data
	dis.AddAddressToParse(0, 0, 0, nil, false)
	return nil
}

func (c *Chip8) IsAddressingIndexed(opcode arch.Opcode) bool {
	// CHIP-8 has indexed addressing for register operations
	// This would depend on the specific instruction
	return false
}

func (c *Chip8) LastCodeAddress() uint16 {
	// CHIP-8 memory goes up to 0xFFF, but typically code ends before interpreter area
	return 0xEFF
}

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
		params := c.formatInstruction(name, opcode)
		if params != "" {
			offsetInfo.Code = fmt.Sprintf("%s %s", name, params)
		} else {
			offsetInfo.Code = name
		}
	} else {
		offsetInfo.Code = name
	}

	// Handle control flow for CHIP-8
	switch {
	case c.isJumpInstruction(name):
		// Extract jump target from instruction
		if target := c.extractJumpTarget(offsetInfo.Data); target != 0 {
			dis.AddAddressToParse(target, offsetInfo.Context, address, instruction, true)
		}
	case c.isCallInstruction(name):
		// Extract call target from instruction
		if target := c.extractJumpTarget(offsetInfo.Data); target != 0 {
			dis.AddAddressToParse(target, offsetInfo.Context, address, instruction, true)
		}
		// Continue execution after call
		nextAddr := pc + uint16(len(offsetInfo.Data))
		dis.AddAddressToParse(nextAddr, offsetInfo.Context, address, instruction, false)
	case c.isDataReferenceInstruction(name, offsetInfo.Data):
		// Extract data reference from instruction (ld I, address)
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

func (c *Chip8) ProcessVariableUsage(offsetInfo *arch.Offset, reference string) error {
	// CHIP-8 doesn't have complex variable usage patterns like NES
	// Most addressing is direct or register-based
	return nil
}

func (c *Chip8) ReadOpParam(dis arch.Disasm, addressing int, address uint16) (any, []byte, error) {
	// CHIP-8 opcodes are 2 bytes, parameters are embedded in the opcode
	// This method is primarily used for reading additional parameter bytes
	// which CHIP-8 doesn't typically have beyond the 2-byte opcode
	return nil, nil, nil
}

// BankWindowSize returns the bank window size.
// CHIP-8 doesn't use banking, return 0 for single bank mapping.
func (c *Chip8) BankWindowSize(_ *cartridge.Cartridge) int {
	return 0
}

// formatInstruction formats a CHIP-8 instruction with its parameters
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

// formatJumpInstruction formats jump instructions
func (c *Chip8) formatJumpInstruction(opcode uint16) string {
	if opcode&0xF000 == 0x1000 {
		return fmt.Sprintf("$%03X", opcode&0x0FFF)
	}
	if opcode&0xF000 == 0xB000 {
		return fmt.Sprintf("V0, $%03X", opcode&0x0FFF)
	}
	return ""
}

// formatCompareInstruction formats comparison instructions
func (c *Chip8) formatCompareInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	if opcode&0x00F0 == 0 {
		return fmt.Sprintf("V%X, $%02X", x, opcode&0x00FF)
	}
	y := (opcode & 0x00F0) >> 4
	return fmt.Sprintf("V%X, V%X", x, y)
}

// formatLoadInstruction formats load instructions
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

// formatAddInstruction formats add instructions
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

// formatBinaryInstruction formats binary operation instructions
func (c *Chip8) formatBinaryInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	y := (opcode & 0x00F0) >> 4
	return fmt.Sprintf("V%X, V%X", x, y)
}

// formatShiftInstruction formats shift instructions
func (c *Chip8) formatShiftInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	return fmt.Sprintf("V%X", x)
}

// formatRandomInstruction formats random number instructions
func (c *Chip8) formatRandomInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	return fmt.Sprintf("V%X, $%02X", x, opcode&0x00FF)
}

// formatDrawInstruction formats draw instructions
func (c *Chip8) formatDrawInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	y := (opcode & 0x00F0) >> 4
	n := opcode & 0x000F
	return fmt.Sprintf("V%X, V%X, $%X", x, y, n)
}

// formatSkipInstruction formats skip instructions
func (c *Chip8) formatSkipInstruction(opcode uint16) string {
	x := (opcode & 0x0F00) >> 8
	return fmt.Sprintf("V%X", x)
}

// isJumpInstruction returns true if the instruction is a jump
func (c *Chip8) isJumpInstruction(name string) bool {
	return name == chip8.Jp.Name
}

// isCallInstruction returns true if the instruction is a call
func (c *Chip8) isCallInstruction(name string) bool {
	return name == chip8.Call.Name
}

// isReturnInstruction returns true if the instruction is a return
func (c *Chip8) isReturnInstruction(name string) bool {
	return name == chip8.Ret.Name
}

// isDataReferenceInstruction returns true if the instruction references data (ld I, address)
func (c *Chip8) isDataReferenceInstruction(name string, data []byte) bool {
	if name != chip8.Ld.Name || len(data) < 2 {
		return false
	}
	opcode := uint16(data[0])<<8 | uint16(data[1])
	// Only ld I, address instructions (0xAXXX)
	return (opcode & 0xF000) == 0xA000
}

// extractJumpTarget extracts the jump target address from instruction bytes
func (c *Chip8) extractJumpTarget(data []byte) uint16 {
	if len(data) < 2 {
		return 0
	}
	opcode := uint16(data[0])<<8 | uint16(data[1])

	// For JP and CALL instructions, target is in lower 12 bits
	if (opcode&0xF000) == 0x1000 || (opcode&0xF000) == 0x2000 {
		return opcode & 0x0FFF
	}

	return 0
}

// extractDataReference extracts the data reference address from ld I, address instruction
func (c *Chip8) extractDataReference(data []byte) uint16 {
	if len(data) < 2 {
		return 0
	}
	opcode := uint16(data[0])<<8 | uint16(data[1])

	// For ld I, address instruction (0xAXXX), address is in lower 12 bits
	if (opcode & 0xF000) == 0xA000 {
		return opcode & 0x0FFF
	}

	return 0
}

// ReadMemory reads a byte from memory using CHIP-8-specific memory mapping.
func (c *Chip8) ReadMemory(dis arch.Disasm, address uint16) (byte, error) {
	// For CHIP-8, all memory reads go through the mapper
	// since CHIP-8 programs use the full address space starting at 0
	value := dis.Mapper().ReadMemory(address)
	return value, nil
}
