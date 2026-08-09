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
// healTarget 回傳這次施法作用的隊員。
//
// 原版對「選一名隊員」那批法術會先呼叫 `sub_1CF8C` 讓玩家選人
// （回傳 `0x1B` 表示取消）。這裡用 `Session.Target` 代替那個選單：
// 沒設（負值或超出範圍）就是施法者自己。
func (s *Session) healTarget(who int) *Character {
	if s.Target >= 0 && s.Target < len(s.Party) {
		return &s.Party[s.Target]
	}
	return &s.Party[who]
}

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
	10: damageSpell(12, 3, 1, 0, "傷痛術"),
	20: damageSpell(49, 11, 3, 4, "噴酸術"),
	27: damageSpell(33, 7, 10, 0, "致命蟲群術"),
	37: damageSpellLo(255, 400, 0, 1, 1, "火焰枷鎖"),
	38: damageSpell(91, 9, 10, 0, "月光術"),
	51: damageSpell(5, 3, 1, 1, "火箭術"),
	56: damageSpell(9, 7, 1, 2, "閃電箭"),
	90: damageSpell(21, 24, 99, 0, "隕石雨"),
	94: damageSpell(161, 39, 99, 0, "星爆術"),

	// 隨等級累加的那一批（`sub_1A82C`）。手冊只在講「每級 N 點」時
	// 與程式碼完全相符（冷凍射線每級 6、超級電擊每級 20、
	// 焚化術每級 20—40）；寫成骰子範圍的那幾條下界對不上，以程式碼為準。
	50: levelDamageSpell(5, 1, 1, 0, "能量爆破術"),
	62: levelDamageSpell(5, 3, 1, 4, "酸液"),
	65: levelDamageSpell(5, 1, 4, 2, "電擊術"),
	68: levelDamageSpell(0, 6, 1, 3, "冷凍射線"),
	70: levelDamageSpell(5, 1, 6, 1, "火球術"),
	76: levelDamageSpell(7, 1, 10, 0, "砂暴術"),
	81: levelDamageSpell(0, 10, 3, 3, "冷凍術"),
	83: levelDamageSpell(0, 20, 1, 2, "超級電擊"),
	84: levelDamageSpell(11, 1, 10, 0, "飛劍術"),
	88: levelDamageSpell(21, 19, 1, 1, "焚化術"),
	89: levelDamageSpell(9, 7, 10, 2, "高壓電擊術"),
	93: levelDamageSpell(16, 4, 10, 1, "地獄之火"),

	// 傷害寫死的三條。分裂術 100 點與冷凍光線 25 點與手冊逐字相符。
	14: fixedDamageSpell(25, 5, 3, "冷凍光線"),
	74: fixedDamageSpell(100, 1, 0, "分裂術"),
	92: fixedDamageSpell(1000, 1, 0, "魔法黑洞"),
	42: gravity,

	// 狀態類。代碼與目標數見 `docs/formats/09-spells.md`；
	// 「4 ＋ 每級 1」那三條的目標數是強推論（原版走 `sub_1719E`）。
	12: statusSpell(1, 0, 6, "沈默術"),
	13: statusSpell(2, 10, 0, "衰弱術"),
	0:  statusSpell(3, 10, 6, "幻影術"),
	54: statusSpell(4, 0, 5, "催眠術"),
	66: statusSpell(5, 0, 6, "魔網"),
	17: statusSpell(5, 5, 6, "定身術"),
	29: statusSpell(5, 10, 6, "麻痺術"),
	69: statusSpell(6, 5, 0, "衰弱心智術"),
	26: statusSpell(7, 1, 0, "狂風陣"),
	34: statusSpell(7, 1, 3, "洪水陣"),
	36: statusSpell(7, 1, 4, "后土陣"),
	40: statusSpell(7, 1, 1, "烈火陣"),
	75: statusSpell(8, 3, 6, "死亡之指"),
	79: statusSpell(9, 3, 0, "粉碎術"),
	87: prismatic,

	// 隊伍增益：全域計數器 +1，上限 255（`cmp ds:XXXX, 0FFh / jae / inc`）。
	// 與 2CAST1 那十二條同一個形狀，只是計數器換一個。
	2:  bump(0x03E3, 0xFF, "全隊獲得祝福。"),
	64: bump(0x03E4, 0xFF, "全隊隱形了。"),
	72: bump(0x03E5, 0xFF, "護罩張了起來。"),
	91: bump(0x03E6, 0xFF, "強力護罩張了起來。"),

	// 戰鬥中一次性：旗標為 0 才生效，生效後設起來（`cmp X,0 / jne / inc X`）。
	// 旗標存在 Encounter 上，一場戰鬥結束就沒了。
	73: banned(BanTimeDistort, combatFlag(0x9FC8, "時間扭曲了。")),
	80: banned(BanTrapMonsters, combatFlag(0x9FC4, "怪物被困住了。")),
	44: combatFlag(0x9FCD, "神明介入了。"),
	6:  turnUndead,
	45: combatFlag(0x9FCA, "神聖之咒生效了。"),

	// 2CAST1 的另外五條。照明系共用 ds:03D5 那一個計數器：
	// 照明術 +1、持續照明術一次 +20。鷹眼術與巫師眼是每級 +5、
	// 上限 250，兩支的組語逐位元組相同，只差計數器位址。
	4:  bump(0x03D5, 0xFE, "四周亮了起來。"),
	18: addCapped(0x03D5, 20, 0xEB, "光持續著。"),
	55: perLevel(0x03E0, 5, 0xFA, "視野變得開闊。"),
	67: perLevel(0x03E1, 5, 0xFA, "看得見牆後了。"),
	5:  healPerLevel(10),

	// 喚醒術：整隊掃過去，狀況位元組低於 0x80 的清掉沈睡位元
	// （`and [記錄+38], 6Fh`）。兩系都有這條。
	1:  awaken,
	48: awaken,

	// 回復陣營：把原始陣營（`+13`）抄回目前陣營（`+106`）。
	23: restoreAlign,

	// 跳躍術：往前跳兩格。
	58: jump,

	// 復活：清掉重症，代價是兩個人變老、目標少一點幸運。
	46: resurrect,

	// 回春術：擲得過才年輕，擲不過反而變老。
	32: rejuvenate,

	// 兩條對背包物品動手的。都用施法者自己的背包（`ds:5DD6`），
	// 不是選隊員。
	82: recharge,
	95: empower,
	85: duplicate,
	47: removeCurse,

	// 傳送到地面：把隊伍送回這張圖登記的地面出口。
	24: toGround,

	// 穿透術：往前一格，不看牆。
	86: banned(BanEtherealize, etherealize),

	// 兩條要玩家輸入數字的傳送。
	78: banned(BanTeleportSpell, teleportSteps),
	43: cityPortal,

	// 魯易浮標：標記與返回。
	60: banned(BanTeleport, beacon),

	// 狂暴術：一名隊員燃燒自己換一輪全場攻擊。
	28: berserk,

	// 飛行術：A–E 欄、1–4 列選一張野外圖。
	63: banned(BanTeleport, flight),

	// 神聖賜與：全隊傷害加成累加「施法者等級 ÷ 2」。
	25: blessDamage,

	// 自然之門：落點隨遊戲內的日子變動。
	9: banned(BanTeleport, natureGate),

	// 三條純顯示。原版只畫面面，不改任何遊戲狀態 ——
	// 引擎照做，內容留給 UI 層。
	49: info("背包裡的魔法物品與剩餘次數顯示出來了。"),
	53: info("目前位置與這一區的地圖顯示出來了。"),
	57: info("怪物的狀況顯示出來了。"),
}

