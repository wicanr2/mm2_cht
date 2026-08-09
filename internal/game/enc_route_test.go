package game_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 排出「地圖 4 的 (8,1) → 固定遭遇格 (12,15)」的按鍵序列。
//
// 地圖 4 的固定遭遇在 EVENTSI 段 4 格 252（`Segment.Index` 就是地圖號），
// 走上去必定觸發，是拍戰鬥畫面最可靠的路。起點 (8,1) 是從中門西門
// 出去之後 dump 讀到的位置。
func TestEncounterRoute(t *testing.T) {
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	// 候選：地圖 → 固定遭遇格（EVENTSI 段號就是地圖號）。
	for _, c := range []struct{ mapIdx, gx, gy int }{
		{2, 3, 5}, {3, 10, 5}, {4, 12, 15},
	} {
		w := newWorld(t)
		w.MapIndex = c.mapIdx
		m := w.CurrentMap()
		if m == nil || c.mapIdx >= len(attrs) {
			continue
		}
		sx, sy, ok := attrs[c.mapIdx].Entry()
		if !ok {
			t.Logf("地圖 %d 沒有預設進入座標", c.mapIdx)
			continue
		}
		gx, gy := c.gx, c.gy
		reach(t, m, sx, sy, gx, gy, c.mapIdx)
	}
}

func reach(t *testing.T, m *game.Map, sx, sy, gx, gy, mapIdx int) {

	type node struct{ x, y int }
	prev := map[node]node{}
	seen := map[node]bool{{sx, sy}: true}
	queue := []node{{sx, sy}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.x == gx && cur.y == gy {
			break
		}
		for f := 0; f < 4; f++ {
			if !m.CanMove(cur.x, cur.y, game.Facing(f)) {
				continue
			}
			dx, dy := game.Facing(f).Delta()
			n := node{cur.x + dx, cur.y + dy}
			if n.x < 0 || n.x > 15 || n.y < 0 || n.y > 15 || seen[n] {
				continue
			}
			seen[n] = true
			prev[n] = cur
			queue = append(queue, n)
		}
	}
	if !seen[node{gx, gy}] {
		// 走不到不是錯 —— 那一格可能要從別的入口進，或先觸發什麼才開。
		t.Logf("地圖 %d：從入口 (%d,%d) **走不到** 遭遇格 (%d,%d)（可達 %d 格）",
			mapIdx, sx, sy, gx, gy, len(seen))
		return
	}
	path := []node{{gx, gy}}
	for p := (node{gx, gy}); p != (node{sx, sy}); {
		p = prev[p]
		path = append([]node{p}, path...)
	}
	face := 0
	for f := 0; f < 4; f++ {
		if dx, dy := game.Facing(f).Delta(); dx == 0 && dy == 1 {
			face = f
		}
	}
	var keys []string
	for i := 1; i < len(path); i++ {
		dx, dy := path[i].x-path[i-1].x, path[i].y-path[i-1].y
		want := face
		for f := 0; f < 4; f++ {
			if ddx, ddy := game.Facing(f).Delta(); ddx == dx && ddy == dy {
				want = f
			}
		}
		for face != want {
			keys = append(keys, "key:Right;wait:1")
			face = (face + 1) & 3
		}
		keys = append(keys, "key:Up;wait:1")
	}
	t.Logf("地圖 %d：入口 (%d,%d) → 遭遇格 (%d,%d) 共 %d 步",
		mapIdx, sx, sy, gx, gy, len(path)-1)
	t.Logf("  timeline：%s", strings.Join(keys, ";"))
}
