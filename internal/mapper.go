package disasm

import "fmt"

var emptyMappedBank = mappedBank{}

type mappedBank struct {
	bank      *bank
	dataStart int
}

type mapper struct {
	addressShifts  int
	bankWindowSize int

	banks  []mappedBank
	mapped []mappedBank
}

const bankWindowSize = 0x2000 // TODO use as parameter

func newMapper(banks []*bank, prgSize int) (*mapper, error) {
	mappedBanks := prgSize / bankWindowSize
	mappedWindows := 0x10000 / bankWindowSize

	m := &mapper{
		addressShifts:  16 - log2(mappedWindows),
		bankWindowSize: bankWindowSize,

		banks:  make([]mappedBank, mappedBanks),
		mapped: make([]mappedBank, mappedWindows),
	}

	bankNumber := 0
	for _, bnk := range banks {
		if len(bnk.prg)%bankWindowSize != 0 {
			return nil, fmt.Errorf("invalid bank alignment for bank size %d", len(bnk.prg))
		}

		for pointer := 0; pointer < len(bnk.prg); pointer += bankWindowSize {
			mapped := mappedBank{
				bank:      bnk,
				dataStart: pointer,
			}
			m.banks[bankNumber] = mapped
			bankNumber++
		}
	}

	// TODO set mapper specific
	bnk := m.banks[0]
	m.setBank(0x8000, bnk)
	bnk = m.banks[1]
	m.setBank(0xa000, bnk)
	bnk = m.banks[len(m.banks)-2]
	m.setBank(0xc000, bnk)
	bnk = m.banks[len(m.banks)-1]
	m.setBank(0xe000, bnk)

	return m, nil
}

func (m *mapper) setBank(address uint16, bank mappedBank) {
	bankWindow := address >> m.addressShifts
	m.mapped[bankWindow] = bank
}

func (m *mapper) getMappedBank(address uint16) *bank {
	bankWindow := address >> m.addressShifts
	bnk := m.mapped[bankWindow]
	return bnk.bank
}

func (m *mapper) getMappedBankIndex(address uint16) uint16 {
	index := int(address) % bankWindowSize
	return uint16(index)
}

func (m *mapper) readMemory(address uint16) byte {
	bankWindow := address >> m.addressShifts
	bnk := m.mapped[bankWindow]
	index := int(address) % bankWindowSize
	pointer := bnk.dataStart + index
	b := bnk.bank.prg[pointer]
	return b
}

func (m *mapper) offsetInfo(address uint16) *offset {
	bankWindow := address >> m.addressShifts
	bnk := m.mapped[bankWindow]
	if bnk == emptyMappedBank {
		return nil
	}

	index := int(address) % bankWindowSize
	pointer := bnk.dataStart + index
	offsetInfo := &bnk.bank.offsets[pointer]
	return offsetInfo
}

// log2 computes the binary logarithm of x, rounded up to the next integer.
func log2(i int) int {
	var n, p int
	for p = 1; p < i; p += p {
		n++
	}
	return n
}
