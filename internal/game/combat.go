package game

import (
	"fmt"
	"sort"
)

// 戰鬥的流程規則出自手冊（docs/manual/part-2.md §2.11）：
//
//   - 速度決定行動順序，最快的人物或怪物先攻。
//   - 力量影響傷害、準確度影響命中、耐力影響生命點數。
//   - 生命點數歸零時失去意識，之後再受任何傷害即死亡。
//
// 傷害與命中的**實際公式尚未從 `2COMBAT.OVL` 解出**，怪物記錄裡那 12 個
// 位元組的語意也還沒定（見 docs/formats/02-data-files.md §6）。
// 所以這一層只固定「流程」——誰先動、指令有哪些、狀態怎麼轉移 ——
// 數值交給 Combatant 介面提供，等公式解出來再換掉實作，流程不必動。

// Condition 是身體狀況。名稱用手冊的官方譯法。
type Condition byte

const (
	CondGood Condition = iota
	CondPoisoned
	CondAsleep
	CondUnconscious
	CondDead
)

var condNames = [...]string{"正常", "中毒", "沈睡", "無意識", "死亡"}

func (c Condition) String() string {
	if int(c) >= len(condNames) {
		return "未知"
	}
	return condNames[c]
}

// Acts 回報這個狀況還能不能行動。
func (c Condition) Acts() bool { return c == CondGood || c == CondPoisoned }

// Combatant 是戰鬥中的一方。角色與怪物都實作它。
type Combatant interface {
	CombatName() string
	CombatSpeed() int
	CombatHP() int
	CombatCondition() Condition
	// TakeDamage 扣血並回傳扣完之後的狀況。
	TakeDamage(n int) Condition
}

// CombatCommand 是戰鬥指令。按鍵與手冊的指令表對得上，
// 分派表在 `2COMBAT.OVL` 的 jpt_19573（`sub ax, 46h` 之後 17 個 case）。
type CombatCommand byte

const (
	CmdFight    CombatCommand = 'F' // 戰鬥（近戰）
	CmdShoot    CombatCommand = 'S' // 射擊
	CmdCast     CombatCommand = 'C' // 施法
	CmdUse      CombatCommand = 'U' // 使用物品
	CmdBlock    CombatCommand = 'B' // 抵擋
	CmdRun      CombatCommand = 'R' // 溜跑
	CmdExchange CombatCommand = 'E' // 對調
	CmdView     CombatCommand = 'V' // 檢視
	CmdProtect  CombatCommand = 'P' // 顯示防護效能
)

var cmdNames = map[CombatCommand]string{
	CmdFight: "戰鬥", CmdShoot: "射擊", CmdCast: "施法", CmdUse: "使用物品",
	CmdBlock: "抵擋", CmdRun: "溜跑", CmdExchange: "對調", CmdView: "檢視",
	CmdProtect: "防護",
}

func (c CombatCommand) String() string {
	if s, ok := cmdNames[c]; ok {
		return s
	}
	return string(rune(c))
}

// 已從反組譯確認有 handler 的指令。其餘的按鍵在跳表裡，
// 但對應的 handler 還沒逐一讀過。
var confirmedCommands = []CombatCommand{CmdFight, CmdShoot, CmdRun, CmdUse, CmdProtect}

// ConfirmedCommands 回傳已在 2COMBAT.OVL 找到 handler 的指令。
func ConfirmedCommands() []CombatCommand {
	out := make([]CombatCommand, len(confirmedCommands))
	copy(out, confirmedCommands)
	return out
}

// Encounter 是一場遭遇戰。
type Encounter struct {
	// Flags 是戰鬥期間的一次性旗標，鍵是原版的 DGROUP 位址
	// （`ds:9FC0`–`ds:9FCD`）。開戰時是空的，戰鬥結束就丟掉。
	Flags map[uint16]byte

	Party    []Combatant
	Monsters []Combatant
	Round    int

	// Front 是這一波實際上場、打得到的怪物數（原版 `ds:9FC5`）。
	// 場上可以有上百隻，前排只有這幾隻 —— 死一隻就要重新夾。
	Front int
}

// MaxFront 是前排的上限。原版 `0x196BB` 把 `ds:9FC5` 夾在 10。
// 名單面板一次也只顯示十行，兩者同源。
const MaxFront = 10

