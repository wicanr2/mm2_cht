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

// BlitKey 與 Blit 相同，但跳過等於 key 的像素。
//
// **透空色不是全域的，是看貼圖。** 牆的貼圖用色號 8（深灰）標出「這裡
// 看得到後面」——側牆矩形四角那兩塊楔形就是它，露出來的是天空與地板。
// 而同一個色號 8 在地板貼圖裡是真的顏色（灰白相間的格子），所以不能
// 在 `Blit` 裡一律跳過。
//
// 判準：把 `TOWN.16` 的影格 4 對回原版截圖，327 個不符的像素**全部**是
// 「樣板 8 → 截圖是天空或地板」，沒有一個例外。
func (s *Screen) BlitKey(src *image.Paletted, x, y int, key uint8) {
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
			if c := src.ColorIndexAt(sx, sy); c != key {
				s.Orig.SetColorIndex(dx, dy, c)
			}
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

// Fit 算出把整張畫面塞進 w×h 的等比例倍率與置中位移。
//
// 取兩軸較小的倍率，所以**長寬比永遠不變**（多出來的一邊留黑邊）。
// 原版像素與中文疊加層是同一張圖，倍率與位移對兩層一致 ——
// 中文不可能相對原版像素跑掉。這是把疊加層做在畫布裡而不是做在
// 視窗上的直接好處。
func Fit(w, h int) (scale, ox, oy float64) {
	if w < 1 || h < 1 {
		return 1, 0, 0
	}
	scale = float64(w) / float64(HiW)
	if sy := float64(h) / float64(HiH); sy < scale {
		scale = sy
	}
	if scale <= 0 {
		scale = 1
	}
	return scale, (float64(w) - float64(HiW)*scale) / 2,
		(float64(h) - float64(HiH)*scale) / 2
}
