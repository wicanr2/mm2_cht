// Package ui 是遊玩的互動層：載入、按鍵、模式切換、畫面組成。
//
// **與視窗系統無關。** Ebiten 主程式（`cmd/mm2`）與 headless 的
// 逐格截圖（測試）跑的是同一份程式碼 —— 沒有 GPU 的環境也驗得了
// 「按這個鍵之後畫面變成什麼」，而不只是「編得過」。
package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/amiga"
	"github.com/wicanr2/mm2_cht/internal/assets/cjk"
	"github.com/wicanr2/mm2_cht/internal/assets/font"
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/assets/items"
	"github.com/wicanr2/mm2_cht/internal/assets/monpack"
	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/assets/msx"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/gamedata"
	"github.com/wicanr2/mm2_cht/internal/i18n"
	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// Key 是互動層看得懂的按鍵。實體按鍵怎麼對到它由呼叫端決定 ——
// Ebiten 綁鍵盤、測試直接送。
type Key int

const (
	KeyNone Key = iota
	KeyForward
	KeyBack
	KeyLeft
	KeyRight
	KeyConfirm // 推進訊息、確認
	KeyYes
	KeyNo
	KeyRest     // 在旅店休息並受訓
	KeyCast     // 開施法選單
	KeyItems    // 看物品欄
	KeyShop     // 開商店
	KeyRef      // 查說明書（第二技能、指令一覽）
	KeyBash     // 撞門
	KeyUnlock   // 開鎖
	KeySave     // 存檔
	KeyRun      // 戰鬥中溜跑
	KeyBlock    // 戰鬥中抵擋
	KeyShoot    // 戰鬥中射擊
	KeyUse      // 使用物品欄裡的東西
	KeyMap      // 開地圖畫面
	KeyWorld    // 開世界地圖畫面（remake 加的）
	KeyStyle    // 切換牆面素材的呈現方式（原版像素 ↔ Scale3x）
	KeyPlatform // 切換素材來自哪個平台（DOS ↔ Amiga）
	KeySearch   // 搜尋：把戰利品撿起來（原版 `S`）
	KeyCreate   // 建立新角色
	KeyExch     // 戰鬥中對調兩名隊員的位置
	KeyProt     // 戰鬥中顯示防護效能
	KeyView     // 戰鬥中檢視某位隊員
	KeyUp       // 選單游標上移
	KeyDown     // 選單游標下移
	KeyCancel
)

// Mode 是目前的互動模式。原版也是這樣分的：走路時吃方向鍵，
// 訊息或提問掛著時吃的是別的鍵。
type Mode int

const (
	ModeExplore Mode = iota
	ModeMessage      // 有訊息待確認
	ModeCombat       // 戰鬥中
	ModeDead         // 全隊倒下
	ModeMenu         // 選單開著（施法／物品／商店共用）
	ModeMap          // 地圖畫面
	ModeCreate       // 建立新角色
	ModeName         // 輸入姓名
	ModeText         // 事件文字輸入
	ModeWorld        // 世界地圖畫面
	ModeIntro        // 片頭畫面
)

func (m Mode) String() string {
	switch m {
	case ModeExplore:
		return "探索"
	case ModeMessage:
		return "訊息"
	case ModeCombat:
		return "戰鬥"
	case ModeDead:
		return "全滅"
	case ModeMenu:
		return "選單"
	case ModeMap:
		return "地圖"
	case ModeWorld:
		return "世界圖"
	case ModeIntro:
		return "片頭"
	case ModeCreate:
		return "建角"
	case ModeName:
		return "命名"
	case ModeText:
		return "輸入"
	}
	return "未知"
}

// Session 是一場遊玩。
type Session struct {
	Game   *game.Session
	Assets view.Assets
	Mode   Mode

	// Lines 是要顯示的訊息，逐次確認往下推。
	Lines []string
	// Notice 是不攔住輸入的事件文字，例如城鎮招牌。原版的 `04 NN` 招牌
	// 顯示後仍能直接前進；它不能混進 Lines，否則會被誤做成確認對話。
	Notice string
	// PromptText 是眼前 `0x2f` 的輸入緩衝；答案交給 game.World 後即清空。
	// 它和建角姓名分開，避免事件中途借用角色建立狀態。
	PromptText string

	// New 是正在建立的角色。名冊在 `Game.Roster`。
	New game.NewCharacter

	// Chest 是眼前的箱子。放在這裡而不是 game.Session 上，是因為
	// **原版什麼時候擺出箱子那一頁還沒解**（`_2misc_e02` 在 `ds:0434`
	// 為 0 時走選單，誰把它設成 0 並擺好內容那一段還沒追）。
	// 腳本擺好的獎賞走的是另一條路，由 Session.ClaimReward 當場領走。
	Chest *game.Chest

	// monSets 與 sets 一一對應：這一套要用哪一包非 DOS 的怪物素材。
	// nil 表示用 DOS 的 `MONSTERS.16`。
	//
	// 之所以是**平行的一條**而不是塞進 `TownSet`：兩者的完整度不一樣。
	// Mega Drive 有全部怪物但牆面還沒抽進引擎，Amiga／MSX 反過來。
	// 綁在一起會逼人在「沒有牆的平台不能選」與「假裝有牆」之間二選一。
	// intro 是片頭畫面的素材，載不到就是 nil（直接進遊戲）。
	intro *view.Intro

	// stinger 是待播的一次性音效，見 music_cue.go。
	stinger MusicCue

	monSets  []*monpack.Set
	monPacks map[*image.Paletted]*image.Paletted // 放大後的快取
	packTick int

	// sets 是可切換的第一人稱素材，依平台排；setIdx 是目前這一套。
	// 第 0 套一定是 DOS（原版），其餘按 platformDirs 的順序。
	sets   []*view.TownSet
	setIdx int

	// Ref 是說明書的參考資料，沒有就是 nil。
	Ref *Reference
	// Menu 是選單模式下開著的那一份，沒開時是 nil。
	Menu *Menu
	// menuKind 決定確認之後要做什麼。
	menuKind menuKind
	// who 是選單正在處理的隊員（施法者、看物品的人、買東西的人）。
	who int

	// 選單項次 → 引擎編號的對照。每次重建選單時覆寫。
	casters []int
	// pickers 是「挑一名隊員」那類選單的索引對照（開鎖用）。
	pickers []int
	// phase 是火炬動畫的相位，由 Tick 前進。
	phase int
	// Hints 是攻略提示（`data/hints.json`），地圖畫面右側顯示。
	Hints *Hints
	// hintPage 是地圖畫面提示的頁碼。
	hintPage int

	// TorchPhase 蓋掉自動前進的相位，給畫面比對用（`cmd/mm2diff`）——
	// 火焰在動，固定一個相位才比得出真正的差異。負數表示照常前進。
	TorchPhase int
	// monBlob 是 MONSTERS.16 的原始內容，monIndex 是它的圖號索引表。
	// 怪物圖很大（168 KB），只在要畫的時候才解一張。
	monBlob  []byte
	monIndex []int
	monCache map[int]gfx.MonsterPic
	// monsterAnim* 保存戰鬥精靈的動畫游標。sprites 每次畫面重建切片，
	// 所以游標不能放在 view.MonsterSprite 裡；Fight 換場時整批重設。
	monsterAnimFight  *game.Encounter
	monsterAnimStates []monsterAnimState
	// townNames 是有地名的地圖，索引即地圖編號。
	townNames []string
	// exchFirst 是對調指令選的第一位（戰鬥隊形裡的位置）。
	exchFirst int
	// arenaTier 是進行中這一場競技賽的階層，-1 表示這一場不是競技賽。
	// 打贏之後才發獎，而戰鬥是逐回合推進的，所以要記著。
	arenaTier int
	// chestAct 是寶箱那一頁選了哪個動作，等挑完人再執行。
	chestAct game.ChestAction
	// attrCur 是建角畫面上的游標，attrPick 是已挑起等著對調的那一項（-1 = 沒有）。
	attrCur, attrPick int
	spells            []int
	spellInfo         []game.Spell
	// spellPrompt 暫存尚未確認的施法輸入；它不進存檔，取消時清掉。
	spellPrompt      game.SpellPrompt
	spellPromptSpell int
	refRows          [][]string
	goods            []int

	rosterRaw []byte
	trans     map[string]string
	cat       *i18n.Catalog
	scr       *render.Screen
}

type monsterAnimState struct {
	picSlot int
	// script 是播放腳本走到第幾項（動畫表的第一段，見 gfx.ScriptStep）。
	script int
	anim   int
	step   int
	hold   int
}

// menuKind 是選單的用途。
type menuKind int

const (
	menuNone menuKind = iota
	menuCaster
	menuSpell
	menuItems
	menuShop
	menuRef
	menuRefSection
	menuUnlock
	menuExchange1
	menuExchange2
	menuChest
	menuChestWho
	menuSpellMember
	menuSpellItem
	menuSpellChoice
	menuSpellColumn
	menuCreateRace
	menuCreateAlign
	menuCreateSex
	menuTemple
	menuInn
	menuInnAdd
	menuInnDrop
	menuSmith
	menuSmithSell
	menuSmithIdent
	menuGuild
	menuTavern
	menuGuildBuy
	menuTrain
	menuDetox
	menuTempleBuy
	menuEventMember
)

// LoadOptions 是載入素材的可選設定。
//
// 平台素材是選配的：空路徑沿用 repo 既有的預設搜尋位置；指定路徑時，
// 只有完整的一組素材成功解碼才會加入 F6 的循環。Theme 留空代表安全從
// DOS 開始；非空值若未知或指定的素材不可用，LoadWithOptions 會明確失敗。
// Theme 只選平台素材，不影響 F5 的 StyleClassic／StyleModern。
type LoadOptions struct {
	AmigaDir   string
	MSXDir     string
	ModernDirs []string
	Theme      string
}

// Load 從原版資料目錄開一場遊玩，保留既有呼叫端的預設行為。
//
// 缺原版資料就回錯誤 —— 這一層不做「找不到就用假資料頂著」，
// 那會讓畫面看起來對、其實在跑別的東西。
func Load(dataDir string) (*Session, error) {
	return LoadWithOptions(dataDir, LoadOptions{})
}

