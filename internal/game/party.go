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
	offGearAC = 31 // 裝備給的防護值，護甲累加在這裡
	offAC     = 36 // 防護等級 = offGearAC + 耐力修正。sub_8398 拿它算命中率
	offCond  = 38   // 狀況，位元遮罩
	offSP    = 88   // uint16 目前 SP（法力點數）
	offMaxSP = 90   // uint16 SP 上限
	offHP    = 94   // uint16 目前 HP
	offMaxHP = 96   // uint16 HP 上限
	offCurAlign = 106 // 目前陣營（`+13` 是原始陣營，回復陣營術把後者抄回這裡）
	offCur      = 107 // 屬性的第二份：受增減益影響後的當前值
	offGems  = 92   // uint16 寶石
	offExp   = 98   // uint32 經驗值
	offGold  = 102  // uint32 黃金
	offResist = 22  // 八種抗性：魔法／火焰／電擊／寒冰／能量／沈睡／毒素／強酸
	offThief  = 30  // 盜行，只有賊與忍者非零
	offEndB   = 39  // 耐力（基礎）
	offSL     = 114 // 法力等級
	offEnd    = 115 // 耐力（當前），與 +39 逐筆相同
	// 物品區是**六組平行陣列**，每組 6 格：已裝備一套、背包一套。
	//
	//	已裝備  id +40  欄B +46  屬性 +52
	//	背包    id +58  欄B +64  屬性 +70
	//
	// `2CMDS.img` 的 `sub_CE12` 以 `記錄 + 基底` 取 `+0x28`（id）與
	// `+0x34`（屬性），基底是 0（已裝備）或 18（背包）；
	// `2PLAY.img` 的 `0x9B44`（給予物品）則直接寫 `+0x3A`／`+0x40`／`+0x46`，
	// 那正是背包的三欄。兩支互相對上。
	offEquipID     = 40
	offEquipCharge = 46
	offEquipAttr   = 52
	offPackID      = 58
	offPackCharge  = 64
	offPackAttr    = 70
	offSkills      = 80 // 兩項第二技能，一個 nibble 一項（0 = 無）
	offSpells      = 81 // 已學法術，6 bytes = 48 個位元
	slotsPerSet  = 6 // 畫面是「(Equipped) A–F」加「(Backpack) 1–6」
	itemSlots    = slotsPerSet * 2
	offWeapDice  = 76 // 近戰武器的傷害骰面數（裝備算出來的）
	offHitBonus  = 77 // 近戰命中加成
	offShotDice  = 78 // 射擊武器的傷害骰面數
	offShotBonus = 79 // 射擊命中加成

	// offBattleLevel 是**戰鬥用的等級副本**（`+113`）。
	//
	// 戰鬥判定讀的是這一格不是 `+32`：命中上限（`0x18E81` 的
	// `[bx+71h]` 除以攻擊除數）、揮擊次數、弓箭手的射擊擲值都走它。
	// 勇氣術（`2CAST2` 的 `sub_1CA40`）直接對它加 6，戰鬥結束再抄回來 ——
	// 手冊寫的「暫時提昇 6 級」就是這麼實作的，所以才需要兩份等級。
	offBattleLevel = 113
)

// 這一批偏移出自 root 的人物資料畫面繪製函式（`2PLAY.img` 的 `0x2A00`
// 起那一大段）：它對每個欄位呼叫一次 `sub_29DE(欄位序號, 值)`，
// 把序號與偏移一次全列了出來。
//
// 語意再用名冊的四十筆資料判：`+114` 只有 0 與 1
// 而且只有施法職業非零、`+115` 的值域與其他屬性一致、`+116` 在賊類明顯偏高。
// 黃金另有直接證據 —— `2PLAY.img` 的 `sub_5188` 把全隊的 `+102` 加總、
// 夠付就扣掉，而 `DEFAULT.DAT` 裡 200 金幣全在第一個角色身上。

