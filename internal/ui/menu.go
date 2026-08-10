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
}

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
	return true
}

// Lines 是要畫出來的行，第一行是標題。
func (m *Menu) Lines() []string {
	out := make([]string, 0, len(m.Items)+1)
	if m.Title != "" {
		out = append(out, m.Title)
	}
	for i, it := range m.Items {
		mark := "  "
		if i == m.Cur {
			mark = "▶ "
		}
		out = append(out, fmt.Sprintf("%s%d) %s", mark, i+1, it))
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
	ids, _ := game.ShopGoods(group, town)
	m := &Menu{Title: fmt.Sprintf("商店（第 %d 類）", group+1)}
	s.goods = s.goods[:0]
	buyer := &s.Game.Party[0]
	for _, id := range ids {
		if id == 0 {
			continue
		}
		s.goods = append(s.goods, id)
		price := game.BuyPrice(s.Game.Items, id, buyer)
		m.Items = append(m.Items, fmt.Sprintf("%-14s %d 金", s.itemName(id), price))
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
	level := (n-1)/8 + 1
	list := game.SpellsOf(school, level)
	i := (n - 1) % 8
	if i >= len(list) {
		return game.Spell{}, false
	}
	return list[i], true
}
