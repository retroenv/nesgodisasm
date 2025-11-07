// Package ca65 provides helpers to create ca65 assembler compatible asm output.
package ca65

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/retroenv/retrodisasm/internal/program"
)

const (
	assemblerName = "ca65"
	linkerName    = "ld65"
)

// Config holds the ROM building configuration.
type Config struct {
	App *program.Program

	PRGSize int
	CHRSize int
}

// AssembleUsingExternalApp calls the external assembler and linker to generate a .nes
// ROM from the given asm file.
func AssembleUsingExternalApp(asmFile, objectFile, outputFile string, conf Config) error {
	if _, err := exec.LookPath(assemblerName); err != nil {
		return fmt.Errorf("%s is not installed", assemblerName)
	}
	if _, err := exec.LookPath(linkerName); err != nil {
		return fmt.Errorf("%s is not installed", linkerName)
	}

	cmd := exec.Command(assemblerName, asmFile, "-o", objectFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("assembling file: %s: %w", strings.TrimSpace(string(out)), err)
	}

	configFile, err := os.CreateTemp("", "rom"+".*.cfg")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(configFile.Name())
	}()

	mapperConfig, err := GenerateMapperConfig(conf)
	if err != nil {
		return fmt.Errorf("generating ca65 config: %w", err)
	}

	if err := os.WriteFile(configFile.Name(), []byte(mapperConfig), 0666); err != nil {
		return fmt.Errorf("writing linker config: %w", err)
	}

	cmd = exec.Command(linkerName, "-C", configFile.Name(), "-o", outputFile, objectFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("linking file: %s: %w", string(out), err)
	}

	return nil
}
