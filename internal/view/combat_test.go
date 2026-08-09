package view_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/view"
)

func monstersBlob(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "workplace", "orig", "MM2", "MONSTERS.16"))
	if err != nil {
		t.Skip("沒有原版 MONSTERS.16，跳過")
	}
	return b
}

// 畫完之後視圖區裡要真的有東西，而且不能溢出視圖區。
//
// 「有東西」用非背景色的像素數量量。只檢查「沒有 panic」擋不住
// 「畫了但全在畫面外」這種錯法。
func TestDrawMonstersStaysInView(t *testing.T) {
	blob := monstersBlob(t)
	index, err := gfx.MonsterIndex(blob)
	if err != nil {
		t.Fatal(err)
	}
	pic, err := gfx.ParseMonsterPic(blob, gfx.ResolveMonsterPic(index, 1))
	if err != nil {
		t.Fatal(err)
	}

	for _, n := range []int{1, 2, 3, 6} {
		s := render.New(gfx.EGAPalette)
		s.Clear(0)
		ms := make([]view.MonsterSprite, n)
		for i := range ms {
			ms[i] = view.MonsterSprite{Pic: pic, Anim: -1}
		}
		view.DrawMonsters(s, ms)

		drawn, outside := 0, 0
		for y := 0; y < render.OrigH; y++ {
			for x := 0; x < render.OrigW; x++ {
				if s.Orig.ColorIndexAt(x, y) == 0 {
					continue
				}
				drawn++
				if x < view.FPX || x >= view.FPX+view.FPW ||
					y < view.FPY || y >= view.FPY+view.FPH {
					outside++
				}
			}
		}
		if drawn == 0 {
			t.Errorf("%d 隻：視圖區裡什麼都沒畫", n)
		}
		if outside > 0 {
			t.Errorf("%d 隻：有 %d 個像素畫到視圖區外", n, outside)
		}
	}
}

// 透明色不能被畫上去 —— 畫了就會在怪物周圍留一圈洋紅方框。
func TestDrawMonstersSkipsTransparent(t *testing.T) {
	blob := monstersBlob(t)
	index, err := gfx.MonsterIndex(blob)
	if err != nil {
		t.Fatal(err)
	}
	pic, err := gfx.ParseMonsterPic(blob, gfx.ResolveMonsterPic(index, 1))
	if err != nil {
		t.Fatal(err)
	}
	s := render.New(gfx.EGAPalette)
	s.Clear(0)
	view.DrawMonsters(s, []view.MonsterSprite{{Pic: pic, Anim: -1}})

	// 基準圖的左上角是透明色，底色是 0；那一格畫完仍該是 0。
	base := pic.Frames[0]
	ox := view.FPX + view.FPW/2 - base.Width/2
	oy := view.FPY + view.FPH - base.Height
	if got := s.Orig.ColorIndexAt(ox, oy); got != 0 {
		t.Errorf("基準圖左上角被畫成 %d，透明色沒有跳過", got)
	}
}

// 動畫每一步都要畫得出來，而且至少有一步與基準圖不同 ——
// 否則等於動畫表沒接上。
func TestDrawMonsterAnimChangesPixels(t *testing.T) {
	blob := monstersBlob(t)
	index, err := gfx.MonsterIndex(blob)
	if err != nil {
		t.Fatal(err)
	}
	pic, err := gfx.ParseMonsterPic(blob, gfx.ResolveMonsterPic(index, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(pic.Anims) == 0 {
		t.Fatal("圖號 1 一段動畫都沒有")
	}

	shot := func(anim, step int) []uint8 {
		s := render.New(gfx.EGAPalette)
		s.Clear(0)
		view.DrawMonsters(s, []view.MonsterSprite{{Pic: pic, Anim: anim, Step: step}})
		out := make([]uint8, len(s.Orig.Pix))
		copy(out, s.Orig.Pix)
		return out
	}

	base := shot(-1, 0)
	diff := 0
	for a, seq := range pic.Anims {
		for step := range seq {
			if !equal(base, shot(a, step)) {
				diff++
			}
		}
	}
	if diff == 0 {
		t.Error("每一步都與基準圖相同，動畫表沒有接上")
	}
}

func equal(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 名單的行數與收尾照實機：十三隻骷髏顯示十行加一行「+3 more Skeletons」。
func TestMonsterListText(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want int
		last string
	}{
		{0, 0, ""},
		{1, 1, "Skeleton"},
		{10, 10, "Skeleton"},
		{13, 11, "+3 more Skeleton"},
	} {
		got := view.MonsterListText("Skeleton", tc.n)
		if len(got) != tc.want {
			t.Errorf("%d 隻產生 %d 行，預期 %d", tc.n, len(got), tc.want)
			continue
		}
		if tc.want > 0 && got[len(got)-1] != tc.last {
			t.Errorf("%d 隻的最後一行是 %q，預期 %q", tc.n, got[len(got)-1], tc.last)
		}
	}
}
