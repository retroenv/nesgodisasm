package arch

type Mapper interface {
	GetMappedBank(address uint16) MappedBank
	GetMappedBankIndex(address uint16) uint16
}

type MappedBank interface {
	ID() int
	OffsetInfo(address uint16) *Offset
}

type BankReference struct {
	Mapped  MappedBank
	Address uint16
	ID      int
	Index   uint16 // index in the bank
}
