// Package main implements a NES ROM disassembler
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	disasm "github.com/retroenv/nesgodisasm/internal"
	"github.com/retroenv/nesgodisasm/internal/ca65"
	"github.com/retroenv/nesgodisasm/internal/disasmoptions"
	"github.com/retroenv/retrogolib/buildinfo"
	"github.com/retroenv/retrogolib/nes/cartridge"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

type optionFlags struct {
	batch       string
	input       string
	output      string
	codeDataLog string

	assembleTest bool
	debug        bool
	quiet        bool

	noHexComments bool
	noOffsets     bool
}

func main() {
	options, disasmOptions := readArguments()

	if !options.quiet {
		printBanner(options)
	}

	files, err := getFiles(options)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	for _, file := range files {
		options.input = file
		if len(files) > 1 || options.output == "" {
			// create output file name by replacing file extension with .asm
			options.output = file[:len(file)-len(filepath.Ext(file))] + ".asm"
		}

		if err := disasmFile(options, disasmOptions); err != nil {
			fmt.Println(fmt.Errorf("disassembling failed: %w", err))
			os.Exit(1)
		}
	}
	fmt.Println()
}

func readArguments() (*optionFlags, *disasmoptions.Options) {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	options := &optionFlags{}
	disasmOptions := disasmoptions.New()
	disasmOptions.Assembler = "ca65"

	flags.StringVar(&options.batch, "batch", "", "process a batch of given path and file mask and automatically .asm file naming, for example *.nes")
	flags.BoolVar(&options.debug, "debug", false, "enable debugging options")
	flags.StringVar(&options.codeDataLog, "cdl", "", "name of the .cdl Code/Data log file to load")
	flags.BoolVar(&options.noHexComments, "nohexcomments", false, "do not output opcode bytes as hex values in comments")
	flags.BoolVar(&options.noOffsets, "nooffsets", false, "do not output offsets in comments")
	flags.StringVar(&options.output, "o", "", "name of the output .asm file, printed on console if no name given")
	flags.BoolVar(&options.quiet, "q", false, "perform operations quietly")
	flags.BoolVar(&options.assembleTest, "verify", false, "verify the generated output by assembling with ca65 and check if it matches the input")
	flags.BoolVar(&disasmOptions.ZeroBytes, "z", false, "output the trailing zero bytes of banks")

	err := flags.Parse(os.Args[1:])
	args := flags.Args()

	if err != nil || (len(args) == 0 && options.batch == "") {
		printBanner(options)
		fmt.Printf("usage: nesgodisasm [options] <file to disassemble>\n\n")
		flags.PrintDefaults()
		fmt.Println()
		os.Exit(1)
	}

	if options.batch == "" {
		options.input = args[0]
	}

	return options, &disasmOptions
}

func printBanner(options *optionFlags) {
	if !options.quiet {
		fmt.Println("[------------------------------------]")
		fmt.Println("[ nesgodisasm - NES ROM disassembler ]")
		fmt.Printf("[------------------------------------]\n\n")
		fmt.Printf("version: %s\n\n", buildinfo.Version(version, commit, date))
	}
}

// getFiles returns the list of files to process, either a single file or the matched files for
// batch processing.
func getFiles(options *optionFlags) ([]string, error) {
	if options.batch == "" {
		return []string{options.input}, nil
	}

	files, err := filepath.Glob(options.batch)
	if err != nil {
		return nil, fmt.Errorf("finding batch files failed: %w", err)
	}

	if len(files) == 0 {
		return nil, errors.New("no input files matched")
	}

	options.output = ""
	return files, nil
}

func disasmFile(options *optionFlags, disasmOptions *disasmoptions.Options) error {
	if !options.quiet {
		fmt.Printf(" * Processing %s ", options.input)
	}

	file, err := os.Open(options.input)
	if err != nil {
		return fmt.Errorf("opening file '%s': %w", options.input, err)
	}

	cart, err := cartridge.LoadFile(file)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	_ = file.Close()

	if err := openCodeDataLog(options, disasmOptions); err != nil {
		return err
	}

	disasmOptions.HexComments = !options.noHexComments
	disasmOptions.OffsetComments = !options.noOffsets

	dis, err := disasm.New(cart, disasmOptions)
	if err != nil {
		return fmt.Errorf("initializing disassembler: %w", err)
	}

	if disasmOptions.CodeDataLog != nil {
		_ = disasmOptions.CodeDataLog.Close()
	}

	var outputFile io.WriteCloser
	if options.output == "" {
		outputFile = os.Stdout
	} else {
		outputFile, err = os.Create(options.output)
		if err != nil {
			return fmt.Errorf("creating file '%s': %w", options.output, err)
		}
	}
	if err = dis.Process(outputFile); err != nil {
		return fmt.Errorf("processing file: %w", err)
	}
	if err = outputFile.Close(); err != nil {
		return fmt.Errorf("closing file: %w", err)
	}

	if options.assembleTest {
		if err = verifyOutput(cart, options); err != nil {
			return fmt.Errorf("- output file mismatch:\n%w", err)
		}
		if !options.quiet {
			fmt.Printf("- output file matched input file\n")
		}
	} else {
		fmt.Println()
	}
	return nil
}