// LoadWithOptions 從原版資料目錄開一場遊玩，並套用可選的平台素材設定。
func LoadWithOptions(dataDir string, opts LoadOptions) (*Session, error) {
	theme := strings.ToLower(strings.TrimSpace(opts.Theme))
	switch theme {
	case "", "dos", "amiga", "msx", "modern":
	default:
		return nil, fmt.Errorf("未知素材主題 %q（可用：dos、amiga、msx、modern）", opts.Theme)
	}
	amigaPath := opts.AmigaDir
	if amigaPath == "" {
		amigaPath = amigaDir
	}
	msxPath := opts.MSXDir
	if msxPath == "" {
		msxPath = msxDir
	}
	modernPaths := opts.ModernDirs
	if len(modernPaths) == 0 {
		modernPaths = modernDirs
	}

	read := func(n string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dataDir, n))
	}
	must := func(n string) []byte {
		b, err := read(n)
		if err != nil {
			panic(err)
		}
		return b
	}
	for _, n := range []string{"MAP.DAT", "EVENTSI.DAT", "ATTRIB.DAT", "MM2.CH", "DEFAULT.DAT"} {
		if _, err := read(n); err != nil {
			return nil, fmt.Errorf("缺少原版檔案 %s: %w", n, err)
		}
	}

	w, err := game.NewWorld(must("MAP.DAT"), must("EVENTSI.DAT"))
	if err != nil {
		return nil, err
	}
	// 全域變數的初值就是 `MM2.EXE` 尾部資料區的內容 —— 不抄的話所有
	// 計數器都從 0 開始，年份會顯示 0 而不是 900。
	if exe, err := read("MM2.EXE"); err == nil {
		w.SeedGlobals(exe)
	}
	party, err := game.ParseCharacters(must("DEFAULT.DAT"))
	if err != nil {
		return nil, err
	}
	defs, err := monsters.Parse(must("MONSTERS.DAT"))
	if err != nil {
		return nil, err
	}
	cat, err := i18n.Load(i18n.DefaultPath)
	if err == nil {
		game.UseText(cat)
	}

	gs := game.NewSession(w, party, defs, 0x1234)
	// 沒有 remake 存檔時，從原版的「選好 Middlegate 隊伍後離開名冊」狀態開始。
	// cmd/mm2 之後若 Restore 成功，會以存檔狀態覆蓋這裡；因此新局與續玩共用
	// 同一條載入管線，卻不會把新玩家放在 Go 零值的 (0,0)。
	start := game.StartMiddlegate
	gs.World.MapIndex, gs.World.X, gs.World.Y, gs.World.Face = start.Map, start.X, start.Y, start.Face
	gs.World.MarkExplored()
	attrs, err := game.ParseMapAttrs(must("ATTRIB.DAT"))
	if err != nil {
		return nil, err
	}
	gs.UseAttrs(attrs)
	if tbl, err := items.Parse(must("ITEMS.DAT")); err == nil {
		gs.UseItems(localizeItems(cat, tbl))
	}

	f, err := font.Parse(must("MM2.CH"))
	if err != nil {
		return nil, err
	}
	a := view.Assets{ASCII: f, Party: &game.Party{Members: gs.Party}}
	// 兩份 atlas：中文全形 24×24、英數字半形 12×24，同一套字型烘出來。
	// 載不到就退回原版 8×8 放大，畫面仍然可讀 —— 只是英文比中文粗一截。
	if b, err := os.ReadFile(filepath.Join("assets", "font", "cjk24.bin")); err == nil {
		if cf, err := cjk.Parse(b); err == nil {
			a.CJK = cf
		}
	}
	if b, err := os.ReadFile(filepath.Join("assets", "font", "lat24.bin")); err == nil {
		if lf, err := cjk.Parse(b); err == nil {
			a.Latin = lf
		}
	}
	var sets []*view.TownSet
	if t, err := loadTown(dataDir); err == nil {
		sets = append(sets, t)
	}
	// 其他平台的素材是選配：抽得出來就多一個可切換的選項，
	// 沒有就只有 DOS。**載不到不是錯誤**，玩家不一定有那份原版。
	if t, err := loadAmigaTown(amigaPath); err == nil {
		sets = append(sets, t)
	}
	if t, err := loadMSXTown(msxPath); err == nil {
		sets = append(sets, t)
	}
	for _, d := range modernPaths {
		if t, err := loadPackTown(d); err == nil {
			sets = append(sets, t)
			break
		}
	}
	if len(sets) > 0 {
		a.Town = sets[0]
	}
	monSets := make([]*monpack.Set, len(sets))
	// Mega Drive 的怪物是烘好的素材包（`tools/mdgfx.py --export`）。
	// 它沒有場景素材，所以多出來的這一套沿用第一套的牆 —— 切過去時
	// 換的只有怪物，訊息也是這樣寫的。
	if len(sets) > 0 {
		for _, dir := range []string{mdMonDir, amigaMonDir} {
			if pack, err := monpack.Load(dir); err == nil {
				sets = append(sets, sets[0])
				monSets = append(monSets, pack)
			}
		}
	}
	selected, err := selectTheme(sets, theme)
	if err != nil {
		return nil, err
	}
	setIdx := 0
	if selected != nil {
		a.Town = selected
		setIdx = themeSetIndex(sets, selected)
	}

	s := &Session{Game: gs, Assets: a, sets: sets, monSets: monSets,
		monPacks: map[*image.Paletted]*image.Paletted{}, setIdx: setIdx, scr: view.NewScreen(), townNames: townNamesCHT,
		Hints:    LoadHints("data"),
		monCache: map[int]gfx.MonsterPic{}, attrPick: -1, arenaTier: -1, TorchPhase: -1}
	// 怪物圖：載不到就不畫，不必讓整場遊玩失敗。
	if b, err := os.ReadFile(filepath.Join(dataDir, "MONSTERS.16")); err == nil {
		if idx, err := gfx.MonsterIndex(b); err == nil {
			s.monBlob, s.monIndex = b, idx
		}
	}
	// 片頭：`MASTER.16` 的 320×196 那一張，加上疊在上面的 13 張動畫。
	// 載不到就跳過片頭直接進遊戲 —— 少一個畫面，不是錯誤。
	if b, err := os.ReadFile(filepath.Join(dataDir, "MASTER.16")); err == nil {
		if in := loadIntro(b); in.Ready() {
			s.intro = in
		}
	}
	s.Ref = LoadReference(gamedata.Dir())
	s.rosterRaw = must("DEFAULT.DAT")
	if cat != nil {
		s.cat = cat
		s.Names(cat, defs)
		s.trans = eventText(cat, w)
	}
	return s, nil
}

// selectTheme 從已通過完整性檢查的素材集合選出初始平台。空值與 dos
// 都安全回到第一套（DOS）；明確指定卻沒有完整組時則失敗，不讓玩家以為
// 已套用某個平台而實際仍在看 DOS。
func selectTheme(sets []*view.TownSet, theme string) (*view.TownSet, error) {
	if theme == "" {
		if len(sets) == 0 {
			return nil, nil
		}
		return sets[0], nil
	}
	want := map[string]view.Platform{
		"dos":    view.PlatformDOS,
		"amiga":  view.PlatformAmiga,
		"msx":    view.PlatformMSX,
		"modern": view.PlatformModern,
	}[theme]
	for _, set := range sets {
		if set != nil && set.Platform == want {
			return set, nil
		}
	}
	return nil, fmt.Errorf("指定素材主題 %q 不可用：找不到完整素材組", theme)
}

// themeSetIndex 回傳目前選定素材在 F6 循環中的位置。選定指標若不在
// 集合中則安全回到 0；正常流程的 selected 一定來自 sets。
func themeSetIndex(sets []*view.TownSet, selected *view.TownSet) int {
	for i, set := range sets {
		if set == selected {
			return i
		}
	}
	return 0
}

// Names 把怪物名的譯文接上。
//
// **鍵是編號不是原文**（`monster.%03d`，見 cmd/mm2strings）。用原文當鍵
// 查不到任何一條，而查不到的行為就是顯示原文 —— 整批 256 個名字沒接上
// 也不會有任何錯誤，畫面看起來只是「還沒翻」。
func (s *Session) Names(cat *i18n.Catalog, defs []monsters.Monster) {
	src := make(map[string]string, len(defs))
	for _, d := range defs {
		src[fmt.Sprintf("monster.%03d", d.Index)] = d.Name
	}
	s.Game.Names = cat.SourceMap(src, "monster.")
}

// localizeItems 把物品名換成譯文（鍵同樣是編號，`item.%03d`）。
//
// 物品名進了不少地方：物品欄、商店、寶箱、鑑定與使用的播報。全部都讀
// `Session.Items` 的 `Name`，所以在這裡換掉一次就到處都是中文。
func localizeItems(cat *i18n.Catalog, tbl []items.Item) []items.Item {
	if cat == nil {
		return tbl
	}
	for i := range tbl {
		if t := cat.Or(fmt.Sprintf("item.%03d", tbl[i].Index), ""); t != "" {
			tbl[i].Name = t
		}
	}
	return tbl
}

// World 是這場遊玩的世界。
func (s *Session) World() *game.World { return s.Game.World }

