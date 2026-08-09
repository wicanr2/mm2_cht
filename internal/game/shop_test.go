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
