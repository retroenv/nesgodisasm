package disasm

type bank struct {
	prg []byte

	constants     map[uint16]constTranslation
	usedConstants map[uint16]constTranslation
	variables     map[uint16]*variable
	usedVariables map[uint16]struct{}

	offsets []offset
}

type bankReference struct {
	bank    *bank
	address uint16
	index   uint16
}

func newBank(prg []byte) *bank {
	return &bank{
		prg:           prg,
		constants:     map[uint16]constTranslation{},
		usedConstants: map[uint16]constTranslation{},
		variables:     map[uint16]*variable{},
		usedVariables: map[uint16]struct{}{},
		offsets:       make([]offset, len(prg)),
	}
}

func (b *bank) offsetInfo(index uint16) *offset {
	offsetInfo := &b.offsets[index]
	return offsetInfo
}