// blessDamage 是神聖賜與（`sub_1CBD8`）：`ds:03E7` 加上
// 「施法者等級 ÷ 2」。手冊寫的「每 2 個等級加 1 點傷害力」
// 就是這個右移一位。
func blessDamage(s *Session, who int) string {
	lv := 1
	if who >= 0 && who < len(s.Party) {
		lv = int(s.Party[who].Level)
	}
	n := byte(lv / 2)
	s.setGlobalAddr(0x03E7, s.World.Globals[0x03E7]+n)
	return fmt.Sprintf("全隊的傷害加了 %d 點。", n)
}

// natureGate 是自然之門（`sub_1C7DA`）。
//
// 前提是 `ds:03CA` 等於 9；日子到 150 以上時它會被改成 8，
// 之後這條法術就再也開不了門。落點查三張表，見 `gamedata.NatureGate`。
func natureGate(s *Session, who int) string {
	w := s.World
	if w.Globals[gateQuest] != 9 {
		return "沒有效果。"
	}
	day := int(w.Globals[gateDayLo]) | int(w.Globals[gateDayHi])<<8
	if data == nil {
		return "沒有效果。"
	}
	m, x, y, ok := data.NatureGate(day)
	if !ok || m >= len(w.Maps) {
		return "沒有效果。"
	}
	if day >= 150 {
		s.setGlobalAddr(gateQuest, 8)
	}
	w.MapIndex, w.X, w.Y = m, x, y
	return "空間通道打開了。"
}

