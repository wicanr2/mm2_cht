package ui

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// Menu 是一份可以用方向鍵挑的清單。
//
// 原版的選單是按字母鍵直選（`A - Might` 那種），這裡同時支援兩種：
// 上下移動游標加確認，或直接按項次對應的數字。行為一致，只是輸入方式。
type Menu struct {
	Title string
	Items []string
	Cur   int
	// top 是目前顯示的第一項。清單比框高時要捲，否則游標移到第 11 項
	// 之後就看不見了 —— 而畫面不會有任何異常，只是「選不到後面」。
	top int
}

// VisibleRows 是選單一次顯示幾列（扣掉標題那一列）。
// 框高 FPH 除以行高 10，再留一列給標題。
const VisibleRows = 11

// Move 移動游標，會夾在範圍內（不繞回，與原版一致）。
func (m *Menu) Move(d int) bool {
	if len(m.Items) == 0 {
		return false
	}
	n := m.Cur + d
	if n < 0 {
		n = 0
	}
	if n >= len(m.Items) {
		n = len(m.Items) - 1
	}
	if n == m.Cur {
		return false
	}
	m.Cur = n
	m.scroll()
	return true
}

// scroll 把顯示範圍拉到游標所在處。
func (m *Menu) scroll() {
	if m.Cur < m.top {
		m.top = m.Cur
	}
	if m.Cur >= m.top+VisibleRows {
		m.top = m.Cur - VisibleRows + 1
	}
	if m.top < 0 {
		m.top = 0
	}
}

// Lines 是要畫出來的行，第一行是標題。清單比框高時只回傳看得見的那一段，
// 標題後面掛「第 N／M 項」讓玩家知道還有東西在下面。
func (m *Menu) Lines() []string {
	m.scroll()
	title := m.Title
	if len(m.Items) > VisibleRows {
		title = fmt.Sprintf("%s（%d／%d）", title, m.Cur+1, len(m.Items))
	}
	out := make([]string, 0, VisibleRows+1)
	if title != "" {
		out = append(out, title)
	}
	for i := m.top; i < len(m.Items) && i < m.top+VisibleRows; i++ {
		mark := "  "
		if i == m.Cur {
			mark = "▶ "
		}
		out = append(out, fmt.Sprintf("%s%d) %s", mark, i+1, m.Items[i]))
	}
	return out
}

// castMenu 組出「誰來施法」的清單：只列得出施法職業且還站著的人。
func (s *Session) castMenu() *Menu {
	m := &Menu{Title: "由誰施法？"}
	s.casters = s.casters[:0]
	for i := range s.Game.Party {
		c := &s.Game.Party[i]
		if !game.CanCast(c.Class) || !c.Condition.Acts() {
			continue
		}
		s.casters = append(s.casters, i)
		m.Items = append(m.Items, fmt.Sprintf("%s  SP %d/%d",
			c.Name, c.SP, c.MaxSP))
	}
	return m
}

// spellMenu 組出某個人會的法術清單。
func (s *Session) spellMenu(who int) *Menu {
	c := &s.Game.Party[who]
	school := game.SpellSchoolOf(c.Class)
	m := &Menu{Title: fmt.Sprintf("%s 要施什麼法術？", c.Name)}
	s.spells, s.spellInfo = s.spells[:0], s.spellInfo[:0]
	for n := 1; n <= 48; n++ {
		if !c.Knows(n) {
			continue
		}
		sp, ok := spellByNumber(school, n)
		if !ok {
			continue
		}
		s.spells = append(s.spells, n)
		s.spellInfo = append(s.spellInfo, sp)
		m.Items = append(m.Items, fmt.Sprintf("%s（%d 級）", sp.Name, sp.Level))
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（一個法術都還不會）")
	}
	return m
}

// isSpellPrompt 回報目前是否停在施法的輸入子選單。
func (s *Session) isSpellPrompt() bool {
	switch s.menuKind {
	case menuSpellMember, menuSpellItem, menuSpellChoice, menuSpellColumn:
		return true
	default:
		return false
	}
}

// openSpellPrompt 在扣費前開啟已知的輸入子選單。
func (s *Session) openSpellPrompt(i int) bool {
	if i < 0 || i >= len(s.spells) {
		return s.closeMenu()
	}
	c := &s.Game.Party[s.who]
	idx := game.SpellIndex(game.SpellSchoolOf(c.Class), s.spells[i])
	s.spellPrompt = game.SpellPromptFor(idx)
	s.spellPromptSpell = s.spells[i]
	// 每次施法都先清掉上一個提示的暫存答案；只有子選單確認後才寫入。
	s.Game.Target, s.Game.Item, s.Game.Choice, s.Game.Column = -1, -1, 0, 0
	switch s.spellPrompt.Kind {
	case game.SpellPromptNone:
		return s.finishSpellPrompt()
	case game.SpellPromptMember:
		return s.open(menuSpellMember, s.spellMemberMenu())
	case game.SpellPromptItem:
		return s.open(menuSpellItem, s.spellItemMenu())
	case game.SpellPromptChoice:
		return s.open(menuSpellChoice, s.spellChoiceMenu("請選擇數字"))
	case game.SpellPromptFlight:
		return s.open(menuSpellColumn, s.spellColumnMenu())
	default:
		return s.cancelSpellPrompt()
	}
}