// NextActor 挑出這一輪下一個行動的人，回傳它與「是不是隊伍這一邊」。
//
// 原版的排程（`0x1A1CC` 與 `0x1A200` 兩個迴圈）：
//
//	怪物：在前十隻裡跳過 ds:5480[i] != 0（這一輪動過了）的，取 ds:9F92[i] 最大
//	角色：在全隊裡跳過 ds:548C[i] != 0 的，取記錄 +110 最大
//	兩邊的最大值再比：角色的 >= 怪物的就角色先動
//
// 兩個計數陣列在每一輪開頭清零（`0x1A123` 與 `0x1A138`）。
// acted 記的就是那兩個陣列；回傳 ok = false 表示這一輪沒人可動了。
func (e *Encounter) NextActor(actedParty, actedMonsters map[int]bool) (i int, party, ok bool) {
	best, bi := 0, -1
	for k, c := range e.Monsters {
		if k >= MaxFront || actedMonsters[k] {
			continue
		}
		if v := c.CombatSpeed(); v > best {
			best, bi = v, k
		}
	}
	pbest, pi := 0, -1
	for k, c := range e.Party {
		if actedParty[k] {
			continue
		}
		if v := c.CombatSpeed(); v > pbest {
			pbest, pi = v, k
		}
	}
	switch {
	case pi >= 0 && pbest >= best:
		return pi, true, true
	case bi >= 0:
		return bi, false, true
	case pi >= 0:
		return pi, true, true
	}
	return 0, false, false
}

// RollFront 決定這一波前排有幾隻（原版 `sub_19640`）。
//
//	室外：rand(10, 39) / 10 + 3            → 4–6
//	室內：rand(10, 69) / 10 + 隊伍人數 / 2  → 1–6 加人數的一半
//
// 接著依難度旗標 `ds:0415` 調整（2 減半、3 加倍），最後夾在
// `[0, min(場上總數, 10)]`。difficulty 給 2 或 3 以外的值就不調整。
func (e *Encounter) RollFront(r *Rand, indoor bool, difficulty int) int {
	var n int
	if indoor {
		n = r.Range(10, 69)/10 + len(e.Party)/2
	} else {
		n = r.Range(10, 39)/10 + 3
	}
	switch difficulty {
	case 2:
		n /= 2
	case 3:
		n *= 2
	}
	e.Front = n
	e.clampFront()
	return e.Front
}

// clampFront 把前排夾回上限：不超過場上總數，也不超過 MaxFront。
func (e *Encounter) clampFront() {
	if e.Front > len(e.Monsters) {
		e.Front = len(e.Monsters)
	}
	if e.Front > MaxFront {
		e.Front = MaxFront
	}
	if e.Front < 0 {
		e.Front = 0
	}
}

// RemoveMonster 把第 i 隻怪物從場上刪掉，後面的往前搬。
//
// 原版 `sub_18A22`：`ds:0508`（場上怪物數）減一，然後從死掉那一格起
// 把六個平行陣列各往前搬一格 —— 編號 `ds:9680`、狀態 `ds:9F86`、
// `ds:9F92`、剩餘行動 `ds:9F9E`、HP `ds:9FAA`（word）、`ds:5480`。
// 搬完重新夾 `ds:9FC5`（前排數）並重畫名單。
//
// **原版沒有屍體** —— 死掉與逃走走同一條路（`sub_18AF4`），
// 差別只在印哪一句話。所以之後 `Monsters[i]` 指的是別隻怪，
// 拿著索引跨越死亡事件是錯的。
func (e *Encounter) RemoveMonster(i int) bool {
	if i < 0 || i >= len(e.Monsters) {
		return false
	}
	e.Monsters = append(e.Monsters[:i], e.Monsters[i+1:]...)
	e.clampFront()
	return true
}

// Reap 把已經倒下的怪物一次清掉，回傳清掉幾隻。
//
// 原版是「誰死了就當場移除」，沒有集中清理這一步；這裡提供它是
// 為了讓上層在一次群體法術之後把場面收乾淨，語意與逐隻呼叫
// RemoveMonster 相同。
func (e *Encounter) Reap() int {
	n := 0
	for i := len(e.Monsters) - 1; i >= 0; i-- {
		if !e.Monsters[i].CombatCondition().Acts() {
			e.RemoveMonster(i)
			n++
		}
	}
	return n
}

