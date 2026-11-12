// Package cjk 載入 tools/build_cjk_font.py 烘出來的中文點陣 atlas。
//
// 中文不走原版的 8×8 字位（縮到那個尺寸會糊成一團），而是獨立的高解析
// 點陣層，畫在放大後的畫布上。預設 24×24 剛好是原版一個字格放大 3 倍，
// 中英文的行距與基準線因此對得齊。
package cjk

import (
	"encoding/binary"
	"fmt"
	"sort"
)

var magic = [8]byte{'M', 'M', '2', 'C', 'J', 'K', 0, 0}

type Font struct {
	W, H     int
	rowBytes int
	codes    []rune // 已排序
	glyphs   []byte
	alpha    bool
	fallback *Font
}

func Parse(blob []byte) (*Font, error) {
	if len(blob) < 16 || (string(blob[:8]) != string(magic[:]) && string(blob[:8]) != "MM2CJA\x00\x00") {
		return nil, fmt.Errorf("不是 CJK atlas（magic 不符）")
	}
	f := &Font{
		W: int(binary.LittleEndian.Uint16(blob[8:])),
		H: int(binary.LittleEndian.Uint16(blob[10:])),
	}
	f.alpha = string(blob[:8]) == "MM2CJA\x00\x00"
	count := int(binary.LittleEndian.Uint32(blob[12:]))
	if f.W < 1 || f.H < 1 || f.W > 128 || f.H > 128 || count > 1<<20 {
		return nil, fmt.Errorf("invalid atlas dimensions or glyph count")
	}
	f.rowBytes = (f.W + 7) / 8
	if f.alpha {
		f.rowBytes = f.W
	}
	need := 16 + count*4 + count*f.rowBytes*f.H
	if len(blob) != need {
		return nil, fmt.Errorf("atlas 應為 %d bytes，實際 %d", need, len(blob))
	}
	f.codes = make([]rune, count)
	for i := 0; i < count; i++ {
		f.codes[i] = rune(binary.LittleEndian.Uint32(blob[16+i*4:]))
		if f.codes[i] < 0 || f.codes[i] > 0x10ffff || i > 0 && f.codes[i] <= f.codes[i-1] {
			return nil, fmt.Errorf("invalid atlas codepoint order")
		}
	}
	f.glyphs = blob[16+count*4:]
	return f, nil
}

// Has 回報 atlas 裡有沒有這個字。烘字型只收譯文用到的字，
// 譯文改了而沒重烘就會缺字 —— 呼叫端要能看出來，不能默默畫成空白。
func (f *Font) Has(r rune) bool {
	_, ok := f.index(r)
	return ok || f.fallback != nil && f.fallback.Has(r)
}

// WithFallback keeps existing glyph bytes and fills only missing codepoints.
func (f *Font) WithFallback(other *Font) *Font {
	if other == nil || f.W != other.W || f.H != other.H { return f }
	copy := *f
	copy.fallback = other
	return &copy
}

func (f *Font) index(r rune) (int, bool) {
	i := sort.Search(len(f.codes), func(i int) bool { return f.codes[i] >= r })
	if i < len(f.codes) && f.codes[i] == r {
		return i, true
	}
	return 0, false
}

// Pixel 回報字 r 的 (x, y) 是否點亮。缺字或超出範圍一律回 false。
func (f *Font) Pixel(r rune, x, y int) bool {
	return f.Coverage(r, x, y) > 0
}

// Coverage supports independent HD atlases without altering legacy bitmap data.
func (f *Font) Coverage(r rune, x, y int) uint8 {
	i, ok := f.index(r)
	if !ok && f.fallback != nil { return f.fallback.Coverage(r, x, y) }
	if !ok || x < 0 || x >= f.W || y < 0 || y >= f.H {
		return 0
	}
	if f.alpha {
		return f.glyphs[i*f.rowBytes*f.H+y*f.rowBytes+x]
	}
	b := f.glyphs[i*f.rowBytes*f.H+y*f.rowBytes+x/8]
	if b&(0x80>>uint(x%8)) != 0 {
		return 255
	}
	return 0
}

// Missing 回傳字串裡不在 atlas 內的字，供翻譯流程檢查。
func (f *Font) Missing(s string) []rune {
	var out []rune
	seen := map[rune]bool{}
	for _, r := range s {
		if r > 0x7F && !f.Has(r) && !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}
