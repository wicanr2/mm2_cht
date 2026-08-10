package game_test

import "testing"


// 勇氣術加的是戰鬥用的等級（+113），不是本體（+32），戰鬥結束會復原。
func TestHeroismRaisesBattleLevel(t *testing.T) {
	s := session(t)
	c := &s.Party[0]
	c.Level, c.BattleLevel = 5, 5
	c.SP, c.MaxSP, c.Gems = 40, 40, 40
	swings := c.AttacksPerRound()

	c.BattleLevel = c.Level + 6 // 直接套效果，不經施法代價
	if c.Level != 5 {
		t.Errorf("本體等級被動到了：%d", c.Level)
	}
	if got := c.EffectiveLevel(); got != 11 {
		t.Errorf("戰鬥等級 %d，預期 11", got)
	}
	if now := c.AttacksPerRound(); now < swings {
		t.Errorf("等級提昇後揮擊次數反而變少：%d → %d", swings, now)
	}

	s.EndCombat()
	if got := c.EffectiveLevel(); got != 5 {
		t.Errorf("戰鬥結束後戰鬥等級是 %d，預期回到 5", got)
	}
}