// 自然之門讀的三個全域。
//
// `ds:03CA` **不是任務階段，是計時器索引** —— `ds:03A2` 是一排計數器，
// `ds:03CA` 選其中一格，而索引 9 指到的正好是 `ds:03B4`。
// 自然之門把 `ds:03B4` 寫死在程式碼裡，所以它必須先確認索引是 9，
// 否則讀到的就不是「目前的日子」。判準是 opcode `0x23`：
// 那條算的是 `ds:03A2[ds:03CA × 2]`，而 `0x3A2 + 9 × 2 = 0x3B4`。
const (
	gateQuest = 0x03CA
	gateDayLo = 0x03B4
	gateDayHi = 0x03B5
)

// info 是純顯示的那幾條：原版只畫畫面，不改遊戲狀態。
func info(what string) func(*Session, int) string {
	return func(s *Session, who int) string { return what }
}

// flight 是飛行術（`sub_1C3EE`）：讀一個 `A`–`E` 的字母與一個
// `1`–`4` 的數字，查 `ds:30BC` 的 5×4 表得到野外地圖編號。
//
// 落點與城市傳送術一樣把 X／Y 設成 `0FFh`，交給 `ATTRIB` `+14`。
func flight(s *Session, who int) string {
	col, row := s.Column, s.Choice-1
	m := -1
	if data != nil {
		m = data.FlightMap(col, row)
	}
	if m < 0 {
		return "沒有指定去處。"
	}
	if m >= len(s.World.Maps) {
		return "沒有效果。"
	}
	s.World.MapIndex = m
	if s.Attrs != nil && m < len(s.Attrs) {
		if x, y, ok := s.Attrs[m].Entry(); ok {
			s.World.X, s.World.Y = x, y
		}
	}
	return fmt.Sprintf("隊伍飛到了 %c%d。", 'A'+rune(col), row+1)
}

// berserk 是狂暴術（`sub_1CC64`）。
//
// 順序照原版，包括那個小瑕疵：**狀況先被設成昏迷，之後才檢查運氣**，
// 所以運氣為 0 的人會白白昏過去。
//
//	一場戰鬥只能一次（`ds:9FC1`）
//	目標狀況必須是正常（`+38 == 0`）
//	+38 = 40h（昏迷）
//	+115（運氣）為 0 就到此為止，否則減一
//	+94（目前生命）= 0
//	傷害 = (`+76` 武器骰 + `+77` 命中加值 + 10) × 2，打 10 隻
func berserk(s *Session, who int) string {
	if s.Fight == nil {
		return "不在戰鬥中。"
	}
	if s.Fight.Flags == nil {
		s.Fight.Flags = map[uint16]byte{}
	}
	if s.Fight.Flags[0x9FC1] != 0 {
		return "已經有人狂暴過了。"
	}
	s.Fight.Flags[0x9FC1]++
	c := s.healTarget(who)
	if c.CondBits != 0 {
		return "沒有效果。"
	}
	c.setCond(CondBitUnconscious)
	luck := c.FieldByte(offLuck)
	if luck == 0 {
		return fmt.Sprintf("%s昏了過去，卻沒能發作。", c.Name)
	}
	c.SetFieldByte(offLuck, 0x00, luck-1)
	c.SetFieldValue(offHP, 2, 0)
	dmg := (int(c.FieldByte(offWeapDice)) + int(c.FieldByte(offHitBonus)) + 10) * 2
	s.Fight.Flags[0x9FC0]++
	return applyDamage(s, who, 10, 0, "狂暴", func() int { return dmg })
}

// 魯易浮標記下的落點：`ds:03E8` 是地圖、`ds:03E9` 是 nibble 打包的座標。
const (
	beaconMap = 0x03E8
	beaconPos = 0x03E9
)

