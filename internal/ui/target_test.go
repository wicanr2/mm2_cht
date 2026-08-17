package ui_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

// startFightN 擺一場有 n 隻怪的仗，每隻血很多、打不太動人 ——
// 這些測試要驗的是「打到誰」，不是誰先倒。
func startFightN(t *testing.T, s *ui.Session, n, front int) []*game.Monster {
	t.Helper()
	var ms []game.Combatant
	var out []*game.Monster
	for i := 0; i < n; i++ {
		var d monsters.Monster
		// Index 決定名字與圖，這裡各給不同的號碼才分得出打到哪一隻。
		d.Index, d.HP, d.Speed, d.AC = 0x20+i, 200, 1, 1
		m := game.NewMonster(d)
		m.Display = string(rune('甲' + i))
		ms = append(ms, m)
		out = append(out, m)
	}
	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	s.Game.Fight = &game.Encounter{Party: party, Monsters: ms, Front: front}
	s.Mode = ui.ModeCombat
	return out
}

// 兩隻以上就要問打哪一隻，而且問的是「打得到的那幾隻」。
func TestAttackAsksWhichTarget(t *testing.T) {
	s := load(t)
	startFightN(t, s, 4, 2)
	if !s.Key(ui.KeyConfirm) {
		t.Fatal("按攻擊沒有反應")
	}
	if s.Mode != ui.ModeMenu || s.Menu == nil {
		t.Fatalf("沒有開目標選單，現在是 %v", s.Mode)
	}
	if len(s.Menu.Items) != 2 {
		t.Fatalf("近戰列了 %d 個目標，前排只有 2 隻：%v", len(s.Menu.Items), s.Menu.Items)
	}
	if !strings.Contains(s.Menu.Title, "打哪一隻") {
		t.Errorf("標題看不出要選什麼：%q", s.Menu.Title)
	}
}

// 射擊打得到後排，所以選單要列滿場上的怪（原版 ds:0508）。
func TestShootListsEveryMonster(t *testing.T) {
	s := load(t)
	startFightN(t, s, 4, 2)
	if !s.Key(ui.KeyShoot) {
		t.Fatal("按射擊沒有反應")
	}
	if s.Menu == nil {
		t.Fatal("射擊沒有開目標選單")
	}
	if len(s.Menu.Items) != 4 {
		t.Fatalf("射擊列了 %d 個目標，場上有 4 隻", len(s.Menu.Items))
	}
}

// 選了第二隻就要打第二隻 —— 這條就是 G1 的重點：玩家能集火。
func TestChosenTargetTakesTheHits(t *testing.T) {
	s := load(t)
	ms := startFightN(t, s, 3, 3)
	s.Key(ui.KeyConfirm)
	if s.Menu == nil {
		t.Fatal("沒有開目標選單")
	}
	s.Key(ui.KeyDown) // 游標移到第二隻
	if !s.Key(ui.KeyConfirm) {
		t.Fatal("選中之後沒有打")
	}
	if s.Mode == ui.ModeMenu {
		t.Fatal("選完還停在選單上")
	}
	if s.Game.Fight == nil {
		t.Fatal("這一回合不該分出勝負")
	}
	if got := s.Game.Fight.Target; got != 1 {
		t.Fatalf("集火目標是第 %d 隻，預期第 1 隻（0 起算）", got)
	}
	// 第一隻不該掉血；第二隻要看得出隊伍打過它。
	if ms[0].CombatHP() != 200 {
		t.Errorf("沒被選中的第一隻掉了血：%d", ms[0].CombatHP())
	}
	if ms[1].CombatHP() == 200 {
		t.Error("被選中的那一隻一點血都沒掉 —— 集火沒有生效")
	}
}

// 只剩一個目標就別問了（原版 `var_C <= 1` 直接打第 0 隻）。
func TestSingleTargetSkipsTheMenu(t *testing.T) {
	s := load(t)
	startFightN(t, s, 1, 1)
	if !s.Key(ui.KeyConfirm) {
		t.Fatal("按攻擊沒有反應")
	}
	if s.Mode == ui.ModeMenu {
		t.Fatal("只有一隻怪還問「打哪一隻」")
	}
}

// 倒下的不列。原版的陣列裡本來就沒有屍體，remake 留到結算才清，
// 所以要自己濾掉 —— 不濾的話選單上會有一格打了等於沒打。
func TestDownedMonstersAreNotListed(t *testing.T) {
	s := load(t)
	ms := startFightN(t, s, 3, 3)
	ms[1].TakeDamage(ms[1].CombatHP())
	s.Key(ui.KeyConfirm)
	if s.Menu == nil {
		t.Fatal("沒有開目標選單")
	}
	if len(s.Menu.Items) != 2 {
		t.Fatalf("列了 %d 個目標，站著的只有 2 隻：%v", len(s.Menu.Items), s.Menu.Items)
	}
	// 第二項要對到陣列的第三隻，不是第二隻。
	s.Key(ui.KeyDown)
	s.Key(ui.KeyConfirm)
	if got := s.Game.Fight.Target; got != 2 {
		t.Fatalf("選單第二項對到第 %d 隻，預期第 2 隻 —— 中間隔著倒下的那一隻", got)
	}
}

// Esc 取消：回合不推進，也不消耗行動（原版 `var_E--` 之後整段跳過）。
func TestEscapeCancelsWithoutSpendingTheRound(t *testing.T) {
	s := load(t)
	ms := startFightN(t, s, 3, 3)
	s.Key(ui.KeyConfirm)
	if s.Menu == nil {
		t.Fatal("沒有開目標選單")
	}
	if !s.Key(ui.KeyCancel) {
		t.Fatal("Esc 沒有反應")
	}
	if s.Mode != ui.ModeCombat {
		t.Fatalf("取消之後是 %v，預期回到戰鬥", s.Mode)
	}
	if s.Game.Fight.Round != 0 {
		t.Errorf("取消卻推進了回合：%d", s.Game.Fight.Round)
	}
	for i, m := range ms {
		if m.CombatHP() != 200 {
			t.Errorf("取消卻打到了第 %d 隻（剩 %d）", i, m.CombatHP())
		}
	}
}

// 快速戰鬥不問目標 —— 它是 remake 自己的便利功能，一路打完為止。
func TestQuickFightDoesNotAskForTarget(t *testing.T) {
	s := load(t)
	startFightN(t, s, 3, 3)
	if !s.Key(ui.KeyQuickFight) {
		t.Fatal("快速戰鬥沒有推進")
	}
	if s.Mode == ui.ModeMenu {
		t.Fatal("快速戰鬥跳出了目標選單")
	}
}
