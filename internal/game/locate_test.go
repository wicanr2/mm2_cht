package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 定位術（法術 5、引擎編號 53）要把整張圖標成看過並請 UI 開地圖。
//
// 原版 `2CAST1 sub_1C1D2` 呼叫 `_2play_e14`（全檔唯一呼叫端）把整張
// 16×16 用 `.16` 的 `B` 圖磚畫出來；手冊寫「顯示目前 16×16 區域之地圖，
// 標示你所在位置及方向」。
func TestLocateSpellRevealsMapAndOpensIt(t *testing.T) {
	w := newWorld(t)
	w.MapIndex, w.X, w.Y = 5, 3, 3
	w.Explored = game.Explored{}

	// Learn 是寫進 Raw 再重新解析的，所以角色要有完整的記錄。
	c := game.Character{Name: "術士", Class: game.Sorcerer, Level: 9,
		SL: 9, SP: 99, Condition: game.CondGood, Raw: make([]byte, game.RecordSize)}
	// Cast 收的是「該系內的序號，1 起算」，不是引擎編號：
	// 定位術引擎編號 53，巫師系 = 53 − 48 + 1 = 第 6 條。
	c.Learn(6)
	c.Name, c.Class, c.Level, c.SL, c.SP, c.Condition =
		"術士", game.Sorcerer, 9, 9, 99, game.CondGood
	s := game.NewSession(w, []game.Character{c}, nil, 3)

	if w.Explored.Seen(5, 0, 0) {
		t.Fatal("施法前就已經看過整張圖，前提不成立")
	}
	res := s.Cast(0, 6)
	if !res.OK {
		t.Fatalf("定位術施不出來：%s", res.Reason)
	}
	if !res.ShowMap {
		t.Error("定位術沒有請 UI 開地圖")
	}
	for _, p := range [][2]int{{0, 0}, {15, 15}, {7, 8}} {
		if !w.Explored.Seen(5, p[0], p[1]) {
			t.Errorf("(%d,%d) 沒有被標成看過", p[0], p[1])
		}
	}
	// 只揭露目前這一張。
	if w.Explored.Seen(6, 0, 0) {
		t.Error("定位術連別張圖也一起打開了")
	}
}
