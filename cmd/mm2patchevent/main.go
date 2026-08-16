// mm2patchevent 在事件檔裡加一筆事件，用來把原版帶到平常走不到的畫面。
//
// 這是 **oracle 工具**，不是遊戲的一部分：有些原版行為只在遊戲最後才看得到
// （例如結局控制室），而「從頭玩到那裡」在自動化流程裡做不到。與其放棄驗證，
// 不如把那一格的事件搬到起點旁邊 —— **執行的是原版自己的程式碼，
// 只有觸發地點被搬動**。
//
//	go run ./cmd/mm2patchevent \
//	    -in workplace/orig/MM2/EVENTSI.DAT \
//	    -out workplace/dosbox/game/EVENTSI.DAT \
//	    -segment 0 -cells 39,54,56,71 -script 0efd
//
// `-list` 只印出目前的事件表與腳本，不寫檔。
//
// **輸出一定要寫到可寫的副本**（`workplace/dosbox/game/`），
// `workplace/orig/` 是唯讀參考。
package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

func main() {
	in := flag.String("in", "workplace/orig/MM2/EVENTSI.DAT", "原版事件檔")
	out := flag.String("out", "", "輸出檔（不給就只列出，不寫檔）")
	segIdx := flag.Int("segment", 0, "要改哪一段")
	cells := flag.String("cells", "", "要加事件的格號，以 , 分隔（格號 = Y×16 + X）")
	script := flag.String("script", "0efd", "新腳本的位元組（十六進位）")
	kind := flag.Int("kind", 0xF0, "事件的方向遮罩，預設 0xF0（四個方向都觸發）")
	retarget := flag.String("retarget", "", "把這些格**既有**的事件改指到新腳本，以 , 分隔")
	list := flag.Bool("list", false, "列出這一段目前的事件表與腳本")
	flag.Parse()

	blob, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	offs, order := segmentOffsets(blob)
	if *segIdx < 0 || *segIdx >= events.SegmentCount || offs[*segIdx] == 0 {
		log.Fatalf("段 %d 是空的或超出範圍", *segIdx)
	}
	raw, err := lzw.Segment(blob, offs[*segIdx])
	if err != nil {
		log.Fatalf("段 %d 解不開：%v", *segIdx, err)
	}
	lay, err := layout(raw)
	if err != nil {
		log.Fatalf("段 %d 不是事件表佈局：%v", *segIdx, err)
	}

	if *list || *out == "" {
		printLayout(*segIdx, raw, lay)
		if *out == "" {
			return
		}
	}

	body, err := hex.DecodeString(strings.TrimSpace(*script))
	if err != nil || len(body) == 0 {
		log.Fatalf("-script 要是十六進位位元組，收到 %q", *script)
	}
	var want []int
	for _, s := range strings.Split(*cells, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 || n > 255 {
			log.Fatalf("格號 %q 不合法（0–255）", s)
		}
		want = append(want, n)
	}
	if len(want) == 0 {
		log.Fatal("沒有給 -cells，沒有東西可以加")
	}

	var reuse []int
	for _, s := range strings.Split(*retarget, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 || n > 255 {
			log.Fatalf("-retarget 的格號 %q 不合法", s)
		}
		reuse = append(reuse, n)
	}

	patched, idx, added, err := addEvent(raw, lay, want, reuse, body, byte(*kind))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("新腳本編號 %d（%s），加到格 %v\n", idx, hex.EncodeToString(body), added)

	newBlob := rebuild(blob, offs, order, *segIdx, patched)
	if err := os.WriteFile(*out, newBlob, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s → %s（%d bytes → %d bytes）\n", *in, *out, len(blob), len(newBlob))

	// 立刻回讀驗證：改完的檔要能被同一支解析器解開，而且新事件在表上。
	back, err := events.Parse(newBlob)
	if err != nil {
		log.Fatalf("寫出來的檔解不開：%v", err)
	}
	for _, s := range back {
		if s.Index != *segIdx {
			continue
		}
		if idx >= len(s.Scripts) {
			log.Fatalf("回讀之後只有 %d 條腳本，取不到第 %d 條", len(s.Scripts), idx)
		}
		if got := hex.EncodeToString(s.Scripts[idx]); got != hex.EncodeToString(body) {
			log.Fatalf("回讀的第 %d 條腳本是 %s，預期 %s", idx, got, hex.EncodeToString(body))
		}
		n := 0
		for _, e := range s.Events {
			for _, c := range added {
				if int(e.Cell) == c && int(e.Index) == idx {
					n++
				}
			}
		}
		if n != len(added) {
			log.Fatalf("回讀只找到 %d 筆新事件，預期 %d 筆", n, len(added))
		}
		fmt.Printf("回讀通過：%d 筆事件、%d 條腳本、%d 條字串\n",
			len(s.Events), len(s.Scripts), len(s.Strings))
	}
}

// segmentOffsets 回傳索引表，以及非空段依偏移排序後的段號。
func segmentOffsets(blob []byte) (offs [events.SegmentCount]int, order []int) {
	for i := range offs {
		offs[i] = int(binary.LittleEndian.Uint32(blob[i*4:]))
		if offs[i] != 0 {
			order = append(order, i)
		}
	}
	sort.Slice(order, func(a, b int) bool { return offs[order[a]] < offs[order[b]] })
	return offs, order
}

// segLayout 是一段解壓後的三個界線。
type segLayout struct {
	tableEnd int // 事件表結束（含 00 00 00）之後的位置，也就是 skip 欄位所在
	scriptAt int // 腳本區起點 = tableEnd + 2
	strAt    int // 字串區起點 = tableEnd + skip
	skip     int
}

func layout(raw []byte) (segLayout, error) {
	p, lastCell := 0, -1
	for p+3 <= len(raw) {
		a, b, c := raw[p], raw[p+1], raw[p+2]
		p += 3
		if a == 0 && b == 0 && c == 0 {
			if p+2 > len(raw) {
				return segLayout{}, fmt.Errorf("事件表之後放不下 skip")
			}
			skip := int(binary.LittleEndian.Uint16(raw[p:]))
			at := p + skip
			if at > len(raw) || skip < 2 {
				return segLayout{}, fmt.Errorf("skip = %d 指到段外", skip)
			}
			return segLayout{tableEnd: p, scriptAt: p + 2, strAt: at, skip: skip}, nil
		}
		if int(a) <= lastCell {
			return segLayout{}, fmt.Errorf("格號在第 %d 筆不再遞增", p/3)
		}
		lastCell = int(a)
		_ = b
		_ = c
	}
	return segLayout{}, fmt.Errorf("掃不到 00 00 00 終止")
}

func printLayout(idx int, raw []byte, lay segLayout) {
	fmt.Printf("段 %d：解壓後 %d bytes，事件表 %d 筆，skip=%d，"+
		"腳本區 %d–%d，字串區 %d 起\n",
		idx, len(raw), lay.tableEnd/3-1, lay.skip, lay.scriptAt, lay.strAt, lay.strAt)
	fmt.Println("  事件表（格號 / 腳本號 / 方向遮罩）：")
	for p := 0; p+3 <= lay.tableEnd-3; p += 3 {
		fmt.Printf("    cell %3d (X=%2d Y=%2d)  script %3d  kind %02X\n",
			raw[p], raw[p]%16, raw[p]/16, raw[p+1], raw[p+2])
	}
	scripts := splitFF(raw[lay.scriptAt:lay.strAt])
	fmt.Printf("  腳本 %d 條：\n", len(scripts))
	for i, s := range scripts {
		if len(s) == 0 {
			continue
		}
		fmt.Printf("    [%2d] %s\n", i, hex.EncodeToString(s))
	}
}

func splitFF(b []byte) [][]byte {
	var out [][]byte
	cur := []byte{}
	for _, v := range b {
		if v == events.Terminator {
			out = append(out, cur)
			cur = []byte{}
			continue
		}
		cur = append(cur, v)
	}
	return append(out, cur)
}

// addEvent 把一條新腳本接到腳本區尾巴，再替指定的格子各加一筆事件。
//
// 新腳本接在**尾巴**是為了不動既有的腳本編號 —— 改到中間的話，
// 所有既有事件的 Index 都要跟著改，而那是完全沒必要的風險。
func addEvent(raw []byte, lay segLayout, cells, reuse []int, body []byte, kind byte) (
	[]byte, int, []int, error) {

	scripts := splitFF(raw[lay.scriptAt:lay.strAt])
	newIdx := len(scripts)
	if newIdx > 255 {
		return nil, 0, nil, fmt.Errorf("腳本已經有 %d 條，編號放不進一個位元組", newIdx)
	}

	// 既有的格子跳過：原版取第一筆相符的，重複加只會製造看不出來的歧義。
	have := map[int]bool{}
	for p := 0; p+3 <= lay.tableEnd-3; p += 3 {
		have[int(raw[p])] = true
	}
	var added []int
	for _, c := range cells {
		if !have[c] {
			added = append(added, c)
		}
	}
	if len(added) == 0 && len(reuse) == 0 {
		return nil, 0, nil, fmt.Errorf("指定的格子都已經有事件了，也沒有給 -retarget")
	}
	sort.Ints(added)
	retarget := map[int]bool{}
	for _, c := range reuse {
		if !have[c] {
			return nil, 0, nil, fmt.Errorf("格 %d 本來就沒有事件，不能 retarget", c)
		}
		retarget[c] = true
	}

	// 1. 事件表：插進去之後格號仍要嚴格遞增。
	var table []byte
	touched := append([]int(nil), added...)
	old := raw[:lay.tableEnd-3]
	ai := 0
	for p := 0; p+3 <= len(old); p += 3 {
		for ai < len(added) && added[ai] < int(old[p]) {
			table = append(table, byte(added[ai]), byte(newIdx), kind)
			ai++
		}
		if retarget[int(old[p])] {
			// 改指的格子**不能塞進 added**：那個切片正被 ai 拿來走
			// 插入位置，邊走邊 append 會把插入點弄亂，而症狀是
			// 「輸出的段整個解不開」。
			touched = append(touched, int(old[p]))
			table = append(table, old[p], byte(newIdx), kind)
			continue
		}
		table = append(table, old[p], old[p+1], old[p+2])
	}
	for ; ai < len(added); ai++ {
		table = append(table, byte(added[ai]), byte(newIdx), kind)
	}
	table = append(table, 0, 0, 0)

	// 2. 腳本區尾巴接上 `FF` ＋ 內容，skip 跟著加。
	extra := append([]byte{events.Terminator}, body...)
	newSkip := lay.skip + len(extra)
	if newSkip > 0xFFFF {
		return nil, 0, nil, fmt.Errorf("skip 溢位")
	}

	outBuf := make([]byte, 0, len(raw)+len(table)-lay.tableEnd+len(extra))
	outBuf = append(outBuf, table...)
	outBuf = append(outBuf, byte(newSkip), byte(newSkip>>8))
	outBuf = append(outBuf, raw[lay.scriptAt:lay.strAt]...)
	outBuf = append(outBuf, extra...)
	outBuf = append(outBuf, raw[lay.strAt:]...)
	if len(outBuf) > 0xFFFF {
		return nil, 0, nil, fmt.Errorf("段長 %d 超過段頭的 uint16", len(outBuf))
	}
	sort.Ints(touched)
	return outBuf, newIdx, touched, nil
}

// rebuild 重寫整個檔：只有改過的那一段重新壓縮，其餘**原樣搬過去**。
//
// 其餘段不重壓是刻意的：這一支的壓縮器與原版的選擇不見得相同（合法但不同的
// 編碼），沒必要為了改一段而讓整個檔的位元組全部換掉 —— 那樣出問題時
// 分不出是「改的那一段錯了」還是「壓縮器錯了」。
func rebuild(blob []byte, offs [events.SegmentCount]int, order []int,
	target int, raw []byte) []byte {

	out := make([]byte, events.SegmentCount*4)
	newOff := make(map[int]int, len(order))
	for i, seg := range order {
		newOff[seg] = len(out)
		if seg == target {
			out = append(out, lzw.PackSegment(raw)...)
			continue
		}
		end := len(blob)
		if i+1 < len(order) {
			end = offs[order[i+1]]
		}
		out = append(out, blob[offs[seg]:end]...)
	}
	for i := 0; i < events.SegmentCount; i++ {
		if o, ok := newOff[i]; ok {
			binary.LittleEndian.PutUint32(out[i*4:], uint32(o))
		}
	}
	return out
}
