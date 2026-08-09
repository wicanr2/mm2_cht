// Package view 把遊戲狀態畫成畫面。
//
// 與輸入和視窗無關，所以 Ebiten 主程式與 headless 的 PNG 產生器可以共用
// 同一份繪製程式碼 —— 沒有 GPU 的環境也驗得了畫面。
package view

import (
	"fmt"
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

	statusY = ViewY + ViewH + 2 // 166
	msgY    = 176               // 176 + 3 行 × 8 = 200，剛好貼齊底部
)

// Assets 是畫面需要的素材。
type Assets struct {
	ASCII *font.Font
	CJK   *cjk.Font
}

// Draw 把世界狀態畫成一張畫面。
//
// 3D 第一人稱視角還沒接上（缺「哪一格是牆」的判定，屬性層只解出 bit3），
// 這裡先畫俯視圖：格子用地形層的高 nibble 上色，事件格加框，隊伍畫箭頭。
// 俯視圖不是最終產物，是讓移動與事件觸發能在畫面上驗收的過渡。
func Draw(s *render.Screen, w *game.World, a Assets, msg string) {
	s.Clear(0)
	m := w.CurrentMap()
	if m == nil {
		return
	}

	cell := CellPx
	for c := 0; c < game.MapCells; c++ {
		cx, cy := c%game.MapW, c/game.MapW
		px, py := ViewX+cx*cell, ViewY+cy*cell
		fill(s, px, py, cell-1, cell-1, m.Terrain[c]>>4)
		if m.Attr[c]&game.AttrHasEvent != 0 {
			outline(s, px, py, cell-1, cell-1, 14)
		}
	}
	// 隊伍：實心方塊 + 朝向指示
	px, py := ViewX+w.X*cell, ViewY+w.Y*cell
	fill(s, px+2, py+2, cell-5, cell-5, 15)
	dx, dy := w.Face.Delta()
	fill(s, px+5+dx*4, py+5+dy*4, 3, 3, 12)

	// 狀態列
	s.DrawASCII(a.ASCII, fmt.Sprintf("MAP %02d  X=%2d Y=%2d  FACE=%s",
		w.MapIndex, w.X, w.Y, w.Face), ViewX, statusY, 15)

	s.Flush()

	// 訊息走高解析層，中文才不會被放大成馬賽克
	st := render.TextStyle{ASCII: a.ASCII, CJK: a.CJK,
		Color: color.RGBA{0xFF, 0xFF, 0x55, 0xFF}}
	if msg != "" {
		s.DrawText(st, msg, ViewX*render.Scale, msgY*render.Scale)
	}
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