// Key 送一個按鍵進去。回傳畫面有沒有變 —— 沒變就不必重畫。
func (s *Session) Key(k Key) bool {
	switch s.Mode {
	case ModeDead:
		return false
	case ModeMessage:
		if p := s.Game.World.Pending; p != nil {
			switch p.Kind {
			case game.PromptYesNo:
				switch k {
				case KeyYes:
					return s.resumeEventYesNo(true)
				case KeyNo:
					return s.resumeEventYesNo(false)
				case KeyConfirm:
					// 先讓玩家翻完提問文字；最後一行後仍停在 Y/N。
					return s.advance()
				}
				return false
			case game.PromptKey:
				if k != KeyConfirm {
					return false
				}
				return s.advance()
			}
		}
		if k != KeyConfirm {
			return false
		}
		return s.advance()
	case ModeText:
		if k == KeyConfirm {
			return s.resumeEventText()
		}
		return false
	case ModeCombat:
		// 原版的戰鬥指令是按字母直選（分派表在 `2COMBAT.img` `0x193F2`
		// 與跳表 `0x19578`）。這裡只綁已經實作的那幾條，其餘見
		// `docs/formats/08-combat.md` 的指令表。
		switch k {
		case KeyConfirm: // 攻擊：打完一回合
			return s.fightRound()
		case KeyRun: // R 溜跑
			return s.runAway()
		case KeyCast: // C 施法
			return s.open(menuCaster, s.castMenu())
		// F5／F6 是顯示設定不是戰鬥指令，戰鬥中也要能按 ——
		// **看得到怪物的時候正是想換素材的時候**。
		//
		// 兩支都會把 Mode 設成 ModeMessage（它們平常從探索模式進來），
		// 所以這裡一律設回來：**換素材不該把人踢出戰鬥**。訊息還是留在
		// `s.Lines` 上，戰鬥畫面的下方一樣看得到。
		case KeyStyle, KeyPlatform:
			var ok bool
			if k == KeyStyle {
				ok = s.toggleStyle()
			} else {
				ok = s.cyclePlatform()
			}
			s.Mode = ModeCombat
			return ok
		case KeyBlock: // B 抵擋：原版就是結束這個人的回合（`0x19442`）
			s.Lines = append(s.Lines, "隊伍原地防禦。")
			return s.fightRound()
		case KeyShoot: // 射擊：改用 +78／+79 那組欄位，而且打得到後排
			return s.shootRound()
		case KeyExch: // E 對調（`_2misc2_e02`）
			return s.open(menuExchange1, s.memberMenu("先選哪一位？"))
		case KeyProt: // P 顯示防護效能（`sub_1A882`）
			// 原版一條一行，這裡排成「標題：條目　條目」一行 ——
			// 下方那一塊只有三行，六條會被擠掉。
			l := s.Game.Fight.Protect.Lines()
			s.Lines = append(s.Lines, l[0]+"："+strings.Join(l[1:], "　"))
			return true
		case KeyView: // V 檢視目前這一位（`loc_19528`）
			s.Lines = append(s.Lines, s.viewLines(s.who)...)
			return true
		}
		return false
	case ModeMenu:
		return s.menuKey(k)
	case ModeMap:
		// 提示不只一頁時 ↑↓ 翻頁，其他鍵離開。
		switch k {
		case KeyUp:
			if s.hintPage > 0 {
				s.hintPage--
				return true
			}
			return false
		case KeyDown:
			if s.hintPage+1 < s.hintPages() {
				s.hintPage++
				return true
			}
			return false
		}
		s.Mode = ModeExplore
		return true
	case ModeWorld:
		s.Mode = ModeExplore
		return true
	case ModeIntro:
		// 任意鍵進遊戲。原版也是這樣：標題畫面按 Enter 才往下走。
		s.Mode = ModeExplore
		return true
	case ModeCreate:
		return s.createKey(k)
	case ModeName:
		return s.nameKey(k)
	}

	switch k {
	case KeyForward:
		return s.step(1)
	case KeyBack:
		return s.step(-1)
	case KeyLeft:
		s.Game.Turn(-1)
		return true
	case KeyRight:
		s.Game.Turn(1)
		return true
	case KeyRest:
		s.Lines = append(s.Game.RestAtInn(), s.Game.TrainParty()...)
		if len(s.Lines) > 0 {
			s.Mode = ModeMessage
		}
		return true
	case KeyCast:
		return s.open(menuCaster, s.castMenu())
	case KeyItems:
		return s.open(menuItems, s.itemMenu(s.who))
	case KeyRef:
		return s.open(menuRef, s.refMenu())
	case KeyBash:
		_, msg := s.Game.BashDoor()
		s.Lines = append(s.Lines, msg)
		if t := s.Game.Trap(); t != "" {
			s.Lines = append(s.Lines, t)
		}
		s.Mode = ModeMessage
		return true
	case KeyUnlock:
		return s.open(menuUnlock, s.unlockMenu())
	case KeyMap:
		s.Mode, s.hintPage = ModeMap, 0
		return true
	case KeyWorld:
		s.Mode = ModeWorld
		return true
	case KeyStyle:
		return s.toggleStyle()
	case KeyPlatform:
		return s.cyclePlatform()
	case KeyCreate:
		s.New = game.RollNewCharacter(s.Game.Rand)
		s.Mode = ModeCreate
		return true
	// 搜尋。原版 root `0x13814`：印 `Search...`，掃三個物品槽與金幣、
	// 寶石，五組全空就印 `Nothing Here!`，否則進 `_2misc_e02`。
	// 它不查地圖格也不記「這一格搜過了」—— 內容就是上一場戰鬥留下的東西。
	case KeySearch:
		if s.Chest != nil {
			// 戰利品到手才播，不是戰鬥一結束就播 —— 原版的戰利品
			// 要按 `S` 才撿得到，那一刻才是「拿到寶」。
			s.queueStinger(MusicCueTreasure)
		}
		if s.Chest == nil {
			// 原版把兩段印在同一列：`Search...` 在第 4 欄、
			// `Nothing Here!` 在第 0x10 欄（`sub_11676` 的兩次定位）。
			s.Lines = append(s.Lines, "搜尋……　這裡什麼都沒有。")
			s.Mode = ModeMessage
			return true
		}
		return s.open(menuChest, s.chestMenu())
	case KeySave:
		s.Lines = append(s.Lines, s.Save())
		s.Mode = ModeMessage
		return true
	case KeyShop:
		// 商店的類別與城號由目前所在的地圖決定：前五張圖是五座城。
		town := s.Game.World.MapIndex
		if town <= 4 {
			return s.open(menuSmith, listMenu("鐵匠鋪",
				[]string{"購買", "出售", "鑑定", "離開"}))
		}
		if town > 4 {
			s.Lines = append(s.Lines, "這裡沒有商店。")
			s.Mode = ModeMessage
			return true
		}
		return s.open(menuShop, s.shopMenu(0, town))
	}
	return false
}

// open 開一個選單。
func (s *Session) open(kind menuKind, m *Menu) bool {
	s.Menu = m
	s.menuKind = kind
	s.Mode = ModeMenu
	return true
}

// closeMenu 關掉選單。戰鬥中開的選單要回戰鬥，不是回探索 ——
// 否則施完法就從戰鬥裡「走出來」了。
func (s *Session) closeMenu() bool {
	s.Menu = nil
	s.menuKind = menuNone
	switch {
	case s.Game.Fight != nil:
		s.Mode = ModeCombat
	case len(s.Lines) > 0:
		s.Mode = ModeMessage
	default:
		s.Mode = ModeExplore
	}
	return true
}

// menuKey 處理選單模式下的按鍵。
//
// 方向鍵移游標、確認鍵選中、取消鍵退出。上下用 KeyUp／KeyDown，
// 而不是沿用 KeyForward／KeyBack —— 走路與選單共用同一顆實體鍵是
// 呼叫端的事，這一層要能分辨「我現在是在走路還是在選」。
func (s *Session) menuKey(k Key) bool {
	if s.Menu == nil {
		return s.closeMenu()
	}
	switch k {
	case KeyUp:
		return s.Menu.Move(-1)
	case KeyDown:
		return s.Menu.Move(1)
	case KeyCancel, KeyNo:
		if s.menuKind == menuEventMember {
			return s.resumeEventMember(0) // 原版 0x26 的 ESC
		}
		if s.isSpellPrompt() {
			return s.cancelSpellPrompt()
		}
		return s.closeMenu()
	case KeyConfirm, KeyYes:
		return s.choose()
	case KeyUse:
		if s.menuKind == menuItems {
			return s.useSelected()
		}
	}
	return false
}

// useSelected 對物品選單上被選中的那一格下「使用」。
//
// 原版把裝備與使用分成兩個指令（裝備在 `_2cmds_e03`、使用在
// `sub_1CED8`／`sub_1BA18`），所以這裡也分成兩顆鍵：確認是裝備／卸下，
// 這一顆是使用。
func (s *Session) useSelected() bool {
	i := s.Menu.Cur
	c := &s.Game.Party[s.who]
	name := "那一格"
	if i >= 0 && i < len(c.Items) && !c.Items[i].Empty() {
		name = s.itemName(c.Items[i].ID)
	}
	res := s.Game.UseItem(s.who, i)
	if res.Err != game.UseOK {
		s.Lines = append(s.Lines, fmt.Sprintf("%s：%s", name, res.Err.Error()))
		return s.closeMenu()
	}
	line := fmt.Sprintf("%s 用了 %s", c.Name, name)
	if res.SpellUsed {
		sp, ok := game.SpellByEngineIndex(res.Spell)
		if ok {
			line += "，發動" + sp.Name
		}
	}
	if res.Effect != "" {
		line += "：" + res.Effect
	}
	if res.UsedUp {
		line += "（用光了）"
	} else {
		line += fmt.Sprintf("（還剩 %d 次）", res.Spent)
	}
	s.Lines = append(s.Lines, line)
	return s.closeMenu()
}

