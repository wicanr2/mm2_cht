package gfx_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "..", "workplace", "orig", "MM2", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("沒有原版檔案 %s，跳過", name)
	}
	return b
}

// 誰有遮罩是**檔案說了算**，不是猜的。正牆與縱列牆整塊不透空、
// 側牆與它們的門那一組帶遮罩 —— 三種場景的容器版面一模一樣。
func TestWallMasksMatchTheContainer(t *testing.T) {
	// 影格 0-3 正牆、4-11 兩側牆、12-15 縱列牆，16-31 是同一批加上門。
	want := map[int]bool{}
	for i := 0; i < 32; i++ {
		d := i % 16
		want[i] = d >= 4 && d <= 11
	}
	for _, scene := range []string{"TOWN", "CAVE", "CASTLE"} {
		set, err := gfx.ParseSet(read(t, scene+".16"))
		if err != nil {
			t.Fatalf("%s.16 解不開：%v", scene, err)
		}
		if len(set) != 32 {
			t.Fatalf("%s.16 應該 32 張，得到 %d", scene, len(set))
		}
		for i, im := range set {
			if got := len(im.Mask) > 0; got != want[i] {
				t.Errorf("%s.16 影格 %d：有遮罩 = %v，預期 %v", scene, i, got, want[i])
			}
			if len(im.Mask) > 0 && len(im.Mask) != im.MaskStride()*im.Height {
				t.Errorf("%s.16 影格 %d：遮罩 %d bytes，預期 %d",
					scene, i, len(im.Mask), im.MaskStride()*im.Height)
			}
		}
	}
}

// 門把是最能分辨「遮罩」與「色號」的一個點：影格 24 的 (9,65)／(9,67)／
// (9,69) 是色號 8，遮罩說不畫（原版那裡露出地板）；同一張圖柵欄那一區
// 的色號 8 遮罩說要畫。任何「色號 8 一律透空」的規則都同時解釋不了兩邊。
func TestDoorFrameMaskBeatsColourKey(t *testing.T) {
	set, err := gfx.ParseSet(read(t, "TOWN.16"))
	if err != nil {
		t.Fatal(err)
	}
	im := set[24]
	pal := im.Paletted(gfx.EGAPalette)
	for _, y := range []int{65, 67, 69} {
		if c := pal.ColorIndexAt(9, y); c != 8 {
			t.Fatalf("(9,%d) 應該是色號 8，得到 %d", y, c)
		}
		if !im.Transparent(9, y) {
			t.Errorf("(9,%d) 遮罩應該說不畫", y)
		}
	}
	for _, p := range [][2]int{{22, 23}, {19, 71}, {21, 27}} {
		if c := pal.ColorIndexAt(p[0], p[1]); c != 8 {
			t.Fatalf("(%d,%d) 應該是色號 8，得到 %d", p[0], p[1], c)
		}
		if im.Transparent(p[0], p[1]) {
			t.Errorf("(%d,%d) 遮罩應該說要畫", p[0], p[1])
		}
	}
	// 四角的楔形（透視梯形之外）整段都不畫。
	for _, p := range [][2]int{{0, 0}, {18, 0}, {0, 119}, {23, 119}, {0, 106}} {
		if !im.Transparent(p[0], p[1]) {
			t.Errorf("(%d,%d) 在梯形之外，遮罩應該說不畫", p[0], p[1])
		}
	}
}

// 檔頭是「每張兩個 word」：資料偏移 ＋ 遮罩偏移。拿它掃過每一個 .16，
// 每一張的寬高與遮罩長度都要落在緩衝裡 —— 一張解不出來就是版面猜錯了。
func TestEverySetParsesWithTheTwoWordHeader(t *testing.T) {
	names, _ := filepath.Glob(filepath.Join("..", "..", "..", "workplace", "orig", "MM2", "*.16"))
	if len(names) == 0 {
		t.Skip("沒有原版檔案，跳過")
	}
	n := 0
	for _, p := range names {
		if filepath.Base(p) == "MONSTERS.16" {
			continue // 自己一套版面，見 monsters.go
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		set, err := gfx.ParseSet(b)
		if err != nil {
			t.Errorf("%s 解不開：%v", filepath.Base(p), err)
			continue
		}
		for i, im := range set {
			if im.Width <= 0 || im.Height <= 0 {
				t.Errorf("%s 影格 %d 尺寸 %dx%d", filepath.Base(p), i, im.Width, im.Height)
			}
			if len(im.Mask) > 0 && len(im.Mask) != im.MaskStride()*im.Height {
				t.Errorf("%s 影格 %d 遮罩長度 %d，預期 %d",
					filepath.Base(p), i, len(im.Mask), im.MaskStride()*im.Height)
			}
		}
		n++
	}
	if n < 20 {
		t.Fatalf("只掃到 %d 個 .16，樣本太少", n)
	}
}