// beacon 是魯易浮標（`sub_1C340`）：讀 `1` 記下現在的位置、
// 讀 `2` 回到記下的位置。
//
// 座標存成一個位元組（`(Y << 4) + X`），與 `ATTRIB` `+14` 同一種打包。
func beacon(s *Session, who int) string {
	w := s.World
	switch s.Choice {
	case 1:
		s.setGlobalAddr(beaconMap, byte(w.MapIndex))
		s.setGlobalAddr(beaconPos, byte(w.Y<<4)+byte(w.X))
		return "記下了這個地方。"
	case 2:
		m := int(w.Globals[beaconMap])
		if m >= len(w.Maps) {
			return "沒有效果。"
		}
		pos := w.Globals[beaconPos]
		w.MapIndex = m
		w.X, w.Y = int(pos&0x0F), int(pos>>4)
		return "回到了記下的地方。"
	}
	return "沒有選。"
}

// teleportSteps 是傳送術（`sub_1C590`）：讀一個 `1`–`9` 的按鍵，
// 往面向的方向走那麼多格，**每一步都 `and 0Fh`、都不查牆**。
//
// 逐步繞邊與「一次加完再遮罩」不同 —— 走九格會真的繞一圈回來。
func teleportSteps(s *Session, who int) string {
	n := s.Choice
	if n < 1 || n > 9 {
		return "沒有指定步數。"
	}
	w := s.World
	dx, dy := w.Face.Delta()
	for i := 0; i < n; i++ {
		w.X, w.Y = (w.X+dx)&0x0F, (w.Y+dy)&0x0F
	}
	return fmt.Sprintf("隊伍傳送了 %d 格。", n)
}

// cityPortal 是城市傳送術（`sub_1CA20`）：讀一個 `1`–`5` 的按鍵，
// 送到那座城（地圖 0–4）。
//
// 它把 X／Y 都設成 `0FFh`，而 root 在 `ds:0393 == 0FFh` 時改用
// `ATTRIB` `+14` 的預設進入座標 —— 落點是資料決定的，不是法術決定的。
func cityPortal(s *Session, who int) string {
	n := s.Choice
	if n < 1 || n > 5 {
		return "沒有指定城市。"
	}
	m := n - 1
	if m >= len(s.World.Maps) {
		return "沒有效果。"
	}
	s.World.MapIndex = m
	if s.Attrs != nil && m < len(s.Attrs) {
		if x, y, ok := s.Attrs[m].Entry(); ok {
			s.World.X, s.World.Y = x, y
		}
	}
	return "隊伍傳送到城裡了。"
}

// etherealize 是穿透術（`sub_1C722`）：往面向的方向走一格，
// **完全不查牆**。座標各自 `and 0Fh`，走出邊界就繞到對邊。
func etherealize(s *Session, who int) string {
	w := s.World
	dx, dy := w.Face.Delta()
	w.X, w.Y = (w.X+dx)&0x0F, (w.Y+dy)&0x0F
	return "隊伍穿過了牆。"
}

// spellBanned 回報目前這張地圖禁不禁止某一類法術（`ATTRIB` `+26`）。
func (s *Session) spellBanned(bit byte) bool {
	if s.Attrs == nil || s.World.MapIndex >= len(s.Attrs) {
		return false
	}
	return s.Attrs[s.World.MapIndex].SpellBanned(bit)
}

// toGround 是傳送到地面（`sub_1C92A`）。
//
// 兩道 guard：這張圖禁止傳送，或它根本沒有登記地面出口
// （`+22` 為 0，野外圖都是這樣）。
func toGround(s *Session, who int) string {
	if s.spellBanned(BanTeleport) {
		return "這裡不能傳送。"
	}
	if s.Attrs == nil || s.World.MapIndex >= len(s.Attrs) {
		return "沒有效果。"
	}
	a := s.Attrs[s.World.MapIndex]
	x, y, ok := a.GroundPos()
	if !ok {
		return "沒有效果。"
	}
	s.World.MapIndex = a.GroundMap()
	s.World.X, s.World.Y = x, y
	return "隊伍回到了地面。"
}

// CursedCharge 是「被詛咒」的標記：充能欄（`+64`）的 `0xFF`。
//
// 判準是去咒術（`sub_1CB10`）—— 它只在該欄等於 `0xFF` 時才動手，
// 動的也只是把它改成 1。
const CursedCharge = 0xFF

// removeCurse 是去咒術（`sub_1CB10`）。
func removeCurse(s *Session, who int) string {
	slot := s.packSlot()
	if slot < 0 {
		return "沒有選物品。"
	}
	c := &s.Party[who]
	off := offPackCharge + slot
	if c.FieldByte(off) != CursedCharge {
		return "沒有效果。"
	}
	c.SetFieldByte(off, 0x00, 1)
	return "詛咒解除了。"
}

