package ui

import (
	"fmt"
	"strings"

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
		return s.enterArena()
	}
	return []string{"隊伍離開酒館。"}
}

// enterArena 報名競技賽：收券、開戰。
//
// 打贏之後才發獎，所以要記住階層等戰鬥結束（`arenaTier`，-1 表示
// 這一場不是競技賽）。原版是同一支函式一路做完，remake 的戰鬥是
// 逐回合推進的，中間會回到主迴圈，所以拆成兩半。
func (s *Session) enterArena() []string {
	e := s.Game.EnterArena()
	if !e.Ready {
		return e.Lines
	}
	enc := s.Game.ArenaEncounter(e.Tier)
	if enc == nil {
		return append(e.Lines, "今天沒有對手。")
	}
	s.Game.Fight = enc
	s.arenaTier = e.Tier
	s.Mode = ModeCombat
	return e.Lines
}

// detoxMenu 是大腦淨化的挑人清單。
//
// 原版先問「付 100 金嗎（y/n）」再問挑誰（`_2brain_e01`）。這裡把兩步
// 合成一個清單：挑誰就是同意付款，選「離開」等於答 N。
// 收費 100 金**原版只驗不扣**，所以清單上把每個人的黃金一起列出來。
func (s *Session) detoxMenu() *Menu {
	m := &Menu{Title: "大腦淨化（清掉次要技能，需要 100 金）"}
	s.pickers = s.pickers[:0]
	for i := range s.Game.Party {
		c := &s.Game.Party[i]
		s.pickers = append(s.pickers, i)
		skills := "沒有技能"
		if names := skillNames(s, c); names != "" {
			skills = names
		}
		m.Items = append(m.Items, fmt.Sprintf("%s　%s　%d金", c.Name, skills, c.Gold))
	}
	m.Items = append(m.Items, "離開")
	return m
}

// skillNames 把兩項第二技能換成名字。名字來自遊戲內說明書
// （`data/reference.json` 的 skills，索引 1–15）。
func skillNames(s *Session, c *game.Character) string {
	var out []string
	for _, sk := range c.Skills {
		if sk <= 0 {
			continue
		}
		name := fmt.Sprintf("技能 %d", sk)
		if s.Ref != nil {
			if n := s.Ref.SkillName(sk); n != "" {
				name = n
			}
		}
		out = append(out, name)
	}
	return strings.Join(out, "、")
}

// templeMenu 列出這座城的神殿賣的三條牧師系法術。
//
// 貨色與價格是每座城固定的（`ds:46B2`／`ds:46C6`），與法師公會同一套結構。
func (s *Session) templeMenu() *Menu {
	town := s.Game.World.MapIndex
	stock := game.TempleStockOf(town)
	who := "沒有人"
	if s.who >= 0 && s.who < len(s.Game.Party) {
		who = s.Game.Party[s.who].Name
	}
	m := &Menu{Title: "神殿法術（買家：" + who + "）"}
	s.pickers = s.pickers[:0]
	for i, it := range stock {
		name := fmt.Sprintf("第 %d 條", it.Spell+1)
		if sp, ok := game.SpellByEngineIndex(it.Spell); ok {
			name = fmt.Sprintf("%s（%d 級）", sp.Name, sp.Level)
		}
		s.pickers = append(s.pickers, i)
		m.Items = append(m.Items, fmt.Sprintf("%s%5d金", padCols(name, 11), it.Price))
	}
	if len(m.Items) == 0 {
		m.Items = append(m.Items, "（這裡沒有神殿）")
	}
	return m
}

// templeServiceMenu 是神殿的主選單，價錢一起列出來。
//
// 價錢是每個人各算一份（`sub_1C6CC` 進來時算的是「目前這一位」的），
// 清單上列的是全隊的總額 —— remake 的服務是對全隊做的，列單價會誤導。
func (s *Session) templeServiceMenu() *Menu {
	m := &Menu{Title: "神殿"}
	for i, name := range game.TempleServiceNames {
		k := game.TempleService(i)
		if k == game.TempleLeave {
			m.Items = append(m.Items, name)
			continue
		}
		total := 0
		if k == game.TempleDonate {
			total = s.Game.TemplePrice(k, 0)
		} else {
			for who := range s.Game.Party {
				total += s.Game.TemplePrice(k, who)
			}
		}
		if total == 0 {
			m.Items = append(m.Items, name+"（不需要）")
			continue
		}
		m.Items = append(m.Items, fmt.Sprintf("%s%6d金", padCols(name, 12), total))
	}
	m.Items = append(m.Items, "買法術")
	return m
}
