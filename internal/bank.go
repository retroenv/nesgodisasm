package disasm

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
)

const (
	singleBankName        = "CODE"
	multiBankNameTemplate = "PRG_BANK_%d"
)

type bank struct {
	prg []byte

	variables     map[uint16]*variable
	usedVariables map[uint16]struct{}

	offsets []*arch.Offset
}

func (b *bank) AddVariableUsage(ref any) {
	varInfo := ref.(*variable)
	b.variables[varInfo.address] = varInfo
	b.usedVariables[varInfo.address] = struct{}{}
}

func newBank(prg []byte) *bank {
	b := &bank{
		prg:           prg,
		variables:     map[uint16]*variable{},
		usedVariables: map[uint16]struct{}{},
		offsets:       make([]*arch.Offset, len(prg)),
	}
	for i := range b.offsets {
		b.offsets[i] = &arch.Offset{}
	}
	return b
}

func (dis *Disasm) initializeBanks(prg []byte) {
	for i := 0; i < len(prg); {
		size := len(prg) - i
		if size > 0x8000 {
			size = 0x8000
		}

		b := prg[i : i+size]
		bnk := newBank(b)
		dis.banks = append(dis.banks, bnk)
		i += size

		dis.constants.AddBank()
	}
}

func setBankVectors(bnk *bank, prgBank *program.PRGBank) {
	idx := len(bnk.prg) - 6
	for i := range 3 {
		b1 := bnk.prg[idx]
		idx++
		b2 := bnk.prg[idx]
		idx++
		addr := uint16(b2)<<8 | uint16(b1)
		prgBank.Vectors[i] = addr
	}
}

func setBankName(prgBank *program.PRGBank, bnkIndex, numBanks int) {
	if bnkIndex == 0 && numBanks == 1 {
		prgBank.Name = singleBankName
		return
	}

	prgBank.Name = fmt.Sprintf(multiBankNameTemplate, bnkIndex)
}
