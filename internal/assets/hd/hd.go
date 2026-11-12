// Package hd validates optional, source-bound RGBA artwork atomically.
package hd

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/amiga"
)

type Entry struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	SourceSHA string `json:"source_sha256"`
	Index     int    `json:"index"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Art       string `json:"art"`
	ArtSHA    string `json:"art_sha256"`
}

type Manifest struct {
	MonsterManifestSHA string  `json:"monster_manifest_sha256"`
	Version            int     `json:"version"`
	Source             string  `json:"source"`
	Scale              int     `json:"scale"`
	Expected           int     `json:"expected"`
	Generated          int     `json:"generated"`
	Entries            []Entry `json:"entries"`
	Frame              struct {
		Path    string `json:"path"`
		SHA     string `json:"sha256"`
		Page    string `json:"page"`
		PageSHA string `json:"page_sha256"`
	} `json:"frame"`
}

type Bundle struct {
	Frame, Page image.Image
	entries     map[string]image.Image
	byPixels    map[string]image.Image
	cache       map[*image.Paletted]image.Image
	Manifest    Manifest
	introMasks  [11]*image.Alpha
}

func safeFile(dir, name string) (string, error) {
	if name == "" || !filepath.IsLocal(name) || strings.Contains(name, "\\") {
		return "", fmt.Errorf("HD: invalid path %q", name)
	}
	path := filepath.Join(dir, name)
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	base, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, real)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("HD: path escapes package")
	}
	return path, nil
}

func checkedBytes(dir, name, hash string) ([]byte, error) {
	path, err := safeFile(dir, name)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 32<<20 {
		return nil, fmt.Errorf("HD: invalid file size %s", name)
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(blob)
	if hex.EncodeToString(sum[:]) != hash {
		return nil, fmt.Errorf("HD: SHA-256 mismatch %s", name)
	}
	return blob, nil
}

func checkedImage(dir, name, hash string, w, h int) (image.Image, error) {
	blob, err := checkedBytes(dir, name, hash)
	if err != nil {
		return nil, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(blob))
	if err != nil {
		return nil, err
	}
	if cfg.Width != w || cfg.Height != h || w <= 0 || h <= 0 || w*h > 4<<20 {
		return nil, fmt.Errorf("HD: invalid dimensions %s", name)
	}
	return png.Decode(bytes.NewReader(blob))
}

func parseScene(data []byte) (*amiga.Set, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("HD: truncated scene")
	}
	count := int(binary.BigEndian.Uint16(data))
	if count < 1 || count > 200 || len(data) < 4+count*6+64 {
		return nil, fmt.Errorf("HD: incompatible scene header")
	}
	for i := 0; i < count; i++ {
		w, h := int(binary.BigEndian.Uint16(data[4+i*6:])), int(binary.BigEndian.Uint16(data[6+i*6:]))
		if w < 8 || w > 400 || h < 4 || h > 300 {
			return nil, fmt.Errorf("HD: unsupported source dimensions")
		}
	}
	return amiga.Parse(data)
}

func Load(dir, amigaDir, monsterDir string) (*Bundle, error) {
	blob, err := os.ReadFile(filepath.Join(dir, "hd.json"))
	if err != nil {
		return nil, err
	}
	if len(blob) > 2<<20 {
		return nil, fmt.Errorf("HD: manifest too large")
	}
	var mf Manifest
	if err = json.Unmarshal(blob, &mf); err != nil {
		return nil, err
	}
	if mf.Version != 1 || mf.Source != "Amiga" || mf.Scale != 3 || mf.Expected <= 0 || mf.Expected > 4096 || mf.Generated != mf.Expected || len(mf.Entries) != mf.Expected {
		return nil, fmt.Errorf("HD: incomplete or incompatible package (%d/%d)", mf.Generated, mf.Expected)
	}
	b := &Bundle{entries: map[string]image.Image{}, byPixels: map[string]image.Image{}, cache: map[*image.Paletted]image.Image{}, Manifest: mf}
	if b.Frame, err = checkedImage(dir, mf.Frame.Path, mf.Frame.SHA, 960, 600); err != nil {
		return nil, err
	}
	if b.Page, err = checkedImage(dir, mf.Frame.Page, mf.Frame.PageSHA, 960, 600); err != nil {
		return nil, err
	}
	scenes := map[string]*amiga.Set{}
	sources := map[string]string{}
	artCache := map[string]image.Image{}
	seen := map[string]bool{}
	seenSource := map[string]bool{}
	pixels := 0
	for _, e := range mf.Entries {
		if e.ID == "" || seen[e.ID] || e.Index < 0 || e.Width <= 0 || e.Height <= 0 || e.Width > 400 || e.Height > 300 {
			return nil, fmt.Errorf("HD: invalid entry %s", e.ID)
		}
		seen[e.ID] = true
		identity := fmt.Sprintf("%s:%s:%d", e.Kind, e.Source, e.Index)
		if seenSource[identity] {
			return nil, fmt.Errorf("HD: duplicate source entry %s", identity)
		}
		seenSource[identity] = true
		var original image.Image
		switch e.Kind {
		case "scene":
			if filepath.Base(e.Source) != e.Source {
				return nil, fmt.Errorf("HD: invalid scene name")
			}
			if _, ok := scenes[e.Source]; !ok {
				data, err := checkedBytes(amigaDir, e.Source, e.SourceSHA)
				if err != nil {
					return nil, err
				}
				set, err := parseScene(data)
				if err != nil {
					return nil, err
				}
				scenes[e.Source], sources[e.Source] = set, e.SourceSHA
			}
			if sources[e.Source] != e.SourceSHA || e.Index >= len(scenes[e.Source].Images) {
				return nil, fmt.Errorf("HD: scene index mismatch %s", e.ID)
			}
			original = scenes[e.Source].Images[e.Index]
		case "monster":
			data, err := checkedBytes(monsterDir, e.Source, e.SourceSHA)
			if err != nil {
				return nil, err
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
			if cfg.Width != e.Width || cfg.Height != e.Height {
				return nil, fmt.Errorf("HD: monster dimensions %s", e.ID)
			}
			original, err = png.Decode(bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("HD: unknown entry kind %s", e.Kind)
		}
		if original.Bounds().Dx() != e.Width || original.Bounds().Dy() != e.Height {
			return nil, fmt.Errorf("HD: source dimensions %s", e.ID)
		}
		cacheKey := e.Art + ":" + e.ArtSHA
		art, ok := artCache[cacheKey]
		if !ok {
			art, err = checkedImage(dir, e.Art, e.ArtSHA, e.Width*3, e.Height*3)
			if err != nil {
				return nil, err
			}
			pixels += e.Width * e.Height * 9
			if pixels > 128<<20 {
				return nil, fmt.Errorf("HD: decoded artwork exceeds memory budget")
			}
			artCache[cacheKey] = art
		}
		if art.Bounds().Dx() != e.Width*3 || art.Bounds().Dy() != e.Height*3 {
			return nil, fmt.Errorf("HD: cached artwork size mismatch %s", e.ID)
		}
		b.entries[e.ID], b.byPixels[fingerprint(original)] = art, art
	}
	// Completeness comes from the real sources, not a self-declared counter.
	expected := map[string]bool{}
	names, err := filepath.Glob(filepath.Join(amigaDir, "*.32"))
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		base := filepath.Base(name)
		set := scenes[base]
		if set == nil {
			data, err := os.ReadFile(name)
			if err != nil {
				return nil, err
			}
			set, err = parseScene(data)
			if err != nil {
				continue
			}
		}
		for index := range set.Images {
			expected[fmt.Sprintf("scene:%s:%d", base, index)] = true
		}
	}
	meta, err := checkedBytes(monsterDir, "set.json", mf.MonsterManifestSHA)
	if err != nil {
		return nil, err
	}
	var monsters struct {
		Pictures []struct {
			Frames []string `json:"frames"`
		} `json:"pictures"`
	}
	if err = json.Unmarshal(meta, &monsters); err != nil {
		return nil, err
	}
	for _, pic := range monsters.Pictures {
		for index, name := range pic.Frames {
			expected[fmt.Sprintf("monster:%s:%d", name, index)] = true
		}
	}
	if len(expected) != mf.Expected {
		return nil, fmt.Errorf("HD: real source count %d differs from declared %d", len(expected), mf.Expected)
	}
	for identity := range expected {
		if !seenSource[identity] {
			return nil, fmt.Errorf("HD: missing real source %s", identity)
		}
	}
	return b, nil
}

// fingerprint deliberately ignores PNG palette encoding and transparency bytes;
// the original index mask is retained independently by the renderer.
func fingerprint(im image.Image) string {
	h := sha256.New()
	r := im.Bounds()
	binary.Write(h, binary.LittleEndian, uint32(r.Dx()))
	binary.Write(h, binary.LittleEndian, uint32(r.Dy()))
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			var c color.NRGBA
			if p, ok := im.(*image.Paletted); ok {
				c = color.NRGBAModel.Convert(p.Palette[p.ColorIndexAt(x, y)]).(color.NRGBA)
			} else {
				c = color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA)
			}
			// Source monster transparent pixels are defined as black in contact sheets.
			if c.A == 0 {
				c.R, c.G, c.B = 0, 0, 0
			}
			h.Write([]byte{c.R, c.G, c.B})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (b *Bundle) Art(im *image.Paletted) image.Image {
	if b == nil || im == nil {
		return nil
	}
	if art, ok := b.cache[im]; ok {
		return art
	}
	art := b.byPixels[fingerprint(im)]
	b.cache[im] = art
	return art
}

// Compose uses the approved frame geometry. All text and numbers come from src.
func (b *Bundle) Compose(dst *image.RGBA, page bool) {
	src := image.NewRGBA(dst.Bounds())
	copy(src.Pix, dst.Pix)
	frame := b.Frame
	if page {
		frame = b.Page
	}
	draw.Draw(dst, dst.Bounds(), frame, image.Point{}, draw.Src)
	if page {
		copyRegion(dst, image.Rect(44, 40, 919, 563), src, image.Rect(12, 12, 948, 570), false)
		return
	}
	copyRegion(dst, image.Rect(44, 40, 652, 375), src, image.Rect(24, 24, 648, 384), false)
	copyRegion(dst, image.Rect(680, 47, 913, 371), src, image.Rect(669, 18, 933, 378), true)
	from := []image.Rectangle{image.Rect(24, 405, 288, 435), image.Rect(312, 405, 501, 435), image.Rect(528, 405, 744, 435), image.Rect(774, 405, 939, 435), image.Rect(24, 447, 933, 558)}
	to := []image.Rectangle{image.Rect(44, 400, 252, 431), image.Rect(295, 400, 464, 431), image.Rect(501, 400, 701, 431), image.Rect(738, 400, 921, 431), image.Rect(54, 456, 912, 557)}
	for i := range from {
		copyRegion(dst, to[i], src, from[i], true)
	}
}

func copyRegion(dst *image.RGBA, to image.Rectangle, src *image.RGBA, from image.Rectangle, text bool) {
	for y := to.Min.Y; y < to.Max.Y; y++ {
		for x := to.Min.X; x < to.Max.X; x++ {
			c := src.RGBAAt(from.Min.X+(x-to.Min.X)*from.Dx()/to.Dx(), from.Min.Y+(y-to.Min.Y)*from.Dy()/to.Dy())
			if text {
				a := max(c.R, c.G, c.B)
				if a == 0 {
					continue
				}
				bg := dst.RGBAAt(x, y)
				c = color.RGBA{uint8(min(255, int(c.R)+int(bg.R)*(255-int(a))/255)), uint8(min(255, int(c.G)+int(bg.G)*(255-int(a))/255)), uint8(min(255, int(c.B)+int(bg.B)*(255-int(a))/255)), 255}
			}
			dst.SetRGBA(x, y, c)
		}
	}
}
