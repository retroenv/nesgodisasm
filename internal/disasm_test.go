package disasm

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/log"
)

func TestDisasmZeroDataReference(t *testing.T) {
	input := []byte{
		0xad, 0x20, 0x80, // lda a:$8020
		0xbd, 0x10, 0x80, // lda a:$8010,X
		0x04, 0xa9, // nop $A9
		0x40, // rti
	}

	expected := `Reset:
        lda a:_data_8020               ; $8000  AD 20 80
        lda a:_data_8010_indexed,X     ; $8003  BD 10 80
        .byte $04, $a9                   ; $8006  04 A9  disambiguous instruction: nop $A9
        rti                            ; $8008  40
        
        .byte $00, $00, $00, $00, $00, $00, $00
        
        _data_8010_indexed:
        .byte $12, $00, $00, $00, $00, $34, $00, $00, $00, $00, $00, $00, $00, $00, $00, $00
        
        _data_8020:
        .byte $00
`

	setup := func(options *options.Disassembler, cart *cartridge.Cartridge) {
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
        .byte $dc                        ; disambiguous instruction: nop $8BAE,X
        
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
        .byte $82, $04                   ; disambiguous instruction: nop #$04
        
        _label_8007:
        rti
`

	runDisasm(t, nil, input, expected)
}

func TestDisasmJumpEngineTableFromCaller(t *testing.T) {
	input := []byte{
		0x20, 0x05, 0x80, // jsr $8005
		0x1a, 0x80, // .word 801a
		0x0a,       // 8005: asl a
		0xa8,       // tay
		0x68,       // pla
		0x85, 0x04, // sta $04
		0x68,       // pla
		0x85, 0x05, // sta $05
		0xc8,       // iny
		0xb1, 0x04, // lda $04,Y
		0x85, 0x06, // sta $06
		0xc8,       // iny
		0xb1, 0x04, // lda $04,Y
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
		0xa5, 0xd7, // lda z:$D7
		0x0a,             // asl a
		0xaa,             // tax
		0xbd, 0x15, 0x80, // lda a:$8015,X
		0x8d, 0x00, 0x02, // sta a:$0200
		0xbd, 0x16, 0x80, // lda a:$8016,X
		0x8d, 0x01, 0x02, // sta a:$0201
		0x6c, 0x00, 0x02, // jmp ($0200)
		0x00, 0x00,
		0x17, 0x80, // .word $8017
		0x40, // rti
	}

	expected := `
        _var_0200 = $0200
        
        Reset:                           ; jump engine detected
        lda z:$D7
        asl a
        tax
        lda a:_jump_table_8015,X
        sta a:_var_0200
        lda a:_jump_table_8015+1,X
        sta a:$0201
        jmp (_var_0200)
        
        .byte $00, $00
        
        _jump_table_8015:
        .word _label_8017
        
        _label_8017:
        rti
`

	runDisasm(t, nil, input, expected)
}

// TODO detect jump engine in generated code
func TestDisasmJumpEngineZeroPage(t *testing.T) {
	input := []byte{
		0xbd, 0x15, 0x80, // lda a:$8015,X
		0x85, 0xe4, // sta z:$e4
		0xbd, 0x16, 0x80, // lda a:$8016,X
		0x85, 0xe5, // sta z:$e5
		0xa9, 0x4c, // lda #$4c
		0x85, 0xe3, // sta z:$e3
		0x20, 0xe3, 0x00, // jsr $00e3
		0x60, // rts
		0x00, 0x00, 0x00,
		0x17, 0x80, // .word $8017
		0x60, // rts
	}

	expected := `
        _var_00e3 = $00E3
        
        Reset:
        lda a:_data_8015_indexed,X
        sta z:$E4
        lda a:_data_8016_indexed,X
        sta z:$E5
        lda #$4C
        sta z:_var_00e3
        jsr a:_var_00e3
        rts
        
        .byte $00, $00, $00
        
        _data_8015_indexed:
        .byte $17
        
        _data_8016_indexed:
        .byte $80, $60
`

	runDisasm(t, nil, input, expected)
}

func TestDisasmMixedAccess(t *testing.T) {
	input := []byte{
		0x85, 0x04, // sta $04
		0xb1, 0x04, // lda $04,Y
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

func TestDisasmDisambiguousInstructions(t *testing.T) {
	input := []byte{
		0x4c, 0x05, 0x80, // jmp $8005
		0x04, 0xa9, // nop $A9
		0xea,       // nop
		0x30, 0xFB, // bmi $03
		0x30, 0xFA, // bmi $04
		0x40, // rti
	}

	expected := `Reset:
        jmp _label_8005
        
        _label_8003:
        .byte $04                        ; branch into instruction detected: disambiguous instruction: nop $A9
        
        _label_8004:
        .byte $a9
        
        _label_8005:
        nop
        bmi _label_8003
        bmi _label_8004
        rti
`

	runDisasm(t, nil, input, expected)
}

func TestDisasmDifferentCodeBaseAddress(t *testing.T) {
	input := []byte{
		0x20, 0x68, 0xa2, // jsr a268
		0xb9, 0xfe, 0xbf, // lda a:$bffe,Y
		0x40, // rti
	}

	expected := `
        _var_bffe_indexed = $BFFE
        
        Reset:
        jsr $A268                      ; $C000  20 68 A2
        lda a:_var_bffe_indexed,Y      ; $C003  B9 FE BF
        rti                            ; $C006  40
`

	setup := func(options *options.Disassembler, cart *cartridge.Cartridge) {
		cart.PRG = make([]byte, 0x4000)
		cart.PRG[0x3FFD] = 0xC0 // reset handler that forces base address to $C000
	}
	runDisasm(t, setup, input, expected)
}

func TestDisasmIndirectJmp(t *testing.T) {
	input := []byte{
		0x6c, 0xce, 0x20, // jmp ($20CE)
	}

	expected := `Reset:                           ; jump engine detected
        jmp ($20CE)                    ; $8000  6C CE 20
`

	setup := func(options *options.Disassembler, cart *cartridge.Cartridge) {}

	runDisasm(t, setup, input, expected)
}

func testProgram(t *testing.T, options *options.Disassembler, cart *cartridge.Cartridge, code []byte) *Disasm {
	t.Helper()

	if len(cart.PRG) == 0x8000 {
		// point reset handler to offset 0 of PRG buffer, aka 0x8000 address
		cart.PRG[0x7FFD] = 0x80
	}

	copy(cart.PRG, code)

	logger := log.NewTestLogger(t)
	disasm, err := New(logger, cart, options)
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

func runDisasm(t *testing.T, setup func(options *options.Disassembler, cart *cartridge.Cartridge), input []byte, expected string) {
	t.Helper()

	opts := options.NewDisassembler("ca65")
	opts.CodeOnly = true

	cart := cartridge.New()

	if setup != nil {
		setup(&opts, cart)
	} else {
		opts.OffsetComments = false
		opts.HexComments = false
	}

	disasm := testProgram(t, &opts, cart, input)

	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)

	err := disasm.Process(writer)
	assert.NoError(t, err)

	assert.NoError(t, writer.Flush())

	buf := trimStringList(buffer.String())
	expected = trimStringList(expected)
	assert.Equal(t, expected, buf)
}
