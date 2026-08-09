package game

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

// ATTRIB.DAT：每張地圖 64 bytes，60 張，解壓後 3,840 bytes。
//
// 定錨靠第一個位元組：六十筆的 `+0` 剛好是 0…59，一個不差。
// 這也是為什麼 stride 只能是 64 —— 換成 15、16 或 26，那個遞增序列就散了。
const (
	MapAttrSize  = 64
	MapAttrCount = 60
)

// 鄰接欄位在 `+5`…`+8`。哪一對是南北、哪一對是東西，由對向性定：
// 「`+5` 的鄰居其 `+7` 指回自己」六十張全中，`+6` 與 `+8` 同理，
// 而交叉配對（+5/+6、+5/+8…）只有 40/60。
//
// `+6` 是東：地圖 5 的 `+6` 是 6，而地圖 6 的 `+8` 是 5，
// 編號沿著這個方向遞增。`+5` 與 `+7` 其一是北、其一是南，還沒定 ——
// 要對照世界地圖的實際方位才能斷。
// BashDifficulty 是撞門的難度門檻（`+18`）。
//
// 位置由 `2MISC.img` 的 `0xC1E4` 定出：撞門時拿隊伍力量與 `ds:5998` 比，
// 而 `ds:5998` 落在地圖屬性那 64 bytes 的 `+18`（`ds:5986` 是起點）。
//
// 值全是十的倍數、0–100：五座城鎮 10/30/20/40/30（中門最低），
// 野外二十張全是 0（沒有門），地城 20–100 隨深度上升。
const (
	attrRunChance = 13 // 戰鬥中「溜跑」的成功率（百分比）
	attrGroundPos = 22 // 地面返回座標（低 nibble X、高 nibble Y），野外圖是 0
	attrGroundMap = 24 // 地面地圖編號；野外圖填自己
	attrSpellBan  = 26 // 禁止哪些法術的位元組（原版載到 `ds:59A0`）
)

const attrBashDifficulty = 18

// attrLockDifficulty 是開鎖失敗後那一擲比對的門檻（`+19`）。
//
// 位置由 `2MISC.img` 的 `0xC2F3` 定出（`ds:5999`）。形狀與 `+18` 一樣：
// 城鎮 10/50/30/30/20、野外全 0、地城 20–90。
const attrLockDifficulty = 19

const (
	attrScene     = 4  // 低 nibble 是場景（貼圖組）碼
	attrEncounter = 9  // 遭遇機率的分母：每走一步擲 rand(1, N)，擲出 1 才遇敵
	attrEntryPos  = 14 // 預設進入座標：低 nibble = X、高 nibble = Y

	// 四個鄰接欄位。方位由 `sub_1B75E`（走出邊界時換圖）定死：
	// X 溢位到 0x10 讀 `+6`（東）、掉到 0xFF 讀 `+8`（西）；
	// Y 溢位到 0x10 讀 `+5`（北）、掉到 0xFF 讀 `+7`（南）。
	// 北是 +Y，所以 `+5` 是北、`+7` 是南。
	neighborNorth = 5
	neighborEast  = 6
	neighborSouth = 7
	neighborWest  = 8

	attrTrapShift = 20 // 陷阱傷害的左移量
)

// MapAttr 是一張地圖的屬性。
type MapAttr struct {
	Index int
	Raw   [MapAttrSize]byte
}

// Indoor 回報這張地圖走不走室內的通行模型。
//
// 判準是撞門難度非零 —— 野外沒有門，那個欄位是 0。六十張裡三十六張是室內
// （五座城鎮加地城），二十四張是野外。
func (a *MapAttr) Indoor() bool { return a.BashDifficulty() != 0 }

// BashDifficulty 回傳這張地圖撞門的難度門檻。
func (a *MapAttr) BashDifficulty() int { return int(a.Raw[attrBashDifficulty]) }

// LockDifficulty 回傳這張地圖開鎖失敗後那一擲的門檻。
func (a *MapAttr) LockDifficulty() int { return int(a.Raw[attrLockDifficulty]) }

// Entry 回傳這張地圖的預設進入座標（`+14`），低 nibble 是 X、高 nibble 是 Y。
//
// 換圖時呼叫端給的 X 是 `0xFF`（＝「沒有指定落點」）就用這個值：
// `sub_1B5EA` 的 `0x1B6DC` 與 `sub_5E68` 的 `0x15E36` 兩處都是把
// `ds:5994`（＝ ATTRIB 記錄 `+14`）拆成兩個 nibble。
//
// **事件腳本的傳送（opcode `0x0c`）走不到這條** —— 它給的 X 是
// `座標 & 0x0F`，永遠在 0–15。會用到的是別的換圖路徑（走到地圖邊緣
// 接到鄰圖之類），那些還沒解。
//
// `ok` 為 false 表示這張圖沒有預設落點（值是 `0xFF`）。
func (a *MapAttr) Entry() (x, y int, ok bool) {
	v := a.Raw[attrEntryPos]
	if v == 0xFF {
		return 0, 0, false
	}
	return int(v & 0x0F), int(v >> 4), true
}

// Neighbor 回傳某個鄰接欄位指到的地圖編號。
func (a *MapAttr) Neighbor(field int) int { return int(a.Raw[field]) }

