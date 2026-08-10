// Package items 讀 ITEMS.DAT。
//
// 256 筆定長記錄，每筆 20 bytes：前 12 是名稱（空白補齊、直接 ASCII，
// 不像 MONSTERS.DAT 那樣整串加 0x80），其後 8 bytes 是屬性。
//
// 屬性的位置出自 `2CMDS.img` 的 `sub_CE12`（裝備換算成戰鬥數值）：
// 它以 `20 × 物品編號` 索引記憶體裡的物品表，取的正是 `+16` 那一欄。
// 分類的範圍出自同一支函式呼叫的四個判斷式（`sub_D00E` 等）。
package items

import (
	"errors"
	"strings"
)

const (
	// RecordSize 是一筆物品記錄的長度。
	RecordSize = 20
	// Count 是物品筆數。
	Count = 256
	// nameSize 是名稱欄的長度。
	nameSize = 12

	offClassMask = 13 // 可用職業的位元遮罩
	offSpecial   = 14 // 特殊能力，兩個位元組（`+15` 是使用效果，見 Use）
	offUse       = 15 // 使用效果
	offDice      = 16 // 傷害骰面數（護甲類是防護加成）
	offPrice     = 18 // 價格，uint16
)

// Category 是物品分類。範圍出自 `2CMDS.img` 的 sub_D00E／D026／D03E／D056。
type Category byte

const (
	CatNone   Category = iota
	CatMelee           // 1–91    近戰武器
	CatRanged          // 92–114  射擊武器
	CatShield          // 115–126 盾牌
	CatArmor           // 127–154 護甲
	CatHelm            // 155–159 頭盔
	CatOther           // 160–255 其他
)

var catNames = [...]string{"無", "近戰武器", "射擊武器", "盾牌", "護甲", "頭盔", "其他"}

func (c Category) String() string {
	if int(c) >= len(catNames) {
		return "未知"
	}
	return catNames[c]
}

// CategoryOf 依物品編號判分類。
func CategoryOf(id int) Category {
	switch {
	case id <= 0:
		return CatNone
	case id <= 91:
		return CatMelee
	case id <= 114:
		return CatRanged
	case id <= 126:
		return CatShield
	case id <= 154:
		return CatArmor
	case id <= 159:
		return CatHelm
	}
	return CatOther
}

// Item 是一件物品。
type Item struct {
	Index int
	Name  string
	Raw   [RecordSize]byte

	Category Category
	// Dice 是武器的傷害骰面數（擲 rand(1, Dice)）。護甲類放的是防護加成。
	Dice int
	// ClassMask 是可用職業的位元遮罩。
	ClassMask byte
	// Price 是價格。
	Price int
	// Special 是特殊能力的兩個位元組。第一個（`+14`）語意未解，
	// 只知道 `0xF0` 那個值會讓裝備被拒（`_2cmds_e03` 錯誤碼 14）；
	// 第二個（`+15`）是 Use。
	Special [2]byte

	// Use 是「使用」這件東西會發生什麼（記錄 `+15`）。
	//
	//	0        不能使用 —— 原版回 `No special power`
	//	>= 0x80  附帶法術，法術編號 = (Use & 0x7F) - 1
	//	其餘     另一種效果，走 `sub_1BBAE`（語意未解）
	//
	// 判讀點在 `2COMBAT.img` 的 `sub_1BA18`（先擋 0）與 `sub_1B92E`／
	// `sub_1B9A4`（再比 0x80 分兩條路）。
	Use byte
}

// Usable 回報這件東西能不能「使用」。
func (it Item) Usable() bool { return it.Use != 0 }

// UseSpell 回傳附帶的法術編號（1 起算的原版編號減一，即 0 起算）。
// ok 為 false 表示它不是法術型的效果。
func (it Item) UseSpell() (n int, ok bool) {
	if it.Use < 0x80 {
		return 0, false
	}
	return int(it.Use&0x7F) - 1, true
}

// Attrs 回傳名稱之後那 8 個還沒解的位元組。
func (it Item) Attrs() []byte { return it.Raw[nameSize:] }

// Parse 解開 ITEMS.DAT。檔案未壓縮。
func Parse(blob []byte) ([]Item, error) {
	if len(blob) != RecordSize*Count {
		return nil, errors.New("ITEMS.DAT 長度不對，應該是 5120 bytes")
	}
	out := make([]Item, Count)
	for i := range out {
		it := Item{Index: i}
		copy(it.Raw[:], blob[i*RecordSize:(i+1)*RecordSize])
		it.Name = decodeName(it.Raw[:nameSize])
		it.Category = CategoryOf(i)
		it.ClassMask = it.Raw[offClassMask]
		it.Special = [2]byte{it.Raw[offSpecial], it.Raw[offSpecial+1]}
		it.Use = it.Raw[offUse]
		it.Dice = int(it.Raw[offDice])
		it.Price = int(it.Raw[offPrice]) | int(it.Raw[offPrice+1])<<8
		out[i] = it
	}
	return out, nil
}

func decodeName(b []byte) string {
	if i := indexZero(b); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(string(b))
}

func indexZero(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i
		}
	}
	return -1
}
