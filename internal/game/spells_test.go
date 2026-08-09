package game_test

import (
	"regexp"
	"strconv"
	"strings"
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

// SPELLS.DAT 的施法代價要與手冊對得起來。
//
// 兩個位元組：A 低 nibble 是寶石、B 位元 0–3 是固定法力、位元 4–6 是
// 每施法者等級的法力。前 48 筆是巫師（root `sub_15644` 對非弓箭手／
// 非巫師的職業把序號加 48），產生時已對調成 spells.json 的順序。
//
// 這條不追求 100%：手冊自己有八條把固定消耗標成「每等級」，
// 寶石超過 15 的五條也塞不進一個 nibble。裁決以程式為準。
func TestSpellCostsMatchManual(t *testing.T) {
	d := testData(t)
	spells := game.Spells()
	if len(spells) != 96 {
		t.Fatalf("法術 %d 條，預期 96", len(spells))
	}
	okSP, okGem, tot := 0, 0, 0
	for i, s := range spells {
		c := d.SpellCostAt(i)
		flat, per, gems := parseCost(s.Cost)
		if flat < 0 {
			continue
		}
		tot++
		if int(c.B&0x0F) == flat && int((c.B>>4)&7) == per {
			okSP++
		}
		if c.Gems() == gems {
			okGem++
		}
		// 值域檢查：每等級的量不會超過 7（只有三個位元）。
		if v := int((c.B >> 4) & 7); v > 7 {
			t.Fatalf("%s 的每等級消耗是 %d", s.Name, v)
		}
	}
	if tot < 80 {
		t.Fatalf("只解析出 %d 條消耗字串", tot)
	}
	if okSP*100/tot < 88 {
		t.Errorf("法力消耗只有 %d/%d 相符", okSP, tot)
	}
	if okGem*100/tot < 90 {
		t.Errorf("寶石消耗只有 %d/%d 相符", okGem, tot)
	}
	t.Logf("%d 條裡法力相符 %d、寶石相符 %d", tot, okSP, okGem)
}

// 每等級的消耗要真的隨等級長。
func TestSpellCostScalesWithLevel(t *testing.T) {
	d := testData(t)
	found := false
	for i := range game.Spells() {
		c := d.SpellCostAt(i)
		if (c.B>>4)&7 == 0 {
			continue
		}
		found = true
		if a, b := c.SP(1), c.SP(2); b-a != int((c.B>>4)&7) {
			t.Errorf("第 %d 條：等級 1 要 %d、等級 2 要 %d，級差不對", i, a, b)
		}
	}
	if !found {
		t.Error("沒有一條法術的消耗隨等級變動")
	}
}

// parseCost 解手冊的消耗字串，回傳 (固定, 每等級, 寶石)；解不出回 -1。
func parseCost(s string) (flat, per, gems int) {
	s = strings.NewReplacer("＋", "+", "寶石", "g", " ", "", "／", "/").Replace(s)
	m := costRE.FindStringSubmatch(s)
	if m == nil {
		return -1, 0, 0
	}
	n, _ := strconv.Atoi(m[1])
	if m[3] != "" {
		per = n
	} else {
		flat = n
	}
	if m[4] != "" {
		gems, _ = strconv.Atoi(m[4])
	}
	return flat, per, gems
}

var costRE = regexp.MustCompile(`^(\d+)(SP)?(/L)?(?:\+(\d+)g)?$`)
