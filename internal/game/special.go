package game

import "fmt"

// 怪物的遠程／法術攻擊：三十二種，與近身攻擊命中後的附帶效果是**兩套系統**。
//
// 這一套的入口是 `2COMBAT.img` 的 `sub_18056`（播報）→ `sub_1B70C`（分派）。
// 它不靠靠近、直接對整隊發動，效果與傷害分配都自己一套。完整反組譯見
// `docs/re/09-2combat-map.md` §4。
//
// 三條主幹：
//
//	純傷害   碼 0、1、3–8、24、31 —— 傷害就是這隻怪目前的 HP
//	跳表     碼 2、9–30 —— 三十路，骰子、上狀況、抽資源各走各的
//	分配     sub_1B116 —— 一次只打 4–7 個位置，順序由四張表擲一張
//
// **群體攻擊不是「全隊都吃」**：位置由 `ds:1462` 四張順序表決定，
// 挑到空位或倒下的人那一格就白費，而且照樣算進次數。

// specialOrder 是傷害分配的順序表（`ds:1462`，4 組 × 8 格）。
//
// 擲 `rand(1,100)`：< 40 用第 0 組、< 60 第 1 組、< 80 第 2 組，其餘第 3 組。
// 第 1 組從隊尾打起 —— **前排不必然先受害**。
var specialOrder = [4][8]int{
	{0, 1, 2, 3, 4, 5, 6, 7},
	{7, 6, 5, 4, 3, 2, 1, 0},
	{0, 2, 4, 6, 1, 3, 5, 7},
	{1, 3, 5, 7, 0, 2, 4, 6},
}

// specialStatus 是上狀況的六對（`ds:1456` 的位元 → `ds:145C` 指到
// `ds:106E` 的訊息）。位元值與訊息直接配對，不靠順序推。
var specialStatus = []struct {
	bit           byte
	key, fallback string
}{
	{CondPetrified, "exe.0D1A", "turns to stone"},
	{CondBitAsleep, "exe.0CDB", "falls asleep"},
	{CondDeadBits, "exe.0D15", "dies"},
	{CondEradicated, "exe.0D29", "is eradicated!!"},
	{CondBitSilenced, "exe.0CF2", "is silenced"},
	{CondBitParalyzed, "exe.0CFE", "is paralyzed"},
}

// 播報用的固定字串。原版在 `ds:1482` 與 `ds:14AF` 各放了一份一模一樣的
// ` is not affected!`，這裡共用同一個鍵。
const (
	msgCasts       = "exe.1131" // " casts"
	msgNotAffected = "exe.14AF" // " is not affected!"
	msgResistedAnd = "exe.1494" // " resisted and"
	msgResisted    = "exe.14C1" // " resisted!"
	msgTakes       = "exe.14A2" // " takes "
	msgPts         = "exe.14AA" // " pts"
)

// specialText 取譯文；沒載入翻譯層時用原文（單元測試與工具會走這條）。
func specialText(key, fallback string) string {
	if text == nil {
		return fallback
	}
	return text.Or(key, fallback)
}

// special 是一次遠程／法術攻擊的執行狀態，欄位對應原版的全域值。
type special struct {
	e   *Encounter
	r   *Rand
	m   *Monster
	def SpecialAttack

	// dmg 是這一次的傷害累加器（`ds:9FD4`）。
	dmg int

	// 抗性的三個通道（`ds:154AC`／`ds:154A8`／`ds:154AD`）。非 0 表示
	// 這個通道還「活著」；判定過後留下非 0 的那一個決定播報哪一句。
	chLuck, chMagic, chElem byte

	log []string
}

// MonsterSpecial 執行一次怪物的遠程／法術攻擊，回傳播報。
//
// 呼叫端要先用 `Monster.UseSpecial` 決定這一次用不用；那一支只擲機率、
// 扣額度，真正發動在這裡。
func (e *Encounter) MonsterSpecial(r *Rand, m *Monster) []string {
	def, ok := Special(m.Def.SpecialIndex)
	if !ok {
		return nil
	}
	s := &special{e: e, r: r, m: m, def: def}
	s.announce()
	s.dispatch()
	return s.log
}

// announce 印出招式（`sub_18056`）。
//
// 碼 15–30（29 除外）先印 ` casts` —— 看起來像法術的那一段念咒，
// 吐息與凝視不念，狂暴夾在中間卻也不念。
func (s *special) announce() {
	k := s.m.Def.SpecialIndex
	line := s.m.CombatName()
	if k >= 0x0F && k <= 0x1E && k != 0x1D {
		line += specialText(msgCasts, " casts")
	}
	s.log = append(s.log, line+" "+specialText(s.def.Key, s.def.Announce)+"!")
}