// duplicate 是複製術（`sub_1C68C`）：把施法者背包裡選中的那件
// 複製到第一個空槽，三個平行陣列（`+58` 編號、`+64` 充能、
// `+70` 屬性）一起抄。
//
// **編號 `0xD0` 以上的複製不了** —— 那道門檻擋掉的是任務物品那一段。
func duplicate(s *Session, who int) string {
	slot := s.packSlot()
	if slot < 0 {
		return "沒有選物品。"
	}
	c := &s.Party[who]
	id := c.FieldByte(offPackID + slot)
	if id == 0 {
		return "沒有效果。"
	}
	if id >= 0xD0 {
		return "這件東西複製不了。"
	}
	free := -1
	for i := 0; i < 6; i++ {
		if c.FieldByte(offPackID+i) == 0 {
			free = i
			break
		}
	}
	if free < 0 {
		return "背包滿了。"
	}
	c.SetFieldByte(offPackID+free, 0x00, id)
	c.SetFieldByte(offPackCharge+free, 0x00, c.FieldByte(offPackCharge+slot))
	c.SetFieldByte(offPackAttr+free, 0x00, c.FieldByte(offPackAttr+slot))
	return "物品複製出來了。"
}

// packSlot 回傳這次要動的背包槽位，沒選就沒得動。
func (s *Session) packSlot() int {
	if s.Item < 0 || s.Item >= 6 {
		return -1
	}
	return s.Item
}

// recharge 是能量補充術（`sub_1C648`）：施法者背包某件物品的
// 充能欄位（`+64`）加 `rand(1,6)`。欄位本來就是 0 的**不能充** ——
// 那是「這件東西沒有充能」與「充能用完」共用同一個 0 的後果。
func recharge(s *Session, who int) string {
	slot := s.packSlot()
	if slot < 0 {
		return "沒有選物品。"
	}
	c := &s.Party[who]
	off := offPackCharge + slot
	if c.FieldByte(off) == 0 {
		return "沒有效果。"
	}
	n := s.Rand.Range(1, 6)
	c.SetFieldByte(off, 0x00, c.FieldByte(off)+byte(n))
	return fmt.Sprintf("充能加了 %d 點。", n)
}

// empower 是加強法力（`sub_1C774`）：把背包某件物品屬性欄（`+70`）
// 的低六位加一，高兩位原樣保留。
//
// 代價是 `50 × 目前值` 點法力，**但原版只檢查夠不夠、沒有真的扣**。
// 低六位到 `0x3F` 時仍然會再加一，進位撞進 bit 6 —— 照抄。
func empower(s *Session, who int) string {
	slot := s.packSlot()
	if slot < 0 {
		return "沒有選物品。"
	}
	c := &s.Party[who]
	off := offPackAttr + slot
	attr := c.FieldByte(off)
	v := attr & 0x3F
	if int(c.SP) < 50*int(v) {
		return "法力不夠。"
	}
	v++
	c.SetFieldByte(off, 0x00, (attr&0xC0)|v)
	return "物品的法力加強了。"
}

// rejuvenate 是回春術（`sub_1C994`）。
//
// 兩次擲骰的順序照原版：**先**擲年數 `rand(1,10)`，**再**擲
// `rand(1,100)`。兩次都一定會擲，成敗只影響年數的正負號。
//
//	rand(1,100) < 50 且年齡 >= 18 → 年齡減年數
//	否則                          → 年齡加年數
func rejuvenate(s *Session, who int) string {
	c := s.healTarget(who)
	years := s.Rand.Range(1, 10)
	roll := s.Rand.Range(1, 100)
	if roll < 50 && c.FieldByte(offAge) >= 18 {
		age := int(c.FieldByte(offAge)) - years
		if age < 0 {
			age = 0
		}
		c.SetFieldByte(offAge, 0x00, byte(age))
		return fmt.Sprintf("%s年輕了 %d 歲。", c.Name, years)
	}
	addAge(c, years)
	return fmt.Sprintf("%s反而老了 %d 歲。", c.Name, years)
}

// addAge 是原版的 `sub_13B68(記錄, n)`：年齡（`+33`）加 n，上限 200。
func addAge(c *Character, n int) {
	v := int(c.FieldByte(offAge)) + n
	if v > 200 {
		v = 200
	}
	c.SetFieldByte(offAge, 0x00, byte(v))
}

