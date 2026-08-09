// Package gfx 解析 MM2 的 .16 圖形檔。
//
// 格式與證據見 docs/formats/04-graphics.md。
package gfx

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"sort"

	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

// EGAPalette 是 EGA 預設 16 色。
//
// 原版開機是否整組換掉調色盤尚未確認（姊妹專案冬之魔在這件事上解錯兩次，
// 教訓是「檔裡沒有調色盤不代表用標準表」）。找到 SETRGBPALETTE 一類的呼叫
// 之前，這裡先用標準值。
var EGAPalette = color.Palette{
	color.RGBA{0x00, 0x00, 0x00, 0xFF}, color.RGBA{0x00, 0x00, 0xAA, 0xFF},
	color.RGBA{0x00, 0xAA, 0x00, 0xFF}, color.RGBA{0x00, 0xAA, 0xAA, 0xFF},
	color.RGBA{0xAA, 0x00, 0x00, 0xFF}, color.RGBA{0xAA, 0x00, 0xAA, 0xFF},
	color.RGBA{0xAA, 0x55, 0x00, 0xFF}, color.RGBA{0xAA, 0xAA, 0xAA, 0xFF},
	color.RGBA{0x55, 0x55, 0x55, 0xFF}, color.RGBA{0x55, 0x55, 0xFF, 0xFF},
	color.RGBA{0x55, 0xFF, 0x55, 0xFF}, color.RGBA{0x55, 0xFF, 0xFF, 0xFF},
	color.RGBA{0xFF, 0x55, 0x55, 0xFF}, color.RGBA{0xFF, 0x55, 0xFF, 0xFF},
	color.RGBA{0xFF, 0xFF, 0x55, 0xFF}, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF},
}

// Image 是一張 4bpp packed 的原版影像。Pixels 保留原始位元組，
// 未知的尾端資料一併留著，讓未解的欄位能原樣往返。
type Image struct {
	Width, Height int
	Pixels        []byte
}

// Set 把 4bpp packed 展開成 Go 的 image.Paletted。
func (im Image) Paletted(pal color.Palette) *image.Paletted {
	dst := image.NewPaletted(image.Rect(0, 0, im.Width, im.Height), pal)
	perRow := (im.Width + 1) / 2
	for y := 0; y < im.Height; y++ {
		for x := 0; x < im.Width; x++ {
			k := y*perRow + x/2
			if k >= len(im.Pixels) {
				continue
			}
			b := im.Pixels[k]
			idx := b >> 4
			if x%2 == 1 {
				idx = b & 0xF
			}
			dst.SetColorIndex(x, y, idx)
		}
	}
	return dst
}

// ParseSet 解析一般的 .16（單一 LZW 段，段內是影像集）。
func ParseSet(blob []byte) ([]Image, error) {
	raw, err := lzw.Segment(blob, 0)
	if err != nil {
		return nil, err
	}
	return parseImages(raw)
}

func parseImages(raw []byte) ([]Image, error) {
	if len(raw) < 6 {
		return nil, fmt.Errorf("影像集只有 %d bytes", len(raw))
	}
	count := int(binary.LittleEndian.Uint16(raw))
	if count == 0 || 2+count*4 > len(raw) {
		return nil, fmt.Errorf("影像數 %d 與緩衝長度 %d 不合", count, len(raw))
	}
	offsets := readOffsets(raw, count)
	if len(offsets) == 0 {
		return nil, fmt.Errorf("解不出影像偏移（count=%d，緩衝 %d bytes）", count, len(raw))
	}

	imgs := make([]Image, 0, count)
	for i, base := range offsets {
		if base+4 > len(raw) {
			break
		}
		end := len(raw)
		if i+1 < len(offsets) {
			end = offsets[i+1]
		}
		w := int(binary.LittleEndian.Uint16(raw[base:]))
		h := int(binary.LittleEndian.Uint16(raw[base+2:]))
		// 影像結尾與下一個偏移之間固定空 4 bytes。資料不夠畫滿宣告的寬高，
		// 就是把非影像的偏移當成影像了 —— 跳過，不要產生垃圾。
		if w == 0 || h == 0 || end < base+4 || (w*h+1)/2 > end-base-4 {
			continue
		}
		imgs = append(imgs, Image{Width: w, Height: h, Pixels: raw[base+4 : end]})
	}
	if len(imgs) == 0 {
		return nil, fmt.Errorf("count=%d 但一張影像都解不出來", count)
	}
	return imgs, nil
}

// readOffsets 處理兩種檔頭。兩者的長度都是 2 + count*4，所以不能靠長度分辨。
//
//	A 型：uint32 offsets[count]                            TOWNT、DISK、NWCP…
//	B 型：uint16 offsetsA[count] + uint16 offsetsB[count]  MASTER、地形圖
//
// 判準是「當 uint32 讀出來的偏移是否遞增且落在緩衝內」：A 型的高位 word
// 剛好都是 0，B 型這樣讀會得到幾百萬的巨值。
func readOffsets(raw []byte, count int) []int {
	u32 := make([]int, count)
	ok := true
	for i := range u32 {
		u32[i] = int(binary.LittleEndian.Uint32(raw[2+i*4:]))
		if u32[i] <= 0 || u32[i] > len(raw) || (i > 0 && u32[i] <= u32[i-1]) {
			ok = false
		}
	}
	if ok {
		return u32
	}

	// B 型：兩組偏移在緩衝裡是交錯的，合併排序後才能用相鄰值界定邊界。
	seen := map[int]bool{}
	var out []int
	for i := 0; i < count*2; i++ {
		v := int(binary.LittleEndian.Uint16(raw[2+i*2:]))
		if v > 0 && v <= len(raw)-4 && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Ints(out)
	return out
}
