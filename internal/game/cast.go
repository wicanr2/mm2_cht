package game

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/gamedata"
)

// 施法。
//
// 這一層做的是**能不能施**與**付出什麼**，不是各條法術的效果。
// 效果在原版是 `2CAST1`／`2CAST2` 的兩張跳表，每條法術一支 handler；
// 那 96 支還沒逐條解出來（見 docs/formats/09-spells.md）。
//
// 已經照原版做的：
//
//   - **法術編號的分派**：`SPELLS.DAT` 與兩個施法 overlay 用的是同一套
//     0–95 編號，前 48 給巫師與弓箭手、後 48 給牧師系（root `sub_15644`
//     對其餘職業把序號加 48）。
//   - **代價**：`data/spellcosts.json`，法力 = 固定 + 每等級 × 施法者等級，
//     另加寶石。
//   - **會不會**：角色記錄 `+81` 起 48 個位元，一位一條。

// CastResult 是一次施法的結果。
type CastResult struct {
	OK    bool
	SP    int // 實際扣掉的法力
	Gems  int // 實際扣掉的寶石
	Spell Spell
	// Reason 是失敗原因，成功時是空字串。
	Reason string
}

func (r CastResult) String() string {
	if !r.OK {
		return r.Reason
	}
	if r.Gems > 0 {
		return fmt.Sprintf("%s：消耗 %d 點法力與 %d 顆寶石。", r.Spell.Name, r.SP, r.Gems)
	}
	return fmt.Sprintf("%s：消耗 %d 點法力。", r.Spell.Name, r.SP)
}

// SpellIndex 把「法術系 + 該系的第幾條（1 起算）」換成 0–95 的全域編號。
//
// 這個編號同時是 `SPELLS.DAT` 的索引與兩個施法 overlay 跳表的索引。
// **前 48 是巫師系**，所以牧師系要加 48。
func SpellIndex(school SpellSchool, n int) int {
	if n < 1 || n > 48 {
		return -1
	}
	if school == SchoolCleric {
		return 48 + n - 1
	}
	return n - 1
}

// Knows 回報這個角色學會了該系的第 n 條法術（1 起算）。
//
// 位元遮罩在記錄 `+81` 起的六個位元組，一位一條，低位在前。
func (c *Character) Knows(n int) bool {
	i := n - 1
	if i < 0 || i >= 48 {
		return false
	}
	return c.SpellsKnown[i/8]>>(uint(i)%8)&1 != 0
}

// Learn 讓角色學會該系的第 n 條法術。
func (c *Character) Learn(n int) {
	i := n - 1
	if i < 0 || i >= 48 || len(c.Raw) != RecordSize {
		return
	}
	c.Raw[offSpells+i/8] |= 1 << (uint(i) % 8)
	*c = parseCharacter(c.Raw)
}

// CanCast 回報這個職業會不會施法。
//
// 四個會：牧師與巫師（一開始就有法力），遊俠與弓箭手（手冊：要較高的
// 經驗等級才有法力等級）。名冊裡只有牧師與巫師的法力欄位非零，
// 與手冊一致。
func CanCast(class Class) bool {
	switch class {
	case Cleric, Sorcerer, Paladin, Archer:
		return true
	}
	return false
}

// SpellSchoolOf 回傳這個職業用哪一系的法術。
//
// 原版 `sub_15644`：職業 2（弓箭手）與 4（巫師）走前半，其餘走後半。
func SpellSchoolOf(class Class) SpellSchool {
	if class == Archer || class == Sorcerer {
		return SchoolSorcerer
	}
	return SchoolCleric
}

// Cast 讓隊伍裡第 who 個人施該系的第 n 條法術。
//
// 效果尚未實作 —— 這裡處理的是資格與代價：會不會、法力等級夠不夠、
// 法力與寶石付不付得起，付得起就扣掉。
func (s *Session) Cast(who, n int) CastResult {
	if who < 0 || who >= len(s.Party) {
		return CastResult{Reason: "沒有這個人。"}
	}
	c := &s.Party[who]
	if !CanCast(c.Class) {
		return CastResult{Reason: fmt.Sprintf("%s不會施法。", ClassName(int(c.Class)))}
	}
	school := SpellSchoolOf(c.Class)
	idx := SpellIndex(school, n)
	if idx < 0 {
		return CastResult{Reason: "沒有這條法術。"}
	}
	all := Spells()
	if idx >= len(all) {
		return CastResult{Reason: "法術表沒載入。"}
	}
	sp := all[idx]
	if !c.Knows(n) {
		return CastResult{Spell: sp, Reason: fmt.Sprintf("%s還沒學會%s。", c.Name, sp.Name)}
	}
	if c.SL < sp.Level {
		return CastResult{Spell: sp, Reason: fmt.Sprintf("%s的法力等級不夠施展%s。", c.Name, sp.Name)}
	}
	cost := gamedata.SpellCost{}
	if data != nil {
		cost = data.SpellCostAt(idx)
	}
	needSP, needGems := cost.SP(c.Level), cost.Gems()
	if c.SP < needSP {
		return CastResult{Spell: sp, Reason: fmt.Sprintf("%s的法力不足（要 %d 點）。", c.Name, needSP)}
	}
	if needGems > 0 && c.Gems < needGems {
		return CastResult{Spell: sp, Reason: fmt.Sprintf("寶石不夠（要 %d 顆）。", needGems)}
	}
	c.SP -= needSP
	c.Raw[offSP] = byte(c.SP)
	c.Raw[offSP+1] = byte(c.SP >> 8)
	if needGems > 0 {
		c.Gems -= needGems
		c.Raw[offGems] = byte(c.Gems)
		c.Raw[offGems+1] = byte(c.Gems >> 8)
	}
	return CastResult{OK: true, SP: needSP, Gems: needGems, Spell: sp}
}
