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
//	攻擊者狀態位元組 ds:9F86 的 bit3（驚嚇）→ 命中率減半
//	                            bit2（衰弱）→ 動作結束後總傷害減半
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

// halvesDamage 是「總傷害減半」這個可選能力。
//
// 原版只有怪物那條路徑有（`0x18447`：狀態位元組 `ds:9F86` 的 bit2 ＝
// 衰弱），而且是**整個攻擊動作結束後對總和減半**，不是每次揮擊各減 ——
// 傷害是奇數時兩種算法的結果不同。
type halvesDamage interface{ DamageHalved() bool }

// Resolve 解一次攻擊動作：逐次揮擊、各自判定命中、傷害累加。
func Resolve(r *Rand, a Attacker, d Defender) AttackResult {
	return ResolveMod(r, a, d, Mods{})
}

// Mods 是攻防兩側的全域修正，來源是防護法術那批計數器
// （`ds:03E3`–`ds:03E7`，清單出自 `sub_1A882` 的「Protection Spells」畫面）。
type Mods struct {
	// Hit 加在命中值上（祝福術 `ds:03E3`，`sub_18DAA` 尾巴）。
	Hit int
	// Damage 是「這一次至少命中一次」時加在總傷害上的值
	//（聖光加值 `ds:03E7`，`0x1903C`）。
	Damage int
	// Halve 把總傷害減半（強力護罩 `ds:03E6`，`sub_17E10` 第一段）。
	Halve bool
	// HalveMelee 只在近戰時再減半一次（防護罩 `ds:03E5`，同一支的第二段）。
	HalveMelee bool
	// Melee 表示這一次是近戰，決定 HalveMelee 生不生效。
	Melee bool
}

