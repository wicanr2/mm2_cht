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
		if c.Age < 14 || c.Age > 60 {
			t.Errorf("%s 的年齡是 %d", c.Name, c.Age)
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
