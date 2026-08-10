package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 屬性是三次 rand(10,79)/10 的和，所以永遠落在 3–21。
func TestRollRange(t *testing.T) {
	r := game.NewRand(4242)
	lo, hi := 99, 0
	for i := 0; i < 3000; i++ {
		n := game.RollNewCharacter(r)
		for _, v := range n.Attr {
			if v < 3 || v > 21 {
				t.Fatalf("擲出 %d，超出 3–21", v)
			}
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	if lo > 5 || hi < 19 {
		t.Errorf("三千次的範圍只有 %d–%d，分佈不對", lo, hi)
	}
	t.Logf("三千次的範圍 %d–%d", lo, hi)
}

// 職業門檻抄自 sub_18952 的八個 case。
func TestClassRequirements(t *testing.T) {
	full := func(v int) [game.NumAttrs]int {
		var a [game.NumAttrs]int
		for i := range a {
			a[i] = v
		}
		return a
	}
	// 全 12：一個職業都選不了
	if e := game.EligibleClasses(full(12)); e[game.Knight] || e[game.Cleric] {
		t.Errorf("屬性全 12 卻選得了職業：%v", e)
	}
	// 全 13：門檻 13 的六個可以，門檻 15 的兩個不行
	e := game.EligibleClasses(full(13))
	for _, c := range []game.Class{game.Paladin, game.Archer, game.Cleric,
		game.Sorcerer, game.Robber, game.Ninja} {
		if !e[c] {
			t.Errorf("屬性全 13 選不了 %v", c)
		}
	}
	if e[game.Knight] || e[game.Barbarian] {
		t.Error("武士與野蠻人的門檻是 15，全 13 不該過")
	}
	if e := game.EligibleClasses(full(15)); !e[game.Knight] || !e[game.Barbarian] {
		t.Error("屬性全 15 卻當不了武士或野蠻人")
	}

	// 逐條驗「只有這幾項達標」
	only := func(v int, idx ...int) [game.NumAttrs]int {
		var a [game.NumAttrs]int
		for i := range a {
			a[i] = 3
		}
		for _, i := range idx {
			a[i] = v
		}
		return a
	}
	for _, tc := range []struct {
		class game.Class
		attrs [game.NumAttrs]int
	}{
		{game.Knight, only(15, 0)},
		{game.Paladin, only(13, 0, 2, 3)},
		{game.Archer, only(13, 1, 5)},
		{game.Cleric, only(13, 2)},
		{game.Sorcerer, only(13, 1)},
		{game.Robber, only(13, 6)},
		{game.Ninja, only(13, 4, 5)},
		{game.Barbarian, only(15, 3)},
	} {
		if !game.EligibleClasses(tc.attrs)[tc.class] {
			t.Errorf("%v 的門檻沒過，屬性 %v", tc.class, tc.attrs)
		}
	}
	// 聖騎士少一項就不行
	if game.EligibleClasses(only(13, 0, 2))[game.Paladin] {
		t.Error("聖騎士只達成兩項就過了 —— 三項都要")
	}
}

// 對調兩項屬性之後，能選的職業要跟著變。
func TestExchangeChangesEligibility(t *testing.T) {
	var n game.NewCharacter
	n.Attr = [game.NumAttrs]int{3, 3, 3, 3, 3, 3, 16} // 只有運氣高
	if !n.Eligible(game.Robber) || n.Eligible(game.Knight) {
		t.Fatal("起始條件不對")
	}
	n.Exchange(0, 6) // 力量與運氣對調
	if n.Eligible(game.Robber) || !n.Eligible(game.Knight) {
		t.Errorf("對調之後可選職業沒跟著變：%v", n.Attr)
	}
}

// 四樣選齊、名字打了才能定案。
func TestFinishNeedsEverything(t *testing.T) {
	var n game.NewCharacter
	n.Attr = [game.NumAttrs]int{16, 16, 16, 16, 16, 16, 16}
	if _, ok := n.Finish(); ok {
		t.Error("什麼都沒選就定案了")
	}
	n.SetClass(game.Knight)
	n.SetRace(game.Human)
	n.SetAlign(game.Good)
	n.SetSex(0)
	if _, ok := n.Finish(); ok {
		t.Error("沒有名字就定案了")
	}
	n.Name = "測試者"
	c, ok := n.Finish()
	if !ok {
		t.Fatal("四樣都選了還是定案不了")
	}
	if c.Level != 1 || c.HP < 1 || c.HP != c.MaxHP {
		t.Errorf("新角色的狀態不對：等級 %d 生命 %d/%d", c.Level, c.HP, c.MaxHP)
	}
	if c.Current[game.Might] != 16 || c.Luck != 16 {
		t.Errorf("屬性沒帶進去：力量 %d 運氣 %d", c.Current[game.Might], c.Luck)
	}

	// 門檻沒過的職業不能定案
	n.Attr[0] = 3
	n.SetClass(game.Knight)
	if _, ok := n.Finish(); ok {
		t.Error("力量 3 也當得了武士")
	}
}
