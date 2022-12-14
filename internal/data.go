package disasm

import (
	"github.com/retroenv/nesgodisasm/internal/program"
	. "github.com/retroenv/retrogolib/nes/addressing"
)

// changeOffsetRangeToData sets a range of code offsets to data types.
// It combines all data bytes that are not split by a label.
func (dis *Disasm) changeOffsetRangeToData(data []byte, index uint16) {
	for i := 0; i < len(data); i++ {
		offsetInfo := &dis.offsets[index+uint16(i)]

		noLabelOffsets := 1
		for j := i + 1; j < len(data); j++ {
			offsetInfoNext := &dis.offsets[index+uint16(j)]
			if offsetInfoNext.Label == "" {
				offsetInfoNext.OpcodeBytes = nil
				offsetInfoNext.SetType(program.CodeAsData | program.DataOffset)
				noLabelOffsets++
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
	for i, offsetInfo := range dis.offsets {
		if offsetInfo.IsType(program.CodeOffset) ||
			offsetInfo.IsType(program.DataOffset) ||
			offsetInfo.IsType(program.FunctionReference) {
			continue
		}

		address := uint16(i + CodeBaseAddress)
		b := dis.readMemory(address)
		dis.offsets[i].OpcodeBytes = []byte{b}
	}
}
