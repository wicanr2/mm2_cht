package game_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

func orig(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "workplace", "orig", "MM2", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("找不到原版檔案 %s（玩家自備合法原版，解到 workplace/orig/）", path)
	}
	return b
}

func newWorld(t *testing.T) *game.World {
	t.Helper()
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestParseMaps(t *testing.T) {
	maps, err := game.ParseMaps(orig(t, "MAP.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	if len(maps) != 60 {
		t.Fatalf("解出 %d 張地圖，預期 60", len(maps))
	}
}

// 屬性層 bit3 與事件表的 Cell 必須完全對得上：
// 五座城的事件格 100% 都設了這個位元（docs/formats/06-map.md §2）。
// 這條同時守著「MAP 段 k 對應 EVENTSI 段 k」。
func TestEventCellsAllFlagged(t *testing.T) {
	w := newWorld(t)
	for _, mi := range []int{0, 1, 2, 3, 4} {
		w.MapIndex = mi
		m := w.CurrentMap()
		n, flagged := 0, 0
		for c := 0; c < game.MapCells; c++ {
			x, y := c%game.MapW, c/game.MapW
			if w.EventAt(x, y) == nil {
				continue
			}
			n++
			if m.HasEvent(x, y) {
				flagged++
			}
		}
		if n == 0 {
			t.Errorf("地圖 %d 沒有事件", mi)
			continue
		}
		if flagged != n {
			t.Errorf("地圖 %d: %d 個事件格裡只有 %d 個設了 bit3", mi, n, flagged)
		}
	}
}

// 錯開一格就對不上 —— 這條守的是「段編號沒有整體位移」。
func TestMapEventPairingIsExact(t *testing.T) {
	w := newWorld(t)
	maps, err := game.ParseMaps(orig(t, "MAP.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	w.MapIndex = 0
	var cells []int
	for c := 0; c < game.MapCells; c++ {
		if w.EventAt(c%game.MapW, c/game.MapW) != nil {
			cells = append(cells, c)
		}
	}
	match := func(m *game.Map) int {
		n := 0
		for _, c := range cells {
			if m.Attr[c]&game.AttrHasEvent != 0 {
				n++
			}
		}
		return n
	}
	if got := match(&maps[0]); got != len(cells) {
		t.Fatalf("地圖 0 只對上 %d/%d", got, len(cells))
	}
	if got := match(&maps[1]); got == len(cells) {
		t.Error("地圖 1 也全對上了，配對條件失效")
	}
}

func TestMoveAndTurn(t *testing.T) {
	w := newWorld(t)
	w.MapIndex = 0
	w.X, w.Y, w.Face = 8, 8, game.North

	if !w.Move(1) || w.Y != 7 {
		t.Errorf("向北走一步後 y=%d，預期 7", w.Y)
	}
	w.Turn(1)
	if w.Face != game.East {
		t.Errorf("右轉後朝向 %v，預期 E", w.Face)
	}
	if !w.Move(1) || w.X != 9 {
		t.Errorf("向東走一步後 x=%d，預期 9", w.X)
	}

	// 走出邊界要原地不動
	w.X, w.Y, w.Face = 0, 0, game.North
	if w.Move(1) {
		t.Error("在北緣往北走居然成功了")
	}
	if w.X != 0 || w.Y != 0 {
		t.Errorf("撞邊界後位置變成 (%d,%d)", w.X, w.Y)
	}
}

// 走到事件格要拿得到文字。Middlegate 第一筆事件在第 8 格（x=8, y=0）。
func TestEventTriggersMessage(t *testing.T) {
	w := newWorld(t)
	w.MapIndex = 0
	w.X, w.Y, w.Face = 8, 1, game.North
	if !w.Move(1) {
		t.Fatal("移動失敗")
	}
	if w.X != 8 || w.Y != 0 {
		t.Fatalf("位置是 (%d,%d)，預期 (8,0)", w.X, w.Y)
	}
	if w.EventAt(8, 0) == nil {
		t.Fatal("(8,0) 應該有事件")
	}
	if w.Message == "" {
		t.Error("踩到事件格但沒有訊息")
	}
}