// choose 處理「在選單上按下確認」。
func (s *Session) choose() bool {
	i := s.Menu.Cur
	switch s.menuKind {
	case menuCaster:
		if i >= len(s.casters) {
			return s.closeMenu()
		}
		s.who = s.casters[i]
		return s.open(menuSpell, s.spellMenu(s.who))
	case menuSpell:
		if len(s.spells) == 0 {
			s.Lines = append(s.Lines,
				fmt.Sprintf("%s 一個法術都還不會。", s.Game.Party[s.who].Name))
			return s.closeMenu()
		}
		if i >= len(s.spells) {
			return s.closeMenu()
		}
		return s.openSpellPrompt(i)
	case menuSpellMember:
		if i >= len(s.pickers) {
			return s.cancelSpellPrompt()
		}
		s.Game.Target = s.pickers[i]
		return s.finishSpellPrompt()
	case menuSpellItem:
		if i < 0 || i >= len(s.pickers) {
			return s.cancelSpellPrompt()
		}
		// 物品 consumer 的 packSlot 明確只接受施法者背包 0–5。
		slot := s.pickers[i]
		if slot >= 0 && slot < game.BackpackSlots {
			s.Game.Item = slot
			return s.finishSpellPrompt()
		}
		return s.cancelSpellPrompt()
	case menuSpellChoice:
		if i < 0 || i >= s.spellPrompt.Max-s.spellPrompt.Min+1 {
			return s.cancelSpellPrompt()
		}
		s.Game.Choice = s.spellPrompt.Min + i
		return s.finishSpellPrompt()
	case menuSpellColumn:
		if i < 0 || i >= s.spellPrompt.Columns {
			return s.cancelSpellPrompt()
		}
		s.Game.Column = i
		return s.open(menuSpellChoice, s.spellChoiceMenu("飛往哪一列？"))
	case menuShop:
		if i >= len(s.goods) {
			return s.closeMenu()
		}
		s.Lines = append(s.Lines, s.buy(s.goods[i]))
		return s.closeMenu()
	case menuRef:
		if s.Ref == nil {
			return s.closeMenu()
		}
		return s.open(menuRefSection, s.refSection(i))
	case menuRefSection:
		return s.open(menuRef, s.refMenu())
	case menuItems:
		return s.toggleEquip(i)
	case menuUnlock:
		return s.unlockBy(i)
	case menuExchange1:
		if i >= len(s.pickers) {
			return s.closeMenu()
		}
		s.exchFirst = s.pickers[i]
		return s.open(menuExchange2, s.memberMenu("跟哪一位對調？"))
	case menuExchange2:
		return s.exchangeWith(i)
	case menuChest:
		if i == int(game.ChestLeave)-1 {
			return s.chestDo(game.ChestLeave, 0)
		}
		s.chestAct = game.ChestAction(i + 1)
		return s.open(menuChestWho, s.memberMenu("由誰來？"))
	case menuInn:
		return s.innChoose(i)
	case menuInnAdd:
		if i >= len(s.pickers) {
			return s.closeMenu()
		}
		msg, _ := s.Game.AddToParty(s.pickers[i])
		s.Lines = append(s.Lines, msg)
		return s.open(menuInn, s.innMenu())
	case menuInnDrop:
		if i >= len(s.pickers) {
			return s.closeMenu()
		}
		msg, _ := s.Game.RemoveFromParty(s.pickers[i])
		s.Lines = append(s.Lines, msg)
		return s.open(menuInn, s.innMenu())
	case menuGuild:
		if i >= len(s.pickers) {
			return s.closeMenu()
		}
		msg, _ := s.Game.GuildBuy(s.Game.World.MapIndex, s.who, s.pickers[i])
		s.Lines = append(s.Lines, msg)
		return s.open(menuGuild, s.guildMenu())
	case menuDetox:
		// 原版是「先問付不付，再挑人」；這裡合成一個清單，
		// 挑誰就是同意付款，選「離開」等於答 N。語意相同、少按一次。
		if i >= 0 && i < len(s.pickers) {
			s.Lines = append(s.Lines, s.Game.Detox(s.pickers[i])...)
		} else {
			s.Lines = append(s.Lines, "隊伍離開大腦淨化中心。")
		}
		s.closeMenu()
		s.Mode = ModeMessage
		return true
	case menuTrain:
		if i == 0 {
			s.Lines = append(s.Lines, s.Game.TrainParty()...)
		} else {
			s.Lines = append(s.Lines, "隊伍離開訓練基地。")
		}
		s.closeMenu()
		s.Mode = ModeMessage
		return true
	case menuTavern:
		s.Lines = append(s.Lines, s.tavern(i)...)
		s.closeMenu()
		s.Mode = ModeMessage
		return true
	case menuSmith:
		return s.smithChoose(i)
	case menuSmithSell:
		if i >= len(s.pickers) {
			return s.closeMenu()
		}
		msg, _ := s.Game.SellItem(s.who, s.pickers[i])
		s.Lines = append(s.Lines, msg)
		return s.open(menuSmith, listMenu("鐵匠鋪",
			[]string{"購買", "出售", "鑑定", "離開"}))
	case menuSmithIdent:
		if i >= len(s.pickers) {
			return s.closeMenu()
		}
		lines, _ := s.Game.IdentifyItem(s.who, s.pickers[i])
		s.Lines = append(s.Lines, lines...)
		return s.open(menuSmith, listMenu("鐵匠鋪",
			[]string{"購買", "出售", "鑑定", "離開"}))
	case menuTemple:
		// 前四項是服務，第五項是「買法術」（原版選單的 D／E／F）。
		if i == len(game.TempleServiceNames) {
			return s.open(menuTempleBuy, s.templeMenu())
		}
		if i < 0 || i > len(game.TempleServiceNames) {
			return s.closeMenu()
		}
		s.Lines = append(s.Lines, s.Game.Serve(game.TempleService(i))...)
		s.closeMenu()
		s.Mode = ModeMessage
		return true
	case menuTempleBuy:
		if i >= 0 && i < len(s.pickers) {
			line, _ := s.Game.TempleBuy(s.Game.World.MapIndex, s.who, s.pickers[i])
			s.Lines = append(s.Lines, line)
		} else {
			s.Lines = append(s.Lines, "隊伍離開神殿。")
		}
		s.closeMenu()
		s.Mode = ModeMessage
		return true
	case menuCreateRace, menuCreateAlign, menuCreateSex:
		return s.createChoose(s.menuKind, i)
	case menuChestWho:
		if i >= len(s.pickers) {
			return s.closeMenu()
		}
		return s.chestDo(s.chestAct, s.pickers[i])
	case menuEventMember:
		if i < 0 || i >= len(s.pickers) {
			return s.resumeEventMember(0)
		}
		return s.resumeEventMember(s.pickers[i] + 1)
	}
	return s.closeMenu()
}

// toggleEquip 把背包那一格的東西裝起來，或把裝備那一格卸下來。
//
// 前六項是已裝備、後六項是背包（`Character.Equipped` 與 `Backpack`）。
// 裝備前先過 `CanEquip`：職業禁用、陣營不符、特殊能力 0xF0 都會擋下來
// （規則抄自 `2CMDS.img` 的裝備指令，見 `internal/game/equip.go`）。
// 部位衝突也擋（`game.SlotConflict`，抄自 `sub_1C8AA`）：同一部位已經
// 有東西、雙手武器與盾牌併用都不行。
func (s *Session) toggleEquip(i int) bool {
	c := &s.Game.Party[s.who]
	n := game.EquippedSlots
	switch {
	case i < 0 || i >= 2*n:
		return s.closeMenu()
	case i < n: // 已裝備 → 卸到背包第一個空格
		if c.Items[i].Empty() {
			s.Lines = append(s.Lines, "那一格是空的。")
			return s.closeMenu()
		}
		for j := 0; j < n; j++ {
			if c.Items[n+j].Empty() {
				name := s.itemName(c.Items[i].ID)
				c.Items[n+j], c.Items[i] = c.Items[i], game.ItemSlot{}
				c.RecomputeGear(s.Game.Items)
				s.Lines = append(s.Lines, fmt.Sprintf("卸下 %s。", name))
				return s.closeMenu()
			}
		}
		s.Lines = append(s.Lines, "背包滿了，卸不下來。")
		return s.closeMenu()
	default: // 背包 → 裝到裝備第一個空格
		if err := c.CanEquip(s.Game.Items, i-n); err != game.EquipOK {
			s.Lines = append(s.Lines, err.String())
			return s.closeMenu()
		}
		for j := 0; j < n; j++ {
			if c.Items[j].Empty() {
				name := s.itemName(c.Items[i].ID)
				c.Items[j], c.Items[i] = c.Items[i], game.ItemSlot{}
				c.RecomputeGear(s.Game.Items)
				s.Lines = append(s.Lines, fmt.Sprintf("裝備 %s。", name))
				return s.closeMenu()
			}
		}
		s.Lines = append(s.Lines, "六格裝備都滿了。")
		return s.closeMenu()
	}
}

// buy 用第一個人的錢買一件東西，放進他背包的第一個空格。
//
// 原版的買賣流程（誰付錢、放哪一格、背包滿了怎麼辦）還沒逐條反組譯，
// 所以這裡的規則是 remake 自己的，只保證兩件事與原版一致：
// **價格走 `ShopPrice`**（含商人技能與附魔等級的折扣），
// 而且**錢不夠就買不成**。
func (s *Session) buy(id int) string {
	c := &s.Game.Party[0]
	price := game.BuyPrice(s.Game.Items, id, c)
	name := s.itemName(id)
	if c.Gold < price {
		return fmt.Sprintf("買不起 %s（要 %d 金，身上只有 %d）", name, price, c.Gold)
	}
	pack := c.Backpack()
	for i := range pack {
		if pack[i].Empty() {
			c.Items[game.EquippedSlots+i] = game.ItemSlot{ID: id}
			c.Gold -= price
			return fmt.Sprintf("買下 %s，花了 %d 金", name, price)
		}
	}
	return "背包滿了。"
}

// step 走一步：可能觸發事件、可能遇敵。
func (s *Session) step(n int) bool {
	// 招牌只在這一步可見；下一次嘗試移動（即使撞牆）就不沿用舊格的名稱。
	s.Notice = ""
	moved, enc := s.Game.Step(n)
	s.refreshEventText()
	return s.settleEvent(moved, enc)
}

func (s *Session) resumeEventKey() bool {
	return s.resumeEvent(s.Game.ResumeEventKey)
}

func (s *Session) resumeEventYesNo(yes bool) bool {
	return s.resumeEvent(func() (*game.Encounter, bool) {
		return s.Game.ResumeEventYesNo(yes)
	})
}

func (s *Session) resumeEventMember(member int) bool {
	return s.resumeEvent(func() (*game.Encounter, bool) {
		return s.Game.ResumeEventMember(member)
	})
}

func (s *Session) resumeEventText() bool {
	if strings.TrimSpace(s.PromptText) == "" {
		// 原版空白輸入會留在輸入器裡重問，不能把它當成預設答錯。
		return false
	}
	answer := s.PromptText
	return s.resumeEvent(func() (*game.Encounter, bool) {
		return s.Game.ResumeEventText(answer)
	})
}

func (s *Session) resumeEvent(resume func() (*game.Encounter, bool)) bool {
	s.Notice = ""
	// 這些行是剛剛已顯示的提問；答案一送出就不能在後半段腳本完成後
	// 留在畫面上，否則 UI 仍會誤以為有一條普通訊息待確認。
	s.Lines = nil
	if s.menuKind == menuEventMember {
		s.Menu = nil
		s.menuKind = menuNone
	}
	enc, ok := resume()
	if !ok {
		return false
	}
	s.PromptText = ""
	s.refreshEventText()
	return s.settleEvent(true, enc)
}

func (s *Session) refreshEventText() {
	if s.cat != nil {
		// 事件可能從目前地圖切到腳本庫，或在回答後傳送到別張圖；來源
		// 段號而非座標決定翻譯鍵，所以每次事件狀態轉移都重建。
		s.trans = eventText(s.cat, s.Game.World)
	}
}

// settleEvent 把 game 層這一次已完成的狀態變化變成 UI 模式。eventText
// 表示這一次真的執行過事件腳本；招牌沒有輸入閘門時只做 Notice，不可鎖住人。
func (s *Session) settleEvent(eventText bool, enc *game.Encounter) bool {
	log := s.Game.Log
	// 沒有輸入閘門的事件文字（特別是城鎮 `04 NN` 招牌）照樣畫出來，
	// 但不得混進 Lines；有 Pending 的文字則由 prompt 模式鎖住輸入。
	if eventText && s.Game.World.Message != "" && !s.Game.World.MessageWait {
		s.Notice = s.localized(s.Game.World.Message)
		// game.Session 依序把事件文字、再把其他結果放進 Log；只拿走第一
		// 項，避免把同一步的撞牆、房間或設施播報一併吞掉。
		if len(log) > 0 && log[0] == s.Game.World.Message {
			log = log[1:]
		}
	}
	s.take(log)
	if s.openEventPrompt() {
		return true
	}
	if enc != nil {
		s.Game.Fight = enc
		s.Mode = ModeCombat
		s.Lines = append(s.Lines, s.encounterLine(enc))
		return true
	}
	// 有選單的設施，踩進去就開。
	switch s.Game.Facility {
	case game.FacilityTemple:
		return s.open(menuTemple, s.templeServiceMenu())
	case game.FacilityInn:
		return s.open(menuInn, s.innMenu())
	case game.FacilityMageGuild:
		return s.open(menuGuild, s.guildMenu())
	case game.FacilityTraining:
		return s.open(menuTrain, listMenu("訓練基地", []string{"受訓", "離開"}))
	case game.FacilityTavern:
		return s.open(menuTavern, listMenu("酒館", []string{"買一輪", "報名競技賽", "離開"}))
	case game.FacilityBlacksmith:
		return s.open(menuSmith, listMenu("鐵匠鋪",
			[]string{"購買", "出售", "鑑定", "離開"}))
	case game.FacilityBrainDetox:
		return s.open(menuDetox, s.detoxMenu())
	}
	if len(s.Lines) > 0 {
		s.Mode = ModeMessage
	} else {
		s.Mode = ModeExplore
	}
	return true
}