// resurrect 是復活術（`sub_1CAA4`）。
//
// 只對重症（`+38 >= 80h`，石化與死亡那一類）有效。代價是
// **施法者年齡 +1、目標年齡 +5、目標幸運 -1**；幸運歸零就救不回來。
// 幸運的兩份（`+39` 與 `+115`）一起寫。
func resurrect(s *Session, who int) string {
	c := s.healTarget(who)
	if c.CondBits < CondBitSevere {
		return "沒有效果。"
	}
	if who >= 0 && who < len(s.Party) {
		addAge(&s.Party[who], 1)
	}
	addAge(c, 5)
	luck := c.FieldByte(offLuckB)
	if luck == 0 {
		return fmt.Sprintf("%s的幸運已經用盡了。", c.Name)
	}
	luck--
	c.SetFieldByte(offLuckB, 0x00, luck)
	c.SetFieldByte(offLuck, 0x00, luck)
	c.setCond(0)
	return fmt.Sprintf("%s復活了。", c.Name)
}

// jump 是跳躍術（`sub_1C23E`）。
//
// 只在室內有效（原版 `cmp ds:039D, 1 / je 失敗`，而接下來查的是室內
// 那套牆位元組）。目前格與中間格兩道牆都要通得過，通過就落在第二格。
// 座標各自 `and 0Fh` 繞回對邊，與一般移動同一套。
func jump(s *Session, who int) string {
	w := s.World
	m := w.CurrentMap()
	if m == nil || !m.Indoor {
		return "沒有效果。"
	}
	dx, dy := w.Face.Delta()
	mx, my := (w.X+dx)&0x0F, (w.Y+dy)&0x0F
	if !m.CanMove(w.X, w.Y, w.Face) || !m.CanMove(mx, my, w.Face) {
		return "有魔法障礙擋著。"
	}
	w.X, w.Y = (mx+dx)&0x0F, (my+dy)&0x0F
	return "隊伍往前跳了兩格。"
}

// awaken 是喚醒術（`sub_1CBEC`）。
func awaken(s *Session, who int) string {
	n := 0
	for i := range s.Party {
		c := &s.Party[i]
		if c.CondBits >= CondBitSevere {
			continue // 石化、死亡那一類喚不醒
		}
		if c.CondBits&CondBitAsleep != 0 {
			n++
		}
		c.setCond(c.CondBits & 0x6F)
	}
	if n == 0 {
		return "沒有人在睡。"
	}
	return fmt.Sprintf("%d 個人醒了過來。", n)
}

// restoreAlign 是回復陣營（`sub_1C8F0`）。
func restoreAlign(s *Session, who int) string {
	c := s.healTarget(who)
	c.SetFieldByte(106, 0x00, c.FieldByte(0x0D))
	return fmt.Sprintf("%s的陣營回復了。", c.Name)
}

// banned 把 `ATTRIB` `+26` 的禁令包在效果外面。
func banned(bit byte, f func(*Session, int) string) func(*Session, int) string {
	return func(s *Session, who int) string {
		if s.spellBanned(bit) {
			return "這裡不能用這個法術。"
		}
		return f(s, who)
	}
}

// turnUndead 是驅魔術：只對不死生物有效（原版查 `ds:9E33`，
// 那一格來自怪物記錄 `+18` 的 bit 7），一場戰鬥一次（`ds:9FCB`）。
func turnUndead(s *Session, who int) string {
	if s.Fight == nil {
		return "不在戰鬥中。"
	}
	n := 0
	for _, m := range s.Fight.Monsters {
		if mm, ok := m.(*Monster); ok && mm.Def.Undead && mm.CombatCondition().Acts() {
			n++
		}
	}
	if n == 0 {
		return "這裡沒有不死生物。"
	}
	return combatFlag(0x9FCB, fmt.Sprintf("%d 隻不死生物被驅散了。", n))(s, who)
}

// combatFlag 是「一場戰鬥只能用一次」的那批。
//
// 原版把旗標放在 `ds:9FC0`–`ds:9FCD` 這段戰鬥用的區域，
// 開戰時整段清 0；`cmp X, 0 / jne 跳過 / inc X` 是共同的形狀。
func combatFlag(addr uint16, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		if s.Fight == nil {
			return "不在戰鬥中。"
		}
		if s.Fight.Flags == nil {
			s.Fight.Flags = map[uint16]byte{}
		}
		if s.Fight.Flags[addr] != 0 {
			return "已經生效了。"
		}
		s.Fight.Flags[addr]++
		return what
	}
}

