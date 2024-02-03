// Package ini contains a generic ini configuration reader.
package ini

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

const (
	commentStart = ';'
	sectionStart = '['
	sectionEnd   = '['
)

type section struct {
	data map[string]any
}

// Ini defines a generic ini reader.
type Ini struct {
	sections map[string]*section

	currentSection *section
}

// Read the passed buffer into an ini structure.
func Read(input io.Reader) (Ini, error) {
	scanner := bufio.NewScanner(input)

	ini := Ini{
		sections: map[string]*section{},
	}

	for scanner.Scan() {
		line := scanner.Text()
		if err := ini.processLine(line); err != nil {
			return Ini{}, fmt.Errorf("processing rune: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return Ini{}, fmt.Errorf("reading line: %w", err)
	}

	return ini, nil
}

func (in *Ini) processLine(s string) error {
	for i, r := range s {
		switch {
		case unicode.IsSpace(r):
			continue

		case r == commentStart:
			return nil

		case r == sectionStart:
			return in.readSectionName(s[i+1:])

		case unicode.IsDigit(r), unicode.IsLetter(r):
			return in.readSectionLine(s)

		default:
			return fmt.Errorf("unexpected character '%c'", r)
		}
	}
	return nil
}

func (in *Ini) readSectionName(s string) error {
	for i, r := range s {
		if r != sectionEnd {
			continue
		}

		name := s[:i] // ignore all characters after section end
		name = strings.ToLower(strings.TrimSpace(name))

		_, ok := in.sections[name]
		if ok {
			return fmt.Errorf("duplicate section name '%s'", name)
		}

		sec := &section{
			data: map[string]any{},
		}
		in.sections[name] = sec
		in.currentSection = sec
		return nil
	}

	return errors.New("no section name delimiter ] found")
}

func (in *Ini) readSectionLine(s string) error {
	if in.currentSection == nil {
		return errors.New("no active section found")
	}

	parts := strings.SplitN(s, "=", 2)
	if len(parts) < 2 {
		return fmt.Errorf("invalid data definition '%s'", s)
	}

	address := strings.ToLower(strings.TrimSpace(parts[0]))
	if _, ok := in.currentSection.data[address]; ok {
		return fmt.Errorf("duplicate data definition for '%s'", address)
	}

	in.currentSection.data[address] = strings.TrimSpace(parts[1])

	return nil
}
