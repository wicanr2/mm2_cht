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
//
// Mask 是那張 1-bit 透空遮罩（沒有就是 nil ＝ 整塊不透空）。**透空是存出來的，
// 不是從顏色推的** —— 每列 `ceil(寬/8)` 個位元組、高位元對應左邊的像素、
// 位元為 1 表示這個像素不畫。
type Image struct {
	Width, Height int
	Pixels        []byte
	Mask          []byte
}

// MaskStride 是遮罩一列佔幾個位元組。
func (im Image) MaskStride() int { return (im.Width + 7) / 8 }

// Transparent 回報這個像素是不是透空。沒有遮罩一律回 false。
func (im Image) Transparent(x, y int) bool {
	if len(im.Mask) == 0 || x < 0 || y < 0 || x >= im.Width || y >= im.Height {
		return false
	}
	k := y*im.MaskStride() + x/8
	if k >= len(im.Mask) {
		return false
	}
	return im.Mask[k]&(0x80>>uint(x%8)) != 0
}

// Stencil 把遮罩攤成一張 alpha 圖：0 ＝透空、255 ＝要畫。沒有遮罩回 nil。
//
// 回 nil 而不是「整張 255」，是為了讓呼叫端分得出「這張不透空」與
// 「這張的遮罩還沒解析」——兩者畫出來一樣，但只有前者是原版的行為。
func (im Image) Stencil() *image.Alpha {
	if len(im.Mask) == 0 {
		return nil
	}
	dst := image.NewAlpha(image.Rect(0, 0, im.Width, im.Height))
	for y := 0; y < im.Height; y++ {
		for x := 0; x < im.Width; x++ {
			if !im.Transparent(x, y) {
				dst.SetAlpha(x, y, color.Alpha{A: 255})
			}
		}
	}
	return dst
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

// parseImages 解析影像集的檔頭。
//
//	uint16 count
//	count × { uint16 資料偏移; uint16 遮罩偏移 }   遮罩偏移 0 ＝整塊不透空
//
// 這個版面是從 EGA 驅動的貼圖常式讀出來的（`EGA.DRV` 的功能 0x13，
// 跳表第 19 項 → 0xCCA）：它拿 `影格 × 4 + 2` 當索引，第一個 word 是資料、
// 第二個是遮罩；遮罩非 0 就改走「每個位元平面都套同一列遮罩」的那條路。
// 影像本身的前四個位元組是寬與高，像素從第五個位元組起。
func parseImages(raw []byte) ([]Image, error) {
	if len(raw) < 6 {
		return nil, fmt.Errorf("影像集只有 %d bytes", len(raw))
	}
	count := int(binary.LittleEndian.Uint16(raw))
	if count == 0 || 2+count*4 > len(raw) {
		return nil, fmt.Errorf("影像數 %d 與緩衝長度 %d 不合", count, len(raw))
	}

	imgs := make([]Image, 0, count)
	for i := 0; i < count; i++ {
		base := int(binary.LittleEndian.Uint16(raw[2+i*4:]))
		mask := int(binary.LittleEndian.Uint16(raw[2+i*4+2:]))
		if base <= 0 || base+4 > len(raw) {
			continue
		}
		w := int(binary.LittleEndian.Uint16(raw[base:]))
		h := int(binary.LittleEndian.Uint16(raw[base+2:]))
		if w <= 0 || h <= 0 || base+4+(w*h+1)/2 > len(raw) {
			continue
		}
		im := Image{Width: w, Height: h, Pixels: raw[base+4:]}
		if mask > 0 {
			n := (w + 7) / 8 * h
			if mask+n <= len(raw) {
				im.Mask = raw[mask : mask+n]
			}
		}
		imgs = append(imgs, im)
	}
	if len(imgs) == 0 {
		return nil, fmt.Errorf("count=%d 但一張影像都解不出來", count)
	}
	return imgs, nil
}