// dispatch 是 `sub_1B70C`：先取三個抗性通道，再分純傷害與跳表兩段。
func (s *special) dispatch() {
	k := s.m.Def.SpecialIndex
	s.chLuck, s.chMagic, s.chElem = s.def.FlagA, s.def.FlagB, byte(s.def.Effect)

	// 純傷害那一段：吐息的威力就是這隻怪自己的 HP，編號 0xB3 以下折半。
	// 半血的龍吐出來的火只有滿血的一半。
	if k < 2 || (k >= 3 && k <= 8) || k == 24 || k == 31 {
		s.dmg = s.m.HP
		if s.m.Def.Index < 0xB3 {
			s.dmg /= 2
		}
		if s.dmg == 0 {
			s.dmg = 1
		}
		s.spread()
		return
	}

	switch k {
	case 2: // casts a curse：全隊的詛咒值 +1，神殿捐獻會清成 0
		if s.e.Protect.Curse < 0xFF {
			s.e.Protect.Curse++
		}
	case 9: // explodes：打完自爆身亡
		s.dmg = s.m.HP/2 + 1
		s.spread()
		s.selfDestruct()
	case 10: // gazes
		s.statusAll(CondPetrified)
	case 11: // drains magic：全隊目前 SP 歸零
		for i := range s.e.Party {
			if c := s.member(i); c != nil {
				c.setSP(0)
			}
		}
	case 12: // drains spell level
		for i := range s.e.Party {
			if c := s.member(i); c != nil && c.SL > 0 {
				c.setSL(c.SL - 1)
			}
		}
	case 13:
		s.vaporize()
	case 14:
		s.juggle()
	case 15: // energy blast
		s.dice(6)
		s.single()
	case 16: // sleep
		s.statusAll(CondBitAsleep)
	case 17, 18: // lightning bolts／fireballs
		s.dice(6)
		s.spread()
	case 19: // fingers of death
		s.statusOne(CondDeadBits)
	case 20: // disintegrate
		s.statusOne(CondEradicated)
	case 21: // super shock
		s.dice(40)
		s.single()
	case 22: // dancing sword
		s.dice(12)
		s.spread()
	case 23: // incinerate
		s.dice(50)
		s.single()
	case 25: // implosion：固定 1000
		s.dmg = 1000
		s.single()
	case 26: // inferno
		s.dice(20)
		s.spread()
	case 27: // pain
		s.dmg = s.r.Range(1, 15) + 1
		s.single()
	case 28: // silence
		s.statusAll(CondBitSilenced)
	case 29: // frenzies：用自己的傷害骰打完後自爆身亡
		for i := 0; i < s.m.Def.Attacks; i++ {
			s.dmg += s.r.Range(1, s.m.Def.DamageDice)
		}
		s.spread()
		s.selfDestruct()
	case 30: // paralyze
		s.statusAll(CondBitParalyzed)
	}
}

// dice 擲群體傷害（`sub_1B362`）。
//
// **顆數是怪物編號的高 4 位** —— 編號愈大的怪，同一招丟愈多顆骰子。
// 那是原版把強度直接綁在編號上的設計，不是另一張表。最後整個累加器
// 乘二加一。
func (s *special) dice(faces int) {
	for i := 0; i < s.m.Def.Tier; i++ {
		s.dmg += s.r.Range(1, faces)
	}
	s.dmg = s.dmg*2 + 1
}

// spread 把傷害分給隊伍（`sub_1B116`）。
func (s *special) spread() {
	n := s.r.Range(1, 4) + 3
	for i := range s.e.Party {
		if s.down(i) {
			n--
		}
	}
	if n > len(s.e.Party) {
		n = len(s.e.Party)
	}
	if n <= 0 {
		n = 1
	}

	roll := s.r.Range(1, 100)
	group := 0
	for _, th := range []int{40, 60, 80} {
		if roll >= th {
			group++
		}
	}
	order := specialOrder[group]

	// 進來時的三個通道每一輪都還原 —— 每個目標各擲各的抗性。
	luck, magic, elem := s.chLuck, s.chMagic, s.chElem
	for i := 0; i < n && i < len(order); i++ {
		s.chLuck, s.chMagic, s.chElem = luck, magic, elem
		pos := order[i]
		if pos >= len(s.e.Party) || s.down(pos) {
			continue // 空位或倒下的人：這一格白費，但照樣算一次
		}
		s.hit(pos)
	}
	s.chLuck, s.chMagic, s.chElem = luck, magic, elem
}

// single 打單一隨機隊員（`sub_18290` 挑人 ＋ `sub_1B226` 收尾）。
func (s *special) single() {
	if pos := s.pickAlive(); pos >= 0 {
		s.hit(pos)
	}
}

