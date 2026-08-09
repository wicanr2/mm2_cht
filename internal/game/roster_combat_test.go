package game_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// 造幾隻可辨識的怪物來看陣列怎麼搬。名字用 A、B、C…，搬錯會直接看出來。
func lineup(n int) []game.Combatant {
	out := make([]game.Combatant, n)
	for i := range out {
		m := game.NewMonster(monsters.Monster{Index: i, HP: 10, Actions: 1})
		m.Display = string(rune('A' + i))
		out[i] = m
	}
	return out
}

func names(cs []game.Combatant) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.CombatName())
	}
	return b.String()
}

// 移除一隻怪物之後，後面的要往前搬 —— 原版 `sub_18A22` 是
// 六個平行陣列各搬一格，這裡的切片刪除要等價。
func TestRemoveMonsterShiftsUp(t *testing.T) {
	e := &game.Encounter{Monsters: lineup(5)} // ABCDE
	if !e.RemoveMonster(1) {
		t.Fatal("刪不掉第 1 隻")
	}
	if got := names(e.Monsters); got != "ACDE" {
		t.Errorf("刪掉 B 之後是 %q，預期 \"ACDE\"", got)
	}
	if !e.RemoveMonster(3) { // 最後一隻
		t.Fatal("刪不掉最後一隻")
	}
	if got := names(e.Monsters); got != "ACD" {
		t.Errorf("再刪掉 E 之後是 %q，預期 \"ACD\"", got)
	}
	for _, i := range []int{-1, 3, 99} {
		if e.RemoveMonster(i) {
			t.Errorf("索引 %d 不該刪得掉", i)
		}
	}
}

// 前排數不能超過場上總數：死到剩三隻，前排就只剩三隻。
// 原版 `0x18A31` 在 `dec ds:0508` 之後立刻做這件事。
func TestRemoveMonsterClampsFront(t *testing.T) {
	e := &game.Encounter{Monsters: lineup(6)}
	e.Front = 6
	for want := 5; want >= 0; want-- {
		if !e.RemoveMonster(0) {
			t.Fatalf("剩 %d 隻時刪不掉", want+1)
		}
		if e.Front != want {
			t.Errorf("剩 %d 隻時前排是 %d，預期 %d", want, e.Front, want)
		}
	}
}

// 前排上限是 10：場上二十隻，前排也只有十隻。
func TestRollFrontBounds(t *testing.T) {
	r := game.NewRand(1234)
	for _, indoor := range []bool{false, true} {
		e := &game.Encounter{Monsters: lineup(20), Party: make([]game.Combatant, 6)}
		lo, hi := 99, -1
		for i := 0; i < 400; i++ {
			n := e.RollFront(r, indoor, 0)
			if n < 0 || n > game.MaxFront {
				t.Fatalf("室內=%v 擲出 %d，超出 [0, %d]", indoor, n, game.MaxFront)
			}
			if n < lo {
				lo = n
			}
			if n > hi {
				hi = n
			}
		}
		// 室外是 rand(10,39)/10+3 → 4–6；室內是 rand(10,69)/10 + 6/2 → 4–9。
		wantLo, wantHi := 4, 6
		if indoor {
			wantLo, wantHi = 4, 9
		}
		if lo != wantLo || hi != wantHi {
			t.Errorf("室內=%v 擲出的範圍是 %d–%d，預期 %d–%d", indoor, lo, hi, wantLo, wantHi)
		}
	}
}

// 難度旗標 2 減半、3 加倍（原版 `0x1969F`／`0x196A4`）。
func TestRollFrontDifficulty(t *testing.T) {
	e := &game.Encounter{Monsters: lineup(20), Party: make([]game.Combatant, 6)}
	half, plain, double := 0, 0, 0
	for i := 0; i < 200; i++ {
		r := game.NewRand(uint16(i + 1))
		half += e.RollFront(r, false, 2)
		r = game.NewRand(uint16(i + 1))
		plain += e.RollFront(r, false, 0)
		r = game.NewRand(uint16(i + 1))
		double += e.RollFront(r, false, 3)
	}
	if !(half < plain && plain < double) {
		t.Errorf("難度 2／無／3 的總和是 %d／%d／%d，應該遞增", half, plain, double)
	}
}

// 場上一隻不剩時前排是 0，而且不會變成負的。
func TestRollFrontEmptyField(t *testing.T) {
	e := &game.Encounter{Party: make([]game.Combatant, 6)}
	if n := e.RollFront(game.NewRand(7), true, 0); n != 0 {
		t.Errorf("場上沒有怪物，前排卻是 %d", n)
	}
}

