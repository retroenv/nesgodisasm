// Package nesasm provides helpers to create nesasm assembler compatible asm output.
package nesasm

import (
	"fmt"
	"os/exec"
	"strings"
)

const assemblerName = "nesasm"

// AssembleUsingExternalApp calls the external assembler and linker to generate a .nes
// ROM from the given asm file.
func AssembleUsingExternalApp(asmFile, outputFile string) error {
	if _, err := exec.LookPath(assemblerName); err != nil {
		return fmt.Errorf("%s is not installed", assemblerName)
	}

	cmd := exec.Command(assemblerName, "-z", "-o", outputFile, asmFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("assembling file: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}
