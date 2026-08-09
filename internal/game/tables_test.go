package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 揮擊次數的除數來自原版的 **ds:101A**：武士／遊俠／野蠻人是 4、
// 弓箭手／賊／忍者是 5、牧師是 7、巫師是 10 —— 同等級下巫師最少。
//
// 相鄰的 ds:1012 是命中擲骰的上限除數，不是這個。兩張表形狀相似，
// 對調之後在低等級看不出差別，要到高等級才會露餡。
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
		c.Level = 21
	}
	knight := byClass[game.Knight].AttacksPerRound()
	sorc := byClass[game.Sorcerer].AttacksPerRound()
	cleric := byClass[game.Cleric].AttacksPerRound()
	if want := 21/4 + 1; knight != want {
		t.Errorf("第 21 級的武士揮擊 %d 次，預期 %d", knight, want)
	}
	if want := 21/10 + 1; sorc != want {
		t.Errorf("第 21 級的巫師揮擊 %d 次，預期 %d", sorc, want)
	}
	if want := 21/7 + 1; cleric != want {
		t.Errorf("第 21 級的牧師揮擊 %d 次，預期 %d", cleric, want)
	}
	if !(knight > cleric && cleric > sorc) {
		t.Errorf("揮擊次數的職業順序不對：武士 %d、牧師 %d、巫師 %d", knight, cleric, sorc)
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
