// mm2shots 產生介面截圖。
//
//	go run ./cmd/mm2shots -data workplace/orig/MM2 -out docs/screenshots
//
// 走的是 internal/ui 那條與視窗無關的路徑，所以沒有 GPU 也跑得出來，
// 而且每次產生的畫面與實際遊玩的一致 —— 截圖不是另外畫的示意圖。
package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

type shot struct {
	name  string
	desc  string
	setup func(*ui.Session)
}

var shots = []shot{
	{"01-first-person", "第一人稱視角：城鎮的牆與地板，側牆上的火炬會動", func(s *ui.Session) {
		s.Key(ui.KeyForward)
		s.Key(ui.KeyConfirm)
	}},
	{"02-cast", "施法選單：法術名、等級與說明都是譯文", func(s *ui.Session) {
		s.Key(ui.KeyCast)
		// 游標移到真的會法術的人身上 —— 停在第一個施法職業身上會拍到
		// 「一個法術都還不會」，那不是這張截圖要說明的事。
		for i := 0; i < 3; i++ {
			s.Key(ui.KeyDown)
		}
		s.Key(ui.KeyConfirm)
	}},
	{"03-items", "物品選單：已裝備六格加背包六格", func(s *ui.Session) {
		s.Key(ui.KeyItems)
	}},
	{"04-shop", "商店：貨色與售價來自原版的商店表", func(s *ui.Session) {
		s.Key(ui.KeyShop)
	}},
	{"05-reference", "查說明書：紙本才有的參考資料收進遊戲裡", func(s *ui.Session) {
		s.Key(ui.KeyRef)
	}},
	{"06-map", "地圖：城鎮整張看得到，其他地圖只顯示走過的格", func(s *ui.Session) {
		s.Key(ui.KeyMap)
	}},
	{"07-combat", "戰鬥：九個指令全部可用，這裡是射擊", func(s *ui.Session) {
		fight(s)
		s.Key(ui.KeyShoot)
	}},
	{"08-protection", "戰鬥中的防護效能（指令 P）", func(s *ui.Session) {
		fight(s)
		s.Game.Fight.Protect = game.Protection{Bless: 3, Shield: 1, HolyBonus: 12}
		s.Key(ui.KeyProt)
	}},
}

// fight 擺一場遭遇，不必等隨機遇敵。
//
// 怪物要從圖鑑裡拿，不能自己捏一隻 —— 捏出來的 Sprite 是 0，
// 查不到圖，畫面上就一隻怪都沒有。
func fight(s *ui.Session) {
	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	var def monsters.Monster
	if len(s.Game.Bestiary) > 3 {
		def = s.Game.Bestiary[3]
	}
	m := game.NewMonster(def)
	if n := s.Game.Names[def.Name]; n != "" {
		m.Display = n
	}
	s.Game.Fight = &game.Encounter{
		Party: party, Monsters: []game.Combatant{m}, Front: 1,
	}
	s.Mode = ui.ModeCombat
}

func main() {
	data := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	out := flag.String("out", "docs/screenshots", "輸出目錄")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, sh := range shots {
		s, err := ui.Load(*data)
		if err != nil {
			log.Fatal(err)
		}
		sh.setup(s)
		scr := s.Draw()
		f, err := os.Create(filepath.Join(*out, sh.name+".png"))
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, scr.Hi); err != nil {
			f.Close()
			log.Fatal(err)
		}
		f.Close()
		fmt.Printf("%-18s %s\n", sh.name+".png", sh.desc)
	}
}
