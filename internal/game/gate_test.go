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

// 屬性層的奇數位元是**整格的旗標**，讀的地方是當前格的快取 `ds:59C8`。
//
// 只掃平面（`ds:5AD6`）找不到讀取點 —— 那正是這三個位元掛了很久的原因。
func TestAttrOddBitsHaveMeaning(t *testing.T) {
	ms, err := game.ParseMaps(orig(t, "MAP.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	count := func(bit byte) (cells, maps int) {
		for i := range ms {
			c := 0
			for _, v := range ms[i].Attr {
				if v&bit != 0 {
					c++
				}
			}
			if c > 0 {
				maps++
			}
			cells += c
		}
		return
	}
	// bit 3 禁休息、bit 1 禁施法：兩者都要真的有格子，而且不能是全圖
	// （全圖表示位元讀錯了）。
	for _, tc := range []struct {
		name string
		bit  byte
	}{{"不能休息", game.AttrNoRest}, {"不能施法", game.AttrNoMagic}} {
		cells, maps := count(tc.bit)
		if cells == 0 {
			t.Errorf("%s 的位元一格都沒有 —— 位元選錯了", tc.name)
		}
		if cells > game.MapCells*len(ms)/4 {
			t.Errorf("%s 有 %d 格，超過四分之一，不像旗標", tc.name, cells)
		}
		t.Logf("%s：%d 格、%d 張圖", tc.name, cells, maps)
	}

	// 禁施法的格子真的擋得住施法
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, cs, nil, 1)
	found := false
	for i := range ms {
		for c := 0; c < game.MapCells; c++ {
			if ms[i].Attr[c]&game.AttrNoMagic == 0 {
				continue
			}
			w.MapIndex, w.X, w.Y = i, c%game.MapW, c/game.MapW
			// 找一個會施法的人
			for who := range s.Party {
				if !game.CanCast(s.Party[who].Class) {
					continue
				}
				if r := s.Cast(who, 1); r.OK {
					t.Errorf("地圖 %d 的 (%d,%d) 禁止施法，卻施成了", i, w.X, w.Y)
				}
				found = true
				break
			}
			break
		}
		if found {
			break
		}
	}
	if !found {
		t.Skip("隊伍裡沒有施法職業")
	}
}
