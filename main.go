// Package main implements a NES ROM disassembler
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	disasm "github.com/retroenv/nesgodisasm/internal"
	"github.com/retroenv/nesgodisasm/internal/assembler"
	"github.com/retroenv/nesgodisasm/internal/assembler/ca65"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/nesgodisasm/internal/verification"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/buildinfo"
	"github.com/retroenv/retrogolib/log"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	logger, opts, disasmOptions := initializeApp()
	if !opts.Quiet {
		printBanner(logger, opts)
	}

	files, err := getFiles(opts)
	if err != nil {
		logger.Fatal(err.Error())
	}

	for _, file := range files {
		opts.Input = file
		if len(files) > 1 || opts.Output == "" {
			// create output file name by replacing file extension with .asm
			opts.Output = file[:len(file)-len(filepath.Ext(file))] + ".asm"
		}

		if err := disasmFile(logger, opts, disasmOptions); err != nil {
			logger.Error("Disassembling failed", log.Err(err))
		}
	}
}

func initializeApp() (*log.Logger, *options.Program, *options.Disassembler) {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	opts := &options.Program{}
	var zeroBytes bool

	flags.StringVar(&opts.Assembler, "a", "", "Assembler compatibility of the generated .asm file (asm6/ca65/nesasm)")
	flags.StringVar(&opts.Batch, "batch", "", "process a batch of given path and file mask and automatically .asm file naming, for example *.nes")
	// TODO add config option to generate ca65 config for the ROM
	flags.BoolVar(&opts.Debug, "debug", false, "enable debugging options for extended logging)")
	flags.StringVar(&opts.CodeDataLog, "cdl", "", "name of the .cdl Code/Data log file to load")
	flags.BoolVar(&opts.NoHexComments, "nohexcomments", false, "do not output opcode bytes as hex values in comments")
	flags.BoolVar(&opts.NoOffsets, "nooffsets", false, "do not output offsets in comments")
	flags.StringVar(&opts.Output, "o", "", "name of the output .asm file, printed on console if no name given")
	flags.BoolVar(&opts.Quiet, "q", false, "perform operations quietly")
	flags.BoolVar(&opts.AssembleTest, "verify", false, "verify the generated output by assembling with ca65 and check if it matches the input")
	flags.BoolVar(&zeroBytes, "z", false, "output the trailing zero bytes of banks")

	err := flags.Parse(os.Args[1:])
	args := flags.Args()

	disasmOptions := options.NewDisassembler(opts.Assembler)
	disasmOptions.ZeroBytes = zeroBytes

	logger := createLogger(opts)

	if err != nil || (len(args) == 0 && opts.Batch == "") {
		printBanner(logger, opts)
		fmt.Printf("usage: nesgodisasm [options] <file to disassemble>\n\n")
		flags.PrintDefaults()
		fmt.Println()
		os.Exit(1)
	}

	if opts.Batch == "" {
		opts.Input = args[0]
	}

	return logger, opts, &disasmOptions
}

func createLogger(options *options.Program) *log.Logger {
	cfg := log.DefaultConfig()
	if options.Debug {
		cfg.Level = log.DebugLevel
	}
	if options.Quiet {
		cfg.Level = log.ErrorLevel
	}
	return log.NewWithConfig(cfg)
}

func printBanner(logger *log.Logger, options *options.Program) {
	if !options.Quiet {
		fmt.Println("[------------------------------------]")
		fmt.Println("[ nesgodisasm - NES ROM disassembler ]")
		fmt.Printf("[------------------------------------]\n\n")
		logger.Info("Build info", log.String("version", buildinfo.Version(version, commit, date)))
	}
}

// getFiles returns the list of files to process, either a single file or the matched files for
// batch processing.
func getFiles(options *options.Program) ([]string, error) {
	if options.Batch == "" {
		return []string{options.Input}, nil
	}

	files, err := filepath.Glob(options.Batch)
	if err != nil {
		return nil, fmt.Errorf("finding batch files failed: %w", err)
	}

	if len(files) == 0 {
		return nil, errors.New("no input files matched")
	}

	options.Output = ""
	return files, nil
}

// nolint: cyclop, funlen
func disasmFile(logger *log.Logger, opts *options.Program, disasmOptions *options.Disassembler) error {
	file, err := os.Open(opts.Input)
	if err != nil {
		return fmt.Errorf("opening file '%s': %w", opts.Input, err)
	}

	cart, err := cartridge.LoadFile(file)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	_ = file.Close()

	if !opts.Quiet {
		logger.Info("Processing ROM",
			log.String("file", opts.Input),
			log.Uint8("mapper", cart.Mapper))
	}
	if cart.Mapper != 0 {
		logger.Warn("Only NROM (mapper 0) is currently supported, multi bank mapper support is still in development")
	}

	if err := openCodeDataLog(opts, disasmOptions); err != nil {
		return err
	}

	disasmOptions.HexComments = !opts.NoHexComments
	disasmOptions.OffsetComments = !opts.NoOffsets

	dis, err := disasm.New(logger, cart, disasmOptions)
	if err != nil {
		return fmt.Errorf("initializing disassembler: %w", err)
	}

	if disasmOptions.CodeDataLog != nil {
		_ = disasmOptions.CodeDataLog.Close()
	}

	var outputFile io.WriteCloser
	if opts.Output == "" {
		outputFile = os.Stdout
	} else {
		outputFile, err = os.Create(opts.Output)
		if err != nil {
			return fmt.Errorf("creating file '%s': %w", opts.Output, err)
		}
	}
	if err = dis.Process(outputFile); err != nil {
		return fmt.Errorf("processing file: %w", err)
	}
	if err = outputFile.Close(); err != nil {
		return fmt.Errorf("closing file: %w", err)
	}

	if opts.Debug && opts.Assembler == assembler.Ca65 {
		printCa65Config(logger, cart, dis)
	}

	if opts.AssembleTest {
		if err = verification.VerifyOutput(logger, cart, opts, dis.CodeBaseAddress()); err != nil {
			return fmt.Errorf("output file mismatch: %w", err)
		}
		if !opts.Quiet {
			logger.Info("Output file matched input file")
		}
	}
	return nil
}

func printCa65Config(logger *log.Logger, cart *cartridge.Cartridge, dis *disasm.Disasm) {
	ca65Config := ca65.Config{
		PrgBase: int(dis.CodeBaseAddress()),
		PRGSize: len(cart.PRG),
		CHRSize: len(cart.CHR),
	}
	cfg := ca65.GenerateMapperConfig(ca65Config)
	logger.Debug("Ca65 config:")
	fmt.Println(cfg)
}

func openCodeDataLog(options *options.Program, disasmOptions *options.Disassembler) error {
	if options.CodeDataLog == "" {
		return nil
	}

	logFile, err := os.Open(options.CodeDataLog)
	if err != nil {
		return fmt.Errorf("opening file '%s': %w", options.CodeDataLog, err)
	}
	disasmOptions.CodeDataLog = logFile
	return nil
}
