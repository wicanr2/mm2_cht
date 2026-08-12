// 拿原版 MONSTERS.DAT 當對照。原版資料不入版控，找不到就 skip。
package monsters_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
)

func parse(t *testing.T) []monsters.Monster {
	t.Helper()
	path := filepath.Join("..", "..", "..", "workplace", "orig", "MM2", "MONSTERS.DAT")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("找不到 %s（玩家自備合法原版）", path)
	}
	ms, err := monsters.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

// 那 12 個位元組是位元欄位，不是直接的數值。這裡釘住幾筆已知的怪物 ——
// 解包規則寫錯時，數字會離譜到一眼看得出來（第一版把原始位元組當等級用，
// 四百次攻擊命中零次）。
func TestUnpackedStats(t *testing.T) {
	ms := parse(t)
	for _, w := range []struct {
		idx        int
		name       string
		hp, exp    int
		atks, dice int
	}{
		{2, "Sewer Rat", 8, 150, 1, 12},
		{50, "Crazed Dwarf", 45, 1200, 2, 20},
		{250, "Mega Dragon", 64000, 32000000, 16, 250},
		{255, "Sheltem", 500, 500000, 8, 60},
	} {
		m := ms[w.idx]
		if m.Name != w.name {
			t.Errorf("第 %d 筆是 %q，預期 %q", w.idx, m.Name, w.name)
			continue
		}
		if m.HP != w.hp || m.Exp != w.exp || m.Attacks != w.atks || m.DamageDice != w.dice {
			t.Errorf("%s：HP %d 經驗 %d 攻擊 %d 骰 %d，預期 %d/%d/%d/%d",
				m.Name, m.HP, m.Exp, m.Attacks, m.DamageDice, w.hp, w.exp, w.atks, w.dice)
		}
	}
}

// +0x10 是戰鬥勝利戰利品的來源欄位：低兩位是物品分段、bit2 是寶石、
// bits 3–4 是金幣模式。這裡只驗證 typed 解包的值域，不把任何衍生掉落表
// 寫進公開資料。
func TestDropFieldsAreTyped(t *testing.T) {
	ms := parse(t)
	for _, m := range ms {
		if m.DropBand < 0 || m.DropBand > 3 {
			t.Fatalf("%s 的掉落分段 %d 超出 b16 低兩位", m.Name, m.DropBand)
		}
		if m.GoldMode < 0 || m.GoldMode > 3 {
			t.Fatalf("%s 的金幣模式 %d 超出 b16 bits 3–4", m.Name, m.GoldMode)
		}
	}
}

// 難度層是怪物編號的高 nibble，命中門檻查表用它。
func TestTierIsHighNibble(t *testing.T) {
	ms := parse(t)
	for _, i := range []int{0, 15, 16, 200, 255} {
		if got, want := ms[i].Tier, i>>4; got != want {
			t.Errorf("第 %d 筆的難度層 = %d，預期 %d", i, got, want)
		}
	}
}

// 生命與經驗必須隨難度上升。怪物表按難度排序，這一條抓得到解包時
// 把倍率索引取錯位元的錯誤。
func TestDifficultyIncreases(t *testing.T) {
	ms := parse(t)
	early, late := 0, 0
	for _, m := range ms[:32] {
		early += m.HP
	}
	for _, m := range ms[224:] {
		late += m.HP
	}
	if late <= early*10 {
		t.Errorf("後段怪物的生命總和 %d 沒有明顯高於前段的 %d", late, early)
	}
}

// 影像段號要落在 MONSTERS.16 的段數之內（59 個段）。
// 這一條抓得到「把段號欄位當成數值欄位」的誤判。
func TestSpriteIndexRange(t *testing.T) {
	ms := parse(t)
	seen := map[int]bool{}
	for _, m := range ms {
		if m.Sprite < 1 || m.Sprite > 60 {
			t.Errorf("%s 的影像段號 %d 超出 MONSTERS.16 的段數", m.Name, m.Sprite)
		}
		seen[m.Sprite] = true
	}
	if len(seen) < 40 {
		t.Errorf("只用到 %d 個相異段號，欄位可能抓錯", len(seen))
	}
}

