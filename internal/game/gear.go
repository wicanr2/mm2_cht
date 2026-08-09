package game

import "github.com/wicanr2/mm2_cht/internal/assets/items"

// 裝備換算成戰鬥數值。
//
// 抄自 `2CMDS.img` 的 `sub_CE12`：逐一看已裝備的欄位，
// 依物品分類把「傷害骰面數」與「加成」寫進記錄的 `+76`…`+79`。
//
//	近戰武器（編號 1–91）   → +76 骰面、+77 命中加成
//	射擊武器（編號 92–114） → +78 骰面、+79 命中加成
//
// 骰面數取自物品表的 `+16`，加成取自角色記錄裡那一欄的屬性位元組低 6 位。
//
// 護甲、盾牌、頭盔那三類也會走同一支函式，但它們加的是防護等級，
// 累加的位置（`記錄+31` 交給 `sub_7726`）還沒追出來 —— 所以這裡不動 `AC`，
// 讀檔時記錄裡的值直接沿用。

// RecomputeGear 依已裝備的物品重算戰鬥數值。
//
// 沒有物品表時什麼都不做 —— 寧可沿用記錄裡原本的值，也不要把它清成 0。
func (c *Character) RecomputeGear(table []items.Item) {
	if len(table) == 0 {
		return
	}
	melee, meleeBonus := 0, 0
	ranged, rangedBonus := 0, 0
	for _, s := range c.Equipped() {
		if s.Empty() || s.ID >= len(table) {
			continue
		}
		it := table[s.ID]
		switch it.Category {
		case items.CatMelee:
			melee, meleeBonus = it.Dice, s.Bonus()
		case items.CatRanged:
			ranged, rangedBonus = it.Dice, s.Bonus()
		}
	}
	c.WeaponDice, c.HitBonus = melee, meleeBonus
	c.ShotDice, c.ShotBonus = ranged, rangedBonus
}

// EquippedWeapon 回傳已裝備的近戰武器，沒有就回 false。
func (c *Character) EquippedWeapon(table []items.Item) (items.Item, bool) {
	for _, s := range c.Equipped() {
		if s.Empty() || s.ID >= len(table) {
			continue
		}
		if it := table[s.ID]; it.Category == items.CatMelee {
			return it, true
		}
	}
	return items.Item{}, false
}
