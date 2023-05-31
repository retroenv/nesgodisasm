package disasm

import (
	"testing"

	"github.com/retroenv/nesgodisasm/internal/disasmoptions"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/log"
)

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
	options := &disasmoptions.Options{
		Assembler: "ca65",
		Logger:    log.NewTestLogger(t),
	}

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			disasm, err := New(cart, options)
			assert.NoError(t, err)
			disasm.offsets = test.Input()
			disasm.changeOffsetRangeToCodeAsData(data, 0)

			for i := range test.Expected {
				assert.Equal(t, test.Expected[i], disasm.offsets[i].OpcodeBytes)
			}
		})
	}
}
