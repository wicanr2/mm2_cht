package game

import "fmt"

// 法師公會（`2TEMPLE.OVL` 的第二個進入點，`0xCB9C`）。
//
// 每座城賣四條法術，清單在 `ds:46DA`（5 列 × 4）、價格在 `ds:46EE`。
// 買的條件（`sub_1C3A0`）：
//
//   - 職業必須是巫師（4）或弓箭手（2）—— 這是**巫師公會**
//   - 記錄 `+35`（法力等級）要 ≥ 法術的等級
//   - 還沒學過（`loc_177B5` 查已學位元）
//   - 錢要夠
//
// 法術編號是**巫師系內的 0 起算序號**。這一點由三件事互相印證：
// 只有巫師與弓箭手能買、四條在巫師系裡查得到而且都對得上城鎮的難度，
// 以及價格隨著城鎮階梯單調上升（米德格特 10–1000、桑德索巴 200–1000、
// 桑達拉 600–3000、佛卡尼亞 5000–25000、亞特蘭汀 50000–100000）。

// guildSpells 是每座城賣的四條法術（`ds:46DA`）。
var guildSpells = [5][4]int{
	{0, 2, 6, 9},     // 米德格特
	{41, 42, 44, 45}, // 亞特蘭汀
	{21, 22, 26, 28}, // 桑達拉
	{31, 33, 35, 37}, // 佛卡尼亞
	{13, 14, 17, 20}, // 桑德索巴
}

// guildPriceCode 是價格的壓縮碼（`ds:46EE`）。
var guildPriceCode = [5][4]byte{
	{10, 129, 37, 65},
	{165, 165, 170, 170},
	{70, 130, 131, 131},
	{133, 133, 133, 153},
	{68, 66, 129, 69},
}

// DecodeGuildPrice 把壓縮碼還原成金額。
//
// `sub_1C3A0`：低 5 位是基數，bit5 乘 10、bit6 乘 100、bit7 乘 1000，
// 三個位元可以疊加。所以 `0x81` 是 1×1000、`0x25` 是 5×10。
func DecodeGuildPrice(code byte) int {
	n := int(code & 0x1F)
	if code&0x20 != 0 {
		n *= 10
	}
	if code&0x40 != 0 {
		n *= 100
	}
	if code&0x80 != 0 {
		n *= 1000
	}
	return n
}

// GuildStock 是某座城的公會賣的東西。
type GuildStock struct {
	// Spell 是巫師系內的 0 起算序號，Price 是金額。
	Spell int
	Price int
}

// GuildStockOf 回傳第 town 座城的公會貨色。
func GuildStockOf(town int) []GuildStock {
	if town < 0 || town >= len(guildSpells) {
		return nil
	}
	out := make([]GuildStock, 0, 4)
	for i, n := range guildSpells[town] {
		out = append(out, GuildStock{Spell: n, Price: DecodeGuildPrice(guildPriceCode[town][i])})
	}
	return out
}

// GuildBuy 讓第 who 名隊員買下第 i 項。
func (s *Session) GuildBuy(town, who, i int) (string, bool) {
	stock := GuildStockOf(town)
	if i < 0 || i >= len(stock) || who < 0 || who >= len(s.Party) {
		return "沒有這一項。", false
	}
	c := &s.Party[who]
	if c.Class != Sorcerer && c.Class != Archer {
		// 原版印 `Sorry, you must be a member of this guild to purchase spells.`
		return guildNotMember(), false
	}
	it := stock[i]
	name := fmt.Sprintf("第 %d 條法術", it.Spell+1)
	level := it.Spell/8 + 1
	if sp, ok := SpellByEngineIndex(48 + it.Spell); ok {
		name, level = sp.Name, sp.Level
	}
	if c.SpellLevel() < level {
		return fmt.Sprintf("%s 的法力等級不夠，%s 是第 %d 級。", c.Name, name, level), false
	}
	n := it.Spell + 1 // Knows 是 1 起算
	if c.Knows(n) {
		return c.Name + " 已經會" + name + "了。", false
	}
	if c.Gold < it.Price {
		return fmt.Sprintf("%s 要 %d 金幣，錢不夠。", name, it.Price), false
	}
	c.Gold -= it.Price
	c.Learn(n)
	return fmt.Sprintf("%s 學會了%s，花了 %d 金幣。", c.Name, name, it.Price), true
}

