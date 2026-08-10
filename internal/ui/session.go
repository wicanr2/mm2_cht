// Package ui 是遊玩的互動層：載入、按鍵、模式切換、畫面組成。
//
// **與視窗系統無關。** Ebiten 主程式（`cmd/mm2`）與 headless 的
// 逐格截圖（測試）跑的是同一份程式碼 —— 沒有 GPU 的環境也驗得了
// 「按這個鍵之後畫面變成什麼」，而不只是「編得過」。
package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/cjk"
	"github.com/wicanr2/mm2_cht/internal/assets/font"
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/assets/items"
	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
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
	KeyRest  // 在旅店休息並受訓
	KeyCast  // 開施法選單
	KeyItems // 看物品欄
	KeyShop  // 開商店
	KeyRef   // 查說明書（第二技能、指令一覽）
	KeyBash  // 撞門
	KeyUnlock // 開鎖
	KeySave  // 存檔
	KeyRun   // 戰鬥中溜跑
	KeyBlock // 戰鬥中抵擋
	KeyShoot // 戰鬥中射擊
	KeyUse   // 使用物品欄裡的東西
	KeyMap   // 開地圖畫面
	KeyChest  // 開寶箱那一頁
	KeyCreate // 建立新角色
	KeyExch  // 戰鬥中對調兩名隊員的位置
	KeyProt  // 戰鬥中顯示防護效能
	KeyView  // 戰鬥中檢視某位隊員
	KeyUp    // 選單游標上移
	KeyDown  // 選單游標下移
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
	case ModeCreate:
		return "建角"
	case ModeName:
		return "命名"
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
	// Answer 是「下一個事件提問要回答什麼」。
	//
	// **引擎的提問是同步回呼**（`World.Answer`）：腳本一次跑到底，
	// 沒有中途停下來等玩家的機制。所以流程是**先設定答案再踩上去**，
	// 而不是踩上去之後跳出提示。探索模式按 Y／N 設定它。
	//
	// 要做成真正的中途提問，得讓 `World.run` 能在 `OpAsk` 停住並保存
	// 續跑點 —— 那是引擎的改動，不是這一層能繞過去的。
	Answer bool

	// New 是正在建立的角色。名冊在 `Game.Roster`。
	New game.NewCharacter

	// Chest 是眼前的箱子。放在這裡而不是 game.Session 上，是因為
	// **原版什麼時候擺出箱子那一頁還沒解**（`_2misc_e02` 在 `ds:0434`
	// 為 0 時走選單，誰把它設成 0 並擺好內容那一段還沒追）。
	// 腳本擺好的獎賞走的是另一條路，由 Session.ClaimReward 當場領走。
	Chest *game.Chest

	// Ref 是說明書的參考資料，沒有就是 nil。
	Ref *Reference
	// Menu 是選單模式下開著的那一份，沒開時是 nil。
	Menu *Menu
	// menuKind 決定確認之後要做什麼。
	menuKind menuKind
	// who 是選單正在處理的隊員（施法者、看物品的人、買東西的人）。
	who int

	// 選單項次 → 引擎編號的對照。每次重建選單時覆寫。
	casters   []int
	// pickers 是「挑一名隊員」那類選單的索引對照（開鎖用）。
	pickers   []int
	// phase 是火炬動畫的相位，由 Tick 前進。
	phase     int
	// monBlob 是 MONSTERS.16 的原始內容，monIndex 是它的圖號索引表。
	// 怪物圖很大（168 KB），只在要畫的時候才解一張。
	monBlob  []byte
	monIndex []int
	monCache map[int]gfx.MonsterPic
	// townNames 是有地名的地圖，索引即地圖編號。
	townNames []string
	// exchFirst 是對調指令選的第一位（戰鬥隊形裡的位置）。
	exchFirst int
	// chestAct 是寶箱那一頁選了哪個動作，等挑完人再執行。
	chestAct game.ChestAction
	// attrCur 是建角畫面上的游標，attrPick 是已挑起等著對調的那一項（-1 = 沒有）。
	attrCur, attrPick int
	spells    []int
	spellInfo []game.Spell
	refRows   [][]string
	goods     []int

	rosterRaw []byte
	trans     map[string]string
	cat   *i18n.Catalog
	scr   *render.Screen
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
)