// openEventPrompt 讓原版事件的輸入種類直接決定畫面，而不是先在探索時
// 設一個「下一題答案」。Y/N／等鍵共用訊息模式；選人與文字各有可操作 UI。
func (s *Session) openEventPrompt() bool {
	p := s.Game.World.Pending
	if p == nil {
		return false
	}
	switch p.Kind {
	case game.PromptKey, game.PromptYesNo:
		s.Mode = ModeMessage
	case game.PromptMember:
		return s.open(menuEventMember, s.eventMemberMenu())
	case game.PromptText:
		s.PromptText = ""
		s.Mode = ModeText
	default:
		return false
	}
	return true
}

func (s *Session) eventMemberMenu() *Menu {
	m := &Menu{Title: "請選一名隊員"}
	s.pickers = s.pickers[:0]
	for i := range s.Game.Party {
		c := &s.Game.Party[i]
		if c.CondBits >= game.CondPetrified {
			continue
		}
		s.pickers = append(s.pickers, i)
		m.Items = append(m.Items, fmt.Sprintf("%d. %s", i+1, c.Name))
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（沒有能選的隊員；按 Esc 取消）")
	}
	return m
}

// take 收下引擎這一步產生的訊息，順便換成譯文。
// Take 把引擎的紀錄搬進訊息佇列。測試用的公開入口。
func (s *Session) Take() { s.take(s.Game.Log) }

func (s *Session) take(log []string) {
	for _, l := range log {
		for _, part := range strings.Split(l, "\n") {
			if part == "" {
				continue
			}
			s.Lines = append(s.Lines, s.localized(part))
		}
	}
	s.Game.Log = nil
	// 打字謎題把答案附在後面。
	//
	// 原版的謎底靠英文文字遊戲（`What has Mark lost?` → `KEYS`、
	// 六塊巨石 → `DRUIDS`），翻成中文之後線索與答案對不起來，
	// **玩家永遠解不開**。這是中文化必然要處理的一類，不是作弊選項。
	if a := s.Game.World.TextExpect; a != "" {
		s.Lines = append(s.Lines, fmt.Sprintf("（要輸入的答案：%s）", a))
	}
}

// localized 把一條原版事件字串換成目前地圖的譯文；找不到時保留原文，
// 因此自製訊息與尚未收錄的原版字串不會消失。
func (s *Session) localized(line string) string {
	if t, ok := s.trans[line]; ok {
		return t
	}
	return line
}

// advance 推掉一條訊息；推完就回探索。
func (s *Session) advance() bool {
	if len(s.Lines) > 0 {
		s.Lines = s.Lines[1:]
	}
	if len(s.Lines) > 0 {
		return true
	}
	if p := s.Game.World.Pending; p != nil {
		switch p.Kind {
		case game.PromptKey:
			return s.resumeEventKey()
		case game.PromptYesNo:
			// 文字翻完仍在問 Y/N；不可把 Enter 偷當成任一答案。
			s.Mode = ModeMessage
			return true
		case game.PromptMember, game.PromptText:
			return s.openEventPrompt()
		}
	}
	s.Mode = ModeExplore
	return true
}

// fightRound 打一回合。全部打完就結算。
func (s *Session) fightRound() bool {
	enc := s.Game.Fight
	if enc == nil {
		s.Mode = ModeExplore
		return true
	}
	for _, line := range enc.Fight(s.Game.Rand, 1) {
		s.Lines = append(s.Lines, line)
	}
	// 這一回合倒下的人與怪各給一個一次性音效。撞在一起時
	// `queueStinger` 依重要性挑一個。
	if enc.Killed > 0 {
		s.queueStinger(MusicCueEnemyKilled)
	}
	if enc.Lost > 0 {
		s.queueStinger(MusicCueMemberKilled)
	}
	if !enc.Over() {
		return true
	}
	won := enc.PartyWon()
	if won {
		s.queueStinger(MusicCueVictory)
	} else {
		s.queueStinger(MusicCueDefeat)
	}
	var chest *game.Chest
	if won {
		exp := enc.AwardExp(s.Game.Party)
		s.Lines = append(s.Lines, fmt.Sprintf("隊伍獲勝，每人獲得 %d 點經驗", exp))
		if s.arenaTier >= 0 {
			s.Lines = append(s.Lines, s.Game.ArenaReward(s.arenaTier)...)
		} else {
			// 一般戰鬥勝利才建立一般寶箱；競技賽與事件 0x2a
			// 仍各走自己的獎賞分支。
			chest = enc.VictoryChestFromItems(s.Game.Rand, s.Game.Items)
		}
	} else {
		s.Lines = append(s.Lines, "隊伍全滅")
	}
	// 不論輸贏，這一場都不再是競技賽了。
	s.arenaTier = -1
	s.Game.EndCombat()
	if !s.Game.Alive() {
		s.Mode = ModeDead
		return true
	}
	// 原版不會自動把戰利品端到玩家面前：`2COMBAT sub_19BF8` 把 `ds:0434`
	// 清成 0、戰利品留在 `ds:6950` 那五組陣列裡，要按 `S` 才撿得到
	// （珍017 上冊 p.39：「戰鬥後不要忘了用 S 找尋戰利品」）。
	if chest != nil {
		s.Chest = chest
		s.Lines = append(s.Lines, "怪物留下了東西，按 S 搜尋。")
	}
	s.Mode = ModeMessage
	return true
}

// viewLines 是「檢視」指令印的內容。
//
// 原版的 `V` 走 `loc_19528`：把鍵碼換成 `目前這一位 + '1'`，再交給
// 同一支人物資料畫面（`loc_1716E`）—— 也就是說 `V` 與直接按數字鍵
// 是同一件事，差別只在它幫你填了「哪一位」。
func (s *Session) viewLines(who int) []string {
	if who < 0 || who >= len(s.Game.Party) {
		return []string{"沒有這一位隊員。"}
	}
	c := &s.Game.Party[who]
	lines := []string{fmt.Sprintf("%s　%s　等級 %d", c.Name, c.Class, c.EffectiveLevel())}
	if lv := c.EffectiveLevel(); lv != c.Level {
		lines[0] += fmt.Sprintf("（本體 %d）", c.Level)
	}
	lines = append(lines,
		fmt.Sprintf("生命 %d/%d　法力 %d/%d　防護 %d",
			c.HP, c.MaxHP, c.SP, c.MaxSP, c.AC),
		fmt.Sprintf("狀況 %s　揮擊 %d 次", c.Condition, c.AttacksPerRound()))
	return lines
}

// shootRound 是戰鬥指令「射擊」：這一回合整隊改用射擊。
//
// 與近戰的差別有兩處，兩處都在 `sub_18DAA`：傷害與命中改讀記錄的
// `+78`／`+79`，可選目標從前排 `ds:9FC5` 換成場上全部 `ds:0508`。
// 打完就把旗標放掉，下一回合回到近戰 —— 原版的 `ds:54A4` 也是每次
// 下指令重設。
func (s *Session) shootRound() bool {
	enc := s.Game.Fight
	if enc == nil {
		s.Mode = ModeExplore
		return true
	}
	enc.Ranged = true
	s.Lines = append(s.Lines, "隊伍射擊。")
	ok := s.fightRound()
	enc.Ranged = false
	return ok
}

// runAway 是戰鬥指令 `R`：目前這一位擲一次，成功就脫離戰鬥。
//
// 成功率是這張地圖的 `ATTRIB +13`（`MapAttr.RunChance`），城鎮一律 100。
// **跑掉的是一個人不是整隊** —— 原版就是這樣（見 `docs/formats/08`）。
func (s *Session) runAway() bool {
	enc := s.Game.Fight
	if enc == nil {
		s.Mode = ModeExplore
		return true
	}
	chance := 0
	if a := s.Game.CurrentAttr(); a != nil {
		chance = a.RunChance()
	}
	who := 0
	name := "隊員"
	if who < len(enc.Party) {
		name = enc.Party[who].CombatName()
	}
	if enc.TryRun(s.Game.Rand, who, chance) {
		s.Lines = append(s.Lines, name+" 溜走了！")
	} else {
		s.Lines = append(s.Lines, name+" 沒跑掉。")
	}
	if len(enc.Party) == 0 {
		s.Lines = append(s.Lines, "全隊都跑光了。")
		s.arenaTier = -1
		s.Game.EndCombat()
		s.Mode = ModeMessage
	}
	return true
}

func (s *Session) encounterLine(enc *game.Encounter) string {
	if len(enc.Monsters) == 0 {
		return "遭遇！"
	}
	return fmt.Sprintf("%s 出現了（%d 隻）！",
		enc.Monsters[0].CombatName(), len(enc.Monsters))
}

// Message 是目前要顯示在訊息區的那一條。
//
// 法術清單開著時顯示的是**游標那條法術的說明** —— 消耗、形式、對象、
// 效果。那些內容原版只印在紙本說明書上，遊戲裡查不到；這個 remake 的
// 目標之一就是把它們收進遊戲，不必再翻紙本（`data/spells.json` 的
// `Desc` 抄自珍017 中文說明書）。
func (s *Session) Message() string {
	if s.Mode == ModeMenu && s.menuKind == menuRefSection && s.Menu != nil {
		return s.refDetail(s.Menu.Cur)
	}
	if s.Mode == ModeMenu && s.menuKind == menuSpell && s.Menu != nil {
		if i := s.Menu.Cur; i >= 0 && i < len(s.spellInfo) {
			sp := s.spellInfo[i]
			return fmt.Sprintf("%s（%s）%s／%s@%s",
				sp.Name, sp.Cost, sp.Form, sp.Target, sp.Desc)
		}
	}
	if len(s.Lines) > 0 {
		return s.Lines[0]
	}
	return s.Notice
}

