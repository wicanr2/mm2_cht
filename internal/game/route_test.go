package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 用引擎自己的牆模型算出中門往西能走幾步，再跟 DOSBox 實機比對。
// 事件資料說 (0,5) 是往地圖 11 的出口（opcode 0x0c），所以從 (7,5)
// 面西應該走得到。走不到就是牆模型與原版不合。
func TestMiddlegateWestRoute(t *testing.T) {
	w := newWorld(t)
	m := w.CurrentMap()
	if m == nil {
		t.Skip("沒有地圖")
	}
	x, y := 7, 5
	west := game.Facing(3) // N=0 逆時針一格
	if dx, _ := west.Delta(); dx != -1 {
		for f := 0; f < 4; f++ {
			if dx, _ := game.Facing(f).Delta(); dx == -1 {
				west = game.Facing(f)
			}
		}
	}
	steps := 0
	for steps < 16 {
		if !m.CanMove(x, y, west) {
			break
		}
		dx, dy := west.Delta()
		x, y = x+dx, y+dy
		steps++
	}
	t.Logf("從 (7,5) 面西走了 %d 步，停在 (%d,%d)", steps, x, y)
	// **引擎與實機在這裡不一致**：引擎算得出七步到 (0,5)，
	// DOSBox 面西走到第四步左右就跳 `Solid!`（`shots/f4.png`）。
	// 差異可能來自起點（`fpv` 的實際座標未經確認，只是照
	// `ATTRIB` `+14` 推定為 (7,5)）或牆模型。
	// 在查清楚之前**不要動牆模型** —— 它有 93.8% 的自洽率撐著，
	// 而起點目前是零證據的推定。
	if x != 0 {
		t.Errorf("引擎走不到出口格 (0,5)，停在 (%d,%d)", x, y)
	}
}
