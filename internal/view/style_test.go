package view_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// sentinel 是原版層的哨兵色。用一個場景裡不會出現的索引，
// 「這一格沒被畫過」才分得出來。
const sentinel = 15

func townAt(t *testing.T) (*render.Screen, *game.World, *view.TownSet) {
	t.Helper()
	w := testWorld(t)
	tw := testTown(t)
	w.MapIndex, w.X, w.Y, w.Face = 0, 7, 5, game.North
	return render.New(gfx.EGAPalette), w, tw
}

// StyleModern 底下**不准有任何東西留在原版層**。
//
// 這條擋的是「有人加了一張新貼圖，直接呼叫 s.Blit」：畫面照樣出得來，
// 只是那一塊在放大之後是馬賽克，而且被 Flush 蓋在高解析貼圖下面 ——
// 沒有錯誤訊息，看起來只是「那一塊沒放大成功」。
func TestModernStyleLeavesOrigLayerUntouched(t *testing.T) {
	s, w, tw := townAt(t)
	tw.Style = view.StyleModern
	s.Clear(sentinel)
	view.DrawFirstPerson(s, w, tw)

	for y := view.FPY; y < view.FPY+view.FPH; y++ {
		for x := view.FPX; x < view.FPX+view.FPW; x++ {
			if c := s.Orig.ColorIndexAt(x, y); c != sentinel {
				t.Fatalf("(%d,%d) 被畫進原版層（色號 %d）—— 有貼圖沒走 TownSet.blit", x, y, c)
			}
		}
	}
}

// 反過來：StyleClassic 一定要畫在原版層，不然切回去會是空白。
func TestClassicStyleDrawsToOrigLayer(t *testing.T) {
	s, w, tw := townAt(t)
	tw.Style = view.StyleClassic
	s.Clear(sentinel)
	view.DrawFirstPerson(s, w, tw)

	painted := 0
	for y := view.FPY; y < view.FPY+view.FPH; y++ {
		for x := view.FPX; x < view.FPX+view.FPW; x++ {
			if s.Orig.ColorIndexAt(x, y) != sentinel {
				painted++
			}
		}
	}
	if painted < view.FPW*view.FPH/2 {
		t.Fatalf("原版層只畫了 %d/%d 個像素", painted, view.FPW*view.FPH)
	}
}

// 兩種風格畫出來的**輪廓**要一致：換風格換的是放大方式，不是畫面內容。
//
// 比法是把高解析層降回原版解析度取每格左上角的顏色。Scale3x 只在色塊
// 邊界動手腳，所以格子的左上角在絕大多數位置會相同；門檻取 90%，
// 剩下的落差就是被補斜角的邊界。
func TestModernKeepsSameSilhouette(t *testing.T) {
	s, w, tw := townAt(t)

	tw.Style = view.StyleClassic
	s.Clear(0)
	view.DrawFirstPerson(s, w, tw)
	s.Flush()
	classic := make([][4]uint32, 0, view.FPW*view.FPH)
	for y := view.FPY; y < view.FPY+view.FPH; y++ {
		for x := view.FPX; x < view.FPX+view.FPW; x++ {
			r, g, b, a := s.Hi.RGBAAt(x*render.Scale, y*render.Scale).RGBA()
			classic = append(classic, [4]uint32{r, g, b, a})
		}
	}

	tw.Style = view.StyleModern
	s.Clear(0)
	view.DrawFirstPerson(s, w, tw)
	s.Flush()
	same, i := 0, 0
	for y := view.FPY; y < view.FPY+view.FPH; y++ {
		for x := view.FPX; x < view.FPX+view.FPW; x++ {
			r, g, b, a := s.Hi.RGBAAt(x*render.Scale, y*render.Scale).RGBA()
			if classic[i] == [4]uint32{r, g, b, a} {
				same++
			}
			i++
		}
	}
	if ratio := float64(same) / float64(i); ratio < 0.90 {
		t.Fatalf("兩種風格的輪廓只有 %.1f%% 相同，換風格不該換掉畫面內容", ratio*100)
	}
}
