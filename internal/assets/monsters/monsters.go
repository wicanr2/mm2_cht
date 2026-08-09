// Package monsters 解析 MONSTERS.DAT。
package monsters

import (
	"fmt"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

// RecordSize 是一筆怪物記錄的長度。
//
// 解壓後 6,656 bytes = 26 × 256。stride 26 而不是別的整除數，
// 是因為名稱欄位在每筆的同一個位置對得整整齊齊 —— 換成 52 就會變成
// 「一筆裡有兩個名字」。
const RecordSize = 26

// Count 是怪物筆數。
const Count = 256

// Monster 是一筆怪物資料。
type Monster struct {
	Index int
	Name  string

	// Stats 是名稱之後那 12 個位元組的原始值。未解的部分要能原樣送回。
	Stats [12]byte

	// 以下由 Stats 的位元欄位解出，解包規則抄自 `2COMBAT.img` 的 `sub_3B80`。
	//
	// 那些欄位是**壓縮過的**：一個位元組裡低位是基數、高位是 10 的冪次的
	// 倍率索引。所以直接拿原始位元組當數值用，會算出荒謬的結果。

	// HP 是生命點數：`(b14 & 0x3F) + 1` 乘上倍率 `[1,10,100,1000][b14>>6]`。
	HP int
	// Exp 是擊敗後給的經驗值：`(b15 & 0x1F) + 1` 乘上倍率
	// `[1,10,100,1000][(b15>>5)&3]`，b15 的 bit7 再乘 1000。
	Exp int
	// Attacks 是一次行動裡揮擊幾次：`(b20 & 0x0F) + 1`。
	Attacks int
	// Actions 是每輪最多行動幾次：`(b20 >> 4) + 1`。
	//
	// 原版把它抄進 `ds:9F9E[怪物編號]`，每行動一次減一，減到 0 就不再
	// 輪到牠（`2COMBAT.img` `0x184A8` 的 `cmp` 與 `0x184C5` 的 `dec`）。
	Actions int
	// MagicResistIndex 是記錄 `+17` 的高 3 位元，索引抗魔法百分比表
	// `ds:4DC0`（`[0,10,20,35,50,75,90,100]`）。
	//
	// 解包在 `2COMBAT.img` 的 `0x13E1E`：同一個位元組拆成五份，
	// bit 0/1/2 各自累加三個計數器（`ds:9E3A`–`9E3C`，語意未定），
	// bit 3–4 進 `ds:9E30`，bit 5–7 `& 7` 之後查表進 `ds:9E31`。
	// **`& 7` 保證索引落在八項之內**。
	//
	// 用法在 `sub_1714A`：`ds:9E31` 非 0 時擲 `rand(施法者等級, 90)`，
	// 抗性大於擲值就擋下整個法術。抗性 100 因此必定擋下。
	MagicResistIndex int
	// DamageDice 是每次攻擊的傷害骰面數，擲 `rand(1, DamageDice)`：
	// `(b23 & 0x1F) + 1`，bit5 再乘 10（乘完超過 25 就固定 250）。
	DamageDice int
	// GroupSize 是這一種怪物一次出現幾隻的上限：`(b19 & 0x0F) + 1`，
	// bit4 再乘 10。湊遭遇時擲 `rand(1, GroupSize)`（原版 `0x198A5`）。
	GroupSize int
	// MoraleTier 是士氣層 `(b19 >> 5) & 3`，索引逃走門檻表 `ds:1036`
	// （`[3, 9, 24, 255]`）。門檻低於隊伍最高等級的一半時，這隻怪
	// 每輪有五成機率逃走（`0x18592`）。255 那一層等於永不逃走。
	MoraleTier int
	// Speed 是行動順序的鍵：`(b24 & 0x1F) + 1`，bit5 再乘 10
	// （乘完超過 25 就固定 250）—— 與生命、防護、傷害骰同一套編碼。
	//
	// 戰鬥開始時抄進 `ds:9F92[槽位]`（`0x195CD`）。每次要挑下一隻
	// 行動的怪物時，在「這一輪還沒動過」的前十隻裡挑 Speed 最大的
	// （`0x1A1CC` 的迴圈）—— 是逐次取最大，不是先排序，
	// 所以同速時**索引小的先動**。
	Speed int
	// AC 是防護等級：`(b22 & 0x1F) + 1`，bit5 再乘 10。
	// `2COMBAT.img` 的隊伍攻擊路徑拿 `ds:9E2C` 與擲出值比，那個值就是它。
	AC int
	// Sprite 是 `MONSTERS.16` 的段號：`b21 & 0x7F`。
	//
	// `2COMBAT.img` `0xA384` 拿它與「目前載入的段號」（ds:9F84）比，
	// 不同才重載 —— 那是圖形索引的用法，不是數值。
	// 256 隻共用 58 個相異值（範圍 1–60），而 `MONSTERS.16` 正好 59 個段。
	Sprite int
	// Resists 是屬性抗性旗標，索引 = `sub_1714A` 的屬性參數減一
	// （0 火、1 電、2 冷、3 酸、4 睡、5 法術狀態、6 未定）。
	//
	// 原版把它們攤成 `ds:9E36`–`9E3C` 七個位元組，`sub_18674(屬性-1)`
	// 只看非 0：**有旗標就完全免疫該屬性**，沒有機率可言。
	// 解包在 `2COMBAT.img` `0x13D7A`–`0x13E19`：
	//
	//	b23 bit7 → 電    b23 bit6 → 火
	//	b22 bit7 → 酸    b22 bit6 → 冷
	//	b17 bit1 → 睡    b17 bit0 → 法術狀態    b17 bit2 → 未定
	//
	// 火那一格四隻名字帶 Fire 的怪物全部命中、零例外；冷那一格
	// Frost Dragon 命中。
	Resists [7]bool

	// Undead 是不死旗標：記錄 `+18`（`Stats[4]`）的 bit 7。
	//
	// 位置由兩件事夾出來：`2COMBAT.img` `0x13CAC` 把某個位元組的
	// bit 7 累加進 `ds:9E33`，而 `ds:9E33` 正是驅魔術（手冊：
	// 「所有的不死怪物」）與死亡之指例外檢查看的那一格。
	// 十二個位元組逐一試，只有 `Stats[4]` 讓名字帶 Zombie／Skeleton／
	// Ghost／Mummy／Vampire／Lich／Wraith／Spectre／Wight 的七隻全部命中、
	// 零漏網。
	Undead bool

	// Tier 是難度層級，等於怪物編號的高 nibble。命中門檻查表用它索引。
	Tier int
}

// multipliers 是生命與經驗的倍率表（DGROUP ds:4DB8）。
//
// 這四個值寫在這裡而不是資料檔，是因為它們是**解包規則的一部分**——
// 位元欄位的語意本身，不是可調的平衡數值。改了就不是原版的格式了。
var multipliers = [4]int{1, 10, 100, 1000}

// unpack 把 12 個位元組的位元欄位攤成數值。
func (m *Monster) unpack() {
	b14, b15, b20, b23 := m.Stats[0], m.Stats[1], m.Stats[6], m.Stats[9]

	m.HP = (int(b14&0x3F) + 1) * multipliers[b14>>6]

	m.Exp = (int(b15&0x1F) + 1) * multipliers[(b15>>5)&3]
	if b15 > 0x7F {
		m.Exp *= 1000
	}

	m.Attacks = int(b20&0x0F) + 1
	// 同一個位元組的高 nibble 是**每輪最多行動幾次**（原版
	// `ds:9E26 = (b20 >> 4) + 1`，戰鬥中每行動一次就減一）。
	m.Actions = int(b20>>4) + 1
	// 記錄 +17 的高 3 位元索引抗魔法百分比表（原版 `ds:9E31 = ds:4DC0[(b17>>5)&7]`）。
	m.MagicResistIndex = int(m.Stats[3]>>5) & 7

	m.DamageDice = int(b23&0x1F) + 1
	if b23&0x20 != 0 {
		if m.DamageDice > 0x19 {
			m.DamageDice = 250
		} else {
			m.DamageDice *= 10
		}
	}

	m.Sprite = int(m.Stats[7] & 0x7F)

	// b19 一個位元組裝三件事（`2COMBAT.img` `0x13CD2`）。
	b19 := m.Stats[5]
	m.GroupSize = int(b19&0x0F) + 1
	if b19&0x10 != 0 {
		m.GroupSize *= 10
	}
	m.MoraleTier = int(b19>>5) & 3

	// b24 的低 5 位是速度，編碼與傷害骰同形（`0x13DCB`）。
	b24 := m.Stats[10]
	m.Speed = int(b24&0x1F) + 1
	if b24&0x20 != 0 {
		if m.Speed > 0x19 {
			m.Speed = 250
		} else {
			m.Speed *= 10
		}
	}

	b22 := m.Stats[8]
	m.AC = int(b22&0x1F) + 1
	if b22&0x20 != 0 {
		m.AC *= 10
	}

	m.Undead = m.Stats[4]&0x80 != 0

	m.Resists[0] = b23&0x40 != 0 // 火
	m.Resists[1] = b23&0x80 != 0 // 電
	m.Resists[2] = b22&0x40 != 0 // 冷
	m.Resists[3] = b22&0x80 != 0 // 酸
	m.Resists[4] = m.Stats[3]&0x02 != 0 // 睡
	m.Resists[5] = m.Stats[3]&0x01 != 0 // 法術狀態
	m.Resists[6] = m.Stats[3]&0x04 != 0

	m.Tier = m.Index >> 4
}

// Parse 解出全部 256 筆。
//
// 名稱是**每個位元組加了 0x80** 的 ASCII，14 bytes、空格填充。
// 這也是為什麼直接 grep `Goblin` 找不到東西 —— 檔案裡存的是 `0xc7 0xef...`。
func Parse(blob []byte) ([]Monster, error) {
	raw, err := lzw.Segment(blob, 0)
	if err != nil {
		return nil, fmt.Errorf("解壓 MONSTERS.DAT: %w", err)
	}
	if len(raw) != RecordSize*Count {
		return nil, fmt.Errorf("解出 %d bytes，預期 %d", len(raw), RecordSize*Count)
	}
	out := make([]Monster, 0, Count)
	for i := 0; i < Count; i++ {
		r := raw[i*RecordSize : (i+1)*RecordSize]
		m := Monster{Index: i, Name: decodeName(r[:14])}
		copy(m.Stats[:], r[14:])
		m.unpack()
		out = append(out, m)
	}
	return out, nil
}

// decodeName 把 +0x80 的名稱還原成 ASCII。
func decodeName(b []byte) string {
	s := make([]byte, len(b))
	for i, c := range b {
		if c >= 0x80 {
			c -= 0x80
		}
		s[i] = c
	}
	return strings.TrimRight(string(s), " ")
}
