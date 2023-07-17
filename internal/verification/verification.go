// Package verification verifies that the generated output file recreates the input.
package verification

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/retroenv/nesgodisasm/internal/ca65"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
	"github.com/retroenv/retrogolib/log"
)

// VerifyOutput verifies that the output file recreates the exact input file.
func VerifyOutput(logger *log.Logger, cart *cartridge.Cartridge, options *options.Program, codeBaseAddress uint16) error {
	if options.Output == "" {
		return errors.New("can not verify console output")
	}

	filePart := filepath.Ext(options.Output)
	objectFile, err := os.CreateTemp("", filePart+".*.o")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(objectFile.Name())
	}()

	var outputFile *os.File
	if options.Debug {
		outputFile, err = os.Create("debug.asm")
		if err != nil {
			return fmt.Errorf("creating file '%s': %w", options.Output, err)
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
	if err = ca65.AssembleUsingExternalApp(options.Output, objectFile.Name(), outputFile.Name(), ca65Config); err != nil {
		return fmt.Errorf("reassembling .nes file failed: %w", err)
	}

	source, err := os.ReadFile(options.Input)
	if err != nil {
		return fmt.Errorf("reading file for comparison: %w", err)
	}

	destination, err := os.ReadFile(outputFile.Name())
	if err != nil {
		return fmt.Errorf("reading file for comparison: %w", err)
	}

	if err = compareCartridgeDetails(logger, source, destination); err != nil {
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
			logger.Error("Offset mismatch",
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
