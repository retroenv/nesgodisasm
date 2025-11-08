// Package asm6 provides helpers to create asm6 assembler compatible asm output.
package asm6

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const assemblerName = "asm6f"

// AssembleUsingExternalApp calls the external assembler and linker to generate a .nes
// ROM from the given asm file.
func AssembleUsingExternalApp(ctx context.Context, asmFile, outputFile string) error {
	if _, err := exec.LookPath(assemblerName); err != nil {
		return fmt.Errorf("%s is not installed", assemblerName)
	}

	cmd := exec.CommandContext(ctx, assemblerName, asmFile, outputFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("assembling file: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}
