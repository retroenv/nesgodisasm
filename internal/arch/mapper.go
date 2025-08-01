package arch

// Mapper provides a mapper manager.
type Mapper interface {
	GetMappedBank(address uint16) MappedBank
	GetMappedBankIndex(address uint16) uint16
	// OffsetInfo returns the offset information for the given address.
	OffsetInfo(address uint16) *Offset
	// ReadMemory reads a byte from memory at the given address.
	ReadMemory(address uint16) byte
}

type MappedBank interface {
	ID() int
	OffsetInfo(address uint16) *Offset
}

// BankReference represents a reference to an address in a bank.
type BankReference struct {
	Mapped  MappedBank // bank reference
	Address uint16     // address in the bank
	ID      int        // bank ID
	Index   uint16     // index in the bank
}
