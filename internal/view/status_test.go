package view_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// 狀態列的三個數字對到原版 `sub_147D8` 逐列印的那三格：
// `ds:03D5` 照明、`ds:03D6` 魔法防護、`ds:03D7` 拒絕傷害。
//
// 先前魔法與力場寫死 0，因為來源還沒解 —— 這條守著它們真的接上了全域值，
// 而不是又回到寫死。
func TestStatusLinesShowsProtectionValues(t *testing.T) {
	w := &game.World{X: 3, Y: 4, Globals: map[uint16]byte{}}
	w.Globals[0x03D5] = 7
	w.Globals[0x03D6] = 25
	w.Globals[0x03D7] = 40

	got := view.StatusLines(w, "中門")
	if len(got) != 2 {
		t.Fatalf("回了 %d 行，預期 2", len(got))
	}
	for _, want := range []string{"照明 7", "魔法 25%", "力場 40%"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("第一行 %q 少了 %q", got[0], want)
		}
	}
	if strings.Contains(got[0], "飄浮") {
		t.Errorf("飄浮術沒開卻印了：%q", got[0])
	}
}

// 第四行原版只在飄浮術（`ds:03D8`）開著時才出現。
func TestStatusLinesShowsLevitateOnlyWhenActive(t *testing.T) {
	w := &game.World{Globals: map[uint16]byte{}}
	if got := view.StatusLines(w, "x"); strings.Contains(got[0], "飄浮") {
		t.Fatalf("預設不該有飄浮：%q", got[0])
	}
	w.Globals[0x03D8] = 1
	if got := view.StatusLines(w, "x"); !strings.Contains(got[0], "飄浮") {
		t.Errorf("飄浮術開著卻沒印：%q", got[0])
	}
}
