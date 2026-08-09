package game_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 用引擎的牆模型排出「從進城起點走到城門」的按鍵序列，給實機驗證用。
//
// 走得到就同時驗證了牆模型；走不到就知道是哪一格判斷不一樣。
// **已用實機驗證過前兩段**：北 2 步到 (7,5)、轉西再 3 步到 (4,5)，
// 兩次都用 `dump:` 讀 `ds:0393`／`ds:0394` 對過（見 docs/playtest §8）。
// 起點 (7,3) 是記憶體 dump 量到的，城門 (0,5) 來自 EVENTSI 段 0 的
// opcode 0x0c。
func TestGateRoute(t *testing.T) {
	w := newWorld(t)
	m := w.CurrentMap()
	if m == nil {
		t.Skip("沒有地圖")
	}
	const sx, sy, gx, gy = 7, 3, 0, 5

	type node struct{ x, y int }
	prev := map[node]node{}
	seen := map[node]bool{{sx, sy}: true}
	queue := []node{{sx, sy}}
	found := false
	for len(queue) > 0 && !found {
		cur := queue[0]
		queue = queue[1:]
		for f := 0; f < 4; f++ {
			if !m.CanMove(cur.x, cur.y, game.Facing(f)) {
				continue
			}
			dx, dy := game.Facing(f).Delta()
			nx, ny := cur.x+dx, cur.y+dy
			if nx < 0 || nx > 15 || ny < 0 || ny > 15 {
				continue
			}
			n := node{nx, ny}
			if seen[n] {
				continue
			}
			seen[n] = true
			prev[n] = cur
			if n.x == gx && n.y == gy {
				found = true
				break
			}
			queue = append(queue, n)
		}
	}
	if !found {
		t.Fatalf("引擎的牆模型走不到城門 (%d,%d)", gx, gy)
	}
	// 還原路徑
	path := []node{{gx, gy}}
	for p := (node{gx, gy}); p != (node{sx, sy}); {
		p = prev[p]
		path = append([]node{p}, path...)
	}

	// 轉成按鍵：先轉到該方向，再前進。原版 Left 是逆時針。
	face := -1
	for f := 0; f < 4; f++ {
		if dx, dy := game.Facing(f).Delta(); dx == 0 && dy == 1 {
			face = f // 進城時朝北
		}
	}
	var keys []string
	for i := 1; i < len(path); i++ {
		dx, dy := path[i].x-path[i-1].x, path[i].y-path[i-1].y
		want := -1
		for f := 0; f < 4; f++ {
			if ddx, ddy := game.Facing(f).Delta(); ddx == dx && ddy == dy {
				want = f
			}
		}
		for face != want {
			// 每個轉向都要自己的 wait —— 連著送 DOSBox 會掉鍵，
			// 而掉鍵的症狀與「牆算錯」長得一樣（見 docs/playtest）。
			keys = append(keys, "key:Right;wait:1")
			face = (face + 1) & 3
		}
		keys = append(keys, "key:Up;wait:1")
	}
	t.Logf("路徑 %d 格：%v", len(path)-1, path)
	t.Logf("timeline 片段：\n%s", strings.Join(keys, ";"))
	if len(path)-1 > 40 {
		t.Errorf("路徑 %d 步，太長了，八成繞錯", len(path)-1)
	}
	_ = fmt.Sprint
}
