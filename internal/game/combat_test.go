package game_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// guaranteedAttacker 只讓本測試固定驗證「快速 Fight 的死亡 hook」；
// 掉落數值仍由 Monster typed 欄位與 Encounter 規則產生。
type guaranteedAttacker struct{}

func (guaranteedAttacker) CombatName() string                  { return "測試隊員" }
func (guaranteedAttacker) CombatSpeed() int                    { return 100 }
func (guaranteedAttacker) CombatHP() int                       { return 10 }
func (guaranteedAttacker) CombatCondition() game.Condition     { return game.CondGood }
func (guaranteedAttacker) TakeDamage(int) game.Condition       { return game.CondGood }
func (guaranteedAttacker) AttackSwings() int                   { return 1 }
func (guaranteedAttacker) AttackDice() int                     { return 1 }
func (guaranteedAttacker) AttackBonus() int                    { return 1 }
func (guaranteedAttacker) Hits(*game.Rand, game.Defender) bool { return true }

// 行動順序照原版讀的那一格排，鍵值大的先動。
//
// **不要在這裡斷言「哪個角色先動」** —— 那等於把「第四格叫什麼」
// 一起釘死，而那件事還沒定案（見 Character.CombatSpeed 的註解）。
// 這裡驗的是排序本身與 NextActor 的挑法一致。
func TestCombatOrderBySpeed(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	e := &game.Encounter{}
	for i := range cs {
		e.Party = append(e.Party, &cs[i])
	}
	order := e.Order()
	if len(order) != len(cs) {
		t.Fatalf("行動順序有 %d 人，預期 %d", len(order), len(cs))
	}
	for i := 1; i < len(order); i++ {
		if order[i-1].CombatSpeed() < order[i].CombatSpeed() {
			t.Errorf("第 %d 位（速度 %d）排在第 %d 位（速度 %d）前面",
				i-1, order[i-1].CombatSpeed(), i, order[i].CombatSpeed())
		}
	}
	// Order 的第一個要與 NextActor 挑的是同一個人 —— 兩條路徑
	// 用的是同一個鍵，不一致就表示其中一條讀錯了欄位。
	i, party, ok := e.NextActor(map[int]bool{}, map[int]bool{})
	if !ok {
		t.Fatal("NextActor 挑不出人")
	}
	if !party {
		t.Fatal("場上沒有怪物，NextActor 卻挑了怪物那一邊")
	}
	if order[0].CombatName() != e.Party[i].CombatName() {
		t.Errorf("Order 先攻是 %q，NextActor 挑的是 %q",
			order[0].CombatName(), e.Party[i].CombatName())
	}
	// 而且挑出來的確實是鍵值最大的那一個。
	for _, c := range e.Party {
		if c.CombatSpeed() > e.Party[i].CombatSpeed() {
			t.Errorf("%q 的鍵值 %d 大於先攻 %q 的 %d",
				c.CombatName(), c.CombatSpeed(),
				e.Party[i].CombatName(), e.Party[i].CombatSpeed())
		}
	}
}

// 手冊：生命點數歸零時失去意識，之後再受任何傷害即死亡。
func TestDamageToUnconsciousThenDead(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	c := &cs[0]
	if c.Condition != game.CondGood {
		t.Fatalf("初始狀況是 %v，預期正常", c.Condition)
	}
	if got := c.TakeDamage(c.MaxHP); got != game.CondUnconscious {
		t.Errorf("扣光血之後是 %v，預期無意識", got)
	}
	if c.HP != 0 {
		t.Errorf("HP 是 %d，預期 0（不該變成負數）", c.HP)
	}
	if got := c.TakeDamage(1); got != game.CondDead {
		t.Errorf("無意識再受傷之後是 %v，預期死亡", got)
	}
	// 死了就不會再變
	if got := c.TakeDamage(99); got != game.CondDead {
		t.Errorf("死亡後又變成 %v", got)
	}
}

