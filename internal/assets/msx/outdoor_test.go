package msx_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/msx"
)

// 每一張戶外素材表的實際尺寸（`d.Image` 讀出來的檔頭，見 docs/research/02）。
var outSheetSize = map[msx.OutSheet][2]int{
	msx.SheetBack:     {154, 64},
	msx.SheetFeatureA: {182, 102},
	msx.SheetFeatureB: {196, 62},
	msx.SheetBand:     {154, 168},
}

// 產生出來的落點表要落在視圖裡、來源要落在表裡。
//
// **這一條擋的是產生器**：`tools/msxout.py` 是符號執行反組譯來的，
// 讀錯一個位移就會產出「看起來像座標」的數字。畫面上一塊貼在視圖外
// 只是少一塊，貼在表外則是切不到 —— 兩種都不會有錯誤訊息。
func TestOutdoorPiecesFitInsideView(t *testing.T) {
	w, h := msx.OutView[0], msx.OutView[1]
	n := 0
	for set := 0; set < 3; set++ {
		for d := 0; d < 4; d++ {
			span := msx.OutDepthRange[d]
			for v := -span; v <= span; v++ {
				for _, p := range msx.OutdoorPieces(set, d, v) {
					n++
					x := p.DXK*v + p.DX
					if x < 0 || x+p.W > w || p.DY < 0 || p.DY+p.H > h {
						t.Errorf("組 %d 深度 %d v=%d：貼到 (%d,%d) %d×%d，視圖是 %d×%d",
							set, d, v, x, p.DY, p.W, p.H, w, h)
					}
					sz, ok := outSheetSize[p.Sheet]
					if !ok {
						t.Errorf("組 %d 深度 %d v=%d：不認得的素材表 %v", set, d, v, p.Sheet)
						continue
					}
					if p.SX < 0 || p.SX+p.W > sz[0] || p.SY < 0 || p.SY+p.H > sz[1] {
						t.Errorf("組 %d 深度 %d v=%d：來源 (%d,%d) %d×%d，表是 %d×%d",
							set, d, v, p.SX, p.SY, p.W, p.H, sz[0], sz[1])
					}
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("一塊都沒列出來 —— 表空了，不是「這些格子沒東西」")
	}
}

// 地形帶的每一塊也要落在視圖裡、來源要落在那張帶裡（含三個變體）。
func TestOutdoorBandFits(t *testing.T) {
	w, h := msx.OutView[0], msx.OutView[1]
	sz := outSheetSize[msx.SheetBand]
	n := 0
	for d := 0; d < 4; d++ {
		span := msx.OutDepthRange[d]
		for v := -span; v <= span; v++ {
			for _, p := range msx.OutdoorBand(d, v) {
				n++
				x := p.XK*v + p.X
				if x < 0 || x+p.W > w || p.DY < 0 || p.DY+p.H > h {
					t.Errorf("深度 %d v=%d：貼到 (%d,%d) %d×%d，視圖是 %d×%d",
						d, v, x, p.DY, p.W, p.H, w, h)
				}
				for _, off := range msx.OutBandVariant {
					if p.SY+off < 0 || p.SY+off+p.H > sz[1] {
						t.Errorf("深度 %d v=%d 變體位移 %d：來源列 %d–%d，帶高 %d",
							d, v, off, p.SY+off, p.SY+off+p.H, sz[1])
					}
				}
				if x+p.W > sz[0] {
					t.Errorf("深度 %d v=%d：來源 x %d–%d，帶寬 %d", d, v, x, x+p.W, sz[0])
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("地形帶一塊都沒列出來")
	}
}

// 三組擋路物在每個深度都要有東西可畫。
// 少一組不會報錯，只會在畫面上安靜地少一種地形。
func TestOutdoorEverySetHasEveryDepth(t *testing.T) {
	for set := 0; set < 3; set++ {
		for d := 0; d < 4; d++ {
			if len(msx.OutdoorPieces(set, d, 0)) == 0 {
				t.Errorf("組 %d 深度 %d 在 v=0 沒有任何一塊", set, d)
			}
		}
	}
}

// 24 張野外圖一張不少，兩個換得掉的槽都只會挑到自己那兩張。
func TestOutdoorMapsAreComplete(t *testing.T) {
	want := 0
	for m := 5; m <= 16; m++ {
		want++
		_ = m
	}
	want += 12 // 33–44
	if len(msx.OutdoorMaps) != want {
		t.Fatalf("野外圖 %d 張，預期 %d 張", len(msx.OutdoorMaps), want)
	}
	for m, e := range msx.OutdoorMaps {
		if e.FeatureA != msx.OutFeatureAID[0] && e.FeatureA != msx.OutFeatureAID[1] {
			t.Errorf("地圖 %d 的擋路物 A 是 %04X，不在 %v 裡", m, e.FeatureA, msx.OutFeatureAID)
		}
		if e.Band != msx.OutBandID[0] && e.Band != msx.OutBandID[1] {
			t.Errorf("地圖 %d 的地形帶是 %04X，不在 %v 裡", m, e.Band, msx.OutBandID)
		}
	}
	for m := 5; m <= 16; m++ {
		if _, ok := msx.OutdoorMaps[m]; !ok {
			t.Errorf("地圖 %d 不在表裡", m)
		}
	}
	for m := 33; m <= 44; m++ {
		if _, ok := msx.OutdoorMaps[m]; !ok {
			t.Errorf("地圖 %d 不在表裡", m)
		}
	}
}

// 地平線帶的變體：沙漠與海洋走第一個，其餘走第二個（`sub_297A`）。
func TestOutBandVariantFor(t *testing.T) {
	for _, c := range []struct {
		terrain, want int
	}{{9, 0}, {10, 0}, {11, 1}, {12, 1}, {0, 1}} {
		if got := msx.OutBandVariantFor(c.terrain); got != c.want {
			t.Errorf("地形碼 %d 的變體是 %d，預期 %d", c.terrain, got, c.want)
		}
		if msx.OutBandVariantFor(c.terrain) >= len(msx.OutBandVariant) {
			t.Errorf("地形碼 %d 的變體越界", c.terrain)
		}
	}
}

// 地圖 41–44 各有自己的地面，蓋在背景下緣那 28 列且不越界。
func TestOutGroundFitsTheBackground(t *testing.T) {
	if len(msx.OutGroundID) != 4 {
		t.Fatalf("地面素材 %d 張，預期 4 張", len(msx.OutGroundID))
	}
	seen := map[uint16]bool{}
	for m, id := range msx.OutGroundID {
		if m < 41 || m > 44 {
			t.Errorf("地圖 %d 不該有自己的地面", m)
		}
		if seen[id] {
			t.Errorf("素材 %04X 被兩張圖用到", id)
		}
		seen[id] = true
	}
	if msx.OutGroundRow+28 != msx.OutView[1] {
		t.Errorf("地面貼在第 %d 列、高 28，背景只有 %d 列", msx.OutGroundRow, msx.OutView[1])
	}
}
