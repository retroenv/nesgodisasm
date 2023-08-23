package asm6

import "github.com/retroenv/retrogolib/arch/nes/parameter"

// ParamConfig configures the instruction parameter string converter.
var ParamConfig = parameter.Config{
	ZeroPagePrefix: "",
	AbsolutePrefix: "a:",
	IndirectPrefix: "(",
	IndirectSuffix: ")",
}