// 倒下的人不排進行動順序，一方全倒戰鬥就結束。
func TestEncounterEndsWhenSideFalls(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	e := &game.Encounter{}
	for i := range cs[:2] {
		e.Party = append(e.Party, &cs[i])
	}
	for i := 2; i < 4; i++ {
		e.Monsters = append(e.Monsters, &cs[i])
	}
	if e.Over() {
		t.Fatal("剛開打就結束了")
	}
	for _, m := range e.Monsters {
		m.TakeDamage(999)
	}
	if !e.Over() {
		t.Error("怪物全倒了，戰鬥卻沒結束")
	}
	if !e.PartyWon() {
		t.Error("怪物全倒、隊伍還在，卻不算贏")
	}
	if n := len(e.Order()); n != 2 {
		t.Errorf("行動順序有 %d 人，預期只剩隊伍那 2 人", n)
	}
}

// 一般戰鬥勝利才消費逐怪死亡時序累加的戰利品；競技賽／事件獎賞不經此 API。
func TestVictoryChestUsesMonsterDropFields(t *testing.T) {
	cs := []game.Character{{Name: "英雄", Condition: game.CondGood}}
	m := &game.Monster{}
	// 測試只使用已由 MONSTERS.DAT typed data 解出的欄位，不注入任意物品表。
	// 怪物已死亡，模擬正常 Fight 內 target 死亡後的狀態。
	m.Def.GemDrop = true
	m.Def.DropBand = 1
	m.Def.GoldMode = 1
	m.Def.Tier = 2
	m.Def.Index = 0x21
	m.Cond = game.CondDead
	e := &game.Encounter{Party: []game.Combatant{&cs[0]}, Monsters: []game.Combatant{m}}
	r := game.NewRand(7)
	c := e.VictoryChest(r)
	if c == nil {
		t.Fatal("有掉落欄位的勝利沒有建立寶箱")
	}
	if c.Gems <= 0 {
		t.Errorf("寶石掉落是 %d，預期由 GemDrop 產生", c.Gems)
	}
	if c.Gold <= 0 {
		t.Errorf("金幣掉落是 %d，預期由 GoldMode 產生", c.Gold)
	}
	for i, it := range c.Items {
		if it.ID != 0 {
			t.Errorf("沒有 ITEMS.DAT 時第 %d 格不應生成假物品：%+v", i, it)
		}
	}
	if again := e.VictoryChest(r); again != nil {
		t.Fatal("同一場勝利重複產生寶箱")
	}
}

func TestFastFightRecordsDefeatBeforeVictoryChest(t *testing.T) {
	m := &game.Monster{Def: monsters.Monster{Index: 0x21, DropBand: 0, GemDrop: true, HP: 1}, HP: 1}
	e := &game.Encounter{Party: []game.Combatant{guaranteedAttacker{}}, Monsters: []game.Combatant{m}, Front: 1}
	e.Fight(game.NewRand(11), 1)
	c := e.VictoryChest(game.NewRand(12))
	if c == nil || c.Gems <= 0 {
		t.Fatalf("快速 Fight 死亡 hook 未產生寶石箱：%+v", c)
	}
}

func TestTacticalRecordsDefeatBeforeReap(t *testing.T) {
	m := &game.Monster{Def: monsters.Monster{Index: 0x21, DropBand: 0, GemDrop: true}, Cond: game.CondDead}
	e := &game.Encounter{Party: []game.Combatant{guaranteedAttacker{}}, Monsters: []game.Combatant{m}, Front: 1}
	e.RecordDefeat(game.NewRand(13))
	if e.Reap() != 1 {
		t.Fatal("tactical 路徑沒有在 RecordDefeat 後 Reap 死亡怪物")
	}
	// RecordDefeat 的累加器保留在 encounter，即使 Reap 已移除怪物。
	c := e.VictoryChest(game.NewRand(14))
	if c == nil || c.Gems <= 0 {
		t.Fatalf("tactical Reap 後遺失戰利品：%+v", c)
	}
}

