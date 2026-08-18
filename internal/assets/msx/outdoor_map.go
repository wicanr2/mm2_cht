package msx

import "fmt"

// 每張地圖自己的戶外資料。
//
// 地圖檔的 id 是 `0x0100 + 地圖號`，768 bytes：前 256 是格子、
// 接著 256 bytes 另有用途（牆），`+0x200` 起是尾段：
//
//	+0x200  地圖號（自我驗證用）
//	+0x201  三格分派表，索引是「格子碼 − 1」
//	+0x204  高 nibble 是地形碼，決定地平線帶用哪一個變體
//
// **MSX 的格子碼與 DOS 的不是同一套。** 地圖 11 兩邊 256 格裡有 255 格
// 相同，看起來像同一件事；地圖 5 只有 50 格相同，地圖 41–44 一格都不同
// （DOS 全是 4，MSX 全是 1）。分派表是這個差異的來源 —— 同一個 MSX 碼
// 在不同地圖上指向不同的常式，所以碼本身沒有跨地圖的意義。
const outMapID = 0x0100

// OutCellKind 是一格要畫什麼。
type OutCellKind int

const (
	// OutCellNone ＝ 這一格什麼都不畫。
	OutCellNone OutCellKind = iota
	// OutCellFeature ＝ 畫擋路物，Arg 是第幾組（0–2）。
	OutCellFeature
	// OutCellBand ＝ 畫地平線的地形帶，Arg 是變體（0–2）。
	OutCellBand
)

// OutCell 是一格的繪圖決定。
type OutCell struct {
	Kind OutCellKind
	Arg  int
}

// OutMapData 是一張野外圖的格子與分派表。
type OutMapData struct {
	Index    int
	cells    [256]byte
	dispatch [3]byte
	// Terrain 是 `+0x204` 的高 nibble。決定地平線帶的變體，
	// 也與 DOS 的貼圖組碼是同一組編號（9 沙漠、10 海洋、11 沼澤、12 凍原）。
	Terrain int
}

// OutdoorMapData 從磁片讀一張野外圖的戶外資料。
func (d *Disk) OutdoorMapData(index int) (*OutMapData, error) {
	raw := d.File(uint16(outMapID + index))
	if len(raw) < 0x205 {
		return nil, fmt.Errorf("msx: 地圖 %d（id %04X）只有 %d bytes",
			index, outMapID+index, len(raw))
	}
	if int(raw[0x200]) != index {
		return nil, fmt.Errorf("msx: 地圖 %d 的 +0x200 寫的是 %d，版面不對",
			index, raw[0x200])
	}
	m := &OutMapData{Index: index, Terrain: int(raw[0x204] >> 4)}
	copy(m.cells[:], raw[:256])
	copy(m.dispatch[:], raw[0x201:0x204])
	return m, nil
}

// Cell 回傳這一格要畫什麼。x、y 都是 0–15。
//
// 規則出自兩支：`sub_2A57`（碼 1–3 查分派表 → 1、2 擋路物 A、3 擋路物 B、
// 4 擋路物 C）與 `sub_297A`（碼 4、5 直接是地形帶，變體看地形碼；
// 碼 1–3 查到 5、6 是變體 0，8 是變體 1，**而且這一段在地圖 41 以後整段
// 跳過** —— 那四張圖因此什麼都不畫，看到的是背景換過的地面）。
func (m *OutMapData) Cell(x, y int) OutCell {
	if m == nil || x < 0 || y < 0 || x > 15 || y > 15 {
		return OutCell{}
	}
	code := int(m.cells[y*16+x] >> 5)
	if code == 4 || code == 5 {
		return OutCell{OutCellBand, OutBandVariantFor(m.Terrain)}
	}
	if code < 1 || code > 3 {
		return OutCell{}
	}
	switch v := int(m.dispatch[code-1]); {
	case v == 1 || v == 2:
		return OutCell{OutCellFeature, 0}
	case v == 3:
		return OutCell{OutCellFeature, 1}
	case v == 4:
		return OutCell{OutCellFeature, 2}
	case m.Index >= 41:
		// `sub_297A` 的後半有 `cp 29h : jp nc` 的閘門。
		return OutCell{}
	case v == 5 || v == 6:
		return OutCell{OutCellBand, 0}
	case v == 8:
		return OutCell{OutCellBand, 1}
	}
	return OutCell{}
}
