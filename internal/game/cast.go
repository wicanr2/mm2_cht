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
	// Effect 是效果的播報，尚未實作效果的法術是空字串。
	Effect string
	// Reason 是失敗原因，成功時是空字串。
	Reason string
}

func (r CastResult) String() string {
	if !r.OK {
		return r.Reason
	}
	cost := fmt.Sprintf("%s：消耗 %d 點法力", r.Spell.Name, r.SP)
	if r.Gems > 0 {
		cost += fmt.Sprintf("與 %d 顆寶石", r.Gems)
	}
	cost += "。"
	if r.Effect != "" {
		cost += r.Effect
	}
	return cost
}

// SpellIndex 把「法術系 + 該系的第幾條（1 起算，照手冊的排列）」
// 換成引擎用的 0–95 編號。
//
// **引擎的編號跟著 `data/spells.json` 走：0–47 牧師系、48–95 巫師系。**
// 原版檔案是**相反的**（`SPELLS.DAT` 與兩個施法 overlay 的跳表都是
// 巫師系在前，見 docs/formats/09），`cmd/mm2data` 產生 `spellcosts.json`
// 時已經把兩半對調，所以這裡不必再換。
//
// 「第幾條」是該系裡的排列順序，不是手冊的 `Index` 欄位 ——
// `Index` 每升一級就重新從 1 數起。
func SpellIndex(school SpellSchool, n int) int {
	if n < 1 || n > 48 {
		return -1
	}
	if school == SchoolSorcerer {
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
	res := CastResult{OK: true, SP: needSP, Gems: needGems, Spell: sp}
	res.Effect = s.applyEffect(idx, who)
	return res
}

// applyEffect 套用已解出效果的法術，回傳給玩家看的一行字。
//
// 目前只有牧師系的治療與解狀況那七條 —— 它們在 `2CAST1.OVL` 的
// handler 短得可以逐行讀完，而且效果就是「檢查狀況位元組、清掉幾位、
// 加生命」。其餘八十幾條的 handler 還沒解（見 docs/formats/09-spells.md），
// 一律回空字串，代價照扣但不假裝有效果。
func (s *Session) applyEffect(idx, who int) string {
	e, ok := spellEffects[idx]
	if !ok {
		return ""
	}
	return e(s, who)
}

// target 是這一版的選人規則：治療類一律作用在施法者身上。
//
// 原版是 `sub_1CF8C` 跳出選單讓玩家挑（回 0x1B 表示取消），
// remake 還沒有那個介面。
func (s *Session) healTarget(who int) *Character { return &s.Party[who] }

// heal 是 `sub_1CE46(N)`：狀況 `>= 0x80` 就沒效，否則清掉狀況的
// bit 4／6／7，再加 N 點生命。
func heal(n int) func(*Session, int) string {
	return func(s *Session, who int) string {
		c := s.healTarget(who)
		if c.CondBits >= CondBitSevere {
			return "沒有效果。"
		}
		c.setCond(c.CondBits & 0x2F)
		c.addHP(n)
		return fmt.Sprintf("%s恢復了 %d 點生命。", c.Name, n)
	}
}

// cure 是解狀況那幾條：狀況 `>= 0x80` 就沒效，否則用遮罩清掉指定的位元。
func cure(mask byte, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		c := s.healTarget(who)
		if c.CondBits >= CondBitSevere {
			return "沒有效果。"
		}
		c.setCond(c.CondBits & mask)
		return fmt.Sprintf("%s的%s解除了。", c.Name, what)
	}
}

// restoreExact 是解除石化與復活術：狀況位元組**整個**等於指定值才有效。
func restoreExact(want byte, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		c := s.healTarget(who)
		if c.CondBits != want {
			return "沒有效果。"
		}
		c.setCond(0)
		return fmt.Sprintf("%s%s了。", c.Name, what)
	}
}