// ResolveMod 與 Resolve 相同，但套用全域修正。
func ResolveMod(r *Rand, a Attacker, d Defender, m Mods) AttackResult {
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
		if !hits(r, a, d, m.Hit) {
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
	// 順序照原版：先加聖光加值（`0x1903C`），再進防護那兩道減半
	//（`sub_17E10` 先無條件、後只對近戰）。
	res.Damage += m.Damage
	if m.Halve {
		res.Damage >>= 1
	}
	if m.HalveMelee && m.Melee {
		res.Damage >>= 1
	}
	if h, ok := a.(halvesDamage); ok && h.DamageHalved() {
		res.Damage >>= 1
	}
	if res.Damage < 1 {
		res.Damage = 1
	}
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

// hits 把全域命中加成餵進判定；沒有實作 HitsWith 的攻擊者就忽略它。
func hits(r *Rand, a Attacker, d Defender, bonus int) bool {
	if hw, ok := a.(interface {
		HitsWith(*Rand, Defender, int) bool
	}); ok {
		return hw.HitsWith(r, d, bonus)
	}
	return a.Hits(r, d)
}

// HitsWith 是加上全域命中加成的版本（祝福術）。
func (c *Character) HitsWith(r *Rand, d Defender, bonus int) bool {
	return c.hits(r, d, c.HitBonus+bonus)
}

// HitsWith 同上，射擊用 +79。
func (s Shooter) HitsWith(r *Rand, d Defender, bonus int) bool {
	return s.Character.hits(r, d, s.MissileBonus+bonus)
}

// Hits 是原版隊伍攻擊的命中判定（`2COMBAT.img` `sub_8E81` 那條路徑）。
func (c *Character) Hits(r *Rand, d Defender) bool { return c.hits(r, d, c.HitBonus) }

// hits 是共用的判定本體，bonus 近戰是 +77、射擊是 +79。
func (c *Character) hits(r *Rand, d Defender, bonus int) bool {
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
	limit := base + c.EffectiveLevel()/div
	if limit > 250 {
		limit = 250
	}
	v := r.Range(1, limit) + bonus
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

	// SpecialLeft 是這一輪還能用幾次特殊攻擊（原版 `ds:9F9E[槽位]`）。
	//
	// **不是「還能行動幾次」** —— 用完之後怪物照樣攻擊，只是不再
	// 使用特殊攻擊。見 UseSpecial。
	SpecialLeft int

	// Display 是要顯示的名字，空的話用原文。
	// 在地化在這裡處理 —— 對戰報做字串取代會誤傷，
	// 怪物「Hermit（隱士）」的名字是角色「The Hermit」的一部分。
	Display string
}

// NewMonster 從怪物定義建一隻參戰的怪物。
func NewMonster(def monsters.Monster) *Monster {
	return &Monster{Def: def, HP: def.HP, SpecialLeft: def.SpecialUses}
}

func (m *Monster) CombatName() string {
	if m.Display != "" {
		return m.Display
	}
	return m.Def.Name
}

// CombatSpeed 是排行動順序用的值，來自怪物記錄 b24。
//
// 原版戰鬥開始時把它抄進 `ds:9F92[槽位]`（`0x195CD`），每次要挑下一隻
// 行動的怪物時，在「這一輪還沒動過」的前十隻裡取最大（`0x1A1CC`）。
// 值域 2–250：Zombie 2、Cripple 5 墊底，Mega Dragon 250、
// Time Lord 210、Master Ninja 110 在頂端。
func (m *Monster) CombatSpeed() int { return m.Def.Speed }

// UseSpecial 判定這一次輪到牠要不要用特殊攻擊，要的話用掉一次額度。
//
// `2COMBAT.img` `sub_1847E`，三道關卡依序：
//
//	心智渙散（狀態 bit6）→ 不用           `0x184A1`
//	額度 ds:9F9E 歸零     → 不用           `0x184A8`
//	rand(1,100) > 使用機率 → 不用           `0x184AE`
//	都過了才 dec 額度                       `0x184C5`
//
// **回傳 false 不等於「這一輪不動」。** 呼叫端（`0x185EC`）在拿到 0
// 之後改走普通攻擊：在前排就近戰（`sub_1846C`），在後排則要
// `ds:9E32` 非零且擲 `rand(1,100) <= 80` 才打得到（`sub_1845A`）。
// 把它當成「能不能行動」會讓一百四十隻沒有特殊攻擊的怪物整場站著不動。
func (m *Monster) UseSpecial(r *Rand) bool {
	if m.Status&MonMindless != 0 {
		return false
	}
	if m.SpecialLeft <= 0 {
		return false
	}
	if r.Range(1, 100) > m.Def.SpecialChance {
		return false
	}
	m.SpecialLeft--
	return true
}

// CanReachFromBack 回報這隻怪在後排時打不打得到隊伍。
//
// `0x185FC`：`ds:9E32`（記錄 b18 bit6）非零，而且擲 `rand(1,100) <= 80`。
func (m *Monster) CanReachFromBack(r *Rand) bool {
	return m.Def.RangedAttack && r.Range(1, 100) <= 80
}

// SpellSilenced 回報這一次的特殊攻擊會不會因為沈默而失敗。
//
// `0x18622`：特殊攻擊型態落在 0x0F–0x1E（`0x1D` 除外）就是「施法」，
// 這時狀態 bit1（沈默）或**當前格屬性** `ds:59C8` 的 bit1 任一成立，
// 就印 `*** Spell Failed ***` 而不是 ` casts`。
//
// 沈默在十四個 overlay 裡只有這一個測試點 —— 它沒有別的作用。
func (m *Monster) SpellSilenced(cellAttr byte) bool {
	k := m.Def.SpecialIndex
	if k < 0x0F || k > 0x1E || k == 0x1D {
		return false
	}
	return m.Status&MonSilenced != 0 || cellAttr&0x02 != 0
}

// ResetRound 把這一輪的特殊攻擊額度補回去。
func (m *Monster) ResetRound() { m.SpecialLeft = m.Def.SpecialUses }
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
//
// 驚嚇（狀態位元組 bit3）讓命中率減半（`0x183F1`）。原版在揮擊迴圈
// **之前**減一次，所以整個動作用的是同一個值。
func (m *Monster) Hits(r *Rand, d Defender) bool {
	chance := 5
	if data != nil {
		chance = data.ToHitPercent(m.Def.Tier, d.ArmorClass())
	}
	if m.Status&MonFrightened != 0 {
		chance >>= 1
	}
	// 原版擲的是 rand(10, 1009) 再整數除以 10，不是 rand(1,100)——
	// 兩者分佈不同，而隨機序列要與原版對得上就得照原樣擲。
	return chance >= r.Range(10, 1009)/10
}

// DamageHalved 回報這隻怪物的總傷害要不要減半：衰弱（bit2）就要。
func (m *Monster) DamageHalved() bool { return m.Status&MonWeakened != 0 }

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

// 射擊與近戰是同一支 `sub_18DAA`，差別在 `ds:54A4`：
// `sub_190D6` 把它設 0（近戰）、`sub_190C0` 設 1（射擊）。
// 攻擊設定那一段（`0x18E81`）依它挑不同的欄位：
//
//	欄位          近戰        射擊
//	骰面 ds:549F  記錄[+76]   記錄[+78]
//	加成 ds:54A3  記錄[+77]   記錄[+79]
//
// **弓箭手（職業 2）射擊時骰面那一格不動**，仍是 `+76`；原版在 `0x18F06`
// 另外擲一次 `rand(1, min(等級, 100))` 放進 `ds:54A1`，而 `ds:54A1` 是
// 「每一擊都要加上去的固定值」。所以弓箭手的等級不是換掉武器骰面，
// 是在每一擊之外多加一份與等級同級距的傷害。那一擲**整個動作只擲一次**，
// 三次揮擊共用同一個值。
//
// 命中判定用的加成也跟著換（`ds:54A3`），所以射擊的命中率看 `+79` 不是 `+77`。

// Shooter 把一個角色包成「用射擊打」的攻擊者。
//
// 用包裝而不是在 Character 上放旗標：旗標是可變狀態，忘了清就會
// 讓下一次近戰也走射擊的欄位，而且不會有任何徵兆。
//
// 弓箭手那一擲在建構時完成（原版也是在揮擊迴圈之前），所以要用
// NewShooter 而不是自己組 struct。
type Shooter struct {
	*Character
	// levelRoll 是弓箭手在動作開始時擲的那一次，加在這個動作的每一擊上。
	levelRoll int
}

// NewShooter 準備一次射擊動作。
func NewShooter(r *Rand, c *Character) Shooter {
	s := Shooter{Character: c}
	if c.Class == Archer {
		lv := c.EffectiveLevel()
		if lv > 100 {
			lv = 100
		}
		if lv < 1 {
			lv = 1
		}
		s.levelRoll = r.Range(1, lv)
	}
	return s
}

// AttackDice 是射擊的傷害骰面數（記錄 +78）。弓箭手不換這一格。
func (s Shooter) AttackDice() int {
	if s.Class == Archer {
		return s.Character.AttackDice()
	}
	if d := s.MissileDice; d > 0 {
		return d
	}
	return 1
}

// AttackBonus 是每一擊的固定加值：射擊加成（+79）加上弓箭手的那一擲。
func (s Shooter) AttackBonus() int { return s.MissileBonus + s.levelRoll }

// Hits 與近戰同一條判定式，只是加成換成 +79。
func (s Shooter) Hits(r *Rand, d Defender) bool {
	return s.Character.hits(r, d, s.MissileBonus)
}
