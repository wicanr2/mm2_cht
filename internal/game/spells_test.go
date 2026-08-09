package game_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
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
	prep(game.CondBitAsleep, 1, 99)
	r := s.Cast(me, 4)
	if !r.OK {
		t.Fatalf("急救術施不出來：%s", r.Reason)
	}
	if party[me].HP != 9 {
		t.Errorf("急救術後生命 %d，預期 9", party[me].HP)
	}
	if party[me].CondBits&game.CondBitAsleep != 0 {
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

// inRange 判斷傷害落在 [lo,hi] 或它減半之後的區間。
func inRange(v, lo, hi int) bool {
	return (v >= lo && v <= hi) || (v >= lo/2 && v <= hi/2)
}

// 隊伍增益是計數器、戰鬥旗標是一次性，兩種形狀要分清楚。
func TestBuffSpells(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	defs, err := monsters.Parse(orig(t, "MONSTERS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:6]...)
	s := game.NewSession(w, party, defs, 55)
	me := -1
	for i := range party {
		if party[i].Class == game.Cleric {
			me = i
		}
	}
	if me < 0 {
		t.Skip("前六人裡沒有牧師")
	}
	party[me].SetFieldByte(114, 0x00, 9)

	// 祝福術（牧師第 3 條）：ds:03E3 每施一次 +1，不會擋。
	party[me].Learn(3)
	for i := 1; i <= 3; i++ {
		party[me].SP, party[me].Gems = 99, 99
		r := s.Cast(me, 3)
		if !r.OK {
			t.Fatalf("祝福術施不出來：%s", r.Reason)
		}
		if got := s.World.Globals[0x03E3]; int(got) != i {
			t.Fatalf("第 %d 次祝福之後計數器是 %d", i, got)
		}
	}

	// 鷹眼術是巫師第 8 條，每級 +5、上限 250。
	if wiz := -1; true {
		for i := range party {
			if party[i].Class == game.Sorcerer {
				wiz = i
			}
		}
		if wiz >= 0 {
			party[wiz].SetFieldByte(114, 0x00, 9)
			party[wiz].Learn(8)
			lv := int(party[wiz].Level)
			party[wiz].SP, party[wiz].Gems = 99, 99
			if r := s.Cast(wiz, 8); !r.OK {
				t.Fatalf("鷹眼術施不出來：%s", r.Reason)
			}
			want := byte(5 * lv)
			if 5*lv > 250 {
				want = 250
			}
			if got := s.World.Globals[0x03E0]; got != want {
				t.Errorf("鷹眼術之後 ds:03E0 是 %d，等級 %d 該是 %d", got, lv, want)
			}
		}
	}

	// 喚醒術（牧師第 2 條）整隊清掉沈睡位元，重症的不動。
	party[me].Learn(2)
	party[0].SetFieldByte(38, 0x00, game.CondBitAsleep)
	party[1].SetFieldByte(38, 0x00, game.CondPetrified) // 石化，喚不醒
	party[me].SP, party[me].Gems = 99, 99
	if r := s.Cast(me, 2); !r.OK {
		t.Fatalf("喚醒術施不出來：%s", r.Reason)
	}
	if party[0].CondBits&game.CondBitAsleep != 0 {
		t.Error("喚醒術沒把沈睡清掉")
	}
	if party[1].CondBits != game.CondPetrified {
		t.Errorf("喚醒術動到了石化的隊員：%#02x", party[1].CondBits)
	}

	// 回復陣營（牧師第 24 條）把 +13 抄到 +106。
	party[me].Learn(24)
	party[me].SetFieldByte(106, 0x00, 2)
	party[me].SP, party[me].Gems = 99, 99
	s.Cast(me, 24)
	if got, want := party[me].FieldByte(106), party[me].FieldByte(0x0D); got != want {
		t.Errorf("回復陣營之後 +106 是 %d，+13 是 %d", got, want)
	}

	// 持續照明術（牧師第 19 條）一次 +20，與照明術共用 ds:03D5。
	party[me].Learn(19)
	party[me].SP, party[me].Gems = 99, 99
	s.Cast(me, 19)
	if got := s.World.Globals[0x03D5]; got != 20 {
		t.Errorf("持續照明術之後 ds:03D5 是 %d，該是 20", got)
	}

	// 神之干涉（牧師第 45 條）：一場戰鬥只生效一次。
	party[me].Learn(45)
	s.Fight = &game.Encounter{Party: s.Combatants(),
		Monsters: []game.Combatant{game.NewMonster(defs[1])}}
	party[me].SP, party[me].Gems = 99, 99
	if r := s.Cast(me, 45); r.Effect != "神明介入了。" {
		t.Fatalf("第一次神之干涉得到 %q", r.Effect)
	}
	party[me].SP, party[me].Gems = 99, 99
	if r := s.Cast(me, 45); r.Effect != "已經生效了。" {
		t.Errorf("第二次神之干涉得到 %q，應該被擋", r.Effect)
	}
	// 換一場戰鬥就重來。
	s.Fight = &game.Encounter{Party: s.Combatants(),
		Monsters: []game.Combatant{game.NewMonster(defs[1])}}
	party[me].SP, party[me].Gems = 99, 99
	if r := s.Cast(me, 45); r.Effect != "神明介入了。" {
		t.Errorf("新戰鬥的神之干涉得到 %q", r.Effect)
	}
}

// 狀態法術要設對位元，而且抗睡的怪物擋得下催眠術。
func TestStatusSpells(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	defs, err := monsters.Parse(orig(t, "MONSTERS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:6]...)
	s := game.NewSession(w, party, defs, 777)
	me := -1
	for i := range party {
		if party[i].Class == game.Sorcerer {
			me = i
		}
	}
	if me < 0 {
		t.Skip("前六人裡沒有巫師")
	}
	party[me].SetFieldByte(114, 0x00, 9)

	// 挑一隻好下手的：不抗睡、不抗法術狀態、抗魔法 0、編號小。
	easy := -1
	for i := range defs {
		d := defs[i]
		if !d.Resists[4] && !d.Resists[5] && d.MagicResistIndex == 0 && d.Index <= 3 && d.Name != "" {
			easy = i
			break
		}
	}
	if easy < 0 {
		t.Fatal("找不到好下手的怪物")
	}

	e := &game.Encounter{Party: s.Combatants()}
	s.Fight = e
	party[me].Learn(7) // 巫師第 7 條 = 催眠術
	slept := 0
	for i := 0; i < 40; i++ {
		m := game.NewMonster(defs[easy])
		e.Monsters = []game.Combatant{m}
		party[me].SP, party[me].Gems = 99, 99
		if r := s.Cast(me, 7); !r.OK {
			t.Fatalf("催眠術施不出來：%s", r.Reason)
		}
		if m.Status&game.MonSlept != 0 {
			slept++
			if m.CombatCondition() != game.CondAsleep {
				t.Fatalf("設了沈睡位元但狀況是 %v", m.CombatCondition())
			}
			if m.CombatCondition().Acts() {
				t.Fatal("沈睡的怪物還能行動")
			}
		}
	}
	if slept < 35 {
		t.Errorf("40 次催眠術只睡著 %d 次（編號 %d、抗魔法 0，該幾乎全中）", slept, defs[easy].Index)
	}

	// 抗睡的怪物完全免疫（屬性 5 是旗標，不是機率）。
	proof := -1
	for i := range defs {
		if defs[i].Resists[4] && defs[i].Index <= 60 {
			proof = i
			break
		}
	}
	if proof < 0 {
		t.Fatal("找不到抗睡的怪物")
	}
	for i := 0; i < 30; i++ {
		m := game.NewMonster(defs[proof])
		e.Monsters = []game.Combatant{m}
		party[me].SP, party[me].Gems = 99, 99
		s.Cast(me, 7)
		if m.Status != 0 {
			t.Fatalf("%s 抗睡卻被催眠術設了 %#02x", defs[proof].Name, m.Status)
		}
	}

	// 死亡之指直接判死（第三層擋下時就完全沒事，不會半死不活）。
	party[me].Learn(28) // 巫師第 28 條 = 死亡之指
	killed, intact := 0, 0
	for i := 0; i < 40; i++ {
		m := game.NewMonster(defs[easy])
		e.Monsters = []game.Combatant{m}
		hp := m.CombatHP()
		party[me].SP, party[me].Gems = 99, 99
		s.Cast(me, 28)
		switch {
		case m.CombatCondition() == game.CondDead:
			killed++
		case m.CombatHP() == hp && m.Status == 0:
			intact++
		default:
			t.Fatalf("死亡之指留下半死不活的怪物：HP %d/%d 狀態 %#02x", m.CombatHP(), hp, m.Status)
		}
	}
	if killed == 0 {
		t.Error("死亡之指 40 次一隻都沒殺掉")
	}
	t.Logf("死亡之指：殺掉 %d、擋下 %d", killed, intact)
}

// 攻擊法術要真的打到怪物，傷害落在 rand(1,N)+M 的範圍內。
func TestDamageSpells(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	defs, err := monsters.Parse(orig(t, "MONSTERS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:6]...)
	s := game.NewSession(w, party, defs, 4321)
	me := -1
	for i := range party {
		if party[i].Class == game.Sorcerer {
			me = i
		}
	}
	if me < 0 {
		t.Skip("前六人裡沒有巫師")
	}
	party[me].Learn(4) // 巫師第 4 條 = 火箭術
	party[me].SetFieldByte(114, 0x00, 9)
	party[me].SP = 99

	// 沒在戰鬥中要說清楚。
	if r := s.Cast(me, 4); r.Effect != "不在戰鬥中。" {
		t.Errorf("不在戰鬥中卻得到 %q", r.Effect)
	}

	// 擺一場戰鬥再施。火箭術 rand(1,5)+3 → 4–8 點。
	e := &game.Encounter{Party: s.Combatants()}
	target := game.NewMonster(defs[100])
	e.Monsters = append(e.Monsters, target)
	s.Fight = e
	hp := target.CombatHP()
	party[me].SP = 99
	r := s.Cast(me, 4)
	if !r.OK {
		t.Fatalf("火箭術施不出來：%s", r.Reason)
	}
	// 第三層可能把傷害減半，所以兩個區間都算合格。
	dmg := hp - target.CombatHP()
	if !inRange(dmg, 4, 8) {
		t.Errorf("火箭術造成 %d 點傷害，預期 4–8（或減半後 2–4）", dmg)
	}
	if r.Effect == "" {
		t.Error("攻擊法術沒有播報")
	}

	// 能量爆破術（巫師第 3 條）隨等級累加：等級 × (1d5 + 1)。
	party[me].Learn(3)
	lv := int(party[me].Level)
	if lv < 1 {
		t.Fatalf("施法者等級 %d", lv)
	}
	for i := 0; i < 20; i++ {
		target = game.NewMonster(defs[100])
		e.Monsters = []game.Combatant{target}
		hp = target.CombatHP()
		party[me].SP = 99
		party[me].Gems = 99
		if r := s.Cast(me, 3); !r.OK {
			t.Fatalf("能量爆破術施不出來：%s", r.Reason)
		}
		dmg := hp - target.CombatHP()
		if !inRange(dmg, 2*lv, 6*lv) {
			t.Fatalf("能量爆破術造成 %d 點傷害，預期 %d–%d 或其減半（等級 %d）", dmg, 2*lv, 6*lv, lv)
		}
	}

	// 抗魔法是機率不是旗標：抗性 90 的怪物擲 rand(等級, 90) 幾乎都擋得下
	// （只有擲出正好 90 才過），抗性 0 的一次都擋不下。
	// 表的第八格（100）值域內沒有怪物用到，所以「必定擋下」無法實測。
	countHits := func(def monsters.Monster, n int) int {
		hits := 0
		for i := 0; i < n; i++ {
			target := game.NewMonster(def)
			e.Monsters = []game.Combatant{target}
			hp := target.CombatHP()
			party[me].SP, party[me].Gems = 99, 99
			if r := s.Cast(me, 4); !r.OK {
				t.Fatalf("火箭術施不出來：%s", r.Reason)
			}
			if target.CombatHP() < hp {
				hits++
			}
		}
		return hits
	}
	var high, none *monsters.Monster
	for i := range defs {
		if defs[i].Resists[0] {
			continue // 抗火的會混進屬性層
		}
		if high == nil && defs[i].MagicResistIndex == 6 {
			high = &defs[i]
		}
		if none == nil && defs[i].MagicResistIndex == 0 {
			none = &defs[i]
		}
	}
	if high == nil || none == nil {
		t.Fatal("找不到對照組")
	}
	if got := countHits(*high, 200); got > 10 {
		t.Errorf("%s 抗魔法 90，200 次裡被打中 %d 次（預期個位數）", high.Name, got)
	}
	if got := countHits(*none, 200); got != 200 {
		t.Errorf("%s 抗魔法 0，200 次裡只被打中 %d 次", none.Name, got)
	}

	// 第三層：擲 rand(1,191) 不超過怪物編號就減半。編號 191 以上必定減半，
	// 編號 1 幾乎不會。用固定 100 點的分裂術量，減半後正好 50。
	var tough, weak *monsters.Monster
	for i := range defs {
		if defs[i].MagicResistIndex != 0 || defs[i].Resists[0] {
			continue
		}
		if weak == nil && defs[i].Index <= 2 {
			weak = &defs[i]
		}
		if defs[i].Index >= 191 {
			tough = &defs[i]
		}
	}
	party[me].Learn(27) // 巫師第 27 條 = 分裂術
	countFull := func(def monsters.Monster, n int) int {
		full := 0
		for i := 0; i < n; i++ {
			target := game.NewMonster(def)
			target.HP = 9999
			e.Monsters = []game.Combatant{target}
			party[me].SP, party[me].Gems = 99, 99
			if r := s.Cast(me, 27); !r.OK {
				t.Fatalf("分裂術施不出來：%s", r.Reason)
			}
			switch 9999 - target.CombatHP() {
			case 100:
				full++
			case 50:
			default:
				t.Fatalf("%s 吃了 %d 點，分裂術只該是 100 或 50", def.Name, 9999-target.CombatHP())
			}
		}
		return full
	}
	if tough != nil {
		if got := countFull(*tough, 50); got != 0 {
			t.Errorf("%s（編號 %d）50 次裡有 %d 次沒減半", tough.Name, tough.Index, got)
		}
	}
	if weak != nil {
		if got := countFull(*weak, 50); got < 45 {
			t.Errorf("%s（編號 %d）50 次裡只有 %d 次全額", weak.Name, weak.Index, got)
		}
	}

	// 屬性抗性是旗標不是機率：抗火的怪物對火箭術完全免疫。
	// 挑抗魔法為 0 的，免得兩層混在一起分不清是哪一層擋的。
	fireproof := -1
	for i := range defs {
		if defs[i].Resists[0] && defs[i].MagicResistIndex == 0 {
			fireproof = i
			break
		}
	}
	if fireproof < 0 {
		t.Fatal("沒有抗火且抗魔法為 0 的怪物")
	}
	for i := 0; i < 30; i++ {
		target = game.NewMonster(defs[fireproof])
		e.Monsters = []game.Combatant{target}
		hp = target.CombatHP()
		party[me].SP, party[me].Gems = 99, 99
		if r := s.Cast(me, 4); !r.OK {
			t.Fatalf("火箭術施不出來：%s", r.Reason)
		}
		if target.CombatHP() != hp {
			t.Fatalf("%s 抗火卻被火箭術扣了 %d 點", defs[fireproof].Name, hp-target.CombatHP())
		}
	}
	// 同一隻怪對無屬性的傷痛術沒有免疫。
	party[me].Learn(3)
	dealt := false
	for i := 0; i < 10; i++ {
		target = game.NewMonster(defs[fireproof])
		e.Monsters = []game.Combatant{target}
		hp = target.CombatHP()
		party[me].SP, party[me].Gems = 99, 99
		s.Cast(me, 3)
		if target.CombatHP() < hp {
			dealt = true
			break
		}
	}
	if !dealt {
		t.Errorf("%s 對無屬性的能量爆破術也免疫，抗性大概掛在錯的地方", defs[fireproof].Name)
	}
}
