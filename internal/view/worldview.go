package view

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/render"
)

// 世界地圖畫面。
//
// **這一頁是 remake 加的，但排的是原版自己的資料。** 二十張野外圖由
// `ATTRIB` 的鄰接欄位連成 5×4 的環面，字母數字沿用珍017 說明書那一頁的
// `A1`–`E4`；推導與兩個錨點見 `docs/research/world-grid-oracle.md`。
//
// 說明書的世界地圖是彩色跨頁掃描，不能散布，所以這裡不是把它收進來，
// 是拿玩家自己那份 `ATTRIB.DAT` 重畫一張。

const (
	worldCellW = 35
	worldCellH = 44
	worldOX    = mapOX
	worldOY    = mapOY
)

// WorldCellInfo 是網格上的一格。
type WorldCellInfo struct {
	Region  string // 區域碼，`A1`–`E4`
	Map     int    // 地圖編號
	Tileset int    // `ATTRIB +4` 低 nibble：9 沙漠、10 海洋、11 沼澤、12 凍原
	Seen    bool   // 這張圖踏進去過沒有
}

// WorldInfo 是世界地圖畫面要標的東西。
type WorldInfo struct {
	Grid  [][]WorldCellInfo   // [列][行]，列 0 最北、行 0 最西
	Here  string              // 隊伍目前的區域碼；不在網格上時是空的
	Place string              // 隊伍目前的地名
	Names map[string][]string // 區域碼 → 說明書上的地名
}

// tilesetColor 把貼圖組碼換成 EGA 顏色。四組各給一個一眼分得開的色。
func tilesetColor(t int) uint8 {
	switch t {
	case 9:
		return 6 // 沙漠：棕
	case 10:
		return 1 // 海洋：藍
	case 11:
		return 2 // 沼澤：綠
	case 12:
		return 7 // 凍原：淺灰
	}
	return 8
}

var tilesetName = map[int]string{9: "沙漠", 10: "海洋", 11: "沼澤", 12: "凍原"}

// DrawWorld 畫世界地圖那一頁。
func DrawWorld(s *render.Screen, a Assets, info WorldInfo) {
	s.Clear(0)
	for r := range info.Grid {
		for c, cell := range info.Grid[r] {
			px := worldOX + c*worldCellW
			py := worldOY + r*worldCellH
			fill(s, px, py, worldCellW-1, worldCellH-1, tilesetColor(cell.Tileset))
			// 沒踏進去過的區域壓暗一半：用一格一格的點陣，不是另一個顏色 ——
			// 換顏色會與貼圖組的四個色撞在一起。
			if !cell.Seen {
				for y := py; y < py+worldCellH-1; y++ {
					for x := px + (y-py)%2; x < px+worldCellW-1; x += 2 {
						fill(s, x, y, 1, 1, 0)
					}
				}
			}
			if cell.Region == info.Here {
				outline(s, px-1, py-1, worldCellW+1, worldCellH+1, 14)
			}
		}
	}

	rows := []string{}
	if info.Here != "" {
		rows = append(rows, "隊伍在 "+info.Here)
	} else if info.Place != "" {
		rows = append(rows, info.Place)
		rows = append(rows, "不在世界網格上")
	}
	for _, n := range info.Names[info.Here] {
		rows = append(rows, HintIndent+n)
	}
	rows = append(rows, "")
	for _, t := range []int{12, 10, 11, 9} {
		rows = append(rows, tilesetName[t])
	}
	// 色票也要在 Flush 之前畫完。**`Flush` 把原版像素層蓋上去，
	// 會抹掉已經畫好的文字** —— 先畫完所有色塊、只 Flush 一次、再畫字。
	for i, l := range rows {
		y := infoTop + i*infoRowH
		if y > infoBottom-infoRowH {
			break
		}
		if n, ok := legendColor(l); ok {
			fill(s, infoX, y+1, 6, 6, n)
		}
	}
	s.Flush()

	st := a.white()
	s.DrawText(st, WorldTitle(info), worldOX*render.Scale, 2*render.Scale)
	for r := range info.Grid {
		for c, cell := range info.Grid[r] {
			px := worldOX + c*worldCellW + 2
			py := worldOY + r*worldCellH + 2
			s.DrawText(st, cell.Region, px*render.Scale, py*render.Scale)
		}
	}
	for i, l := range rows {
		y := infoTop + i*infoRowH
		if y > infoBottom-infoRowH {
			break
		}
		x := infoX
		if _, ok := legendColor(l); ok {
			x = infoX + 9 // 讓開色票
		}
		s.DrawText(st, l, x*render.Scale, y*render.Scale)
	}
	s.DrawText(st, "按任意鍵離開", infoX*render.Scale, (infoBottom-8)*render.Scale)
}

// legendColor 認出圖例那四行，回傳它的色票顏色。
func legendColor(line string) (uint8, bool) {
	for t, n := range tilesetName {
		if line == n {
			return tilesetColor(t), true
		}
	}
	return 0, false
}

// WorldTitle 是這一頁的標題，給測試與截圖用。
func WorldTitle(info WorldInfo) string {
	if info.Here == "" {
		return "世界地圖"
	}
	return fmt.Sprintf("世界地圖　%s", info.Here)
}
