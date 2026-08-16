package msx_test

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/msx"
)

func open(t *testing.T) *msx.Disk {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join("..", "..", "..", "workplace", "msx", "*Disk 1*.dsk"))
	for _, p := range m {
		if filepath.Base(p)[0] == '.' {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		d, err := msx.Open(b)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	t.Skip("沒有 MSX 磁片")
	return nil
}

// 兩張表都要讀到。只讀第一張的話圖形檔一個都看不到，而**不會報錯** ——
// 第一張表本身完全合法，解得出一批檔案。
func TestBothTables(t *testing.T) {
	d := open(t)
	if n := len(d.IDs()); n < 200 {
		t.Fatalf("只讀到 %d 個檔案，兩張表應該有兩百多個", n)
	}
	for _, id := range []uint16{0xFFF0, 0x2020, 0x2044, 0x2100} {
		if d.File(id) == nil {
			t.Errorf("找不到檔案 %04X", id)
		}
	}
}

// 第一人稱的四套場景各是一張 462×128。
func TestSceneSheets(t *testing.T) {
	d := open(t)
	pal, err := d.Palette()
	if err != nil {
		t.Fatal(err)
	}
	if len(pal) != 16 {
		t.Fatalf("調色盤 %d 色", len(pal))
	}
	for id := uint16(0x2020); id <= 0x2023; id++ {
		im, err := d.Image(id, pal)
		if err != nil {
			t.Fatalf("%04X: %v", id, err)
		}
		if w, h := im.Bounds().Dx(), im.Bounds().Dy(); w != 462 || h != 128 {
			t.Errorf("%04X 是 %d×%d，應該是 462×128", id, w, h)
		}
	}
}

// 解出來的長度要與檔頭宣告的完全相同 —— RLE 解碼器餵什麼都吐得出東西，
// 「有輸出」不是驗證。Image 內部就是拿這個當判準，這裡守住它真的會擋。
func TestNonImageRejected(t *testing.T) {
	d := open(t)
	pal, _ := d.Palette()
	if _, err := d.Image(0xFFF0, pal); err == nil {
		t.Fatal("常駐引擎被當成圖解出來了")
	}
}

// 火炬的三張影格必須真的不一樣。相同的話畫面照樣正常，只是火炬不會動 ——
// 而那看起來像「原版本來就這樣」。
func TestTorchFramesDiffer(t *testing.T) {
	d := open(t)
	pal, _ := d.Palette()
	sheet, err := d.Image(msx.SceneID[0], pal)
	if err != nil {
		t.Fatal(err)
	}
	_, torches, _, _, _ := msx.Scene(sheet)
	same := 0
	for slot := 0; slot < len(torches)/msx.TorchFrames; slot++ {
		a := torches[slot*msx.TorchFrames]
		if a == nil {
			continue
		}
		for f := 1; f < msx.TorchFrames; f++ {
			b := torches[slot*msx.TorchFrames+f]
			if b == nil {
				t.Errorf("槽 %d 影格 %d 是 nil", slot, f)
				continue
			}
			if string(a.Pix) == string(b.Pix) {
				same++
				t.Errorf("槽 %d 的影格 %d 與影格 0 完全相同", slot, f)
			}
		}
	}
	if same == 0 {
		t.Logf("每個火炬位置的 %d 張影格都不同", msx.TorchFrames)
	}
}

// `WallSlots`／`TorchSlots` 說有圖的每一格都要真的切得出圖，而且不能是
// 一整塊背景色。
//
// 這條擋的不是「畫面難看」，是**整套素材安靜消失**：貼圖表裡有一筆矩形
// 超出素材表右緣時，`Scene` 那一格回 nil，`internal/ui` 的完整性檢查
// 因此打掉整組 MSX 素材，而 loader 的失敗屬於「玩家不一定有那份原版」
// 那一類 —— 不印任何訊息。症狀是 MSX 從 `F6` 循環裡不見了，看起來
// 與「這台機器沒有 MSX 磁片」一模一樣。踩過一次（`docs/todo.md` A8）。
func TestDeclaredSlotsHavePixels(t *testing.T) {
	d := open(t)
	pal, _ := d.Palette()
	sheet, err := d.Image(msx.SceneID[0], pal)
	if err != nil {
		t.Fatal(err)
	}
	walls, torches, _, _, bg := msx.Scene(sheet)
	if bg == nil {
		t.Error("背景（天空與地板）切不出來")
	}
	drawn := func(im *image.Paletted) int {
		n := 0
		for _, p := range im.Pix {
			if p != 0 {
				n++
			}
		}
		return n
	}
	check := func(what string, slots []int, imgs []*image.Paletted) {
		for _, i := range slots {
			if i >= len(imgs) || imgs[i] == nil {
				t.Errorf("%s 第 %d 格切不出來（矩形超出素材表？）", what, i)
				continue
			}
			if n := drawn(imgs[i]); n == 0 {
				t.Errorf("%s 第 %d 格整塊是背景色 —— 那不是素材，是切錯位置", what, i)
			}
		}
	}
	check("牆面", msx.WallSlots, walls)
	check("火炬", msx.TorchSlots, torches)
	if len(msx.WallSlots) == 0 || len(msx.TorchSlots) == 0 {
		t.Fatal("宣告的槽位清單是空的")
	}
}
