package ui

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 旅店（`1RETINN.OVL`）是名冊與隊伍的編組畫面。
//
// 原版的畫面同時列出「角色」與「傭兵」兩張清單，用 `A`–`X` 檢視、
// `Ctrl` + 字母加入或移出。這裡拆成幾個小選單，語意相同 ——
// 一次做一件事，不必記快捷鍵。

// innMenu 是旅店的主選單。
func (s *Session) innMenu() *Menu {
	return &Menu{
		Title: fmt.Sprintf("旅店（隊伍 %d／%d　名冊 %d）",
			len(s.Game.Party), game.MaxParty, len(s.Game.Roster)),
		Items: []string{"休息並受訓", "編入隊伍", "移出隊伍", "存檔", "離開"},
	}
}

// innChoose 接住旅店主選單的選擇。
func (s *Session) innChoose(i int) bool {
	switch i {
	case 0:
		s.Lines = append(s.Lines, s.Game.RestAtInn()...)
		s.Lines = append(s.Lines, s.Game.TrainParty()...)
	case 1:
		return s.open(menuInnAdd, s.rosterMenu())
	case 2:
		return s.open(menuInnDrop, s.partyMenu())
	case 3:
		s.Lines = append(s.Lines, s.Save())
	default:
		s.Lines = append(s.Lines, "隊伍離開旅店。")
	}
	s.closeMenu()
	s.Mode = ModeMessage
	return true
}

// rosterMenu 列出名冊裡還沒編進隊伍的人。
func (s *Session) rosterMenu() *Menu {
	m := &Menu{Title: "編入誰？"}
	s.pickers = s.pickers[:0]
	inParty := map[string]bool{}
	for _, p := range s.Game.Party {
		inParty[p.Name] = true
	}
	for i, c := range s.Game.Roster {
		if inParty[c.Name] {
			continue
		}
		s.pickers = append(s.pickers, i)
		m.Items = append(m.Items, fmt.Sprintf("%s　%v %v　等級 %d",
			c.Name, c.Race, c.Class, c.Level))
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（名冊裡沒有人可以編）")
	}
	return m
}

// partyMenu 列出目前的隊伍。
func (s *Session) partyMenu() *Menu {
	m := &Menu{Title: "移出誰？"}
	s.pickers = s.pickers[:0]
	for i := range s.Game.Party {
		c := &s.Game.Party[i]
		s.pickers = append(s.pickers, i)
		m.Items = append(m.Items, fmt.Sprintf("%s　%v　生命 %d/%d",
			c.Name, c.Class, c.HP, c.MaxHP))
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（隊伍是空的）")
	}
	return m
}

// smithChoose 接住鐵匠鋪主選單。
//
// 三項服務都對**背包**那六格動作（原版畫面上的 `A`–`F`）。
func (s *Session) smithChoose(i int) bool {
	switch i {
	case 0:
		town := s.Game.World.MapIndex
		return s.open(menuShop, s.shopMenu(0, town))
	case 1:
		return s.open(menuSmithSell, s.backpackMenu("賣掉哪一件？"))
	case 2:
		return s.open(menuSmithIdent, s.backpackMenu("鑑定哪一件？"))
	}
	s.closeMenu()
	s.Lines = append(s.Lines, "隊伍離開鐵匠鋪。")
	s.Mode = ModeMessage
	return true
}

// backpackMenu 列出目前這個人的背包六格。
func (s *Session) backpackMenu(title string) *Menu {
	m := &Menu{Title: title}
	s.pickers = s.pickers[:0]
	if s.who < 0 || s.who >= len(s.Game.Party) {
		s.who = 0
	}
	c := &s.Game.Party[s.who]
	for i, it := range c.Backpack() {
		if it.Empty() {
			continue
		}
		s.pickers = append(s.pickers, i)
		m.Items = append(m.Items, fmt.Sprintf("%c %s", 'A'+i, s.itemName(it.ID)))
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（背包是空的）")
	}
	return m
}

// guildMenu 列出這座城的法師公會賣的四條法術。
//
// 貨色與價格是每座城固定的（`ds:46DA`／`ds:46EE`），不是隨機的。
func (s *Session) guildMenu() *Menu {
	town := s.Game.World.MapIndex
	stock := game.GuildStockOf(town)
	who := "沒有人"
	if s.who >= 0 && s.who < len(s.Game.Party) {
		who = s.Game.Party[s.who].Name
	}
	m := &Menu{Title: "法師公會（買家：" + who + "）"}
	s.pickers = s.pickers[:0]
	for i, it := range stock {
		name := fmt.Sprintf("第 %d 條", it.Spell+1)
		if sp, ok := game.SpellByEngineIndex(48 + it.Spell); ok {
			name = fmt.Sprintf("%s（%d 級）", sp.Name, sp.Level)
		}
		s.pickers = append(s.pickers, i)
		m.Items = append(m.Items, fmt.Sprintf("%s%5d金", padCols(name, 22), it.Price))
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（這裡沒有公會）")
	}
	return m
}

// tavern 是酒館的兩項服務。
//
// 原版的酒館（`2BRAIN.OVL`）辦競技賽，要入場券才能參加
// （`Sorry, but you must have a ticket to compete in these games.`）；
// 打聽消息那一段的內容還沒解，先給一句中性的回應。
func (s *Session) tavern(i int) []string {
	switch i {
	case 0:
		return []string{"隊伍點了一輪。氣氛熱絡了一些。"}
	case 1:
		return []string{"酒保搖搖頭。（競技賽要入場券；打聽消息那一段還沒解出來）"}
	}
	return []string{"隊伍離開酒館。"}
}
