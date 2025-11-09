package mapper

import (
	"github.com/retroenv/retrodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/system/nes/codedatalog"
)

// ApplyCodeDataLog applies code data log flags to mark code and entry points.
// It processes the CDL file data and marks bytes as code or data, and identifies
// subroutine entry points.
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
