package ui_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

// 一場必勝的仗：一隻 1 點血、速度 1 的怪。與 TestCombatVictoryNeedsSearch
// 用的是同一組數值。
func startFight(t *testing.T, s *ui.Session) {
	t.Helper()
	var d monsters.Monster
	d.Index, d.HP, d.SpecialUses, d.Speed, d.AC = 0x21, 1, 1, 1, 1
	// 掉落欄位要填：沒有它們就沒有戰利品，而「沒有戰利品」與
	// 「快速戰鬥沒走結算」在測試裡長得一樣。
	d.DropBand, d.GoldMode, d.GemDrop = 1, 1, true
	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	s.Game.Fight = &game.Encounter{
		Party: party, Monsters: []game.Combatant{game.NewMonster(d)}, Front: 1}
	s.Mode = ui.ModeCombat
}

// 快速戰鬥一鍵打完，而且走的是與 Enter 相同的結算 —— 戰利品照樣留下。
func TestQuickFightEndsBattle(t *testing.T) {
	s := load(t)
	startFight(t, s)
	if !s.Key(ui.KeyQuickFight) {
		t.Fatal("快速戰鬥沒有推進")
	}
	if s.Mode == ui.ModeCombat {
		t.Fatalf("按完還在戰鬥中：%v", s.Lines)
	}
	if s.Game.Fight != nil && !s.Game.Fight.Over() {
		t.Error("戰鬥沒有分出結果")
	}
	if s.Chest == nil {
		t.Error("勝利後沒有留下戰利品 —— 快速戰鬥該走同一條結算")
	}
}

// 沒有戰鬥時按快速戰鬥不該當掉，也不該假裝打了一場。
func TestQuickFightWithoutBattle(t *testing.T) {
	s := load(t)
	s.Mode = ui.ModeCombat
	s.Game.Fight = nil
	if !s.Key(ui.KeyQuickFight) {
		t.Fatal("沒有戰鬥時應該直接回到探索")
	}
	if s.Mode != ui.ModeExplore {
		t.Fatalf("模式是 %v，預期回到探索", s.Mode)
	}
}

// 設定畫面：切換之後選單上的文字要跟著變 —— 看不到現在是哪一邊
// 的設定畫面等於沒有設定畫面。
func TestSettingsTogglesRewardMode(t *testing.T) {
	s := load(t)
	if !s.Game.AutoClaimReward {
		t.Fatal("預設應該是自動入袋（polish-spec P11）")
	}
	if !s.Key(ui.KeySettings) || s.Mode != ui.ModeMenu {
		t.Fatalf("F2 沒有開設定，現在是 %v", s.Mode)
	}
	first := s.Menu.Items[0]
	if !strings.Contains(first, "自動") {
		t.Fatalf("第一項是 %q，看不出目前的設定", first)
	}
	if !s.Key(ui.KeyConfirm) {
		t.Fatal("選第一項沒有反應")
	}
	if s.Game.AutoClaimReward {
		t.Error("切換之後仍然是自動入袋")
	}
	if s.Mode != ui.ModeMenu {
		t.Fatalf("切換之後模式是 %v，預期還在設定畫面", s.Mode)
	}
	if got := s.Menu.Items[0]; got == first || !strings.Contains(got, "S") {
		t.Errorf("切換之後第一項是 %q，沒有反映新的設定", got)
	}
}

// 切成「照原版」之後，事件擺好的獎賞要留到按 S 才入袋。
func TestRewardWaitsForSearch(t *testing.T) {
	s := load(t)
	s.Game.AutoClaimReward = false
	s.Game.World.Reward = game.Reward{Pending: true, Gold: 500}
	before := s.Game.Party[0].Gold

	// 沒有寶箱、獎賞也還沒領：按 S 應該把它領走。
	if !s.Key(ui.KeySearch) {
		t.Fatal("按 S 沒有反應")
	}
	if s.Game.World.Reward.Pending {
		t.Error("按 S 之後獎賞還掛著")
	}
	got := 0
	for i := range s.Game.Party {
		got += s.Game.Party[i].Gold
	}
	if got <= before {
		t.Errorf("全隊金錢 %d，領獎賞之前第一位是 %d —— 錢沒進來", got, before)
	}
	if !strings.Contains(strings.Join(s.Lines, "|"), "500") {
		t.Errorf("播報沒有提到金額：%v", s.Lines)
	}
}
