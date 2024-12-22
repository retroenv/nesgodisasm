package nesasm

import (
	"fmt"
	"io"

	"github.com/retroenv/nesgodisasm/internal/program"
)

const bankSize = 0x2000

// addPrgBankSelectors adds PRG bank selectors to every 0x2000 byte offsets, as required
// by nesasm to avoid the error "Bank overflow, offset > $1FFF".
func addPrgBankSelectors(codeBaseAddress int, prg []*program.PRGBank) int {
	counter := 0
	bankNumber := 0
	bankAddress := codeBaseAddress
	bankSwitch := true

	for _, bank := range prg {
		index := 0

		for {
			if bankSwitch { // if switch was carried over after last bank was filled
				setPrgBankSelector(bank.Offsets, index, &bankAddress, &bankNumber)
				bankSwitch = false
			}

			bankSpaceLeft := counter % bankSize
			if bankSpaceLeft == 0 {
				bankSpaceLeft = bankSize
			}

			bankBytesLeft := len(bank.Offsets[index:])
			if bankSpaceLeft > bankBytesLeft {
				counter += bankBytesLeft
				break
			}
			if bankSpaceLeft == bankBytesLeft {
				counter += bankBytesLeft
				bankSwitch = true
				break
			}

			setPrgBankSelector(bank.Offsets, index+bankSpaceLeft, &bankAddress, &bankNumber)

			index += bankSpaceLeft
			counter += bankSpaceLeft
		}
	}

	return bankNumber
}

// chrBanks adds CHR bank selectors to every 0x2000 byte offsets, as required
// by nesasm to avoid the error "Bank overflow, offset > $1FFF".
func chrBanks(nextBank int, chr program.CHR) []program.CHR {
	banks := make([]program.CHR, 0, len(chr)/bankSize)
	remaining := len(chr)

	for index := 0; remaining > 0; nextBank++ {
		toWrite := remaining
		if toWrite > bankSize {
			toWrite = bankSize
		}

		bank := chr[index : index+toWrite]
		//WriteCallback: writeBankSelector(nextBank, -1),
		banks = append(banks, bank)

		index += toWrite
		remaining -= toWrite
	}

	return banks
}

func setPrgBankSelector(prg []program.Offset, index int, bankAddress, bankNumber *int) {
	offsetInfo := &prg[index]

	// handle bank switches in the middle of an instruction by converting it to data bytes
	if offsetInfo.IsType(program.CodeOffset) && len(offsetInfo.Data) == 0 {
		// look backwards for instruction start
		instructionStartIndex := index - 1
		for offsetInfo = &prg[instructionStartIndex]; len(offsetInfo.Data) == 0; {
			instructionStartIndex--
			offsetInfo = &prg[instructionStartIndex]
		}

		offsetInfo.Comment = fmt.Sprintf("bank switch in instruction detected: %s %s",
			offsetInfo.Comment, offsetInfo.Code)
		data := offsetInfo.Data

		for i := range data {
			offsetInfo = &prg[instructionStartIndex+i]
			offsetInfo.Data = data[i : i+1]
			offsetInfo.ClearType(program.CodeOffset)
			offsetInfo.SetType(program.CodeAsData | program.DataOffset)
		}
		offsetInfo = &prg[index]
	}

	offsetInfo.WriteCallback = writeBankSelector(*bankNumber, *bankAddress)

	*bankAddress += bankSize
	*bankNumber++
}

func writeBankSelector(bankNumber, bankAddress int) func(writer io.Writer) error {
	return func(writer io.Writer) error {
		if _, err := fmt.Fprintf(writer, "\n .bank %d\n", bankNumber); err != nil {
			return fmt.Errorf("writing bank switch: %w", err)
		}

		if bankAddress >= 0 {
			if _, err := fmt.Fprintf(writer, " .org $%04x\n\n", bankAddress); err != nil {
				return fmt.Errorf("writing segment: %w", err)
			}
		}
		return nil
	}
}
