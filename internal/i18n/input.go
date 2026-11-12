package i18n

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// InputAliases translates explicit player input into a stable original answer.
// Display translations never participate in this lookup.
type InputAliases struct {
	answers map[string]map[string]string
}

func LoadInputAliases(path string) (*InputAliases, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.FieldsPerRecord = '\t', 3
	header, err := r.Read()
	if err != nil || strings.Join(header, ",") != "canonical,locale,text" {
		return nil, fmt.Errorf("input aliases: invalid header")
	}
	a := &InputAliases{answers: map[string]map[string]string{}}
	seen := map[string]bool{}
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		canonical := strings.ToUpper(row[0])
		key := canonical + "\x00" + row[1]
		if row[0] == "" || row[0] != canonical || !ValidLocale(row[1]) || !utf8.ValidString(row[2]) || strings.TrimSpace(row[2]) != row[2] || row[2] == "" || seen[key] {
			return nil, fmt.Errorf("input aliases: invalid or duplicate entry %q", key)
		}
		for _, ch := range canonical {
			if ch < '0' || ch > '9' && ch < 'A' || ch > 'Z' {
				return nil, fmt.Errorf("input aliases: invalid canonical answer")
			}
		}
		seen[key] = true
		if a.answers[canonical] == nil {
			a.answers[canonical] = map[string]string{}
		}
		a.answers[canonical][row[1]] = row[2]
	}
	for canonical, locales := range a.answers {
		if len(locales) != 2 {
			return nil, fmt.Errorf("input aliases: incomplete locales for %s", canonical)
		}
	}
	return a, nil
}

// NormalizeInputWidth handles fullwidth ASCII emitted by CJK input methods.
func NormalizeInputWidth(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= '\uff01' && r <= '\uff5e' {
			return r - 0xfee0
		}
		if r == '\u3000' {
			return ' '
		}
		return r
	}, text)
}

func (a *InputAliases) Resolve(text, expected string) string {
	text = strings.TrimSpace(NormalizeInputWidth(text))
	if a != nil {
		canonical := strings.ToUpper(expected)
		for _, alias := range a.answers[canonical] {
			if text == alias {
				return canonical
			}
		}
	}
	return text
}

func (a *InputAliases) Label(expected, locale string) string {
	if a != nil {
		if label := a.answers[strings.ToUpper(expected)][locale]; label != "" {
			return label
		}
	}
	return expected
}
