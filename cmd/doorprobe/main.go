// doorprobe 是 O4（門狀態 DOSBox 複驗）的排路工具。
//
// 慢回饋的驗證要先把證據看齊：DOSBox 跑一輪要好幾分鐘，路線猜錯就白跑。
// 這支從 `MAP.DAT` 直接算出「門在哪、怎麼走過去、走到哪裡會換圖」，
// 再把 timeline 的按鍵序列印出來。
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/game"
)

type pt struct{ X, Y int }

type dir struct {
	dx, dy int
	bit    uint
	key    string
}

// 北是 Y 增加（`docs/playtest/02` 用記憶體傾印量過）。
var dirs = []dir{{0, 1, 6, "N"}, {1, 0, 4, "E"}, {0, -1, 2, "S"}, {-1, 0, 0, "W"}}

func main() {
	mapIndex := 0
	if len(os.Args) > 1 {
		fmt.Sscan(os.Args[1], &mapIndex)
	}
	read := func(n string) []byte {
		b, err := os.ReadFile("workplace/orig/MM2/" + n)
		if err != nil {
			fmt.Println("讀不到", n, err)
			os.Exit(1)
		}
		return b
	}
	w, err := game.NewWorld(read("MAP.DAT"), read("EVENTSI.DAT"))
	if err != nil {
		fmt.Println("載入失敗:", err)
		os.Exit(1)
	}
	w.MapIndex = mapIndex
	m := w.CurrentMap()
	if m == nil {
		fmt.Println("沒有這張地圖")
		os.Exit(1)
	}
	attrs, err := game.ParseMapAttrs(read("ATTRIB.DAT"))
	if err != nil {
		fmt.Println("ATTRIB 解不開:", err)
		os.Exit(1)
	}
	if mapIndex < len(attrs) {
		a := &attrs[mapIndex]
		fmt.Printf("地圖 %d：撞門難度 %d、開鎖難度 %d、室內 %v；鄰接 北%d 南%d 東%d 西%d\n",
			mapIndex, a.BashDifficulty(), a.LockDifficulty(), a.Indoor(),
			a.North(), a.South(), a.East(), a.West())
	}

	// `doorprobe <地圖> attr` 印出該圖的屬性層 256 bytes（十六進位），
	// 拿去跟 DOSBox 傾印裡的 `ds:5AD6` 比對 —— 這是「換圖會不會重讀整層」
	// 的直接判準。
	if len(os.Args) > 2 && os.Args[2] == "attr" {
		for i := 0; i < 256; i++ {
			fmt.Printf("%02x", m.Attr[i])
		}
		fmt.Println()
		return
	}

	start := pt{7, 3} // 旅店的落點（`ds:21E8` 第 0 項），面北
	// 第二、三個參數給「終點 x y」時，直接印 DOSBox timeline 的按鍵序列。
	// 手推轉向序列會錯，而 DOSBox 跑一輪好幾分鐘 —— 讓機器算。
	if len(os.Args) > 3 {
		var to pt
		fmt.Sscan(os.Args[2], &to.X)
		fmt.Sscan(os.Args[3], &to.Y)
		if len(os.Args) > 5 {
			fmt.Sscan(os.Args[4], &start.X)
			fmt.Sscan(os.Args[5], &start.Y)
		}
		seen, prev, via := bfs(m, start, true)
		if !seen[to] {
			fmt.Printf("(%d,%d) → (%d,%d) 走不到（門也算開的情況下）\n", start.X, start.Y, to.X, to.Y)
			os.Exit(1)
		}
		var moves []string
		for p := to; p != start; p = prev[p] {
			moves = append([]string{via[p]}, moves...)
		}
		fmt.Printf("%v → %v：%v\n", start, to, moves)
		fmt.Println(keysFor(moves, "N"))
		return
	}
	for _, doors := range []bool{false, true} {
		seen, prev, via := bfs(m, start, doors)
		label := "不開門"
		if doors {
			label = "把門當成走得過去"
		}
		fmt.Printf("\n== %s：走得到 %d 格\n", label, len(seen))
		route := func(to pt) string {
			var out []string
			for p := to; p != start; p = prev[p] {
				out = append([]string{via[p]}, out...)
			}
			return strings.Join(out, " ")
		}
		var cells []pt
		for p := range seen {
			cells = append(cells, p)
		}
		sort.Slice(cells, func(i, j int) bool {
			if cells[i].Y != cells[j].Y {
				return cells[i].Y < cells[j].Y
			}
			return cells[i].X < cells[j].X
		})
		fmt.Print("   門：")
		n := 0
		for _, p := range cells {
			c := game.Cell(p.X, p.Y)
			for _, d := range dirs {
				if (m.Terrain[c]>>d.bit)&3 != 2 {
					continue
				}
				n++
				// 牆位元就是 `wallBit`（北 6／東 4／南 2／西 0），**不是 +1** ——
				// 奇數位元是整格的旗標。差一位的症狀是「門看起來都是關的」，
				// 而那與「真的關著」長得一模一樣（`Map.HasWall` 是準）。
				fmt.Printf("\n     (%2d,%2d) 朝 %s，牆位元 %d（%s），路線 [%s]",
					p.X, p.Y, d.key, (m.Attr[c]>>d.bit)&1,
					map[bool]string{true: "關著", false: "開著"}[(m.Attr[c]>>d.bit)&1 == 1],
					route(p))
			}
		}
		if n == 0 {
			fmt.Print("（沒有）")
		}
		fmt.Println()
		// 換圖事件：走到那一格、面對指定方向就換圖
		if seg := w.EventSegment(); seg != nil && doors {
			fmt.Println("   換圖事件（opcode 0x0c）：")
			for _, ev := range seg.Events {
				var body []byte
				if int(ev.Index) < len(seg.Scripts) {
					body = seg.Scripts[ev.Index]
				}
				for i := 0; i+2 < len(body); i++ {
					if body[i] != 0x0c {
						continue
					}
					p := pt{int(ev.Cell) % 16, int(ev.Cell) / 16}
					r := "走不到"
					if seen[p] {
						r = "[" + route(p) + "]"
					}
					fmt.Printf("     格 %3d (%2d,%2d) 方向遮罩 %02X → 地圖 %d 的 (%d,%d)　路線 %s\n",
						ev.Cell, p.X, p.Y, ev.Kind&0xF0, body[i+1]&0x3F,
						body[i+2]&0x0F, body[i+2]>>4, r)
					break
				}
			}
		}
	}
}

