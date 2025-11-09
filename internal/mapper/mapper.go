// Package mapper provides a mapper manager.
package mapper

import (
	"fmt"
	"strings"

	"github.com/retroenv/retrodisasm/internal/instruction"
	"github.com/retroenv/retrodisasm/internal/offset"
	"github.com/retroenv/retrodisasm/internal/options"
	"github.com/retroenv/retrodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/codedatalog"
)

// architecture defines the minimal interface needed from arch.Architecture
type architecture interface {
	// BankWindowSize returns the bank window size.
	BankWindowSize(cart *cartridge.Cartridge) int
}

// variableManager defines the minimal interface needed for variable management
type variableManager interface {
	// AddBank adds a new bank to the variable manager.
	AddBank()
	// SetBankVariables sets the used variables in the bank for outputting.
	SetBankVariables(bankID int, prgBank *program.PRGBank)
}

// constantManager defines the minimal interface needed for constant management
type constantManager interface {
	// AddBank adds a new bank to the constant manager.
	AddBank()
	// SetBankConstants sets the used constants in the bank for outputting.
	SetBankConstants(bankID int, prgBank *program.PRGBank)
}

// disasm defines the minimal interface needed from the disassembler.
type disasm interface {
	// AddAddressToParse adds an address to the list to be processed.
	AddAddressToParse(address, context, from uint16, currentInstruction instruction.Instruction, isABranchDestination bool)
	// Options returns the disassembler options.
	Options() options.Disassembler
}

// Dependencies contains the dependencies needed by Mapper.
type Dependencies struct {
	Disasm disasm
	Vars   variableManager
	Consts constantManager
}

type Mapper struct {
	banks []*bank

	addressShifts   int
	bankWindowSize  int
	codeBaseAddress uint16 // Code base address for single-bank systems

	banksMapped []mappedBank
	mapped      []mappedBank

	dis    disasm          // Reference to disasm for single-bank systems
	vars   variableManager // Reference to variable manager
	consts constantManager // Reference to constant manager
}

// New creates a new mapper manager.
func New(ar architecture, cart *cartridge.Cartridge) (*Mapper, error) {
	bankWindowSize := ar.BankWindowSize(cart)

	if bankWindowSize == 0 {
		return createSingleBankMapper(cart)
	}

	return createMultiBankMapper(cart, bankWindowSize)
}

// createSingleBankMapper creates a mapper for single bank systems (e.g., CHIP-8)
func createSingleBankMapper(cart *cartridge.Cartridge) (*Mapper, error) {
	bnk := newBank(cart.PRG)

	return &Mapper{
		banks: []*bank{bnk},
		mapped: []mappedBank{
			{bank: bnk},
		},
	}, nil
}

// createMultiBankMapper creates a mapper for multi-bank systems (e.g., NES)
func createMultiBankMapper(cart *cartridge.Cartridge, bankWindowSize int) (*Mapper, error) {
	prgSize := len(cart.PRG)
	mappedBanks := prgSize / bankWindowSize
	mappedWindows := 0x10000 / bankWindowSize

	m := &Mapper{
		addressShifts:  16 - log2(mappedWindows),
		bankWindowSize: bankWindowSize,
		banksMapped:    make([]mappedBank, mappedBanks),
		mapped:         make([]mappedBank, mappedWindows),
	}

	m.initializeBanks(cart.PRG)

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

// InjectDependencies sets the required dependencies for this mapper.
func (m *Mapper) InjectDependencies(deps Dependencies) {
	m.dis = deps.Disasm
	m.vars = deps.Vars
	m.consts = deps.Consts
}

// InitializeDependencyBanks initializes the bank structures in the injected
// vars and consts managers to match this mapper's bank configuration.
// This must be called after InjectDependencies.
func (m *Mapper) InitializeDependencyBanks() {
	for range m.banks {
		m.vars.AddBank()
		m.consts.AddBank()
	}
}

// SetCodeBaseAddress sets the code base address for single-bank systems.
func (m *Mapper) SetCodeBaseAddress(address uint16) {
	m.codeBaseAddress = address
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

func (m *Mapper) MappedBank(address uint16) offset.MappedBank {
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

func (m *Mapper) MappedBankIndex(address uint16) uint16 {
	var index int
	if m.bankWindowSize == 0 {
		// Single bank system (e.g., CHIP-8) - subtract code base address to get ROM offset
		index = int(address) - int(m.codeBaseAddress)
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
		index = int(address) - int(m.codeBaseAddress)
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

func (m *Mapper) OffsetInfo(address uint16) *offset.DisasmOffset {
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
		index = int(address) - int(m.codeBaseAddress)
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
func (m *Mapper) SetProgramBanks(app *program.Program) error {
	for bnkIndex, bnk := range m.banks {
		prgBank := program.NewPRGBank(len(bnk.offsets))

		for i := range len(bnk.offsets) {
			offsetInfo := bnk.offsets[i]
			programOffsetInfo, err := getProgramOffset(m, m.codeBaseAddress+uint16(i), offsetInfo)
			if err != nil {
				return err
			}

			prgBank.Offsets[i] = programOffsetInfo
		}

		m.consts.SetBankConstants(bnkIndex, prgBank)
		m.vars.SetBankVariables(bnkIndex, prgBank)

		setBankName(prgBank, bnkIndex, len(m.banks))
		setBankVectors(bnk, prgBank)

		app.PRG = append(app.PRG, prgBank)
	}
	return nil
}

func (m *Mapper) ApplyCodeDataLog(prgFlags []codedatalog.PrgFlag) {
	bank0 := m.banks[0]
	for index, flags := range prgFlags {
		if index > len(bank0.offsets) {
			return
		}

		if flags&codedatalog.Code != 0 {
			m.dis.AddAddressToParse(m.codeBaseAddress+uint16(index), 0, 0, nil, false)
		}
		if flags&codedatalog.SubEntryPoint != 0 {
			bank0.offsets[index].SetType(program.CallDestination)
		}
	}
}

func getProgramOffset(m *Mapper, address uint16, offsetInfo *offset.DisasmOffset) (program.Offset, error) {
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

		if err := setComment(m, address, &programOffset); err != nil {
			return program.Offset{}, err
		}
	} else {
		programOffset.SetType(program.DataOffset)
	}

	return programOffset, nil
}

func setComment(m *Mapper, address uint16, programOffset *program.Offset) error {
	var comments []string

	opts := m.dis.Options()
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