// Draw 把目前狀態畫進畫面並回傳。
//
// 選單開著時蓋在視圖上 —— 原版也是把選單畫在同一塊區域，
// 不另開視窗。
func (s *Session) Draw() *render.Screen {
	if s.Mode == ModeMap {
		view.DrawMap(s.scr, s.Game.World, s.Assets, s.mapInfo())
		return s.scr
	}
	if s.Mode == ModeWorld {
		view.DrawWorld(s.scr, s.Assets, s.worldInfo())
		return s.scr
	}
	if s.Mode == ModeIntro {
		view.DrawIntro(s.scr, s.intro, s.packTick, s.Assets, "按任意鍵開始")
		return s.scr
	}
	var menu []string
	switch {
	case s.Mode == ModeMenu && s.Menu != nil:
		menu = s.Menu.Lines()
	case s.Mode == ModeCreate:
		menu = s.CreateLines()
	case s.Mode == ModeName:
		menu = s.NameLines()
	case s.Mode == ModeText:
		menu = s.PromptLines()
	}
	a := s.Assets
	a.Place = s.mapTitle()
	// 戰鬥中把怪物疊上去 —— 沒有這一步，打起來畫面上一隻怪都看不到。
	if menu == nil && s.Mode == ModeCombat {
		a.Monsters = s.sprites()
	}
	phase := s.phase
	if s.TorchPhase >= 0 {
		phase = s.TorchPhase
	}
	view.DrawPhase(s.scr, s.Game.World, a, s.Message(), menu, phase)
	return s.scr
}

// townNamesCHT 是五座城鎮的譯名，索引即地圖編號。
//
// 順序來自 `MM2.EXE` 尾部的城鎮列表，與 MAP 段同序（docs/formats/06 §3）；
// 譯名取自 translations/glossary.md（手冊有兩種寫法時取地圖集那一版）。
var townNamesCHT = []string{"米德格特", "亞特蘭汀", "桑達拉", "佛卡尼亞", "桑德索巴"}

// sprites 把場上的怪物換成可以畫的圖。
//
// 圖號在怪物記錄 `+0x15`（`Sprite`），要先經 `ResolveMonsterPic` ——
// 索引表指到空槽時原版會往後借一張（`sub_6818`）。
func (s *Session) sprites() []view.MonsterSprite {
	f := s.Game.Fight
	if f == nil {
		s.resetMonsterAnimations()
		return nil
	}
	if pack := s.monPack(); pack != nil {
		return s.packSprites(f, pack)
	}
	if s.monBlob == nil {
		s.resetMonsterAnimations()
		return nil
	}
	s.syncMonsterAnimations(f)
	var out []view.MonsterSprite
	for i, c := range f.Monsters {
		m, ok := c.(*game.Monster)
		if !ok {
			continue
		}
		slot := gfx.ResolveMonsterPic(s.monIndex, m.Def.Sprite)
		if slot < 0 {
			continue
		}
		pic, ok := s.monCache[slot]
		if !ok {
			p, err := gfx.ParseMonsterPic(s.monBlob, slot)
			if err != nil {
				continue
			}
			s.monCache[slot] = p
			pic = p
		}
		state := s.monsterAnimStates[i]
		out = append(out, view.MonsterSprite{Pic: pic, Anim: state.anim, Step: state.step})
	}
	return out
}

// monPack 是目前這一套素材要用的怪物包，沒有就回 nil（用 DOS 的）。
func (s *Session) monPack() *monpack.Set {
	if s.setIdx < 0 || s.setIdx >= len(s.monSets) {
		return nil
	}
	return s.monSets[s.setIdx]
}

// packSprites 用素材包畫怪物。
//
// 影格照 `packTick` 輪播：素材包裡沒有原版那張動畫表（Mega Drive 是
// 每張圖自己一串影格），所以這裡就是等速循環，**不宣稱與原版同步**。
func (s *Session) packSprites(f *game.Encounter, pack *monpack.Set) []view.MonsterSprite {
	var out []view.MonsterSprite
	for _, c := range f.Monsters {
		m, ok := c.(*game.Monster)
		if !ok {
			continue
		}
		slot := gfx.ResolveMonsterPic(s.monIndex, m.Def.Sprite)
		frames := pack.Pics[slot]
		if len(frames) == 0 {
			continue
		}
		scaled := make([]*image.Paletted, len(frames))
		for i, im := range frames {
			up, ok := s.monPacks[im]
			if !ok {
				up = render.ScaleN(im, render.Scale)
				s.monPacks[im] = up
			}
			scaled[i] = up
		}
		out = append(out, view.MonsterSprite{
			Pack: scaled, PackStep: s.packTick / packHold,
			PackClear: monpack.TransparentIndex,
		})
	}
	return out
}

// packHold 是素材包的影格各停留幾個 tick。原版的 hold 表是 DOS 那份的，
// 換平台之後不適用，所以取一個看得出在動又不刺眼的值。
const packHold = 4

func (s *Session) resetMonsterAnimations() {
	s.monsterAnimFight = nil
	s.monsterAnimStates = nil
}

// syncMonsterAnimations 建立／保留每隻怪物的動畫游標。動畫資料的語意目前
// 是 remake 的 strong inference：只選第一段完整、非空的序列，不宣稱是
// 原版 idle 行為。
func (s *Session) syncMonsterAnimations(f *game.Encounter) {
	if s.monsterAnimFight != f || len(s.monsterAnimStates) != len(f.Monsters) {
		s.monsterAnimFight = f
		s.monsterAnimStates = make([]monsterAnimState, len(f.Monsters))
		for i := range s.monsterAnimStates {
			s.monsterAnimStates[i].picSlot = -1
			s.monsterAnimStates[i].anim = -1
		}
	}
	for i, c := range f.Monsters {
		m, ok := c.(*game.Monster)
		if !ok {
			s.monsterAnimStates[i].anim = -1
			continue
		}
		slot := gfx.ResolveMonsterPic(s.monIndex, m.Def.Sprite)
		state := &s.monsterAnimStates[i]
		if state.picSlot == slot {
			continue
		}
		state.picSlot, state.anim, state.step, state.hold = slot, -1, 0, 0
		pic, ok := s.monCache[slot]
		if !ok {
			p, err := gfx.ParseMonsterPic(s.monBlob, slot)
			if err != nil {
				continue
			}
			if s.monCache == nil {
				s.monCache = map[int]gfx.MonsterPic{}
			}
			s.monCache[slot], pic, ok = p, p, true
		}
		state.script = 0
		s.enterMonsterScriptStep(pic, state)
	}
}

// enterMonsterScriptStep 照播放腳本挑下一段。
//
// 原版 root `0x15715` 起的迴圈：讀腳本的段編號位元組，bit 7 設起來就
// `rand(1, 低7位)` 隨機挑，否則固定用那一段；播完一段再讀腳本的下一項。
// 段編號是 1 起算，對到 `Anims[Seq-1]`。
//
// 腳本或段指不到時把 anim 設成 -1（退回基準圖），不再像先前那樣
// 「跳過非法段找下一段」—— 影格編號越界不是非法，原版會畫影格 0。
func (s *Session) enterMonsterScriptStep(pic gfx.MonsterPic, state *monsterAnimState) {
	state.anim, state.step, state.hold = -1, 0, 0
	if len(pic.Script) == 0 || len(pic.Anims) == 0 {
		return
	}
	state.script %= len(pic.Script)
	st := pic.Script[state.script]
	seq := st.Seq
	if st.Random && seq > 0 && s.Game != nil && s.Game.Rand != nil {
		seq = s.Game.Rand.Range(1, seq)
	}
	if seq < 1 || seq > len(pic.Anims) || len(pic.Anims[seq-1]) == 0 {
		return
	}
	state.anim = seq - 1
	state.hold = safeMonsterHold(pic.Anims[seq-1][0].Hold)
}

func safeMonsterHold(hold int) int {
	if hold <= 0 {
		return 1
	}
	return hold
}

func (s *Session) advanceMonsterAnimations() {
	f := s.Game.Fight
	if f == nil {
		s.resetMonsterAnimations()
		return
	}
	s.syncMonsterAnimations(f)
	for i, state := range s.monsterAnimStates {
		if state.anim < 0 || i >= len(f.Monsters) {
			continue
		}
		pic, ok := s.monCache[state.picSlot]
		if !ok || state.anim >= len(pic.Anims) {
			continue
		}
		seq := pic.Anims[state.anim]
		state.hold = safeMonsterHold(state.hold) - 1
		if state.hold <= 0 {
			state.step++
			if state.step >= len(seq) {
				// 一段播完 → 腳本前進一項再挑（原版 `sub_15772` 走到
				// `FF` 就把指標設回 -1，外層迴圈讀腳本的下一對）。
				state.script++
				s.enterMonsterScriptStep(pic, &state)
			} else {
				state.hold = safeMonsterHold(seq[state.step].Hold)
			}
		}
		s.monsterAnimStates[i] = state
	}
}

// Tick 前進一格火炬動畫，回報畫面要不要重畫。
//
// 動畫與遊戲邏輯分開：邏輯是「按一次鍵走一步」，火炬是連續的。
// 呼叫端（Ebiten 的 Update）自己決定多久叫一次。
func (s *Session) Tick() bool {
	s.phase = (s.phase + 1) % view.TorchFrames
	s.packTick++
	s.advanceMonsterAnimations()
	return true
}

// mapTitle 是地圖畫面的標題：有地名就用地名。
//
// 五座城鎮的名字來自 `MM2.EXE` 尾部的城鎮列表（與 MAP 段同序，
// 見 docs/formats/06 §3），其餘只印編號 —— 沒查證過的名字不編。
func (s *Session) mapTitle() string {
	i := s.Game.World.MapIndex
	if i >= 0 && i < len(s.townNames) {
		return s.townNames[i]
	}
	return ""
}

// mapInfo 湊出地圖畫面要的東西：地名與這張圖的攻略提示。
//
// 這張圖沒有對應的提示時退回通用提示 —— 空著一片比較糟，
// 而通用那幾條（職業考驗、經驗值上限之類）在哪裡看都成立。
func (s *Session) mapInfo() view.MapInfo {
	info := view.MapInfo{Title: s.mapTitle(), HintPage: s.hintPage}
	title, lines := s.Hints.ForMap(s.Game.World.MapIndex)
	if len(lines) == 0 {
		title, lines = "通用提示", s.Hints.GeneralLines()
	}
	info.HintTitle, info.Hints = title, lines
	return info
}

// worldInfo 組出世界地圖那一頁要畫的東西。
//
// 網格由玩家自己那份 `ATTRIB.DAT` 現算（`game.WorldGrid`），地名取自
// `data/reference.json` 的 `worldMap` —— 說明書那一頁只給區域碼與地名，
// 圖是掃描件不能散布，所以圖自己畫、字沿用轉錄。
func (s *Session) worldInfo() view.WorldInfo {
	info := view.WorldInfo{Place: s.mapTitle(), Names: map[string][]string{}}
	if s.Ref != nil {
		for _, r := range s.Ref.WorldMap {
			if len(r.Cols) >= 2 {
				info.Names[r.Cols[0]] = append(info.Names[r.Cols[0]], r.Cols[1])
			}
		}
	}
	grid := game.WorldGrid(s.Game.Attrs)
	for _, row := range grid {
		cells := make([]view.WorldCellInfo, 0, len(row))
		for _, m := range row {
			cells = append(cells, view.WorldCellInfo{
				Region:  game.RegionOf(s.Game.Attrs, m),
				Map:     m,
				Tileset: game.WorldTileset(s.Game.Attrs, m),
				Seen:    s.Game.World.Explored.Count(m) > 0,
			})
		}
		info.Grid = append(info.Grid, cells)
	}
	info.Here = game.RegionOf(s.Game.Attrs, s.Game.World.MapIndex)
	return info
}

