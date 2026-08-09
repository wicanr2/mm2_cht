package render

import (
	"image/color"

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
