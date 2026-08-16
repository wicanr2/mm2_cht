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
