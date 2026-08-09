// Package font 解析 MM2.CH：原版的 8×8 點陣字型。
//
// 1,024 bytes = 8 bytes/字元 × 128 字元，每 byte 一列，MSB 在左。
// 字碼 32–127 是標準 ASCII 字形，0–31 放自訂繪圖符號。
// 詳見 docs/formats/02-data-files.md §2。
package font

import "fmt"

const (
	Glyphs     = 128
	GlyphW     = 8
	GlyphH     = 8
	bytesPer   = 8
	FileLength = Glyphs * bytesPer
)

type Font struct {
	rows [Glyphs][GlyphH]byte
}

func Parse(blob []byte) (*Font, error) {
	if len(blob) != FileLength {
		return nil, fmt.Errorf("MM2.CH 應為 %d bytes，實際 %d", FileLength, len(blob))
	}
	f := &Font{}
	for g := 0; g < Glyphs; g++ {
		copy(f.rows[g][:], blob[g*bytesPer:(g+1)*bytesPer])
	}
	return f, nil
}

// Pixel 回報字元 g 的 (x, y) 是否點亮。超出範圍一律回 false，
// 讓呼叫端不必先做邊界檢查。
func (f *Font) Pixel(g, x, y int) bool {
	if g < 0 || g >= Glyphs || x < 0 || x >= GlyphW || y < 0 || y >= GlyphH {
		return false
	}
	return f.rows[g][y]&(0x80>>uint(x)) != 0
}

// Row 回傳字元 g 第 y 列的位元圖樣。
func (f *Font) Row(g, y int) byte {
	if g < 0 || g >= Glyphs || y < 0 || y >= GlyphH {
		return 0
	}
	return f.rows[g][y]
}
