package view_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// chdirRoot 換到 repo 根 —— `origAt` 走的是相對路徑，不換就一律 skip，
// 而 **skip 與 pass 在 CI 的輸出裡長得一樣**。
func chdirRoot(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("workplace"); err == nil {
		return
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })
}

// 野外的每一張貼圖都要落在視圖裡。
//
// **這一條擋的是「畫到隔壁面板上」**：地形帶是 208 寬的整條，
// 拿擋路物那組落點（x 到 176）去畫就會衝出右緣，蓋掉旁邊的隊伍名單 ——
// 而那在截圖上看起來像「隊伍面板破了一塊」，不像座標用錯表。踩過一次。
func TestOutdoorPiecesStayInsideView(t *testing.T) {
	chdirRoot(t)
	load := func(name string) []gfx.Image {
		b := origAt(t, name)
		im, err := gfx.ParseSet(b)
		if err != nil {
			t.Fatalf("%s 解不開：%v", name, err)
		}
		return im
	}
	feats := [][]gfx.Image{load("OUTDOOR1.16"), load("OUTDOOR2.16"), load("OUTDOOR3.16")}
	bands := load("OCEAN.16") // 四個地形檔張數與尺寸相同，驗一個就夠

	check := func(what string, im gfx.Image, x, y int) {
		t.Helper()
		if x < view.FPX || x+im.Width > view.FPX+view.FPW {
			t.Errorf("%s：x %d 寬 %d → %d，視圖是 %d–%d",
				what, x, im.Width, x+im.Width, view.FPX, view.FPX+view.FPW)
		}
		if y < view.FPY || y+im.Height > view.FPY+view.FPH {
			t.Errorf("%s：y %d 高 %d → %d，視圖是 %d–%d",
				what, y, im.Height, y+im.Height, view.FPY, view.FPY+view.FPH)
		}
	}
	for _, g := range view.OutdoorGeometry() {
		if g.Band {
			if g.Frame < len(bands) {
				check(g.What, bands[g.Frame], g.X, g.Y)
			}
			continue
		}
		for i, set := range feats {
			if g.Frame < len(set) {
				check(g.What+" 第"+string(rune('1'+i))+"組", set[g.Frame], g.X, g.Y)
			}
		}
	}
}

// 三組擋路物的張數必須一樣 —— 影格編號是共用的（`ds:54B0` 只存組號，
// 影格另外查表），少一張就會在某些地圖上安靜地少畫一塊。
func TestOutdoorSetsHaveSameFrames(t *testing.T) {
	chdirRoot(t)
	n := -1
	for _, name := range []string{"OUTDOOR1.16", "OUTDOOR2.16", "OUTDOOR3.16"} {
		im, err := gfx.ParseSet(origAt(t, name))
		if err != nil {
			t.Fatalf("%s 解不開：%v", name, err)
		}
		if n < 0 {
			n = len(im)
		} else if len(im) != n {
			t.Errorf("%s 有 %d 張，前一組是 %d 張", name, len(im), n)
		}
	}
	if n != 8 {
		t.Errorf("每組應該是 8 張（四個深度的正面 ＋ 左右近處），實際 %d", n)
	}
}
