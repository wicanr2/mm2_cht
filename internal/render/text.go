package render

import (
	"image/color"

	"github.com/wicanr2/mm2_cht/internal/assets/cjk"
	"github.com/wicanr2/mm2_cht/internal/assets/font"
)

// DrawASCII 用原版的 8×8 字型把一行 ASCII 畫進原版層。
// 原版怎麼排版就怎麼排，中文不走這條路徑。
func (s *Screen) DrawASCII(f *font.Font, text string, x, y int, idx uint8) {
	for i, ch := range []byte(text) {
		gx := x + i*font.GlyphW
		for row := 0; row < font.GlyphH; row++ {
			bits := f.Row(int(ch), row)
			for col := 0; col < font.GlyphW; col++ {
				if bits&(0x80>>uint(col)) == 0 {
					continue
				}
				px, py := gx+col, y+row
				if px < 0 || px >= OrigW || py < 0 || py >= OrigH {
					continue
				}
				s.Orig.SetColorIndex(px, py, idx)
			}
		}
	}
}

// DrawASCIIHi 把原版字型畫進高解析層，每個原版像素放大成 Scale×Scale。
//
// 用途是讓英文與中文在同一層、同一個基準線上排版：中文走高解析點陣，
// 英文若還留在原版層，兩者的字距與基準線會對不齊。放大後的英文字形
// 仍然是原版的點陣，不是另一套字體。
func (s *Screen) DrawASCIIHi(f *font.Font, text string, x, y int, c color.RGBA) {
	for i, ch := range []byte(text) {
		gx := x + i*font.GlyphW*Scale
		for row := 0; row < font.GlyphH; row++ {
			bits := f.Row(int(ch), row)
			for col := 0; col < font.GlyphW; col++ {
				if bits&(0x80>>uint(col)) == 0 {
					continue
				}
				for dy := 0; dy < Scale; dy++ {
					for dx := 0; dx < Scale; dx++ {
						px, py := gx+col*Scale+dx, y+row*Scale+dy
						if px < 0 || px >= HiW || py < 0 || py >= HiH {
							continue
						}
						s.Hi.SetRGBA(px, py, c)
					}
				}
			}
		}
	}
}

// TextStyle 決定混排時各層怎麼畫。
type TextStyle struct {
	ASCII *font.Font // 原版 8×8 字型，放大 Scale 倍
	CJK   *cjk.Font  // 高解析中文點陣
	Color color.RGBA
}

// DrawText 在高解析層畫一段可能中英混排的文字。
//
// ASCII 走原版字型放大，中文走獨立點陣，兩者同在高解析層，
// 所以基準線一致、字距可以一起算。原版的換行符 '@' 在這裡斷行。
// 回傳畫完之後的下一行 y，方便連續輸出。
func (s *Screen) DrawText(st TextStyle, text string, x, y int) int {
	lineH := font.GlyphH * Scale
	if st.CJK != nil && st.CJK.H > lineH {
		lineH = st.CJK.H
	}
	cx, cy := x, y
	for _, r := range text {
		if r == LineBreakRune {
			cx, cy = x, cy+lineH
			continue
		}
		if r < 0x80 {
			if st.ASCII != nil {
				s.DrawASCIIHi(st.ASCII, string(byte(r)), cx, cy, st.Color)
			}
			cx += font.GlyphW * Scale
			continue
		}
		if st.CJK == nil {
			continue
		}
		for gy := 0; gy < st.CJK.H; gy++ {
			for gx := 0; gx < st.CJK.W; gx++ {
				if !st.CJK.Pixel(r, gx, gy) {
					continue
				}
				px, py := cx+gx, cy+gy
				if px < 0 || px >= HiW || py < 0 || py >= HiH {
					continue
				}
				s.Hi.SetRGBA(px, py, st.Color)
			}
		}
		cx += st.CJK.W
	}
	return cy + lineH
}

// LineBreakRune 是原版字串裡的換行符。
const LineBreakRune = '@'
