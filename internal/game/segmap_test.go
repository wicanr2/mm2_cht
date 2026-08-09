package game_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// 驗「`Segment.Index` ＝ 地圖號」。
//
// `Parse` 用檔頭那張 71 項偏移表的**索引**當 `Segment.Index`，空槽跳過，
// 所以 **`Index` 是地圖號、切片位置不是**。這一條就是釘住這件事 ——
// 拿切片位置去比會得到「兩邊各半落錯」的假象。
//
// 判準：
// `EVENTSI` 是室內、`EVENTSO` 是室外，如果段號等於地圖號，
// 那麼有內容的 EVENTSI 段應該落在室內圖（`ATTRIB` `+18` 非 0）上，
// EVENTSO 段落在室外圖上。
func TestSegmentIsMapIndex(t *testing.T) {
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	indoor := func(i int) bool { return i < len(attrs) && attrs[i].BashDifficulty() != 0 }

	// 順便量兩個候選假設的規模。若「段號 ＝ 該類型的第幾張圖」成立，
	// 有內容的段數應該等於該類型的地圖數。
	in, out := 0, 0
	for i := range attrs {
		if indoor(i) {
			in++
		} else {
			out++
		}
	}
	t.Logf("地圖：室內 %d 張、室外 %d 張", in, out)

	for _, tc := range []struct {
		file string
		want bool // 期望的室內／室外
		什麼 string
	}{
		{"EVENTSI.DAT", true, "室內"},
		{"EVENTSO.DAT", false, "室外"},
	} {
		b, err := os.ReadFile(filepath.Join("..", "..", "workplace", "orig", "MM2", tc.file))
		if err != nil {
			t.Skipf("找不到 %s", tc.file)
		}
		segs, err := events.Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		hit, miss, empty := 0, 0, 0
		var missList []int
		for _, sg := range segs {
			if len(sg.Events) == 0 && len(sg.Strings) == 0 {
				empty++
				continue
			}
			// Segment.Index 已經是**地圖號**（Parse 用檔頭偏移表的
			// 索引當它），不是切片位置 —— 空槽會被跳過，兩者不同。
			i := sg.Index
			if i >= len(attrs) {
				continue // ATTRIB 只有 60 張，60–70 沒有分類可比
			}
			if indoor(i) == tc.want {
				hit++
			} else {
				miss++
				if len(missList) < 12 {
					missList = append(missList, i)
				}
			}
		}
		t.Logf("%s：可比對 %d 段，落在%s圖的 %d、落錯的 %d（%v）",
			tc.file, hit+miss, tc.什麼, hit, miss, missList)
		if miss != 0 {
			t.Errorf("%s 有 %d 段落錯：%v", tc.file, miss, missList)
		}
	}
}
