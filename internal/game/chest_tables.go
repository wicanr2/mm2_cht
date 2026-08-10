package game

// 寶箱：踩到寶物格之後那一頁（`2MISC.OVL` 的 `_2misc_e02`）。
//
// 四個選項是 `ds:2A36` 那張四筆的指標表：開箱、找陷阱、偵測魔法、離開。
// 開箱與找陷阱共用同一段判定，只差在做完之後要不要接著開箱。

// chestName 是一條原版字串：key 給翻譯層查，fallback 是原文。
type chestName struct{ key, fallback string }

func (c chestName) String() string {
	if text == nil {
		return c.fallback
	}
	return text.Or(c.key, c.fallback)
}

// chestNames 是箱子的名字：**5 列（種類）× 8 欄（品質）**。
// 原版在 `ds:28A2`，列由 `sub_1CA00()` 決定、欄是算出來的品質。
var chestNames = [5][8]chestName{
	{
		{"exe.22BA", "Wooden Crate"},
		{"exe.22C9", "Tin Lockbox"},
		{"exe.22D7", "Steel Safe"},
		{"exe.22E4", "Copper Safe"},
		{"exe.22F2", "Bronze Safe"},
		{"exe.2300", "Steel Safe"},
		{"exe.230D", "Gold Safe"},
		{"exe.2319", "Stasis Safe"},
	},
	{
		{"exe.2327", "Hidden Cache"},
		{"exe.2336", "Wicker Chest"},
		{"exe.2345", "Rusty Trunk"},
		{"exe.2353", "Copper Box"},
		{"exe.2360", "Bronze Box"},
		{"exe.236D", "Steel Box"},
		{"exe.2379", "Gold Box"},
		{"exe.2384", "Doomsday Box"},
	},
	{
		{"exe.2393", "Rotting Box"},
		{"exe.23A1", "Rusty Chest"},
		{"exe.23AF", "Stone Chest"},
		{"exe.23BD", "Copper Chest"},
		{"exe.23CC", "Bronze Chest"},
		{"exe.23DB", "Steel Chest"},
		{"exe.23E9", "Gold Chest"},
		{"exe.23F6", "Statis Box"},
	},
	{
		{"exe.2403", "Wooden Chest"},
		{"exe.2412", "Rusty Chest"},
		{"exe.2420", "Copper Chest"},
		{"exe.242F", "Bronze Chest"},
		{"exe.243E", "Silver Chest"},
		{"exe.244D", "Gold Chest"},
		{"exe.245A", "Platinum Box"},
		{"exe.2469", "Doomsday Box"},
	},
	{
		{"exe.2478", "Ceramic Case"},
		{"exe.2487", "Lacquer Box"},
		{"exe.2495", "Jewelled Box"},
		{"exe.24A4", "Copper Trunk"},
		{"exe.24B3", "Bronze Trunk"},
		{"exe.24C2", "Silver Trunk"},
		{"exe.24D1", "Gold Trunk"},
		{"exe.24DE", "Statis Box"},
	},
}

var trapLines = [3][4][2]chestName{
	{
		{{"exe.24EB", "White-hot arcs of electricity explode"}, {"exe.2511", "from the trap!"}},
		{{"exe.2520", "Flames leap from the trap, scorching"}, {"exe.2545", "the party!"}},
		{{"exe.2550", "Noxious gas asphyxiates the party!"}, {"exe.2573", ""}},
		{{"exe.2574", "A barrage of spikes rips through the"}, {"exe.2599", "party!"}},
	},
	{
		{{"exe.25A0", "A ball of lightning engulfs the party!"}, {"exe.25C7", ""}},
		{{"exe.25C8", "A fireball engulfs the party!"}, {"exe.25E6", ""}},
		{{"exe.25E7", "Toxic gas issues from tiny vents"}, {"exe.2608", ""}},
		{{"exe.2609", "A barrage of molten metal explodes!"}, {"exe.262D", ""}},
	},
	{
		{{"exe.262E", "A rushing torrent of pure energy tears"}, {"exe.2655", "through the party!"}},
		{{"exe.2668", "A roaring inferno engulfs the party!"}, {"exe.268D", ""}},
		{{"exe.268E", "A cloud of noxious gas boils out of a"}, {"exe.26B4", "hidden grill!"}},
		{{"exe.26C2", "The trap sends razor-sharp metal"}, {"exe.26E3", "slivers into the party!"}},
	},
}
