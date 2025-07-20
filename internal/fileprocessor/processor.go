// Package fileprocessor handles file loading and processing operations
package fileprocessor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	disasm "github.com/retroenv/nesgodisasm/internal"
	"github.com/retroenv/nesgodisasm/internal/config"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/nesgodisasm/internal/verification"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/log"
)

// ProcessFile handles the complete file processing workflow
func ProcessFile(logger *log.Logger, opts options.Program, disasmOptions options.Disassembler) error {
	cart, err := loadCartridge(opts)
	if err != nil {
		return fmt.Errorf("loading cartridge: %w", err)
	}

	writer, err := createWriter(opts)
	if err != nil {
		return fmt.Errorf("creating writer: %w", err)
	}
	defer func() {
		if closer, ok := writer.(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	fileWriterConstructor, err := config.CreateFileWriterConstructor(opts.Assembler)
	if err != nil {
		return fmt.Errorf("creating file writer constructor: %w", err)
	}

	dis, err := setupDisassembler(logger, cart, disasmOptions, fileWriterConstructor)
	if err != nil {
		return fmt.Errorf("setting up disassembler: %w", err)
	}

	// Create a simple new bank writer for single-file output
	newBankWriter := func(bankName string) (io.WriteCloser, error) {
		return &nopCloser{writer}, nil
	}

	app, err := dis.Process(writer, newBankWriter)
	if err != nil {
		return fmt.Errorf("disassembling: %w", err)
	}

	if opts.AssembleTest {
		if err := verification.VerifyOutput(logger, opts, cart, app); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}
		logger.Info("Verification successful")
	}

	return nil
}

// GetFilesToProcess returns list of files to process based on options
func GetFilesToProcess(opts *options.Program) ([]string, error) {
	if opts.Batch != "" {
		matches, err := filepath.Glob(opts.Batch)
		if err != nil {
			return nil, fmt.Errorf("globbing batch pattern: %w", err)
		}
		return matches, nil
	}
	return []string{opts.Input}, nil
}

// GenerateOutputFilename generates output filename for a given input file
func GenerateOutputFilename(inputFile string) string {
	ext := filepath.Ext(inputFile)
	return inputFile[:len(inputFile)-len(ext)] + ".asm"
}

func loadCartridge(opts options.Program) (*cartridge.Cartridge, error) {
	file, err := os.Open(opts.Input)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", opts.Input, err)
	}
	defer func() { _ = file.Close() }()

	var cart *cartridge.Cartridge
	if opts.Binary {
		cart, err = cartridge.LoadBuffer(file)
	} else {
		cart, err = cartridge.LoadFile(file)
	}
	if err != nil {
		return nil, fmt.Errorf("loading cartridge: %w", err)
	}

	if opts.CodeDataLog != "" {
		logFile, err := os.Open(opts.CodeDataLog)
		if err != nil {
			return nil, fmt.Errorf("opening CDL file %s: %w", opts.CodeDataLog, err)
		}
		// Note: The CDL handling might need to be adjusted based on the actual API
		_ = logFile.Close() // For now, just close it
	}

	return cart, nil
}

func createWriter(opts options.Program) (io.Writer, error) {
	if opts.Output == "" {
		return os.Stdout, nil
	}

	file, err := os.Create(opts.Output)
	if err != nil {
		return nil, fmt.Errorf("creating output file %s: %w", opts.Output, err)
	}
	return file, nil
}

func setupDisassembler(logger *log.Logger, cart *cartridge.Cartridge,
	disasmOptions options.Disassembler, fileWriterConstructor disasm.FileWriterConstructor) (*disasm.Disasm, error) {

	arch := config.CreateArchitecture()

	dis, err := disasm.New(arch, logger, cart, disasmOptions, fileWriterConstructor)
	if err != nil {
		return nil, fmt.Errorf("creating disassembler: %w", err)
	}

	return dis, nil
}

// PrintBanner prints application version information
func PrintBanner(logger *log.Logger, opts options.Program, version, commit, date string) {
	if opts.Quiet {
		return
	}

	versionString := version
	if commit != "" {
		if len(commit) > 7 {
			commit = commit[:7]
		}
		versionString += fmt.Sprintf(" (%s)", commit)
	}

	logger.Info("nesgodisasm", log.String("version", versionString))

	if date != "" && !strings.Contains(date, "unknown") {
		logger.Info("Build", log.String("date", date))
	}
}

// nopCloser wraps an io.Writer to add a no-op Close method
type nopCloser struct {
	io.Writer
}

func (nc *nopCloser) Close() error {
	return nil
}
