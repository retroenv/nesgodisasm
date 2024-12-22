package disasm

import "github.com/retroenv/nesgodisasm/internal/arch"

const (
	singleBankName        = "CODE"
	multiBankNameTemplate = "PRG_BANK_%d"
)

type bank struct {
	prg []byte

	constants     map[uint16]arch.ConstTranslation
	usedConstants map[uint16]arch.ConstTranslation
	variables     map[uint16]*variable
	usedVariables map[uint16]struct{}

	offsets []offset
}

type bankReference struct {
	mapped  mappedBank
	address uint16
	index   uint16
}

func newBank(prg []byte) *bank {
	return &bank{
		prg:           prg,
		constants:     map[uint16]arch.ConstTranslation{},
		usedConstants: map[uint16]arch.ConstTranslation{},
		variables:     map[uint16]*variable{},
		usedVariables: map[uint16]struct{}{},
		offsets:       make([]offset, len(prg)),
	}
}
