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
	// 九組：左側牆三階、右側牆三階、正牆深度 0／2／1。第 7 組（DOS 的
	// 補牆）留空。一組佔 stride 張 —— 那是相位數，跟著素材包走。
	stride := set.TorchStride()
	if stride != 9 {
		t.Fatalf("一組火炬有 %d 張，Mega Drive 的火焰是 9 個相位", stride)
	}
	var want []int
	for g := 0; g < 10; g++ {
		if g != 7 {
			want = append(want, g*stride)
		}
	}
	for _, first := range want {
		if first >= len(set.Torch) || set.Torch[first] == nil {
			t.Fatalf("第 %d 張火炬是空的", first)
		}
		b := set.Torch[first].Bounds()
		for f := 1; f < stride; f++ {
			im := set.Torch[first+f]
			if im == nil {
				t.Fatalf("第 %d 張火炬（相位 %d）是空的", first, f)
			}
			if im.Bounds() != b {
				t.Errorf("第 %d 組的相位 %d 是 %v，與相位 0 的 %v 不同",
					first/stride, f, im.Bounds(), b)
			}
		}
	}
	for i := 7 * stride; i < 8*stride; i++ {
		if i < len(set.Torch) && set.Torch[i] != nil {
			t.Errorf("補牆那一組（索引 %d）不該有圖", i)
		}
	}
	// 九個相位換的是**調色盤的第 5–7 格**，圖一樣、其餘顏色一樣。
	// 全部相同就表示火焰不會動；第 0–4 格跟著變就表示烘錯了那一條。
	base := set.Torch[0].Palette
	moved := 0
	for f := 1; f < stride; f++ {
		p := set.Torch[f].Palette
		for i := range base {
			if p[i] == base[i] {
				continue
			}
			if i < 5 || i > 7 {
				t.Fatalf("相位 %d 的第 %d 格也變了，火焰只該動第 5–7 格", f, i)
			}
			moved++
		}
	}
	if moved == 0 {
		t.Error("九個相位的調色盤完全相同，火焰不會動")
	}
	// 左右對稱：三對側牆火炬在 208 寬的視圖裡都要鏡射對稱
	// （原版的鏡射常數是 30 個 tile ＝ 240 像素，視圖是其中的 2–27 行）。
	for _, pair := range [][2]int{{0, 3 * stride}, {stride, 4 * stride}, {2 * stride, 5 * stride}} {
		l, r := set.TorchPos(pair[0]), set.TorchPos(pair[1])
		w := set.Torch[pair[0]].Bounds().Dx()
		if want := 208 - w - l.X; r.X != want || l.Y != r.Y {
			t.Errorf("第 %d／%d 組在 %v 與 %v，右邊該在 x=%d",
				pair[0]/stride, pair[1]/stride, l, r, want)
		}
	}
}