// finishSpellPrompt 是唯一從正常 UI 呼叫 Cast 的入口；提示未確認前不會扣費。
func (s *Session) finishSpellPrompt() bool {
	res := s.Game.Cast(s.who, s.spellPromptSpell)
	s.Lines = append(s.Lines, res.String())
	s.spellPrompt = game.SpellPrompt{}
	s.spellPromptSpell = 0
	s.Game.Target, s.Game.Item, s.Game.Choice, s.Game.Column = -1, -1, 0, 0
	ok := s.closeMenu()
	// 定位術把整張圖畫出來（原版 `_2play_e14`）—— remake 已經有地圖畫面，
	// 就切過去，不要只丟一句話給玩家自己按鍵。
	if res.ShowMap {
		s.Mode, s.hintPage = ModeMap, 0
	}
	return ok
}

// cancelSpellPrompt 回到法術清單；這保證 Esc 不會付出任何代價。
func (s *Session) cancelSpellPrompt() bool {
	s.spellPrompt = game.SpellPrompt{}
	s.spellPromptSpell = 0
	s.Game.Target, s.Game.Item, s.Game.Choice, s.Game.Column = -1, -1, 0, 0
	return s.open(menuSpell, s.spellMenu(s.who))
}

func (s *Session) spellMemberMenu() *Menu {
	m := &Menu{Title: "請選一名隊員（Esc 取消）"}
	s.pickers = s.pickers[:0]
	for i := range s.Game.Party {
		s.pickers = append(s.pickers, i)
		m.Items = append(m.Items, fmt.Sprintf("%d. %s", i+1, s.Game.Party[i].Name))
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（沒有隊員；按 Esc 取消）")
	}
	return m
}

func (s *Session) spellItemMenu() *Menu {
	m := &Menu{Title: "請選施法者的背包物品（Esc 取消）"}
	s.pickers = s.pickers[:0]
	c := &s.Game.Party[s.who]
	for slot, it := range c.Backpack() {
		if it.Empty() {
			continue
		}
		s.pickers = append(s.pickers, slot)
		m.Items = append(m.Items, fmt.Sprintf("%d. 背包%d %s", len(m.Items)+1, slot+1, s.itemName(it.ID)))
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（背包沒有可用物品；按 Esc 取消）")
	}
	return m
}

func (s *Session) spellChoiceMenu(title string) *Menu {
	m := &Menu{Title: title + "（Esc 取消）"}
	for n := s.spellPrompt.Min; n <= s.spellPrompt.Max; n++ {
		m.Items = append(m.Items, fmt.Sprintf("%d", n))
	}
	return m
}

func (s *Session) spellColumnMenu() *Menu {
	m := &Menu{Title: "請選飛行欄（Esc 取消）"}
	for i := 0; i < s.spellPrompt.Columns; i++ {
		m.Items = append(m.Items, fmt.Sprintf("%c", 'A'+rune(i)))
	}
	return m
}

// itemMenu 列出一個人身上的東西：已裝備六格加背包六格。
func (s *Session) itemMenu(who int) *Menu {
	c := &s.Game.Party[who]
	m := &Menu{Title: fmt.Sprintf("%s 的物品", c.Name)}
	add := func(label string, slots []game.ItemSlot) {
		for i, it := range slots {
			name := "（空）"
			if !it.Empty() {
				name = s.itemName(it.ID)
			}
			m.Items = append(m.Items, fmt.Sprintf("%s%d %s", label, i+1, name))
		}
	}
	add("裝備", c.Equipped())
	add("背包", c.Backpack())
	return m
}

// itemName 查物品名，查不到就回編號。
func (s *Session) itemName(id int) string {
	tbl := s.Game.Items
	if id < 0 || id >= len(tbl) {
		return fmt.Sprintf("物品 %d", id)
	}
	return tbl[id].Name
}

// shopMenu 列出這座城這一類商店賣的東西與價格。
func (s *Session) shopMenu(group, town int) *Menu {
	ids := s.Game.ShopShelf(group, town)
	m := &Menu{Title: fmt.Sprintf("商店（第 %d 類）", group+1)}
	s.goods = s.goods[:0]
	buyer := &s.Game.Party[0]
	for _, id := range ids {
		if id == 0 {
			continue
		}
		s.goods = append(s.goods, id)
		price := game.BuyPrice(s.Game.Items, id, buyer)
		// 補位要數字數不是位元組數（`%-14s` 補的是位元組，中文一個字三個），
		// 不然價格那一欄會隨名字長短左右跳。
		m.Items = append(m.Items, fmt.Sprintf("%s%5d金",
			padCols(s.itemName(id), 16), price))
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（今天沒有貨）")
	}
	return m
}

// spellByNumber 找某一系第 n 號法術。原版的編號是 1–48，每級八條。
func spellByNumber(school game.SpellSchool, n int) (game.Spell, bool) {
	if n < 1 || n > 48 {
		return game.Spell{}, false
	}
	// n 是該系的連續序號；資料的 Level/Index 是手冊分組，並非每級
	// 固定八條。直接走 engine index 才能讓顯示法術與 Cast handler 同一條。
	idx := game.SpellIndex(school, n)
	all := game.Spells()
	if idx < 0 || idx >= len(all) || all[idx].School != school {
		return game.Spell{}, false
	}
	return all[idx], true
}
