package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 行動順序照速度排，最快的先動 —— 手冊的規則。
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
	// 六個預設角色裡速度最高的是弓箭手 Sure Valla（21）
	if got := order[0].CombatName(); got != "Sure Valla" {
		t.Errorf("先攻是 %q，預期 Sure Valla（速度最高）", got)
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

// 指令的按鍵要與手冊的指令表對得上。
func TestCombatCommandNames(t *testing.T) {
	want := map[game.CombatCommand]string{
		game.CmdFight: "戰鬥", game.CmdShoot: "射擊", game.CmdRun: "溜跑",
		game.CmdUse: "使用物品", game.CmdProtect: "防護",
	}
	for c, n := range want {
		if c.String() != n {
			t.Errorf("指令 %c 是 %q，預期 %q", byte(c), c.String(), n)
		}
	}
	if len(game.ConfirmedCommands()) != 5 {
		t.Errorf("已確認 handler 的指令有 %d 個，預期 5", len(game.ConfirmedCommands()))
	}
}