// Resist 是抗性的種類。
type Resist byte

const (
	ResistMagic Resist = iota
	ResistFire
	ResistElectric
	ResistCold
	ResistEnergy
	ResistSleep
	ResistPoison
	ResistAcid
	NumResists = 8
)

var resistNames = [...]string{"魔法", "火焰", "電擊", "寒冰", "能量", "沈睡", "毒素", "強酸"}

func (r Resist) String() string {
	if int(r) >= len(resistNames) {
		return "未知"
	}
	return resistNames[r]
}

// ItemSlot 是一個物品欄。
type ItemSlot struct {
	// ID 是物品編號，0 表示空。
	ID int
	// Attr 是這一件的屬性位元組。低 6 位是命中／防護加成
	// （`sub_CE12` 取 `& 0x3F` 當加成），高 2 位語意未解。
	Attr byte
	// Charge 是可使用次數。使用一次減一（`2COMBAT.img` 的 `sub_1B92E`
	// 與 `sub_1B9A4`），減到 0 時把 ID 填成 0xFF、Attr 清 0；
	// 是 0 的時候原版印 `No charges`。讀取端與寫入端都在那兩支裡。
	Charge byte
}

// Empty 回報這一欄是不是空的。
func (s ItemSlot) Empty() bool { return s.ID == 0 }

// Bonus 是這一件帶的加成（屬性的低 6 位）。
func (s ItemSlot) Bonus() int { return int(s.Attr & 0x3F) }

// EquippedSlots、BackpackSlots 分別是已裝備與背包的欄位數。
const (
	EquippedSlots = slotsPerSet
	BackpackSlots = slotsPerSet
)

// Equipped 回傳已裝備的六欄。
func (c *Character) Equipped() []ItemSlot { return c.Items[:EquippedSlots] }

// Backpack 回傳背包的六欄。
func (c *Character) Backpack() []ItemSlot { return c.Items[EquippedSlots:] }

func readU32(r []byte, off int) int {
	return int(r[off]) | int(r[off+1])<<8 | int(r[off+2])<<16 | int(r[off+3])<<24
}

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
// 手冊列的是**七項**，而記錄的屬性區只有六項 —— **不在區塊裡的是耐力**，
// 不是運氣：基礎在 +39、當前在 +115。三條獨立證據都指向同一個順序：
//
//   - 建角色的寫入處 `sub_18624`（`1MENU2`）把畫面上第 4 格（耐力）寫進
//     `+0x27`／`+0x73`，第 5–7 格（速度／準確度／運氣）才寫進
//     `+0x13`／`+0x14`／`+0x15`。
//   - 大腦淨化 `sub_1C5CA`（`2BRAIN`）逐項扣掉第二技能給的加值，
//     七個有屬性效果的技能與手冊寫的屬性**七比七全中**。
//   - 原版自己的六項加值標籤表（`ds:4318`）就是
//     Might／Intellect／Personality／Speed／Accuracy／Luck，沒有 Endurance。
type Stat int

const (
	Might Stat = iota
	Intellect
	Personality
	Speed
	Accuracy
	Luck
	NumStats
)