func TestFleeRecordsDefeatBeforeRemoval(t *testing.T) {
	for seed := uint16(1); seed < 1000; seed++ {
		p := &game.Character{Level: 100, Condition: game.CondGood}
		m := &game.Monster{Def: monsters.Monster{Index: 0x21, MoraleTier: 0, GemDrop: true}, HP: 1}
		e := &game.Encounter{Party: []game.Combatant{p}, Monsters: []game.Combatant{m}, Front: 1}
		if !e.TryFlee(game.NewRand(seed), 0, false) {
			continue
		}
		c := e.VictoryChest(game.NewRand(seed + 1))
		if c == nil || c.Gems <= 0 {
			t.Fatalf("逃跑移除前未保留戰利品：%+v", c)
		}
		return
	}
	t.Fatal("1000 顆固定種子都沒有觸發逃跑")
}

// 指令的按鍵要與手冊的指令表對得上。
func TestCombatCommandNames(t *testing.T) {
	want := map[game.CombatCommand]string{
		game.CmdFight: "戰鬥", game.CmdShoot: "射擊", game.CmdCast: "施法",
		game.CmdUse: "使用物品", game.CmdBlock: "抵擋", game.CmdRun: "溜跑",
		game.CmdExchange: "對調", game.CmdView: "檢視", game.CmdProtect: "防護",
	}
	for c, n := range want {
		if c.String() != n {
			t.Errorf("指令 %c 是 %q，預期 %q", byte(c), c.String(), n)
		}
	}
	// 九個指令的 handler 都讀完了；少一個就代表有人退回去了。
	if got := game.ConfirmedCommands(); len(got) != len(want) {
		t.Errorf("已確認 handler 的指令有 %d 個，預期 %d", len(got), len(want))
	}
}

// 五條防護法術要真的改變戰鬥數值，不是施完就沒事。
func TestProtectionAffectsCombat(t *testing.T) {
	p, _ := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	ms := mons(t)
	build := func(pr game.Protection) *game.Encounter {
		e := &game.Encounter{Front: 1, Protect: pr}
		e.Party = append(e.Party, &p[0])
		m := game.NewMonster(ms[5])
		e.Monsters = append(e.Monsters, m)
		return e
	}
	// 聖光加值：命中就多加那麼多傷害
	plain, holy := 0, 0
	for seed := 0; seed < 40; seed++ {
		a := build(game.Protection{})
		b := build(game.Protection{HolyBonus: 10})
		plain += damageDealt(a, game.NewRand(uint16(seed)))
		holy += damageDealt(b, game.NewRand(uint16(seed)))
	}
	if holy <= plain {
		t.Errorf("聖光加值沒有加傷害：%d vs %d", plain, holy)
	}

	// 強力護罩：隊伍受到的傷害減半
	e := build(game.Protection{PowerShield: 1})
	if m := e.Mods(false); !m.Halve {
		t.Error("強力護罩沒有讓對隊伍的傷害減半")
	}
	if m := e.Mods(true); m.Halve {
		t.Error("強力護罩不該影響隊伍打出去的傷害")
	}
	// 防護罩只擋近戰
	e = build(game.Protection{Shield: 1})
	if m := e.Mods(false); !m.HalveMelee {
		t.Error("防護罩沒有生效")
	}
	// 祝福術加在命中值上
	e = build(game.Protection{Bless: 7})
	if m := e.Mods(true); m.Hit != 7 {
		t.Errorf("祝福術的命中加成是 %d，預期 7", m.Hit)
	}
}

func damageDealt(e *game.Encounter, r *game.Rand) int {
	before := e.Monsters[0].CombatHP()
	e.Fight(r, 1)
	return before - e.Monsters[0].CombatHP()
}

// 防護效能畫面只列非零的那幾條，最後一條連數值一起印。
func TestProtectionLines(t *testing.T) {
	if got := (game.Protection{}).Lines(); len(got) != 2 || got[1] != "（一條都沒有）" {
		t.Errorf("全空時印 %v", got)
	}
	got := game.Protection{Bless: 3, HolyBonus: 12}.Lines()
	joined := strings.Join(got, "|")
	for _, want := range []string{"祝福術", "聖光加值 12"} {
		if !strings.Contains(joined, want) {
			t.Errorf("缺少「%s」：%v", want, got)
		}
	}
	if strings.Contains(joined, "隱身術") {
		t.Errorf("列了計數器是 0 的項目：%v", got)
	}
}
