package disasm

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/retroenv/nesgodisasm/internal/disasmoptions"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/nes/cartridge"
)

func TestDisasmCodeExample(t *testing.T) {
	input := []byte{
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

	expected := `Reset:
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

	setup := func(options *disasmoptions.Options, cart *cartridge.Cartridge) {
		cart.PRG[0x0020] = 0x12
		cart.PRG[0x0030] = 0x34
	}
	runDisasm(t, setup, input, expected)
}

func TestDisasmReferencingUnofficialInstruction(t *testing.T) {
	input := []byte{
		0xbd, 0x06, 0x80, // $8000 lda a:_data_8005_indexed+1,X
		0x90, 0x02, // $8003 bcc _label_8007
		0x82, 0x04, // $8005 unofficial nop instruction: nop #$04
		0x40, // $8007 rti
	}

	expected := `Reset:
  lda a:_data_8005_indexed+1,X
  bcc _label_8007

_data_8005_indexed:
.byte $82, $04                   ; unofficial nop instruction: nop #$04

_label_8007:
  rti
`

	runDisasm(t, nil, input, expected)
}

func TestDisasmJumpEngine(t *testing.T) {
	input := []byte{
		0x20, 0x05, 0x80, // // $8000 jsr $8005
		0x1a, 0x80, // $8005 dw $8005
		0x0A,       // $8005 asl a
		0xA8,       // $8007 tay
		0x68,       // $8007 pla
		0x85, 0x04, // $8007 sta $04
		0x68,       // $8007 pla
		0x85, 0x05, // $8007 sta $05
		0xC8,       // $8007 iny
		0xB1, 0x04, // $8007 lda (_var_0004_indexed),Y
		0x85, 0x06, // $8007 sta $06
		0xC8,       // $8007 iny
		0xB1, 0x04, // $8007 lda (_var_0004_indexed),Y
		0x85, 0x07, // $8007 sta $07
		0x6C, 0x06, 0x00, // $8007 jmp ($0006)
		0x40, // $8007 rti
	}

	expected := `Reset:
  lda a:_data_8005_indexed+1,X
  bcc _label_8007

_data_8005_indexed:
.byte $82, $04                   ; unofficial nop instruction: nop #$04

_label_8007:
  rti
`

	runDisasm(t, nil, input, expected)
}

func testProgram(t *testing.T, options *disasmoptions.Options, cart *cartridge.Cartridge, code []byte) *Disasm {
	t.Helper()

	// point reset handler to offset 0 of PRG buffer, aka 0x8000 address
	cart.PRG[0x7FFD] = 0x80

	copy(cart.PRG, code)

	disasm, err := New(cart, options)
	assert.NoError(t, err)

	return disasm
}

func runDisasm(t *testing.T, setup func(options *disasmoptions.Options, cart *cartridge.Cartridge), input []byte, expected string) {
	t.Helper()

	options := disasmoptions.New()
	options.CodeOnly = true
	options.Assembler = "ca65"

	cart := cartridge.New()

	if setup != nil {
		setup(&options, cart)
	} else {
		options.OffsetComments = false
		options.HexComments = false
	}

	disasm := testProgram(t, &options, cart, input)

	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)

	err := disasm.Process(writer)
	assert.NoError(t, err)

	assert.NoError(t, writer.Flush())

	buf := buffer.String()
	assert.Equal(t, expected, buf)
}
