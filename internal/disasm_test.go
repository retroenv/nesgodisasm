package disasm

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/retroenv/nesgodisasm/internal/disasmoptions"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/nes/cartridge"
)

func TestDisasmZeroDataReference(t *testing.T) {
	input := []byte{
		0xAD, 0x20, 0x80, // lda a:$8020
		0xBD, 0x10, 0x80, // lda a:$8010,X
		0x40, // rti
	}

	expected := `Reset:
        lda a:_data_8020               ; $8000 AD 20 80
        lda a:_data_8010_indexed,X     ; $8003 BD 10 80
        rti                            ; $8006 40
        
        .byte $00, $00, $00, $00, $00, $00, $00, $00, $00
        
        _data_8010_indexed:
        .byte $12, $00, $00, $00, $00, $34, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00
        
        _data_8020:
        .byte $00
`

	setup := func(options *disasmoptions.Options, cart *cartridge.Cartridge) {
		cart.PRG[0x0010] = 0x12
		cart.PRG[0x0015] = 0x34
	}
	runDisasm(t, setup, input, expected)
}

func TestDisasmBranchIntoUnofficialNop(t *testing.T) {
	input := []byte{
		0x90, 0x01, // bcc +1
		0xdc, 0xae, 0x8b, // nop $8BAE,X
		0x78, // sei
		0x40, // rti
	}

	expected := `Reset:
        bcc _label_8003
        .byte $dc                        ; unofficial nop instruction: nop $8BAE,X
        
        _label_8003:
        ldx a:$788B                    ; branch into instruction detected
        rti
`

	runDisasm(t, nil, input, expected)
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

func TestDisasmJumpEngineTableFromCaller(t *testing.T) {
	input := []byte{
		0x20, 0x05, 0x80, // jsr $8005
		0x1a, 0x80, // .word 801a
		0x0A,       // 8005: asl a
		0xA8,       // tay
		0x68,       // pla
		0x85, 0x04, // sta $04
		0x68,       // pla
		0x85, 0x05, // sta $05
		0xC8,       // iny
		0xB1, 0x04, // lda $04,Y
		0x85, 0x06, // sta $06
		0xC8,       // iny
		0xB1, 0x04, // lda $04,Y
		0x85, 0x07, // sta $07
		0x6C, 0x06, 0x00, // jmp ($0006)
		0x40, // 801a: rti
	}

	expected := `
        _var_0004_indexed = $0004
        _var_0006 = $0006
        
        Reset:
        jsr _jump_engine_8005
        
        .word _label_801a
        
        _jump_engine_8005:               ; jump engine detected
        asl a
        tay
        pla
        sta z:_var_0004_indexed
        pla
        sta z:$05
        iny
        lda (_var_0004_indexed),Y
        sta z:_var_0006
        iny
        lda (_var_0004_indexed),Y
        sta z:$07
        jmp (_var_0006)
        
        _label_801a:
        rti
`

	runDisasm(t, nil, input, expected)
}

func TestDisasmJumpEngineTableAppended(t *testing.T) {
	input := []byte{
		0xa5, 0xd7, //
		0x0a,             //
		0xaa,             //
		0xbd, 0x15, 0x80, //
		0x8d, 0x00, 0x02, //
		0xbd, 0x16, 0x80, //
		0x8d, 0x01, 0x02, //
		0x6c, 0x00, 0x02, //
		0x00, 0x00, //
		0x12, 0x80, //
		0x40, //
	}

	expected := `
        _var_0200 = $0200
        
        Reset:                           ; jump engine detected
        lda z:$D7
        asl a
        tax
        lda a:_data_8015_indexed,X
        sta a:_var_0200
        lda a:_data_8016_indexed,X
        sta a:$0201
        jmp (_var_0200)
        
        .byte $00, $00
        
        _data_8015_indexed:
        .byte $12
        
        _data_8016_indexed:
        .byte $80, $40
`

	runDisasm(t, nil, input, expected)
}

func TestDisasmMixedAccess(t *testing.T) {
	input := []byte{
		0x85, 0x04, // sta $04
		0xB1, 0x04, // lda $04,Y
		0x40, // rti
	}

	expected := `
        _var_0004_indexed = $0004
        
        Reset:
        sta z:_var_0004_indexed
        lda (_var_0004_indexed),Y
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

func trimStringList(s string) string {
	sl := strings.Split(s, "\n")
	for i, s := range sl {
		sl[i] = strings.TrimSpace(s)
	}
	s = strings.Join(sl, "\n")
	return s
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

	buf := trimStringList(buffer.String())
	expected = trimStringList(expected)
	assert.Equal(t, expected, buf)
}
