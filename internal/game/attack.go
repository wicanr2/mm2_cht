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
// 隊伍打怪物走的是**另一條路徑**（`sub_8E81`），形狀完全不同：
//
//	揮擊次數 = 等級 / 揮擊除數[職業] + 1        （ds:101A）
//	每次揮擊先擲 rand(1,100)：< 6 直接命中、6–8 直接落空、其餘正常判定
//	正常判定：上限 = min(25 + 等級 / 命中除數[職業], 250)   （ds:1012）
//	          擲 = rand(1, 上限) + 命中加成
//	          擲 > 255 命中；擲 ≤ 10 落空；否則要 ≥ 目標的防護等級
//	傷害 = rand(1, 武器骰) + 傷害加成，超過 250 就變成 1
//
// **武器的骰面數與加成還是假設的** —— 它們在記錄的 +76…+79，而那幾個
// 欄位是裝備算出來的，未裝備時全是 0，裝備欄本身還沒解。

// AttackResult 是一次攻擊的結果。
//
// 原版一個「攻擊動作」含好幾次攻擊（`Attacks` 次），每次各自判定命中，
// 所以這裡記的是總計，不是單次。
type AttackResult struct {
	Hit    bool
	Hits   int // 命中次數
	Swings int // 揮擊次數
	Damage int
	Target Condition
}

func (r AttackResult) String() string {
	if !r.Hit {
		return fmt.Sprintf("%d 次全部落空", r.Swings)
	}
	return fmt.Sprintf("%d 次裡命中 %d 次，造成 %d 點傷害", r.Swings, r.Hits, r.Damage)
}

// Attacker 是攻擊方需要提供的東西。
//
// 命中判定交給實作自己做 —— 怪物與角色走的是原版兩條不同的路徑，
// 硬湊成同一條公式就等於兩邊都不對。
type Attacker interface {
	CombatName() string
	// AttackSwings 是這一個動作揮擊幾次。
	AttackSwings() int
	// AttackDice 是每次命中的傷害骰面數，擲 rand(1, n)。
	AttackDice() int
	// AttackBonus 是加在每次傷害上的固定值。
	AttackBonus() int
	// Hits 判定一次揮擊有沒有命中。
	Hits(r *Rand, target Defender) bool
}

// Defender 是受擊方需要提供的數值。
type Defender interface {
	Combatant
	ArmorClass() int
}

// Resolve 解一次攻擊動作：逐次揮擊、各自判定命中、傷害累加。
func Resolve(r *Rand, a Attacker, d Defender) AttackResult {
	swings := a.AttackSwings()
	if swings < 1 {
		swings = 1
	}
	res := AttackResult{Swings: swings, Target: d.CombatCondition()}
	dice := a.AttackDice()
	if dice < 1 {
		dice = 1
	}
	for i := 0; i < swings; i++ {
		if !a.Hits(r, d) {
			continue
		}
		res.Hits++
		dmg := r.Range(1, dice) + a.AttackBonus()
		// 原版的上溢處理：超過 250 就當成 1，不是夾在 250。
		if dmg > 250 || dmg < 1 {
			dmg = 1
		}
		res.Damage += dmg
	}
	if res.Hits == 0 {
		return res
	}
	res.Hit = true
	res.Target = d.TakeDamage(res.Damage)
	return res
}

// AttackSwings 走原版的揮擊除數表（`ds:101A`）。
func (c *Character) AttackSwings() int { return c.AttacksPerRound() }

// AttackDice 是武器的傷害骰面數（記錄 +76）。
//
// **假設**：那個欄位是裝備算出來的，未裝備時是 0，而裝備欄還沒解。
// 沒有值時由力量推一個合理的骰面數，讓沒裝備的角色仍打得動。
func (c *Character) AttackDice() int {
	if d := c.WeaponDice; d > 0 {
		return d
	}
	if d := c.Current[Might] / 3; d >= 2 {
		return d
	}
	return 2
}

// AttackBonus 是傷害加成。**假設**，理由同 AttackDice。
func (c *Character) AttackBonus() int { return c.DamageBonus }

// Hits 是原版隊伍攻擊的命中判定（`2COMBAT.img` `sub_8E81` 那條路徑）。
func (c *Character) Hits(r *Rand, d Defender) bool {
	switch roll := r.Range(1, 100); {
	case roll < 6:
		return true // 5% 直接命中
	case roll < 9:
		return false // 3% 直接落空
	}
	base := 25
	if c.CondBits&0x01 != 0 {
		base = 3 // 狀況 bit0 會把命中上限壓到剩 3
	}
	div := 1
	if data != nil {
		div = data.AttackDivisorFor(int(c.Class))
	}
	limit := base + c.Level/div
	if limit > 250 {
		limit = 250
	}
	v := r.Range(1, limit) + c.HitBonus
	switch {
	case v > 255:
		return true
	case v <= 10:
		return false
	}
	return v >= d.ArmorClass()
}

// Monster 是戰鬥中的一隻怪物。數值全部來自怪物記錄的位元欄位。
type Monster struct {
	Def  monsters.Monster
	// Status 是狀態位元組，對應原版的 `ds:9F86[槽位]`。
	Status byte
	HP   int
	Cond Condition

	// Left 是這一輪還剩幾次行動（原版 `ds:9F9E[編號]`）。
	Left int

	// Display 是要顯示的名字，空的話用原文。
	// 在地化在這裡處理 —— 對戰報做字串取代會誤傷，
	// 怪物「Hermit（隱士）」的名字是角色「The Hermit」的一部分。
	Display string
}

