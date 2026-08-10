// Package ui 是遊玩的互動層：載入、按鍵、模式切換、畫面組成。
//
// **與視窗系統無關。** Ebiten 主程式（`cmd/mm2`）與 headless 的
// 逐格截圖（測試）跑的是同一份程式碼 —— 沒有 GPU 的環境也驗得了
// 「按這個鍵之後畫面變成什麼」，而不只是「編得過」。
package ui

import (
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
	spells    []int
	spellInfo []game.Spell
	refRows   [][]string
	goods     []int

	trans map[string]string
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
		gs.UseItems(tbl)
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

	s := &Session{Game: gs, Assets: a, scr: view.NewScreen()}
	// 事件腳本問 Y／N 時回答目前設定的值。
	w.Answer = func() bool { return s.Answer }
	s.Ref = LoadReference(gamedata.Dir())
	if cat != nil {
		s.cat = cat
		s.Names(cat, defs)
		s.trans = eventText(cat, w)
	}
	return s, nil
}

// Names 把怪物名的譯文接上。
func (s *Session) Names(cat *i18n.Catalog, defs []monsters.Monster) {
	m := make(map[string]string, len(defs))
	for _, d := range defs {
		if t := cat.Or("monster."+d.Name, ""); t != "" {
			m[d.Name] = t
		}
	}
	s.Game.Names = m
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
		if k != KeyConfirm {
			return false
		}
		return s.fightRound()
	case ModeMenu:
		return s.menuKey(k)
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
		s.who = 0
		return s.open(menuItems, s.itemMenu(0))
	case KeyRef:
		return s.open(menuRef, s.refMenu())
	case KeyShop:
		// 商店的類別與城號由目前所在的地圖決定：前五張圖是五座城。
		town := s.Game.World.MapIndex
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

// closeMenu 關掉選單回到探索（或把剩下的訊息顯示完）。
func (s *Session) closeMenu() bool {
	s.Menu = nil
	s.menuKind = menuNone
	if len(s.Lines) > 0 {
		s.Mode = ModeMessage
	} else {
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
	}
	return false
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
	}
	return s.closeMenu()
}

// toggleEquip 把背包那一格的東西裝起來，或把裝備那一格卸下來。
//
// 前六項是已裝備、後六項是背包（`Character.Equipped` 與 `Backpack`）。
// 原版的裝備限制（職業能不能用、部位衝突）還沒逐條反組譯，所以這裡
// 只做「換格子」與**重算戰鬥數值**（`RecomputeGear`，那條規則是抄來的）。
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
		if c.Items[i].Empty() {
			s.Lines = append(s.Lines, "那一格是空的。")
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
	} else if len(s.Lines) > 0 {
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
	s.Game.Fight = nil
	if !s.Game.Alive() {
		s.Mode = ModeDead
		return true
	}
	s.Mode = ModeMessage
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
	var menu []string
	if s.Mode == ModeMenu && s.Menu != nil {
		menu = s.Menu.Lines()
	}
	view.DrawWith(s.scr, s.Game.World, s.Assets, s.Message(), menu)
	return s.scr
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
