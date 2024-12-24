// Package mapper provides a mapper manager.
package mapper

import (
	"fmt"
	"strings"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/nes/codedatalog"
)

type Mapper struct {
	banks []*bank

	addressShifts  int
	bankWindowSize int

	emptyMappedBank mappedBank
	banksMapped     []mappedBank
	mapped          []mappedBank
}

const bankWindowSize = 0x2000 // TODO use as parameter

// New creates a new mapper manager.
func New(dis arch.Disasm, prg []byte) (*Mapper, error) {
	prgSize := len(prg)
	mappedBanks := prgSize / bankWindowSize
	mappedWindows := 0x10000 / bankWindowSize

	m := &Mapper{
		addressShifts:  16 - log2(mappedWindows),
		bankWindowSize: bankWindowSize,

		emptyMappedBank: mappedBank{},
		banksMapped:     make([]mappedBank, mappedBanks),
		mapped:          make([]mappedBank, mappedWindows),
	}

	m.initializeBanks(dis, prg)

	bankNumber := 0
	for bankIndex, bnk := range m.banks {
		if len(bnk.prg)%bankWindowSize != 0 {
			return nil, fmt.Errorf("invalid bank alignment for bank size %d", len(bnk.prg))
		}

		for pointer := 0; pointer < len(bnk.prg); pointer += bankWindowSize {
			mapped := mappedBank{
				bank:      bnk,
				id:        bankIndex,
				dataStart: pointer,
			}
			m.banksMapped[bankNumber] = mapped
			bankNumber++
		}
	}

	// TODO set mapper specific
	bnk := m.banksMapped[0]
	m.setMappedBank(0x8000, bnk)
	bnk = m.banksMapped[1]
	m.setMappedBank(0xa000, bnk)
	bnk = m.banksMapped[len(m.banksMapped)-2]
	m.setMappedBank(0xc000, bnk)
	bnk = m.banksMapped[len(m.banksMapped)-1]
	m.setMappedBank(0xe000, bnk)

	return m, nil
}

func (m *Mapper) setMappedBank(address uint16, bank mappedBank) {
	bankWindow := address >> m.addressShifts
	m.mapped[bankWindow] = bank
}

func (m *Mapper) GetMappedBank(address uint16) arch.MappedBank {
	bankWindow := address >> m.addressShifts
	mapped := m.mapped[bankWindow]
	return mapped
}

func (m *Mapper) GetMappedBankIndex(address uint16) uint16 {
	index := int(address) % bankWindowSize
	return uint16(index)
}

func (m *Mapper) ReadMemory(address uint16) byte {
	bankWindow := address >> m.addressShifts
	bnk := m.mapped[bankWindow]
	index := int(address) % bankWindowSize
	pointer := bnk.dataStart + index
	b := bnk.bank.prg[pointer]
	return b
}

func (m *Mapper) OffsetInfo(address uint16) *arch.Offset {
	bankWindow := address >> m.addressShifts
	bnk := m.mapped[bankWindow]
	if bnk == m.emptyMappedBank {
		return nil
	}

	index := int(address) % bankWindowSize
	pointer := bnk.dataStart + index
	offsetInfo := bnk.bank.offsets[pointer]
	return offsetInfo
}

// ProcessData sets all data bytes for offsets that have not being identified as code.
func (m *Mapper) ProcessData() {
	for _, bnk := range m.banks {
		for i, offsetInfo := range bnk.offsets {
			if offsetInfo.IsType(program.CodeOffset) ||
				offsetInfo.IsType(program.DataOffset) ||
				offsetInfo.IsType(program.FunctionReference) {

				continue
			}

			bnk.offsets[i].Data = []byte{bnk.prg[i]}
		}
	}
}
func (m *Mapper) SetProgramBanks(dis arch.Disasm, app *program.Program) error {
	for bnkIndex, bnk := range m.banks {
		prgBank := program.NewPRGBank(len(bnk.offsets))

		for i := range len(bnk.offsets) {
			offsetInfo := bnk.offsets[i]
			programOffsetInfo, err := getProgramOffset(dis, dis.CodeBaseAddress()+uint16(i), offsetInfo)
			if err != nil {
				return err
			}

			prgBank.Offsets[i] = programOffsetInfo
		}

		dis.Constants().SetBankConstants(bnkIndex, prgBank)
		dis.Variables().SetBankVariables(bnkIndex, prgBank)

		setBankName(prgBank, bnkIndex, len(m.banks))
		setBankVectors(bnk, prgBank)

		app.PRG = append(app.PRG, prgBank)
	}
	return nil
}

func (m *Mapper) ApplyCodeDataLog(dis arch.Disasm, prgFlags []codedatalog.PrgFlag) {
	bank0 := m.banks[0]
	for index, flags := range prgFlags {
		if index > len(bank0.offsets) {
			return
		}

		if flags&codedatalog.Code != 0 {
			dis.AddAddressToParse(dis.CodeBaseAddress()+uint16(index), 0, 0, nil, false)
		}
		if flags&codedatalog.SubEntryPoint != 0 {
			bank0.offsets[index].SetType(program.CallDestination)
		}
	}
}

func getProgramOffset(dis arch.Disasm, address uint16, offsetInfo *arch.Offset) (program.Offset, error) {
	programOffset := offsetInfo.Offset
	programOffset.Address = address

	if offsetInfo.BranchingTo != "" {
		programOffset.Code = fmt.Sprintf("%s %s", offsetInfo.Code, offsetInfo.BranchingTo)
	}

	if offsetInfo.IsType(program.CodeOffset | program.CodeAsData | program.FunctionReference) {
		if len(programOffset.Data) == 0 && programOffset.Label == "" {
			return programOffset, nil
		}

		if offsetInfo.IsType(program.FunctionReference) {
			programOffset.Code = ".word " + offsetInfo.BranchingTo
		}

		if err := setComment(dis, address, &programOffset); err != nil {
			return program.Offset{}, err
		}
	} else {
		programOffset.SetType(program.DataOffset)
	}

	return programOffset, nil
}

func setComment(dis arch.Disasm, address uint16, programOffset *program.Offset) error {
	var comments []string

	opts := dis.Options()
	if opts.OffsetComments {
		programOffset.HasAddressComment = true
		comments = []string{fmt.Sprintf("$%04X", address)}
	}

	if opts.HexComments {
		hexCodeComment, err := hexCodeComment(programOffset)
		if err != nil {
			return err
		}
		comments = append(comments, hexCodeComment)
	}

	if programOffset.Comment != "" {
		comments = append(comments, programOffset.Comment)
	}
	programOffset.Comment = strings.Join(comments, "  ")
	return nil
}

func hexCodeComment(offset *program.Offset) (string, error) {
	buf := &strings.Builder{}

	for _, b := range offset.Data {
		if _, err := fmt.Fprintf(buf, "%02X ", b); err != nil {
			return "", fmt.Errorf("writing hex comment: %w", err)
		}
	}

	comment := strings.TrimRight(buf.String(), " ")
	return comment, nil
}

// log2 computes the binary logarithm of x, rounded up to the next integer.
func log2(i int) int {
	var n, p int
	for p = 1; p < i; p += p {
		n++
	}
	return n
}