func verifyOutput(cart *cartridge.Cartridge, options *optionFlags) error {
	if options.output == "" {
		return errors.New("can not verify console output")
	}

	filePart := filepath.Ext(options.output)
	objectFile, err := os.CreateTemp("", filePart+".*.o")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(objectFile.Name())
	}()

	var outputFile *os.File
	if options.debug {
		outputFile, err = os.Create("debug.asm")
		if err != nil {
			return fmt.Errorf("creating file '%s': %w", options.output, err)
		}
		defer func() {
			_ = outputFile.Close()
		}()
	} else {
		outputFile, err = os.CreateTemp("", filePart+".*.nes")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		defer func() {
			_ = os.Remove(outputFile.Name())
		}()
	}

	ca65Config := ca65.Config{
		PRGSize: len(cart.PRG),
		CHRSize: len(cart.CHR),
	}
	if err = ca65.AssembleUsingExternalApp(options.output, objectFile.Name(), outputFile.Name(), ca65Config); err != nil {
		return fmt.Errorf("creating .nes file failed: %w", err)
	}

	source, err := os.ReadFile(options.input)
	if err != nil {
		return fmt.Errorf("reading file for comparison: %w", err)
	}

	destination, err := os.ReadFile(outputFile.Name())
	if err != nil {
		return fmt.Errorf("reading file for comparison: %w", err)
	}

	if err := checkBufferEqual(source, destination, options.debug); err != nil {
		if detailsErr := compareCartridgeDetails(source, destination, options.debug); detailsErr != nil {
			return fmt.Errorf("comparing cartridge details: %w", detailsErr)
		}
		return err
	}
	return nil
}

func checkBufferEqual(input, output []byte, debug bool) error {
	if len(input) != len(output) {
		return fmt.Errorf("mismatched lengths, %d != %d", len(input), len(output))
	}

	var diffs uint64
	firstDiff := -1
	var firstDiffBytes [2]byte
	for i := range input {
		if input[i] == output[i] {
			continue
		}
		diffs++
		if firstDiff == -1 {
			firstDiff = i
			firstDiffBytes[0] = input[i]
			firstDiffBytes[1] = output[i]
		}
		if debug {
			fmt.Printf("offset 0x%04X mismatch - expected 0x%02X - got 0x%02X\n",
				i, input[i], output[i])
		}
	}
	if diffs == 0 {
		return nil
	}
	return fmt.Errorf("%d offset mismatches\n"+
		"first at offset %d (0x%04X) - expected 0x%02X - got 0x%02X", diffs,
		firstDiff, firstDiff, firstDiffBytes[0], firstDiffBytes[1])
}

func compareCartridgeDetails(input, output []byte, debug bool) error {
	inputReader := bytes.NewReader(input)
	outputReader := bytes.NewReader(output)

	cart1, err := cartridge.LoadFile(inputReader)
	if err != nil {
		return fmt.Errorf("loading cartridge file: %w", err)
	}
	cart2, err := cartridge.LoadFile(outputReader)
	if err != nil {
		return fmt.Errorf("loading cartridge file: %w", err)
	}

	if err := checkBufferEqual(cart1.PRG, cart2.PRG, debug); err != nil {
		fmt.Printf("PRG difference: %s\n", err)
	}
	if err := checkBufferEqual(cart1.CHR, cart2.CHR, debug); err != nil {
		fmt.Printf("CHR difference: %s\n", err)
	}
	if err := checkBufferEqual(cart1.Trainer, cart2.Trainer, debug); err != nil {
		fmt.Printf("Trainer difference: %s\n", err)
	}
	if cart1.Mapper != cart2.Mapper {
		fmt.Println("Mapper header does not match")
	}
	if cart1.Mirror != cart2.Mirror {
		fmt.Println("Mirror header does not match")
	}
	if cart1.Battery != cart2.Battery {
		fmt.Println("Battery header does not match")
	}
	return nil
}

func openCodeDataLog(options *optionFlags, disasmOptions *disasmoptions.Options) error {
	if options.codeDataLog == "" {
		return nil
	}

	log, err := os.Open(options.codeDataLog)
	if err != nil {
		return fmt.Errorf("opening file '%s': %w", options.codeDataLog, err)
	}
	disasmOptions.CodeDataLog = log
	return nil
}
