package arch

// Mapper provides a mapper manager.
type Mapper interface {
	GetMappedBank(address uint16) MappedBank
	GetMappedBankIndex(address uint16) uint16
	// OffsetInfo returns the offset information for the given address.
	OffsetInfo(address uint16) *Offset
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
