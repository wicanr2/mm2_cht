package game

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/assets/items"
)

// 裝備的限制。規則逐條抄自 `2CMDS.img` 的裝備指令（`0x1CA01` 起，
// 印 `Equip Which (A-F)?` 的那一支）：
//
//	物品位址 = 20 × 記錄[+58+槽] + 0x6960
//	屬性     = 記錄[+70+槽]
//	if 屬性 == 0FFh:                         錯誤 3
//	if 物品[+0Dh] & ds:3408[職業]:           錯誤 4   ← 職業
//	if 屬性>>6 != 0 且 ds:3404[屬性>>6] != 記錄[+106]: 錯誤 5   ← 陣營
//	if 物品[+0Eh] == 0F0h:                   錯誤 14
//
// **`+0x0D` 是「禁用」不是「可用」** —— `test` 之後**非零才是錯誤**。
// 資料也只有這個讀法說得通：41 件近戰刀劍類**全部**設了牧師那個位元
// （零例外），而棍棒類一個位元都沒設。反過來讀的話牧師就只能用刀劍。

// classBit 是職業 → 位元（`ds:3408`）。**職業 0 是最高位**，
// 不是最低位 —— 反過來排的話限制會整批對到別的職業身上。
var classBit = [8]byte{0x80, 0x40, 0x20, 0x10, 0x08, 0x04, 0x02, 0x01}

// alignOfTier 是附魔層 → 陣營（`ds:3404`）。層 0 表示沒有陣營限制。
var alignOfTier = [4]byte{0, 2, 0, 1}

// EquipError 是裝不上去的原因，數字是原版的錯誤碼。
type EquipError int

const (
	EquipOK        EquipError = 0
	EquipEmpty     EquipError = 3  // 那一格沒有東西
	EquipClass     EquipError = 4  // 這個職業不能用
	EquipAlignment EquipError = 5  // 陣營不符
	EquipSpecial   EquipError = 14 // 特殊能力 0xF0 的東西裝不上
)

func (e EquipError) String() string {
	switch e {
	case EquipOK:
		return ""
	case EquipEmpty:
		return "那一格是空的。"
	case EquipClass:
		return "這個職業不能使用。"
	case EquipAlignment:
		return "陣營不符，裝不上去。"
	case EquipSpecial:
		return "這件東西裝不上去。"
	}
	return fmt.Sprintf("裝不上去（代碼 %d）", int(e))
}

// CanEquip 判斷某個人能不能把背包第 slot 格的東西裝起來。
//
// slot 是**背包的第幾格**（0–5），與原版的 `Equip Which (A-F)?` 一致。
func (c *Character) CanEquip(table []items.Item, slot int) EquipError {
	if slot < 0 || slot >= EquippedSlots {
		return EquipEmpty
	}
	it := c.Items[EquippedSlots+slot]
	if it.Empty() || it.ID >= len(table) {
		return EquipEmpty
	}
	if it.Attr == 0xFF {
		return EquipEmpty
	}
	rec := table[it.ID]
	if class := int(c.Class); class >= 0 && class < len(classBit) {
		if rec.Raw[offItemClassMask]&classBit[class] != 0 {
			return EquipClass
		}
	}
	// 陣營比的是**目前陣營**（記錄 +106），不是原始陣營（+13）——
	// 回復陣營術會把後者抄回前者，兩者可能不同。
	if tier := it.Attr >> 6; tier != 0 {
		if int(tier) < len(alignOfTier) && alignOfTier[tier] != c.FieldByte(offCurAlign) {
			return EquipAlignment
		}
	}
	if rec.Raw[offItemSpecial] == 0xF0 {
		return EquipSpecial
	}
	return EquipOK
}

const (
	offItemClassMask = 0x0D
	offItemSpecial   = 0x0E
)
