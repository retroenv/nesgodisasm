package disasm

import (
	"testing"

	"github.com/retroenv/nesgodisasm/internal/assembler"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/log"
)

// nolint:funlen
func TestChangeOffsetRangeToData(t *testing.T) {
	t.Parallel()

	data := []byte{12, 34, 56}

	tests := []struct {
		Name     string
		Input    func() []offset
		Expected [][]byte
	}{
		{
			Name: "no label",
			Input: func() []offset {
				return make([]offset, 3)
			},
			Expected: [][]byte{{12, 34, 56}},
		},
		{
			Name: "1 label",
			Input: func() []offset {
				data := make([]offset, 3)
				data[1].Label = "label1"
				return data
			},
			Expected: [][]byte{{12}, {34, 56}},
		},
		{
			Name: "2 labels",
			Input: func() []offset {
				data := make([]offset, 3)
				data[1].Label = "label1"
				data[2].Label = "label2"
				return data
			},
			Expected: [][]byte{{12}, {34}, {56}},
		},
	}

	cart := cartridge.New()
	opts := &options.Disassembler{
		Assembler: assembler.Ca65,
	}
	logger := log.NewTestLogger(t)

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			disasm, err := New(logger, cart, opts)
			assert.NoError(t, err)
			input := test.Input()
			b := make([]byte, len(input))
			bank := newBank(b)
			bank.offsets = input
			disasm.changeAddressRangeToCodeAsData(0x8000, data)

			for i := range test.Expected {
				assert.Equal(t, test.Expected[i], bank.offsets[i].OpcodeBytes)
			}
		})
	}
}
