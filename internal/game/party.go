package game

import (
	"bytes"
	"fmt"
)

// 角色記錄 130 bytes/筆。`DEFAULT.DAT` = 780 = 6 × 130，六個預設角色；
// `ROSTER.DAT` 是玩家存檔的名冊。見 docs/formats/02-data-files.md §5。
const RecordSize = 130

// 記錄裡已定位的欄位。
const (
	offName  = 0x00 // 10 bytes，空格填充，第 11 個位元組是 0
	offSex   = 0x0C // 性別 0=男 1=女
	offAlign = 0x0D // 陣營 0=善良 1=中立 2=邪惡
	offRace  = 0x0E // 種族 0–4
	offClass = 0x0F // 職業 0–7
	offStats = 0x10 // 六個屬性，一個 byte 一個
	offLevel = 32   // 經驗等級
	offAge   = 33   // 年齡
	offFood  = 37   // 食物
	offCond  = 38   // 狀況，位元遮罩
	offSP    = 88   // uint16 目前 SP（法力點數）
	offMaxSP = 90   // uint16 SP 上限
	offHP    = 94   // uint16 目前 HP
	offMaxHP = 96   // uint16 HP 上限
	offCur   = 107  // 屬性的第二份：受增減益影響後的當前值
)

// Sex 是性別。
type Sex byte

const (
	Male Sex = iota
	Female
)

func (s Sex) String() string {
	if s == Female {
		return "女"
	}
	return "男"
}

// Alignment 是陣營。
//
// 定錨依據：六個預設角色裡只有遊俠 Terwin III 與牧師 Gene Eric 是 0，
// 而手冊說遊俠必須是善良陣營 —— 所以 0 是善良。
type Alignment byte

const (
	Good Alignment = iota
	Neutral
	Evil
)


// String 的名稱來自 data/labels.json（原文讀自 MM2.EXE），譯文由
// game.UseText 提供。原文與譯文都不進 Go 原始碼。
func (a Alignment) String() string { return AlignName(int(a)) }

// Race 是種族。順序照手冊第 23–24 頁的選單（按 1–5 選）。
//
// 定錨依據是手冊的屬性修正：半獸人「+1 力量耐力、−1 智慧人格運氣」，
// 而 +0x0E 為 4 的 Sir Felgar 正是力量 21、智慧 10、人格 9；
// 精靈「+1 智慧準確度」，值為 1 的 Cassandra 智慧 21、The Hermit
// 準確度 21。六個角色的修正方向都對得上。
type Race byte

const (
	Human Race = iota
	Elf
	Dwarf
	Gnome
	HalfOrc
)


func (r Race) String() string { return RaceName(int(r)) }

// Class 是職業。
type Class byte

const (
	Knight Class = iota
	Paladin
	Archer
	Cleric
	Sorcerer
	Robber
	Ninja
	Barbarian
)


func (c Class) String() string { return ClassName(int(c)) }

// Stat 是屬性。順序先由六個預設角色反推 —— 每個角色的峰值都落在自己職業
// 該高的那一項（武士／遊俠→力量、弓箭手→速度、牧師→人格、巫師→智慧、
// 賊→準確度），六個全中 —— 之後由手冊的屬性表確認，順序一字不差。
//
// 手冊列的是**七項**，第七項是運氣（Luck）。記錄裡的屬性區只有六項，
// 第二份（+107）也只有六項，所以運氣存在別處，位置未定。
// 候選是 +0x21（六個預設角色都是 18），未經驗證。
type Stat int

const (
	Might Stat = iota
	Intellect
	Personality
	Endurance
	Speed
	Accuracy
	NumStats
)

// 屬性名用手冊的官方譯名。手冊本身對 Accuracy 有三種寫法
// （準確度／精確度／準確性），取內文的「準確度」。
var statNames = [NumStats]string{"力量", "智慧", "人格", "耐力", "速度", "準確度"}

func (s Stat) String() string {
	if s < 0 || s >= NumStats {
		return fmt.Sprintf("屬性%d", int(s))
	}
	return statNames[s]
}

