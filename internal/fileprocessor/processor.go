// Package fileprocessor handles file loading and processing operations
package fileprocessor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	disasm "github.com/retroenv/retrodisasm/internal"
	"github.com/retroenv/retrodisasm/internal/app"
	"github.com/retroenv/retrodisasm/internal/arch/chip8"
	"github.com/retroenv/retrodisasm/internal/arch/m6502"
	"github.com/retroenv/retrodisasm/internal/assembler"
	"github.com/retroenv/retrodisasm/internal/options"
	"github.com/retroenv/retrodisasm/internal/program"
	"github.com/retroenv/retrodisasm/internal/verification"
	"github.com/retroenv/retrogolib/arch"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/parameter"
	"github.com/retroenv/retrogolib/log"
)

// ProcessFile handles the complete file processing workflow
func ProcessFile(ctx context.Context, logger *log.Logger, opts options.Program, disasmOptions options.Disassembler) error {
	system := determineSystem(logger, opts)

	// Update disasm options with the determined system
	disasmOptions.System = system

	// Validate assembler is supported for this system
	if err := assembler.ValidateSystemAssembler(system, opts.Assembler); err != nil {
		return fmt.Errorf("incompatible assembler: %w", err)
	}

	cart, err := loadCartridge(opts, system)
	if err != nil {
		return fmt.Errorf("loading cartridge: %w", err)
	}

	writer, err := createWriter(opts)
	if err != nil {
		return fmt.Errorf("creating writer: %w", err)
	}
	defer closeWriter(writer)

	dis, err := createDisassembler(logger, opts, disasmOptions, cart, system)
	if err != nil {
		return fmt.Errorf("creating disassembler: %w", err)
	}

	app.PrintInfo(logger, opts, cart, system)

	result, err := runDisassembly(dis, writer)
	if err != nil {
		return fmt.Errorf("disassembling: %w", err)
	}

	if opts.AssembleTest {
		if err := verification.VerifyOutput(ctx, logger, opts, cart, result); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}
		logger.Info("Verification successful")
	}

	return nil
}

// determineSystem determines the system type from options or file detection
func determineSystem(logger *log.Logger, opts options.Program) arch.System {
	system, _ := arch.SystemFromString(opts.System)
	if system == "" {
		system = detectSystemFromFile(opts.Input)
		logger.Debug("Auto-detected system",
			log.Stringer("system", system),
			log.String("file", opts.Input))
	}
	return system
}

// createDisassembler creates and configures the disassembler
func createDisassembler(logger *log.Logger, opts options.Program, disasmOptions options.Disassembler, cart *cartridge.Cartridge, system arch.System) (*disasm.Disasm, error) {
	fileWriterConstructor, paramConverter, err := app.InitializeAssemblerCompatibleMode(opts.Assembler)
	if err != nil {
		return nil, fmt.Errorf("initializing assembler compatible mode: %w", err)
	}

	dis, err := createDisassemblerForSystem(logger, system, paramConverter, cart, disasmOptions, fileWriterConstructor)
	if err != nil {
		return nil, fmt.Errorf("creating disassembler: %w", err)
	}
	return dis, nil
}

// runDisassembly executes the disassembly process
func runDisassembly(dis *disasm.Disasm, writer io.Writer) (*program.Program, error) {
	newBankWriter := func(bankName string) (io.WriteCloser, error) {
		return &nopCloser{writer}, nil
	}

	result, err := dis.Process(writer, newBankWriter)
	if err != nil {
		return nil, fmt.Errorf("processing disassembly: %w", err)
	}
	return result, nil
}

// closeWriter safely closes the writer if it implements io.Closer
func closeWriter(writer io.Writer) {
	if closer, ok := writer.(io.Closer); ok {
		_ = closer.Close()
	}
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

func loadCartridge(opts options.Program, system arch.System) (*cartridge.Cartridge, error) {
	file, err := os.Open(opts.Input)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", opts.Input, err)
	}
	defer func() { _ = file.Close() }()

	var cart *cartridge.Cartridge

	// Handle CHIP-8 files as binary data
	switch {
	case opts.Binary, system == arch.CHIP8System:
		cart, err = cartridge.LoadBuffer(file)
	default:
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

// detectSystemFromFile determines the system type based on file extension
func detectSystemFromFile(filename string) arch.System {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".ch8", ".rom":
		// ROM files could be CHIP-8, default to CHIP-8 for .rom extension
		return arch.CHIP8System
	case ".nes":
		return arch.NES
	default:
		// Default to M6502 for unknown extensions
		return arch.NES
	}
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

	logger.Info("retrodisasm", log.String("version", versionString))

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

// createDisassemblerForSystem creates a disassembler for the specified system architecture.
func createDisassemblerForSystem(logger *log.Logger, system arch.System, paramConverter parameter.Converter,
	cart *cartridge.Cartridge, disasmOptions options.Disassembler, fileWriterConstructor disasm.FileWriterConstructor) (*disasm.Disasm, error) {

	switch system {
	case arch.NES:
		arch := m6502.New(logger, paramConverter)
		dis, err := disasm.New(logger, arch, cart, disasmOptions, fileWriterConstructor)
		if err != nil {
			return nil, fmt.Errorf("creating m6502 disassembler: %w", err)
		}
		return dis, nil
	case arch.CHIP8System:
		arch := chip8.New(paramConverter)
		dis, err := disasm.New(logger, arch, cart, disasmOptions, fileWriterConstructor)
		if err != nil {
			return nil, fmt.Errorf("creating chip8 disassembler: %w", err)
		}
		return dis, nil
	default:
		return nil, fmt.Errorf("unsupported system '%s'", system)
	}
}
