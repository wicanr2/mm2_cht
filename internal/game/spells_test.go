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

// 施法的資格與代價：會不會、法力等級夠不夠、法力與寶石扣得對不對。
func TestCastChecksAndCosts(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:6]...)
	s := game.NewSession(w, party, nil, 1)

	// 名冊的第 4 個是牧師、第 5 個是巫師（法力欄位非零的那兩個）。
	cleric, sorcerer := -1, -1
	for i := range party {
		switch party[i].Class {
		case game.Cleric:
			cleric = i
		case game.Sorcerer:
			sorcerer = i
		}
	}
	if cleric < 0 || sorcerer < 0 {
		t.Skip("前六人裡沒有牧師或巫師")
	}

	// 武士不會施法。
	for i := range party {
		if party[i].Class == game.Knight {
			if r := s.Cast(i, 1); r.OK {
				t.Error("武士竟然施得出法術")
			}
			break
		}
	}

	// 沒學過的施不出來。
	if r := s.Cast(cleric, 40); r.OK {
		t.Error("沒學過的法術竟然施得出來")
	}

	// 學會第 1 條之後應該施得出來，而且扣掉的法力與表一致。
	d := testData(t)
	party[cleric].Learn(1)
	before := party[cleric].SP
	idx := game.SpellIndex(game.SchoolCleric, 1)
	want := d.SpellCostAt(idx).SP(party[cleric].Level)
	r := s.Cast(cleric, 1)
	if !r.OK {
		t.Fatalf("學會了還是施不出來：%s", r.Reason)
	}
	if r.SP != want {
		t.Errorf("扣了 %d 點法力，表上是 %d", r.SP, want)
	}
	if party[cleric].SP != before-want {
		t.Errorf("法力剩 %d，預期 %d", party[cleric].SP, before-want)
	}

	// 法力不足要擋下來。
	party[cleric].SP = 0
	if r := s.Cast(cleric, 1); r.OK {
		t.Error("法力 0 竟然還施得出來")
	}
}

// 法術編號的分派：巫師系在前 48，牧師系在後 48。
//
// 兩個施法 overlay 的跳表與 SPELLS.DAT 用的是同一套編號 ——
// 「喚醒術」同時出現在跳表的第 0 與第 49 項，兩者指向同一支 handler。
func TestSpellIndexSplit(t *testing.T) {
	// 引擎的編號跟著 spells.json：牧師在前。原版檔案是相反的，
	// 對調由 cmd/mm2data 在產生 spellcosts.json 時做掉。
	if got := game.SpellIndex(game.SchoolCleric, 1); got != 0 {
		t.Errorf("牧師第 1 條是 %d，預期 0", got)
	}
	if got := game.SpellIndex(game.SchoolSorcerer, 1); got != 48 {
		t.Errorf("巫師第 1 條是 %d，預期 48", got)
	}
	if got := game.SpellIndex(game.SchoolSorcerer, 48); got != 95 {
		t.Errorf("巫師第 48 條是 %d，預期 95", got)
	}
	// 編號要真的對到那條法術。
	if n := game.Spells()[game.SpellIndex(game.SchoolCleric, 4)].Name; n != "急救術" {
		t.Errorf("牧師第 4 條是 %q，預期急救術", n)
	}
	// 職業對法術系的分派照原版 sub_15644。
	for _, tc := range []struct {
		c game.Class
		s game.SpellSchool
	}{
		{game.Sorcerer, game.SchoolSorcerer}, {game.Archer, game.SchoolSorcerer},
		{game.Cleric, game.SchoolCleric}, {game.Paladin, game.SchoolCleric},
	} {
		if got := game.SpellSchoolOf(tc.c); got != tc.s {
			t.Errorf("%v 用第 %v 系，預期 %v", tc.c, got, tc.s)
		}
	}
}

