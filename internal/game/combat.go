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
	Party    []Combatant
	Monsters []Combatant
	Round    int
}

// Order 回傳這一回合的行動順序：速度高的先動，同速時隊伍優先。
//
// 手冊只說「最快的人物或怪物最先攻擊」，沒說同速怎麼辦；
// 這裡讓隊伍先動，是為了讓結果可重現 —— 真正的規則待反組譯確認。
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
func (c *Character) CombatName() string        { return c.Name }
func (c *Character) CombatSpeed() int          { return c.Current[Speed] }
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
