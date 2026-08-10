package game_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
)

func mons(t *testing.T) []monsters.Monster {
	t.Helper()
	ms, err := monsters.Parse(orig(t, "MONSTERS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

func party(t *testing.T) []game.Character {
	t.Helper()
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

// 同一顆種子必然打出同一場戰鬥 —— 這是能重播的前提，
// 也是「隨機序列與原版同源」這句話唯一站得住的依據。
func TestCombatIsReproducible(t *testing.T) {
	run := func() []game.AttackResult {
		cs, ms := party(t), mons(t)
		r := game.NewRand(0x4321)
		a := &cs[0]
		var out []game.AttackResult
		for i := 0; i < 40; i++ {
			m := game.NewMonster(ms[i%8])
			out = append(out, game.Resolve(r, a, m))
		}
		return out
	}
	x, y := run(), run()
	for i := range x {
		if x[i] != y[i] {
			t.Fatalf("第 %d 次攻擊分歧：%+v vs %+v", i, x[i], y[i])
		}
	}
}

// 打得夠久，怪物一定會倒；倒下之後狀況不再變。
func TestMonsterEventuallyDies(t *testing.T) {
	cs, ms := party(t), mons(t)
	r := game.NewRand(7)
	m := game.NewMonster(ms[2]) // Sewer Rat
	a := &cs[0]
	for i := 0; i < 500 && m.CombatCondition() != game.CondDead; i++ {
		game.Resolve(r, a, m)
	}
	if m.CombatCondition() != game.CondDead {
		t.Fatalf("打了五百次，%s 還活著（HP=%d）", m.CombatName(), m.CombatHP())
	}
	if m.CombatHP() != 0 {
		t.Errorf("死掉之後 HP 是 %d，預期 0", m.CombatHP())
	}
	before := m.CombatHP()
	game.Resolve(r, a, m)
	if m.CombatHP() != before {
		t.Error("死掉之後又被扣血")
	}
}

// 命中與落空都要發生 —— 全中或全落空表示判定式寫壞了。
func TestBothHitAndMissHappen(t *testing.T) {
	cs, ms := party(t), mons(t)
	r := game.NewRand(0xC0DE)
	a := &cs[0]
	hit, miss := 0, 0
	for i := 0; i < 400; i++ {
		m := game.NewMonster(ms[20])
		if game.Resolve(r, a, m).Hit {
			hit++
		} else {
			miss++
		}
	}
	if hit == 0 || miss == 0 {
		t.Errorf("四百次攻擊：命中 %d、落空 %d", hit, miss)
	}
	t.Logf("命中 %d、落空 %d", hit, miss)
}

// 傷害至少 1 點，而且不會超過離譜的上限。
func TestDamageBounds(t *testing.T) {
	cs, ms := party(t), mons(t)
	r := game.NewRand(99)
	a := &cs[0]
	for i := 0; i < 300; i++ {
		m := game.NewMonster(ms[i%256])
		res := game.Resolve(r, a, m)
		if res.Hit && (res.Damage < 1 || res.Damage > 64) {
			t.Fatalf("傷害是 %d", res.Damage)
		}
	}
}

// 全部 256 隻怪物都要能參戰而不炸 —— 那 12 個位元組的語意未定，
// 拿來當數值用就得確保任何值都不會讓流程崩掉。
func TestAllMonstersCanFight(t *testing.T) {
	cs, ms := party(t), mons(t)
	r := game.NewRand(5)
	a := &cs[0]
	for _, def := range ms {
		m := game.NewMonster(def)
		if m.CombatHP() < 1 {
			t.Errorf("%s 的初始 HP 是 %d", m.CombatName(), m.CombatHP())
		}
		for i := 0; i < 5; i++ {
			game.Resolve(r, a, m)
		}
	}
}

// 一場完整的遭遇戰要能打到結束，而且同一顆種子打出同一份戰報。
func TestFullEncounterRuns(t *testing.T) {
	cs, ms := party(t), mons(t)
	build := func() (*game.Encounter, *game.Rand) {
		p, _ := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
		e := &game.Encounter{}
		for i := range p {
			e.Party = append(e.Party, &p[i])
		}
		for i := 0; i < 3; i++ {
			e.Monsters = append(e.Monsters, game.NewMonster(ms[5+i]))
		}
		// 前排要設 —— 前排 0 代表近戰打不到任何人，戰鬥會變成單方面挨打。
		e.Front = len(e.Monsters)
		return e, game.NewRand(0x1111)
	}
	e, r := build()
	log := e.Fight(r, 100)
	if len(log) == 0 {
		t.Fatal("戰鬥沒有任何動作")
	}
	if !e.Over() {
		t.Errorf("打了一百回合還沒結束")
	}
	t.Logf("共 %d 個動作，隊伍%s", len(log), map[bool]string{true: "獲勝", false: "落敗"}[e.PartyWon()])
	for _, l := range log[:min(4, len(log))] {
		t.Log("  " + l)
	}
	// 重播必須一模一樣
	e2, r2 := build()
	log2 := e2.Fight(r2, 100)
	if len(log) != len(log2) {
		t.Fatalf("重播長度不同：%d vs %d", len(log), len(log2))
	}
	for i := range log {
		if log[i] != log2[i] {
			t.Fatalf("第 %d 個動作分歧：\n  %s\n  %s", i, log[i], log2[i])
		}
	}
	_ = cs
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 射擊走的是另一組欄位；弓箭手的等級是額外加值，不是換掉骰面。
func TestShooterUsesMissileFields(t *testing.T) {
	c := &game.Character{}
	c.Level = 7
	c.WeaponDice, c.HitBonus = 8, 3
	c.MissileDice, c.MissileBonus = 5, 2
	r := game.NewRand(1)

	if got := c.AttackDice(); got != 8 {
		t.Errorf("近戰骰面 %d，預期 8（+76）", got)
	}
	sh := game.NewShooter(r, c)
	if got := sh.AttackDice(); got != 5 {
		t.Errorf("射擊骰面 %d，預期 5（+78）", got)
	}
	if got := sh.AttackBonus(); got != 2 {
		t.Errorf("射擊加成 %d，預期 2（+79）", got)
	}

	// 弓箭手：骰面仍是 +76，多出來的是一次 rand(1, min(等級,100))，
	// 而且整個動作只擲一次 —— 同一個 Shooter 問幾次都一樣。
	c.Class = game.Archer
	for i := 0; i < 30; i++ {
		sh := game.NewShooter(r, c)
		if got := sh.AttackDice(); got != 8 {
			t.Fatalf("弓箭手射擊骰面 %d，預期 8（+76 不換）", got)
		}
		roll := sh.AttackBonus() - c.MissileBonus
		if roll < 1 || roll > c.Level {
			t.Fatalf("弓箭手等級擲值 %d，預期落在 1..%d", roll, c.Level)
		}
		if again := sh.AttackBonus(); again != sh.AttackBonus() {
			t.Fatalf("同一次動作的加值變了")
		}
	}
	c.Level = 250
	for i := 0; i < 30; i++ {
		roll := game.NewShooter(r, c).AttackBonus() - c.MissileBonus
		if roll < 1 || roll > 100 {
			t.Fatalf("弓箭手 250 級的擲值 %d，預期夾在 100", roll)
		}
	}
}

// 近戰只打得到前排，射擊打得到整場 —— 這是兩者唯一的目標差異。
func TestReachableFrontVsRanged(t *testing.T) {
	ms := mons(t)
	e := &game.Encounter{Front: 2}
	for i := 0; i < 12; i++ {
		e.Monsters = append(e.Monsters, game.NewMonster(ms[5]))
	}
	if n := len(e.Reachable(false)); n != 2 {
		t.Errorf("近戰打得到 %d 隻，預期 2（前排）", n)
	}
	// 場上十二隻，但選單只發得出十個字母
	if n := len(e.Reachable(true)); n != game.MaxFront {
		t.Errorf("射擊打得到 %d 隻，預期夾在 %d", n, game.MaxFront)
	}
	e.Monsters = e.Monsters[:3]
	e.RemoveMonster(0)
	if n := len(e.Reachable(true)); n != 2 {
		t.Errorf("剩兩隻時射擊打得到 %d 隻", n)
	}
}

// 前排清空但後排還在時，近戰整隊白站，射擊照樣打得到。
func TestMeleeCannotReachBackRank(t *testing.T) {
	p, _ := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	ms := mons(t)
	build := func(ranged bool) *game.Encounter {
		e := &game.Encounter{Front: 0, Ranged: ranged}
		e.Party = append(e.Party, &p[0])
		e.Monsters = append(e.Monsters, game.NewMonster(ms[5]))
		return e
	}
	melee := build(false)
	hp := melee.Monsters[0].CombatHP()
	log := melee.Fight(game.NewRand(7), 1)
	if melee.Monsters[0].CombatHP() != hp {
		t.Errorf("前排 0 卻打得到怪：%d → %d", hp, melee.Monsters[0].CombatHP())
	}
	if len(log) == 0 || !strings.Contains(log[0], "打不到") {
		t.Errorf("沒說明打不到：%v", log)
	}

	// 射擊打得到，但一回合不一定命中；多打幾回合再看有沒有掉血。
	shot := build(true)
	hp = shot.Monsters[0].CombatHP()
	shot.Fight(game.NewRand(7), 20)
	if shot.Monsters[0].CombatHP() == hp && !shot.Over() {
		t.Errorf("射擊二十回合都碰不到後排（HP 仍是 %d）", hp)
	}
}
