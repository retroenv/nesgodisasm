package disasm

import (
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/retrogolib/arch/nes"
)

// changeOffsetRangeToCodeAsData sets a range of code offsets to code as
// data types. It combines all data bytes that are not split by a label.
func (dis *Disasm) changeOffsetRangeToCodeAsData(bnk *bank, data []byte, index uint16) {
	for i := 0; i < len(data); i++ {
		offsetInfo := &bnk.offsets[index+uint16(i)]

		noLabelOffsets := 1
		for j := i + 1; j < len(data); j++ {
			offsetInfoNext := &bnk.offsets[index+uint16(j)]
			if offsetInfoNext.Label == "" {
				offsetInfoNext.OpcodeBytes = nil
				offsetInfoNext.SetType(program.CodeAsData | program.DataOffset)
				noLabelOffsets++

				skipAddressToParse := dis.codeBaseAddress + index + uint16(j)
				bnk.offsetsParsed[skipAddressToParse] = struct{}{}
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
func (dis *Disasm) processData(bnk *bank) {
	for i, offsetInfo := range bnk.offsets {
		if offsetInfo.IsType(program.CodeOffset) ||
			offsetInfo.IsType(program.DataOffset) ||
			offsetInfo.IsType(program.FunctionReference) {
			continue
		}

		address := uint16(i + nes.CodeBaseAddress)
		b := dis.readMemory(address)
		bnk.offsets[i].OpcodeBytes = []byte{b}
	}
}
