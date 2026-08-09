package game

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
)

// 攻擊解析。
//
// **這是手冊規則的實作，不是原版公式的還原。**
//
// 原版的命中與傷害公式還沒從 overlay 讀出來 —— 它們用的查表在 DGROUP 的
// BSS 區，執行時才填，檔案裡讀不到（見 docs/formats/01 §2.6）。
// 在解出來之前，這一層照手冊寫得明白的規則做：
//
//   - 力量影響傷害（docs/manual/part-1.md 屬性表）
//   - 準確度影響命中
//   - 速度增加防禦等級
//   - 生命歸零→無意識→再受傷即死亡
//
// 擲骰一律走 `Rand`，也就是原版那顆 RNG（`sub_11C88` 逐行重寫）。
// 所以隨機序列與原版同源，只有「怎麼用這些亂數」是暫定的。
//
// 公式解出來之後，換掉 Resolve 一支函式即可，戰鬥流程與介面不必動。

// AttackResult 是一次攻擊的結果。
type AttackResult struct {
	Hit    bool
	Damage int
	Roll   int // 命中判定擲出的點數，方便對照與除錯
	Target Condition
}

func (r AttackResult) String() string {
	if !r.Hit {
		return fmt.Sprintf("落空（擲出 %d）", r.Roll)
	}
	return fmt.Sprintf("命中，造成 %d 點傷害", r.Damage)
}

// Attacker 是攻擊方需要提供的數值。
type Attacker interface {
	CombatName() string
	AttackAccuracy() int // 準確度
	AttackMight() int    // 力量
}

// Defender 是受擊方需要提供的數值。
type Defender interface {
	Combatant
	ArmorClass() int
}

// Resolve 解一次攻擊。
//
// 判定：`rand(1, 20) + 準確度/4` 要大於 `10 + 防護等級`。
// 傷害：`rand(1, 6) + 力量/4`，至少 1 點。
//
// 這兩條的形狀取自手冊「準確度影響命中、力量影響傷害」，
// 數字是暫定的 —— 原版的實際係數未解。
func Resolve(r *Rand, a Attacker, d Defender) AttackResult {
	roll := r.Range(1, 20)
	res := AttackResult{Roll: roll, Target: d.CombatCondition()}
	if roll+a.AttackAccuracy()/4 <= 10+d.ArmorClass() {
		return res
	}
	dmg := r.Range(1, 6) + a.AttackMight()/4
	if dmg < 1 {
		dmg = 1
	}
	res.Hit = true
	res.Damage = dmg
	res.Target = d.TakeDamage(dmg)
	return res
}

// Character 的攻守數值。
func (c *Character) AttackAccuracy() int { return c.Current[Accuracy] }
func (c *Character) AttackMight() int    { return c.Current[Might] }

// ArmorClass 是防護等級。
//
// 記錄裡的 AC 欄位還沒定位，所以照手冊「速度增加防禦等級」暫時由速度推算。
func (c *Character) ArmorClass() int { return c.Current[Speed] / 4 }

// Monster 是戰鬥中的一隻怪物。
//
// 名稱來自 `MONSTERS.DAT`，數值**由怪物在表中的索引推算**。
//
// 為什麼不用記錄裡那 12 個位元組：它們的語意未定，直接當數值用會算出
// 荒謬的結果 —— 第一版拿 `+0x0F`（值域 37–255）當等級、`% 16` 當防護等級，
// 四百次攻擊命中零次，因為防護等級動輒 15。
//
// 索引則是**已經確認過的資訊**：怪物表按難度排序（前面是下水道鼠、
// 哥布林，最後五筆是四位元素領主與 Sheltem）。拿它推等級，
// 至少強弱關係與原版一致。
//
// 這仍是暫定值。那 12 個位元組解出來之後換掉 NewMonster 即可。
type Monster struct {
	Def   monsters.Monster
	HP    int
	Cond  Condition
	Level int

	// Display 是要顯示的名字，空的話用原文。
	// 在地化在這裡處理 —— 對戰報做字串取代會誤傷，
	// 怪物「Hermit（隱士）」的名字是角色「The Hermit」的一部分。
	Display string
}

// NewMonster 從怪物定義建一隻參戰的怪物。
func NewMonster(def monsters.Monster) *Monster {
	lv := def.Index/8 + 1 // 256 隻攤成 1–32 級
	return &Monster{Def: def, HP: lv * 4, Level: lv}
}

func (m *Monster) CombatName() string {
	if m.Display != "" {
		return m.Display
	}
	return m.Def.Name
}
func (m *Monster) CombatSpeed() int          { return m.Level }
func (m *Monster) CombatHP() int             { return m.HP }
func (m *Monster) CombatCondition() Condition { return m.Cond }
func (m *Monster) AttackAccuracy() int       { return m.Level }
func (m *Monster) AttackMight() int          { return m.Level }
func (m *Monster) ArmorClass() int           { return m.Level / 4 }

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