// 屬性名用手冊的官方譯名。手冊本身對 Accuracy 有三種寫法
// （準確度／精確度／準確性），取內文的「準確度」。
var statNames = [NumStats]string{"力量", "智慧", "人格", "速度", "準確度", "運氣"}

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
	// BattleLevel 是戰鬥判定用的等級（記錄 `+113`）。平常等於 Level，
	// 勇氣術會把它加 6，戰鬥結束再由 ResetBattleLevel 抄回來。
	BattleLevel int
	Food        int
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

	// Exp 是經驗點數（記錄 +98，uint32）。
	//
	// 名冊四十筆跟著等級一路從 0 走到 1,280,000：一級 0–772、二級 3,225、
	// 五級 12,000–16,000、九級 192,000–256,000、十三級 1,280,000。
	Exp int

	// Gold、Gems 是黃金（+102，uint32）與寶石（+92，uint16）。
	Gold, Gems int

	// SL 是法力等級（+114）、Endurance 是耐力（+115）、Thievery 是盜行（+30）。
	//
	// **耐力是唯一不在 +0x10 那六格區塊裡的屬性**：基礎在 +39、當前在 +115
	// （見 Stat 的三條證據）。四十筆逐筆相同 —— 沒有人被增減益動過耐力，
	// 所以分不出哪個是基礎、哪個是當前，這裡照「基礎在前」的慣例。
	//
	// 盜行只有賊與忍者非零 —— 四十筆裡六個非零，職業全是這兩種，
	// 與手冊「扒手技能增加盜行」的設定一致。
	SL, Endurance, Thievery int

	// Skills 是兩項第二技能的代碼（1–15，0 表示沒有），存在 +80 的兩個
	// nibble 裡。
	//
	// 值域看起來很怪（名冊裡有 219、197、154、244）就是因為它是兩項擠在
	// 一個位元組：219 = 0xDB → 13 與 11。root 的人物資料畫面對 `+80`
	// 呼叫了**兩次**（序號 0x14 與 0x15），正是要顯示兩項。
	Skills [2]int

	// SpellsKnown 是已學法術的位元遮罩，48 個位元對應該系的 48 個法術。
	//
	// 驗證：不會施法的職業四十筆一個位元都沒有；會施法的十四筆全非零，
	// 而且位元數跟著等級走 —— 一級 4 個、二級 7 個、三級以上全滿。
	SpellsKnown [6]byte

	// Resist 是八種抗性的百分比，順序照介面：
	// 魔法、火焰、電擊、寒冰、能量、沈睡、毒素、強酸。
	//
	// 與手冊的種族修正吻合：侏儒魔法 35、矮人毒素 60、精靈沈睡 30、
	// 半獸人沈睡與毒素各 30（手冊：「對睡眠法術及毒藥有少許抗力」）。
	Resist [NumResists]int

	// Items 是十二個物品欄：前六個是已裝備（畫面上的 A–F），
	// 後六個是背包（1–6）。位置出自 `2CMDS.img` 的 `sub_CE12`：
	// 它以 `記錄 + 欄位 + 0x28` 取物品編號、`+ 0x34` 取屬性。
	Items [itemSlots]ItemSlot

	// WeaponDice、HitBonus 是近戰的傷害骰面數與命中加成（+76／+77），
	// ShotDice、ShotBonus 是射擊版本（+78／+79）。
	//
	// 這幾個是**裝備算出來的**：預設角色全是 0，名冊裡有裝備的人才非零
	// （+76 最高 7、+77 最高 2）。裝備欄本身還沒解，所以沒有裝備時
	// 戰鬥層會退回由力量推的骰面數。
	WeaponDice, HitBonus   int
	// MissileDice、MissileBonus 是射擊的骰面數與加成（+78／+79）。
	// 弓箭手射擊時骰面不看這一格，改用 min(等級, 100)（見 Shooter）。
	MissileDice, MissileBonus int
	ShotDice, ShotBonus    int

	// DamageBonus 是傷害加成。原版由武器、屬性與全域加成合出來
	// （`ds:54A1`），這裡先跟著武器走。
	DamageBonus int

	// AC 是防護等級（記錄 +36）。
	//
	// 位置由 `2COMBAT.img` 的 `sub_8398` 確認：那支程序拿目標記錄的
	// `[bx+24h]`（就是 +36）去減命中門檻，差值就是命中率。
	//
	// 資料佐證：六個預設角色是 0–4，名冊四十筆是 0–13 —— 低等級人物
	// 沒什麼護甲，數量級對得上。
	AC int

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
		Level:       int(r[offLevel]),
		BattleLevel: int(r[offBattleLevel]),
		Food:  int(r[offFood]),
		AC:       int(r[offAC]),
		CondBits: r[offCond],
		HP:    int(r[offHP]) | int(r[offHP+1])<<8,
		MaxHP: int(r[offMaxHP]) | int(r[offMaxHP+1])<<8,
		SP:    int(r[offSP]) | int(r[offSP+1])<<8,
		MaxSP: int(r[offMaxSP]) | int(r[offMaxSP+1])<<8,
		Gems:  int(r[offGems]) | int(r[offGems+1])<<8,
		Exp:   readU32(r, offExp),
		Gold:  readU32(r, offGold),
		SL:    int(r[offSL]),
		Endurance: int(r[offEnd]),
		Thievery: int(r[offThief]),
		WeaponDice: int(r[offWeapDice]),
		HitBonus:   int(r[offHitBonus]),
		MissileDice:  int(r[offHitBonus+1]),
		MissileBonus: int(r[offHitBonus+2]),
		ShotDice:   int(r[offShotDice]),
		ShotBonus:  int(r[offShotBonus]),
		Raw:   append([]byte(nil), r...),
	}
	for i := Stat(0); i < NumStats; i++ {
		c.Base[i] = int(r[offStats+int(i)])
		c.Current[i] = int(r[offCur+int(i)])
	}
	for i := 0; i < NumResists; i++ {
		c.Resist[i] = int(r[offResist+i])
	}
	for i := 0; i < slotsPerSet; i++ {
		c.Items[i] = ItemSlot{
			ID: int(r[offEquipID+i]), Charge: r[offEquipCharge+i], Attr: r[offEquipAttr+i],
		}
		c.Items[slotsPerSet+i] = ItemSlot{
			ID: int(r[offPackID+i]), Charge: r[offPackCharge+i], Attr: r[offPackAttr+i],
		}
	}
	c.Skills = [2]int{int(r[offSkills] >> 4), int(r[offSkills] & 0x0F)}
	copy(c.SpellsKnown[:], r[offSpells:offSpells+6])
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

	// 底下四個由 `2CAST1.OVL` 的治療系法術定出來 —— 每一支清掉哪幾位，
	// 位元的語意就是那條法術治的東西：
	//
	//	解毒術   狀況 &= 0x77   清 bit 3
	//	治病術   狀況 &= 0x7B   清 bit 2
	//	急救術   狀況 &= 0x2F   清 bit 4（連同 6、7）
	//	恢復術   狀況 = 0       全清（前提是 < 0x80）
	//	解除石化 狀況 == 0x82 才作用
	//	復活術   狀況 == 0x81 才作用
	CondBitDiseased = 0x04 // 疾病
	CondBitPoisoned = 0x08 // 中毒
	// CondBitAsleep 是沈睡。判準是喚醒術（`sub_1CBEC`）整隊掃過去
	// 只做一件事：`and [記錄+38], 6Fh` —— 清掉的就是這一位元。
	CondBitAsleep = 0x10

	// CondPetrified、CondDeadBits 是**整個位元組**的值，不是單一位元。
	// 解除石化與復活術用 `cmp` 比整個位元組，不是測位元。
	CondPetrified = 0x82
	CondDeadBits  = 0x81
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

