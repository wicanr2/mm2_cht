package view

import (
	"fmt"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 地圖畫面。
//
// **這個全螢幕的地圖畫面是 remake 加的。** 1988 年靠磁片出貨，玩家拿方格紙
// 自己畫，五座城鎮的地圖印在手冊裡。這裡加它是為了把紙本的東西收進遊戲，
// 所以規則跟著紙本走：
//
// 原版其實有一張俯視圖：`.16` 的 `B` 系列（`TOWNB`／`CAVEB`／`CASTLEB`／
// `OUTB`）畫的就是它 —— 藍底格子、四面牆的各種組合、綠色的門，
// 加上隊伍位置一個方向箭頭，探索中按 `M`（`2PLAY` 分派 `0x18189`）或
// 施定位術就會出現（見 docs/research/command-keys-oracle.md）。
// 這裡不去逐像素模仿那張圖：原版一次只畫 16×16 的當前圖，而這一頁要放
// 已探索狀態、房間輪廓與攻略提示，版面需求不一樣。
//
//   - 五座城鎮：整張看得到（手冊本來就印了）
//   - 其他地圖：只顯示走過的格（紙本沒給，當年也是自己畫出來的）
//
// 版面用整個 320×200 的原版像素層畫格線與牆，地名與座標走高解析層。

const (
	mapCell = 11 // 一格 11 px，16 格 = 176
	mapOX   = 8  // 左上角
	mapOY   = 14 // 標題底下
	mapSide = game.MapW * mapCell
)

// MapInfo 是地圖畫面要標的字。
type MapInfo struct {
	// Title 是地名，空的話只印編號。
	Title string

	// HintTitle 與 Hints 是這張地圖的攻略提示（見 ui.Hints）。
	// 右側那一欄只有 130 px 寬，所以提示要折行也要分頁。
	HintTitle string
	Hints     []string
	// HintPage 是目前顯示第幾頁（0 起算）。
	HintPage int
}

// 右側資訊欄。
const (
	infoX      = mapOX + mapSide + 6 // 190
	infoW      = 320 - infoX - 4     // 126
	infoRowH   = 10
	infoTop    = mapOY
	infoBottom = 196
)

// HintIndent 是提示裡「出處那一行」的前綴（一個全形空白）。
// 產生提示行的 `ui` 與畫它的這裡共用同一個常數 —— 兩邊各寫一份時，
// 改了一邊顏色就分不出來，而那不會報錯。
const HintIndent = "　"

// HintRowsPerPage 是右側欄一頁放得下幾行提示。
//
// 由版面算出來：資訊三行加標題之後剩下的高度除以行高。寫成常數是給
// `ui` 端算頁數用的 —— 兩邊各算一套會在最後一頁差一行。
const HintRowsPerPage = (infoBottom-(infoTop+3*12+4)-infoRowH)/infoRowH - 1 // −1 給標題

// HintPages 回傳這一批提示折行之後總共幾頁。分頁的依據是**折行之後的
// 行數**不是提示條數 —— 一條長提示會佔好幾行，照條數分會爆版。
func HintPages(hints []string, perPage int) int {
	if perPage <= 0 {
		return 1
	}
	n := len(wrapHints(hints))
	if n == 0 {
		return 1
	}
	return (n + perPage - 1) / perPage
}

// wrapHints 把每一條提示折成右側欄放得下的寬度。
func wrapHints(hints []string) []string {
	var out []string
	for _, h := range hints {
		out = append(out, wrap(h, infoW-2)...)
	}
	return out
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
	for i, l := range info2 {
		s.DrawText(st, l, infoX*render.Scale, (infoTop+i*12)*render.Scale)
	}

	// 提示接在下面，剩多少高度就放幾行。
	top := infoTop + len(info2)*12 + 4
	perPage := (infoBottom - top - infoRowH) / infoRowH
	if perPage < 1 || len(info.Hints) == 0 {
		s.DrawText(st, "按任意鍵離開", infoX*render.Scale, (infoBottom-8)*render.Scale)
		return
	}
	if info.HintTitle != "" {
		s.DrawText(a.style(egaRGBA(14)), info.HintTitle, infoX*render.Scale, top*render.Scale)
		top += infoRowH
		perPage--
	}
	all := wrapHints(info.Hints)
	pages := (len(all) + perPage - 1) / perPage
	page := info.HintPage
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}
	for i := page * perPage; i < len(all) && i < (page+1)*perPage; i++ {
		y := top + (i-page*perPage)*infoRowH
		col := egaRGBA(15)
		if strings.HasPrefix(all[i], HintIndent) {
			col = egaRGBA(7) // 出處那一行壓暗，提示本文才是重點
		}
		s.DrawText(a.style(col), all[i], infoX*render.Scale, y*render.Scale)
	}
	s.DrawText(st, fmt.Sprintf("↑↓ %d／%d　Esc 離開", page+1, pages),
		infoX*render.Scale, (infoBottom-8)*render.Scale)
}
