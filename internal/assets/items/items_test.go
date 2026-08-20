// 拿原版 ITEMS.DAT 當對照。原版資料不入版控，找不到就 skip。
package items_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/items"
)

func parse(t *testing.T) []items.Item {
	t.Helper()
	path := filepath.Join("..", "..", "..", "workplace", "orig", "MM2", "ITEMS.DAT")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("找不到 %s（玩家自備合法原版）", path)
	}
	list, err := items.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// 分類的邊界出自 2CMDS 的四個範圍判斷式。用名稱釘住兩端 ——
// 邊界抓錯時，武器會被當成護甲，傷害骰整組跑掉。
func TestCategoryBoundaries(t *testing.T) {
	list := parse(t)
	for _, w := range []struct {
		id   int
		name string
		cat  items.Category
	}{
		{1, "Small Club", items.CatMelee},
		{91, "Sun Naginata", items.CatMelee},
		{92, "Blowpipe", items.CatRanged},
		{115, "Small Shield", items.CatShield},
		{127, "Padded Armor", items.CatArmor},
		{155, "Helm", items.CatHelm},
		{160, "Magic Herbs", items.CatOther},
	} {
		it := list[w.id]
		if it.Name != w.name {
			t.Errorf("第 %d 件是 %q，預期 %q", w.id, it.Name, w.name)
		}
		if it.Category != w.cat {
			t.Errorf("%s 的分類是 %v，預期 %v", it.Name, it.Category, w.cat)
		}
	}
}

// 傷害骰面數要隨武器等級上升。這一條抓得到「取錯欄」——
// 取到價格那一欄的話，數字會大到離譜。
func TestWeaponDice(t *testing.T) {
	list := parse(t)
	for _, w := range []struct {
		id   int
		dice int
	}{
		{1, 2},   // Small Club
		{4, 4},   // Dagger
		{19, 8},  // Long Sword
		{24, 10}, // Katana
	} {
		if got := list[w.id].Dice; got != w.dice {
			t.Errorf("%s 的骰面是 %d，預期 %d", list[w.id].Name, got, w.dice)
		}
	}
	// 近戰武器的骰面不該超過三位數（那是價格欄的量級）。
	for _, it := range list {
		if it.Category == items.CatMelee && it.Dice > 100 {
			t.Errorf("%s 的骰面 %d 太大，可能取錯欄", it.Name, it.Dice)
		}
	}
}

// 價格要隨武器等級上升。
func TestPrice(t *testing.T) {
	list := parse(t)
	if list[1].Price >= list[24].Price {
		t.Errorf("Small Club %d 不該貴過 Katana %d", list[1].Price, list[24].Price)
	}
	if list[24].Price != 150 {
		t.Errorf("Katana 的價格是 %d，預期 150", list[24].Price)
	}
}

// `+14` 的兩個 nibble：高位選欄位、低位給量。
//
// 用名稱釘住 —— 這個欄位先前被判定成「程式從不讀」，而**判錯與判對在
// 資料上長得一樣**：兩種讀法都是位元組，都會回一個數字。真正的分辨方式
// 是問「這個數字配上這件東西的名字說不說得通」。`Sage Robe` 加智慧、
// `Fire Shield` 加抗火焰、`Skeleton Key` 加盜行 —— 三十七個相異值全部如此。
func TestEquipBonusFieldAndAmount(t *testing.T) {
	list := parse(t)
	// 記錄偏移：+16..+21 六個屬性、+22..+29 八種抗性、+30 盜行、+31 裝備防護。
	for _, w := range []struct {
		id     int
		name   string
		field  int
		amount int
	}{
		{206, "Sage Robe", 17, 6},     // 智慧
		{118, "Fire Shield", 23, 15},  // 抗火焰
		{121, "Cold Shield", 25, 15},  // 抗寒冰
		{120, "Acid Shield", 29, 15},  // 抗強酸
		{125, "Magic Shield", 22, 15}, // 抗魔法
		{156, "Iron Helm", 27, 15},    // 抗沈睡
		{157, "Bronze Helm", 28, 15},  // 抗毒素
		{158, "Silver Helm", 26, 15},  // 抗能量
		{159, "Gold Helm", 21, 15},    // 運氣
		{133, "Plate Mail", 16, 0},    // 純護甲：0x00 ＝ 力量 +0，防護走 Dice
		{0, "BLANK", 31, 0},           // 0xF0 ＝ 防護 +0，同時是「不能裝備」的哨兵
	} {
		it := list[w.id]
		if got := strings.TrimSpace(it.Name); got != w.name {
			t.Fatalf("編號 %d 是 %q，不是 %q —— 對照表跟資料對不上了", w.id, got, w.name)
		}
		if f, a := it.BonusField(), it.BonusAmount(); f != w.field || a != w.amount {
			t.Errorf("%s：加 +%d 到記錄 +%d，該是 +%d 到 +%d",
				w.name, a, f, w.amount, w.field)
		}
	}
}

// 盜行那一格（記錄 +30）只有賊系的道具會加 —— 反過來當交叉檢查：
// 加盜行的六件全部是鑰匙、撬鎖或潛行類。
func TestThieveryBonusItemsAreThiefGear(t *testing.T) {
	list := parse(t)
	var names []string
	for _, it := range list {
		if it.BonusAmount() > 0 && it.BonusField() == 30 {
			names = append(names, strings.TrimSpace(it.Name))
		}
	}
	sort.Strings(names)
	want := []string{"Castle Key", "Looter Knife", "Pirates xBow", "Skeleton Key", "Stealth Cape", "Thief's Pick"}
	if len(names) != len(want) {
		t.Fatalf("加盜行的有 %d 件（%v），該是 %d 件", len(names), names, len(want))
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("第 %d 件是 %q，該是 %q", i, n, want[i])
		}
	}
}
