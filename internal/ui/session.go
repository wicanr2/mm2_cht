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
	KeyRest // 在旅店休息並受訓
)

// Mode 是目前的互動模式。原版也是這樣分的：走路時吃方向鍵，
// 訊息或提問掛著時吃的是別的鍵。
type Mode int

const (
	ModeExplore Mode = iota
	ModeMessage      // 有訊息待確認
	ModeAsk          // 事件在等 Y／N
	ModeCombat       // 戰鬥中
	ModeDead         // 全隊倒下
)

func (m Mode) String() string {
	switch m {
	case ModeExplore:
		return "探索"
	case ModeMessage:
		return "訊息"
	case ModeAsk:
		return "提問"
	case ModeCombat:
		return "戰鬥"
	case ModeDead:
		return "全滅"
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
	// Answer 在 ModeAsk 時由 KeyYes／KeyNo 填，交給事件腳本。
	Answer bool

	trans map[string]string
	cat   *i18n.Catalog
	scr   *render.Screen
}

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
	case ModeAsk:
		switch k {
		case KeyYes:
			s.Answer = true
		case KeyNo:
			s.Answer = false
		default:
			return false
		}
		s.Mode = ModeExplore
		return s.resume()
	case ModeCombat:
		if k != KeyConfirm {
			return false
		}
		return s.fightRound()
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
	}
	return false
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

// resume 回答完提問之後把腳本跑完。
func (s *Session) resume() bool {
	s.take(s.Game.Log)
	if len(s.Lines) > 0 {
		s.Mode = ModeMessage
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
func (s *Session) Message() string {
	if len(s.Lines) == 0 {
		return ""
	}
	return s.Lines[0]
}

// Draw 把目前狀態畫進畫面並回傳。
func (s *Session) Draw() *render.Screen {
	view.Draw(s.scr, s.Game.World, s.Assets, s.Message())
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