// Reap 一次清掉所有倒下的，剩下的順序不變。
func TestReapKeepsOrder(t *testing.T) {
	e := &game.Encounter{Monsters: lineup(5)} // ABCDE
	e.Front = 5
	for _, i := range []int{1, 3} { // B、D 打死
		e.Monsters[i].TakeDamage(9999)
	}
	if n := e.Reap(); n != 2 {
		t.Errorf("清掉 %d 隻，預期 2", n)
	}
	if got := names(e.Monsters); got != "ACE" {
		t.Errorf("清完是 %q，預期 \"ACE\"", got)
	}
	if e.Front != 3 {
		t.Errorf("清完前排是 %d，預期 3", e.Front)
	}
}

// 倒下與逃走是同一條路，差別只在播報。
func TestLeaveMessage(t *testing.T) {
	for _, tc := range []struct {
		fled bool
		want string
	}{
		{false, "Skeleton goes down!"},
		{true, "Skeleton runs away!"},
	} {
		if got := game.LeaveMessage("Skeleton", tc.fled); got != tc.want {
			t.Errorf("fled=%v 得到 %q，預期 %q", tc.fled, got, tc.want)
		}
	}
}

// 驚嚇讓命中率減半、衰弱讓總傷害減半。兩者都在攻擊者的狀態位元組上
// （`ds:9F86`），而且衰弱是**動作結束後對總和**減半。
func TestMonsterStatusFlagsAffectAttack(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	def := monsters.Monster{Index: 0x30, HP: 20, Actions: 1, Attacks: 8, DamageDice: 6, Tier: 3}
	target := &game.Character{Name: "靶", HP: 9999, MaxHP: 9999, AC: 0}

	total := func(status byte, seed uint16) (hits, dmg int) {
		for i := 0; i < 300; i++ {
			m := game.NewMonster(def)
			m.Status = status
			res := game.Resolve(game.NewRand(seed+uint16(i)), m, target)
			hits += res.Hits
			dmg += res.Damage
		}
		return
	}

	baseHits, baseDmg := total(0, 1)
	frightHits, _ := total(game.MonFrightened, 1)
	_, weakDmg := total(game.MonWeakened, 1)

	if baseHits == 0 {
		t.Fatal("正常狀態一次都沒命中，這個對照沒有意義")
	}
	if frightHits >= baseHits {
		t.Errorf("驚嚇命中 %d 次、正常 %d 次，減半沒生效", frightHits, baseHits)
	}
	// 衰弱不影響命中，只影響傷害；總和應該落在一半附近。
	if weakDmg >= baseDmg || weakDmg*2 < baseDmg-baseDmg/10 {
		t.Errorf("衰弱總傷害 %d、正常 %d，不像減半", weakDmg, baseDmg)
	}
}

// 逃走判定：士氣層 3（門檻 255）永遠不逃；士氣層 0（門檻 3）
// 在隊伍夠強時會逃，而且逃走就從場上消失。
func TestTryFlee(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	party := []game.Combatant{&game.Character{Name: "強者", Level: 20, HP: 50, MaxHP: 50}}

	build := func(tier int) *game.Encounter {
		m := game.NewMonster(monsters.Monster{Index: 1, HP: 10, Actions: 1, MoraleTier: tier})
		return &game.Encounter{Party: party, Monsters: []game.Combatant{m}, Front: 1}
	}

	// 士氣層 3：門檻 255，隊伍再強也不逃。
	fled := 0
	for i := 0; i < 200; i++ {
		e := build(3)
		if e.TryFlee(game.NewRand(uint16(i+1)), 0, false) {
			fled++
		}
	}
	if fled != 0 {
		t.Errorf("士氣層 3 逃了 %d 次，應該一次都不逃", fled)
	}

	// 士氣層 0：門檻 3 < 20/2，擲 rand(1,100) <= 50 才逃，
	// 兩百次應該落在五成附近。
	//
	// 這裡共用同一顆 Rand ——「每次重播種再取第一個值」的分佈是偏的，
	// 而原版整場戰鬥只有一條隨機序列。
	r := game.NewRand(0x1234)
	fled = 0
	for i := 0; i < 200; i++ {
		e := build(0)
		if e.TryFlee(r, 0, false) {
			fled++
			if len(e.Monsters) != 0 {
				t.Fatal("逃走了卻還在場上")
			}
			if e.Front != 0 {
				t.Fatalf("逃走之後前排是 %d，預期 0", e.Front)
			}
		}
	}
	if fled < 60 || fled > 140 {
		t.Errorf("士氣層 0 兩百次逃了 %d 次，離五成太遠", fled)
	}

	// 隊伍太弱（最高等級 4 → 門檻 3 不小於 2）時逃不了。
	weak := []game.Combatant{&game.Character{Name: "菜鳥", Level: 4, HP: 10, MaxHP: 10}}
	fled = 0
	for i := 0; i < 200; i++ {
		m := game.NewMonster(monsters.Monster{Index: 1, HP: 10, Actions: 1})
		e := &game.Encounter{Party: weak, Monsters: []game.Combatant{m}, Front: 1}
		if e.TryFlee(game.NewRand(uint16(i+1)), 0, false) {
			fled++
		}
	}
	if fled != 0 {
		t.Errorf("隊伍等級 4 時逃了 %d 次，應該一次都不逃", fled)
	}

	// 禁逃旗標蓋過一切。
	e := build(0)
	if e.TryFlee(game.NewRand(1), 0, true) {
		t.Error("禁逃旗標沒有生效")
	}
}