// NewMonster 從怪物定義建一隻參戰的怪物。
func NewMonster(def monsters.Monster) *Monster {
	return &Monster{Def: def, HP: def.HP, Left: def.Actions}
}

func (m *Monster) CombatName() string {
	if m.Display != "" {
		return m.Display
	}
	return m.Def.Name
}

// CombatSpeed 用難度層當行動順序的依據。
//
// 原版**不排順序** —— 怪物是輪到誰就擲一次「這次行不行動」
// （見 CanAct 與 Actions）。這裡的排序是 remake 自己的，
// 只影響同一輪內的先後，不影響誰能行動幾次。
func (m *Monster) CombatSpeed() int { return m.Def.Tier }

// CanAct 判定這次輪到牠時能不能行動，並用掉一次額度。
//
// 額度來自 `2COMBAT.img` `0x1847E`：每隻怪物在 `ds:9F9E[編號]` 有一個
// 計數，初值是記錄 `+20` 的高 nibble 加一，行動一次減一（`0x184C5` 的
// `dec`），減到 0 就不再輪到牠（`0x184A8` 的 `cmp`）。
//
// **同一支函式還有一道擲骰沒解**：它先擲 `rand(1,100)`，與 `ds:9E25` 比，
// 大於就不行動。`ds:9E25` 來自 `ds:4DC0[(記錄+17 & 0xE0) >> 4]`，
// 而 `ds:4DC0` 只有八個位元組（0,10,20,35,50,75,90,100）——
// 那個索引卻會走到 0,2,…,14，一半落在表外的字串上。
// 照面值實作會讓 140 隻怪物（索引 0 → 0%）一整場都不動，
// 顯然不對，所以這一道**先不實作**，只保留額度。
func (m *Monster) CanAct(r *Rand) bool {
	if m.Left <= 0 {
		return false
	}
	m.Left--
	return true
}

// ResetRound 把這一輪的行動額度補回去。
func (m *Monster) ResetRound() { m.Left = m.Def.Actions }
func (m *Monster) CombatHP() int              { return m.HP }
func (m *Monster) CombatCondition() Condition {
	if m.Cond == CondGood && m.Status&MonCantAct != 0 {
		if m.Status&MonSlept != 0 {
			return CondAsleep
		}
		return CondUnconscious
	}
	return m.Cond
}

// AddStatus 把狀態位元 OR 進去（原版 `or ds:9F86[槽位], ds:1022[代碼-1]`）。
func (m *Monster) AddStatus(mask byte) { m.Status |= mask }
func (m *Monster) AttackSwings() int          { return m.Def.Attacks }
func (m *Monster) AttackDice() int            { return m.Def.DamageDice }
func (m *Monster) AttackBonus() int           { return 0 }

// ArmorClass 是怪物的防護等級，來自記錄第 22 個位元組的位元欄位。
func (m *Monster) ArmorClass() int { return m.Def.AC }

// 怪物的狀態位元（`ds:9F86[槽位]`）。位元遮罩表在 `ds:1022`，
// 名稱表在 `ds:0FEA`，兩張都用狀態代碼減一索引。
const (
	MonSilenced   = 0x02 // silenced，不能施法
	MonWeakened   = 0x04 // weakened
	MonFrightened = 0x08 // frightened
	MonSlept      = 0x10 // slept
	MonHeld       = 0x20 // held
	MonMindless   = 0x40 // mindless
	MonEncased    = 0x80 // encased

	// MonCantAct 是擋住行動的三個位元。原版在 `0x18582` 一次 test 0xB0。
	MonCantAct = MonSlept | MonHeld | MonEncased
)

// StatusMask 回傳狀態代碼對應的位元（代碼 1–7），代碼 8/9 沒有位元。
func StatusMask(code int) byte {
	if code < 1 || code > 7 {
		return 0
	}
	return byte(1) << uint(code)
}

// StatusName 是狀態代碼的名稱，順序照 `ds:0FEA` 的字串表。
func StatusName(code int) string {
	names := [...]string{"沈默", "衰弱", "驚嚇", "沈睡", "定身", "心智渙散", "封困", "死亡", "粉碎"}
	if code < 1 || code > len(names) {
		return "未知"
	}
	return names[code-1]
}

// ResistsElement 回報怪物對某個屬性是否免疫（屬性編號同
// `sub_1714A` 的第三個參數，0 表示無屬性、恆不免疫）。
func (m *Monster) ResistsElement(el int) bool {
	if el < 1 || el > len(m.Def.Resists) {
		return false
	}
	return m.Def.Resists[el-1]
}

// MagicResist 是抗魔法百分比，用記錄 `+17 >> 5` 查 `ds:4DC0`。
// 施法路徑擲 `rand(施法者等級, 90)`，抗性大於擲值就擋下整個法術。
func (m *Monster) MagicResist() int {
	if data == nil {
		return 0
	}
	return data.MonsterMagicResist(m.Def.MagicResistIndex)
}

// Hits 是原版怪物攻擊的命中判定（`sub_8398`）：命中率是百分比，
// 由難度層的門檻減掉目標的防護等級，保底 5%。
func (m *Monster) Hits(r *Rand, d Defender) bool {
	chance := 5
	if data != nil {
		chance = data.ToHitPercent(m.Def.Tier, d.ArmorClass())
	}
	// 原版擲的是 rand(10, 1009) 再整數除以 10，不是 rand(1,100)——
	// 兩者分佈不同，而隨機序列要與原版對得上就得照原樣擲。
	return chance >= r.Range(10, 1009)/10
}

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