// TryFlee 判定第 i 隻怪物這一輪要不要逃走，要的話當場移除。
//
// 原版 `0x1858C`：`ds:9FC4`（禁逃旗標）為 0 時，拿這隻怪的士氣門檻
// 與 `ds:0FC2`（隊伍最高等級的一半）比，門檻**小於**它才可能逃 ——
// 隊伍越強，弱怪越留不住。過了這關再擲 `rand(1,100) <= 50`。
//
// 士氣層 3 的門檻是 255，`ds:0FC2` 是等級的一半、上限 127，
// 所以那一層的怪物永遠不逃。
func (e *Encounter) TryFlee(r *Rand, i int, blocked bool) bool {
	if blocked || i < 0 || i >= len(e.Monsters) {
		return false
	}
	m, ok := e.Monsters[i].(*Monster)
	if !ok {
		return false
	}
	threshold := 255
	if data != nil {
		threshold = data.FleeThreshold(m.Def.MoraleTier)
	}
	if threshold >= e.partyPower() {
		return false
	}
	if r.Range(1, 100) > 50 {
		return false
	}
	e.RemoveMonster(i)
	return true
}

// TryRun 是戰鬥指令 `R`（溜跑）：第 i 名角色擲一次，成功就脫離戰鬥。
//
// `sub_1914A`：擲 `rand(1, 100)`，**小於**這張地圖的 `ATTRIB +13`
// 才算成功。跑掉的是**下指令的那一個人**，不是整隊 —— 其他人留在
// 場上繼續打。chance 由 `MapAttr.RunChance()` 提供。
func (e *Encounter) TryRun(r *Rand, i, chance int) bool {
	if i < 0 || i >= len(e.Party) {
		return false
	}
	if r.Range(1, 100) >= chance {
		return false
	}
	e.RemoveMember(i)
	return true
}

// RemoveMember 把第 i 名角色移出戰鬥。
//
// 原版用的是**把最後一格搬進空出來的那一格**（`0x1917C`），不是像
// 怪物那樣整批往前搬 —— 所以隊伍在戰鬥中的順序會變。同一支 overlay
// 裡兩種移除法並存，照抄才會與原版的後續判定一致。
func (e *Encounter) RemoveMember(i int) bool {
	if i < 0 || i >= len(e.Party) {
		return false
	}
	last := len(e.Party) - 1
	if i != last {
		e.Party[i] = e.Party[last]
	}
	e.Party = e.Party[:last]
	return true
}

// partyPower 是 `ds:0FC2`：隊伍裡最高等級的一半（`sub_1974C`）。
func (e *Encounter) partyPower() int {
	best := 0
	for _, c := range e.Party {
		ch, ok := c.(*Character)
		if !ok {
			continue
		}
		if v := ch.Level / 2; v > best {
			best = v
		}
	}
	return best
}

// 死亡與逃走的播報。兩句都在 EXE 尾部的字串表裡，
// 由 `sub_18AB8` 依 `ds:54A6` 二選一，前面接怪物名。
const (
	msgGoesDown = "exe.1197" // " goes down!"
	msgRunsAway = "exe.118B" // " runs away!"
)

// LeaveMessage 組出某隻怪物離場的播報：逃走是 fled，倒下是 !fled。
func LeaveMessage(name string, fled bool) string {
	key, fallback := msgGoesDown, " goes down!"
	if fled {
		key, fallback = msgRunsAway, " runs away!"
	}
	if text == nil {
		return name + fallback
	}
	return name + text.Or(key, fallback)
}

// Order 回傳這一回合的行動順序：行動鍵大的先動，平手時隊伍優先。
//
// 原版**不預先排序**：每次要挑下一個行動者時，在「這一輪還沒動過」的
// 裡面各自取最大（怪物 `0x1A1CC`、角色 `0x1A200`），兩邊的最大值再比一次，
// **平手時角色先動**（`0x1A243` 的 `jb`）。NextActor 照那個做法；
// Order 是它的整輪展開，只在同鍵時的先後上與原版一致。
func (e *Encounter) Order() []Combatant {
	type slot struct {
		c     Combatant
		party bool
	}
	all := make([]slot, 0, len(e.Party)+len(e.Monsters))
	for _, c := range e.Party {
		all = append(all, slot{c, true})
	}
	for _, c := range e.Monsters {
		all = append(all, slot{c, false})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].c.CombatSpeed() != all[j].c.CombatSpeed() {
			return all[i].c.CombatSpeed() > all[j].c.CombatSpeed()
		}
		return all[i].party && !all[j].party
	})
	out := make([]Combatant, 0, len(all))
	for _, s := range all {
		if s.c.CombatCondition().Acts() {
			out = append(out, s.c)
		}
	}
	return out
}

