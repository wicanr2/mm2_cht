// mm2roster 改寫 DOSBox 那份**可寫的**名冊副本，讓某位施法者到得了
// 平常要玩很久才到得了的狀態 —— 高法力等級、學會指定的法術、SP 灌滿。
//
// 為什麼要有這支：有些原版行為只有高階法術才看得到（傳送術要巫師五級、
// 城市傳送術要牧師八級），而預設隊伍的兩名施法者法力等級都是 1。
// 與 `cmd/mm2patchevent` 同一個想法 —— **改資料不改程式**：跑的仍然是原版
// 自己的程式碼，只有「玩家練到哪」被改了，所以量到的提示、扣費與取消行為
// 都是原版的。做法見 `docs/playtest/02-event-relocation.md`。
//
//	go run ./cmd/mm2roster -in workplace/dosbox/game/ROSTER.DAT \
//	    -who 1 -sl 9 -sp 200 -learn 1,2,3 -list
//
// **輸出一定要寫到 `workplace/dosbox/game/`**（可寫副本）。`workplace/orig/`
// 是唯讀參考，驗完把那個檔案還原回去（或直接刪掉整個 game 目錄重建）。
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/game"
)

func main() {
	in := flag.String("in", "workplace/dosbox/game/ROSTER.DAT", "名冊檔（可寫副本）")
	out := flag.String("out", "", "輸出檔（空值＝就地改寫 -in）")
	who := flag.Int("who", -1, "第幾位（1 起算，照檔案順序）")
	sl := flag.Int("sl", -1, "法力等級（記錄 +114）")
	sp := flag.Int("sp", -1, "SP 目前值與上限（記錄 +88／+90）")
	gems := flag.Int("gems", -1, "寶石（記錄 +92）")
	lvl := flag.Int("level", -1, "等級（記錄 +32 與 +113 一起改）")
	spd := flag.Int("speed", -1, "速度（基礎 +0x13 與當前一起改）—— 戰鬥的行動順序照速度排，"+
		"調高就會第一個行動，自動化才停得到指定的人")
	learn := flag.String("learn", "", "學會該系的第幾條，逗號分隔（1 起算）")
	all := flag.Bool("learn-all", false, "整系 48 條全學")
	list := flag.Bool("list", false, "只列出名冊，不改")
	flag.Parse()

	blob, err := os.ReadFile(*in)
	if err != nil {
		die(err)
	}
	cs, err := game.ParseCharacters(blob)
	if err != nil {
		die(err)
	}
	if *list || *who < 1 {
		for i := range cs {
			c := &cs[i]
			fmt.Printf("%2d. %-14s %-10s 等級 %2d 法力等級 %d SP %3d/%-3d 寶石 %3d 已學位元 %x\n",
				i+1, c.Name, c.Class, c.Level, c.SL, c.SP, c.MaxSP, c.Gems, c.SpellsKnown)
		}
		if *who < 1 {
			return
		}
	}
	if *who > len(cs) {
		die(fmt.Errorf("名冊只有 %d 筆", len(cs)))
	}
	c := &cs[*who-1]

	// 每一項都直接寫記錄的原始位元組，再重新解析 —— `EncodeRoster` 只會
	// 把已解欄位寫回去，沒解的原樣保留，所以檔案其餘部分逐位元組不動。
	set := func(off int, v int, width int) {
		for i := 0; i < width; i++ {
			c.Raw[off+i] = byte(v >> (8 * i))
		}
	}
	if *sl >= 0 {
		set(114, *sl, 1)
	}
	if *sp >= 0 {
		set(88, *sp, 2)
		set(90, *sp, 2)
	}
	if *gems >= 0 {
		set(92, *gems, 2)
	}
	if *lvl >= 0 {
		set(32, *lvl, 1)
		set(113, *lvl, 1) // 戰鬥判定讀的是 +113，見 docs/formats/08
	}
	if *spd >= 0 {
		set(0x10+3, *spd, 1) // 基礎：六格屬性的第 4 格
		set(107+3, *spd, 1)  // 當前：+107 起的第二份
	}
	if *all {
		for i := 81; i < 87; i++ {
			c.Raw[i] = 0xFF
		}
	}
	for _, s := range strings.Split(*learn, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			die(err)
		}
		i := n - 1
		if i < 0 || i >= 48 {
			die(fmt.Errorf("法術編號 %d 超出 1–48", n))
		}
		c.Raw[81+i/8] |= 1 << (uint(i) % 8)
	}
	// 改完原始位元組之後重新解析那一筆（沒有匯出的單筆解析器，
	// 就拿 130 bytes 當成一份只有一筆的名冊解）。
	if one, err := game.ParseCharacters(c.Raw); err == nil && len(one) == 1 {
		cs[*who-1] = one[0]
	}

	got, err := game.EncodeRoster(cs, blob)
	if err != nil {
		die(err)
	}
	dst := *out
	if dst == "" {
		dst = *in
	}
	if err := os.WriteFile(dst, got, 0o644); err != nil {
		die(err)
	}
	n := 0
	for i := range blob {
		if blob[i] != got[i] {
			n++
		}
	}
	c2 := &cs[*who-1]
	fmt.Printf("%s：改了 %d 個位元組\n", dst, n)
	fmt.Printf("  %s 等級 %d 法力等級 %d SP %d/%d 寶石 %d 已學位元 %x\n",
		c2.Name, c2.Level, c2.SL, c2.SP, c2.MaxSP, c2.Gems, c2.SpellsKnown)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
