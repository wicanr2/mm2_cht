package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// DEFAULT.DAT 是六個預設角色。名字、職業順序、屬性都在這裡定錨。
func TestParseDefaultRoster(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 6 {
		t.Fatalf("解出 %d 個角色，預期 6", len(cs))
	}
	want := []struct {
		name  string
		class game.Class
	}{
		{"Sir Felgar", game.Knight},
		{"Terwin III", game.Paladin},
		{"Sure Valla", game.Archer},
		{"Gene Eric", game.Cleric},
		{"Cassandra", game.Sorcerer},
		{"The Hermit", game.Robber},
	}
	for i, w := range want {
		if cs[i].Name != w.name {
			t.Errorf("第 %d 個角色叫 %q，預期 %q", i, cs[i].Name, w.name)
		}
		if cs[i].Class != w.class {
			t.Errorf("%s 的職業是 %v，預期 %v", w.name, cs[i].Class, w.class)
		}
	}
}

// 屬性順序是從「每個角色的峰值落在自己職業該高的那一項」反推的。
// 這條把那個推論釘住：六個角色一個都不能錯。
func TestStatOrderMatchesClass(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	peak := map[game.Class]game.Stat{
		game.Knight:   game.Might,
		game.Paladin:  game.Might,
		game.Archer:   game.Speed,
		game.Cleric:   game.Personality,
		game.Sorcerer: game.Intellect,
		game.Robber:   game.Accuracy,
	}
	for _, c := range cs {
		want, ok := peak[c.Class]
		if !ok {
			continue
		}
		best, bestI := -1, game.Stat(-1)
		for i := game.Stat(0); i < game.NumStats; i++ {
			if c.Base[i] > best {
				best, bestI = c.Base[i], i
			}
		}
		if bestI != want {
			t.Errorf("%s（%v）最高的屬性是 %v=%d，預期 %v=%d",
				c.Name, c.Class, bestI, best, want, c.Base[want])
		}
	}
}

// 屬性值要落在合理範圍，HP 也不能是 0 或大得離譜 ——
// 欄位位置錯開一個位元組就會踩到這條。
func TestCharacterFieldsAreSane(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		for i := game.Stat(0); i < game.NumStats; i++ {
			if v := c.Base[i]; v < 3 || v > 25 {
				t.Errorf("%s 的%v是 %d，超出 3–25", c.Name, i, v)
			}
			if c.Current[i] != c.Base[i] {
				t.Errorf("%s 的%v：當前 %d 與基礎 %d 不同（預設角色不該有增減益）",
					c.Name, i, c.Current[i], c.Base[i])
			}
		}
		if c.HP < 1 || c.HP > 100 {
			t.Errorf("%s 的 HP 是 %d", c.Name, c.HP)
		}
		if c.HP != c.MaxHP {
			t.Errorf("%s 的 HP %d 與上限 %d 不同（預設角色應該是滿的）", c.Name, c.HP, c.MaxHP)
		}
		if c.Age != 18 {
			t.Errorf("%s 的年齡是 %d，預設角色應該都是 18（手冊 p.34 的截圖也是 Age=18）", c.Name, c.Age)
		}
		if c.Level != 1 {
			t.Errorf("%s 的等級是 %d，預設角色應該都是第一級", c.Name, c.Level)
		}
		if c.Food != 10 {
			t.Errorf("%s 的食物是 %d，預設角色應該都是 10", c.Name, c.Food)
		}
	}
}

// SP 只有一開始就能施法的職業才有。這條把 +88/+90 這一對釘住 ——
// 挪一個位元組，牧師與巫師的 7 點法力就對不上了。
func TestSpellPointsMatchClass(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		switch {
		case c.Class.Caster() && c.SP == 0:
			t.Errorf("%s 是%v，SP 卻是 0", c.Name, c.Class)
		case !c.Class.Caster() && c.SP != 0:
			t.Errorf("%s 是%v，第一級不該有法力，SP 卻是 %d", c.Name, c.Class, c.SP)
		}
		if c.SP != c.MaxSP {
			t.Errorf("%s 的 SP %d 與上限 %d 不同（預設角色應該是滿的）", c.Name, c.SP, c.MaxSP)
		}
	}
}

// ROSTER.DAT 不是 130 的整數倍，尾端不成一筆的部分要略過而不是硬湊。
func TestParseRosterHandlesTail(t *testing.T) {
	blob := orig(t, "ROSTER.DAT")
	cs, err := game.ParseCharacters(blob)
	if err != nil {
		t.Fatal(err)
	}
	if want := len(blob) / game.RecordSize; len(cs) != want {
		t.Errorf("解出 %d 筆，預期 %d", len(cs), want)
	}
}

