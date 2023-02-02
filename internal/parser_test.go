package disasm

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
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

	for _, test := range tests {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			disasm := Disasm{}
			disasm.offsets = test.Input()
			disasm.changeOffsetRangeToCodeAsData(data, 0)

			for i := range test.Expected {
				assert.Equal(t, test.Expected[i], disasm.offsets[i].OpcodeBytes)
			}
		})
	}
}
