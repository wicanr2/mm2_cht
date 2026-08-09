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
}

func Parse(blob []byte) (*Font, error) {
	if len(blob) < 16 || string(blob[:8]) != string(magic[:]) {
		return nil, fmt.Errorf("不是 CJK atlas（magic 不符）")
	}
	f := &Font{
		W: int(binary.LittleEndian.Uint16(blob[8:])),
		H: int(binary.LittleEndian.Uint16(blob[10:])),
	}
	count := int(binary.LittleEndian.Uint32(blob[12:]))
	f.rowBytes = (f.W + 7) / 8
	need := 16 + count*4 + count*f.rowBytes*f.H
	if len(blob) != need {
		return nil, fmt.Errorf("atlas 應為 %d bytes，實際 %d", need, len(blob))
	}
	f.codes = make([]rune, count)
	for i := 0; i < count; i++ {
		f.codes[i] = rune(binary.LittleEndian.Uint32(blob[16+i*4:]))
	}
	f.glyphs = blob[16+count*4:]
	return f, nil
}

// Has 回報 atlas 裡有沒有這個字。烘字型只收譯文用到的字，
// 譯文改了而沒重烘就會缺字 —— 呼叫端要能看出來，不能默默畫成空白。
func (f *Font) Has(r rune) bool {
	_, ok := f.index(r)
	return ok
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
	i, ok := f.index(r)
	if !ok || x < 0 || x >= f.W || y < 0 || y >= f.H {
		return false
	}
	b := f.glyphs[i*f.rowBytes*f.H+y*f.rowBytes+x/8]
	return b&(0x80>>uint(x%8)) != 0
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