// prismatic 是奇異之光術：擲 `rand(1,9)` 隨機挑一個狀態代碼，
// 對 10 隻生效（原版 `ds:9FC2 = rand(1,9)`，擲到 7 再擲 `ds:9FC9` 選元素）。
func prismatic(s *Session, who int) string {
	if s.Fight == nil {
		return "不在戰鬥中。"
	}
	code := s.Rand.Range(1, 9)
	return applyStatus(s, who, 10, 0, code, "奇異之光術")
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
// perLevel 是「每級加 step、加到 cap 為止」那一型（鷹眼術、巫師眼）。
//
// 原版先做一次 `if 值 > cap { 值 = 0FFh }`，再跑等級次的迴圈，
// 迴圈裡每次都重新檢查 cap。所以已經滿的會被推到 255。
func perLevel(addr uint16, step, cap byte, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		v := s.World.Globals[addr]
		if v > cap {
			v = 0xFF
		}
		lv := 1
		if who >= 0 && who < len(s.Party) {
			lv = int(s.Party[who].Level)
		}
		for i := 0; i < lv; i++ {
			if v < cap {
				v += step
			}
		}
		s.setGlobalAddr(addr, v)
		return what
	}
}

// addCapped 是「一次加 step」那一型（持續照明術）。
func addCapped(addr uint16, step, cap byte, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		if v := s.World.Globals[addr]; v > cap {
			s.setGlobalAddr(addr, 0xFF)
		} else {
			s.setGlobalAddr(addr, v+step)
		}
		return what
	}
}

// healPerLevel 是強效治療：擲等級次 1d10 加總再治療。
func healPerLevel(sides int) func(*Session, int) string {
	return func(s *Session, who int) string {
		lv := 1
		if who >= 0 && who < len(s.Party) {
			lv = int(s.Party[who].Level)
		}
		n := 0
		for i := 0; i < lv; i++ {
			n += s.Rand.Range(1, sides)
		}
		return heal(n)(s, who)
	}
}

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
func damageSpell(hi, add, count, el int, what string) func(*Session, int) string {
	return damageSpellLo(1, hi, add, count, el, what)
}

// damageSpellLo 是下界不是 1 的版本（目前只有火焰枷鎖）。
func damageSpellLo(lo, hi, add, count, el int, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		if s.Fight == nil {
			return "不在戰鬥中。"
		}
		return applyDamage(s, who, count, el, what, func() int { return s.Rand.Range(lo, hi) + add })
	}
}

// applyDamage 對前 count 隻還站著的怪物各擲一次傷害。
//
// 多目標的 handler 是**逐隻重擲**的（原版在迴圈內再呼叫一次擲骰），
// 不是擲一次套用到全部。
//
// `sub_1714A` 的三層抗性都在這裡：
//
//  1. 抗魔法百分比非 0 時擲 `rand(施法者等級, 90)`，抗性大於擲值就整個擋下。
//  2. 屬性抗性是旗標，有就完全免疫。
//  3. 擲 `rand(1, 191)`，不超過**怪物編號**就把傷害減半。
func applyDamage(s *Session, who, count, el int, what string, roll func() int) string {
	lv := 1
	if who >= 0 && who < len(s.Party) {
		lv = int(s.Party[who].Level)
	}
	hit, total, resisted, halved := 0, 0, 0, 0
	for _, m := range s.Fight.Monsters {
		if hit >= count {
			break
		}
		if !m.CombatCondition().Acts() {
			continue
		}
		hit++
		if mm, ok := m.(*Monster); ok {
			if mm.MagicResist() > 0 && mm.MagicResist() > s.Rand.Range(lv, 90) {
				resisted++
				continue
			}
			// 第二層：屬性抗性是旗標，有就完全免疫（`sub_18674`）。
			if mm.ResistsElement(el) {
				resisted++
				continue
			}
		}
		dmg := roll()
		// 第三層：擲 rand(1,191)，不超過怪物編號就減半。
		// 怪物表照難度排序，所以編號本身就是強度 —— 越後面的怪
		// 越常吃到這個減半。編號 191 以上必定減半。
		if mm, ok := m.(*Monster); ok && s.Rand.Range(1, 191) <= mm.Def.Index {
			dmg >>= 1
			halved++
		}
		m.TakeDamage(dmg)
		total += dmg
	}
	if hit == 0 {
		return "沒有目標。"
	}
	if resisted == hit {
		return fmt.Sprintf("%s被擋下了。", what)
	}
	msg := fmt.Sprintf("%s對 %d 個目標造成 %d 點傷害", what, hit-resisted, total)
	if resisted > 0 {
		msg += fmt.Sprintf("，%d 個擋下了", resisted)
	}
	if halved > 0 {
		msg += fmt.Sprintf("，%d 個減半", halved)
	}
	return msg + "。"
}

