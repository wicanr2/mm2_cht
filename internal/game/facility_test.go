package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 每級生命點數的骰子上限照手冊的職業表。
func TestHitDiceMatchManual(t *testing.T) {
	want := map[game.Class]int{
		game.Knight: 12, game.Paladin: 10, game.Archer: 10, game.Cleric: 8,
		game.Sorcerer: 6, game.Robber: 8, game.Ninja: 8, game.Barbarian: 12,
	}
	for c, n := range want {
		if got := c.HitDice(); got != n {
			t.Errorf("%v 每級生命點數上限是 %d，手冊寫 %d", c, got, n)
		}
	}
}

// 升級要擲在骰子範圍內、年齡加一、HP 補滿。
func TestTrainRaisesLevel(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	r := game.NewRand(2024)
	c := &cs[0] // 武士，每級 1–12
	if c.CanTrain() {
		t.Fatal("第一級沒經驗值就能受訓")
	}
	c.Exp = game.ExpForLevel(2)
	if !c.CanTrain() {
		t.Fatal("經驗值夠了卻不能受訓")
	}
	lv, age, max := c.Level, c.Age, c.MaxHP
	gained, err := c.Train(r)
	if err != nil {
		t.Fatal(err)
	}
	if c.Level != lv+1 {
		t.Errorf("等級是 %d，預期 %d", c.Level, lv+1)
	}
	if c.Age != age+1 {
		t.Errorf("年齡是 %d，預期 %d", c.Age, age+1)
	}
	if gained < 1 || gained > 12 {
		t.Errorf("擲出 %d 點生命，超出武士的 1–12", gained)
	}
	if c.MaxHP != max+gained || c.HP != c.MaxHP {
		t.Errorf("HP 是 %d/%d，預期 %d/%d", c.HP, c.MaxHP, max+gained, max+gained)
	}
}

// 倒下的人不能受訓。
func TestDownedCannotTrain(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	c := &cs[0]
	c.Exp = 999999
	c.TakeDamage(c.MaxHP)
	if c.CanTrain() {
		t.Error("無意識的人可以受訓")
	}
	if _, err := c.Train(game.NewRand(1)); err == nil {
		t.Error("無意識的人受訓沒有回錯誤")
	}
}

// 法力等級照手冊的對照表：經驗等級 1、3、5… 各對應法力等級 1…9。
func TestSpellLevelThresholds(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cleric := cs[3] // 牧師
	for _, w := range []struct{ level, sl int }{
		{1, 1}, {2, 1}, {3, 2}, {5, 3}, {9, 5}, {17, 9}, {25, 9},
	} {
		cleric.Level = w.level
		if got := cleric.SpellLevel(); got != w.sl {
			t.Errorf("牧師第 %d 級的法力等級是 %d，預期 %d", w.level, got, w.sl)
		}
	}
	// 武士不會施法
	knight := cs[0]
	knight.Level = 20
	if got := knight.SpellLevel(); got != 0 {
		t.Errorf("武士的法力等級是 %d，預期 0", got)
	}
}

// 旅店休息補滿並解除無意識，但救不回死亡。
func TestRestAtInn(t *testing.T) {
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, cs, nil, 1)
	s.Party[0].TakeDamage(s.Party[0].MaxHP) // 無意識
	s.Party[1].TakeDamage(s.Party[1].MaxHP)
	s.Party[1].TakeDamage(1) // 死亡
	s.Party[2].HP = 1

	s.RestAtInn()
	if s.Party[0].Condition != game.CondGood || s.Party[0].HP != s.Party[0].MaxHP {
		t.Errorf("無意識的人休息後是 %v HP=%d", s.Party[0].Condition, s.Party[0].HP)
	}
	if s.Party[1].Condition != game.CondDead {
		t.Errorf("死亡的人被休息救回來了：%v", s.Party[1].Condition)
	}
	if s.Party[2].HP != s.Party[2].MaxHP {
		t.Errorf("受傷的人沒補滿：%d/%d", s.Party[2].HP, s.Party[2].MaxHP)
	}
}

// 打贏會給經驗，累積夠了就升得了級 —— 整條成長線要接得起來。
func TestProgressionEndToEnd(t *testing.T) {
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	ms := mons(t)
	s := game.NewSession(w, cs, ms, 555)

	// 打到經驗夠升級為止
	for i := 0; i < 200 && s.Party[0].Exp < game.ExpForLevel(2); i++ {
		e := &game.Encounter{Party: s.Combatants()}
		e.Monsters = append(e.Monsters, game.NewMonster(ms[3]))
		e.Fight(s.Rand, 50)
		if e.PartyWon() {
			e.AwardExp(s.Party)
		}
		s.RestAtInn() // 補血再打下一場
	}
	if s.Party[0].Exp < game.ExpForLevel(2) {
		t.Fatalf("打了兩百場，經驗只有 %d，升到第二級要 %d",
			s.Party[0].Exp, game.ExpForLevel(2))
	}
	maxHP := s.Party[0].MaxHP
	log := s.TrainParty()
	if len(log) == 0 {
		t.Fatal("受訓沒有任何結果")
	}
	if s.Party[0].Level != 2 {
		t.Errorf("受訓後等級是 %d，預期 2", s.Party[0].Level)
	}
	if s.Party[0].MaxHP <= maxHP {
		t.Errorf("升級後生命上限沒增加：%d → %d", maxHP, s.Party[0].MaxHP)
	}
	t.Logf("%s", log[0])
}