// Character 是一個角色。
type Character struct {
	Name  string
	Sex   Sex
	Align Alignment
	Race  Race
	Class Class
	Age   int
	Level int
	Food  int
	HP    int
	MaxHP int
	SP    int
	MaxSP int

	// Base 是基礎屬性，Current 是受增減益影響後的值。
	// 兩者在記錄裡各存一份（+0x10 與 +107）。
	Base    [NumStats]int
	Current [NumStats]int

	// Condition 是身體狀況，供戰鬥層使用。
	Condition Condition

	// Exp 是經驗點數。
	//
	// **記錄裡的位置還沒定出來**，所以這個欄位只活在記憶體裡，
	// `Encode` 不會寫回去 —— 存檔之後經驗值會歸零。掃過名冊的
	// 每一個 uint16／uint32 欄位，與等級高度相關的那些都是跨欄位的
	// 假相關（含到 +32 的等級位元組），沒有一個的值域像經驗值。
	Exp int

	// CondBits 是記錄裡 +38 的原始位元遮罩。
	//
	// 位置由 `sub_1AFBC` 確認：那支程序把 HP（`[bx+5Eh]`）歸零，
	// 然後在 `[bx+26h]`（就是 +38）設 bit 6 —— 除非已經有 bit 7。
	// 所以 **bit 6 是 HP 歸零時設的（無意識）、bit 7 是更嚴重的狀況**，
	// 其餘六個位元對應手冊列的中毒、沈睡、痲痺、石化、根除，
	// 但哪個位元是哪一項還沒定。
	//
	// 資料佐證：六個預設角色全是 0（正常），名冊四十筆裡三十七筆是 0、
	// 三筆是 0x81（bit 0 + bit 7）。
	CondBits byte

	Raw []byte // 未解的欄位原樣保留，寫回時不能丟
}

// ParseCharacters 解析一份角色檔（DEFAULT.DAT 或 ROSTER.DAT）。
//
// ROSTER.DAT 是 8,293 bytes，不是 130 的整數倍（130×63 + 103），
// 尾端那 103 bytes 不成一筆，略過而不是硬湊。
func ParseCharacters(blob []byte) ([]Character, error) {
	// 名稱、職業表這些都在資料層，解人物之前要先有。
	if err := EnsureData(); err != nil {
		return nil, err
	}
	n := len(blob) / RecordSize
	if n == 0 {
		return nil, fmt.Errorf("檔案只有 %d bytes，放不下一筆 %d bytes 的記錄", len(blob), RecordSize)
	}
	out := make([]Character, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, parseCharacter(blob[i*RecordSize:(i+1)*RecordSize]))
	}
	return out, nil
}

func parseCharacter(r []byte) Character {
	name := r[offName : offName+10]
	if k := bytes.IndexByte(name, 0); k >= 0 {
		name = name[:k]
	}
	c := Character{
		Name:  string(bytes.TrimRight(name, " ")),
		Sex:   Sex(r[offSex]),
		Align: Alignment(r[offAlign]),
		Race:  Race(r[offRace]),
		Class: Class(r[offClass]),
		Age:   int(r[offAge]),
		Level: int(r[offLevel]),
		Food:  int(r[offFood]),
		CondBits: r[offCond],
		HP:    int(r[offHP]) | int(r[offHP+1])<<8,
		MaxHP: int(r[offMaxHP]) | int(r[offMaxHP+1])<<8,
		SP:    int(r[offSP]) | int(r[offSP+1])<<8,
		MaxSP: int(r[offMaxSP]) | int(r[offMaxSP+1])<<8,
		Raw:   append([]byte(nil), r...),
	}
	for i := Stat(0); i < NumStats; i++ {
		c.Base[i] = int(r[offStats+int(i)])
		c.Current[i] = int(r[offCur+int(i)])
	}
	switch {
	case c.CondBits&CondBitSevere != 0:
		c.Condition = CondDead
	case c.CondBits&CondBitUnconscious != 0:
		c.Condition = CondUnconscious
	}
	return c
}

// 狀況位元裡已經確認語意的兩個。
const (
	CondBitUnconscious = 0x40 // sub_1AFBC 在 HP 歸零時設
	CondBitSevere      = 0x80 // 更嚴重的狀況；設 bit 6 之前會先檢查它
)

// Caster 回報這個職業一開始就有法力。
//
// 手冊：牧師與巫師從第一級就能施法，遊俠與弓箭手要到高經驗等級才有 ——
// 六個預設角色都是第一級，所以只有牧師與巫師的 SP 非零。
func (c Class) Caster() bool { return c == Cleric || c == Sorcerer }

// Empty 回報這一格名冊是不是有效的角色。
//
// ROSTER.DAT 裡有刪除後殘留的槽位，名字欄不見得歸零 —— 有的留著半截
// 舊資料。判準取「名字要是乾淨的可見 ASCII」，比只看空字串可靠。
func (c Character) Empty() bool {
	if c.Name == "" {
		return true
	}
	for _, ch := range c.Name {
		if ch < 32 || ch > 126 {
			return true
		}
	}
	return false
}

// Party 是目前的隊伍。原版最多六人。
type Party struct {
	Members []Character
}

// MaxParty 是隊伍人數上限。
const MaxParty = 6
