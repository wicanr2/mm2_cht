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
		idx           int
		name          string
		hp, exp       int
		atks, dice    int
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
		if m.Actions < 1 || m.Actions > 16 {
			t.Fatalf("%s 每輪行動 %d 次", m.Name, m.Actions)
		}
		if m.MagicResistIndex < 0 || m.MagicResistIndex > 7 {
			t.Fatalf("%s 的抗魔法索引是 %d", m.Name, m.MagicResistIndex)
		}
		act[m.Actions]++
		chance[m.MagicResistIndex]++
	}
	// 大多數怪物一輪行動一次；索引也不該全部擠在同一格。
	if act[1] < len(act) {
		t.Log("每輪行動次數分佈:", act)
	}
	if len(chance) < 3 {
		t.Errorf("抗魔法索引只用到 %d 種，位元寬度可能取錯", len(chance))
	}
	t.Logf("每輪行動次數 %v；抗魔法索引 %v", act, chance)
}
