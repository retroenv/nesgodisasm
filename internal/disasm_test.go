package disasm

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/retroenv/nesgodisasm/internal/disasmoptions"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/nes/cartridge"
)

var testCodeDefault = []byte{
	0x78,             // sei
	0x4C, 0x04, 0x80, // jmp + 3
	0xAD, 0x30, 0x80, // lda a:$8030
	0xBD, 0x20, 0x80, // lda a:$8020,X
	0xea,       // nop
	0x90, 0x01, // bcc +1
	0xdc, 0xae, 0x8b, // nop $8BAE,X
	0x78, // sei
	0x40, // rti
}

var expectedDefault = `Reset:
  sei                            ; $8000 78
  jmp _label_8004                ; $8001 4C 04 80

_label_8004:
  lda a:_data_8030               ; $8004 AD 30 80
  lda a:_data_8020_indexed,X     ; $8007 BD 20 80
  nop                            ; $800A EA
  bcc _label_800e                ; $800B 90 01
.byte $dc                        ; $800D unofficial nop instruction: nop $8BAE,X DC

_label_800e:
  ldx a:$788B                    ; $800E AE 8B 78
  rti                            ; $8011 40

.byte $00, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00

_data_8020_indexed:
.byte $12, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00

_data_8030:
.byte $34
`

var testCodeNoHexNoAddress = []byte{
	0x78,             // sei
	0x4C, 0x05, 0x80, // jmp + 3
	0x1a, // nop
	0x40, // rti
}

var expectedNoOffsetNoHex = `Reset:
  sei
  jmp _label_8005

.byte $1a

_label_8005:
  rti
`

func testProgram(t *testing.T, options *disasmoptions.Options, cart *cartridge.Cartridge, code []byte) *Disasm {
	t.Helper()

	// point reset handler to offset 0 of PRG buffer, aka 0x8000 address
	cart.PRG[0x7FFD] = 0x80

	copy(cart.PRG, code)

	disasm, err := New(cart, options)
	assert.NoError(t, err)

	return disasm
}

func TestDisasm(t *testing.T) {
	tests := []struct {
		Name     string
		Setup    func(options *disasmoptions.Options, cart *cartridge.Cartridge)
		Input    []byte
		Expected string
	}{
		{
			Name: "default",
			Setup: func(options *disasmoptions.Options, cart *cartridge.Cartridge) {
				cart.PRG[0x0020] = 0x12
				cart.PRG[0x0030] = 0x34
			},
			Input:    testCodeDefault,
			Expected: expectedDefault,
		},
		{
			Name: "no hex no address",
			Setup: func(options *disasmoptions.Options, cart *cartridge.Cartridge) {
				options.OffsetComments = false
				options.HexComments = false
			},
			Input:    testCodeNoHexNoAddress,
			Expected: expectedNoOffsetNoHex,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			options := disasmoptions.New()
			options.CodeOnly = true
			options.Assembler = "ca65"

			cart := cartridge.New()
			test.Setup(&options, cart)

			disasm := testProgram(t, &options, cart, test.Input)

			var buffer bytes.Buffer
			writer := bufio.NewWriter(&buffer)

			err := disasm.Process(writer)
			assert.NoError(t, err)

			assert.NoError(t, writer.Flush())

			buf := buffer.String()
			assert.Equal(t, test.Expected, buf)
		})
	}
}
