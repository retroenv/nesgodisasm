// Package mapper provides a mapper manager.
package mapper

import (
	"fmt"
	"strings"

	"github.com/retroenv/retrodisasm/internal/arch"
	"github.com/retroenv/retrodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/codedatalog"
)

type Mapper struct {
	banks []*bank

	addressShifts  int
	bankWindowSize int

	banksMapped []mappedBank
	mapped      []mappedBank

	dis arch.Disasm // Reference to disasm for single-bank systems
}

// New creates a new mapper manager.
func New(ar arch.Architecture, dis arch.Disasm, cart *cartridge.Cartridge) (*Mapper, error) {
	bankWindowSize := ar.BankWindowSize(cart)

	if bankWindowSize == 0 {
		return createSingleBankMapper(cart, dis)
	}

	return createMultiBankMapper(cart, dis, bankWindowSize)
}

// createSingleBankMapper creates a mapper for single bank systems (e.g., CHIP-8)
func createSingleBankMapper(cart *cartridge.Cartridge, dis arch.Disasm) (*Mapper, error) {
	bnk := newBank(cart.PRG)
	dis.Constants().AddBank()
	dis.Variables().AddBank()

	return &Mapper{
		banks: []*bank{bnk},
		mapped: []mappedBank{
			{bank: bnk},
		},
		dis: dis,
	}, nil
}

// createMultiBankMapper creates a mapper for multi-bank systems (e.g., NES)
func createMultiBankMapper(cart *cartridge.Cartridge, dis arch.Disasm, bankWindowSize int) (*Mapper, error) {
	prgSize := len(cart.PRG)
	mappedBanks := prgSize / bankWindowSize
	mappedWindows := 0x10000 / bankWindowSize

	m := &Mapper{
		addressShifts:  16 - log2(mappedWindows),
		bankWindowSize: bankWindowSize,
		banksMapped:    make([]mappedBank, mappedBanks),
		mapped:         make([]mappedBank, mappedWindows),
	}

	m.initializeBanks(dis, cart.PRG)

	if err := m.populateBankMappings(bankWindowSize); err != nil {
		return nil, err
	}

	m.configureDefaultBankMapping()

	return m, nil
}

// populateBankMappings creates the bank mappings for multi-bank systems
func (m *Mapper) populateBankMappings(bankWindowSize int) error {
	bankNumber := 0
	for bankIndex, bnk := range m.banks {
		if len(bnk.prg)%bankWindowSize != 0 {
			return fmt.Errorf("invalid bank alignment for bank size %d", len(bnk.prg))
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
	return nil
}

// configureDefaultBankMapping sets up default bank mappings for NES systems
func (m *Mapper) configureDefaultBankMapping() {
	if m.bankWindowSize == 0x2000 {
		m.setMappedBank(0x8000, m.banksMapped[0])
		m.setMappedBank(0xa000, m.banksMapped[1])
		m.setMappedBank(0xc000, m.banksMapped[len(m.banksMapped)-2])
		m.setMappedBank(0xe000, m.banksMapped[len(m.banksMapped)-1])
	}
}

func (m *Mapper) setMappedBank(address uint16, bank mappedBank) {
	var bankWindow uint16
	if m.bankWindowSize == 0 {
		// Single bank system (e.g., CHIP-8)
		bankWindow = 0
	} else {
		// Multi-bank system
		bankWindow = address >> m.addressShifts
	}
	m.mapped[bankWindow] = bank
}

func (m *Mapper) GetMappedBank(address uint16) arch.MappedBank {
	var bankWindow uint16
	if m.bankWindowSize == 0 {
		// Single bank system (e.g., CHIP-8)
		bankWindow = 0
	} else {
		// Multi-bank system
		bankWindow = address >> m.addressShifts
	}
	mapped := m.mapped[bankWindow]
	return mapped
}

func (m *Mapper) GetMappedBankIndex(address uint16) uint16 {
	var index int
	if m.bankWindowSize == 0 {
		// Single bank system (e.g., CHIP-8) - subtract code base address to get ROM offset
		index = int(address) - int(m.dis.CodeBaseAddress())
	} else {
		// Multi-bank system - use modulo for bank window
		index = int(address) % m.bankWindowSize
	}
	return uint16(index)
}

func (m *Mapper) ReadMemory(address uint16) byte {
	var bankWindow uint16
	var index int

	if m.bankWindowSize == 0 {
		// Single bank system (e.g., CHIP-8) - subtract code base address
		bankWindow = 0
		index = int(address) - int(m.dis.CodeBaseAddress())
	} else {
		// Multi-bank system - calculate bank window and index
		bankWindow = address >> m.addressShifts
		index = int(address) % m.bankWindowSize
	}

	bnk := m.mapped[bankWindow]
	pointer := bnk.dataStart + index
	b := bnk.bank.prg[pointer]
	return b
}

func (m *Mapper) OffsetInfo(address uint16) *arch.Offset {
	var bankWindow uint16
	if m.bankWindowSize == 0 {
		// Single bank system (e.g., CHIP-8)
		bankWindow = 0
	} else {
		// Multi-bank system
		bankWindow = address >> m.addressShifts
	}
	bnk := m.mapped[bankWindow]
	if bnk.bank == nil {
		return nil
	}

	var index int
	if m.bankWindowSize > 0 {
		// Multi-bank: use modulo to convert memory address to bank offset
		index = int(address) % m.bankWindowSize
	} else {
		// Single-bank: subtract code base address to convert memory address to ROM offset
		index = int(address) - int(m.dis.CodeBaseAddress())
	}
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