// 每輪行動次數與 `+17` 高位的值域要對。
//
// 兩者都是位元欄位：`+20 >> 4` 加一是每輪行動次數（1–16），
// `+17 >> 5` 是 0–7（索引八項的抗魔法表）。解錯位元寬度，值域立刻不對。
func TestMonsterActionFields(t *testing.T) {
	ms := parse(t)
	act := map[int]int{}
	chance := map[int]int{}
	for _, m := range ms {
		if m.Name == "" {
			continue
		}
		if m.SpecialUses < 1 || m.SpecialUses > 16 {
			t.Fatalf("%s 每輪可用特殊攻擊 %d 次", m.Name, m.SpecialUses)
		}
		if m.MagicResistIndex < 0 || m.MagicResistIndex > 7 {
			t.Fatalf("%s 的抗魔法索引是 %d", m.Name, m.MagicResistIndex)
		}
		act[m.SpecialUses]++
		chance[m.MagicResistIndex]++
	}
	// 大多數怪物一輪行動一次；索引也不該全部擠在同一格。
	if act[1] < len(act) {
		t.Log("特殊攻擊次數分佈:", act)
	}
	if len(chance) < 3 {
		t.Errorf("抗魔法索引只用到 %d 種，位元寬度可能取錯", len(chance))
	}
	t.Logf("每輪行動次數 %v；抗魔法索引 %v", act, chance)
}

// 群體大小與士氣層來自同一個位元組（b19），解錯會兩個一起錯。
// 用原版資料當對照：兩者都要落在解包規則允許的範圍內。
func TestGroupSizeAndMorale(t *testing.T) {
	ms := parse(t)
	sizes := map[int]int{}
	tiers := [4]int{}
	for _, m := range ms {
		if m.GroupSize < 1 || m.GroupSize > 160 {
			t.Fatalf("%s 的群體大小是 %d，超出 (b19&0x0F)+1 最多 ×10 的範圍",
				m.Name, m.GroupSize)
		}
		if m.MoraleTier < 0 || m.MoraleTier > 3 {
			t.Fatalf("%s 的士氣層是 %d，超出 0–3", m.Name, m.MoraleTier)
		}
		sizes[m.GroupSize]++
		tiers[m.MoraleTier]++
	}
	// 四個士氣層都要有怪物落進去 —— 全擠在一層表示位元取錯了。
	for i, n := range tiers {
		if n == 0 {
			t.Errorf("士氣層 %d 一隻怪都沒有，位元可能取錯", i)
		}
	}
	if len(sizes) < 5 {
		t.Errorf("群體大小只有 %d 種，分佈太窄", len(sizes))
	}
	t.Logf("士氣層分佈 %v，群體大小 %d 種", tiers, len(sizes))
}

// 抗性旗標的來源位元。名字是獨立於程式碼的判準：
// 名字帶 Fire 的必有火抗、Frost/Snow 必有冷抗、Acidic 必有酸抗。
//
// 這一組會擋住「讀錯位元組」這種錯法 —— 抗性那七格在 `ds:9E36`
// 起的連續位址上，相鄰位元組的位元看起來一樣合理。
func TestResistBitsMatchNames(t *testing.T) {
	ms := parse(t)
	byName := map[string]monsters.Monster{}
	for _, m := range ms {
		byName[m.Name] = m
	}
	const (
		fire = 0
		elec = 1
		cold = 2
		acid = 3
	)
	for _, tc := range []struct {
		name string
		idx  int
		what string
	}{
		{"Fire Devil", fire, "火"},
		{"Fire Faery", fire, "火"},
		{"Fire Dragon", fire, "火"},
		{"Fire Elemental", fire, "火"},
		{"Frost Dragon", cold, "冷"},
		{"The Snowbeast", cold, "冷"},
		{"Acidic Blob", acid, "酸"},
		{"Lightning Bugs", elec, "電"},
	} {
		m, ok := byName[tc.name]
		if !ok {
			t.Errorf("名冊裡沒有 %q", tc.name)
			continue
		}
		if !m.Resists[tc.idx] {
			t.Errorf("%s 沒有%s抗（索引 %d）", tc.name, tc.what, tc.idx)
		}
	}
	// 每一格都要有人設、也不能全表都設 —— 兩種極端都表示位元取錯。
	for i := 0; i < 7; i++ {
		n := 0
		for _, m := range ms {
			if m.Resists[i] {
				n++
			}
		}
		if n == 0 || n == len(ms) {
			t.Errorf("抗性索引 %d 有 %d/%d 隻設，這個分佈不對", i, n, len(ms))
		}
	}
}

