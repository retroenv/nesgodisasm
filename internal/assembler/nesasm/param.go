package nesasm

import "github.com/retroenv/retrogolib/arch/nes/parameter"

// ParamConfig configures the instruction parameter string converter.
var ParamConfig = parameter.Config{
	IndirectNoParentheses: true,
}
