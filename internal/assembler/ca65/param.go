package ca65

import "github.com/retroenv/retrogolib/arch/system/nes/parameter"

// ParamConfig configures the instruction parameter string converter.
var ParamConfig = parameter.Config{
	ZeroPagePrefix: "z:",
	AbsolutePrefix: "a:",
	IndirectPrefix: "(",
	IndirectSuffix: ")",
}
