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
	ASCII *font.Font // 原版 8×8 字型，放大 Scale 倍（Latin 為 nil 時的退路）
	CJK   *cjk.Font  // 高解析中文點陣，全形
	// Latin 是高解析的英數字點陣，半形。
	//
	// 原版的 8×8 字型放大三倍是 3×3 的方塊像素，與 24×24 原生的中文
	// 擺在一起明顯粗一截。改用同一套字型烘出來的拉丁字母之後筆畫粗細
	// 一致，寬度取中文的一半 —— 漢字全形、拉丁半形是中日韓排版的慣例。
	//
	// 沒有這份 atlas 時退回 ASCII 那條路，畫面仍然可讀。
	Latin *cjk.Font
	Color color.RGBA
}

// Advance 是一個字畫完之後 x 要前進多少（高解析像素）。
//
// **排版估算一律走這一支**：折行、補位、欄寬如果自己算一套，
// 就會與實際畫出來的位置漂移，而漂移的症狀（欄位對不齊、行提早斷）
// 看起來像版面沒調好，不像有兩套互相矛盾的寬度定義。
func (st TextStyle) Advance(r rune) int {
	if r < 0x80 {
		if st.Latin != nil {
			return st.Latin.W
		}
		return font.GlyphW * Scale
	}
	if st.CJK != nil {
		return st.CJK.W
	}
	return font.GlyphW * Scale
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
		f := st.CJK
		if r < 0x80 {
			if st.Latin == nil {
				if st.ASCII != nil {
					s.DrawASCIIHi(st.ASCII, string(byte(r)), cx, cy, st.Color)
				}
				cx += st.Advance(r)
				continue
			}
			f = st.Latin
		}
		if f == nil {
			continue
		}
		s.blitGlyph(f, r, cx, cy, st.Color)
		cx += st.Advance(r)
	}
	return cy + lineH
}

// LineBreakRune 是原版字串裡的換行符。
const LineBreakRune = '@'

// blitGlyph 把 atlas 裡的一個字畫進高解析層。
func (s *Screen) blitGlyph(f *cjk.Font, r rune, x, y int, c color.RGBA) {
	for gy := 0; gy < f.H; gy++ {
		for gx := 0; gx < f.W; gx++ {
			if !f.Pixel(r, gx, gy) {
				continue
			}
			px, py := x+gx, y+gy
			if px < 0 || px >= HiW || py < 0 || py >= HiH {
				continue
			}
			s.Hi.SetRGBA(px, py, c)
		}
	}
}
