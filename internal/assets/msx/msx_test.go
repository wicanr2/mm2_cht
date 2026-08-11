package msx_test

import (
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
