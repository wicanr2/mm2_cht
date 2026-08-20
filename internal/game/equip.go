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
	case EquipHaveMelee:
		return "已經拿著武器了。"
	case EquipHaveMissile:
		return "已經拿著射擊武器了。"
	case EquipHaveShield:
		return "已經拿著盾了。"
	case EquipHaveArmor:
		return "已經穿著護甲了。"
	case EquipHaveHelm:
		return "已經戴著頭盔了。"
	case EquipTwoHanded:
		return "雙手武器不能跟盾一起用。"
	case EquipShieldBusy:
		return "拿著雙手武器，配不了盾。"
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
	return c.SlotConflict(it.ID)
}

const (
	offItemClassMask = 0x0D
	offItemSpecial   = 0x0E
)

// 物品編號的分類。六個範圍抄自 `2CMDS.img` 的六個判斷式
// （`sub_1CFDE`／`1CFF6`／`1D00E`／`1D026`／`1D03E`／`1D056`）。
//
// **近戰在這裡分成兩段**：1–65 單手、66–91 雙手。裝備換算那條路徑
// （`sub_CE12`）只當它們是「近戰」不分家，但部位衝突要分 ——
// 雙手武器與盾互斥就是靠這一刀。
const (
	melee1HLo, melee1HHi = 0x01, 0x41 // 1–65   單手近戰
	melee2HLo, melee2HHi = 0x42, 0x5B // 66–91  雙手近戰
	missileLo, missileHi = 0x5C, 0x72 // 92–114 射擊
	shieldLo, shieldHi   = 0x73, 0x7E // 115–126 盾
	armorLo, armorHi     = 0x7F, 0x9A // 127–154 護甲
	helmLo, helmHi       = 0x9B, 0x9F // 155–159 頭盔
)

func inRange(id, lo, hi int) bool { return id >= lo && id <= hi }

// 部位衝突的錯誤碼，數字是原版的。
const (
	EquipHaveMelee   EquipError = 6  // 已經拿著近戰武器
	EquipHaveMissile EquipError = 7  // 已經拿著射擊武器
	EquipHaveShield  EquipError = 8  // 已經拿著盾
	EquipHaveArmor   EquipError = 9  // 已經穿著護甲
	EquipHaveHelm    EquipError = 10 // 已經戴著頭盔
	EquipTwoHanded   EquipError = 12 // 雙手武器不能跟盾一起
	EquipShieldBusy  EquipError = 13 // 拿著雙手武器就配不了盾
)

// hasEquipped 回報已裝備的六格裡有沒有落在某個編號範圍的東西。
func (c *Character) hasEquipped(lo, hi int) bool {
	for i := 0; i < EquippedSlots; i++ {
		if s := c.Items[i]; !s.Empty() && inRange(s.ID, lo, hi) {
			return true
		}
	}
	return false
}

// SlotConflict 檢查部位衝突（`2CMDS.img` 的 `sub_1C8AA`）。
//
//	單手近戰 1–65    已有任何近戰武器 → 6
//	雙手近戰 66–91   已有任何近戰武器 → 6；已有盾 → 12
//	射擊 92–114      已有射擊武器 → 7
//	盾 115–126       已有盾 → 8；已有雙手近戰 → 13
//	護甲 127–154     已有護甲 → 9
//	頭盔 155–159     已有頭盔 → 10
//
// 「任何近戰武器」是 `sub_1D06E`，它把單手與雙手兩個判斷式加起來。
func (c *Character) SlotConflict(id int) EquipError {
	hasMelee := c.hasEquipped(melee1HLo, melee1HHi) || c.hasEquipped(melee2HLo, melee2HHi)
	switch {
	case inRange(id, melee1HLo, melee1HHi):
		if hasMelee {
			return EquipHaveMelee
		}
	case inRange(id, melee2HLo, melee2HHi):
		if hasMelee {
			return EquipHaveMelee
		}
		if c.hasEquipped(shieldLo, shieldHi) {
			return EquipTwoHanded
		}
	case inRange(id, missileLo, missileHi):
		if c.hasEquipped(missileLo, missileHi) {
			return EquipHaveMissile
		}
	case inRange(id, shieldLo, shieldHi):
		if c.hasEquipped(shieldLo, shieldHi) {
			return EquipHaveShield
		}
		if c.hasEquipped(melee2HLo, melee2HHi) {
			return EquipShieldBusy
		}
	case inRange(id, armorLo, armorHi):
		if c.hasEquipped(armorLo, armorHi) {
			return EquipHaveArmor
		}
	case inRange(id, helmLo, helmHi):
		if c.hasEquipped(helmLo, helmHi) {
			return EquipHaveHelm
		}
	}
	return EquipOK
}

