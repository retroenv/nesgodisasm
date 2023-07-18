// Package asm6 provides helpers to create asm6 assembler compatible asm output.
package asm6

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	assembler = "asm6f"
)

// AssembleUsingExternalApp calls the external assembler and linker to generate a .nes
// ROM from the given asm file.
func AssembleUsingExternalApp(asmFile, outputFile string) error {
	if _, err := exec.LookPath(assembler); err != nil {
		return fmt.Errorf("%s is not installed", assembler)
	}

	cmd := exec.Command(assembler, asmFile, outputFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("assembling file: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}
