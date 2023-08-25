package nesasm

import (
	"fmt"
	"io"

	"github.com/retroenv/nesgodisasm/internal/program"
)

const bankSize = 0x2000

// addBankSelectors adds bank selectors to every 0x2000 byte offsets, as required
// by nesasm to avoid the error "Bank overflow, offset > $1FFF".
func addBankSelectors(codeBaseAddress int, prg []*program.PRGBank) int {
	counter := 0
	bankNumber := 0
	bankAddress := codeBaseAddress
	bankSwitch := true

	for _, bank := range prg {
		index := 0

		for {
			if bankSwitch { // if switch was carried over after last bank was filled
				setBankSelector(bank.PRG, index, &bankAddress, &bankNumber)
				bankSwitch = false
			}

			bankSpaceLeft := counter % bankSize
			if bankSpaceLeft == 0 {
				bankSpaceLeft = bankSize
			}

			bankBytesLeft := len(bank.PRG[index:])
			if bankSpaceLeft > bankBytesLeft {
				counter += bankBytesLeft
				break
			}
			if bankSpaceLeft == bankBytesLeft {
				counter += bankBytesLeft
				bankSwitch = true
				break
			}

			setBankSelector(bank.PRG, index+bankSpaceLeft, &bankAddress, &bankNumber)

			index += bankSpaceLeft
			counter += bankSpaceLeft
		}
	}

	return bankNumber
}

func setBankSelector(prg []program.Offset, index int, bankAddress, bankNumber *int) {
	offsetInfo := &prg[index]

	// handle bank switches in the middle of an instruction by converting it to data bytes
	if offsetInfo.IsType(program.CodeOffset) && len(offsetInfo.OpcodeBytes) == 0 {
		// look backwards for instruction start
		instructionStartIndex := index - 1
		for offsetInfo = &prg[instructionStartIndex]; len(offsetInfo.OpcodeBytes) == 0; {
			instructionStartIndex--
			offsetInfo = &prg[instructionStartIndex]
		}

		offsetInfo.Comment = fmt.Sprintf("bank switch in instruction detected: %s %s",
			offsetInfo.Comment, offsetInfo.Code)
		data := offsetInfo.OpcodeBytes

		for i := 0; i < len(data); i++ {
			offsetInfo = &prg[instructionStartIndex+i]
			offsetInfo.OpcodeBytes = data[i : i+1]
			offsetInfo.ClearType(program.CodeOffset)
			offsetInfo.SetType(program.CodeAsData | program.DataOffset)
		}
		offsetInfo = &prg[index]
	}

	offsetInfo.WriteCallback = writeBankSelector(*bankAddress, *bankNumber)

	*bankAddress += bankSize
	*bankNumber++
}

func writeBankSelector(bankAddress, bankNumber int) func(writer io.Writer) error {
	return func(writer io.Writer) error {
		if _, err := fmt.Fprintf(writer, "\n .bank %d\n", bankNumber); err != nil {
			return fmt.Errorf("writing bank switch: %w", err)
		}
		if _, err := fmt.Fprintf(writer, " .org $%04x\n\n", bankAddress); err != nil {
			return fmt.Errorf("writing segment: %w", err)
		}
		return nil
	}
}
