// mm2fight 用原版資料打一場遭遇戰，把戰報印出來。
//
// 命中與傷害走原版的兩條路徑（怪物打隊伍 `sub_8398`、隊伍打怪物
// `sub_8E81`），擲骰走原版那顆 RNG。細節見 internal/game/attack.go。
//
//	go run ./cmd/mm2fight -seed 4321 -monster 5 -count 3
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
)

func main() {
	dataDir := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	seed := flag.Int("seed", 4321, "亂數種子")
	first := flag.Int("monster", 5, "怪物編號（0–255，表是按難度排的）")
	count := flag.Int("count", 3, "怪物數量")
	lang := flag.String("lang", "translations/zh-Hant.json", "譯文檔；空字串顯示原文")
	flag.Parse()

	read := func(n string) []byte {
		b, err := os.ReadFile(*dataDir + "/" + n)
		if err != nil {
			log.Fatal(err)
		}
		return b
	}
	defs, err := monsters.Parse(read("MONSTERS.DAT"))
	if err != nil {
		log.Fatal(err)
	}
	cs, err := game.ParseCharacters(read("DEFAULT.DAT"))
	if err != nil {
		log.Fatal(err)
	}
	zh := loadNames(*lang)

	e := &game.Encounter{}
	for i := range cs {
		e.Party = append(e.Party, &cs[i])
	}
	for i := 0; i < *count && *first+i < len(defs); i++ {
		m := game.NewMonster(defs[*first+i])
		m.Display = zh[m.Def.Name]
		e.Monsters = append(e.Monsters, m)
	}

	fmt.Println("隊伍：")
	for i, c := range cs {
		fmt.Printf("  %d %-12s %v %v %v  HP %d/%d  力量 %d 速度 %d 準確度 %d\n",
			i+1, c.Name, c.Race, c.Class, c.Align, c.HP, c.MaxHP,
			c.Base[game.Might], c.Base[game.Speed], c.Base[game.Accuracy])
	}
	fmt.Println("遭遇：")
	for _, m := range e.Monsters {
		fmt.Printf("  %-14s HP %d\n", m.CombatName(), m.CombatHP())
	}
	fmt.Println()

	for _, line := range e.Fight(game.NewRand(uint16(*seed)), 100) {
		fmt.Println(" ", line)
	}
	fmt.Println()
	if e.PartyWon() {
		fmt.Println("隊伍獲勝。")
	} else {
		fmt.Println("隊伍全滅。")
	}
}

// loadNames 取怪物名的譯文。
func loadNames(path string) map[string]string {
	out := map[string]string{}
	if path == "" {
		return out
	}
	b, err := os.ReadFile("translations/strings.json")
	if err != nil {
		return out
	}
	var rows []struct{ Key, Source, Target string }
	if json.Unmarshal(b, &rows) != nil {
		return out
	}
	for _, r := range rows {
		if strings.HasPrefix(r.Key, "monster.") && r.Target != "" {
			out[r.Source] = r.Target
		}
	}
	return out
}
