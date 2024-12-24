package mapper

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
)

const (
	singleBankName        = "CODE"
	multiBankNameTemplate = "PRG_BANK_%d"
)

var _ arch.MappedBank = mappedBank{}

type bank struct {
	prg []byte

	offsets []*arch.Offset
}

type mappedBank struct {
	bank      *bank
	id        int
	dataStart int
}

func newBank(prg []byte) *bank {
	b := &bank{
		prg:     prg,
		offsets: make([]*arch.Offset, len(prg)),
	}
	for i := range b.offsets {
		b.offsets[i] = &arch.Offset{}
	}
	return b
}

func (m *Mapper) initializeBanks(dis arch.Disasm, prg []byte) {
	for i := 0; i < len(prg); {
		size := len(prg) - i
		if size > 0x8000 {
			size = 0x8000
		}

		b := prg[i : i+size]
		bnk := newBank(b)
		m.banks = append(m.banks, bnk)
		i += size

		dis.Constants().AddBank()
		dis.Variables().AddBank()
	}
}

func (m mappedBank) OffsetInfo(index uint16) *arch.Offset {
	offset := int(index) + m.dataStart
	offsetInfo := m.bank.offsets[offset]
	return offsetInfo
}

func (m mappedBank) ID() int {
	return m.id
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
