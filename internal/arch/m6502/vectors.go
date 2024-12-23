package m6502

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/arch/nes"
	"github.com/retroenv/retrogolib/log"
)

func (ar *Arch6502) Initialize(dis arch.Disasm) error {
	if err := ar.initializeIrqHandlers(dis); err != nil {
		return fmt.Errorf("initializing IRQ handlers: %w", err)
	}
	return nil
}

// initializeIrqHandlers reads the 3 IRQ handler addresses and adds them to the addresses to be
// followed for execution flow. Multiple handler can point to the same address.
// nolint:funlen
func (ar *Arch6502) initializeIrqHandlers(dis arch.Disasm) error {
	logger := dis.Logger()
	opts := dis.Options()
	handlers := program.Handlers{
		NMI:   "0",
		Reset: "Reset",
		IRQ:   "0",
	}

	nmi, err := dis.ReadMemoryWord(m6502.NMIAddress)
	if err != nil {
		return fmt.Errorf("reading NMI address: %w", err)
	}
	if nmi != 0 {
		logger.Debug("NMI handler", log.String("address", fmt.Sprintf("0x%04X", nmi)))
		offsetInfo := dis.OffsetInfo(nmi)
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
		reset, err = dis.ReadMemoryWord(m6502.ResetAddress)
		if err != nil {
			return fmt.Errorf("reading reset address: %w", err)
		}
	}

	logger.Debug("Reset handler", log.String("address", fmt.Sprintf("0x%04X", reset)))
	offsetInfo := dis.OffsetInfo(reset)
	if offsetInfo != nil {
		if offsetInfo.Label != "" {
			handlers.NMI = "Reset"
		}
		offsetInfo.Label = "Reset"
		offsetInfo.SetType(program.CallDestination)
	}

	irq, err := dis.ReadMemoryWord(m6502.IrqAddress)
	if err != nil {
		return fmt.Errorf("reading IRQ address: %w", err)
	}
	if irq != 0 {
		logger.Debug("IRQ handler", log.String("address", fmt.Sprintf("0x%04X", irq)))
		offsetInfo = dis.OffsetInfo(irq)
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

	ar.calculateCodeBaseAddress(dis, reset)

	// add IRQ handlers to be parsed after the code base address has been calculated
	dis.AddAddressToParse(nmi, nmi, 0, nil, false)
	dis.AddAddressToParse(reset, reset, 0, nil, false)
	dis.AddAddressToParse(irq, irq, 0, nil, false)

	dis.SetHandlers(handlers)
	return nil
}

// calculateCodeBaseAddress calculates the code base address that is assumed by the code.
// If the code size is only 0x4000 it will be mirror-mapped into the 0x8000 byte of RAM starting at
// 0x8000. The handlers can be set to any of the 2 mirrors as base, based on this the code base
// address is calculated. This ensures that jsr instructions will result in the same opcode, as it
// is based on the code base address.
func (ar *Arch6502) calculateCodeBaseAddress(dis arch.Disasm, resetHandler uint16) {
	cart := dis.Cart()
	halfPrg := len(cart.PRG) % 0x8000
	codeBaseAddress := uint16(0x8000 + halfPrg)
	vectorsStartAddress := uint16(m6502.InterruptVectorStartAddress)

	// fix up calculated code base address for half sized PRG ROMs that have a different
	// code base address configured in the assembler, like "M.U.S.C.L.E."
	if resetHandler < codeBaseAddress {
		codeBaseAddress = nes.CodeBaseAddress
		vectorsStartAddress -= uint16(halfPrg)
	}

	dis.SetCodeBaseAddress(codeBaseAddress)
	dis.SetVectorsStartAddress(vectorsStartAddress)
}
