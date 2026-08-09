package events_test

import (
	"fmt"
	"sort"
	"os"
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
					t.Logf("段0 格%d → Index %d（腳本 %d，共 %d 條）Kind %#02x：%s",
						e.Cell, e.Index, si, len(sg.Scripts), e.Kind, head)
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
