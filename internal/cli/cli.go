// Package cli handles command line interface logic
package cli

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/retroenv/retrodisasm/internal/assembler"
	"github.com/retroenv/retrodisasm/internal/options"
)

// ParseFlags parses command line flags and returns program and disassembler options
func ParseFlags() (options.Program, options.Disassembler, error) {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	var opts options.Program
	var disasmOptions options.Disassembler

	readOptionFlags(flags, &opts)
	readDisasmOptionFlags(flags)

	err := flags.Parse(os.Args[1:])
	args := flags.Args()
	if err != nil || (len(args) == 0 && opts.Batch == "") {
		return opts, options.Disassembler{}, &UsageError{flags: flags}
	}

	if err := validateArgs(args); err != nil {
		return opts, options.Disassembler{}, err
	}

	if err := normalizeOptions(&opts); err != nil {
		return opts, options.Disassembler{}, err
	}

	if opts.Batch == "" {
		opts.Input = args[0]
	}

	// Update disasm options after parsing program options
	disasmOptions = createDisasmOptions(opts)
	applyDisasmOptionFlags(&disasmOptions)

	return opts, disasmOptions, nil
}

// UsageError represents an error that should show usage information
type UsageError struct {
	flags *flag.FlagSet
	msg   string
}

func (e *UsageError) Error() string {
	return e.msg
}

func (e *UsageError) ShowUsage() {
	fmt.Printf("usage: retrodisasm [options] <file to disassemble>\n\n")
	e.flags.PrintDefaults()
	fmt.Println()
}

// validateArgs checks if arguments are in correct order
func validateArgs(args []string) error {
	for i, arg := range args {
		if i > 0 && arg[0] == '-' {
			return &UsageError{
				msg: fmt.Sprintf("Potential argument %s found after file to disassemble, please pass the file to disassemble as last argument", arg),
			}
		}
	}
	return nil
}

// normalizeOptions normalizes and validates option values
func normalizeOptions(opts *options.Program) error {
	opts.Assembler = strings.ToLower(opts.Assembler)
	if opts.Assembler == "asm6f" {
		opts.Assembler = "asm6"
	}

	// Validate assembler type
	validAssemblers := []string{"asm6", "ca65", "nesasm", "retroasm"}
	if !slices.Contains(validAssemblers, opts.Assembler) {
		return fmt.Errorf("unsupported assembler: %s. Valid options: %s",
			opts.Assembler, strings.Join(validAssemblers, ", "))
	}

	return nil
}

// createDisasmOptions creates disassembler options based on program options
func createDisasmOptions(opts options.Program) options.Disassembler {
	disasmOptions := options.NewDisassembler(opts.Assembler, opts.System)

	// nesasm doesn't support unofficial instructions in output
	if opts.Assembler == assembler.Nesasm {
		disasmOptions.AssemblerSupportsUnofficial = false
	}

	return disasmOptions
}

func readOptionFlags(flags *flag.FlagSet, opts *options.Program) {
	flags.StringVar(&opts.Input, "i", "", "name of the input ROM file")
	flags.StringVar(&opts.Output, "o", "", "name of the output .asm file, printed on console if no name given")
	flags.StringVar(&opts.Assembler, "a", "ca65", "Assembler compatibility of the generated .asm file (asm6/ca65/nesasm/retroasm)")
	flags.StringVar(&opts.Config, "c", "", "Config file name to write output to for ca65 assembler")
	flags.StringVar(&opts.CodeDataLog, "cdl", "", "name of the .cdl Code/Data log file to load")
	flags.StringVar(&opts.Batch, "batch", "", "process a batch of given path and file mask and automatically .asm file naming, for example *.nes")
	flags.StringVar(&opts.System, "s", "", "system to disassemble for (nes, chip8) - if not auto-detected from file extension")
	flags.BoolVar(&opts.Binary, "binary", false, "read input file as raw binary file without any header")
	flags.BoolVar(&opts.Debug, "debug", false, "enable debugging options for extended logging")
	flags.BoolVar(&opts.Quiet, "q", false, "perform operations quietly")
	flags.BoolVar(&opts.AssembleTest, "verify", false, "verify the generated output by assembling with ca65 and check if it matches the input")
}

// disasmFlagVars holds temporary flag variables
type disasmFlagVars struct {
	noHexComments    bool
	noOffsets        bool
	stopAtUnofficial bool
	zeroBytes        bool
}

var disasmFlags disasmFlagVars

func readDisasmOptionFlags(flags *flag.FlagSet) {
	flags.BoolVar(&disasmFlags.noHexComments, "nohexcomments", false, "do not output opcode bytes as hex values in comments")
	flags.BoolVar(&disasmFlags.noOffsets, "nooffsets", false, "do not output offsets in comments")
	flags.BoolVar(&disasmFlags.stopAtUnofficial, "stop-at-unofficial", false, "stop tracing at unofficial opcodes unless explicitly branched to")
	flags.BoolVar(&disasmFlags.zeroBytes, "z", false, "output the trailing zero bytes of banks")
}

func applyDisasmOptionFlags(opts *options.Disassembler) {
	// Apply inverse logic for hex comments and offsets
	opts.HexComments = !disasmFlags.noHexComments
	opts.OffsetComments = !disasmFlags.noOffsets
	opts.StopAtUnofficial = disasmFlags.stopAtUnofficial
	opts.ZeroBytes = disasmFlags.zeroBytes
}