// pickAlive 擲一個還站著的隊伍位置（`sub_18290`／`sub_1B4CA` 共用的挑法）。
//
// 擲 `rand(1, 人數) − 1`，挑到倒下的就往後找，走到底繞回第 0 個。
// 全隊都倒下時回 −1（原版在這裡會無限迴圈，但戰鬥早就結束了）。
func (s *special) pickAlive() int {
	n := len(s.e.Party)
	if n == 0 {
		return -1
	}
	pos := s.r.Range(1, n) - 1
	for i := 0; i < n; i++ {
		if !s.down(pos) {
			return pos
		}
		pos++
		if pos >= n {
			pos = 0
		}
	}
	return -1
}

// hit 是單一目標的收尾（`sub_1B226` → `sub_17E10` → root `sub_13928`）。
func (s *special) hit(pos int) {
	c := s.member(pos)
	if c == nil {
		return
	}
	// 傷害累加器每個目標各算各的，算完要還原 —— 下一個人吃的是同一份基數。
	base := s.dmg
	dmg := s.applyResist(c, base)

	name := c.CombatName()
	switch {
	case s.chMagic != 0:
		s.log = append(s.log, name+specialText(msgNotAffected, " is not affected!"))
		s.dmg = base
		return
	case s.chLuck != 0 || s.chElem != 0:
		name += specialText(msgResistedAnd, " resisted and")
	}
	s.log = append(s.log, fmt.Sprintf("%s%s%d%s", name,
		specialText(msgTakes, " takes "), dmg, specialText(msgPts, " pts")))
	if c.TakeDamage(dmg) == CondDead {
		s.e.Lost++
	}
	s.dmg = base
}

// applyResist 把兩道護罩與三個抗性通道套上去，回傳真正要扣的傷害。
//
// 護罩在 `sub_17E10`（強力護罩一律減半、防護罩再減半），三個通道在
// root 的 `sub_13928`：
//
//	抗魔法通道擋下 → 完全不受影響
//	幸運通道擋下   → 傷害減半，播報 ` resisted and`
//	元素通道擋下   → 傷害再除以四
//
// 通道沒擋下就把該旗標清成 0；播報看的是最後還留著非 0 的那一個。
func (s *special) applyResist(c *Character, dmg int) int {
	if s.e.Protect.PowerShield > 0 {
		dmg /= 2
	}
	// `ds:154A4` 是「這一次是射擊」，怪物的特殊攻擊不設它 ——
	// 每個怪物動作結束時 `sub_184FE` 都會把它清成 0，所以防護罩生效。
	if s.e.Protect.Shield > 0 {
		dmg /= 2
	}

	if s.chMagic != 0 {
		if c.Resist[ResistMagic]+s.e.Protect.MagicBonus >= s.r.Range(1, 100) {
			return dmg
		}
	}
	s.chMagic = 0

	if s.chLuck != 0 {
		if s.luckSave(c) {
			return dmg / 2
		}
		s.chLuck = 0
	}

	if s.chElem != 0 && s.chElem != specialNoResist {
		if int(c.FieldByte(int(s.chElem)))+s.e.Protect.ElementBonus >= s.r.Range(1, 100) {
			return dmg / 4
		}
	}
	s.chElem = 0
	return dmg
}

// specialNoResist 是 `ds:1436` 的哨兵值：這一項不吃元素抗性。
const specialNoResist = 0x63

// luckSave 是第二個抗性通道（root `sub_138A8`）：一道幸運加等級的豁免。
//
//	擲 rand(1,100)
//	≤ 5   → 一定擋不住
//	≥ 95  → 一定擋下
//	其餘  → 等級 + 幸運修正 ≥ rand(1, 等級 + 20) 就擋下
func (s *special) luckSave(c *Character) bool {
	roll := s.r.Range(1, 100)
	if roll <= 5 {
		return false
	}
	if roll >= 0x5F {
		return true
	}
	target := s.r.Range(1, c.Level+20)
	bonus := 0
	if data != nil {
		bonus = data.StatBonus(c.Current[Luck])
	}
	v := c.Level + bonus
	if v < c.Level {
		v = 2 // 溢位保險，原版一樣
	}
	return v >= target
}

// statusAll 對每個隊員各擲一次（`sub_1B3A8`）。**不是一擲定全隊。**
func (s *special) statusAll(bit byte) {
	luck, magic, elem := s.chLuck, s.chMagic, s.chElem
	for i := range s.e.Party {
		s.chLuck, s.chMagic, s.chElem = luck, magic, elem
		s.status(i, bit)
	}
	s.chLuck, s.chMagic, s.chElem = luck, magic, elem
}

// statusOne 對隨機一名隊員上狀況（`sub_1B4CA`）。
func (s *special) statusOne(bit byte) {
	if pos := s.pickAlive(); pos >= 0 {
		s.status(pos, bit)
	}
}

