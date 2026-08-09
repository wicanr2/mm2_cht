package game

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
)

// 攻擊解析。
//
// 公式抄自 `2COMBAT.img` 的 `sub_8398`（怪物攻擊隊伍那條路徑）：
//
//	命中率 = 命中門檻[攻擊者難度層] − 目標的防護等級
//	         目標防護高於門檻時固定 5%
//	每次攻擊各擲一次 rand(10, 1009)/10（＝ 1–100），命中率 ≥ 擲出值即命中
//	命中的傷害 = rand(1, 骰面數)，逐次累加
//	攻擊者旗標 bit3 → 命中率減半；bit2 → 總傷害減半
//
// 攻擊次數與骰面數來自怪物記錄的位元欄位（`internal/assets/monsters`），
// 命中門檻表在 `data/combat.json`。擲骰走原版那顆 RNG。
//
// **隊伍打怪物那條路徑的公式還沒解出來。** 訊息端已定位（`0x8c78` 印
// 「背刺」「暴擊」），計算端還沒。這裡沿用同一個形狀 ——
// 百分比命中、保底 5% —— 但攻擊者的門檻改由等級推，標為假設。

// AttackResult 是一次攻擊的結果。
//
// 原版一個「攻擊動作」含好幾次攻擊（`Attacks` 次），每次各自判定命中，
// 所以這裡記的是總計，不是單次。
type AttackResult struct {
	Hit    bool
	Hits   int // 命中次數
	Swings int // 揮擊次數
	Damage int
	Chance int // 這一次的命中率（百分比），方便對照與除錯
	Target Condition
}

func (r AttackResult) String() string {
	if !r.Hit {
		return fmt.Sprintf("%d 次全部落空（命中率 %d%%）", r.Swings, r.Chance)
	}
	return fmt.Sprintf("%d 次裡命中 %d 次，造成 %d 點傷害", r.Swings, r.Hits, r.Damage)
}

// Attacker 是攻擊方需要提供的數值。
type Attacker interface {
	CombatName() string
	// AttackTier 是查命中門檻用的難度層（0–15）。
	AttackTier() int
	// AttackSwings 是這一個動作揮擊幾次。
	AttackSwings() int
	// AttackDice 是每次命中的傷害骰面數，擲 rand(1, n)。
	AttackDice() int
}

// Defender 是受擊方需要提供的數值。
type Defender interface {
	Combatant
	ArmorClass() int
}

// Resolve 解一次攻擊動作。
func Resolve(r *Rand, a Attacker, d Defender) AttackResult {
	swings := a.AttackSwings()
	if swings < 1 {
		swings = 1
	}
	chance := 5
	if data != nil {
		chance = data.ToHitPercent(a.AttackTier(), d.ArmorClass())
	}
	res := AttackResult{Swings: swings, Chance: chance, Target: d.CombatCondition()}

	dice := a.AttackDice()
	if dice < 1 {
		dice = 1
	}
	for i := 0; i < swings; i++ {
		// 原版擲的是 rand(10, 1009) 再整數除以 10，不是 rand(1,100)——
		// 兩者分佈不同，而隨機序列要與原版對得上就得照原樣擲。
		if chance < r.Range(10, 1009)/10 {
			continue
		}
		res.Hits++
		res.Damage += r.Range(1, dice)
	}
	if res.Hits == 0 {
		return res
	}
	res.Hit = true
	res.Target = d.TakeDamage(res.Damage)
	return res
}

// Character 的攻守數值。
//
// **攻擊面是假設**：原版隊伍攻擊的計算還沒解出來，這裡用等級推難度層，
// 讓強弱關係成立。防守面則是原版的 —— 防護等級就是記錄的 `+36`。
func (c *Character) AttackTier() int {
	tier := c.Level / 4
	if tier > 15 {
		tier = 15
	}
	return tier
}

// AttackSwings 走原版的職業除數表（`sub_18DAA`）。
func (c *Character) AttackSwings() int { return c.AttacksPerRound() }

// AttackDice 是武器的傷害骰。**假設**：裝備欄還沒解出來，
// 這裡由力量推一個合理的骰面數。
func (c *Character) AttackDice() int {
	d := c.Current[Might] / 3
	if d < 2 {
		d = 2
	}
	return d
}

// Monster 是戰鬥中的一隻怪物。數值全部來自怪物記錄的位元欄位。
type Monster struct {
	Def  monsters.Monster
	HP   int
	Cond Condition

	// Display 是要顯示的名字，空的話用原文。
	// 在地化在這裡處理 —— 對戰報做字串取代會誤傷，
	// 怪物「Hermit（隱士）」的名字是角色「The Hermit」的一部分。
	Display string
}

// NewMonster 從怪物定義建一隻參戰的怪物。
func NewMonster(def monsters.Monster) *Monster {
	return &Monster{Def: def, HP: def.HP}
}

func (m *Monster) CombatName() string {
	if m.Display != "" {
		return m.Display
	}
	return m.Def.Name
}

// CombatSpeed 用難度層當行動順序的依據。**假設**：怪物的速度欄位
// 還沒從那 12 個位元組裡指認出來。
func (m *Monster) CombatSpeed() int           { return m.Def.Tier }
func (m *Monster) CombatHP() int              { return m.HP }
func (m *Monster) CombatCondition() Condition { return m.Cond }
func (m *Monster) AttackTier() int            { return m.Def.Tier }
func (m *Monster) AttackSwings() int          { return m.Def.Attacks }
func (m *Monster) AttackDice() int            { return m.Def.DamageDice }

// ArmorClass 是怪物的防護等級。**假設**：對應的位元欄位還沒指認出來，
// 由難度層推。
func (m *Monster) ArmorClass() int { return m.Def.Tier }

// TakeDamage 與角色同一套規則。
func (m *Monster) TakeDamage(n int) Condition {
	if m.Cond == CondDead {
		return CondDead
	}
	m.HP -= n
	if m.HP <= 0 {
		m.HP = 0
		m.Cond = CondDead // 怪物沒有無意識這一段
	}
	return m.Cond
}

// ArmorClass 是防護等級，直接讀記錄的 +36。
func (c *Character) ArmorClass() int { return c.AC }
