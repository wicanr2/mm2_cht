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
	w := newWorld(t)
	w.MapIndex = 4
	m := w.CurrentMap()
	if m == nil {
		t.Skip("沒有地圖 4")
	}
	const sx, sy, gx, gy = 8, 1, 12, 15

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
		// 走不到不是錯 —— 城鎮本來就可能被牆分成幾塊，
		// (12,15) 也可能要從別的入口進去。記下來換一張圖試。
		t.Skipf("地圖 4 上從 (%d,%d) 走不到遭遇格 (%d,%d)：可走到的格子只有 %d 個",
			sx, sy, gx, gy, len(seen))
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
	t.Logf("路徑 %d 步", len(path)-1)
	t.Logf("timeline：%s", strings.Join(keys, ";"))
}
