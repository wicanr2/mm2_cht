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

// 從原版的起始位置面北走四步會進神殿，這是對照原版行為的基準
// （原版走四步顯示 "A slim cleric in a cowled robe…"）。
// 這條同時守著起始座標、移動方向、事件觸發與腳本 opcode 4。
func TestWalkToTempleFromStart(t *testing.T) {
	w := newWorld(t)
	s := game.StartMiddlegate
	w.MapIndex, w.X, w.Y, w.Face = s.Map, s.X, s.Y, s.Face

	for i := 0; i < 4; i++ {
		if !w.Move(1) {
			t.Fatalf("第 %d 步走不動", i+1)
		}
	}
	if w.X != 7 || w.Y != 6 {
		t.Fatalf("走四步後在 (%d,%d)，預期 (7,6)", w.X, w.Y)
	}
	if w.Message != "Gateway Temple" {
		t.Errorf("神殿格的訊息是 %q，預期 \"Gateway Temple\"", w.Message)
	}
}

// 腳本不是單純顯示字串的格子，不能亂猜著顯示東西。
// (8,0) 是登山術訓練，腳本以 opcode 1 開頭（詢問／扣錢／給技能），
// 那些 opcode 還沒實作，所以不該有訊息。
func TestComplexScriptShowsNothing(t *testing.T) {
	w := newWorld(t)
	w.MapIndex = 0
	w.X, w.Y, w.Face = 8, 1, game.North
	if !w.Move(1) || w.X != 8 || w.Y != 0 {
		t.Fatalf("位置是 (%d,%d)，預期 (8,0)", w.X, w.Y)
	}
	if w.EventAt(8, 0) == nil {
		t.Fatal("(8,0) 應該有事件")
	}
	if w.Message != "" {
		t.Errorf("複雜腳本不該顯示訊息，卻顯示了 %q", w.Message)
	}
}
