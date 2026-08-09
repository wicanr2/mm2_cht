// 拿原版 ITEMS.DAT 當對照。原版資料不入版控，找不到就 skip。
package items_test

import (
	"os"
	"path/filepath"
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