// FieldByte 讀角色記錄裡指定偏移的位元組。
//
// 事件腳本的 `0x15`／`0x18` 用選擇器指名欄位，選擇器→偏移的對照
// 出自原版 `sub_1AA00` 的跳表（`data/fields.json`）。
func (c *Character) FieldByte(off int) byte {
	if off < 0 || off >= len(c.Raw) {
		return 0
	}
	return c.Raw[off]
}

// SetFieldByte 依 `舊值 & 遮罩 | 新值` 改寫記錄，然後重新解析整筆。
//
// 改的是 `Raw`，不是解析出來的欄位 —— 腳本可以動任何一個位元組，
// 包含還沒解出語意的那些。重新解析讓已知欄位跟著更新。
func (c *Character) SetFieldByte(off int, mask, val byte) {
	if off < 0 || off >= len(c.Raw) {
		return
	}
	c.Raw[off] = c.Raw[off]&mask | val
	*c = parseCharacter(c.Raw)
}

// FieldValue 讀記錄裡寬度為 1／2／4 的欄位（小端序）。
func (c *Character) FieldValue(off, width int) uint32 {
	if off < 0 || off+width > len(c.Raw) {
		return 0
	}
	var v uint32
	for i := width - 1; i >= 0; i-- {
		v = v<<8 | uint32(c.Raw[off+i])
	}
	return v
}

