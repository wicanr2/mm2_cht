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
	"strings"

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

// shotPlatform 是「這一張非拍到哪一套素材不可」的截圖，拍不到就**不寫檔**。
//
// 平台是按 F6 循環出來的，而循環的內容取決於**這台機器上哪幾套素材通得過
// 完整性檢查**（玩家自備的原版磁片、ROM，加上素材表本身有沒有壞）。
// 少一套，「按 N 次 F6」就落到隔壁那一套，而檔名還是原來那個 ——
// 症狀是「某一張截圖悄悄換了平台」。踩過兩次，兩次都不是機器少了原版：
// 一次是 MSX 的素材表有一格越界，整套被完整性檢查打掉；那張「MSX 截圖」
// 其實是高解析素材包的畫面，而且在版控裡待了好幾個 commit。
//
// **所以會換平台的截圖一律走 `wantPlatform` ＋ 這張表**，不要數 F6 按幾次。
var shotPlatform = map[string]view.Platform{
	"01c-first-person-amiga":        view.PlatformAmiga,
	"01d-first-person-amiga-modern": view.PlatformAmiga,
	"01e-first-person-pack":         view.PlatformModern,
	"01f-first-person-msx":          view.PlatformMSX,
	"01g-msx-torch":                 view.PlatformMSX,
	"01h-first-person-md":           view.PlatformMegaDrive,
}

// wantPlatform 按 F6 直到畫面上是指定的素材，回報有沒有按到。
func wantPlatform(s *ui.Session, p view.Platform) bool {
	for i := 0; i < 8; i++ {
		if s.Assets.Town != nil && s.Assets.Town.Platform == p {
			return true
		}
		s.Key(ui.KeyPlatform)
		s.Mode, s.Lines = ui.ModeExplore, nil
	}
	return s.Assets.Town != nil && s.Assets.Town.Platform == p
}

