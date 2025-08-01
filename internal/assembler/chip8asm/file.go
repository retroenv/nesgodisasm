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
	// Write CHIP-8 header
	_, err := fmt.Fprintf(w.mainWriter, "; CHIP-8 ROM Disassembly\n")
	if err != nil {
		return err
	}
	
	_, err = fmt.Fprintf(w.mainWriter, "; Code base address: $%04X\n", w.app.CodeBaseAddress)
	if err != nil {
		return err
	}
	
	_, err = fmt.Fprintf(w.mainWriter, "; Program starts at $200 in CHIP-8 memory space\n\n")
	if err != nil {
		return err
	}
	
	_, err = fmt.Fprintf(w.mainWriter, ".org $200\n\n")
	if err != nil {
		return err
	}
	
	// Write all PRG banks (usually just one for CHIP-8)
	for _, bank := range w.app.PRG {
		err = w.writeBank(bank)
		if err != nil {
			return err
		}
	}
	
	return nil
}

// writeBank writes a CHIP-8 program bank.
func (w *FileWriter) writeBank(bank *program.PRGBank) error {
	// For CHIP-8, find the last non-zero byte without NES vector logic
	endIndex := w.getChip8EndIndex(bank)
	
	for i := 0; i < endIndex; i++ {
		offset := bank.Offsets[i]
		
		// Skip empty offsets
		if len(offset.Data) == 0 && offset.Code == "" {
			continue
		}
		
		// Write label if present
		if offset.Label != "" {
			_, err := fmt.Fprintf(w.mainWriter, "%s:\n", offset.Label)
			if err != nil {
				return err
			}
		}
		
		// Write the instruction or data
		if offset.Code != "" {
			// CHIP-8 instruction
			_, err := fmt.Fprintf(w.mainWriter, "    %s", offset.Code)
			if err != nil {
				return err
			}
			
			// Add hex comment if enabled
			if w.options.OffsetComments && len(offset.Data) >= 2 {
				_, err = fmt.Fprintf(w.mainWriter, " ; $%04X", 
					uint16(offset.Data[0])<<8|uint16(offset.Data[1]))
				if err != nil {
					return err
				}
			}
			
			_, err = fmt.Fprintf(w.mainWriter, "\n")
			if err != nil {
				return err
			}
		} else if len(offset.Data) > 0 {
			// Raw data
			_, err := fmt.Fprintf(w.mainWriter, "    .byte $%02X", offset.Data[0])
			if err != nil {
				return err
			}
			for j := 1; j < len(offset.Data); j++ {
				_, err = fmt.Fprintf(w.mainWriter, ", $%02X", offset.Data[j])
				if err != nil {
					return err
				}
			}
			
			// Add offset comment if enabled
			if w.options.OffsetComments {
				_, err = fmt.Fprintf(w.mainWriter, " ; $%04X", w.app.CodeBaseAddress+uint16(i))
				if err != nil {
					return err
				}
			}
			
			_, err = fmt.Fprintf(w.mainWriter, "\n")
			if err != nil {
				return err
			}
		}
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