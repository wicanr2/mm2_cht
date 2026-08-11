package render_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/render"
)

func pal4() color.Palette {
	return color.Palette{
		color.RGBA{0, 0, 0, 255},
		color.RGBA{255, 255, 255, 255},
		color.RGBA{255, 0, 0, 255},
		color.RGBA{0, 255, 0, 255},
	}
}

func from(rows []string) *image.Paletted {
	im := image.NewPaletted(image.Rect(0, 0, len(rows[0]), len(rows)), pal4())
	for y, r := range rows {
		for x, c := range r {
			im.SetColorIndex(x, y, uint8(c-'0'))
		}
	}
	return im
}

func dump(im *image.Paletted) []string {
	b := im.Bounds()
	out := make([]string, b.Dy())
	for y := 0; y < b.Dy(); y++ {
		row := make([]byte, b.Dx())
		for x := 0; x < b.Dx(); x++ {
			row[x] = '0' + im.ColorIndexAt(b.Min.X+x, b.Min.Y+y)
		}
		out[y] = string(row)
	}
	return out
}

func TestScale3xSize(t *testing.T) {
	got := render.Scale3x(from([]string{"01", "10"}))
	if b := got.Bounds(); b.Dx() != 6 || b.Dy() != 6 {
		t.Fatalf("尺寸 %v，要 6×6", b)
	}
	if render.Scale3x(nil) != nil {
		t.Fatal("nil 要回 nil")
	}
}

// 純色放大之後還是純色 —— 演算法不會在平坦區憑空長出東西。
func TestScale3xFlatStaysFlat(t *testing.T) {
	got := render.Scale3x(from([]string{"222", "222", "222"}))
	for _, r := range dump(got) {
		for _, c := range r {
			if c != '2' {
				t.Fatalf("純色區出現色號 %c：\n%v", c, dump(got))
			}
		}
	}
}

// **不引入新顏色**是選 Scale3x 而不選內插的理由，也是透明色不會被
// 混成雜色的保證。這一條若壞了，牆面四角的透空會出現半透明邊。
func TestScale3xIntroducesNoNewColors(t *testing.T) {
	src := from([]string{
		"0011",
		"0231",
		"1320",
		"1100",
	})
	seen := map[uint8]bool{}
	b := src.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			seen[src.ColorIndexAt(x, y)] = true
		}
	}
	got := render.Scale3x(src)
	gb := got.Bounds()
	for y := 0; y < gb.Dy(); y++ {
		for x := 0; x < gb.Dx(); x++ {
			if c := got.ColorIndexAt(x, y); !seen[c] {
				t.Fatalf("(%d,%d) 出現來源沒有的色號 %d", x, y, c)
			}
		}
	}
}

// 斜邊會被補成階梯狀的斜角，而不是放大三倍的方塊。
// 判準取中心像素的左上角：對角相同（上與左都是 1）時該被填成 1。
func TestScale3xFillsDiagonalCorner(t *testing.T) {
	got := dump(render.Scale3x(from([]string{
		"010",
		"100",
		"000",
	})))
	// 中心 (1,1) 是 0，其上 (1,0)=1、其左 (0,1)=1 → 左上角補 1。
	if got[3][3] != '1' {
		t.Fatalf("左上角沒補成 1：\n%v", got)
	}
	// 而它的右下角兩側都是 0，維持 0。
	if got[5][5] != '0' {
		t.Fatalf("右下角不該被填：\n%v", got)
	}
}

// 邊界的鄰居用 clamp。若改成補 0，第一列會沿著整張圖長出一條假邊。
func TestScale3xEdgeClampNoFalseBorder(t *testing.T) {
	got := dump(render.Scale3x(from([]string{"11", "11"})))
	for y, r := range got {
		for x, c := range r {
			if c != '1' {
				t.Fatalf("(%d,%d) 邊界長出 %c：\n%v", x, y, c, got)
			}
		}
	}
}
