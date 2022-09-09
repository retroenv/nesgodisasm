package ca65

import "fmt"

var mapper0Config = `
MEMORY {
    ZP:     start = $00,    size = $100,    type = rw, file = "";
    RAM:    start = $0200,  size = $600,    type = rw, file = "";
    HDR:    start = $0000,  size = $10,     type = ro, file = %%O, fill = yes;
    PRG:    start = $%04X,  size = $%04X,   type = ro, file = %%O, fill = yes;
    CHR:    start = $0000,  size = $%04X,   type = ro, file = %%O, fill = yes;
}

SEGMENTS {
    ZEROPAGE:   load = ZP,  type = zp;
    OAM:        load = RAM, type = bss, start = $200, optional = yes;
    BSS:        load = RAM, type = bss;
    HEADER:     load = HDR, type = ro;
    CODE:       load = PRG, type = ro, start = $%04X;
    DPCM:       load = PRG, type = ro, start = $C000, optional = yes;
    VECTORS:    load = PRG, type = ro, start = $FFFA;
    TILES:      load = CHR, type = ro;
}
`

// generateMapperConfig generates a ca65 linker config dynamically based on the passed ROM settings.
func generateMapperConfig(conf Config) string {
	prgSize := conf.PRGSize
	prgStart := 0x10000 - prgSize

	generatedConfig := fmt.Sprintf(mapper0Config, prgStart, prgSize, conf.CHRSize, prgStart)
	return generatedConfig
}