// keysFor 把絕對方向的路線換成 DOSBox 的按鍵，含轉向。
// 每走一步補一次 space —— 事件訊息會等按鍵，不補的話後面整串全錯位。
func keysFor(moves []string, facing string) string {
	order := []string{"N", "E", "S", "W"}
	idx := func(d string) int {
		for i, o := range order {
			if o == d {
				return i
			}
		}
		return 0
	}
	var out []string
	cur := idx(facing)
	for _, mv := range moves {
		want := idx(mv)
		for turn := (want - cur + 4) % 4; turn != 0; turn = (want - cur + 4) % 4 {
			if turn == 3 {
				out = append(out, "key:Left;wait:1")
				cur = (cur + 3) % 4
			} else {
				out = append(out, "key:Right;wait:1")
				cur = (cur + 1) % 4
			}
		}
		out = append(out, "key:Up;wait:1;key:space;wait:1")
	}
	return strings.Join(out, ";")
}

func bfs(m *game.Map, start pt, doors bool) (map[pt]bool, map[pt]pt, map[pt]string) {
	// **擋路的是屬性層的牆位元，不是地形層的兩位元**（`Map.HasWall`）。
	// 地形層只說「那一面是什麼」（1 牆 2 門 3 有訊息的牆），開門翻的是
	// 屬性層。所以照屬性層走才是玩家實際走得到的範圍；`doors` 那一輪
	// 額外把「地形是門」的牆當成走得過去，代表「撞開之後」。
	pass := func(p pt, bit uint) bool {
		c := game.Cell(p.X, p.Y)
		if c < 0 {
			return false
		}
		if (m.Attr[c]>>bit)&1 == 0 {
			return true
		}
		return doors && (m.Terrain[c]>>bit)&3 == 2
	}
	seen := map[pt]bool{start: true}
	prev := map[pt]pt{}
	via := map[pt]string{}
	q := []pt{start}
	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		for _, d := range dirs {
			if !pass(p, d.bit) {
				continue
			}
			n := pt{p.X + d.dx, p.Y + d.dy}
			if n.X < 0 || n.X > 15 || n.Y < 0 || n.Y > 15 || seen[n] {
				continue
			}
			seen[n], prev[n], via[n] = true, p, d.key
			q = append(q, n)
		}
	}
	return seen, prev, via
}