// 治療系那七條的效果：清掉哪些狀況位元、加幾點生命。
//
// 遮罩是從 `2CAST1.OVL` 的 handler 逐行讀出來的 —— 每一支清掉哪幾位，
// 位元的語意就是那條法術治的東西。
func TestHealingSpellEffects(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:6]...)
	s := game.NewSession(w, party, nil, 1)
	me := -1
	for i := range party {
		if party[i].Class == game.Cleric {
			me = i
		}
	}
	if me < 0 {
		t.Skip("前六人裡沒有牧師")
	}
	// 名冊的牧師是一級，法力等級 1；這裡測的是效果不是成長，
	// 所以直接把法力等級拉到 9（手冊的上限）。
	prep := func(cond byte, hp, sp int) {
		party[me].SetFieldByte(38, 0x00, cond)
		// 法力等級寫進記錄（+114）—— 改 Character.SL 會在下一次
		// SetFieldByte 重新解析時被蓋掉。
		party[me].SetFieldByte(114, 0x00, 9)
		party[me].SetFieldByte(92, 0x00, 99) // 寶石，高階治療術要付
		party[me].HP, party[me].SP = hp, sp
	}

	// 急救術（第 3 條）：加 8 點生命，並清掉 bit 4。
	party[me].Learn(4)
	prep(game.CondBitWeak, 1, 99)
	r := s.Cast(me, 4)
	if !r.OK {
		t.Fatalf("急救術施不出來：%s", r.Reason)
	}
	if party[me].HP != 9 {
		t.Errorf("急救術後生命 %d，預期 9", party[me].HP)
	}
	if party[me].CondBits&game.CondBitWeak != 0 {
		t.Error("急救術沒有清掉那一位狀況")
	}

	// 解毒術（第 16 條）只清中毒，不動疾病。
	party[me].Learn(17)
	prep(game.CondBitPoisoned|game.CondBitDiseased, 5, 99)
	if r := s.Cast(me, 17); !r.OK {
		t.Fatalf("解毒術施不出來：%s", r.Reason)
	}
	if party[me].CondBits&game.CondBitPoisoned != 0 {
		t.Error("解毒術沒有清掉中毒")
	}
	if party[me].CondBits&game.CondBitDiseased == 0 {
		t.Error("解毒術把疾病也清掉了")
	}

	// 復活術（第 39 條）只在狀況正好是 0x81 時有效。
	party[me].Learn(40)
	prep(game.CondPetrified, 1, 99) // 石化不是死亡
	if r := s.Cast(me, 40); r.Effect != "沒有效果。" {
		t.Errorf("對石化用復活術得到 %q（%s）", r.Effect, r.Reason)
	}
	prep(game.CondDeadBits, 1, 99)
	if r := s.Cast(me, 40); r.Effect == "沒有效果。" {
		t.Error("對死亡用復活術卻沒有效果")
	}
	if party[me].CondBits != 0 {
		t.Errorf("復活後狀況是 %#02x，預期 0", party[me].CondBits)
	}

	// 生命不會超過上限。
	party[me].Learn(8)
	prep(0, party[me].MaxHP, 99)
	if r := s.Cast(me, 8); !r.OK {
		t.Fatal(r.Reason)
	}
	if party[me].HP != party[me].MaxHP {
		t.Errorf("治傷術把生命加到 %d，上限是 %d", party[me].HP, party[me].MaxHP)
	}
}

// 全域計數型法術要寫進與事件腳本共用的那一份全域。
//
// 四個界傳送術寫的 ds:03DC–03DF 正是腳本選擇器 0x27–0x2A，
// 所以施法之後腳本問得出「開了哪幾道元素之門」。
func TestGlobalCounterSpells(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:6]...)
	s := game.NewSession(w, party, nil, 1)
	me := -1
	for i := range party {
		if party[i].Class == game.Cleric {
			me = i
		}
	}
	if me < 0 {
		t.Skip("前六人裡沒有牧師")
	}
	ready := func(n int) {
		party[me].Learn(n)
		party[me].SetFieldByte(114, 0x00, 9)
		party[me].SetFieldByte(92, 0x00, 99)
		party[me].SP = 99
	}

	// 水界傳送術（牧師第 36 條）→ ds:03DC = 1，也就是腳本選擇器 0x27。
	ready(36)
	if r := s.Cast(me, 36); !r.OK {
		t.Fatalf("水界傳送術施不出來：%s", r.Reason)
	}
	if got := w.Global(0x27); got != 1 {
		t.Errorf("選擇器 0x27 讀到 %d，預期 1", got)
	}

	// 製造食物（牧師第 16 條）：+8，上限 40。
	ready(16)
	party[me].SetFieldByte(37, 0x00, 35)
	if r := s.Cast(me, 16); !r.OK {
		t.Fatalf("製造食物施不出來：%s", r.Reason)
	}
	if party[me].Food != 40 {
		t.Errorf("製造食物後是 %d，預期夾在 40", party[me].Food)
	}

	// 拒絕傷害（牧師第 12 條）→ ds:03D7 = 等級 + 20。
	ready(12)
	if r := s.Cast(me, 12); !r.OK {
		t.Fatalf("拒絕傷害施不出來：%s", r.Reason)
	}
	if got := w.Globals[0x03D7]; int(got) != party[me].Level+20 {
		t.Errorf("ds:03D7 是 %d，預期等級 %d + 20", got, party[me].Level)
	}
}
