package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 手冊：法術系統 96 條 = 牧師 48 + 巫師 48。
func TestSpellCount(t *testing.T) {
	if len(game.Spells()) != 96 {
		t.Fatalf("法術有 %d 條，預期 96", len(game.Spells()))
	}
	c, s := 0, 0
	for _, sp := range game.Spells() {
		switch sp.School {
		case game.SchoolCleric:
			c++
		case game.SchoolSorcerer:
			s++
		}
	}
	if c != 48 || s != 48 {
		t.Errorf("牧師 %d 條、巫師 %d 條，預期各 48", c, s)
	}
}

// 每一系的每一級都要有法術，而且序號連續不跳號。
func TestSpellLevelsAreComplete(t *testing.T) {
	for _, school := range []game.SpellSchool{game.SchoolCleric, game.SchoolSorcerer} {
		for lvl := 1; lvl <= 9; lvl++ {
			got := game.SpellsOf(school, lvl)
			if len(got) == 0 {
				t.Errorf("%v 第 %d 級沒有法術", school, lvl)
				continue
			}
			for i, sp := range got {
				if sp.Index != i+1 {
					t.Errorf("%v 第 %d 級第 %d 條的序號是 %d", school, lvl, i+1, sp.Index)
				}
			}
		}
	}
}

// 每一條都要有中文名、英文名與消耗 —— 手冊抄漏會在這裡現形。
func TestSpellFieldsFilled(t *testing.T) {
	for _, sp := range game.Spells() {
		if sp.Name == "" || sp.Origin == "" {
			t.Errorf("%v %d-%d 缺名稱：%q / %q", sp.School, sp.Level, sp.Index, sp.Name, sp.Origin)
		}
		if sp.Cost == "" {
			t.Errorf("%v %d-%d（%s）沒有消耗", sp.School, sp.Level, sp.Index, sp.Name)
		}
	}
}
