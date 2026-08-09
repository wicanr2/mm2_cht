// Package gfx 解析 MM2 的 .16 圖形檔。
//
// 格式與證據見 docs/formats/04-graphics.md。
package gfx

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

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
	offsets := make([]int, count)
	for i := range offsets {
		offsets[i] = int(binary.LittleEndian.Uint32(raw[2+i*4:]))
	}
	// offsets[0] 恆等於檔頭長度；不成立就是把別種檔案當影像集在解。
	if offsets[0] != 2+count*4 {
		return nil, fmt.Errorf("offsets[0] = %d，應為 %d（這不是影像集）", offsets[0], 2+count*4)
	}

	imgs := make([]Image, 0, count)
	for i, base := range offsets {
		if base+4 > len(raw) {
			return nil, fmt.Errorf("影像 %d 的偏移 %d 超出緩衝", i, base)
		}
		end := len(raw)
		if i+1 < count {
			end = offsets[i+1]
		}
		if end > len(raw) || end < base+4 {
			return nil, fmt.Errorf("影像 %d 的範圍 %d..%d 不合法", i, base, end)
		}
		imgs = append(imgs, Image{
			Width:  int(binary.LittleEndian.Uint16(raw[base:])),
			Height: int(binary.LittleEndian.Uint16(raw[base+2:])),
			Pixels: raw[base+4 : end],
		})
	}
	return imgs, nil
}
