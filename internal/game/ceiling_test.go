package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// `ATTRIB.DAT` 的 `+32`…`+63` 是 16×16 的天花板位元圖，第一人稱視圖拿它
// 挑 `SKY.16` 的影格（`2PLAY _2play_e02`）。
//
// 這條守的是**資料自己分的群**：全 0 的 24 張必須精確等於野外圖
// （`+18` 撞門難度為 0 的那批），全 1 的是地城與城堡。這個對應是位置
// 判對了的獨立佐證 —— 換一個欄位來讀，分群立刻散掉。
func TestCeilingBitmapMatchesIndoorOutdoor(t *testing.T) {
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	var none, all, mixed []int
	for i := range attrs {
		n := 0
		for y := 0; y < game.MapH; y++ {
			for x := 0; x < game.MapW; x++ {
				if attrs[i].Ceiling(x, y) {
					n++
				}
			}
		}
		switch n {
		case 0:
			none = append(none, i)
		case game.MapCells:
			all = append(all, i)
		default:
			mixed = append(mixed, i)
		}
	}
	if len(none)+len(all)+len(mixed) != 60 {
		t.Fatalf("只分到 %d 張", len(none)+len(all)+len(mixed))
	}
	// 完全沒有天花板的那批 ＝ 野外圖，一張不多一張不少。
	for _, i := range none {
		if attrs[i].Indoor() {
			t.Errorf("地圖 %d 沒有任何天花板，卻是室內圖", i)
		}
	}
	outdoor := 0
	for i := range attrs {
		if !attrs[i].Indoor() {
			outdoor++
		}
	}
	if len(none) != outdoor {
		t.Errorf("沒有天花板的有 %d 張，野外圖有 %d 張，應該相等", len(none), outdoor)
	}
	// 五座城鎮是混合的：街道露天、屋簷底下不是。
	for _, i := range []int{0, 1, 2, 3, 4} {
		found := false
		for _, m := range mixed {
			if m == i {
				found = true
			}
		}
		if !found {
			t.Errorf("城鎮 %d 的天花板位元圖不是混合的", i)
		}
	}
	if len(all) == 0 {
		t.Error("沒有任何一張圖是整張都有天花板")
	}
}