// spellEffects 的 key 是引擎的 0–95 編號（＝ `data/spells.json` 的索引）。
//
// 這七條的原始 handler 在 `2CAST1.OVL`。跳表用的是原版編號，換算回來是：
//
//	跳表 51 → 3  急救術     sub_1CC5C = sub_1CE46(8)
//	跳表 55 → 7  治傷術     sub_1CCA8 = sub_1CE46(0Fh)
//	跳表 64 → 16 解毒術     狀況 &= 77h
//	跳表 70 → 22 治病術     狀況 &= 7Bh
//	跳表 78 → 30 恢復術     狀況 = 0
//	跳表 81 → 33 解除石化   狀況 == 82h 才作用
//	跳表 87 → 39 復活術     狀況 == 81h 才作用
var spellEffects = map[int]func(*Session, int) string{
	3:  heal(8),                             // 急救術
	7:  heal(15),                            // 治傷術
	16: cure(0x77, "中毒"),                    // 解毒術
	22: cure(0x7B, "疾病"),                    // 治病術
	30: cureAll,                             // 恢復術
	33: restoreExact(CondPetrified, "解除了石化"), // 解除石化
	39: restoreExact(CondDeadBits, "復活"),     // 復活術

	// 全域計數型。位址與 handler：
	//
	//	sub_1C1B4  照明術       ds:03D5 += 1（上限 0FEh）
	//	sub_1C320  飄浮術       ds:03D8 += 1
	//	sub_1C550  守衛術       ds:03DA += 1
	//	sub_1C570  庇護術       ds:0414 += 1
	//	sub_1C8C8  水行術       ds:03D9 = 1
	//	sub_1C8E0  風界傳送術   ds:03DD = 1
	//	sub_1C984  地界傳送術   ds:03DF = 1
	//	sub_1CA00  水界傳送術   ds:03DC = 1
	//	sub_1CA10  火界傳送術   ds:03DE = 1
	//	sub_1CC3A  魔法防護術   ds:03D6 = 等級 + 10
	//	sub_1CCB4  拒絕傷害     ds:03D7 = 等級 + 20
	//	sub_1C87C  製造食物     記錄 +37 += 8，上限 40
	11: levelPlus(0x03D7, 20, "全隊獲得拒絕傷害的保護。"),
	15: makeFood,
	19: setOne(0x03D9, "全隊可以在水面行走。"),
	21: setOne(0x03DD, "風之界的傳送門開啟了。"),
	31: setOne(0x03DF, "地之界的傳送門開啟了。"),
	35: setOne(0x03DC, "水之界的傳送門開啟了。"),
	41: setOne(0x03DE, "火之界的傳送門開啟了。"),
	52: bump(0x03D5, 0xFE, "四周亮了起來。"),
	59: bump(0x03D8, 0xFF, "全隊飄浮起來。"),
	61: levelPlus(0x03D6, 10, "全隊獲得魔法防護。"),
	71: bump(0x03DA, 0xFF, "全隊獲得守衛。"),
	77: bump(0x0414, 0xFF, "全隊獲得庇護。"),

	// 攻擊法術（`2CAST2.OVL`）。傷害是逐行讀出來的：
	//
	//	sub_1CA6E  傷痛術     rand(1,12) + 3
	//	sub_1CB8A  噴酸術     rand(1,49) + 11
	//	sub_1CC34  致命蟲群術 rand(1,33) + 7
	//	sub_1CDBA  月光術     rand(1,91) + 9
	//	sub_1C16A  火箭術     rand(1,5)  + 3
	//	sub_1C1EA  閃電箭     rand(1,9)  + 7
	//	sub_1C75C  奇異之光術 rand(1,9)
	//	sub_1C81A  隕石雨     rand(1,21) + 24
	//	sub_1C8CA  星爆術     rand(1,161) + 39
	//
	// 目標數取自手冊的「目標」欄，等級是強推論。
	10: damageSpell(12, 3, 1, "傷痛術"),
	20: damageSpell(49, 11, 3, "噴酸術"),
	27: damageSpell(33, 7, 10, "致命蟲群術"),
	37: damageSpellLo(255, 400, 0, 1, "火焰枷鎖"),
	38: damageSpell(91, 9, 10, "月光術"),
	51: damageSpell(5, 3, 1, "火箭術"),
	56: damageSpell(9, 7, 1, "閃電箭"),
	90: damageSpell(21, 24, 99, "隕石雨"),
	94: damageSpell(161, 39, 99, "星爆術"),

	// 隨等級累加的那一批（`sub_1A82C`）。手冊只在講「每級 N 點」時
	// 與程式碼完全相符（冷凍射線每級 6、超級電擊每級 20、
	// 焚化術每級 20—40）；寫成骰子範圍的那幾條下界對不上，以程式碼為準。
	50: levelDamageSpell(5, 1, 1, "能量爆破術"),
	62: levelDamageSpell(5, 3, 1, "酸液"),
	65: levelDamageSpell(5, 1, 4, "電擊術"),
	68: levelDamageSpell(0, 6, 1, "冷凍射線"),
	70: levelDamageSpell(5, 1, 6, "火球術"),
	76: levelDamageSpell(7, 1, 10, "砂暴術"),
	81: levelDamageSpell(0, 10, 3, "冷凍術"),
	83: levelDamageSpell(0, 20, 1, "超級電擊"),
	84: levelDamageSpell(11, 1, 10, "飛劍術"),
	88: levelDamageSpell(21, 19, 1, "焚化術"),
	89: levelDamageSpell(9, 7, 10, "高壓電擊術"),
	93: levelDamageSpell(16, 4, 10, "地獄之火"),
}

// ── 全域計數型的法術 ─────────────────────────────────────────────────────
//
// 這一批的 handler 都只有十幾到三十幾個位元組，動作是「把某個 DGROUP
// 位元組加一或設成一」。那些位元組與事件腳本的全域變數是**同一批** ——
// 四個界傳送術寫的 `ds:03DC`–`ds:03DF` 正是腳本選擇器 `0x27`–`0x2A`，
// 所以腳本問得出「開了哪幾道元素之門」。
//
// 位址一律走 `World.Globals`（key 就是 DGROUP 位址），與腳本共用一份。

