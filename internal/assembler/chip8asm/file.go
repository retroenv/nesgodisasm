// Package chip8asm provides CHIP-8 assembler file writer implementation.
package chip8asm

import (
	"fmt"
	"io"

	"github.com/retroenv/nesgodisasm/internal/assembler"
	"github.com/retroenv/nesgodisasm/internal/options"
	"github.com/retroenv/nesgodisasm/internal/program"
	"github.com/retroenv/nesgodisasm/internal/writer"
)

// FileWriter writes CHIP-8 assembly files.
type FileWriter struct {
	app           *program.Program
	options       options.Disassembler
	mainWriter    io.Writer
	newBankWriter assembler.NewBankWriter
	writer        *writer.Writer
}

// New creates a new CHIP-8 file writer.
func New(app *program.Program, options options.Disassembler, mainWriter io.Writer, newBankWriter assembler.NewBankWriter) writer.AssemblerWriter {
	opts := writer.Options{
		OffsetComments: options.OffsetComments,
	}
	return &FileWriter{
		app:           app,
		options:       options,
		mainWriter:    mainWriter,
		newBankWriter: newBankWriter,
		writer:        writer.New(app, mainWriter, opts),
	}
}

// Write writes the CHIP-8 assembly file.
func (w *FileWriter) Write() error {
	if _, err := fmt.Fprintf(w.mainWriter, "; CHIP-8 ROM Disassembly\n"); err != nil {
		return fmt.Errorf("writing header comment: %w", err)
	}

	if _, err := fmt.Fprintf(w.mainWriter, "; Code base address: $%04X\n", w.app.CodeBaseAddress); err != nil {
		return fmt.Errorf("writing code base address: %w", err)
	}

	if _, err := fmt.Fprintf(w.mainWriter, "; Program starts at $200 in CHIP-8 memory space\n\n"); err != nil {
		return fmt.Errorf("writing memory space comment: %w", err)
	}

	if _, err := fmt.Fprintf(w.mainWriter, ".org $200\n\n"); err != nil {
		return fmt.Errorf("writing org directive: %w", err)
	}

	// Write all PRG banks (usually just one for CHIP-8)
	for _, bank := range w.app.PRG {
		if err := w.writeBank(bank); err != nil {
			return fmt.Errorf("writing bank: %w", err)
		}
	}

	return nil
}

// writeBank writes a CHIP-8 program bank.
func (w *FileWriter) writeBank(bank *program.PRGBank) error {
	endIndex := w.getChip8EndIndex(bank)

	for i := range endIndex {
		offset := bank.Offsets[i]

		if len(offset.Data) == 0 && offset.Code == "" {
			continue
		}

		if err := w.writeLabel(offset); err != nil {
			return fmt.Errorf("writing label: %w", err)
		}

		if err := w.writeOffset(offset, i); err != nil {
			return fmt.Errorf("writing offset: %w", err)
		}
	}

	return nil
}

// writeLabel writes a label if present in the offset.
func (w *FileWriter) writeLabel(offset program.Offset) error {
	if offset.Label != "" {
		if _, err := fmt.Fprintf(w.mainWriter, "%s:\n", offset.Label); err != nil {
			return fmt.Errorf("writing label %s: %w", offset.Label, err)
		}
	}
	return nil
}

// writeOffset writes either code or data for an offset.
func (w *FileWriter) writeOffset(offset program.Offset, index int) error {
	if offset.Code != "" {
		return w.writeCode(offset)
	}
	if len(offset.Data) > 0 {
		return w.writeData(offset, index)
	}
	return nil
}

// writeCode writes a CHIP-8 instruction.
func (w *FileWriter) writeCode(offset program.Offset) error {
	if _, err := fmt.Fprintf(w.mainWriter, "    %s", offset.Code); err != nil {
		return fmt.Errorf("writing code: %w", err)
	}

	if w.options.OffsetComments && len(offset.Data) >= 2 {
		hexValue := uint16(offset.Data[0])<<8 | uint16(offset.Data[1])
		if _, err := fmt.Fprintf(w.mainWriter, " ; $%04X", hexValue); err != nil {
			return fmt.Errorf("writing hex comment: %w", err)
		}
	}

	if _, err := fmt.Fprintf(w.mainWriter, "\n"); err != nil {
		return fmt.Errorf("writing newline: %w", err)
	}

	return nil
}

// writeData writes raw data bytes.
func (w *FileWriter) writeData(offset program.Offset, index int) error {
	if _, err := fmt.Fprintf(w.mainWriter, "    .byte $%02X", offset.Data[0]); err != nil {
		return fmt.Errorf("writing first data byte: %w", err)
	}

	for j := 1; j < len(offset.Data); j++ {
		if _, err := fmt.Fprintf(w.mainWriter, ", $%02X", offset.Data[j]); err != nil {
			return fmt.Errorf("writing data byte %d: %w", j, err)
		}
	}

	if w.options.OffsetComments {
		address := w.app.CodeBaseAddress + uint16(index)
		if _, err := fmt.Fprintf(w.mainWriter, " ; $%04X", address); err != nil {
			return fmt.Errorf("writing address comment: %w", err)
		}
	}

	if _, err := fmt.Fprintf(w.mainWriter, "\n"); err != nil {
		return fmt.Errorf("writing newline: %w", err)
	}

	return nil
}

// getChip8EndIndex finds the last meaningful byte in a CHIP-8 ROM
// without NES-specific vector logic.
func (w *FileWriter) getChip8EndIndex(bank *program.PRGBank) int {
	// If zero bytes are requested, use full bank
	if w.options.ZeroBytes {
		return len(bank.Offsets)
	}

	// Search backwards from the end to find the last non-zero byte
	for i := len(bank.Offsets) - 1; i >= 0; i-- {
		offset := bank.Offsets[i]
		if len(offset.Data) > 0 {
			// Check if this offset has actual data (not just zeros)
			for _, b := range offset.Data {
				if b != 0 {
					return i + 1
				}
			}
		}
		// Also include any labeled or code offsets
		if offset.Label != "" || offset.Code != "" {
			return i + 1
		}
	}

	return 0
}
