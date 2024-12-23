package disasm

import (
	"testing"

	"github.com/retroenv/nesgodisasm/internal/arch"
	"github.com/retroenv/nesgodisasm/internal/arch/m6502"
	"github.com/retroenv/nesgodisasm/internal/assembler"
	"github.com/retroenv/nesgodisasm/internal/assembler/ca65"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/nes/parameter"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/log"
)

// nolint:funlen
func TestChangeOffsetRangeToData(t *testing.T) {
	t.Parallel()

	data := []byte{12, 34, 56}

	tests := []struct {
		Name     string
		Input    func(offsets []*arch.Offset)
		Expected [][]byte
	}{
		{
			Name:     "no label",
			Input:    func(offsets []*arch.Offset) {},
			Expected: [][]byte{{12, 34, 56}},
		},
		{
			Name: "1 label",
			Input: func(offsets []*arch.Offset) {
				offsets[1].Label = "label1"
			},
			Expected: [][]byte{{12}, {34, 56}},
		},
		{
			Name: "2 labels",
			Input: func(offsets []*arch.Offset) {
				offsets[1].Label = "label1"
				offsets[2].Label = "label2"
			},
			Expected: [][]byte{{12}, {34}, {56}},
		},
	}

	cart := cartridge.New()
	opts := options.Disassembler{
		Assembler: assembler.Ca65,
	}
	logger := log.NewTestLogger(t)

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			ar := m6502.New(parameter.New(ca65.ParamConfig))
			disasm, err := New(ar, logger, cart, opts, ca65.New)
			assert.NoError(t, err)

			offsets := make([]*arch.Offset, 3)
			for i := range offsets {
				offsets[i] = &arch.Offset{}
			}
			test.Input(offsets)

			m := disasm.mapper.getMappedBank(0x8000)
			mapped := m.(mappedBank)
			mapped.bank.offsets = offsets
			disasm.ChangeAddressRangeToCodeAsData(0x8000, data)

			for i := range test.Expected {
				assert.Equal(t, test.Expected[i], mapped.bank.offsets[i].Data)
			}
		})
	}
}
