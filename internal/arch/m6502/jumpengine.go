package m6502

import (
	"encoding/binary"
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/log"
)

const (
	jumpEngineMaxContextSize = 0x25
)

// checkForJumpEngineJmp checks if the current instruction is the jump instruction inside a jump engine function.
// The function offsets after the call to the jump engine will be used as destinations to disassemble as code.
// This can be found in some official games like Super Mario Bros.
func (ar *Arch6502) checkForJumpEngineJmp(dis arch.Disasm, jumpAddress uint16, offsetInfo arch.Offset) error {
	instruction := offsetInfo.Opcode().Instruction()
	addressing := m6502.AddressingMode(offsetInfo.Opcode().Addressing())
	if instruction.Name() != m6502.Jmp.Name || addressing != m6502.IndirectAddressing {
		return nil
	}

	jumpEngine := dis.JumpEngine()
	contextOffsets, contextAddresses := jumpEngine.JumpContextInfo(dis, jumpAddress, offsetInfo)
	contextSize := jumpAddress - offsetInfo.Context() + 3
	dataReferences, err := jumpEngine.GetContextDataReferences(dis, contextOffsets, contextAddresses)
	if err != nil {
		return fmt.Errorf("getting context data references: %w", err)
	}

	if len(dataReferences) > 1 {
		jumpEngine.GetFunctionTableReference(offsetInfo.Context(), dataReferences)
	}

	dis.Logger().Debug("Jump engine detected",
		log.String("address", fmt.Sprintf("0x%04X", jumpAddress)),
		log.Uint16("code_size", contextSize),
	)

	// if code reaches this point, no branching instructions beside the final indirect jmp have been found
	// in the function, this makes it likely a jump engine
	jumpEngine.AddJumpEngine(offsetInfo.Context())

	if contextSize < jumpEngineMaxContextSize {
		if err := jumpEngine.HandleJumpEngineCallers(dis, offsetInfo.Context()); err != nil {
			return fmt.Errorf("handling jump engine callers: %w", err)
		}
		return nil
	}
	offsetInfo.SetComment("jump engine detected")
	return nil
}

// checkForJumpEngineCall checks if the current instruction is a call into a jump engine function.
func (ar *Arch6502) checkForJumpEngineCall(dis arch.Disasm, address uint16, offsetInfo arch.Offset) error {
	instruction := offsetInfo.Opcode().Instruction()
	addressing := m6502.AddressingMode(offsetInfo.Opcode().Addressing())
	if instruction.Name() != m6502.Jsr.Name || addressing != m6502.AbsoluteAddressing {
		return nil
	}

	pc := dis.ProgramCounter()
	_, opcodes, err := ar.ReadOpParam(dis, offsetInfo.Opcode().Addressing(), pc)
	if err != nil {
		return err
	}

	jumpEngine := dis.JumpEngine()
	destination := binary.LittleEndian.Uint16(opcodes)
	if err := jumpEngine.HandleJumpEngineDestination(dis, address, destination); err != nil {
		return fmt.Errorf("handling jump engine destination: %w", err)
	}
	return nil
}
