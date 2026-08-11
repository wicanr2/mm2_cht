package view_test

import (
	"image"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// 素材包（已放大 render.Scale 倍）畫出來要與原版逐格對齊。
//
// 幾何是算在原版座標上的：置中、貼齊底邊、鏡射全都要用素材的寬高。
// 拿放大後的寬高去算，每一個「除以二」都會差三倍，畫面整個往上往左散開
// —— 而那看起來像座標公式寫錯，不像素材選錯，會往完全錯的方向查。
//
// 判準用得起「完全相同」是因為 Scale3x **保留中心點**（3×3 的正中央就是
// 原本那一格），所以放大版每個區塊的中心必須等於原版那一格。
func TestPackSetAlignsWithOriginal(t *testing.T) {
	w := testWorld(t)
	w.MapIndex, w.X, w.Y, w.Face = 0, 8, 0, game.East

	orig := testTown(t)
	up := func(src []*image.Paletted) []*image.Paletted {
		out := make([]*image.Paletted, len(src))
		for i, im := range src {
			out[i] = render.Scale3x(im)
		}
		return out
	}
	pack := view.NewPackSet(view.PlatformModern,
		up(orig.Walls), up(orig.Floor), up(orig.Torch), up(orig.Sky), 8, 4)

	a := render.New(gfx.EGAPalette)
	view.DrawFirstPersonAt(a, w, orig, 0)
	a.Flush()

	b := render.New(gfx.EGAPalette)
	b.Clear(0)
	view.DrawFirstPersonAt(b, w, pack, 0)
	b.Flush()

	bad := 0
	for y := view.FPY; y < view.FPY+view.FPH; y++ {
		for x := view.FPX; x < view.FPX+view.FPW; x++ {
			c := render.Scale/2 + 1
			g, e := b.Hi.RGBAAt(x*render.Scale+c-1, y*render.Scale+c-1),
				a.Hi.RGBAAt(x*render.Scale+c-1, y*render.Scale+c-1)
			if g != e {
				if bad == 0 {
					t.Errorf("(%d,%d) 素材包畫成 %v，原版是 %v", x, y, g, e)
				}
				bad++
			}
		}
	}
	if bad > 0 {
		t.Errorf("視圖區 %d 格對不上（共 %d 格）", bad, view.FPW*view.FPH)
	}
}
