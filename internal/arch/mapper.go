package arch

type Bank interface {
	AddVariableUsage(ref any)
}

type MappedBank interface {
	Bank() Bank
	OffsetInfo(address uint16) *Offset
}

type BankReference struct {
	Mapped  MappedBank
	Address uint16
	Index   uint16
}