// Over 回報戰鬥是否結束：一方全倒就結束。
func (e *Encounter) Over() bool {
	return countActive(e.Party) == 0 || countActive(e.Monsters) == 0
}

// PartyWon 只在怪物全倒、而隊伍還有人站著時為真。
func (e *Encounter) PartyWon() bool {
	return countActive(e.Monsters) == 0 && countActive(e.Party) > 0
}

func countActive(cs []Combatant) int {
	n := 0
	for _, c := range cs {
		if c.CombatCondition().Acts() {
			n++
		}
	}
	return n
}

// CombatName 等方法讓 Character 直接參戰。
func (c *Character) CombatName() string { return c.Name }

// CombatSpeed 是排行動順序用的值。
//
// 原版讀的是記錄 `+110`（`0x1A216` 的 `[bx+0x6e]`），也就是**屬性區
// 第四格的當前值**，而不是第五格。同一格也是防護等級加成的來源
// （root `sub_14F3A` 讀基礎那一份的 `+19`）。
//
// ⚠ 那一格叫什麼**還沒定案**。EXE 的標籤表（`ds:07A8` 起，18 bytes
// 一格）順序是 Might／Intellect／Personality／Endurance／Speed／
// Accuracy，而角色卡把六格依序印出來（欄位 id 4、8、9、15、18、19
// 遞增），照這個對法第四格是 Endurance；但中文手冊說防護等級與
// 出手順序**都由速度決定**（part-2 p.292、p.438）。兩邊衝突時
// 以程式碼為準：**讀第四格**，名稱先維持現狀不動。
func (c *Character) CombatSpeed() int { return c.Current[Endurance] }
func (c *Character) CombatHP() int             { return c.HP }
func (c *Character) CombatCondition() Condition { return c.Condition }

// TakeDamage 依手冊的規則扣血：歸零時失去意識，已經無意識再受傷就死亡。
func (c *Character) TakeDamage(n int) Condition {
	if c.Condition == CondDead {
		return CondDead
	}
	if c.Condition == CondUnconscious {
		c.Condition = CondDead
		return c.Condition
	}
	c.HP -= n
	if c.HP <= 0 {
		c.HP = 0
		c.Condition = CondUnconscious
	}
	return c.Condition
}

// Fight 打完一整場，回傳每一回合的過程。
//
// 每回合照速度順序輪一次，各自攻擊對面第一個還站著的目標。
// 這是**最簡單的目標選擇**，原版會依隊形與指令決定 —— 那部分還沒解。
func (e *Encounter) Fight(r *Rand, maxRounds int) []string {
	var log []string
	for e.Round = 1; e.Round <= maxRounds && !e.Over(); e.Round++ {
		// 每輪開頭把怪物的行動額度補回去（原版把 ds:9F9E 重設）。
		for _, m := range e.Monsters {
			if mm, ok := m.(*Monster); ok {
				mm.ResetRound()
			}
		}
		for _, c := range e.Order() {
			if !c.CombatCondition().Acts() {
				continue
			}
			// 怪物每次輪到都要先擲「這次行不行動」。
			if m, ok := c.(*Monster); ok && !m.CanAct(r) {
				continue
			}
			foes := e.Monsters
			if !e.isParty(c) {
				foes = e.Party
			}
			target := firstStanding(foes)
			if target == nil {
				break
			}
			a, okA := c.(Attacker)
			d, okD := target.(Defender)
			if !okA || !okD {
				continue
			}
			res := Resolve(r, a, d)
			line := fmt.Sprintf("第 %d 回合　%s → %s：%s",
				e.Round, c.CombatName(), target.CombatName(), res)
			if res.Hit && res.Target == CondDead {
				line += "（倒下）"
			}
			log = append(log, line)
			if e.Over() {
				break
			}
		}
	}
	return log
}

func (e *Encounter) isParty(c Combatant) bool {
	for _, p := range e.Party {
		if p == c {
			return true
		}
	}
	return false
}

func firstStanding(cs []Combatant) Combatant {
	for _, c := range cs {
		if c.CombatCondition().Acts() {
			return c
		}
	}
	return nil
}