// SetFieldValue 寫回寬度為 1／2／4 的欄位，然後重新解析整筆。
func (c *Character) SetFieldValue(off, width int, v uint32) {
	if off < 0 || off+width > len(c.Raw) {
		return
	}
	for i := 0; i < width; i++ {
		c.Raw[off+i] = byte(v >> (8 * i))
	}
	*c = parseCharacter(c.Raw)
}

// RecomputeAC 依原版的公式重算防護等級（記錄 `+36`）。
//
// 抄自 root 的 `sub_14F3A`（每次刷新隊伍面板時對每個成員跑一次）：
//
//	修正 = 屬性修正(記錄 +19 速度)     ; sub_1354A，門檻表 ds:4D84
//	修正 < 0 → 當 0                    ; `cmp al, 0F0h` 那一行
//	防護 = 記錄 +31 + 修正
//	防護 <= 0 → 0；> 255 → 255
//
// `+31` 是**裝備累加出來的防護值**；穿脫護甲改的是它，不是 `+36`。
//
// 建角時寫進 `+36` 的是另一張表（`ds:74D`，`gamedata.Creation.ACByStat`），
// 那不是第二套規則 —— 它就是同一道階梯往上挪兩點，見
// `docs/formats/08-combat.md`「建角的防護等級表與屬性修正表是同一張」。
//
// 名冊裡沒被編進隊伍的角色，`+36` 是 0 —— 原版只對隊伍成員算，
// 所以那些槽位的值是舊的。載入時重算一次就對得起來。
func (c *Character) RecomputeAC() {
	bonus := 0
	if data != nil {
		bonus = data.StatBonus(c.Base[Speed])
	}
	if bonus < 0 {
		bonus = 0
	}
	ac := int(c.Raw[offGearAC]) + bonus
	switch {
	case ac < 0:
		ac = 0
	case ac > 255:
		ac = 255
	}
	c.AC = ac
	c.Raw[offAC] = byte(ac)
}

// GearAC 是裝備給的防護值（記錄 `+31`）。
func (c *Character) GearAC() int { return int(c.Raw[offGearAC]) }

// RemovePackItem 把背包第 slot 格的物品拿掉，後面的往前補。
//
// 抄自 root 的 `sub_13766`：三個平行陣列一起搬 ——
// 編號 `+58`、充能 `+64`、屬性 `+70`，各六格。
// 那三個位移是先前從 `sub_CE12` 推出來的，這支函式獨立印證了一次。
func (c *Character) RemovePackItem(slot int) {
	if slot < 0 || slot >= slotsPerSet || len(c.Raw) != RecordSize {
		return
	}
	for i := slot; i < slotsPerSet-1; i++ {
		c.Raw[offPackID+i] = c.Raw[offPackID+i+1]
		c.Raw[offPackCharge+i] = c.Raw[offPackCharge+i+1]
		c.Raw[offPackAttr+i] = c.Raw[offPackAttr+i+1]
	}
	last := slotsPerSet - 1
	c.Raw[offPackID+last] = 0
	c.Raw[offPackCharge+last] = 0
	c.Raw[offPackAttr+last] = 0
	*c = parseCharacter(c.Raw)
}

