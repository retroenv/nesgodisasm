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
	"github.com/retroenv/retrogolib/log"
	"github.com/retroenv/retrogolib/nes/cartridge"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

type optionFlags struct {
	logger *log.Logger

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
		options.logger.Fatal(err.Error())
	}

	for _, file := range files {
		options.input = file
		if len(files) > 1 || options.output == "" {
			// create output file name by replacing file extension with .asm
			options.output = file[:len(file)-len(filepath.Ext(file))] + ".asm"
		}

		if err := disasmFile(options, disasmOptions); err != nil {
			options.logger.Error("Disassembling failed", err)
		}
	}
}

func createLogger(options *optionFlags) *log.Logger {
	cfg := log.DefaultConfig()
	if options.debug {
		cfg.Level = log.DebugLevel
	}
	return log.NewWithConfig(cfg)
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

	logger := createLogger(options)
	options.logger = logger
	disasmOptions.Logger = logger

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
		options.logger.Info("Build info", log.String("version", buildinfo.Version(version, commit, date)))
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
		options.logger.Info("Processing ROM", log.String("file", options.input))
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
		if err = verifyOutput(cart, options, dis.CodeBaseAddress()); err != nil {
			return fmt.Errorf("output file mismatch: %w", err)
		}
		if !options.quiet {
			options.logger.Info("Output file matched input file")
		}
	}
	return nil
}

func verifyOutput(cart *cartridge.Cartridge, options *optionFlags, codeBaseAddress uint16) error {
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
		PrgBase: int(codeBaseAddress),
		PRGSize: len(cart.PRG),
		CHRSize: len(cart.CHR),
	}
	if err = ca65.AssembleUsingExternalApp(options.output, objectFile.Name(), outputFile.Name(), ca65Config); err != nil {
		return fmt.Errorf("reassembling .nes file failed: %w", err)
	}

	source, err := os.ReadFile(options.input)
	if err != nil {
		return fmt.Errorf("reading file for comparison: %w", err)
	}

	destination, err := os.ReadFile(outputFile.Name())
	if err != nil {
		return fmt.Errorf("reading file for comparison: %w", err)
	}

	if err = compareCartridgeDetails(options.logger, source, destination); err != nil {
		return fmt.Errorf("comparing cartridge details: %w", err)
	}

	return nil
}

func checkBufferEqual(logger *log.Logger, input, output []byte) error {
	if len(input) != len(output) {
		return fmt.Errorf("mismatched lengths, %d != %d", len(input), len(output))
	}

	var diffs uint64
	for i := range input {
		if input[i] == output[i] {
			continue
		}

		diffs++
		if diffs < 10 {
			logger.Error("Offset mismatch", nil,
				log.String("offset", fmt.Sprintf("0x%04X", i)),
				log.String("expected", fmt.Sprintf("0x%02X", input[i])),
				log.String("got", fmt.Sprintf("0x%02X", output[i])))
		}
	}
	if diffs == 0 {
		return nil
	}
	return fmt.Errorf("%d offset mismatches", diffs)
}

func compareCartridgeDetails(logger *log.Logger, input, output []byte) error {
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

	if err := checkBufferEqual(logger, cart1.PRG, cart2.PRG); err != nil {
		return fmt.Errorf("segment PRG mismatch: %w", err)
	}
	if err := checkBufferEqual(logger, cart1.CHR, cart2.CHR); err != nil {
		return fmt.Errorf("segment CHR mismatch: %w", err)
	}
	if err := checkBufferEqual(logger, cart1.Trainer, cart2.Trainer); err != nil {
		return fmt.Errorf("trainer mismatch: %w", err)
	}
	if cart1.Mapper != cart2.Mapper {
		return fmt.Errorf("mapper mismatch, expected %d but got %d", cart1.Mapper, cart2.Mapper)
	}
	if cart1.Mirror != cart2.Mirror {
		return fmt.Errorf("mirror mismatch, expected %d but got %d", cart1.Mirror, cart2.Mirror)
	}
	if cart1.Battery != cart2.Battery {
		return fmt.Errorf("battery mismatch, expected %d but got %d", cart1.Battery, cart2.Battery)
	}
	return nil
}

func openCodeDataLog(options *optionFlags, disasmOptions *disasmoptions.Options) error {
	if options.codeDataLog == "" {
		return nil
	}

	logFile, err := os.Open(options.codeDataLog)
	if err != nil {
		return fmt.Errorf("opening file '%s': %w", options.codeDataLog, err)
	}
	disasmOptions.CodeDataLog = logFile
	return nil
}
