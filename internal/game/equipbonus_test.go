package game_test

import (
	"testing"

	"strings"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 裝備加成：物品 `+0x0E` 的高 nibble 選欄位、低 nibble 給量。
//
// 這一份用**名字**釘住每一條 —— 加成解錯欄位在數字上看不出來
// （抗火焰與抗寒冰都是 0–255 的百分比），只有「這件東西的名字說不說得通」
// 分得出來。`Fire Shield` 就該加抗火焰，不該加抗寒冰。

// hero 借 DEFAULT.DAT 的第一個角色，六個裝備欄先清空。
func hero(t *testing.T) *game.Character {
	t.Helper()
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	c := &cs[0]
	for i := 0; i < game.EquippedSlots; i++ {
		c.Items[i] = game.ItemSlot{}
	}
	return c
}

func TestEquipBonusGoesToTheRightField(t *testing.T) {
	tbl := itemTable(t)
	for _, w := range []struct {
		name   string
		id     int
		read   func(*game.Character) int
		amount int
	}{
		{"Fire Shield", 118, func(c *game.Character) int { return c.Resist[game.ResistFire] }, 15},
		{"Cold Shield", 121, func(c *game.Character) int { return c.Resist[game.ResistCold] }, 15},
		{"Magic Shield", 125, func(c *game.Character) int { return c.Resist[game.ResistMagic] }, 15},
		{"Skeleton Key", 188, func(c *game.Character) int { return c.Thievery }, 10},
		{"Sage Robe", 206, func(c *game.Character) int { return c.Base[game.Intellect] }, 6},
		{"Defense Ring", 189, func(c *game.Character) int { return c.GearAC() }, 2},
	} {
		c := hero(t)
		if got := strings.TrimSpace(tbl[w.id].Name); got != w.name {
			t.Fatalf("編號 %d 是 %q，不是 %q —— 對照表與資料對不上了", w.id, got, w.name)
		}
		before := w.read(c)
		c.Items[0] = game.ItemSlot{ID: w.id}
		c.ApplyEquipBonus(tbl, 0)
		if got := w.read(c); got != before+w.amount {
			t.Errorf("裝上 %s 之後那一格是 %d，該是 %d", w.name, got, before+w.amount)
		}
		c.RemoveEquipBonus(tbl, 0)
		if got := w.read(c); got != before {
			t.Errorf("卸下 %s 之後那一格是 %d，該回到 %d", w.name, got, before)
		}
	}
}

// 六個屬性有兩份 —— 基礎與當前，原版兩份都改。抗性、盜行與防護只有一份。
func TestStatBonusTouchesBothCopies(t *testing.T) {
	tbl := itemTable(t)
	c := hero(t)
	b, cur := c.Base[game.Intellect], c.Current[game.Intellect]
	c.Items[0] = game.ItemSlot{ID: 206} // Sage Robe：智慧 +6
	c.ApplyEquipBonus(tbl, 0)
	if c.Base[game.Intellect] != b+6 || c.Current[game.Intellect] != cur+6 {
		t.Errorf("基礎 %d→%d、當前 %d→%d，兩份都該 +6",
			b, c.Base[game.Intellect], cur, c.Current[game.Intellect])
	}
}

// 加成量要把該槽的附魔等級一起算進去（原版 `var_6 = 附魔等級 + 低 nibble`）。
func TestEnchantLevelAddsToBonus(t *testing.T) {
	tbl := itemTable(t)
	c := hero(t)
	b := c.Base[game.Intellect]
	c.Items[0] = game.ItemSlot{ID: 206, Attr: 3} // 附魔三級
	c.ApplyEquipBonus(tbl, 0)
	if got := c.Base[game.Intellect]; got != b+9 {
		t.Errorf("智慧 %d→%d，該是 +9（低 nibble 6 加附魔 3）", b, got)
	}
}

// **量是 0 就整支不做事** —— 普通護甲的 `+0x0E` 是 0x00（力量 +0），
// 它的防護值走 `+0x10`，不走這一條。附魔等級也不能讓它憑空生出加成。
func TestZeroBonusItemChangesNothing(t *testing.T) {
	tbl := itemTable(t)
	c := hero(t)
	before := c.Base[game.Might]
	c.Items[0] = game.ItemSlot{ID: 133, Attr: 5} // Plate Mail，附魔五級
	c.ApplyEquipBonus(tbl, 0)
	if got := c.Base[game.Might]; got != before {
		t.Errorf("普通護甲把力量從 %d 改成 %d", before, got)
	}
}

// **屬性氣球（社群回報的 Bug 5 之外的 Bug 8）照抄，不修。**
//
// 加飽和到 255、減飽和到 0，兩邊不對稱：當前值被打到 0 之後脫掉再穿上，
// 卸下那一次什麼也沒扣、穿上照樣加，屬性就憑空長回加成量。
// 這是原版的行為（root `0x13608` 與 `0x135F0`），驗收釘住它免得被「順手修掉」。
func TestStatBalloonIsKept(t *testing.T) {
	tbl := itemTable(t)
	c := hero(t)
	c.Items[0] = game.ItemSlot{ID: 206} // Sage Robe：智慧 +6
	c.ApplyEquipBonus(tbl, 0)
	// 魔法滑梯陷阱那種「當前值直接被砍掉」的狀態。
	c.Current[game.Intellect] = 0
	c.RemoveEquipBonus(tbl, 0)
	if got := c.Current[game.Intellect]; got != 0 {
		t.Fatalf("卸下之後當前智慧是 %d，飽和減該停在 0", got)
	}
	c.ApplyEquipBonus(tbl, 0)
	if got := c.Current[game.Intellect]; got != 6 {
		t.Errorf("再穿上之後當前智慧是 %d，原版會憑空長回 6", got)
	}
}

// 加成上限 255，不回捲。
func TestBonusSaturatesAt255(t *testing.T) {
	tbl := itemTable(t)
	c := hero(t)
	c.Resist[game.ResistFire] = 250
	c.Items[0] = game.ItemSlot{ID: 118} // Fire Shield：抗火焰 +15
	c.ApplyEquipBonus(tbl, 0)
	if got := c.Resist[game.ResistFire]; got != 255 {
		t.Errorf("抗火焰 250 加 15 得到 %d，該飽和在 255", got)
	}
}

// 防護值改的是記錄 `+31`，防護等級 `+36` 由它加速度修正重算出來。
func TestGearACBonusRecomputesArmorClass(t *testing.T) {
	tbl := itemTable(t)
	c := hero(t)
	// 先正規化一次 —— `DEFAULT.DAT` 存的 `+36` 是建角表算的，
	// 與 `+31` 加速度修正的公式差兩點（見 RecomputeAC 的註解）。
	c.RecomputeAC()
	before := c.AC
	c.Items[0] = game.ItemSlot{ID: 189} // Defense Ring：防護 +2
	c.ApplyEquipBonus(tbl, 0)
	if got := c.AC; got != before+2 {
		t.Errorf("防護等級 %d→%d，該 +2", before, got)
	}
	c.RemoveEquipBonus(tbl, 0)
	if got := c.AC; got != before {
		t.Errorf("卸下之後防護等級是 %d，該回到 %d", got, before)
	}
}
