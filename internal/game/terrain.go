package game

import "github.com/wicanr2/mm2_cht/internal/gamedata"

// 野外的通行判定。
//
// 室內看牆位元（見 wall.go），**室外走完全不同的一套**：地形碼查表分成
// 五個類別，山區與森林要隊伍裡有兩名對應的第二技能才過得去。
// 原版在 `2PLAY.img` 的 `sub_5E68` 依 `ds:039D` 分這兩條路。
//
// 室外那條的流程（`sub_5E68` 0x5EB4 之後）：
//
//	類別 = sub_5F40(地形碼)          ; 碼 & 0x1F 之後查 ds:52B2 的 32 項表
//	類別 1 → n = 具備技能 0x0B（登山家）的人數；n < 2 就是 Impassable!
//	類別 3 → n = 具備技能 0x0D（探險家）的人數；n < 2 就是 Impassable!
//	類別 4 且 ds:16DA == 0x0A → Can't swim!
//
// 技能編號 0x0B 與 0x0D 與手冊的第二技能表對得上（11 登山家、13 探險家），
// 「兩人以上」也正是手冊寫的條件 —— 原版的判斷就是 `si < 2`。

// TerrainClass 回傳這一格的野外地形類別。
func (m *Map) TerrainClass(x, y int) int {
	c := Cell(x, y)
	if c < 0 || data == nil {
		return gamedata.TerrainOpen
	}
	return data.TerrainClass(m.Attr[c])
}

// CountSkill 回傳隊伍裡具備某項第二技能的人數。
//
// 每個人有兩項，各佔記錄 `+80` 的一個 nibble。
func (s *Session) CountSkill(skill int) int {
	n := 0
	for _, c := range s.Party {
		if c.Empty() {
			continue
		}
		for _, k := range c.Skills {
			if k == skill {
				n++
				break // 同一個人不重複計
			}
		}
	}
	return n
}

// EnterOutdoor 判斷隊伍能不能走進野外的某一格，不能就回傳原版的訊息。
func (s *Session) EnterOutdoor(x, y int) (bool, string) {
	m := s.World.CurrentMap()
	if m == nil {
		return true, ""
	}
	switch m.TerrainClass(x, y) {
	case gamedata.TerrainMountain:
		if s.CountSkill(gamedata.SkillMountaineer) < gamedata.SkillPartyNeeded {
			return false, "過不去！"
		}
	case gamedata.TerrainForest:
		if s.CountSkill(gamedata.SkillPathfinder) < gamedata.SkillPartyNeeded {
			return false, "過不去！"
		}
	case gamedata.TerrainWater:
		// 原版還要看 ds:16DA 與 ds:03D9 兩個旗標（船／水上行走之類），
		// 那兩個還沒解，所以水域一律擋住。
		return false, "不會游泳！"
	}
	return true, ""
}