// GivePackItem 把一件物品放進第一個空的背包格，成功回 true。
//
// 三個平行陣列一起寫（編號 `+58`、可用次數 `+64`、屬性 `+70`），
// 與 RemovePackItem 的搬移是同一組位移。原版的寫入處在 `2PLAY.img`
// 的 `sub_19B44`（事件腳本的「給予物品」）。
func (c *Character) GivePackItem(it ItemSlot) bool {
	if len(c.Raw) != RecordSize {
		return false
	}
	for i := 0; i < slotsPerSet; i++ {
		if c.Raw[offPackID+i] != 0 {
			continue
		}
		c.Raw[offPackID+i] = byte(it.ID)
		c.Raw[offPackCharge+i] = it.Charge
		c.Raw[offPackAttr+i] = it.Attr
		*c = parseCharacter(c.Raw)
		return true
	}
	return false
}

// setCond 改寫狀況位元組並重新推導 Condition。
func (c *Character) setCond(v byte) {
	c.CondBits = v
	if len(c.Raw) == RecordSize {
		c.Raw[offCond] = v
	}
	switch {
	case v&CondBitSevere != 0:
		c.Condition = CondDead
	case v&CondBitUnconscious != 0:
		c.Condition = CondUnconscious
	default:
		c.Condition = CondGood
	}
}

// addHP 加生命，夾在上限。
func (c *Character) addHP(n int) {
	c.HP += n
	if c.HP > c.MaxHP {
		c.HP = c.MaxHP
	}
	if len(c.Raw) == RecordSize {
		c.Raw[offHP] = byte(c.HP)
		c.Raw[offHP+1] = byte(c.HP >> 8)
	}
}

// RollAttributes 擲一組新角色的屬性（`1MENU2.img` 的 `sub_189F8`）。
//
//	重複 3 輪，對 7 格各做一次：該格 += rand(10, 79) / 10
//
// `rand(10, 79) / 10` 的商是 **1–7 均勻分佈** —— 十個十位數區間各含
// 十個值，除以 10 之後每個商各佔十分之一。原版沒有 `1d7`，這是用
// 既有的 `rand(下限, 上限)` 湊出來的，照抄才會與原版的隨機序列一致。
//
// 所以每格是 **3d7、值域 3–21**，七格一起擲，沒有重骰也沒有分配點數。
//
// 回傳 7 格而不是 6 格：擲骰程序處理的是七格（第七格是運氣），
// 而角色記錄的屬性區只有六格 —— 那是兩件事，見
// `docs/formats/10-character-creation.md`。
func RollAttributes(r *Rand) [7]int {
	var out [7]int
	for round := 0; round < 3; round++ {
		for i := range out {
			out[i] += r.Range(10, 79) / 10
		}
	}
	return out
}

// EffectiveLevel 是戰鬥判定要用的等級。
//
// 原版讀的是記錄 `+113`（`0x18E81` 的 `[bx+71h]`），不是 `+32`。
// 名冊裡兩者逐筆相同，所以直接用 `+32` 也「看起來對」——
// 直到勇氣術把 `+113` 加 6 為止。
//
// 低於 `Level` 時當成「沒設」而回 `Level`：原版把絕對值存在 `+113`，
// 但目前找到的唯一寫入端（勇氣術）只會往上加。這個下限同時擋掉一整類
// 錯誤 —— 只改了 `Level` 卻忘了同步 `+113`，戰力會無聲地停在舊等級。
// 哪天找到會**調降** `+113` 的效果，要改的就是這裡。
func (c *Character) EffectiveLevel() int {
	if c.BattleLevel > c.Level {
		return c.BattleLevel
	}
	return c.Level
}

// ResetBattleLevel 把戰鬥用的等級抄回本體，戰鬥結束時呼叫。
func (c *Character) ResetBattleLevel() { c.BattleLevel = c.Level }

// FreeBackpackSlot 回傳第一個空的背包格（Items 的索引），滿了回 -1。
func (c *Character) FreeBackpackSlot() int {
	for i := EquippedSlots; i < len(c.Items); i++ {
		if c.Items[i].Empty() {
			return i
		}
	}
	return -1
}