var shots = []shot{
	{"17-intro", "片頭：原版標題畫面與動畫", func(s *ui.Session) {
		s.ShowIntro()
		// 走幾格動畫再拍：第 0 格兩處都停在還原圖上，看不出有東西在動。
		for i := 0; i < 4; i++ {
			s.Tick()
		}
	}},
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
		wantPlatform(s, view.PlatformAmiga)
	}},
	{"01d-first-person-amiga-modern", "Amiga 素材加 Scale3x", func(s *ui.Session) {
		s.Game.World.X, s.Game.World.Y = 8, 0
		s.Game.World.Face = game.East
		wantPlatform(s, view.PlatformAmiga)
		if s.Assets.Town != nil {
			s.Assets.Town.Style = view.StyleModern
		}
	}},
	{"01g-msx-torch", "MSX 的火炬三個相位（動畫影格是 remake 產生的，原版只有一張）", func(s *ui.Session) {
		// (0,0) 面北看得到火炬 —— 起點那格看不到，挑錯格子會拍到
		// 「火炬沒接上」的假象。
		s.Game.World.X, s.Game.World.Y = 0, 0
		s.Game.World.Face = game.North
		wantPlatform(s, view.PlatformMSX)
		s.TorchPhase = 1
	}},
	{"01f-first-person-msx", "MSX2 版素材：整套場景是一張 462×128 的素材表，每面牆是表裡的一塊", func(s *ui.Session) {
		s.Game.World.X, s.Game.World.Y = 8, 0
		s.Game.World.Face = game.East
		wantPlatform(s, view.PlatformMSX)
	}},
	{"01h-first-person-md", "Mega Drive 版素材：側牆柱是一整根 120 高的圖，火炬直接改 nametable", func(s *ui.Session) {
		s.Game.World.X, s.Game.World.Y = 8, 0
		s.Game.World.Face = game.East
		wantPlatform(s, view.PlatformMegaDrive)
	}},
	{"01e-first-person-pack", "第三套素材：烘好的高解析素材包（cmd/mm2modern）", func(s *ui.Session) {
		s.Game.World.X, s.Game.World.Y = 8, 0
		s.Game.World.Face = game.East
		wantPlatform(s, view.PlatformModern)
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
	{"07a-combat-anim-00", "正常戰鬥 UI：怪物第一個合法動畫序列的起始步", func(s *ui.Session) {
		fight(s)
	}},
	{"07a-combat-anim-15", "正常戰鬥 UI：依原始 hold 前進十五個 tick", func(s *ui.Session) {
		fight(s)
		for i := 0; i < 15; i++ {
			s.Tick()
		}
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
		s.Key(ui.KeySearch)
	}},
	{"14-world-grid", "世界網格：二十張野外圖排成 5×4，字母數字沿用說明書", func(s *ui.Session) {
		s.Game.World.MapIndex = 11
		s.Key(ui.KeyWorld)
	}},
	{"15-md-monster", "F6 切到 Mega Drive 的怪物素材（場景沿用 DOS）",
		monsterPackShot("怪物換成 Mega Drive")},
	{"16-amiga-monster", "F6 再按一次換成 Amiga 的怪物", monsterPackShot("怪物換成 Amiga")},
	{"07b-target", "攻擊前挑目標：列的是這一擊打得到的那幾隻", func(s *ui.Session) {
		fightMany(s, 4, 2)
		s.Key(ui.KeyShoot) // 射擊打得到後排，四隻全列得出來
	}},
	{"08-protection", "戰鬥中的防護效能（指令 P）", func(s *ui.Session) {
		fight(s)
		s.Game.Fight.Protect = game.Protection{Bless: 3, Shield: 1, HolyBonus: 12}
		s.Key(ui.KeyProt)
	}},
	{"19-settings", "設定（F2）：remake 與原版不同的地方在這裡切回去", func(s *ui.Session) {
		s.Key(ui.KeySettings)
	}},
}

// templeGreeting 是神殿門口的招呼語，原文在 `STR.DAT`，
// 對照原版截圖 `shots/c2.png` 就是同一段。
const templeGreeting = "一名身形清瘦、罩著兜帽長袍的@牧師望向你們，以沉靜的嗓音問：@「旅人，需要我的幫助嗎（y/n）？」"

// fight 擺一場遭遇，不必等隨機遇敵。
//
// 怪物要從圖鑑裡拿，不能自己捏一隻 —— 捏出來的 Sprite 是 0，
// 查不到圖，畫面上就一隻怪都沒有。
// monsterPackShot 按 F6 直到怪物素材換成指定平台為止。
//
// 戰鬥中也能按 —— 那正是唯一看得到怪物的時候。
func monsterPackShot(want string) func(*ui.Session) {
	return func(s *ui.Session) {
		fight(s)
		for i := 0; i < 10; i++ {
			s.Lines = nil
			s.Key(ui.KeyPlatform)
			if len(s.Lines) > 0 && strings.Contains(s.Lines[0], want) {
				break
			}
		}
		s.Mode = ui.ModeCombat
	}
}

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

// fightMany 擺一場 n 隻怪、前排 front 隻的仗 —— 目標選單要有得選才拍得出來。
//
// 怪從圖鑑裡連號取，四隻各不相同：全部同名的話「選了哪一隻」在畫面上
// 看不出差別，那張截圖就證明不了任何事。
func fightMany(s *ui.Session, n, front int) {
	fight(s)
	var ms []game.Combatant
	for i := 0; i < n && i < len(s.Game.Bestiary); i++ {
		def := s.Game.Bestiary[3+i]
		m := game.NewMonster(def)
		if nm := s.Game.Names[def.Name]; nm != "" {
			m.Display = nm
		}
		ms = append(ms, m)
	}
	s.Game.Fight.Monsters = ms
	s.Game.Fight.Front = front
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
		if want, ok := shotPlatform[sh.name]; ok &&
			(s.Assets.Town == nil || s.Assets.Town.Platform != want) {
			fmt.Printf("%-18s 跳過：這台機器沒有 %v 的素材\n", sh.name+".png", want)
			continue
		}
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
