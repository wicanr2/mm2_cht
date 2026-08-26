package view

import (
	"fmt"
	"image"
	"image/color"

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
	Grid   [][]WorldCellInfo   // [列][行]，列 0 最北、行 0 最西
	Here   string              // 隊伍目前的區域碼；不在網格上時是空的
	Place  string              // 隊伍目前的地名
	Names  map[string][]string // 區域碼 → 說明書上的地名
	Map    *image.Paletted     // 修復後的彩色世界地圖；nil 時只顯示格線頁
	Page   int                 // 0 彩色大地圖、1 ATTRIB.DAT 格線圖
	Nearby string              // 目前區域最近的五座城之一
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
	if info.Page == 0 && info.Map != nil {
		drawWorldMap(s, a, info)
		return
	}
	drawWorldGrid(s, a, info)
}

func drawWorldGrid(s *render.Screen, a Assets, info WorldInfo) {
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
	s.DrawText(st, "← →　按任意鍵離開", infoX*render.Scale, (infoBottom-8)*render.Scale)
}

// drawWorldMap 顯示說明書摺頁的完整彩色平面圖。圖片以等比例 257×188
// 原版座標（771×564 輸出像素）置中，底部留一列導覽文字；不裁掉卷軸外框，
// 也不把提示蓋在 A–E／1–4 的格線標題上。
func drawWorldMap(s *render.Screen, a Assets, info WorldInfo) {
	const x, y, w, h = 31, 0, 257, 188
	s.Clear(0)
	s.BlitHiFit(info.Map, x, y, w, h)
	s.Flush()

	if mx, my, ok := worldMarker(info.Here, x, y, w, h); ok {
		drawMarker(s, mx, my)
	}

	label := info.Here
	if info.Nearby != "" {
		label += " / " + info.Nearby
	} else if info.Place != "" {
		label = info.Place
	}
	st := a.white()
	s.DrawText(st, "← →　"+label, 6*render.Scale, 190*render.Scale)
}

// worldMarker 把 A1–E4 的格線中心投影到平面圖。四個比例量自未裁切的
// 1280×935 說明書掃描外框；不靠圖片內容猜測地點。
func worldMarker(region string, x, y, w, h int) (int, int, bool) {
	if len(region) != 2 || region[0] < 'A' || region[0] > 'E' ||
		region[1] < '1' || region[1] > '4' {
		return 0, 0, false
	}
	const left, right = 0.107, 0.898
	const top, bottom = 0.052, 0.931
	c, r := float64(region[0]-'A'), float64(region[1]-'1')
	px := float64(x*render.Scale) + (left+(c+0.5)*(right-left)/5)*float64(w*render.Scale)
	py := float64(y*render.Scale) + (top+(r+0.5)*(bottom-top)/4)*float64(h*render.Scale)
	return int(px + 0.5), int(py + 0.5), true
}

func drawMarker(s *render.Screen, cx, cy int) {
	red := color.RGBA{R: 255, G: 32, B: 24, A: 255}
	yellow := color.RGBA{R: 255, G: 244, B: 64, A: 255}
	for dy := -15; dy <= 15; dy++ {
		for dx := -15; dx <= 15; dx++ {
			d2 := dx*dx + dy*dy
			if d2 >= 12*12 && d2 <= 15*15 {
				setHi(s, cx+dx, cy+dy, red)
			}
		}
	}
	for d := -9; d <= 9; d++ {
		setHi(s, cx+d, cy, yellow)
		setHi(s, cx, cy+d, yellow)
	}
}

func setHi(s *render.Screen, x, y int, c color.RGBA) {
	if image.Pt(x, y).In(s.Hi.Bounds()) {
		s.Hi.SetRGBA(x, y, c)
	}
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
