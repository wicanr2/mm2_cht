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
