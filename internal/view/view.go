// Package view 把遊戲狀態畫成畫面。
//
// 與輸入和視窗無關，所以 Ebiten 主程式與 headless 的 PNG 產生器可以共用
// 同一份繪製程式碼 —— 沒有 GPU 的環境也驗得了畫面。
package view

import (
	"fmt"
	"strings"
	"image/color"

	"github.com/wicanr2/mm2_cht/internal/assets/cjk"
	"github.com/wicanr2/mm2_cht/internal/assets/font"
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 原版第一人稱視角的版面是左上 3D 視圖 208×120 + 右上狀態面板
// （量自原版截圖，見 docs/playtest/01）。俯視圖是過渡產物，不套那個版面。
//
// 320×200 的畫布裡：地圖 160×160 靠左上，狀態列在它下面，
// 最底下 24 px 留給訊息 —— 中文一行佔 8 個原版像素（高解析 24 px），
// 剛好排得下三行，與原版訊息框的行數一致。
const (
	ViewX, ViewY = 4, 4
	CellPx       = 10
	ViewW        = game.MapW * CellPx
	ViewH        = game.MapH * CellPx

	// 狀態列排在原版那條橫列的位置（y 133..146），訊息在它下面 ——
	// 隊伍名單佔掉 y 149..186（見 party.go）。
	statusY = 135
	msgY    = 189
)

// Assets 是畫面需要的素材。
type Assets struct {
	ASCII *font.Font
	CJK   *cjk.Font
	Town  *TownSet
	Party *game.Party
	// Monsters 是戰鬥中要畫在視圖裡的怪物。
	//
	// **一定要在 Flush 之前畫** —— Flush 把原版像素層整片蓋到高解析層，
	// 之後再往原版層畫的東西不會出現在畫面上，而且不會有任何錯誤。
	Monsters []MonsterSprite
}

// Draw 把世界狀態畫成一張畫面：左上第一人稱視角，右上小地圖，
// 下方狀態列與訊息。
//
// 小地圖不是原版的東西，是驗收用的：走一趟就能看出位置、朝向與牆的判定
// 對不對，不必逐張比對 3D 畫面。
func Draw(s *render.Screen, w *game.World, a Assets, msg string) {
	DrawWith(s, w, a, msg, nil)
}

// DrawWith 與 Draw 相同，但可以額外蓋一個選單。
//
// **兩者要在同一支裡畫完**：`Flush` 是把原版像素層整片蓋到高解析層，
// 分成兩次呼叫就會讓後畫的那一次把前一次的高解析文字洗掉 ——
// 症狀是「訊息不見了」或「選單只剩空框」，而且測試全綠。
func DrawWith(s *render.Screen, w *game.World, a Assets, msg string, menu []string) {
	DrawPhase(s, w, a, msg, menu, 0)
}

// DrawPhase 與 DrawWith 相同，但指定火炬的動畫相位。
func DrawPhase(s *render.Screen, w *game.World, a Assets, msg string, menu []string, phase int) {
	s.Clear(0)
	m := w.CurrentMap()
	if m == nil {
		return
	}

	DrawFirstPersonAt(s, w, a.Town, phase)
	DrawMonsters(s, a.Monsters)
	if a.Party != nil && len(a.Party.Members) > 0 {
		DrawParty(s, a, a.Party)
	} else {
		drawMinimap(s, w, m)
	}

	s.DrawASCII(a.ASCII, fmt.Sprintf("MAP %02d  X=%2d Y=%2d  FACE=%s",
		w.MapIndex, w.X, w.Y, w.Face), ViewX, statusY, 15)

	// 底下那一列只放得下一行；換行過的訊息改成蓋一個視窗在視圖上 ——
	// 原版的 opcode `02` 也是這樣（`loc_16EFD` 開一個 0x26×0x16 的視窗）。
	lines := msgLines(msg)
	window := menu == nil && len(lines) > 1
	if menu != nil && len(lines) > 1 {
		// 選單開著又有長訊息（例如法術說明）：把它接在清單下面，
		// 別丟到底下那一列 —— 那裡只放得下一行，其餘會被畫到畫面外。
		menu = append(append(append([]string{}, menu...), ""), lines...)
		msg, lines = "", nil
	}
	if menu != nil || window {
		drawMenuBox(s)
	}

	s.Flush()

	DrawPartyText(s, a, a.Party)
	if menu != nil {
		drawMenuText(s, a, menu)
	}

	// 訊息走高解析層，中文才不會被放大成馬賽克
	st := render.TextStyle{ASCII: a.ASCII, CJK: a.CJK,
		Color: color.RGBA{0xFF, 0xFF, 0x55, 0xFF}}
	switch {
	case window:
		drawMenuText(s, a, lines)
	case msg != "":
		s.DrawText(st, msg, ViewX*render.Scale, msgY*render.Scale)
	}
}

// msgLines 把訊息拆成行。原版用 `@` 當換行符（見 render.LineBreakRune）。
func msgLines(msg string) []string {
	if msg == "" {
		return nil
	}
	return strings.Split(msg, string(render.LineBreakRune))
}

// drawMinimap 在右上角畫 16×16 的小地圖：牆畫成格線，事件格點亮，
// 隊伍是方塊加朝向。
func drawMinimap(s *render.Screen, w *game.World, m *game.Map) {
	const cell = 7
	ox, oy := PanelX, FPY
	for c := 0; c < game.MapCells; c++ {
		cx, cy := c%game.MapW, c/game.MapW
		px, py := ox+cx*cell/2, oy+cy*cell/2
		if m.Attr[c]&game.AttrHasEvent != 0 {
			fill(s, px+1, py+1, 2, 2, 6)
		}
		if m.HasWall(cx, cy, game.North) {
			fill(s, px, py, cell/2, 1, 8)
		}
		if m.HasWall(cx, cy, game.West) {
			fill(s, px, py, 1, cell/2, 8)
		}
	}
	px, py := ox+w.X*cell/2, oy+w.Y*cell/2
	fill(s, px+1, py+1, 2, 2, 15)
	dx, dy := w.Face.Delta()
	fill(s, px+1+dx*2, py+1+dy*2, 1, 1, 12)
}

func fill(s *render.Screen, x, y, w, h int, idx uint8) {
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			px, py := x+i, y+j
			if px < 0 || px >= render.OrigW || py < 0 || py >= render.OrigH {
				continue
			}
			s.Orig.SetColorIndex(px, py, idx)
		}
	}
}

