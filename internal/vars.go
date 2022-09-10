package disasm

import (
	"fmt"

	"github.com/retroenv/nesgodisasm/internal/program"
	. "github.com/retroenv/retrogolib/nes/addressing"
	"github.com/retroenv/retrogolib/nes/cpu"
	"github.com/retroenv/retrogolib/nes/parameter"
)

const (
	dataNaming            = "_data_%04x"
	dataNamingIndexed     = "_data_%04x_indexed"
	variableNaming        = "_var_%04x"
	variableNamingIndexed = "_var_%04x_indexed"
)

type variable struct {
	reads  bool
	writes bool

	name         string
	indexedUsage bool     // access with X/Y registers indicates table
	usageAt      []uint16 // list of all addresses that use this offset
}

func (dis *Disasm) addVariableReference(offset uint16, opcode cpu.Opcode, address uint16) bool {
	var reads, writes bool
	if opcode.ReadWritesMemory() {
		reads = true
		writes = true
	} else {
		reads = opcode.ReadsMemory()
		writes = opcode.WritesMemory()
	}
	if !reads && !writes {
		return false
	}

	varInfo := dis.variables[address]
	if varInfo == nil {
		varInfo = &variable{}
		dis.variables[address] = varInfo
	}
	varInfo.usageAt = append(varInfo.usageAt, offset)

	if reads {
		varInfo.reads = true
	}
	if writes {
		varInfo.writes = true
	}

	switch opcode.Addressing {
	case ZeroPageXAddressing, ZeroPageYAddressing,
		AbsoluteXAddressing, AbsoluteYAddressing,
		IndirectXAddressing, IndirectYAddressing:
		varInfo.indexedUsage = true
	}

	return true
}

// processJumpTargets processes all variables and updates the instructions that use them
// with a generated alias name.
func (dis *Disasm) processVariables() error {
	for address, varInfo := range dis.variables {
		if len(varInfo.usageAt) == 1 && !varInfo.indexedUsage && address < CodeBaseAddress {
			if !varInfo.reads || !varInfo.writes {
				continue // ignore only once usages or ones that are not read and write
			}
		}

		var offsetInfo *offset
		if address >= CodeBaseAddress {
			offset := dis.addressToOffset(address)
			offsetInfo = &dis.offsets[offset]
		} else {
			dis.usedVariables[address] = struct{}{}
		}

		var reference string
		varInfo.name, reference = dis.dataName(offsetInfo, varInfo.indexedUsage, address)

		for _, usedAddress := range varInfo.usageAt {
			offset := dis.addressToOffset(usedAddress)
			offsetInfo := &dis.offsets[offset]

			converted, err := parameter.String(dis.converter, offsetInfo.opcode.Addressing, reference)
			if err != nil {
				return err
			}

			switch offsetInfo.opcode.Addressing {
			case ZeroPageAddressing, ZeroPageXAddressing, ZeroPageYAddressing:
				offsetInfo.Code = fmt.Sprintf("%s z:%s", offsetInfo.opcode.Instruction.Name, converted)
			case AbsoluteAddressing, AbsoluteXAddressing, AbsoluteYAddressing:
				offsetInfo.Code = fmt.Sprintf("%s a:%s", offsetInfo.opcode.Instruction.Name, converted)
			case IndirectAddressing, IndirectXAddressing, IndirectYAddressing:
				offsetInfo.Code = fmt.Sprintf("%s %s", offsetInfo.opcode.Instruction.Name, converted)
			}
		}
	}
	return nil
}

// dataName calculates the name of a variable based on its address.
// It returns the name of the variable and a string to reference it, it is possible that the reference
// is using an adjuster like -1.
func (dis *Disasm) dataName(offsetInfo *offset, indexedUsage bool, address uint16) (string, string) {
	var reference string
	if offsetInfo != nil {
		offsetInfo, address, reference = dis.setNameOfNextOffset(address)

		// if destination has an existing label, reuse it
		if offsetInfo.Label != "" {
			return offsetInfo.Label, reference
		}
	}

	var name string
	prgAccess := offsetInfo != nil

	switch {
	case prgAccess && indexedUsage:
		name = fmt.Sprintf(dataNamingIndexed, address)
	case prgAccess && !indexedUsage:
		name = fmt.Sprintf(dataNaming, address)
	case !prgAccess && indexedUsage:
		name = fmt.Sprintf(variableNamingIndexed, address)
	default:
		name = fmt.Sprintf(variableNaming, address)
	}

	reference = name + reference
	if offsetInfo != nil {
		offsetInfo.Label = name
	}
	return name, reference
}

// setNameOfNextOffset checks whether the destination is inside the 2. or 3. byte of an instruction
// and points the label to the offset after it.
func (dis *Disasm) setNameOfNextOffset(address uint16) (*offset, uint16, string) {
	for i := address; i < 0xFFFA; i++ {
		offset := dis.addressToOffset(i)
		offsetInfo := &dis.offsets[offset]

		if offsetInfo.IsType(program.CodeOffset) && len(offsetInfo.OpcodeBytes) == 0 {
			continue
		}

		var adjuster string
		j := i - address
		if j > 0 {
			adjuster = fmt.Sprintf("-%d", j)
		}

		return offsetInfo, i, adjuster
	}
	panic("no offset for label found")
}
