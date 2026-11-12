package hd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func testBytes(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func testPNG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			im.SetNRGBA(x, y, color.NRGBA{80, 120, 160, 255})
		}
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, im); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestLoadSourceIntegrity(t *testing.T) {
	root := t.TempDir()
	art := filepath.Join(root, "art")
	scenes := filepath.Join(root, "scenes")
	monsters := filepath.Join(root, "monsters")
	for _, dir := range []string{art, scenes, monsters} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	sourceHash := testPNG(t, monsters, "one.png", 8, 4)
	metaHash := testBytes(t, monsters, "set.json", []byte(`{"pictures":[{"frames":["one.png"]}]}`))
	m := Manifest{Version: 1, Source: "Amiga", Scale: 3, Expected: 1, Generated: 1, MonsterManifestSHA: metaHash}
	m.Frame.Path = "frame.png"
	m.Frame.SHA = testPNG(t, art, "frame.png", 960, 600)
	m.Frame.Page = "page.png"
	m.Frame.PageSHA = testPNG(t, art, "page.png", 960, 600)
	m.Entries = []Entry{{ID: "one", Kind: "monster", Source: "one.png", SourceSHA: sourceHash, Width: 8, Height: 4, Art: "one-hd.png", ArtSHA: testPNG(t, art, "one-hd.png", 24, 12)}}
	write := func() {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		testBytes(t, art, "hd.json", b)
	}
	write()
	b, err := Load(art, scenes, monsters)
	if err != nil {
		t.Fatal(err)
	}
	pal := color.Palette{color.NRGBA{80, 120, 160, 255}}
	im := image.NewPaletted(image.Rect(0, 0, 8, 4), pal)
	if b.Art(im) == nil || b.Art(im).Bounds().Dx() != 24 {
		t.Fatal("source image did not resolve to artwork")
	}
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"incomplete", func() { m.Generated = 0 }},
		{"uncovered source", func() {
			m.MonsterManifestSHA = testBytes(t, monsters, "set.json", []byte(`{"pictures":[{"frames":["one.png","missing.png"]}]}`))
		}},
		{"source hash", func() { m.Entries[0].SourceSHA = "bad" }},
		{"art hash", func() { m.Entries[0].ArtSHA = "bad" }},
		{"escaping path", func() { m.Entries[0].Art = "../one-hd.png" }},
		{"wrong size", func() { m.Entries[0].ArtSHA = testPNG(t, art, "one-hd.png", 23, 12) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := m
			old.Entries = append([]Entry(nil), m.Entries...)
			tc.change()
			write()
			if _, err := Load(art, scenes, monsters); err == nil {
				t.Fatal("invalid bundle accepted")
			}
			m = old
			testBytes(t, monsters, "set.json", []byte(`{"pictures":[{"frames":["one.png"]}]}`))
			testPNG(t, art, "one-hd.png", 24, 12)
		})
	}
}