// status 是上狀況的本體（`sub_1B410`）。
//
// 已經倒下的人直接算「不受影響」；沒擋下就把位元 OR 進狀況位元組，
// 再查那六對印訊息。
func (s *special) status(pos int, bit byte) {
	c := s.member(pos)
	if c == nil {
		return
	}
	name := c.CombatName()
	if s.down(pos) {
		s.log = append(s.log, name+specialText(msgNotAffected, " is not affected!"))
		return
	}
	switch s.statusResist(c) {
	case resistMagic:
		s.log = append(s.log, name+specialText(msgNotAffected, " is not affected!"))
		return
	case resistOther:
		s.log = append(s.log, name+specialText(msgResisted, " resisted!"))
		return
	}

	c.setCond(c.CondBits | bit)
	for _, st := range specialStatus {
		if st.bit == bit {
			s.log = append(s.log, name+" "+specialText(st.key, st.fallback))
		}
	}
	if c.Condition == CondDead {
		s.e.Lost++
	}
}

// 上狀況那條路的三種結果。
type resistOutcome int

const (
	resistNone  resistOutcome = iota // 沒擋下
	resistMagic                      // 抗魔法通道擋下
	resistOther                      // 幸運或元素通道擋下
)

// statusResist 是 `sub_1B2DE`：一擲決定，三個通道依序試，第一個擋下的就結束。
//
// 原版在第三個通道把表值減了 `0x11` 才當偏移用，讀到的是名字尾巴與
// 性別、陣營那一段（傷害那條路的 root `sub_13928` 會把 `0x11` 加回去，
// 讀的是對的欄位）。**remake 兩條路都讀對的抗性欄位**，理由與差異
// 記在 `docs/polish-spec.md`。
func (s *special) statusResist(c *Character) resistOutcome {
	roll := s.r.Range(1, 100)
	if s.chMagic != 0 {
		if c.Resist[ResistMagic]+s.e.Protect.MagicBonus >= roll {
			return resistMagic
		}
	}
	s.chMagic = 0
	if s.chLuck != 0 {
		if s.luckSave(c) {
			return resistOther
		}
		s.chLuck = 0
	}
	if s.chElem != 0 && s.chElem != specialNoResist {
		if int(c.FieldByte(int(s.chElem)))+s.e.Protect.ElementBonus >= roll {
			return resistOther
		}
	}
	s.chElem = 0
	return resistNone
}

// vaporize 是碼 13（`sub_1B5C8`）：擲一次 `rand(1,100)`，> 65 才發作，
// 發作就整隊一起清 —— 名冊索引 24 以上（雇傭兵）清寶石，其餘清金錢。
//
// remake 沒有記錄「這個位置是不是雇傭兵」，所以一律走清金錢那一條。
func (s *special) vaporize() {
	if s.r.Range(1, 100) <= 65 {
		return
	}
	for i := range s.e.Party {
		if c := s.member(i); c != nil {
			c.setGold(0)
		}
	}
}

// juggle 是碼 14（`sub_1B62E`）：先造 3 面骰的群體傷害，再把隊伍位置
// 隨機對調 `rand(1,8)` 次。**不足三人就不換。**
func (s *special) juggle() {
	times := s.r.Range(1, 8)
	s.dice(3)
	s.spread()
	if len(s.e.Party) < 3 {
		return
	}
	for i := 0; i < times; i++ {
		a := s.r.Range(1, len(s.e.Party)) - 1
		b := s.r.Range(1, len(s.e.Party)) - 1
		s.e.Party[a], s.e.Party[b] = s.e.Party[b], s.e.Party[a]
	}
}

// selfDestruct 讓這隻怪打完就倒（碼 9 自爆、碼 29 狂暴）。
func (s *special) selfDestruct() {
	if s.m.Cond == CondDead {
		return
	}
	s.m.HP = 0
	s.m.Cond = CondDead
	s.e.recordDefeat(s.r, s.m)
	s.e.Killed++
}

// member 取隊伍第 i 個位置的角色；不是角色（測試用的假戰鬥員）就回 nil。
func (s *special) member(i int) *Character {
	if i < 0 || i >= len(s.e.Party) {
		return nil
	}
	c, _ := s.e.Party[i].(*Character)
	return c
}

// down 回報這個位置算不算倒下（原版 `記錄 +38 >= 0x80`：無意識、死亡、
// 石化、抹消都算）。
func (s *special) down(i int) bool {
	if i < 0 || i >= len(s.e.Party) {
		return true
	}
	if c, ok := s.e.Party[i].(*Character); ok {
		return c.CondBits >= 0x80 || c.Condition == CondDead ||
			c.Condition == CondUnconscious
	}
	return !s.e.Party[i].CombatCondition().Acts()
}