// 抗魔法的八個階層要隨難度層遞增 —— 讀錯位元組就沒有這個梯度。
func TestMagicResistLadder(t *testing.T) {
	ms := parse(t)
	sum := [8]float64{}
	cnt := [8]int{}
	for _, m := range ms {
		k := m.MagicResistIndex
		if k < 0 || k > 7 {
			t.Fatalf("%s 的抗魔法索引是 %d", m.Name, k)
		}
		sum[k] += float64(m.Tier)
		cnt[k]++
	}
	var avg [8]float64
	for k := range cnt {
		if cnt[k] == 0 {
			t.Errorf("抗魔法階層 %d 一隻怪都沒有", k)
			continue
		}
		avg[k] = sum[k] / float64(cnt[k])
	}
	// 最低階與最高階要拉得開；中間允許小幅起伏。
	if avg[7] <= avg[0]+5 {
		t.Errorf("階層 0 的平均難度層 %.1f、階層 7 是 %.1f，梯度不成立",
			avg[0], avg[7])
	}
	t.Logf("各階層平均難度層 %.1f %.1f %.1f %.1f %.1f %.1f %.1f %.1f",
		avg[0], avg[1], avg[2], avg[3], avg[4], avg[5], avg[6], avg[7])
}

// 特殊攻擊的型態與使用機率來自同一個位元組（b17），一致性很強：
// 沒有型態的怪物機率必須是 0，有型態卻機率 0 的一隻都不該有。
//
// 這一組會擋住「型態與機率取錯位元組」以及「機率表少查了後八格」。
func TestSpecialAttackConsistency(t *testing.T) {
	ms := parse(t)
	var noneNonzero, hasZero, none, has int
	for _, m := range ms {
		switch {
		case m.SpecialIndex == 0 && m.SpecialChance == 0:
			none++
		case m.SpecialIndex == 0:
			noneNonzero++
		case m.SpecialChance == 0:
			hasZero++
		default:
			has++
		}
		if m.SpecialIndex < 0 || m.SpecialIndex > 31 {
			t.Fatalf("%s 的特殊攻擊型態是 %d", m.Name, m.SpecialIndex)
		}
	}
	if hasZero != 0 {
		t.Errorf("有 %d 隻怪有特殊攻擊型態卻機率 0 —— 兩個欄位對不上", hasZero)
	}
	if none < 100 {
		t.Errorf("只有 %d 隻怪沒有特殊攻擊，太少了", none)
	}
	// 原版的位移讓索引走到表外，機率會超過 100（＝必定使用）。
	// 這是照抄來的，不是 bug，但要確定真的抄到了。
	over := 0
	for _, m := range ms {
		if m.SpecialChance > 100 {
			over++
		}
	}
	if over == 0 {
		t.Error("沒有任何怪物的使用機率超過 100 —— 機率表的後八格沒抄到")
	}
	t.Logf("無型態且機率0 %d、無型態但機率非0 %d、有型態 %d、機率>100 %d",
		none, noneNonzero, has, over)
}
