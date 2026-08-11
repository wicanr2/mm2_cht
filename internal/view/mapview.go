package view

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 地圖畫面。
//
// **原版沒有這個東西** —— 1988 年靠磁片出貨，遊戲裡沒有任何地圖畫面，
// 玩家拿方格紙自己畫，五座城鎮的地圖印在手冊裡。這裡加它是為了把紙本的
// 東西收進遊戲，所以規則跟著紙本走：
//
//   - 五座城鎮：整張看得到（手冊本來就印了）
//   - 其他地圖：只顯示走過的格（紙本沒給，當年也是自己畫出來的）
//
// 版面用整個 320×200 的原版像素層畫格線與牆，地名與座標走高解析層。

const (
	mapCell = 11             // 一格 11 px，16 格 = 176
	mapOX   = 8              // 左上角
	mapOY   = 14             // 標題底下
	mapSide = game.MapW * mapCell
)

// MapInfo 是地圖畫面要標的字。
type MapInfo struct {
	// Title 是地名，空的話只印編號。
	Title string
}

// DrawMap 把目前這張地圖畫成一整頁。
// roomWall 是房間輪廓那些牆的顏色（EGA 11 亮青），一般的牆是 15 白。
const roomWall = 11

func DrawMap(s *render.Screen, w *game.World, a Assets, info MapInfo) {
	s.Clear(0)
	m := w.CurrentMap()
	if m == nil {
		return
	}
	full := game.Mapped(w.MapIndex)

	// 外框
	outline(s, mapOX-1, mapOY-1, mapSide+2, mapSide+2, 8)

	for c := 0; c < game.MapCells; c++ {
		cx, cy := c%game.MapW, c/game.MapW
		// 第 0 列在南邊，畫面上要翻過來（見 docs/formats/06 §座標系）。
		px := mapOX + cx*mapCell
		py := mapOY + (game.MapH-1-cy)*mapCell
		seen := full || w.Explored.Seen(w.MapIndex, cx, cy)
		if !seen {
			continue
		}
		// 走過的地板塗一層暗底，沒去過的留黑 —— 一眼看得出哪裡還沒探。
		fill(s, px, py, mapCell, mapCell, 8)
		fill(s, px+1, py+1, mapCell-2, mapCell-2, 0)
		// 房間輪廓上的牆換一個顏色（見 game.AttrRoom）。125 格分佈在 6 張圖，
		// 排成矩形外框或三面牆的房間 —— 在自動地圖上一眼看得出是一個區塊。
		wallCol := uint8(15)
		if m.InRoom(cx, cy) {
			wallCol = roomWall
		}
		if m.HasWall(cx, cy, game.North) {
			fill(s, px, py, mapCell, 2, wallCol)
		}
		if m.HasWall(cx, cy, game.South) {
			fill(s, px, py+mapCell-2, mapCell, 2, wallCol)
		}
		if m.HasWall(cx, cy, game.West) {
			fill(s, px, py, 2, mapCell, wallCol)
		}
		if m.HasWall(cx, cy, game.East) {
			fill(s, px+mapCell-2, py, 2, mapCell, wallCol)
		}
	}

	// 隊伍：一個亮塊加朝向的一小截
	px := mapOX + w.X*mapCell
	py := mapOY + (game.MapH-1-w.Y)*mapCell
	fill(s, px+3, py+3, mapCell-6, mapCell-6, 14)
	// 朝向：地圖座標的 y 往北增加，畫面 y 往下增加，所以 y 要反號、x 不用。
	dx, dy := w.Face.Delta()
	fill(s, px+4+dx*3, py+4-dy*3, 3, 3, 12)

	s.Flush()

	st := a.white()
	title := info.Title
	if title == "" {
		title = fmt.Sprintf("地圖 %d", w.MapIndex)
	}
	s.DrawText(st, title, mapOX*render.Scale, 2*render.Scale)

	// 右側：座標、朝向、探索進度
	info2 := []string{
		fmt.Sprintf("X %2d  Y %2d", w.X, w.Y),
		fmt.Sprintf("朝向 %s", w.Face),
	}
	if full {
		info2 = append(info2, "手冊有印全圖")
	} else {
		info2 = append(info2, fmt.Sprintf("已探 %d／%d 格",
			w.Explored.Count(w.MapIndex), game.MapCells))
	}
	info2 = append(info2, "", "按任意鍵離開")
	for i, l := range info2 {
		s.DrawText(st, l, (mapOX+mapSide+6)*render.Scale, (mapOY+i*12)*render.Scale)
	}
}