// hintPages 是目前這張地圖的提示共幾頁。
func (s *Session) hintPages() int {
	_, lines := s.Hints.ForMap(s.Game.World.MapIndex)
	if len(lines) == 0 {
		lines = s.Hints.GeneralLines()
	}
	return view.HintPages(lines, view.HintRowsPerPage)
}

// toggleStyle 在原版像素與 Scale3x 之間切換，並把結果講出來。
//
// 做成可以隨時切換而不是啟動選項，是因為**這件事只能用眼睛驗收**：
// 兩種畫法的差別在邊界，靜態截圖並排看不出哪一種在遊玩時比較舒服。
// 當場切換才比得出來。
func (s *Session) toggleStyle() bool {
	if s.Assets.Town == nil {
		return false
	}
	if s.Assets.Town.Fixed() {
		s.Lines = append(s.Lines, "這一套素材本來就是高解析的，沒有原版像素可以切。")
		s.Mode = ModeMessage
		return true
	}
	if s.Assets.Town.Style == view.StyleModern {
		s.Assets.Town.Style = view.StyleClassic
		s.Lines = append(s.Lines, "牆面改回原版像素。")
	} else {
		s.Assets.Town.Style = view.StyleModern
		s.Lines = append(s.Lines, "牆面改用平滑放大（Scale3x）。")
	}
	s.Assets.Town.Prepare()
	s.Mode = ModeMessage
	return true
}

// cyclePlatform 換到下一個平台的素材。
//
// 風格（原版像素／Scale3x）跟著一起帶過去 —— 兩個設定是正交的，
// 換平台不該把玩家剛選好的風格重設掉。
func (s *Session) cyclePlatform() bool {
	if len(s.sets) < 2 {
		s.Lines = append(s.Lines, "只有 DOS 版素材可用。")
		s.Mode = ModeMessage
		return true
	}
	style := s.Assets.Town.Style
	s.setIdx = (s.setIdx + 1) % len(s.sets)
	s.Assets.Town = s.sets[s.setIdx]
	s.Assets.Town.Style = style
	// 放大在這裡一次算完。**不在載入時算**：多數玩家一輩子不會按這個鍵，
	// 卻要每次開檔多等 0.25 秒；算在按鍵當下只停一次，而且停在
	// 「玩家剛下指令」那一刻 —— 那是唯一一個停頓不像當掉的時機。
	s.Assets.Town.Prepare()
	if p := s.monPack(); p != nil {
		s.Lines = append(s.Lines, "怪物換成 "+p.Source+"（場景沿用 "+
			s.Assets.Town.Platform.String()+"）。")
	} else {
		s.Lines = append(s.Lines, "場景素材換成 "+s.Assets.Town.Platform.String()+" 版。")
	}
	s.Mode = ModeMessage
	return true
}

const (
	// amigaDir 與 `-data` 一樣是 repo 相對路徑。Amiga 版是原版資料，
	// 不進版控（`workplace/` 整個 gitignore），玩家自備。
	amigaDir = "workplace/amiga"
	// msxDir 放 MSX 版的磁片映像（`.dsk`）。同樣是原版資料，玩家自備。
	msxDir = "workplace/msx"
	// monPackDirs 是烘好的怪物素材包，由 `tools/mdgfx.py --export` 與
	// `tools/amiga32.py --export-monsters` 產生。同樣是原版美術，不進版控。
	mdMonDir    = "workplace/md-monsters"
	amigaMonDir = "workplace/amiga-monsters"
)

// modernDirs 是高解析素材包的搜尋順序。
//
//	assets/modern     重畫的原創美術，跟著 repo 走
//	workplace/modern  玩家自己用 cmd/mm2modern 從原版烘的，不進版控
//
// 兩者的差別不是畫質而是**授權**：放大過的原版美術仍然是原版美術。
var modernDirs = []string{"assets/modern", "workplace/modern"}

// packManifest 是素材包的 `set.json`，欄位見 cmd/mm2modern。
type packManifest struct {
	Source      string `json:"source"`
	Clear       int    `json:"clear"`
	TorchStride int    `json:"torchStride"`
	Scale       int    `json:"scale"`
}

// loadPackTown 載入烘好的高解析素材包。
func loadPackTown(dir string) (*view.TownSet, error) {
	b, err := os.ReadFile(filepath.Join(dir, "set.json"))
	if err != nil {
		return nil, err
	}
	var mf packManifest
	if err := json.Unmarshal(b, &mf); err != nil {
		return nil, err
	}
	// 倍率不合就不要載。畫出來會是「位置對、大小錯」，那看起來像
	// 座標算錯，不像素材選錯 —— 在這裡擋掉比在畫面上找便宜得多。
	if mf.Scale != render.Scale {
		return nil, fmt.Errorf("素材包放大 %d 倍，畫面要 %d 倍", mf.Scale, render.Scale)
	}
	group := func(name string, minimum int) ([]*image.Paletted, error) {
		var out []*image.Paletted
		for i := 0; ; i++ {
			f, err := os.Open(filepath.Join(dir, name, fmt.Sprintf("%02d.png", i)))
			if err != nil {
				if i == 0 || i < minimum {
					return nil, err
				}
				return out, nil
			}
			im, err := png.Decode(f)
			f.Close()
			if err != nil {
				return nil, err
			}
			pi, ok := im.(*image.Paletted)
			if !ok {
				// 索引色是必要的：透空色是**色號**不是顏色，
				// 存成 RGB 之後那個號碼就沒了。
				return nil, fmt.Errorf("%s/%02d.png 不是索引色", name, i)
			}
			out = append(out, pi)
		}
	}
	walls, err := group("walls", 32)
	if err != nil {
		return nil, err
	}
	floor, err := group("floor", 1)
	if err != nil {
		return nil, err
	}
	torch, err := group("torch", 36)
	if err != nil {
		return nil, err
	}
	sky, err := group("sky", 1)
	if err != nil {
		return nil, err
	}
	set := view.NewPackSet(view.PlatformModern, walls, floor, torch, sky,
		uint8(mf.Clear), mf.TorchStride)
	return requireTownSet(set)
}

// loadAmigaTown 載入 Amiga 版的城鎮素材。
//
// 檔名與 DOS 同名只差大小寫，張數與排列也一一對應，所以幾何完全共用。
// 兩個平台真正的差異只有透空色（DOS 8、Amiga 0）與火炬的張數。
func loadAmigaTown(dir string) (*view.TownSet, error) {
	set := func(name string) ([]*image.Paletted, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		st, err := amiga.Parse(b)
		if err != nil {
			return nil, err
		}
		return st.Images, nil
	}
	walls, err := set("town.32")
	if err != nil {
		return nil, err
	}
	floor, err := set("townf.32")
	if err != nil {
		return nil, err
	}
	torch, err := set("townt.32")
	if err != nil {
		return nil, err
	}
	sky, err := set("sky.32")
	if err != nil {
		return nil, err
	}
	town := view.NewSceneSet(view.PlatformAmiga, walls, floor, torch, sky,
		amiga.TransparentIndex, view.AmigaTorchStride)
	return requireTownSet(town)
}

// loadMSXTown 從 MSX 版的磁片載第一人稱素材。
//
// 與 DOS／Amiga 不同的是**素材不是一張一張的檔案**：整套場景是一張
// 462×128 的素材表，每一面牆是表裡的一塊矩形，落點另有一張表
// （見 `internal/assets/msx`）。原版靠 VDP 的矩形搬移組出畫面，
// remake 直接切圖來貼，不必模擬 VRAM。
func loadMSXTown(dir string) (*view.TownSet, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.dsk"))
	if err != nil || len(names) == 0 {
		return nil, fmt.Errorf("msx: %s 底下沒有 .dsk", dir)
	}
	for _, n := range names {
		// 兩片各有一個 `[a]` 版本，內容重複。
		if strings.Contains(n, "[a]") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			continue
		}
		d, err := msx.Open(b)
		if err != nil {
			continue
		}
		pal, err := d.Palette()
		if err != nil {
			continue // 只有第一片有常駐引擎，調色盤取不到就換下一片
		}
		sheet, err := d.Image(msx.SceneID[0], pal)
		if err != nil {
			continue
		}
		walls, torches, place, torchPlace, bg := msx.Scene(sheet)
		set := view.NewPlacedSet(view.PlatformMSX, walls, torches, place, torchPlace,
			bg, 0, msx.TorchFrames, image.Pt(msx.ViewW, msx.ViewH))
		return requireTownSet(set)
	}
	return nil, fmt.Errorf("msx: %s 底下的 .dsk 都讀不出場景素材", dir)
}

// loadTown 載入城鎮第一人稱視角的三組素材。
func loadTown(dir string) (*view.TownSet, error) {
	set := func(name string) ([]gfx.Image, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		return gfx.ParseSet(b)
	}
	walls, err := set("TOWN.16")
	if err != nil {
		return nil, err
	}
	floor, err := set("TOWNF.16")
	if err != nil {
		return nil, err
	}
	torch, err := set("TOWNT.16")
	if err != nil {
		return nil, err
	}
	// 天空是獨立素材（`SKY.16`），不是場景那一組的一部分 ——
	// 三種室內場景共用同一份。
	sky, err := set("SKY.16")
	if err != nil {
		return nil, err
	}
	return requireTownSet(view.NewTownSet(walls, floor, torch, sky))
}

// requireTownSet 是切換素材前的最後一道完整性檢查。各 loader 可能成功
// 解出「有一些影像」；那不代表能安全拿來走完整個第一人稱繪圖路徑。
// DOS／Amiga／Modern 共用 32 格牆與 36 格火炬的索引契約，MSX 則由
// Scene 產生同樣的牆槽與 27 張（9 個位置 × 3 影格）火炬。
func requireTownSet(t *view.TownSet) (*view.TownSet, error) {
	if t == nil {
		return nil, fmt.Errorf("素材組為空")
	}
	needTorch := 36
	if t.Platform == view.PlatformAmiga || t.Platform == view.PlatformMSX {
		needTorch = 27
	}
	if len(t.Walls) < 32 {
		return nil, fmt.Errorf("%s 素材組牆面只有 %d/32 張", t.Platform, len(t.Walls))
	}
	if t.Platform != view.PlatformMSX && len(t.Floor) < 1 {
		return nil, fmt.Errorf("%s 素材組缺少地板", t.Platform)
	}
	if len(t.Torch) < needTorch {
		return nil, fmt.Errorf("%s 素材組火炬只有 %d/%d 張", t.Platform, len(t.Torch), needTorch)
	}
	if len(t.Sky) < 1 || t.Sky[0] == nil {
		return nil, fmt.Errorf("%s 素材組缺少天空", t.Platform)
	}
	for i, im := range t.Walls {
		if i < 32 && im == nil {
			return nil, fmt.Errorf("%s 素材組牆面第 %d 張無法解碼", t.Platform, i)
		}
	}
	for i, im := range t.Torch {
		if i < needTorch && im == nil {
			return nil, fmt.Errorf("%s 素材組火炬第 %d 張無法解碼", t.Platform, i)
		}
	}
	return t, nil
}

