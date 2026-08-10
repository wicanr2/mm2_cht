package view_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/view"
)

func testWorld(t *testing.T) *game.World {
	t.Helper()
	// 換到 repo 根，`data/` 才找得到（EnsureData 走相對路徑）。
	// 同一個測試裡呼叫兩次也要能用，所以先探當前目錄。
	if _, err := os.Stat("data"); err != nil {
		wd, _ := os.Getwd()
		if err := os.Chdir(filepath.Join("..", "..")); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chdir(wd) })
	}
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：" + err.Error())
	}
	w, err := game.NewWorld(origAt(t, "MAP.DAT"), origAt(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// origAt 在換過目錄之後讀原版檔。
func origAt(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("workplace", "orig", "MM2", name))
	if err != nil {
		t.Skip("沒有原版 " + name + "，跳過")
	}
	return b
}

// testAssets 只需要字型 —— 地圖畫面不畫第一人稱視角。
func testAssets(t *testing.T) view.Assets {
	t.Helper()
	return view.Assets{}
}

// 地圖上的朝向標記要指對邊：北在上、東在右。
func TestMapFacingMarker(t *testing.T) {
	w := testWorld(t)
	a := testAssets(t)
	w.MapIndex, w.X, w.Y = 0, 8, 8
	s := view.NewScreen()

	mark := func(f game.Facing) (x, y int) {
		w.Face = f
		view.DrawMap(s, w, a, view.MapInfo{})
		// 找出朝向標記那個顏色（12）的重心
		sx, sy, n := 0, 0, 0
		b := s.Orig.Bounds()
		for py := b.Min.Y; py < b.Max.Y; py++ {
			for px := b.Min.X; px < b.Max.X; px++ {
				if s.Orig.ColorIndexAt(px, py) == 12 {
					sx, sy, n = sx+px, sy+py, n+1
				}
			}
		}
		if n == 0 {
			t.Fatalf("朝向 %v 找不到標記", f)
		}
		return sx / n, sy / n
	}
	nx, ny := mark(game.North)
	sx, sy := mark(game.South)
	ex, ey := mark(game.East)
	wx, wy := mark(game.West)
	if ny >= sy {
		t.Errorf("北的標記沒有比南高：北 y=%d 南 y=%d", ny, sy)
	}
	if ex <= wx {
		t.Errorf("東的標記沒有比西右：東 x=%d 西 x=%d", ex, wx)
	}
	if nx != sx {
		t.Errorf("南北的標記 x 不同：%d vs %d", nx, sx)
	}
	if ey != wy {
		t.Errorf("東西的標記 y 不同：%d vs %d", ey, wy)
	}
}