// 性別、陣營、種族都要落在合法值域，而且與手冊的規則對得上。
func TestSexAlignmentRace(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if c.Sex > game.Female {
			t.Errorf("%s 的性別是 %d", c.Name, c.Sex)
		}
		if c.Align > game.Evil {
			t.Errorf("%s 的陣營是 %d", c.Name, c.Align)
		}
		if c.Race > game.HalfOrc {
			t.Errorf("%s 的種族是 %d", c.Name, c.Race)
		}
		// 手冊：遊俠必須是善良陣營
		if c.Class == game.Paladin && c.Align != game.Good {
			t.Errorf("%s 是遊俠，陣營卻是%v", c.Name, c.Align)
		}
	}
	// 六個預設角色裡女性正好是 Sure Valla 與 Cassandra
	var female []string
	for _, c := range cs {
		if c.Sex == game.Female {
			female = append(female, c.Name)
		}
	}
	if len(female) != 2 || female[0] != "Sure Valla" || female[1] != "Cassandra" {
		t.Errorf("女性角色是 %v，預期 [Sure Valla Cassandra]", female)
	}
}

// 手冊的種族屬性修正要在數值上看得出來。
// 半獸人 +1 力量耐力、−1 智慧人格運氣；精靈 +1 智慧準確度、−1 力量耐力。
func TestRaceModifiersShowInStats(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]game.Character{}
	for _, c := range cs {
		byName[c.Name] = c
	}
	if c := byName["Sir Felgar"]; c.Race != game.HalfOrc {
		t.Errorf("Sir Felgar 的種族是%v，預期半獸人", c.Race)
	} else if c.Base[game.Might] <= c.Base[game.Intellect] {
		t.Errorf("半獸人的力量 %d 應該高過智慧 %d", c.Base[game.Might], c.Base[game.Intellect])
	}
	if c := byName["Cassandra"]; c.Race != game.Elf {
		t.Errorf("Cassandra 的種族是%v，預期精靈", c.Race)
	}
	if c := byName["Sure Valla"]; c.Race != game.Human {
		t.Errorf("Sure Valla 的種族是%v，預期人類", c.Race)
	}
}

// 名冊裡的欄位值域要站得住。這條用 ROSTER.DAT 的四十個練過的角色驗，
// 比只有六個一級新角色的 DEFAULT.DAT 有力得多。
func TestRosterFieldRanges(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range cs {
		if c.Empty() {
			continue // 刪除後殘留的槽位
		}
		n++
		if c.Level < 1 || c.Level > 60 {
			t.Errorf("%s 的等級是 %d", c.Name, c.Level)
		}
		if c.Age < 14 || c.Age > 80 {
			t.Errorf("%s 的年齡是 %d", c.Name, c.Age)
		}
		if c.Food > 40 {
			t.Errorf("%s 的食物是 %d，超過上限 40", c.Name, c.Food)
		}
		if c.Align > game.Evil || c.Race > game.HalfOrc || c.Class > game.Barbarian {
			t.Errorf("%s 的陣營／種族／職業超出值域：%d/%d/%d",
				c.Name, c.Align, c.Race, c.Class)
		}
	}
	if n < 20 {
		t.Fatalf("名冊裡只有 %d 個角色，樣本太少", n)
	}
	t.Logf("驗過 %d 個角色", n)
}

// 狀況欄位：預設角色全部正常，名冊裡只有少數幾筆帶位元。
// 位置由 sub_1AFBC 定案（HP 歸零時在 +38 設 bit 6）。
func TestConditionBits(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if c.CondBits != 0 || c.Condition != game.CondGood {
			t.Errorf("%s 的狀況是 %#x/%v，預設角色應該都正常", c.Name, c.CondBits, c.Condition)
		}
	}
	rs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	good, bad := 0, 0
	for _, c := range rs {
		if c.Empty() {
			continue
		}
		if c.CondBits == 0 {
			good++
		} else {
			bad++
		}
	}
	if good == 0 {
		t.Error("名冊裡沒有任何狀況正常的角色")
	}
	if good+bad < 20 {
		t.Fatalf("名冊樣本只有 %d 筆", good+bad)
	}
	// 帶狀況的應該是少數；如果過半都有位元，多半是欄位挑錯了
	if bad*2 > good {
		t.Errorf("名冊裡 %d/%d 筆帶狀況位元，比例過高", bad, good+bad)
	}
	t.Logf("名冊：正常 %d 筆、帶狀況 %d 筆", good, bad)
}
