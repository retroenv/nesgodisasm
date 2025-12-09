package cli

import (
	"flag"
	"os"
	"testing"

	"github.com/retroenv/retrodisasm/internal/options"
	"github.com/retroenv/retrogolib/assert"
)

func TestParseFlags_DisasmOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want options.Disassembler
	}{
		{
			name: "default flags",
			args: []string{"prog", "test.nes"},
			want: options.Disassembler{HexComments: true, OffsetComments: true},
		},
		{
			name: "nohexcomments flag",
			args: []string{"prog", "-nohexcomments", "test.nes"},
			want: options.Disassembler{OffsetComments: true},
		},
		{
			name: "nooffsets flag",
			args: []string{"prog", "-nooffsets", "test.nes"},
			want: options.Disassembler{HexComments: true},
		},
		{
			name: "z flag",
			args: []string{"prog", "-z", "test.nes"},
			want: options.Disassembler{HexComments: true, OffsetComments: true, ZeroBytes: true},
		},
		{
			name: "all disasm flags",
			args: []string{"prog", "-nohexcomments", "-nooffsets", "-z", "test.nes"},
			want: options.Disassembler{ZeroBytes: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disasmFlags = disasmFlagVars{}
			oldArgs := os.Args
			t.Cleanup(func() { os.Args = oldArgs })

			os.Args = tt.args

			_, got, err := ParseFlags()
			assert.NoError(t, err)
			assert.Equal(t, tt.want.HexComments, got.HexComments)
			assert.Equal(t, tt.want.OffsetComments, got.OffsetComments)
			assert.Equal(t, tt.want.ZeroBytes, got.ZeroBytes)
		})
	}
}

func TestReadDisasmOptionFlags(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	readDisasmOptionFlags(flags)

	assert.True(t, flags.Lookup("nohexcomments") != nil)
	assert.True(t, flags.Lookup("nooffsets") != nil)
	assert.True(t, flags.Lookup("z") != nil)
}

func TestApplyDisasmOptionFlags(t *testing.T) {
	tests := []struct {
		name string
		in   disasmFlagVars
		want options.Disassembler
	}{
		{
			name: "defaults",
			want: options.Disassembler{HexComments: true, OffsetComments: true},
		},
		{
			name: "nohexcomments",
			in:   disasmFlagVars{noHexComments: true},
			want: options.Disassembler{OffsetComments: true},
		},
		{
			name: "nooffsets",
			in:   disasmFlagVars{noOffsets: true},
			want: options.Disassembler{HexComments: true},
		},
		{
			name: "both disabled",
			in:   disasmFlagVars{noHexComments: true, noOffsets: true},
		},
		{
			name: "output unofficial as code",
			in:   disasmFlagVars{outputUnofficialAsCode: true},
			want: options.Disassembler{HexComments: true, OffsetComments: true, OutputUnofficialAsMnemonics: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disasmFlags = tt.in
			var got options.Disassembler

			applyDisasmOptionFlags(&got)

			assert.Equal(t, tt.want.HexComments, got.HexComments)
			assert.Equal(t, tt.want.OffsetComments, got.OffsetComments)
			assert.Equal(t, tt.want.OutputUnofficialAsMnemonics, got.OutputUnofficialAsMnemonics)
		})
	}
}

func TestValidateOptionCombinations(t *testing.T) {
	tests := []struct {
		name        string
		opts        options.Program
		disasmOpts  options.Disassembler
		expectError bool
	}{
		{
			name:        "no conflict",
			opts:        options.Program{},
			disasmOpts:  options.Disassembler{},
			expectError: false,
		},
		{
			name:        "verify only",
			opts:        options.Program{AssembleTest: true},
			disasmOpts:  options.Disassembler{},
			expectError: false,
		},
		{
			name:        "output unofficial only",
			opts:        options.Program{},
			disasmOpts:  options.Disassembler{OutputUnofficialAsMnemonics: true},
			expectError: false,
		},
		{
			name:        "verify and output unofficial conflict",
			opts:        options.Program{AssembleTest: true},
			disasmOpts:  options.Disassembler{OutputUnofficialAsMnemonics: true},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOptionCombinations(tt.opts, tt.disasmOpts)
			if tt.expectError {
				assert.True(t, err != nil)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