// 心智渙散讓怪物完全不行動，而且不消耗行動額度
// （原版 `0x184A1` 在 `dec ds:9F9E` 之前就跳出去了）。
func TestMindlessBlocksAction(t *testing.T) {
	m := game.NewMonster(monsters.Monster{Index: 1, HP: 10, Actions: 3})
	m.ResetRound()
	m.Status = game.MonMindless
	r := game.NewRand(9)
	for i := 0; i < 5; i++ {
		if m.CanAct(r) {
			t.Fatalf("第 %d 次仍然行動了", i)
		}
	}
	// 解除之後額度要原封不動。
	m.Status = 0
	n := 0
	for m.CanAct(r) {
		n++
	}
	if n != 3 {
		t.Errorf("解除之後行動 %d 次，預期 3 —— 額度被心智渙散吃掉了", n)
	}
}

// 溜跑成功率是地圖屬性 +13。城鎮 100 表示一定跑得掉，
// 而且跑掉的只有下指令的那一個人。
func TestTryRun(t *testing.T) {
	party := func() []game.Combatant {
		return []game.Combatant{
			&game.Character{Name: "A", HP: 10, MaxHP: 10},
			&game.Character{Name: "B", HP: 10, MaxHP: 10},
			&game.Character{Name: "C", HP: 10, MaxHP: 10},
		}
	}
	r := game.NewRand(0x4321)

	// 成功率 100：rand(1,100) 永遠 < 100 除了擲出 100 那次。
	e := &game.Encounter{Party: party()}
	if !e.TryRun(r, 1, 100) {
		t.Error("成功率 100 卻沒跑掉（擲到 100 了？重跑一次看看）")
	}
	if len(e.Party) != 2 {
		t.Fatalf("跑掉之後隊伍剩 %d 人，預期 2", len(e.Party))
	}
	// 原版是把最後一格搬進空出來的那一格，所以順序會變成 A、C。
	if e.Party[0].CombatName() != "A" || e.Party[1].CombatName() != "C" {
		t.Errorf("跑掉之後是 %s、%s，預期 A、C（最後一格補洞）",
			e.Party[0].CombatName(), e.Party[1].CombatName())
	}

	// 成功率 0：一次都跑不掉。
	e = &game.Encounter{Party: party()}
	for i := 0; i < 100; i++ {
		if e.TryRun(r, 0, 0) {
			t.Fatal("成功率 0 竟然跑掉了")
		}
	}
	if len(e.Party) != 3 {
		t.Errorf("沒跑掉隊伍卻剩 %d 人", len(e.Party))
	}

	// 成功率 40 應該落在四成附近。
	got := 0
	for i := 0; i < 400; i++ {
		e := &game.Encounter{Party: party()}
		if e.TryRun(r, 0, 40) {
			got++
		}
	}
	if got < 130 || got > 190 {
		t.Errorf("成功率 40 跑掉 %d/400 次，離四成太遠", got)
	}
}

// 原版的地圖屬性：五座城鎮的溜跑成功率都是 100。
func TestRunChanceFromAttrib(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "workplace", "orig", "MM2", "ATTRIB.DAT"))
	if err != nil {
		t.Skip("沒有原版 ATTRIB.DAT，跳過")
	}
	as, err := game.ParseMapAttrs(b)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if got := as[i].RunChance(); got != 100 {
			t.Errorf("城鎮 %d 的溜跑成功率是 %d，預期 100", i, got)
		}
	}
	for _, a := range as {
		v := a.RunChance()
		if v < 20 || v > 100 || v%10 != 0 {
			t.Errorf("地圖 %d 的溜跑成功率是 %d，不像 10 的倍數的百分比", a.Index, v)
		}
	}
}
