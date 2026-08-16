package ui

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/view"
)

// TestMDSceneSetLoads 守住 Mega Drive 場景素材那條路。
//
// 素材包在 `workplace/`（gitignore），沒有就跳過 —— 這個測試要抓的是
// 「包在、但載不進來」，不是「這台機器沒烘過」。
func TestMDSceneSetLoads(t *testing.T) {
	set, err := loadMDTown("../../" + mdSceneDir)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("沒有 workplace/md-scene，先跑 tools/mdscene.py --export")
	}
	if err != nil {
		t.Fatalf("Mega Drive 場景素材載不進來：%v", err)
	}
	if set.Platform != view.PlatformMegaDrive {
		t.Fatalf("平台是 %v", set.Platform)
	}
	// 正牆 0–3、左側牆 4–7、右側牆 8–11，門的變體 +16。缺一格就是
	// 烘的時候漏了，畫面上會變成「那個深度突然沒有牆」。
	for _, i := range mdWallSlots {
		if set.Walls[i] == nil {
			t.Errorf("第 %d 張牆是空的", i)
		}
	}
	// 側牆柱是一整根 120 高的圖，正牆不是 —— 這一條同時擋住
	// 「拿 DOS 的槽位順序去烘」那種錯（DOS 的側牆比正牆矮）。
	if b := set.Walls[4].Bounds(); b.Dy() != 120 {
		t.Errorf("左側牆第 0 深度高 %d，Mega Drive 的側牆柱該是 120", b.Dy())
	}
	if b := set.Walls[0].Bounds(); b.Dy() >= 120 {
		t.Errorf("最近那面正牆高 %d，不該和側牆柱一樣高", b.Dy())
	}
}

// TestMDSceneTorches 守住火炬那一路。
//
// 素材包烘的是「一張圖 × 三份調色盤」——原版每一幀改的是 CRAM 第 3 條的
// 第 5–7 格，不是換 tile。所以三張的**尺寸一定相同**，而且中間那一組
// （DOS 的補牆，索引 21–23）在 Mega Drive 是空的：那一對火炬在原版畫在
// 斜前方的牆上，而 remake 的牆面沒有那一層，只點火炬會浮在空中。
func TestMDSceneTorches(t *testing.T) {
	set, err := loadMDTown("../../" + mdSceneDir)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("沒有 workplace/md-scene，先跑 tools/mdscene.py --export")
	}
	if err != nil {
		t.Fatalf("Mega Drive 場景素材載不進來：%v", err)
	}
	// 九組：左側牆三階、右側牆三階、正牆深度 0／1／2。第 7 組留空。
	want := []int{0, 3, 6, 9, 12, 15, 18, 24, 27}
	for _, first := range want {
		if first >= len(set.Torch) || set.Torch[first] == nil {
			t.Fatalf("第 %d 張火炬是空的", first)
		}
		b := set.Torch[first].Bounds()
		for f := 1; f < 3; f++ {
			im := set.Torch[first+f]
			if im == nil {
				t.Fatalf("第 %d 張火炬（相位 %d）是空的", first, f)
			}
			if im.Bounds() != b {
				t.Errorf("第 %d 組的相位 %d 是 %v，與相位 0 的 %v 不同",
					first/3, f, im.Bounds(), b)
			}
		}
	}
	for _, i := range []int{21, 22, 23} {
		if i < len(set.Torch) && set.Torch[i] != nil {
			t.Errorf("補牆那一組（索引 %d）不該有圖", i)
		}
	}
	// 左右對稱：三對側牆火炬在 208 寬的視圖裡都要鏡射對稱
	// （原版的鏡射常數是 30 個 tile ＝ 240 像素，視圖是其中的 2–27 行）。
	for _, pair := range [][2]int{{0, 9}, {3, 12}, {6, 15}} {
		l, r := set.TorchPos(pair[0]), set.TorchPos(pair[1])
		w := set.Torch[pair[0]].Bounds().Dx()
		if want := 208 - w - l.X; r.X != want || l.Y != r.Y {
			t.Errorf("第 %d／%d 組在 %v 與 %v，右邊該在 x=%d",
				pair[0]/3, pair[1]/3, l, r, want)
		}
	}
}
