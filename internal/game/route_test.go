package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 用引擎自己的牆模型算中門往西能走幾步，跟 DOSBox 實機比對。
//
// 事件資料說 (0,5) 是往地圖 11（野外）的城門（opcode `0x0c`）。
// 實機面西走四步左右就 `Solid!`，引擎從 (7,5) 算卻是七步暢通 ——
// 差異的來源是**起點**：記憶體 dump 顯示進城後在 (7,3)。
func TestMiddlegateWestRoute(t *testing.T) {
	w := newWorld(t)
	m := w.CurrentMap()
	if m == nil {
		t.Skip("沒有地圖")
	}
	// 起點是實機量到的：`dump:pos` 讀 DGROUP 的 `ds:0393`／`ds:0394`
	// 得到 (7,3)，不是 `ATTRIB` `+14` 那個 (7,5)。
	x, y := 7, 3
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
	t.Logf("從 (7,3) 面西走了 %d 步，停在 (%d,%d)", steps, x, y)
	if steps > 4 {
		t.Errorf("從 (7,3) 面西走了 %d 步，實機在第四步左右就 Solid!", steps)
	}
}
