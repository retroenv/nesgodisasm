package m6502

import (
	"fmt"

	"github.com/retroenv/retrodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/log"
)

func (ar *Arch6502) Initialize() error {
	if err := ar.initializeIrqHandlers(); err != nil {
		return fmt.Errorf("initializing IRQ handlers: %w", err)
	}
	return nil
}

// initializeIrqHandlers reads the 3 IRQ handler addresses and adds them to the addresses to be
// followed for execution flow. Multiple handler can point to the same address.
// nolint:funlen
func (ar *Arch6502) initializeIrqHandlers() error {
	opts := ar.dis.Options()
	handlers := program.Handlers{
		NMI:   "0",
		Reset: "Reset",
		IRQ:   "0",
	}

	nmi, err := ar.dis.ReadMemoryWord(m6502.NMIAddress)
	if err != nil {
		return fmt.Errorf("reading NMI address: %w", err)
	}
	if nmi != 0 {
		ar.logger.Debug("NMI handler", log.String("address", fmt.Sprintf("0x%04X", nmi)))
		offsetInfo := ar.mapper.OffsetInfo(nmi)
		if offsetInfo != nil {
			offsetInfo.Label = "NMI"
			offsetInfo.SetType(program.CallDestination)
		}
		handlers.NMI = "NMI"
	}

	var reset uint16
	if opts.Binary {
		reset = uint16(nes.CodeBaseAddress)
	} else {
		reset, err = ar.dis.ReadMemoryWord(m6502.ResetAddress)
		if err != nil {
			return fmt.Errorf("reading reset address: %w", err)
		}
	}

	ar.logger.Debug("Reset handler", log.String("address", fmt.Sprintf("0x%04X", reset)))
	offsetInfo := ar.mapper.OffsetInfo(reset)
	if offsetInfo != nil {
		if offsetInfo.Label != "" {
			handlers.NMI = "Reset"
		}
		offsetInfo.Label = "Reset"
		offsetInfo.SetType(program.CallDestination)
	}

	irq, err := ar.dis.ReadMemoryWord(m6502.IrqAddress)
	if err != nil {
		return fmt.Errorf("reading IRQ address: %w", err)
	}
	if irq != 0 {
		ar.logger.Debug("IRQ handler", log.String("address", fmt.Sprintf("0x%04X", irq)))
		offsetInfo = ar.mapper.OffsetInfo(irq)
		if offsetInfo != nil {
			if offsetInfo.Label == "" {
				offsetInfo.Label = "IRQ"
				handlers.IRQ = "IRQ"
			} else {
				handlers.IRQ = offsetInfo.Label
			}
			offsetInfo.SetType(program.CallDestination)
		}
	}

	if nmi == reset {
		handlers.NMI = handlers.Reset
	}
	if irq == reset {
		handlers.IRQ = handlers.Reset
	}

	ar.calculateCodeBaseAddress(reset)

	// add IRQ handlers to be parsed after the code base address has been calculated
	ar.dis.AddAddressToParse(nmi, nmi, 0, nil, false)
	ar.dis.AddAddressToParse(reset, reset, 0, nil, false)
	ar.dis.AddAddressToParse(irq, irq, 0, nil, false)

	ar.dis.SetHandlers(handlers)
	return nil
}

// calculateCodeBaseAddress calculates the code base address that is assumed by the code.
// If the code size is only 0x4000 it will be mirror-mapped into the 0x8000 byte of RAM starting at
// 0x8000. The handlers can be set to any of the 2 mirrors as base, based on this the code base
// address is calculated. This ensures that jsr instructions will result in the same opcode, as it
// is based on the code base address.
func (ar *Arch6502) calculateCodeBaseAddress(resetHandler uint16) {
	cart := ar.dis.Cart()
	halfPrg := len(cart.PRG) % 0x8000
	codeBaseAddress := uint16(0x8000 + halfPrg)
	vectorsStartAddress := uint16(m6502.InterruptVectorStartAddress)

	// fix up calculated code base address for half sized PRG ROMs that have a different
	// code base address configured in the assembler, like "M.U.S.C.L.E."
	if resetHandler < codeBaseAddress {
		codeBaseAddress = nes.CodeBaseAddress
		vectorsStartAddress -= uint16(halfPrg)
	}

	ar.dis.SetCodeBaseAddress(codeBaseAddress)
	ar.dis.SetVectorsStartAddress(vectorsStartAddress)
}
