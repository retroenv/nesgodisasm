package mocks

import (
	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/program"
)

// Mapper is a minimal mock implementation of arch.Mapper for testing.
type Mapper struct {
	offsets map[uint16]*arch.Offset
}

// NewMapper creates a new mock Mapper.
func NewMapper() *Mapper {
	return &Mapper{
		offsets: make(map[uint16]*arch.Offset),
	}
}

func (m *Mapper) GetMappedBank(uint16) arch.MappedBank {
	return nil
}

func (m *Mapper) GetMappedBankIndex(uint16) uint16 {
	return 0
}

func (m *Mapper) ReadMemory(address uint16) byte {
	return 0
}

func (m *Mapper) OffsetInfo(address uint16) *arch.Offset {
	if offset, ok := m.offsets[address]; ok {
		return offset
	}
	offset := &arch.Offset{
		Offset: program.Offset{
			Type: program.DataOffset,
			Data: []byte{0},
		},
	}
	m.offsets[address] = offset
	return offset
}

// SetOffsetInfo sets the offset info for the given address (for testing purposes).
func (m *Mapper) SetOffsetInfo(address uint16, offset *arch.Offset) {
	m.offsets[address] = offset
}
