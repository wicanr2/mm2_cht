// Package render 管理兩層畫布。
//
// 原版是 320×200 的 EGA 畫面。中文筆畫多，縮到原版的 8×8 字位會糊成一團，
// 所以採「原版像素層維持原解析度、整數倍 nearest 放大，中文另走高解析層」
// 的分工（見 CLAUDE.md §6）。這裡把兩層的座標關係固定下來，
// 避免後續各處各自換算。
package render

import (
	"image"
	"image/color"
)

const (
	// OrigW/OrigH 是原版畫面的邏輯尺寸，不隨顯示倍率改變。
	OrigW, OrigH = 320, 200
	// Scale 是原版像素層放大的整數倍率。非整數倍即使用 nearest 也會
	// 出現寬窄不一的像素，所以只允許整數。
	Scale = 3
	// HiW/HiH 是高解析層（也就是實際輸出）的尺寸。
	HiW, HiH = OrigW * Scale, OrigH * Scale
)

// Screen 疊合兩層：Orig 是原版 EGA 像素，Hi 是放大後的輸出。
// 中文字直接畫進 Hi，不經過 Orig，才不會被放大成馬賽克。
type Screen struct {
	Orig *image.Paletted
	Hi   *image.RGBA
}

func New(pal color.Palette) *Screen {
	return &Screen{
		Orig: image.NewPaletted(image.Rect(0, 0, OrigW, OrigH), pal),
		Hi:   image.NewRGBA(image.Rect(0, 0, HiW, HiH)),
	}
}

// Clear 把原版層填成指定的調色盤索引。
func (s *Screen) Clear(idx uint8) {
	for i := range s.Orig.Pix {
		s.Orig.Pix[i] = idx
	}
}

// Blit 把一張原版影像畫到原版層的 (x, y)。
func (s *Screen) Blit(src *image.Paletted, x, y int) {
	b := src.Bounds()
	for sy := 0; sy < b.Dy(); sy++ {
		dy := y + sy
		if dy < 0 || dy >= OrigH {
			continue
		}
		for sx := 0; sx < b.Dx(); sx++ {
			dx := x + sx
			if dx < 0 || dx >= OrigW {
				continue
			}
			s.Orig.SetColorIndex(dx, dy, src.ColorIndexAt(sx, sy))
		}
	}
}

// Flush 把原版層以 nearest-neighbor 放大到高解析層。
// 一律在畫中文之前呼叫：中文層畫完再 Flush 會把中文洗掉。
func (s *Screen) Flush() {
	pal := s.Orig.Palette
	for y := 0; y < OrigH; y++ {
		for x := 0; x < OrigW; x++ {
			c := pal[s.Orig.ColorIndexAt(x, y)]
			r, g, b, a := c.RGBA()
			px := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
			for dy := 0; dy < Scale; dy++ {
				for dx := 0; dx < Scale; dx++ {
					s.Hi.SetRGBA(x*Scale+dx, y*Scale+dy, px)
				}
			}
		}
	}
}
