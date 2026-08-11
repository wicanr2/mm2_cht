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
	"github.com/wicanr2/mm2_cht/internal/view"
)

type shot struct {
	name  string
	desc  string
	setup func(*ui.Session)
}

var shots = []shot{
	{"01-first-person", "第一人稱視角：城鎮的牆與地板，牆上的火炬會動", func(s *ui.Session) {
		// 米德格特 (8,0) 朝東是走廊看得最深、火炬最多的視角之一
		// （四個深度全開、四盞火炬）。起點正前方是一面沒火炬的牆，
		// 拍那裡看不出這張截圖要說明的東西。
		s.Game.World.X, s.Game.World.Y = 8, 0
		s.Game.World.Face = game.East
	}},
	{"01b-first-person-modern", "同一個視角改用 Scale3x：牆的邊界跟上中文的解析度", func(s *ui.Session) {
		s.Game.World.X, s.Game.World.Y = 8, 0
		s.Game.World.Face = game.East
		if s.Assets.Town != nil {
			s.Assets.Town.Style = view.StyleModern
		}
	}},
	{"01c-first-person-amiga", "同一個視角換成 Amiga 版素材：32 色、同一套幾何", func(s *ui.Session) {
		s.Game.World.X, s.Game.World.Y = 8, 0
		s.Game.World.Face = game.East
		s.Key(ui.KeyPlatform)
		s.Mode = ui.ModeExplore
	}},
	{"01d-first-person-amiga-modern", "Amiga 素材加 Scale3x", func(s *ui.Session) {
		s.Game.World.X, s.Game.World.Y = 8, 0
		s.Game.World.Face = game.East
		s.Key(ui.KeyPlatform)
		s.Mode, s.Lines = ui.ModeExplore, nil
		if s.Assets.Town != nil {
			s.Assets.Town.Style = view.StyleModern
		}
	}},
	{"01g-msx-torch", "MSX 的火炬三個相位（動畫影格是 remake 產生的，原版只有一張）", func(s *ui.Session) {
		// (0,0) 面北看得到火炬 —— 起點那格看不到，挑錯格子會拍到
		// 「火炬沒接上」的假象。
		s.Game.World.X, s.Game.World.Y = 0, 0
		s.Game.World.Face = game.North
		s.Key(ui.KeyPlatform)
		s.Mode = ui.ModeExplore
		s.Key(ui.KeyPlatform)
		s.Mode, s.Lines = ui.ModeExplore, nil
		s.TorchPhase = 1
	}},
	{"01f-first-person-msx", "MSX2 版素材：整套場景是一張 462×128 的素材表，每面牆是表裡的一塊", func(s *ui.Session) {
		s.Game.World.X, s.Game.World.Y = 8, 0
		s.Game.World.Face = game.East
		s.Key(ui.KeyPlatform)
		s.Mode = ui.ModeExplore
		s.Key(ui.KeyPlatform)
		s.Mode, s.Lines = ui.ModeExplore, nil
	}},
	{"01e-first-person-pack", "第三套素材：烘好的高解析素材包（cmd/mm2modern）", func(s *ui.Session) {
		s.Game.World.X, s.Game.World.Y = 8, 0
		s.Game.World.Face = game.East
		// 訊息模式會吃掉按鍵，兩次切換之間要先回探索模式。
		s.Key(ui.KeyPlatform)
		s.Mode = ui.ModeExplore
		s.Key(ui.KeyPlatform)
		s.Mode = ui.ModeExplore
		s.Key(ui.KeyPlatform)
		s.Mode, s.Lines = ui.ModeExplore, nil
	}},
	{"00-chinese", "中文疊在原版畫面上：原版像素一個都沒改", func(s *ui.Session) {
		// 神殿門口那段招呼，取自 `STR.DAT`（`str.339` 的第二段）。
		// **不自己編台詞** —— 截圖裡的中文必須是真的譯文，
		// 否則讀者看到的是一句遊戲裡不存在的話。
		s.Lines = []string{templeGreeting}
		s.Mode = ui.ModeMessage
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
		// 停在鐵匠鋪的主選單只拍得到四個動作，貨架才是這張要說明的東西。
		s.Key(ui.KeyConfirm)
	}},
	{"05-reference", "查說明書：紙本才有的參考資料收進遊戲裡", func(s *ui.Session) {
		s.Key(ui.KeyRef)
	}},
	{"12-worldmap", "世界地圖：手冊摺頁上的地名收進遊戲", func(s *ui.Session) {
		s.Key(ui.KeyRef)
		for i := 0; i < 5; i++ {
			s.Key(ui.KeyDown)
		}
		s.Key(ui.KeyConfirm)
	}},
	{"13-puzzles", "打字謎題的答案：英文文字遊戲翻成中文就解不開了", func(s *ui.Session) {
		s.Key(ui.KeyRef)
		for i := 0; i < 9; i++ {
			s.Key(ui.KeyDown)
		}
		s.Key(ui.KeyConfirm)
	}},
	{"11-lore", "說明書的手札：序言與科隆的歷史也收進遊戲", func(s *ui.Session) {
		s.Key(ui.KeyRef)
		n := 0
		if s.Menu != nil {
			n = len(s.Menu.Items)
		}
		for i := 0; i < n; i++ {
			s.Key(ui.KeyDown)
		}
		s.Key(ui.KeyConfirm)
	}},
	{"06-map", "地圖與攻略提示：城鎮整張看得到，右欄是當年只在雜誌上的提示", func(s *ui.Session) {
		s.Key(ui.KeyMap)
	}},
	{"07-combat", "戰鬥：九個指令全部可用，這裡是射擊", func(s *ui.Session) {
		fight(s)
		s.Key(ui.KeyShoot)
	}},
	{"10-create", "建立新角色：屬性、可選職業與對調", func(s *ui.Session) {
		s.Key(ui.KeyCreate)
		// 擲到至少有一個職業可選為止，截圖才看得到那一行。
		for i := 0; i < 200; i++ {
			any := false
			for c := 0; c < 8; c++ {
				if game.EligibleClasses(s.New.Attr)[c] {
					any = true
					break
				}
			}
			if any {
				break
			}
			s.Key(ui.KeyConfirm)
		}
	}},
	{"09-chest", "寶箱：打開、找陷阱、偵測魔法、離開", func(s *ui.Session) {
		c := &game.Chest{Kind: 3, Gold: 400, Gems: 12, Trap: 3}
		c.Items[0] = game.ChestItem{ID: 30, Level: 9}
		s.Chest = c
		s.Key(ui.KeyChest)
	}},
	{"08-protection", "戰鬥中的防護效能（指令 P）", func(s *ui.Session) {
		fight(s)
		s.Game.Fight.Protect = game.Protection{Bless: 3, Shield: 1, HolyBonus: 12}
		s.Key(ui.KeyProt)
	}},
}

// templeGreeting 是神殿門口的招呼語，原文在 `STR.DAT`，
// 對照原版截圖 `shots/c2.png` 就是同一段。
const templeGreeting = "一名身形清瘦、罩著兜帽長袍的@牧師望向你們，以沉靜的嗓音問：@「旅人，需要我的幫助嗎（y/n）？」"

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