func outline(s *render.Screen, x, y, w, h int, idx uint8) {
	for i := 0; i < w; i++ {
		fill(s, x+i, y, 1, 1, idx)
		fill(s, x+i, y+h-1, 1, 1, idx)
	}
	for j := 0; j < h; j++ {
		fill(s, x, y+j, 1, 1, idx)
		fill(s, x+w-1, y+j, 1, 1, idx)
	}
}

// NewScreen 建立一張畫布。
func NewScreen() *render.Screen { return render.New(gfx.EGAPalette) }

// 選單蓋在第一人稱視圖上，尺寸就用那一區 —— 先前用小地圖的寬度
// （160 原版像素），中英混排的一行放不下就被截掉，效果那一欄先不見。
func menuBox() (x0, y0, w, h int) {
	const pad = 2
	return FPX + pad, FPY + pad, FPW - pad*2, FPH - pad*2
}

// drawMenuBox 塗底與畫框，走原版像素層。
func drawMenuBox(s *render.Screen) {
	x0, y0, w, h := menuBox()
	for y := y0; y < y0+h && y < render.OrigH; y++ {
		for x := x0; x < x0+w && x < render.OrigW; x++ {
			s.Orig.SetColorIndex(x, y, 0)
		}
	}
	for x := x0; x < x0+w && x < render.OrigW; x++ {
		s.Orig.SetColorIndex(x, y0, 15)
		s.Orig.SetColorIndex(x, y0+h-1, 15)
	}
	for y := y0; y < y0+h && y < render.OrigH; y++ {
		s.Orig.SetColorIndex(x0, y, 15)
		s.Orig.SetColorIndex(x0+w-1, y, 15)
	}
}

// drawMenuText 寫選單的字，走高解析層，所以必須在 Flush 之後。
func drawMenuText(s *render.Screen, a Assets, lines []string) {
	x0, y0, w, h := menuBox()
	st := render.TextStyle{ASCII: a.ASCII, CJK: a.CJK,
		Color: color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}}
	row := 0
	for _, l := range lines {
		for _, seg := range wrap(l, w-8) {
			y := y0 + 3 + row*10
			if y+8 > y0+h {
				return
			}
			s.DrawText(st, seg, (x0+3)*render.Scale, y*render.Scale)
			row++
		}
	}
}

// wrap 依框寬把一行折成幾行。
//
// **不要改成截斷**：法術說明、參考資料那類長句被截掉之後，讀到的人
// 不會知道少了什麼。估寬用原版像素：ASCII 一格 8、中文一格 16。
func wrap(text string, limit int) []string {
	if text == "" {
		return []string{""}
	}
	var out []string
	used, start := 0, 0
	for i, r := range text {
		cw := 8
		if r > 0x7F {
			cw = 16
		}
		if used+cw > limit && i > start {
			out = append(out, text[start:i])
			start, used = i, 0
		}
		used += cw
	}
	return append(out, text[start:])
}