// Load 從原版資料目錄開一場遊玩。
//
// 缺原版資料就回錯誤 —— 這一層不做「找不到就用假資料頂著」，
// 那會讓畫面看起來對、其實在跑別的東西。
func Load(dataDir string) (*Session, error) {
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
	if b, err := os.ReadFile(filepath.Join("assets", "font", "cjk24.bin")); err == nil {
		if cf, err := cjk.Parse(b); err == nil {
			a.CJK = cf
		}
	}
	if t, err := loadTown(dataDir); err == nil {
		a.Town = t
	}

	s := &Session{Game: gs, Assets: a, scr: view.NewScreen(), townNames: townNamesCHT,
		monCache: map[int]gfx.MonsterPic{}, attrPick: -1}
	// 怪物圖：載不到就不畫，不必讓整場遊玩失敗。
	if b, err := os.ReadFile(filepath.Join(dataDir, "MONSTERS.16")); err == nil {
		if idx, err := gfx.MonsterIndex(b); err == nil {
			s.monBlob, s.monIndex = b, idx
		}
	}
	// 事件腳本問 Y／N 時回答目前設定的值。
	w.Answer = func() bool { return s.Answer }
	s.Ref = LoadReference(gamedata.Dir())
	s.rosterRaw = must("DEFAULT.DAT")
	if cat != nil {
		s.cat = cat
		s.Names(cat, defs)
		s.trans = eventText(cat, w)
	}
	return s, nil
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
		if k != KeyConfirm {
			return false
		}
		return s.advance()
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
		// 任意鍵離開 —— 這是一頁靜態畫面，沒有可操作的東西。
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
	case KeyYes, KeyNo:
		s.Answer = k == KeyYes
		word := "否"
		if s.Answer {
			word = "是"
		}
		s.Lines = append(s.Lines, "下一個提問將回答："+word)
		s.Mode = ModeMessage
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
		s.Mode = ModeMap
		return true
	case KeyCreate:
		s.New = game.RollNewCharacter(s.Game.Rand)
		s.Mode = ModeCreate
		return true
	case KeyChest:
		if s.Chest == nil {
			s.Lines = append(s.Lines, "這裡沒有箱子。")
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
	if sp, ok := game.SpellByEngineIndex(res.Spell); ok {
		line += "，發動" + sp.Name
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
		res := s.Game.Cast(s.who, s.spells[i])
		s.Lines = append(s.Lines, res.String())
		return s.closeMenu()
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
		if i < 0 || i >= len(game.TempleServiceNames) {
			return s.closeMenu()
		}
		s.Lines = append(s.Lines, s.Game.Serve(game.TempleService(i))...)
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
	}
	return s.closeMenu()
}

// toggleEquip 把背包那一格的東西裝起來，或把裝備那一格卸下來。
//
// 前六項是已裝備、後六項是背包（`Character.Equipped` 與 `Backpack`）。
// 裝備前先過 `CanEquip`：職業禁用、陣營不符、特殊能力 0xF0 都會擋下來
// （規則抄自 `2CMDS.img` 的裝備指令，見 `internal/game/equip.go`）。
// **部位衝突還沒解** —— 原版另外呼叫 `sub_1C8AA` 檢查同一部位是不是
// 已經有東西，那一支還沒讀。
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
	before := s.Game.World.MapIndex
	_, enc := s.Game.Step(n)
	if s.cat != nil && s.Game.World.MapIndex != before {
		// 譯文對照表是逐段的，換圖就要重建。
		s.trans = eventText(s.cat, s.Game.World)
	}
	s.take(s.Game.Log)
	if enc != nil {
		s.Game.Fight = enc
		s.Mode = ModeCombat
		s.Lines = append(s.Lines, s.encounterLine(enc))
		return true
	}
	// 有選單的設施，踩進去就開。
	switch s.Game.Facility {
	case game.FacilityTemple:
		return s.open(menuTemple, listMenu("神殿", game.TempleServiceNames[:]))
	case game.FacilityInn:
		return s.open(menuInn, s.innMenu())
	case game.FacilityMageGuild:
		return s.open(menuGuild, s.guildMenu())
	case game.FacilityTraining:
		return s.open(menuTrain, listMenu("訓練基地", []string{"受訓", "離開"}))
	case game.FacilityTavern:
		return s.open(menuTavern, listMenu("酒館", []string{"買一輪", "打聽消息", "離開"}))
	case game.FacilityBlacksmith:
		return s.open(menuSmith, listMenu("鐵匠鋪",
			[]string{"購買", "出售", "鑑定", "離開"}))
	}
	if len(s.Lines) > 0 {
		s.Mode = ModeMessage
	}
	return true
}

