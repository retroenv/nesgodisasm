package ca65

import (
	"fmt"
	"strings"
)

const (
	memoryConfigPart1 = `
MEMORY {
    ZP:          start = $00,    size = $100,    type = rw, file = "";
    RAM:         start = $0200,  size = $600,    type = rw, file = "";
    HDR:         start = $0000,  size = $10,     type = ro, file = %O, fill = yes;
`

	memoryPrgBankTemplate = `    %-12s start = $%04X,  size = $%04X,   type = ro, file = %%O, fill = yes;
`

	memoryConfigPart2 = `    CHR:         start = $0000,  size = $%04X,   type = ro, file = %%O, fill = yes;
}

`

	segmentsConfigPart1 = `
SEGMENTS {
    ZEROPAGE:    load = ZP,  type = zp;
    OAM:         load = RAM, type = bss, start = $200, optional = yes;
    BSS:         load = RAM, type = bss;
    HEADER:      load = HDR, type = ro;
`

	segmentsPrgBankTemplate = `    %-12s load = %s, type = ro, start = $%04X;
`

	segmentsConfigPart2 = `    VECTORS:     load = %s, type = ro, start = $%04X;
    TILES:       load = CHR, type = ro;
}
`
)

// GenerateMapperConfig generates a ca65 linker config dynamically based on the passed ROM settings.
func GenerateMapperConfig(conf Config) (string, error) {
	prgSize := conf.PRGSize
	vectorStart := conf.App.CodeBaseAddress + uint16(prgSize) - 6

	buf := &strings.Builder{}
	buf.WriteString(memoryConfigPart1)

	for _, bank := range conf.App.PRG {
		if _, err := fmt.Fprintf(buf, memoryPrgBankTemplate, bank.Name+":", conf.App.CodeBaseAddress, len(bank.Offsets)); err != nil {
			return "", fmt.Errorf("writing memory bank line: %w", err)
		}
	}

	if _, err := fmt.Fprintf(buf, memoryConfigPart2, conf.CHRSize); err != nil {
		return "", fmt.Errorf("writing memory config: %w", err)
	}

	buf.WriteString(segmentsConfigPart1)

	for _, bank := range conf.App.PRG {
		if _, err := fmt.Fprintf(buf, segmentsPrgBankTemplate, bank.Name+":", bank.Name, conf.App.CodeBaseAddress); err != nil {
			return "", fmt.Errorf("writing segment bank line: %w", err)
		}
	}

	lastBank := conf.App.PRG[len(conf.App.PRG)-1]
	if _, err := fmt.Fprintf(buf, segmentsConfigPart2, lastBank.Name, vectorStart); err != nil {
		return "", fmt.Errorf("writing segments config: %w", err)
	}

	generated := buf.String()
	return generated, nil
}