// levelDamageSpell 是隨施法者等級累加的傷害（原版 `sub_1A82C`）：
//
//	ds:9FC6 = 0
//	n = 施法者記錄 +113（等級）
//	重複 n 次：ds:9FC6 += (sides 為 0 ? 0 : rand(1, sides)) + bonus
//
// 所以 `sides=0` 的那幾條是「每級固定 bonus 點」，其餘是
// 「每級擲一次 1d sides 再加 bonus」—— 逐級重擲，不是擲一次乘等級。
func levelDamageSpell(sides, bonus, count, el int, what string) func(*Session, int) string {
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
		return applyDamage(s, who, count, el, what, roll)
	}
}

// fixedDamageSpell 是傷害寫死的那幾條（handler 直接 `mov ds:9FC6, imm`）。
func fixedDamageSpell(dmg, count, el int, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		if s.Fight == nil {
			return "不在戰鬥中。"
		}
		return applyDamage(s, who, count, el, what, func() int { return dmg })
	}
}

// gravity 是扭曲重力術：傷害是**選定目標**目前生命的一半，
// 算一次之後套用到兩隻身上（原版 `ds:9FC6 = ds:9FAA[目標] >> 1`，
// 迴圈外算好，不逐隻重算）。
func gravity(s *Session, who int) string {
	if s.Fight == nil {
		return "不在戰鬥中。"
	}
	dmg := 0
	for _, m := range s.Fight.Monsters {
		if m.CombatCondition().Acts() {
			dmg = m.CombatHP() / 2
			break
		}
	}
	return applyDamage(s, who, 2, 0, "扭曲重力術", func() int { return dmg })
}

// statusSpell 是施加狀態的那一批。
//
// 三層抗性與傷害走同一條，差在第三層：代碼小於 9 的**完全擋下**，
// 代碼 9（粉碎術）改判 50 點傷害（原版 `ds:9FC6 = 0x32`、`ds:9FC3 = 0`）。
// 代碼 8（死亡之指）與 9 不設位元，直接判死。
func statusSpell(code, count, el int, what string) func(*Session, int) string {
	return func(s *Session, who int) string {
		if s.Fight == nil {
			return "不在戰鬥中。"
		}
		return applyStatus(s, who, count, el, code, what)
	}
}

func applyStatus(s *Session, who, count, el, code int, what string) string {
	lv := 1
	if who >= 0 && who < len(s.Party) {
		lv = int(s.Party[who].Level)
	}
	if count == 0 {
		count = 4 + lv // 手冊「4 個怪物＋1 個怪物／等級」，原版走 sub_1719E
	}
	hit, done, resisted := 0, 0, 0
	for _, m := range s.Fight.Monsters {
		if hit >= count {
			break
		}
		if !m.CombatCondition().Acts() {
			continue
		}
		hit++
		mm, ok := m.(*Monster)
		if !ok {
			continue
		}
		if mm.MagicResist() > 0 && mm.MagicResist() > s.Rand.Range(lv, 90) {
			resisted++
			continue
		}
		if mm.ResistsElement(el) {
			resisted++
			continue
		}
		if s.Rand.Range(1, 191) <= mm.Def.Index {
			// 第三層擋下；粉碎術例外，改判 50 點傷害。
			if code == 9 {
				mm.TakeDamage(50)
				done++
			} else {
				resisted++
			}
			continue
		}
		if code >= 8 {
			mm.TakeDamage(mm.CombatHP())
		} else {
			mm.AddStatus(StatusMask(code))
		}
		done++
	}
	if hit == 0 {
		return "沒有目標。"
	}
	if done == 0 {
		return fmt.Sprintf("%s被擋下了。", what)
	}
	return fmt.Sprintf("%s讓 %d 個目標%s。", what, done, StatusName(code))
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
