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
	w.X, w.Y, w.Face = 7, 8, game.South

	if !w.Move(1) || w.Y != 7 {
		t.Errorf("向南走一步後 y=%d，預期 7", w.Y)
	}
	w.Turn(1)
	if w.Face != game.West {
		t.Errorf("面南右轉後朝向 %v，預期 W", w.Face)
	}

	// 走出邊界要原地不動。第 0 列在南邊，所以 y=0 是南緣。
	w.X, w.Y, w.Face = 0, 0, game.South
	if w.Move(1) {
		t.Error("在南緣往南走居然成功了")
	}
	if w.X != 0 || w.Y != 0 {
		t.Errorf("撞邊界後位置變成 (%d,%d)", w.X, w.Y)
	}
}

// 走進神殿那一格要顯示對應的字串。這條守著事件觸發、腳本 opcode 4
// 與「MAP 段 k 對應 EVENTSI 段 k」。
//
// 路徑取 (7,8) 往南兩步，終點 (7,6) 是事件表 Index=4 所在的格 103 ——
// 手冊的城鎮地圖把神廟標在同一個位置（見 docs/formats/06-map.md §5）。
func TestWalkToTemple(t *testing.T) {
	w := newWorld(t)
	w.MapIndex, w.X, w.Y, w.Face = 0, 7, 8, game.South

	for i := 0; i < 2; i++ {
		if !w.Move(1) {
			t.Fatalf("第 %d 步走不動", i+1)
		}
	}
	if w.X != 7 || w.Y != 6 {
		t.Fatalf("走兩步後在 (%d,%d)，預期 (7,6)", w.X, w.Y)
	}
	if w.Message != "Gateway Temple" {
		t.Errorf("神殿格的訊息是 %q，預期 \"Gateway Temple\"", w.Message)
	}
}

// 腳本不是以「顯示字串」開頭的格子，不能亂猜著顯示東西 ——
// 其餘 opcode 還沒解出來，寧可空白也不要編一條訊息出來。
//
// 已解出的顯示字串 opcode 是 0x01（靠左）與 0x04（置中）。
func TestComplexScriptShowsNothing(t *testing.T) {
	w := newWorld(t)
	w.MapIndex = 0
	m := w.CurrentMap()
	seg := w.EventSegment()
	if seg == nil {
		t.Fatal("找不到 Middlegate 的事件段")
	}

	for y := 0; y < game.MapH; y++ {
		for x := 0; x < game.MapW; x++ {
			ev := w.EventAt(x, y)
			if ev == nil {
				continue
			}
			i := int(ev.Index)
			if i >= len(seg.Scripts) || len(seg.Scripts[i]) == 0 ||
				seg.Scripts[i][0] == game.OpShowString ||
				seg.Scripts[i][0] == game.OpShowStringLeft {
				continue
			}
			// 找一個能走進這一格的方向
			for _, f := range []game.Facing{game.North, game.East, game.South, game.West} {
				dx, dy := f.Delta()
				fx, fy := x-dx, y-dy
				if game.Cell(fx, fy) < 0 || m.HasWall(fx, fy, f) {
					continue
				}
				w.X, w.Y, w.Face = fx, fy, f
				if !w.Move(1) || w.X != x || w.Y != y {
					t.Fatalf("從 (%d,%d) 朝 %v 走不到 (%d,%d)", fx, fy, f, x, y)
				}
				if w.Message != "" {
					t.Errorf("(%d,%d) 的腳本以 opcode %#x 開頭，不該顯示訊息，卻顯示了 %q",
						x, y, seg.Scripts[i][0], w.Message)
				}
				return
			}
		}
	}
	t.Skip("Middlegate 沒有「腳本非顯示字串、且走得進去」的事件格")
}
