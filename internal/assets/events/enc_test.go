package events_test

import (
	"bytes"
	"fmt"
	"sort"
	"os"
	"strings"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// 固定遭遇（opcode `0x12`／`0x13`）的清單。
//
// 用途有兩個：一是確認腳本裡真的有固定遭遇（`EVENTSI` 177 處、
// `EVENTSO` 也有），二是給 DOSBox 的畫面對照一條**確定性路線** ——
// 隨機遇敵拍不到就是拍不到，固定遭遇踩上去必定觸發。
//
// 運算元是 **12 個怪物編號**（0 表示該位置沒有），所以一場固定遭遇的
// 陣容是寫死的 —— 拍出來的畫面可以逐隻比對。
//
// 這支測試同時是「查詢回空」的示範：第一版掃出 0 筆，原因是
// `game.OpLen` 在資料表沒載入時一律回 0，掃描器在第一個 opcode 就
// 停住。**先做正對照**（`OpLen(0x04) >= 1`）才知道掃描器會動。
func TestFixedEncounters(t *testing.T) {
	// OpLen 靠 game 套件的全域資料表，沒載入就一律回 0 ——
	// 掃描器會在第一個 opcode 就停住，看起來像「什麼都沒有」。
	os.Setenv("MM2_DATA_DIR", filepath.Join("..", "..", "..", "data"))
	if err := game.EnsureData(); err != nil {
		t.Skipf("載不到 opcode 長度表：%v", err)
	}
	if game.OpLen(0x04) < 1 {
		t.Fatal("正對照失敗：0x04 的長度是 0，掃描器不會動")
	}
	// 實機從中門西門出去落在地圖 4 的 (8,1)，也就是 opcode `0c 04 18`。
	// 直接在段 0 的原始位元組裡找這三個位元組。
	if b, err := os.ReadFile(filepath.Join("..", "..", "..", "workplace", "orig", "MM2", "EVENTSI.DAT")); err == nil {
		if segs, err := events.Parse(b); err == nil {
			for _, sg := range segs {
				if sg.Index != 0 {
					continue
				}
				for _, want := range [][]byte{{0x0c, 0x04, 0x18}, {0x0c, 0x0b, 0x37}} {
				for i := 0; i+3 <= len(sg.Raw); i++ {
					if sg.Raw[i] == want[0] && sg.Raw[i+1] == want[1] && sg.Raw[i+2] == want[2] {
						lo, hi := i-8, i+6
						if lo < 0 {
							lo = 0
						}
						if hi > len(sg.Raw) {
							hi = len(sg.Raw)
						}
						t.Logf("段0 原始位元組 @%d 找到 % x，前後：% x", i, want, sg.Raw[lo:hi])
					}
				}
				}
			}
		}
	}

	// 「事件數 vs 腳本數」的整體分佈：從單一樣本推規則已經翻車兩次。
	for _, fn := range []string{"EVENTSI.DAT", "EVENTSO.DAT"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", "workplace", "orig", "MM2", fn))
		if err != nil {
			continue
		}
		segs, err := events.Parse(b)
		if err != nil {
			continue
		}
		same, diff, maxIdxOver := 0, 0, 0
		for _, sg := range segs {
			if sg.Irregular {
				continue
			}
			if len(sg.Events) == len(sg.Scripts) {
				same++
			} else {
				diff++
			}
			for _, e := range sg.Events {
				if int(e.Index) > len(sg.Scripts) {
					maxIdxOver++
				}
			}
		}
		t.Logf("%s：事件數＝腳本數的段 %d、不等的 %d；Index 超出腳本數的事件 %d 個",
			fn, same, diff, maxIdxOver)
	}

	// 中門西門通往地圖 11 的 (7,3)＝格 55。那一格有沒有事件？
	if b, err := os.ReadFile(filepath.Join("..", "..", "..", "workplace", "orig", "MM2", "EVENTSO.DAT")); err == nil {
		if segs, err := events.Parse(b); err == nil {
			for _, sg := range segs {
				if sg.Index != 11 {
					continue
				}
				empties := 0
				for _, sc := range sg.Scripts {
					if len(sc) == 0 {
						empties++
					}
				}
				t.Logf("地圖 11：%d 個事件、%d 條腳本（其中空的 %d 條，第 0 條空？%v）",
					len(sg.Events), len(sg.Scripts), empties, len(sg.Scripts[0]) == 0)
				for _, e := range sg.Events {
					if e.Cell != 55 {
						continue
					}
					si := int(e.Index) - 1
					head := "（超出範圍）"
					if si >= 0 && si < len(sg.Scripts) {
						head = fmt.Sprintf("% x", sg.Scripts[si])
					}
					t.Logf("地圖 11 格55 (7,3) → 腳本 %d：%s", si, head)
				}
			}
		}
	}

	// 段 0 的事件表：哪一格指到哪一條腳本，那條腳本開頭是什麼。
	if b, err := os.ReadFile(filepath.Join("..", "..", "..", "workplace", "orig", "MM2", "EVENTSI.DAT")); err == nil {
		if segs, err := events.Parse(b); err == nil {
			for _, sg := range segs {
				if sg.Index != 0 {
					continue
				}
				for _, e := range sg.Events {
					if e.Cell != 80 && e.Cell != 42 {
						continue
					}
					si := int(e.Index) - 1
					head := "（超出範圍）"
					if si >= 0 && si < len(sg.Scripts) {
						head = fmt.Sprintf("% x", sg.Scripts[si])
					}
					t.Logf("段0 格%d → Index %d（腳本 %d，共 %d 條，第 0 條空？%v）Kind %#02x：%s",
						e.Cell, e.Index, si, len(sg.Scripts), len(sg.Scripts[0]) == 0, e.Kind, head)
				}
			}
		}
	}

	for _, name := range []string{"EVENTSI.DAT", "EVENTSO.DAT"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", "workplace", "orig", "MM2", name))
		if err != nil {
			t.Skipf("找不到 %s", name)
		}
		segs, err := events.Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		seen := map[byte]int{}
		maps := map[int]int{}
		for _, sg := range segs {
			for si, sc := range sg.Scripts {
				for p := 0; p < len(sc); {
					op := sc[p]
					l := game.OpLen(op)
					if l < 1 || p+l > len(sc) {
						break
					}
					seen[op]++
					if (op == 0x12 || op == 0x13) || (op == 0x0c && sg.Index == 0 && name == "EVENTSI.DAT") {
						cells := []byte{}
						for _, e := range sg.Events {
							if int(e.Index) == si+1 {
								cells = append(cells, e.Cell)
							}
						}
						if op != 0x0c {
							maps[sg.Index]++
						}
						if sg.Index == 0 && len(cells) > 0 {
							for _, c := range cells {
								end := p + l
								if end > len(sc) {
									end = len(sc)
								}
								t.Logf("%s 段%d 格%d = (X %d, Y %d) opcode %#02x 怪物 %v",
									name, sg.Index, c, c%16, c/16, op, sc[p+1:end])
							}
						}
						n++
					}
					p += l
				}
			}
		}
		t.Logf("%s 固定遭遇共 %d 處；掃到的 opcode 種類 %d 個、總數 %d",
			name, n, len(seen), func() int { s := 0; for _, v := range seen { s += v }; return s }())
		top := ""
		for _, o := range []byte{0x04, 0x02, 0x0e, 0x0c, 0x12, 0x13, 0x09} {
			top += fmt.Sprintf(" %#02x=%d", o, seen[o])
		}
		t.Logf("%s 對照：%s", name, top)
		keys := []int{}
		for k := range maps {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		t.Logf("%s 有固定遭遇的地圖：%v", name, keys)
	}
}

// parseFile 讀原版的事件檔並解開；沒有原版就 skip。
func parseFile(t *testing.T, name string) []events.Segment {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "workplace", "orig", "MM2", name))
	if err != nil {
		t.Skipf("沒有原版 %s，跳過", name)
	}
	segs, err := events.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return segs
}

