// mm2play 跑一段完整的遊玩：走地圖、觸發事件、遇敵開打、最後存檔。
//
//	go run ./cmd/mm2play -steps "F,F,L,F,F,F,R,F" -seed 4321
//
// 按鍵：F 前進、B 後退、L 左轉、R 右轉。
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/i18n"
)

func main() {
	dataDir := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	steps := flag.String("steps", "F,F,F,F,F,F,F,F", "按鍵序列")
	seed := flag.Int("seed", 4321, "亂數種子")
	x := flag.Int("x", 7, "起始 X")
	y := flag.Int("y", 8, "起始 Y")
	save := flag.String("save", "", "結束後把名冊寫到這裡（空字串則不寫）")
	flag.Parse()

	read := func(n string) []byte {
		b, err := os.ReadFile(filepath.Join(*dataDir, n))
		if err != nil {
			log.Fatal(err)
		}
		return b
	}
	w, err := game.NewWorld(read("MAP.DAT"), read("EVENTSI.DAT"))
	if err != nil {
		log.Fatal(err)
	}
	w.MapIndex, w.X, w.Y, w.Face = 0, *x, *y, game.South

	roster := read("DEFAULT.DAT")
	party, err := game.ParseCharacters(roster)
	if err != nil {
		log.Fatal(err)
	}
	defs, err := monsters.Parse(read("MONSTERS.DAT"))
	if err != nil {
		log.Fatal(err)
	}

	cat, err := i18n.Load(i18n.DefaultPath)
	if err != nil {
		log.Fatal(err)
	}
	game.UseText(cat)
	s := game.NewSession(w, party, defs, uint16(*seed))
	s.Names = monsterNames(cat, defs)
	trans := eventText(cat, w)

	fights, won := 0, 0
	for i, k := range strings.Split(*steps, ",") {
		k = strings.ToUpper(strings.TrimSpace(k))
		var enc *game.Encounter
		switch k {
		case "F":
			_, enc = s.Step(1)
		case "B":
			_, enc = s.Step(-1)
		case "L":
			s.Turn(-1)
		case "R":
			s.Turn(1)
		}
		fmt.Printf("%2d %s  (%2d,%2d) %v", i+1, k, w.X, w.Y, w.Face)
		for _, l := range s.Log {
			if t, ok := trans[l]; ok {
				l = t
			}
			fmt.Printf("  %s", strings.ReplaceAll(l, "\n", " / "))
		}
		fmt.Println()
		if enc == nil {
			continue
		}
		fights++
		for _, line := range enc.Fight(s.Rand, 100) {
			fmt.Println("     ", line)
		}
		if enc.PartyWon() {
			won++
			exp := enc.AwardExp(s.Party)
			fmt.Printf("      → 隊伍獲勝，每人獲得 %d 點經驗\n", exp)
		} else {
			fmt.Println("      → 隊伍全滅")
		}
		if !s.Alive() {
			fmt.Println("\n全隊倒下，遊戲結束。")
			break
		}
	}

	// 回到旅店：休息、受訓、存檔 —— 手冊說要在旅店登記才能存檔。
	fmt.Println()
	for _, l := range s.RestAtInn() {
		fmt.Println("  " + l)
	}
	for _, l := range s.TrainParty() {
		fmt.Println("  " + l)
	}

	fmt.Printf("\n走了 %d 步，遭遇 %d 場，勝 %d 場\n", len(strings.Split(*steps, ",")), fights, won)
	fmt.Println("隊伍狀態：")
	for _, c := range s.Party {
		fmt.Printf("  %-12s %v %v  Lv%-2d  HP %2d/%-2d  SP %2d/%-2d  經驗 %-5d 法力等級 %d  %v\n",
			c.Name, c.Race, c.Class, c.Level, c.HP, c.MaxHP, c.SP, c.MaxSP,
			c.Exp, c.SpellLevel(), c.Condition)
	}
	if *save != "" {
		out, err := game.EncodeRoster(s.Party, roster)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(*save, out, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\n名冊已存到 %s（%d bytes，未解的欄位原樣保留）\n", *save, len(out))
	}
}

// monsterNames 組出「怪物英文名 → 譯名」。key 用怪物在表中的序號，
// 與 cmd/mm2strings 匯出時一致。
func monsterNames(c *i18n.Catalog, defs []monsters.Monster) map[string]string {
	src := make(map[string]string, len(defs))
	for _, m := range defs {
		src[fmt.Sprintf("monster.%03d", m.Index)] = m.Name
	}
	return c.SourceMap(src, "monster.")
}

// eventText 組出目前這張地圖的「事件原文 → 譯文」。
func eventText(c *i18n.Catalog, w *game.World) map[string]string {
	seg := w.EventSegment()
	if seg == nil {
		return nil
	}
	src := map[string]string{}
	for i, str := range seg.Strings {
		src[fmt.Sprintf("indoor.%02d.%03d", seg.Index, i)] = str
	}
	return c.SourceMap(src, fmt.Sprintf("indoor.%02d.", seg.Index))
}
