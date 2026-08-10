package game

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/assets/items"
)

// 商店的定價（`2SMITH.img` 的 `0xC80A`）。
//
// 商店在背包的**副本**上作業（`0xC150` 把 `+58`／`+64`／`+70` 三排
// 各 6 bytes 抄到工作區），所以沒寫回去就等於沒發生。
//
// 定價只看兩件事：物品記錄 `+18` 的基礎價，與屬性欄（`+70`）低六位的
// 附魔等級。等級每加一級**價格加倍**，另外再加 `1000 × 等級` ——
// 加倍在前、加常數在後，順序反了會差很多。

// ShopMode 是商店的模式，對應原版的 `ds:582A`。
type ShopMode int

const (
	// ShopBuy 是買進。持有商人技能的人買得比較便宜（減半）。
	ShopBuy ShopMode = iota
	// ShopSell 是賣出（`ds:582A == 5`）：一律先減半，
	// 沒有商人技能再減半一次。
	ShopSell
	// ShopIdentify 是鑑定（`ds:582A == 6`）：與基礎價無關，
	// 沒附魔收 10、有附魔收 `100 × 等級`。
	ShopIdentify
)

// SkillMerchant 是決定商店折扣的第二技能代碼。
//
// 原版用 `sub_1774A(ds:583E, 10)` 問「這個人有沒有第 10 項技能」。
// 名稱由手冊佐證：珍017 中文說明書 p.28–29 的第二技能表第 10 項正是
// Merchant（商人），「增加買賣時的商業手段」。
const SkillMerchant = 10

// HasSkill 回報這個人有沒有某項第二技能。
func (c *Character) HasSkill(skill int) bool {
	for _, k := range c.Skills {
		if k == skill {
			return true
		}
	}
	return false
}

// EnchantLevel 是背包某個槽位的附魔等級（屬性欄 `+70` 的低六位）。
func (c *Character) EnchantLevel(slot int) int {
	if slot < 0 || slot >= 6 {
		return 0
	}
	return int(c.FieldByte(offPackAttr+slot) & 0x3F)
}

// ShopPrice 算出背包某個槽位在某個模式下的價錢。空槽回 0。
func (c *Character) ShopPrice(table []items.Item, slot int, mode ShopMode) int {
	if slot < 0 || slot >= 6 {
		return 0
	}
	id := int(c.FieldByte(offPackID + slot))
	if id == 0 {
		return 0
	}
	lv := c.EnchantLevel(slot)

	if mode == ShopIdentify {
		if lv == 0 {
			return 10
		}
		return 100 * lv
	}

	if id >= len(table) {
		return 0
	}
	v := table[id].Price
	for i := 0; i < lv; i++ {
		v *= 2
	}
	if lv != 0 {
		v += 1000 * lv
	}

	skilled := c.HasSkill(SkillMerchant)
	if mode == ShopSell {
		v /= 2
		if !skilled {
			v /= 2
		}
	} else if skilled {
		v /= 2
	}
	return v
}

// BuyPrice 是**從貨架上買一件東西**要付多少。
//
// 與 ShopPrice 的差別在「東西在哪」：ShopPrice 的 slot 是**自己背包的
// 第幾格**（賣出與鑑定看的是你身上那件，附魔等級才有意義），而貨架上
// 的東西還不在任何人身上，只有物品編號。兩者共用同一條折扣規則：
// 持有商人技能買得便宜一半。
//
// 把貨架的物品編號餵給 ShopPrice 會拿到 0 或別件東西的價錢 ——
// 那個參數不是編號。
func BuyPrice(table []items.Item, id int, buyer *Character) int {
	if id <= 0 || id >= len(table) {
		return 0
	}
	v := table[id].Price
	if buyer != nil && buyer.HasSkill(SkillMerchant) {
		v /= 2
	}
	return v
}

// 鐵匠鋪（`2SMITH.OVL`）的三項服務：`Buy (A-F)`、`Sell (A-F)`、
// `Identify(A-F)`。三者都對**背包**那六格動作（畫面上的 A–F）。

// SellItem 把第 who 名隊員背包第 slot 格賣掉。
func (s *Session) SellItem(who, slot int) (string, bool) {
	if who < 0 || who >= len(s.Party) {
		return "沒有這個人。", false
	}
	c := &s.Party[who]
	i := EquippedSlots + slot
	if slot < 0 || slot >= BackpackSlots || c.Items[i].Empty() {
		return "那一格是空的。", false
	}
	name := s.itemName(c.Items[i].ID)
	price := c.ShopPrice(s.Items, slot, ShopSell)
	c.Items[i] = ItemSlot{}
	c.Gold += price
	return fmt.Sprintf("賣掉 %s，得到 %d 金幣。", name, price), true
}

// IdentifyItem 鑑定背包第 slot 格，回傳鐵匠說的那幾行。
//
// 原版印的是 `Spell Number N`、`Damage = 1-N`、`Armor bonus = N`
// （`ds:4520`／`ds:452E`／`ds:453A`）—— 也就是把物品記錄裡的欄位講出來。
func (s *Session) IdentifyItem(who, slot int) ([]string, bool) {
	if who < 0 || who >= len(s.Party) {
		return []string{"沒有這個人。"}, false
	}
	c := &s.Party[who]
	i := EquippedSlots + slot
	if slot < 0 || slot >= BackpackSlots || c.Items[i].Empty() {
		return []string{"那一格是空的。"}, false
	}
	id := c.Items[i].ID
	price := c.ShopPrice(s.Items, slot, ShopIdentify)
	if c.Gold < price {
		return []string{fmt.Sprintf("鑑定要 %d 金幣，錢不夠。", price)}, false
	}
	c.Gold -= price
	out := []string{fmt.Sprintf("%s（鑑定費 %d 金幣）", s.itemName(id), price)}
	if id >= 0 && id < len(s.Items) {
		it := s.Items[id]
		if n, ok := it.UseSpell(); ok {
			if idx, ok := ItemSpellToEngine(n); ok {
				if sp, ok := SpellByEngineIndex(idx); ok {
					out = append(out, "附帶法術："+sp.Name)
				}
			}
		}
		if it.Dice > 0 {
			out = append(out, fmt.Sprintf("傷害 = 1-%d", it.Dice))
		}
		if lv := c.EnchantLevel(slot); lv > 0 {
			out = append(out, fmt.Sprintf("附魔等級 %d", lv))
		}
	}
	if c.Items[i].Charge > 0 {
		out = append(out, fmt.Sprintf("可用次數 %d", c.Items[i].Charge))
	}
	return out, true
}

// itemName 查物品名。
func (s *Session) itemName(id int) string {
	if id < 0 || id >= len(s.Items) {
		return fmt.Sprintf("物品 %d", id)
	}
	return s.Items[id].Name
}