// 編號 60 以上的段是**沒有事件表的腳本庫**，不是解不出來的段。
//
// 這一組的存在理由：那些段先前被歸進 Irregular 而整段跳過，
// 於是所有掃腳本的工具都看不到它們 —— 城門那個「落點對不上」的
// 矛盾就是這樣被藏了十一輪。
func TestLibrarySegments(t *testing.T) {
	for _, name := range []string{"EVENTSI.DAT", "EVENTSO.DAT"} {
		segs := parseFile(t, name)
		lib := 0
		for _, sg := range segs {
			if !sg.Library {
				continue
			}
			lib++
			if sg.Index < 60 {
				t.Errorf("%s 段 %d 被當成腳本庫，但腳本庫應該都在 60 以上",
					name, sg.Index)
			}
			if len(sg.Events) != 0 {
				t.Errorf("%s 段 %d 是腳本庫，不該有事件表", name, sg.Index)
			}
			if len(sg.Scripts) == 0 || len(sg.Strings) == 0 {
				t.Errorf("%s 段 %d 解出 %d 條腳本、%d 條字串",
					name, sg.Index, len(sg.Scripts), len(sg.Strings))
			}
		}
		if lib == 0 {
			t.Errorf("%s 一段腳本庫都沒認出來", name)
		}
	}
}

// 段 61 腳本 0 是「付 10 金傳送到 Sandsobar」——
// 實機在中門西側 (0,5) 答 y 之後落在地圖 4 的 (8,1)，
// 而全遊戲只有這一處位元組會產生那個落點。
func TestSandsobarTravelScript(t *testing.T) {
	segs := parseFile(t, "EVENTSI.DAT")
	var sg events.Segment
	for _, s := range segs {
		if s.Index == 61 {
			sg = s
		}
	}
	if !sg.Library {
		t.Fatal("段 61 沒有被認成腳本庫")
	}
	if len(sg.Scripts) == 0 {
		t.Fatal("段 61 沒有腳本")
	}
	want := []byte{0x0c, 0x04, 0x18} // 傳送到地圖 4，座標 0x18 → X=8 Y=1
	if !bytes.Contains(sg.Scripts[0], want) {
		t.Errorf("段 61 腳本 0 是 % x，找不到 % x", sg.Scripts[0], want)
	}
	if !strings.Contains(sg.Strings[0], "Sandsobar") {
		t.Errorf("段 61 字串 0 是 %q，預期提到 Sandsobar", sg.Strings[0])
	}
}

// 事件記錄的 Index 是 1 起算：最大的 Index 應該等於腳本條數，
// 不是條數減一。拿它當 0-based 用會整批對到前一條腳本。
func TestEventIndexIsOneBased(t *testing.T) {
	for _, name := range []string{"EVENTSI.DAT", "EVENTSO.DAT"} {
		for _, sg := range parseFile(t, name) {
			if sg.Irregular || sg.Library || len(sg.Events) == 0 {
				continue
			}
			max := 0
			for _, ev := range sg.Events {
				if int(ev.Index) > max {
					max = int(ev.Index)
				}
			}
			if max > len(sg.Scripts) {
				t.Errorf("%s 段 %d 的最大 Index 是 %d，但只有 %d 條腳本",
					name, sg.Index, max, len(sg.Scripts))
			}
		}
	}
}
