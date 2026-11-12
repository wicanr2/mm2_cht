package i18n

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/cjk"
)

func TestLatinDisplayLettersShareBaselineAndHeight(t *testing.T) {
	d, err := LoadDisplay(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	bounds := func(f *cjk.Font, r rune) image.Rectangle {
		out := image.Rectangle{}
		for y := 0; y < f.H; y++ {
			for x := 0; x < f.W; x++ {
				if f.Coverage(r, x, y) > 32 {
					out = out.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
		return out
	}
	for _, percent := range []int{100, 125, 150} {
		cf := d.Font(Traditional)
		blob, err := os.ReadFile(filepath.Join("..", "..", "assets", "font", fmt.Sprintf("display-sans%d.bin", 24*percent/100)))
		if err != nil {
			t.Fatal(err)
		}
		lf, err := cjk.Parse(blob)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range "MHD0%" {
			b, a := bounds(lf, r), bounds(lf, 'A')
			if abs(b.Dy()-a.Dy()) > 1 || abs(b.Max.Y-a.Max.Y) > 1 {
				t.Fatalf("%d%%: wide/capital glyph %c shrank or left baseline: %v vs %v", percent, r, b, a)
			}
		}
		a, m, g := bounds(lf, 'A'), bounds(lf, 'm'), bounds(lf, 'g')
		if abs(a.Max.Y-m.Max.Y) > 1 || m.Min.Y <= a.Min.Y || g.Max.Y <= m.Max.Y+2 {
			t.Fatalf("%d%%: lowercase baseline/x-height/descender lost: A=%v m=%v g=%v", percent, a, m, g)
		}
		word := image.Rectangle{}
		for _, r := range "Amiga" {
			word = word.Union(bounds(lf, r))
		}
		if word.Min.Y < 1 || word.Max.Y > lf.H-1 || word.Dy()*5 < bounds(cf, '材').Dy()*4 {
			t.Fatalf("%d%%: Latin word height does not match CJK line: %v", percent, word)
		}
	}
	if a, m := bounds(d.Serif, 'A'), bounds(d.Serif, 'M'); abs(a.Dy()-m.Dy()) > 1 || abs(a.Max.Y-m.Max.Y) > 1 {
		t.Fatalf("HD serif capitals have inconsistent metrics: %v %v", a, m)
	}
}

func TestToolbarCapitalsMatchCJKVisibleHeight(t *testing.T) {
	d, err := LoadDisplay(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{Traditional, Simplified} {
		for _, percent := range []int{100, 125, 150} {
			cf, lf := d.ToolbarFonts(locale, percent)
			bounds := func(f *cjk.Font, r rune) image.Rectangle {
				var box image.Rectangle
				for y := 0; y < f.H; y++ {
					for x := 0; x < f.W; x++ {
						if f.Coverage(r, x, y) > 32 {
							box = box.Union(image.Rect(x, y, x+1, y+1))
						}
					}
				}
				return box
			}
			ref := bounds(cf, []rune(d.Text(locale, "toolbar.assets"))[0])
			for _, r := range "Amiga Mega Drive DOS MSX HD 0123456789%" {
				if r == ' ' {
					continue
				}
				b := bounds(lf, r)
				if abs(b.Dy()-ref.Dy()) > 1 || abs(b.Min.Y-ref.Min.Y) > 1 {
					t.Fatalf("%s %d%%: %c ink %v differs from CJK %v", locale, percent, r, b, ref)
				}
				if b.Min.Y < 1 || b.Max.Y > lf.H-1 {
					t.Fatalf("toolbar glyph clips: %c %v", r, b)
				}
			}
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestDisplayCatalogKeepsKeysFormatsAndGlyphs(t *testing.T) {
	root := filepath.Join("..", "..")
	d, err := LoadDisplay(root)
	if err != nil {
		t.Fatal(err)
	}
	format := regexp.MustCompile(`%[-+0-9.]*[a-zA-Z]|@|\n`)
	for _, name := range []string{"zh-Hant.json", "md-flavor.json"} {
		b, err := os.ReadFile(filepath.Join(root, "translations", name))
		if err != nil {
			t.Fatal(err)
		}
		var source []struct {
			Key    string `json:"key"`
			SHA    string `json:"src_sha8"`
			Target string `json:"target"`
		}
		if err = json.Unmarshal(b, &source); err != nil {
			t.Fatal(err)
		}
		scName := "zh-Hans.json"
		if name == "md-flavor.json" {
			scName = "md-flavor-zh-Hans.json"
		}
		b, err = os.ReadFile(filepath.Join(root, "translations", scName))
		if err != nil {
			t.Fatal(err)
		}
		var target []struct {
			Key    string `json:"key"`
			SHA    string `json:"src_sha8"`
			Target string `json:"target"`
		}
		if err = json.Unmarshal(b, &target); err != nil {
			t.Fatal(err)
		}
		if len(source) != len(target) {
			t.Fatalf("%s: entry count changed", name)
		}
		for i, s := range source {
			got := d.Convert(Simplified, s.Target)
			if target[i].Key != s.Key || target[i].SHA != s.SHA || target[i].Target != got {
				t.Fatalf("conversion identity mismatch %s", s.Key)
			}
			if len(d.SC.Missing(got)) != 0 || len(d.TC.Missing(s.Target)) != 0 {
				t.Fatalf("missing glyph %s", s.Key)
			}
			a, b := format.FindAllString(s.Target, -1), format.FindAllString(got, -1)
			if len(a) != len(b) {
				t.Fatalf("format count changed %s", s.Key)
			}
			for n := range a {
				if a[n] != b[n] {
					t.Fatalf("format changed %s", s.Key)
				}
			}
		}
	}
	text := "MM2.EXE str.001 %d @ WAFE Preamble"
	if d.Convert(Simplified, text) != text {
		t.Fatal("semantic ASCII changed")
	}
	for locale, labels := range d.labels {
		for key, label := range labels {
			if missing := d.Font(locale).Missing(label); len(missing) > 0 {
				t.Fatalf("%s %s: %U", locale, key, missing)
			}
		}
	}
}