// eventText 組出目前事件來源段的「原文 → 譯文」。大多數事件來源就是
// 目前地圖；特殊設施會暫時切進腳本庫，必須跟著 Pending／MessageSegment
// 查，否則原版文字會安靜退回英文。
func eventText(cat *i18n.Catalog, w *game.World) map[string]string {
	segment := w.MessageSegment
	if p := w.Pending; p != nil {
		segment = p.Segment
	}
	if segment < 0 {
		if current := w.EventSegment(); current != nil {
			segment = current.Index
		}
	}
	seg := w.EventSegment()
	if segment >= 0 {
		seg = nil
	}
	for i := range w.Events {
		if w.Events[i].Index == segment {
			seg = &w.Events[i]
			break
		}
	}
	if seg == nil {
		return nil
	}
	src := map[string]string{}
	for i, str := range seg.Strings {
		src[fmt.Sprintf("indoor.%02d.%03d", seg.Index, i)] = str
	}
	return cat.SourceMap(src, fmt.Sprintf("indoor.%02d.", seg.Index))
}

// chestMenu 是寶箱那一頁的四個選項（原版 `_2misc_e02` 的 `ds:2A36`）。
func (s *Session) chestMenu() *Menu {
	name := "箱子"
	if s.Chest != nil {
		name = s.Chest.Name()
	}
	return &Menu{
		Title: name,
		Items: []string{"打開", "找陷阱", "偵測魔法", "離開"},
	}
}

// chestDo 執行選中的動作。
func (s *Session) chestDo(act game.ChestAction, who int) bool {
	res := s.Game.Do(s.Chest, act, who)
	s.Lines = append(s.Lines, res.Lines...)
	s.closeMenu()
	if res.Done {
		s.Chest = nil
		s.Mode = ModeMessage
		return true
	}
	// 偵測魔法看完還回得去那一頁。
	return s.open(menuChest, s.chestMenu())
}

// memberMenu 是「挑一名隊員」的清單，戰鬥中的對調與開鎖共用同一個形狀。
//
// 原版的兩個索引都可以按 Esc 取消（`sub_1C1BC` 回 `0x1B`），
// 任一個取消整件事就不做 —— 這裡由選單的取消鍵對應。
func (s *Session) memberMenu(title string) *Menu {
	m := &Menu{Title: title}
	s.pickers = s.pickers[:0]
	src := s.Game.Party
	var names []string
	if f := s.Game.Fight; f != nil {
		// 戰鬥中要照**戰鬥隊形**列，不是名冊順序 —— 對調換的就是這個順序。
		for i, c := range f.Party {
			s.pickers = append(s.pickers, i)
			names = append(names, fmt.Sprintf("%d. %s", i+1, c.CombatName()))
		}
	} else {
		for i := range src {
			s.pickers = append(s.pickers, i)
			names = append(names, fmt.Sprintf("%d. %s", i+1, src[i].Name))
		}
	}
	m.Items = names
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（沒有人）")
	}
	return m
}

// exchangeWith 把先前選的那一位跟這一位對調。
func (s *Session) exchangeWith(i int) bool {
	f := s.Game.Fight
	if f == nil || i >= len(s.pickers) {
		return s.closeMenu()
	}
	a, b := s.exchFirst, s.pickers[i]
	na := ""
	if a >= 0 && a < len(f.Party) {
		na = f.Party[a].CombatName()
	}
	nb := ""
	if b < len(f.Party) {
		nb = f.Party[b].CombatName()
	}
	if !f.Exchange(a, b) {
		s.Lines = append(s.Lines, "位置沒有變。")
		return s.closeMenu()
	}
	s.Lines = append(s.Lines, fmt.Sprintf("%s 與 %s 對調了位置。", na, nb))
	return s.closeMenu()
}

// unlockMenu 是「誰來開鎖」的清單。
//
// 原版的開鎖也要挑人（跟施法一樣），先前這裡自動挑盜行最高的 ——
// 那對「想讓別人練」或「盜行最高的那個中毒了」都給不出正確答案。
// 游標預設停在盜行最高、而且還站著的那一位。
func (s *Session) unlockMenu() *Menu {
	m := &Menu{Title: "誰來開鎖？"}
	s.pickers = s.pickers[:0]
	for i := range s.Game.Party {
		c := &s.Game.Party[i]
		if !c.Condition.Acts() {
			continue
		}
		s.pickers = append(s.pickers, i)
		m.Items = append(m.Items, fmt.Sprintf("%s　盜行 %d", c.Name, c.Thievery))
		if i == s.bestThief() {
			m.Cur = len(m.Items) - 1
		}
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（沒有人站得起來）")
	}
	return m
}

// unlockBy 讓選中的那一位開鎖。
func (s *Session) unlockBy(i int) bool {
	if i >= len(s.pickers) {
		return s.closeMenu()
	}
	_, msg := s.Game.Unlock(s.pickers[i])
	s.Lines = append(s.Lines, msg)
	if t := s.Game.Trap(); t != "" {
		s.Lines = append(s.Lines, t)
	}
	s.closeMenu()
	s.Mode = ModeMessage
	return true
}

// SelectMember 換成看哪一位隊員的物品。選單開著時會重建清單。
func (s *Session) SelectMember(i int) bool {
	if i < 0 || i >= len(s.Game.Party) {
		return false
	}
	s.who = i
	if s.Mode == ModeMenu && s.menuKind == menuItems {
		s.Menu = s.itemMenu(i)
	}
	return true
}

// bestThief 是隊伍裡盜行最高、而且還站著的那一個。
func (s *Session) bestThief() int {
	best, bi := -1, 0
	for i := range s.Game.Party {
		c := &s.Game.Party[i]
		if !c.Condition.Acts() {
			continue
		}
		if v := c.Thievery; v > best {
			best, bi = v, i
		}
	}
	return bi
}

// SavePath 是遊玩狀態的存檔位置。名冊走原版格式另存一份。
const SavePath = "save/state.json"

// RosterPath 是名冊的存檔位置（原版格式，位元組完全一致往返）。
const RosterPath = "save/ROSTER.DAT"

// Save 把遊玩狀態與名冊寫出去，回傳一句播報。
//
// 分兩份是因為兩者性質不同：名冊是**原版格式**（要能被原版讀回去），
// 遊玩狀態（位置、種子、劇情旗標）是 remake 自己的 JSON ——
// 原版把那些放哪還沒解。
func (s *Session) Save() string {
	if err := os.MkdirAll(filepath.Dir(SavePath), 0o755); err != nil {
		return "存檔失敗：" + err.Error()
	}
	b, err := json.MarshalIndent(s.Game.State(), "", " ")
	if err != nil {
		return "存檔失敗：" + err.Error()
	}
	if err := os.WriteFile(SavePath, b, 0o644); err != nil {
		return "存檔失敗：" + err.Error()
	}
	if raw := s.rosterRaw; raw != nil {
		out, err := game.EncodeRoster(s.Game.Party, raw)
		if err == nil {
			os.WriteFile(RosterPath, out, 0o644)
		}
	}
	return "已存檔。"
}

// Restore 從存檔接續。沒有存檔就什麼都不做。
func (s *Session) Restore() bool {
	b, err := os.ReadFile(SavePath)
	if err != nil {
		return false
	}
	var st game.State
	if json.Unmarshal(b, &st) != nil {
		return false
	}
	if s.Game.LoadState(st) != nil {
		return false
	}
	s.Lines = nil
	s.Notice = ""
	s.PromptText = ""
	if s.cat != nil {
		s.trans = eventText(s.cat, s.Game.World)
	}
	if s.Game.World.Pending != nil {
		if msg := s.Game.World.Message; msg != "" {
			s.take([]string{msg})
		}
		s.openEventPrompt()
	}
	return true
}

// ShowIntro 切到片頭畫面，回報切成功沒有。
//
// **片頭不是 Session 的預設狀態**，要由前端明講。理由是「開起來先看片頭」
// 是可玩程式的呈現決定，不是遊戲狀態；測試與截圖工具要的是「一場進行中的
// 遊戲」，不該每一支都先按一次鍵把片頭關掉。
func (s *Session) ShowIntro() bool {
	if !s.intro.Ready() {
		return false
	}
	s.Mode = ModeIntro
	return true
}

// loadIntro 從 `MASTER.16` 取片頭要用的圖：底圖加上熱點表列到的疊圖。
//
// 熱點表在 `view.IntroLoopSpots`／`view.IntroPopSpots`，位置與圖號是拿
// DOSBox 截圖比對出來的（見那邊的說明）。任何一張缺圖就整個熱點不要，
// 不用別張頂替。
func loadIntro(blob []byte) *view.Intro {
	imgs, err := gfx.ParseSet(blob)
	if err != nil {
		return nil
	}
	in := &view.Intro{}
	for _, im := range imgs {
		if im.Width == introTitleW && im.Height == introTitleH {
			in.Title = im.Paletted(gfx.EGAPalette)
			break
		}
	}
	if in.Title == nil {
		return nil
	}
	in.Loop = loadIntroSpots(imgs, view.IntroLoopSpots)
	in.Pop = loadIntroSpots(imgs, view.IntroPopSpots)
	return in
}

// introTitleW/H 是片頭底圖的尺寸，也是在 `MASTER.16` 十五張裡認出它的判準。
const (
	introTitleW = 320
	introTitleH = 196
)

func loadIntroSpots(imgs []gfx.Image, defs []view.IntroSpotDef) []view.IntroSpot {
	var out []view.IntroSpot
	for _, d := range defs {
		sp := view.IntroSpot{X: d.X, Y: d.Y}
		for _, i := range d.Pics {
			if i < 0 || i >= len(imgs) {
				sp.Frames = nil
				break
			}
			sp.Frames = append(sp.Frames, imgs[i].Paletted(gfx.EGAPalette))
		}
		if len(sp.Frames) > 0 {
			out = append(out, sp)
		}
	}
	return out
}
