package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 中門（地圖 0）四邊的鄰接圖。
//
// 用途是排除一個解釋：實機從 (0,5) 再往西一步之後落在地圖 4，
// 如果西鄰是 4，那就只是**走出邊界**而不是踩到城門事件。
// 實測四邊全是 0 —— **中門沒有邊界轉移**，所以那個落點另有來源。
func TestMap0Neighbours(t *testing.T) {
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	a := attrs[0]
	t.Logf("地圖 0 的鄰接：北 %d 東 %d 南 %d 西 %d",
		a.North(), a.East(), a.South(), a.West())
	if a.North()|a.East()|a.South()|a.West() != 0 {
		t.Errorf("中門有邊界鄰接，先前的推論要重看")
	}
}
