package nesasm

import (
	"fmt"
	"io"

	"github.com/retroenv/nesgodisasm/internal/program"
)

// addBankSelectors adds bank selectors to every 0x2000 byte offsets, as required
// by nesasm to avoid the error "Bank overflow, offset > $1FFF".
func addBankSelectors(codeBaseAddress int, prg []*program.PRGBank) int {
	counter := 0
	bankNumber := 0
	bankAddress := codeBaseAddress
	bankSize := 0x2000
	bankSwitch := true

	for _, bank := range prg {
		index := 0

		for {
			if bankSwitch { // if switch was carried over after last bank was filled
				bank.PRG[index].WriteCallback = writeBankSelector(bankAddress, bankNumber)
				bankAddress += bankSize
				bankNumber++
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

			bank.PRG[index+bankSpaceLeft].WriteCallback = writeBankSelector(bankAddress, bankNumber)
			bankAddress += bankSize
			bankNumber++

			index += bankSpaceLeft
			counter += bankSpaceLeft
		}
	}

	return bankNumber
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
