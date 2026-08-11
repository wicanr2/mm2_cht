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
	// 室內／室外的標記要從 ATTRIB.DAT 來。少了它所有地圖都算野外，
	// 牆位元就不會生效 —— 那正是原版分兩條路的地方。
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range w.Maps {
		if i < len(attrs) {
			w.Maps[i].Indoor = attrs[i].Indoor()
		}
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

// 城鎮的 ATTRIB 鄰接雖然指向自己，外圈牆仍必須先攔住移動；不能因為
// 座標出界就直接跨圖繞過牆。這正是 Middlegate 西側 Sandsobar 提問答 N
// 後，原版下一個前進鍵會顯示 Solid! 的條件。
func TestIndoorBoundaryWallBlocksCrossEdge(t *testing.T) {
	w := newWorld(t)
	// 讓 crossEdge 本身可走，才真的測到「牆先於跨圖」的順序。
	w.Neighbor = make([][4]int, len(w.Maps))
	w.Neighbor[0][game.West] = 0
	w.MapIndex, w.X, w.Y, w.Face = 0, 0, 5, game.West
	m := w.CurrentMap()
	if m == nil || !m.Indoor || !m.HasWall(w.X, w.Y, w.Face) {
		t.Fatal("Middlegate 西側預期是有外圈牆的室內格")
	}
	if w.Move(1) {
		t.Fatal("室內外圈牆被換圖路徑繞過")
	}
	if w.MapIndex != 0 || w.X != 0 || w.Y != 5 {
		t.Errorf("撞西牆後到了圖%d (%d,%d)", w.MapIndex, w.X, w.Y)
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

// 城鎮招牌只有 `04 NN`，原版畫出名稱卻不等玩家確認。這條與 DOSBox 的
// 新局路徑對照，避免 UI 又把招牌誤做成停住移動的對話。
func TestFacilitySignsDoNotWaitForConfirm(t *testing.T) {
	w := newWorld(t)
	w.MapIndex, w.X, w.Y, w.Face = 0, 7, 4, game.North

	for _, want := range []string{"Middlegate Inn", "Gateway Temple"} {
		if !w.Move(1) {
			t.Fatalf("走到招牌 %q 前走不動", want)
		}
		if w.Message != want {
			t.Fatalf("招牌是 %q，預期 %q", w.Message, want)
		}
		if w.MessageWait {
			t.Errorf("招牌 %q 不該執行等鍵 opcode", want)
		}
	}
}

func TestWaitKeyMarksEventMessageBlocking(t *testing.T) {
	w := newWorld(t)
	w.RunScriptForTest([]byte{game.OpWaitKey})
	if !w.MessageWait {
		t.Error("0x07 沒有標記為等待玩家輸入")
	}
}

// 腳本不含顯示字串的格子，不能憑其他效果臆造一條訊息。
// 顯示與狀態改變是兩條獨立路徑。
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

// 門是牆的一個子集，而且**只出現在室內圖**。把室內外分開之後，
// 門佔牆面的比例是 1.8%（283 面 / 36 張），對向一致率 99.7%。
// 這一條釘住「門是少數」與「野外沒有門」兩件事。
func TestWallKindDoors(t *testing.T) {
	w := newWorld(t)
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range w.Maps {
		if i < len(attrs) {
			w.Maps[i].Indoor = attrs[i].Indoor()
		}
	}
	outdoorDoors := 0
	solid, door := 0, 0
	for i := range w.Maps {
		m := &w.Maps[i]
		for y := 0; y < game.MapW; y++ {
			for x := 0; x < game.MapW; x++ {
				for f := game.Facing(0); f < 4; f++ {
					switch m.WallKind(x, y, f) {
					case game.WallSolid:
						solid++
					case game.WallDoor:
						door++
						if !m.Indoor {
							outdoorDoors++
						}
					}
				}
			}
		}
	}
	if solid+door == 0 {
		t.Fatal("一面牆都沒有")
	}
	ratio := float64(door) / float64(solid+door)
	if ratio > 0.4 {
		t.Errorf("門佔了 %.1f%% 的牆面，判準可能反了", ratio*100)
	}
	if door == 0 {
		t.Error("一扇門都沒有")
	}
	if ratio > 0.05 {
		t.Errorf("門佔了 %.1f%% 的牆面，室內圖應該只有 1.8%% 上下", ratio*100)
	}
	if outdoorDoors != 0 {
		t.Errorf("野外圖出現了 %d 扇門，室內外的判準可能沒生效", outdoorDoors)
	}
}
