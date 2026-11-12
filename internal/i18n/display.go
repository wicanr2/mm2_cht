package i18n

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/cjk"
)

const (
	Traditional = "zh-Hant"
	Simplified  = "zh-Hans"
)

// Display is a presentation-only catalog. It never changes semantic game text.
type Display struct {
	labels              map[string]map[string]string
	convert             *strings.Replacer
	TC, SC, Sans, Serif *cjk.Font
	InputTC, InputSC    *cjk.Font
	InputAliases        *InputAliases
	toolbar             map[int]map[string]*cjk.Font
}

func LoadDisplay(root string) (*Display, error) {
	d := &Display{labels: map[string]map[string]string{Traditional: {}, Simplified: {}}}
	f, err := os.Open(filepath.Join(root, "translations/interface.tsv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.FieldsPerRecord = '\t', 3
	header, err := r.Read()
	if err != nil || strings.Join(header, ",") != "key,locale,text" {
		return nil, fmt.Errorf("interface.tsv: invalid header")
	}
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		catalog := d.labels[row[1]]
		if catalog == nil || row[0] == "" || row[2] == "" || catalog[row[0]] != "" {
			return nil, fmt.Errorf("interface.tsv: invalid or duplicate key %q", row[0])
		}
		catalog[row[0]] = row[2]
	}
	for key := range d.labels[Traditional] {
		if d.labels[Simplified][key] == "" {
			return nil, fmt.Errorf("interface.tsv: missing %s", key)
		}
	}
	if len(d.labels[Traditional]) != len(d.labels[Simplified]) {
		return nil, fmt.Errorf("interface.tsv: locale key mismatch")
	}
	blob, err := os.ReadFile(filepath.Join(root, "translations/simplified-map.json"))
	if err != nil {
		return nil, err
	}
	var pairs [][2]string
	if err := json.Unmarshal(blob, &pairs); err != nil {
		return nil, err
	}
	flat := make([]string, 0, len(pairs)*2)
	for _, pair := range pairs {
		if pair[0] == "" {
			return nil, fmt.Errorf("empty display conversion key")
		}
		flat = append(flat, pair[0], pair[1])
	}
	d.convert = strings.NewReplacer(flat...)
	for name, dest := range map[string]**cjk.Font{"tc": &d.TC, "sc": &d.SC, "sans": &d.Sans, "serif": &d.Serif} {
		blob, err := os.ReadFile(filepath.Join(root, "assets/font/display-"+name+"24.bin"))
		if err != nil {
			return nil, err
		}
		font, err := cjk.Parse(blob)
		if err != nil {
			return nil, err
		}
		*dest = font
	}
	for name, dest := range map[string]**cjk.Font{"tc": &d.InputTC, "sc": &d.InputSC} {
		blob, err := os.ReadFile(filepath.Join(root, "assets/font/input-"+name+"24.bin"))
		if err != nil {
			return nil, err
		}
		font, err := cjk.Parse(blob)
		if err != nil {
			return nil, err
		}
		*dest = font
	}
	d.TC, d.SC = d.TC.WithFallback(d.InputTC), d.SC.WithFallback(d.InputSC)
	d.InputAliases, err = LoadInputAliases(filepath.Join(root, "translations/input-aliases.tsv"))
	if err != nil {
		return nil, err
	}
	d.toolbar = map[int]map[string]*cjk.Font{}
	for _, size := range []int{24, 30, 36} {
		d.toolbar[size] = map[string]*cjk.Font{}
		for _, name := range []string{"tc", "sc", "sans"} {
			prefix := "display"
			if name == "sans" {
				prefix = "toolbar"
			}
			blob, err := os.ReadFile(filepath.Join(root, fmt.Sprintf("assets/font/%s-%s%d.bin", prefix, name, size)))
			if err != nil {
				return nil, err
			}
			font, err := cjk.Parse(blob)
			if err != nil {
				return nil, err
			}
			d.toolbar[size][name] = font
		}
	}
	return d, nil
}

func (d *Display) ToolbarFonts(locale string, percent int) (*cjk.Font, *cjk.Font) {
	size := 24 * percent / 100
	name := "tc"
	if locale == Simplified {
		name = "sc"
	}
	if fonts := d.toolbar[size]; fonts != nil {
		return fonts[name], fonts["sans"]
	}
	return d.Font(locale), d.Sans
}

func ValidLocale(locale string) bool { return locale == Traditional || locale == Simplified }

func (d *Display) Text(locale, key string) string {
	if d == nil {
		return key
	}
	if text := d.labels[locale][key]; text != "" {
		return text
	}
	return key
}

func (d *Display) Convert(locale, text string) string {
	if d == nil || locale != Simplified {
		return text
	}
	return d.convert.Replace(text)
}

func (d *Display) Font(locale string) *cjk.Font {
	if locale == Simplified {
		return d.SC
	}
	return d.TC
}