// 裝備加成：物品的 `+0x0E` 高 nibble 選欄位、低 nibble 給量。
//
// 兩支入口對應原版 `2CMDS` 的 `sub_1CCD4`（裝上）與 `sub_1CC54`（卸下），
// 兩支共用 `sub_1CC14(記錄, n, &當前欄位指標)` 取欄位位址：
//
//	欄位   = 記錄 + 0x10 + n            ; 一定改
//	當前值 = 記錄 + 0x6B + n，n > 5 時沒有這一份
//	量     = 物品[+0x0E] & 0x0F + 該槽附魔等級（記錄 +52+槽 的低六位）
//	量 == 0 → 整支不做事
//
// `n` 的值域 0–15，對到記錄 `+16`–`+31`：六個屬性、八種抗性、盜行、
// 裝備防護值。**加減不對稱**：加走原版 root `0x13608`（飽和到 255）、
// 減走 root `0x135F0`（飽和到 0）。那個不對稱就是社群回報的「屬性氣球」
// （Bug 8）—— 當前值被打到 0 之後脫掉再穿上，卸下那一次什麼也沒扣、
// 穿上照樣加，屬性憑空長回加成量。**照抄，不修**：那是原版的行為，
// 而且要靠魔法滑梯陷阱才碰得到（見 `docs/research/07`）。
//
// 順序照原版：卸下**先扣加成再把東西搬走**，裝上**先把東西搬進裝備欄
// 再加加成** —— 兩支都是從裝備槽讀物品編號與附魔等級的。
func (c *Character) ApplyEquipBonus(table []items.Item, slot int) {
	c.adjustEquipBonus(table, slot, +1)
}

// RemoveEquipBonus 見 ApplyEquipBonus。
func (c *Character) RemoveEquipBonus(table []items.Item, slot int) {
	c.adjustEquipBonus(table, slot, -1)
}

func (c *Character) adjustEquipBonus(table []items.Item, slot, sign int) {
	if slot < 0 || slot >= EquippedSlots {
		return
	}
	s := c.Items[slot]
	if s.Empty() || s.ID >= len(table) {
		return
	}
	it := table[s.ID]
	amount := it.BonusAmount()
	if amount == 0 {
		return
	}
	amount += s.Bonus()
	switch n := it.BonusField() - offStats; {
	case n <= 5:
		// 六個屬性有兩份：基礎與當前。原版兩份都改。
		c.Base[n] = satAdjust(c.Base[n], amount, sign)
		c.Current[n] = satAdjust(c.Current[n], amount, sign)
	case n <= 13:
		c.Resist[n-6] = satAdjust(c.Resist[n-6], amount, sign)
	case n == 14:
		c.Thievery = satAdjust(c.Thievery, amount, sign)
	default:
		// 記錄 `+31` 只活在 Raw 裡（Encode 從 Raw 帶過去），
		// 所以這一格要自己寫回去。
		c.setGearAC(satAdjust(c.GearAC(), amount, sign))
	}
	// 原版每次加減完都呼叫 `loc_1768A`（root `sub_14F3A`）重算防護等級。
	c.RecomputeAC()
}

// satAdjust 是原版那一對飽和加減：加到 255 為止、減到 0 為止。
//
//	root 0x13608:  [欄位] > 0xFF - 量 → 0xFF，否則 += 量
//	root 0x135F0:  [欄位] < 量        → 0，   否則 -= 量
func satAdjust(v, amount, sign int) int {
	if sign > 0 {
		if v > 255-amount {
			return 255
		}
		return v + amount
	}
	if v < amount {
		return 0
	}
	return v - amount
}

// setGearAC 寫記錄 `+31`（裝備累加出來的防護值）。
func (c *Character) setGearAC(v int) {
	if len(c.Raw) <= offGearAC {
		return
	}
	switch {
	case v < 0:
		v = 0
	case v > 255:
		v = 255
	}
	c.Raw[offGearAC] = byte(v)
}
