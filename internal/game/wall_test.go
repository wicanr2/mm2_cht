package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 牆有兩面：格 (x,y) 的東側位元必須等於格 (x+1,y) 的西側位元。
// 第 0 列在南邊，所以 y+1 那一格在北 —— 南北向要比的是
// (x,y) 的北側與 (x,y+1) 的南側。
// 這條不靠任何 oracle，純粹是資料自己要對得起自己 —— 方向與位元的
// 對應就是從這裡定出來的（見 docs/formats/06-map.md §4）。
//
// 不是 100%：野外地圖不用牆，地圖邊界也不受這條約束。定案的排列是 93.8%，
// 次高的排列只有 86.7%，隨機期望 50%，所以門檻設在 92%。
func TestWallsAgreeFromBothSides(t *testing.T) {
	maps, err := game.ParseMaps(orig(t, "MAP.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	same, total := 0, 0
	for i := range maps {
		m := &maps[i]
		for y := 0; y < game.MapH; y++ {
			for x := 0; x < game.MapW; x++ {
				if x+1 < game.MapW {
					if m.HasWall(x, y, game.East) == m.HasWall(x+1, y, game.West) {
						same++
					}
					total++
				}
				if y+1 < game.MapH {
					if m.HasWall(x, y, game.North) == m.HasWall(x, y+1, game.South) {
						same++
					}
					total++
				}
			}
		}
	}
	if r := float64(same) / float64(total); r < 0.92 {
		t.Errorf("牆面自洽率 %.1f%%，低於門檻 92%%", r*100)
	}
}

// 城鎮是封閉的：Middlegate 最外圈朝外的每一面都要有牆。
// 第 0 列在南邊，所以 y=0 那一排朝外的是南面。
// 否則走一走就掉出地圖。方向對應錯了這條立刻爆。
func TestMiddlegateIsEnclosed(t *testing.T) {
	maps, err := game.ParseMaps(orig(t, "MAP.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	m := &maps[0]
	var open [][3]int
	for i := 0; i < game.MapW; i++ {
		for _, c := range [][3]int{
			{i, 0, int(game.South)}, {i, game.MapH - 1, int(game.North)},
			{0, i, int(game.West)}, {game.MapW - 1, i, int(game.East)},
		} {
			if !m.HasWall(c[0], c[1], game.Facing(c[2])) {
				open = append(open, c)
			}
		}
	}
	if len(open) > 0 {
		t.Errorf("外圈有 %d 面沒牆，例如 %v", len(open), open[:min(3, len(open))])
	}
}

// 撞牆要走不動，而且位置不能變。
func TestWallBlocksMovement(t *testing.T) {
	w := newWorld(t)
	w.MapIndex = 0
	m := w.CurrentMap()

	for y := 1; y < game.MapH-1; y++ {
		for x := 1; x < game.MapW-1; x++ {
			for _, f := range []game.Facing{game.North, game.East, game.South, game.West} {
				if !m.HasWall(x, y, f) {
					continue
				}
				w.X, w.Y, w.Face = x, y, f
				if w.Move(1) {
					t.Fatalf("(%d,%d) 朝 %v 有牆，卻走過去了", x, y, f)
				}
				if w.X != x || w.Y != y {
					t.Fatalf("撞牆後位置變成 (%d,%d)", w.X, w.Y)
				}
				return
			}
		}
	}
	t.Fatal("Middlegate 找不到任何一面牆")
}

// 後退看的是背後那面牆，不是前方那面。
func TestBackStepChecksRearWall(t *testing.T) {
	w := newWorld(t)
	w.MapIndex = 0
	m := w.CurrentMap()
	for y := 1; y < game.MapH-1; y++ {
		for x := 1; x < game.MapW-1; x++ {
			if m.HasWall(x, y, game.North) || !m.HasWall(x, y, game.South) {
				continue
			}
			w.X, w.Y, w.Face = x, y, game.North
			if w.Move(-1) {
				t.Fatalf("(%d,%d) 面北，南邊有牆卻後退成功了", x, y)
			}
			w.X, w.Y = x, y
			if !w.Move(1) {
				t.Fatalf("(%d,%d) 面北，北邊沒牆卻前進失敗", x, y)
			}
			return
		}
	}
	t.Skip("Middlegate 沒有「前方可走、後方有牆」的格子")
}