// take 收下引擎這一步產生的訊息，順便換成譯文。
func (s *Session) take(log []string) {
	for _, l := range log {
		for _, part := range strings.Split(l, "\n") {
			if part == "" {
				continue
			}
			if t, ok := s.trans[part]; ok {
				part = t
			}
			s.Lines = append(s.Lines, part)
		}
	}
	s.Game.Log = nil
}

// advance 推掉一條訊息；推完就回探索。
func (s *Session) advance() bool {
	if len(s.Lines) > 0 {
		s.Lines = s.Lines[1:]
	}
	if len(s.Lines) == 0 {
		s.Mode = ModeExplore
	}
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
	if !enc.Over() {
		return true
	}
	if enc.PartyWon() {
		exp := enc.AwardExp(s.Game.Party)
		s.Lines = append(s.Lines, fmt.Sprintf("隊伍獲勝，每人獲得 %d 點經驗", exp))
	} else {
		s.Lines = append(s.Lines, "隊伍全滅")
	}
	s.Game.EndCombat()
	if !s.Game.Alive() {
		s.Mode = ModeDead
		return true
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
	if len(s.Lines) == 0 {
		return ""
	}
	return s.Lines[0]
}

// Draw 把目前狀態畫進畫面並回傳。
//
// 選單開著時蓋在視圖上 —— 原版也是把選單畫在同一塊區域，
// 不另開視窗。
func (s *Session) Draw() *render.Screen {
	if s.Mode == ModeMap {
		view.DrawMap(s.scr, s.Game.World, s.Assets, view.MapInfo{Title: s.mapTitle()})
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
	}
	a := s.Assets
	a.Place = s.mapTitle()
	// 戰鬥中把怪物疊上去 —— 沒有這一步，打起來畫面上一隻怪都看不到。
	if menu == nil && s.Mode == ModeCombat {
		a.Monsters = s.sprites()
	}
	view.DrawPhase(s.scr, s.Game.World, a, s.Message(), menu, s.phase)
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
	if f == nil || s.monBlob == nil {
		return nil
	}
	var out []view.MonsterSprite
	for _, c := range f.Monsters {
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
		out = append(out, view.MonsterSprite{Pic: pic, Anim: -1})
	}
	return out
}

// Tick 前進一格火炬動畫，回報畫面要不要重畫。
//
// 動畫與遊戲邏輯分開：邏輯是「按一次鍵走一步」，火炬是連續的。
// 呼叫端（Ebiten 的 Update）自己決定多久叫一次。
func (s *Session) Tick() bool {
	s.phase = (s.phase + 1) % view.TorchFrames
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
	return view.NewTownSet(walls, floor, torch), nil
}

// eventText 組出目前這張地圖的「事件原文 → 譯文」。
//
// 換地圖時要重建 —— 對照表是逐段的，拿別段的表去查只會查不到。
func eventText(cat *i18n.Catalog, w *game.World) map[string]string {
	seg := w.EventSegment()
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
	if s.cat != nil {
		s.trans = eventText(s.cat, s.Game.World)
	}
	return true
}
