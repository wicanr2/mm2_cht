package game

import "fmt"

// 世界網格：二十張野外圖由 `ATTRIB` 的鄰接欄位連成 5×4 的**環面**。
//
// 走出來的，不是排出來的：從地圖 5 一路往東 `5→6→7→8→33→5` 週期 5，
// 一路往北 `5→15→12→9→5` 週期 4，5×4 正好是全部野外圖。世界會繞回來，
// 所以拿 BFS 當平面排會得到二十幾筆假矛盾。
//
// 推導、與珍017 說明書 `A1`–`E4` 的對照，以及兩個錨點見
// `docs/research/world-grid-oracle.md`。
const (
	WorldCols = 5 // A–E，往東遞增
	WorldRows = 4 // 1–4，往南遞增
)

// worldAnchor 是網格原點 `A1` 的地圖編號。
//
// **整份程式只有這一個值是寫死的**，其餘全部由玩家自己那份 `ATTRIB.DAT`
// 現算。它定得住是因為說明書寫米德格特在「區域 C2，X=7 Y=3」，而野外
// 地圖 11 的 `(7,3)` 事件腳本是 `0c 00 f5`（換到地圖 0 ＝ 米德格特）——
// 從 `C2 = 11` 反推回原點就是 5。
const worldAnchor = 5

// WorldGrid 是二十張野外圖排好的 5×4 網格，`[列][行]`，
// 列 0 是最北的 `1`、行 0 是最西的 `A`。
//
// attrs 不足或鄰接資料不成環時回 nil —— 不猜、不補。
func WorldGrid(attrs []MapAttr) [][]int {
	if worldAnchor >= len(attrs) {
		return nil
	}
	// 先量兩個方向的週期。長度不對就不是這份資料。
	row := ring(attrs, worldAnchor, (*MapAttr).East)
	col := ring(attrs, worldAnchor, (*MapAttr).North)
	if len(row) != WorldCols || len(col) != WorldRows {
		return nil
	}
	grid := make([][]int, WorldRows)
	m := worldAnchor
	for r := 0; r < WorldRows; r++ {
		grid[r] = make([]int, WorldCols)
		c := m
		for x := 0; x < WorldCols; x++ {
			grid[r][x] = c
			c = attrs[c].East()
		}
		// 往南走一列。`+7` 是南由 `sub_1B75E` 定死，不是猜的。
		m = attrs[m].South()
	}
	return grid
}

// ring 沿著某個方向一直走回起點，回傳走過的地圖編號。
// 走超過 60 步就是資料不成環，回 nil。
func ring(attrs []MapAttr, start int, step func(*MapAttr) int) []int {
	out := []int{start}
	for m := step(&attrs[start]); m != start; m = step(&attrs[m]) {
		if m < 0 || m >= len(attrs) || len(out) > MapAttrCount {
			return nil
		}
		out = append(out, m)
	}
	return out
}

// RegionOf 回傳某張地圖在世界網格上的區域碼（`A1`–`E4`）。
// 不在網格上（城鎮、地城、四個元素領域）回空字串。
func RegionOf(attrs []MapAttr, mapIndex int) string {
	grid := WorldGrid(attrs)
	for r := range grid {
		for c, m := range grid[r] {
			if m == mapIndex {
				return regionCode(r, c)
			}
		}
	}
	return ""
}

// regionCode 把列行換成說明書的區域碼。
func regionCode(row, col int) string {
	return fmt.Sprintf("%c%d", 'A'+rune(col), row+1)
}

// WorldTileset 是某一格的貼圖組碼（`ATTRIB +4` 低 nibble）：
// 9 沙漠、10 海洋、11 沼澤、12 凍原。畫世界圖時拿它上色。
func WorldTileset(attrs []MapAttr, mapIndex int) int {
	if mapIndex < 0 || mapIndex >= len(attrs) {
		return 0
	}
	return attrs[mapIndex].Scene()
}
