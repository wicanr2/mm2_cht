package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// findDoor 在某張圖上找一面門，回傳座標與朝向。
//
// 不寫死座標：地圖是玩家自備的原版資料，寫死一個 (x, y, 面) 會在資料換版本
// 時變成「測試通過但測的不是門」—— 而那和真的沒有門長得一模一樣。
func findDoor(t *testing.T, m *game.Map) (int, int, game.Facing) {
	t.Helper()
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			for _, f := range []game.Facing{game.North, game.East, game.South, game.West} {
				if m.WallKind(x, y, f) == game.WallDoor {
					return x, y, f
				}
			}
		}
	}
	t.Skip("這張圖沒有門")
	return 0, 0, game.North
}

// 開門改的是屬性層的牆位元，地形層那兩個位元不動。
//
// 原版 root `sub_13A64`：`attr ^= (terrain & 方向遮罩) >> 1`。
// 所以開完之後**門還是門**（畫面照樣畫門），只是不再擋路 ——
// 拿 `DrawKind` 一起驗，才不會做成「開門＝把門變成通道」。
func TestOpenDoorFlipsWallBitOnly(t *testing.T) {
	w := newWorld(t)
	w.MapIndex = 0
	m := w.CurrentMap()
	x, y, f := findDoor(t, m)

	if m.CanMove(x, y, f) {
		t.Fatalf("(%d,%d) 面 %v 是門卻走得過去，前提就不成立", x, y, f)
	}
	terrain := m.Terrain[game.Cell(x, y)]

	w.OpenDoor(x, y, f)

	if !m.CanMove(x, y, f) {
		t.Errorf("(%d,%d) 面 %v 開門後仍然擋路", x, y, f)
	}
	if got := m.Terrain[game.Cell(x, y)]; got != terrain {
		t.Errorf("開門動到了地形層：%#02x → %#02x", terrain, got)
	}
	if k := m.DrawKind(x, y, f); k != game.WallDoor {
		t.Errorf("開門後畫出來的不是門而是 %v", k)
	}
}

// 離開地圖再回來，門要關回去。
//
// 原版的屬性層只有當前地圖一份，`2PLAY sub_1BE24` 在地圖編號改變時整層
// 從 `MAP.DAT` 重讀。remake 六十張圖都在記憶體裡，所以要自己還原。
func TestDoorClosesAfterLeavingMap(t *testing.T) {
	w := newWorld(t)
	w.MapIndex = 0
	m := w.CurrentMap()
	x, y, f := findDoor(t, m)

	w.OpenDoor(x, y, f)
	if !m.CanMove(x, y, f) {
		t.Fatalf("門沒開成，後面的驗收沒有意義")
	}

	w.MapIndex = 1
	w.CurrentMap() // 換圖：這一刻還原上一張
	w.MapIndex = 0
	if w.CurrentMap().CanMove(x, y, f) {
		t.Errorf("回到地圖 0 之後門還開著，屬性層沒有重讀")
	}
}

// 事件旗標（屬性層 bit 7）與門同一層，生存期也一樣。
func TestConsumedEventReturnsAfterLeavingMap(t *testing.T) {
	w := newWorld(t)
	w.MapIndex = 0
	m := w.CurrentMap()

	x, y := -1, -1
	for c := 0; c < 256 && x < 0; c++ {
		if m.Attr[c]&game.AttrHasEvent != 0 {
			x, y = c%16, c/16
		}
	}
	if x < 0 {
		t.Skip("地圖 0 沒有事件格")
	}

	w.X, w.Y = x, y
	w.ConsumeEvent()
	if m.HasEvent(x, y) {
		t.Fatalf("(%d,%d) 的事件旗標沒被清掉", x, y)
	}

	w.MapIndex = 1
	w.CurrentMap()
	w.MapIndex = 0
	if !w.CurrentMap().HasEvent(x, y) {
		t.Errorf("回到地圖 0 之後事件旗標沒有回來")
	}
}
