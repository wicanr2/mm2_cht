package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/items"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// 定價的三條規則各驗一次，順序也要對：加倍在前、加常數在後。
func TestShopPrice(t *testing.T) {
	table, err := items.Parse(orig(t, "ITEMS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	c := &cs[0]
	// 挑一件有基礎價的物品放進第 0 槽。
	id := 0
	for i := 1; i < len(table); i++ {
		if table[i].Price > 0 {
			id = i
			break
		}
	}
	if id == 0 {
		t.Fatal("物品表裡找不到有價格的")
	}
	base := table[id].Price
	c.SetFieldByte(58, 0x00, byte(id))
	c.Skills = [2]int{0, 0} // 沒有商人技能

	c.SetFieldByte(70, 0x00, 0) // 沒附魔
	if got := c.ShopPrice(table, 0, game.ShopBuy); got != base {
		t.Errorf("沒附魔的買價 %d，基礎價是 %d", got, base)
	}
	if got := c.ShopPrice(table, 0, game.ShopSell); got != base/4 {
		t.Errorf("沒技能的賣價 %d，該是基礎價的四分之一 %d", got, base/4)
	}
	if got := c.ShopPrice(table, 0, game.ShopIdentify); got != 10 {
		t.Errorf("沒附魔的鑑定費 %d，該是 10", got)
	}

	c.SetFieldByte(70, 0x00, 3) // 附魔三級
	want := base*8 + 3000       // 加倍三次，再加 1000 × 3
	if got := c.ShopPrice(table, 0, game.ShopBuy); got != want {
		t.Errorf("三級附魔的買價 %d，該是 %d（加倍在前、加常數在後）", got, want)
	}
	if got := c.ShopPrice(table, 0, game.ShopIdentify); got != 300 {
		t.Errorf("三級附魔的鑑定費 %d，該是 300", got)
	}

	// 有商人技能：買價減半，賣價只砍一次。
	c.Skills = [2]int{game.SkillMerchant, 0}
	if got := c.ShopPrice(table, 0, game.ShopBuy); got != want/2 {
		t.Errorf("有技能的買價 %d，該是 %d", got, want/2)
	}
	if got := c.ShopPrice(table, 0, game.ShopSell); got != want/2 {
		t.Errorf("有技能的賣價 %d，該是 %d", got, want/2)
	}

	// 空槽一律 0。
	c.SetFieldByte(58, 0x00, 0)
	if got := c.ShopPrice(table, 0, game.ShopBuy); got != 0 {
		t.Errorf("空槽的價錢是 %d", got)
	}
}

// 貨架表：四類商店 × 五座城 × 六件，編號都要是有名字的物品。
func TestShopStock(t *testing.T) {
	table, err := items.Parse(orig(t, "ITEMS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	total, named := 0, 0
	for g := 0; g < 4; g++ {
		for town := 0; town < 5; town++ {
			ids, extra := game.ShopGoods(g, town)
			if len(ids) != 6 || len(extra) != 6 {
				t.Fatalf("第 %d 類第 %d 城取到 %d/%d 件", g, town, len(ids), len(extra))
			}
			for k, id := range ids {
				total++
				if id <= 0 || id >= len(table) {
					t.Errorf("第 %d 類第 %d 城第 %d 件的編號 %d 超出物品表", g, town, k, id)
					continue
				}
				if table[id].Name != "" {
					named++
				}
				if table[id].Price <= 0 {
					t.Errorf("%s（編號 %d）在貨架上卻沒有價格", table[id].Name, id)
				}
			}
		}
	}
	if total != 120 {
		t.Fatalf("貨架共 %d 件，該是 120", total)
	}
	if named < total {
		t.Errorf("%d/%d 件貨有名字，該是全部", named, total)
	}
	t.Logf("貨架 %d 件全部對得上物品表", total)
}

// 裝備限制：+0x0D 是「禁用」職業遮罩，不是「可用」。
//
// 判準用資料本身：41 件近戰刀劍類**全部**禁牧師（零例外），
// 棍棒類一個位元都沒設。反過來讀的話牧師就只能用刀劍。
func TestEquipClassRestriction(t *testing.T) {
	tbl, err := items.Parse(orig(t, "ITEMS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	c := &game.Character{}
	c.Class = 3 // 牧師
	// Dagger（編號 4）禁牧師
	c.Items[game.EquippedSlots] = game.ItemSlot{ID: 4}
	if got := c.CanEquip(tbl, 0); got != game.EquipClass {
		t.Errorf("牧師裝匕首得到 %v，預期 EquipClass", got)
	}
	// Small Club（編號 1）誰都能用
	c.Items[game.EquippedSlots] = game.ItemSlot{ID: 1}
	if got := c.CanEquip(tbl, 0); got != game.EquipOK {
		t.Errorf("牧師裝短棍得到 %v，預期可以", got)
	}
	// 換成武士就裝得了匕首
	c.Class = 0
	c.Items[game.EquippedSlots] = game.ItemSlot{ID: 4}
	if got := c.CanEquip(tbl, 0); got != game.EquipOK {
		t.Errorf("武士裝匕首得到 %v，預期可以", got)
	}
	// 空格
	c.Items[game.EquippedSlots] = game.ItemSlot{}
	if got := c.CanEquip(tbl, 0); got != game.EquipEmpty {
		t.Errorf("空格得到 %v，預期 EquipEmpty", got)
	}
}

// 部位衝突：雙手武器與盾互斥，同部位不能裝兩件。
func TestEquipSlotConflict(t *testing.T) {
	c := &game.Character{}
	// 空手時什麼都裝得上
	for _, id := range []int{1, 70, 100, 120, 130, 156} {
		if got := c.SlotConflict(id); got != game.EquipOK {
			t.Errorf("空手裝編號 %d 得到 %v", id, got)
		}
	}
	// 拿了單手武器就不能再拿武器
	c.Items[0] = game.ItemSlot{ID: 4} // Dagger，單手
	if got := c.SlotConflict(10); got != game.EquipHaveMelee {
		t.Errorf("已有武器再裝武器得到 %v，預期 EquipHaveMelee", got)
	}
	if got := c.SlotConflict(120); got != game.EquipOK {
		t.Errorf("已有武器裝盾得到 %v，預期可以", got)
	}
	// 拿著盾就不能裝雙手武器，反之亦然
	c.Items[1] = game.ItemSlot{ID: 120} // 盾
	if got := c.SlotConflict(70); got != game.EquipHaveMelee {
		t.Errorf("已有武器時裝雙手得到 %v，預期先擋在武器那關", got)
	}
	c.Items[0] = game.ItemSlot{} // 放下武器，只剩盾
	if got := c.SlotConflict(70); got != game.EquipTwoHanded {
		t.Errorf("有盾裝雙手武器得到 %v，預期 EquipTwoHanded", got)
	}
	// 拿著雙手武器就配不了盾
	c.Items[0] = game.ItemSlot{ID: 70}
	c.Items[1] = game.ItemSlot{}
	if got := c.SlotConflict(120); got != game.EquipShieldBusy {
		t.Errorf("有雙手武器裝盾得到 %v，預期 EquipShieldBusy", got)
	}
	// 護甲與頭盔各自只能一件
	c2 := &game.Character{}
	c2.Items[0] = game.ItemSlot{ID: 130}
	if got := c2.SlotConflict(140); got != game.EquipHaveArmor {
		t.Errorf("兩件護甲得到 %v", got)
	}
	c2.Items[1] = game.ItemSlot{ID: 156}
	if got := c2.SlotConflict(157); got != game.EquipHaveHelm {
		t.Errorf("兩頂頭盔得到 %v", got)
	}
}