// East、West 回傳東西向的鄰接地圖。
func (a *MapAttr) East() int { return a.Neighbor(neighborEast) }
func (a *MapAttr) West() int { return a.Neighbor(neighborWest) }

// North、South 回傳南北向的鄰接地圖。
func (a *MapAttr) North() int { return a.Neighbor(neighborNorth) }
func (a *MapAttr) South() int { return a.Neighbor(neighborSouth) }

// Neighbors 依 Facing 的順序（北／東／南／西）回傳四個鄰接地圖。
func (a *MapAttr) Neighbors() [4]int {
	return [4]int{a.North(), a.East(), a.South(), a.West()}
}

// Scene 是這張地圖的場景碼（`+4` 的低 nibble）。
//
// `sub_1B1D4` 用它跟 `ds:16DA`（目前載入的貼圖組）比對決定要不要換圖組。
// 六十張裡室內全是 0，野外是 9／10／11／12（沙漠、沼澤、凍原、森林之類）。
func (a *MapAttr) Scene() int { return int(a.Raw[attrScene] & 0x0F) }

// TrapShift 是這張地圖的陷阱傷害左移量（`+20`）。
//
// `sub_1C338` 算的是 `基礎傷害[場景] << 這個值`。值域 1–6，
// 地圖 0 是 1、最深的地城是 5–6。
func (a *MapAttr) TrapShift() int { return int(a.Raw[attrTrapShift]) }

// EncounterRate 是這張地圖的遭遇機率分母：每走一步擲 `rand(1, N)`，
// 擲出 1 才遇敵。
//
// 出自 `sub_17EB9`：`al = ds:598F`（＝ ATTRIB `+9`）→ `rand(1, al)`
// → 結果等於 1 才進戰鬥。五座城鎮是 200 或 100，野外多半 100，
// 最低 50、最高 250。
func (a *MapAttr) EncounterRate() int { return int(a.Raw[attrEncounter]) }

// SelfContained 回報這張圖四面都指向自己 —— 城鎮就是這樣，
// 走到邊界不會接到別張圖。前五張（五座主要城鎮）全是如此。
func (a *MapAttr) SelfContained() bool {
	for _, f := range []int{neighborNorth, neighborEast, neighborSouth, neighborWest} {
		if a.Neighbor(f) != a.Index {
			return false
		}
	}
	return true
}

// ParseMapAttrs 解開 ATTRIB.DAT。
func ParseMapAttrs(blob []byte) ([]MapAttr, error) {
	raw, err := lzw.Segment(blob, 0)
	if err != nil {
		return nil, fmt.Errorf("解壓 ATTRIB.DAT: %w", err)
	}
	if len(raw) != MapAttrSize*MapAttrCount {
		return nil, fmt.Errorf("解出 %d bytes，預期 %d", len(raw), MapAttrSize*MapAttrCount)
	}
	out := make([]MapAttr, 0, MapAttrCount)
	for i := 0; i < MapAttrCount; i++ {
		m := MapAttr{Index: i}
		copy(m.Raw[:], raw[i*MapAttrSize:(i+1)*MapAttrSize])
		out = append(out, m)
	}
	return out, nil
}

// 禁止法術的位元。`ds:59A0` 在十四個 overlay 裡只有讀沒有寫 ——
// 它是 `ATTRIB` 記錄 `+26` 隨地圖一起載進來的。
//
// 認出這一段的線索是 `ds:5994`：root 在 `ds:0393 == 0FFh` 時拿它
// 當預設進入座標拆 nibble，那正是 `+14`。所以記錄基底是 `ds:5986`，
// `ds:599C`／`ds:599E`／`ds:59A0` 就是 `+22`／`+24`／`+26`。
const (
	BanTrapMonsters = 0x01 // 陷敵術
	BanTimeDistort  = 0x08 // 時間扭曲
	BanTeleportSpell = 0x10 // 傳送術
	BanEtherealize   = 0x20 // 穿透術
	BanTeleport     = 0x40 // 魯易浮標、飛行術、傳送到地面
)

// RunChance 是戰鬥中下 `R`（溜跑）成功的百分比。
//
// 出自戰鬥指令跳表 `0x19578` 的 `R` 那一格 → `sub_1914A`：擲
// `rand(1, 100)`，小於這個值就成功，那名角色當場脫離戰鬥。
//
// 值全是十的倍數：**五座城鎮都是 100**（在城裡一定跑得掉），
// 野外 20–90，地城 20–100。
func (a *MapAttr) RunChance() int { return int(a.Raw[attrRunChance]) }

// GroundPos 回傳地面返回座標。野外圖沒有（回 ok = false）。
func (a *MapAttr) GroundPos() (x, y int, ok bool) {
	v := a.Raw[attrGroundPos]
	if v == 0 {
		return 0, 0, false
	}
	return int(v & 0x0F), int(v >> 4), true
}

// GroundMap 回傳地面的地圖編號。
func (a *MapAttr) GroundMap() int { return int(a.Raw[attrGroundMap]) }

// SpellBanned 回報這張圖是否禁止某一類法術。
func (a *MapAttr) SpellBanned(bit byte) bool { return a.Raw[attrSpellBan]&bit != 0 }
