package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 場景碼由地圖編號算出來，對照原版 `2PLAY sub_1B410` 的三張表。
//
// 七個區間把 60 張圖不重不漏地分完 —— 這條同時擋住「表抄錯一格」與
// 「某張圖落到預設值」，因為兩種錯都會在這裡變成不連續。
func TestSceneRanges(t *testing.T) {
	want := map[int][2]int{
		0: {0, 4},
		3: {5, 16},
		1: {17, 32},
		6: {33, 40},
		4: {41, 44},
		5: {45, 54},
		2: {55, 59},
	}
	byMap := map[int]int{}
	for code, r := range want {
		for m := r[0]; m <= r[1]; m++ {
			if prev, dup := byMap[m]; dup {
				t.Fatalf("地圖 %d 同時落在場景 %d 與 %d", m, prev, code)
			}
			byMap[m] = code
		}
	}
	if len(byMap) != 60 {
		t.Fatalf("七個區間只蓋了 %d 張圖，預期 60", len(byMap))
	}

	w := &game.World{}
	for m := 0; m < 60; m++ {
		w.MapIndex = m
		if got := w.Scene(); got != byMap[m] {
			t.Errorf("地圖 %d 的場景是 %d，預期 %d", m, got, byMap[m])
		}
	}
	// 五座城鎮是場景 0 —— 這是表讀對了的獨立佐證。
	for m := 0; m <= 4; m++ {
		w.MapIndex = m
		if w.Scene() != 0 {
			t.Errorf("城鎮 %d 不是場景 0", m)
		}
	}
	// 出界回「不在世界裡」。
	for _, m := range []int{-1, 60, 999} {
		w.MapIndex = m
		if got := w.Scene(); got != game.SceneOutside {
			t.Errorf("地圖 %d 的場景是 %d，預期 %d", m, got, game.SceneOutside)
		}
	}
}
