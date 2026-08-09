package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 攻擊次數的除數來自原版的 ds:1012：武士／遊俠／弓箭手／野蠻人是 1，
// 巫師是 4 —— 所以同樣等級下巫師的攻擊次數最少。
func TestAttacksPerRoundByClass(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	byClass := map[game.Class]*game.Character{}
	for i := range cs {
		byClass[cs[i].Class] = &cs[i]
	}
	for _, c := range byClass {
		c.Level = 12
	}
	knight := byClass[game.Knight].AttacksPerRound()
	sorc := byClass[game.Sorcerer].AttacksPerRound()
	cleric := byClass[game.Cleric].AttacksPerRound()
	if knight <= sorc {
		t.Errorf("第 12 級：武士 %d 次、巫師 %d 次，武士應該多", knight, sorc)
	}
	if cleric <= sorc {
		t.Errorf("第 12 級：牧師 %d 次、巫師 %d 次，牧師應該多", cleric, sorc)
	}
	// 第一級誰都至少一次
	for k, c := range byClass {
		c.Level = 1
		if n := c.AttacksPerRound(); n < 1 {
			t.Errorf("%v 第一級的攻擊次數是 %d", k, n)
		}
	}
}

// 遭遇類別要落在 0–6，而且七類都出得來。
func TestEncounterBandDistribution(t *testing.T) {
	r := game.NewRand(0x2468)
	seen := map[int]int{}
	for i := 0; i < 4000; i++ {
		b := game.RollEncounterBand(r)
		if b < 0 || b > 6 {
			t.Fatalf("類別是 %d", b)
		}
		seen[b]++
	}
	// 門檻 25/40/50/55/70/75/100 → 各段寬度 25/15/10/5/15/5/25
	if seen[0] == 0 || seen[6] == 0 {
		t.Errorf("四千次沒出現過第 0 或第 6 類：%v", seen)
	}
	// 第 0 類（1–25）應該比第 3 類（51–55）多得多
	if seen[0] <= seen[3] {
		t.Errorf("第 0 類 %d 次、第 3 類 %d 次，門檻寬度是 25 比 5", seen[0], seen[3])
	}
}

// 挑出來的怪物編號要落在合法範圍，難度越高編號越大。
func TestMonsterIndexByDifficulty(t *testing.T) {
	r := game.NewRand(13)
	lowSum, highSum := 0, 0
	const n = 500
	for i := 0; i < n; i++ {
		lowSum += game.RollMonsterIndex(r, 1)
		highSum += game.RollMonsterIndex(r, 3)
	}
	if lowSum/n >= highSum/n {
		t.Errorf("難度 1 的平均編號 %d 不低於難度 3 的 %d", lowSum/n, highSum/n)
	}
	for i := 0; i < 200; i++ {
		if idx := game.RollMonsterIndex(r, 2); idx < 0 || idx > 255 {
			t.Fatalf("怪物編號是 %d", idx)
		}
	}
}