// guildNotMember 是「不是本公會成員」那句（`STR.DAT` 神殿／公會段）。
func guildNotMember() string {
	if text == nil {
		return "Sorry, you must be a member of this guild to purchase spells."
	}
	return text.Or("guild.notmember", "抱歉，你必須是本公會的成員才能購買法術。")
}

// 神殿也賣法術（`2TEMPLE.OVL` 的第一個進入點，選單上的 D／E／F）。
//
// 結構與法師公會完全平行，只是換一系、換一組表：
//
//	          法術表     價格碼     買家職業
//	神殿      ds:46B2    ds:46C6    牧師（3）或聖騎士（1）
//	法師公會  ds:46DA    ds:46EE    巫師（4）或弓箭手（2）
//
// 四張表在 DGROUP 裡是連著的（`46B2` → `46C6` → `46DA` → `46EE`，各 20 bytes）。
// 判準是 `sub_1C4A2`（神殿）與 `sub_1C3A0`（公會）兩支逐行對稱，
// 差別只在讀哪一組表、以及職業那兩個 `cmp [bx+0Fh]`。
//
// **神殿只賣三條**（選單是 D／E／F），第 4 欄固定是 `0x80` 的填充。
// 法術編號在表裡**加了 `0x30`**（`sub al, 30h` 才還原）——
// 那是為了讓整張表在十六進位傾印裡看起來像可列印字元。
var templeSpells = [5][3]int{
	{0, 1, 5},    // 米德格特
	{42, 46, 47}, // 亞特蘭汀
	{14, 18, 23}, // 桑達拉
	{25, 30, 37}, // 佛卡尼亞
	{8, 11, 13},  // 桑德索巴
}

// templePriceCode 是神殿的價格碼（`ds:46C6`），編碼與公會同一套。
var templePriceCode = [5][3]byte{
	{10, 10, 129},
	{148, 165, 170},
	{68, 65, 69},
	{130, 131, 138},
	{57, 67, 66},
}

// TempleStockOf 回傳第 town 座城的神殿賣的三條牧師系法術。
func TempleStockOf(town int) []GuildStock {
	if town < 0 || town >= len(templeSpells) {
		return nil
	}
	out := make([]GuildStock, 0, 3)
	for i, n := range templeSpells[town] {
		out = append(out, GuildStock{Spell: n, Price: DecodeGuildPrice(templePriceCode[town][i])})
	}
	return out
}

// TempleBuy 讓第 who 名隊員在神殿買下第 i 條法術。
func (s *Session) TempleBuy(town, who, i int) (string, bool) {
	stock := TempleStockOf(town)
	if i < 0 || i >= len(stock) || who < 0 || who >= len(s.Party) {
		return "沒有這一項。", false
	}
	c := &s.Party[who]
	if c.Class != Cleric && c.Class != Paladin {
		return guildNotMember(), false
	}
	it := stock[i]
	// 神殿賣的是牧師系 —— 引擎內部的編號 0–47 就是牧師系
	// （原版是巫師 0–47／牧師 48–95，兩邊相反，見 ItemSpellToEngine）。
	name := fmt.Sprintf("第 %d 條法術", it.Spell+1)
	level := it.Spell/8 + 1
	if sp, ok := SpellByEngineIndex(it.Spell); ok {
		name, level = sp.Name, sp.Level
	}
	if c.SpellLevel() < level {
		return fmt.Sprintf("%s 的法力等級不夠，%s 是第 %d 級。", c.Name, name, level), false
	}
	n := it.Spell + 1
	if c.Knows(n) {
		return c.Name + " 已經會" + name + "了。", false
	}
	if c.Gold < it.Price {
		return fmt.Sprintf("%s 要 %d 金幣，錢不夠。", name, it.Price), false
	}
	c.Gold -= it.Price
	c.Learn(n)
	return fmt.Sprintf("%s 學會了%s，花了 %d 金幣。", c.Name, name, it.Price), true
}
