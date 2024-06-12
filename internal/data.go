package disasm

import (
	"github.com/retroenv/nesgodisasm/internal/program"
)

// changeAddressRangeToCodeAsData sets a range of code address to code as
// data types. It combines all data bytes that are not split by a label.
func (dis *Disasm) changeAddressRangeToCodeAsData(address uint16, data []byte) {
	for i := 0; i < len(data); i++ {
		offsetInfo := dis.mapper.offsetInfo(address + uint16(i))

		noLabelOffsets := 1
		for j := i + 1; j < len(data); j++ {
			offsetInfoNext := dis.mapper.offsetInfo(address + uint16(j))
			if offsetInfoNext.Label == "" {
				offsetInfoNext.OpcodeBytes = nil
				offsetInfoNext.SetType(program.CodeAsData | program.DataOffset)
				noLabelOffsets++

				skipAddressToParse := address + uint16(j)
				dis.offsetsParsed[skipAddressToParse] = struct{}{}
				continue
			}
			break
		}

		offsetInfo.OpcodeBytes = data[i : i+noLabelOffsets]
		offsetInfo.ClearType(program.CodeOffset)
		offsetInfo.SetType(program.CodeAsData | program.DataOffset)
		i += noLabelOffsets - 1
	}
}

// processData sets all data bytes for offsets that have not being identified as code.
func (dis *Disasm) processData() {
	for _, mappedBank := range dis.mapper.banks {
		bnk := mappedBank.bank

		for i, offsetInfo := range bnk.offsets {
			if offsetInfo.IsType(program.CodeOffset) ||
				offsetInfo.IsType(program.DataOffset) ||
				offsetInfo.IsType(program.FunctionReference) {

				continue
			}

			bnk.offsets[i].OpcodeBytes = []byte{bnk.prg[i]}
		}
	}
}