// bump 是「把某個全域加一」那一型，到上限就不再加。
func bump(addr uint16, max byte, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		if v := s.World.Globals[addr]; v < max {
			s.setGlobalAddr(addr, v+1)
		}
		return what
	}
}

// setOne 是「把某個全域設成 1」那一型。
func setOne(addr uint16, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		s.setGlobalAddr(addr, 1)
		return what
	}
}

// levelPlus 是「把某個全域設成施法者等級加 N」那一型。
//
// 原版讀的是記錄 `+0x71`（＝ +113），名冊四十筆裡它與等級（`+32`）
// 逐筆相同 —— 那是等級的另一份。
func levelPlus(addr uint16, n int, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		v := s.Party[who].Level + n
		if v > 255 {
			v = 255
		}
		s.setGlobalAddr(addr, byte(v))
		return what
	}
}

// makeFood 是製造食物：施法者的食物 `+8`，上限 40（`0x28`）。
// 食物在記錄 `+37`，原版寫的是 `[bx+25h]`。
func makeFood(s *Session, who int) string {
	c := &s.Party[who]
	if c.Food >= 40 {
		return "食物已經帶滿了。"
	}
	c.Food += 8
	if c.Food > 40 {
		c.Food = 40
	}
	if len(c.Raw) == RecordSize {
		c.Raw[offFood] = byte(c.Food)
	}
	return fmt.Sprintf("%s的食物增加到 %d。", c.Name, c.Food)
}

// setGlobalAddr 直接依 DGROUP 位址寫全域，繞過腳本的選擇器。
func (s *Session) setGlobalAddr(addr uint16, v byte) {
	if s.World.Globals == nil {
		s.World.Globals = map[uint16]byte{}
	}
	s.World.Globals[addr] = v
}

// ── 攻擊法術 ─────────────────────────────────────────────────────────────
//
// `2CAST2.OVL` 收的是戰鬥法術（`2CAST1` 收非戰鬥的那些）。傷害那一段的
// 形狀一致：`rand(1, N)` 之後加一個常數，結果寫進 `ds:9FC6`，再交給
// 套用傷害的程序。
//
// 目標數取自手冊的「目標」欄 —— 原版是逐條在 handler 裡決定，
// 還沒逐條讀完，所以那一半的等級是**強推論**。

// damageSpell 對前 count 隻還站著的怪物各造成 rand(1,hi) + add 點傷害。
func damageSpell(hi, add, count int, what string) func(*Session, int) string {
	return damageSpellLo(1, hi, add, count, what)
}

// damageSpellLo 是下界不是 1 的版本（目前只有火焰枷鎖）。
func damageSpellLo(lo, hi, add, count int, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		if s.Fight == nil {
			return "不在戰鬥中。"
		}
		return applyDamage(s, count, what, func() int { return s.Rand.Range(lo, hi) + add })
	}
}

// applyDamage 對前 count 隻還站著的怪物各擲一次傷害。
//
// 多目標的 handler 是**逐隻重擲**的（原版在迴圈內再呼叫一次擲骰），
// 不是擲一次套用到全部。
func applyDamage(s *Session, count int, what string, roll func() int) string {
	hit, total := 0, 0
	for _, m := range s.Fight.Monsters {
		if hit >= count {
			break
		}
		if !m.CombatCondition().Acts() {
			continue
		}
		dmg := roll()
		m.TakeDamage(dmg)
		total += dmg
		hit++
	}
	if hit == 0 {
		return "沒有目標。"
	}
	return fmt.Sprintf("%s對 %d 個目標造成 %d 點傷害。", what, hit, total)
}

// levelDamageSpell 是隨施法者等級累加的傷害（原版 `sub_1A82C`）：
//
//	ds:9FC6 = 0
//	n = 施法者記錄 +113（等級）
//	重複 n 次：ds:9FC6 += (sides 為 0 ? 0 : rand(1, sides)) + bonus
//
// 所以 `sides=0` 的那幾條是「每級固定 bonus 點」，其餘是
// 「每級擲一次 1d sides 再加 bonus」—— 逐級重擲，不是擲一次乘等級。
func levelDamageSpell(sides, bonus, count int, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		lv := int(s.Party[who].Level)
		roll := func() int {
			total := 0
			for i := 0; i < lv; i++ {
				n := 0
				if sides != 0 {
					n = s.Rand.Range(1, sides)
				}
				total += n + bonus
			}
			return total
		}
		return applyDamage(s, count, what, roll)
	}
}

// cureAll 是恢復術：狀況 < 0x80 就整個清成 0。
func cureAll(s *Session, who int) string {
	c := s.healTarget(who)
	if c.CondBits >= CondBitSevere {
		return "沒有效果。"
	}
	c.setCond(0)
	return fmt.Sprintf("%s的狀況恢復了。", c.Name)
}
