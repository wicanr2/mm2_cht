package view

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 框線的位置量自原版截圖（`shots/fpv.png`），**框裡放什麼由我們決定**。
//
//	y   5..130   x   5..216   第一人稱視圖（見 firstperson.go 的 FPX/FPY）
//	y   5..130   x 222..313   隊伍，一人兩行
//	y 133..146   x   5..313   一條橫列（'O' 選項、Day、Year、Face）
//	y 149..186   x   5..313   訊息；沒有訊息時放狀態
//
// 原版把隊伍擠在下面那塊大框的兩欄各三列，右上那格放 `Protection` 三行。
// 兩欄一欄只有 154 px，扣掉編號與生命值，名字剩九格 —— 照抄的結果是
// 「Sir Felgar」被截成「Sir Felg」，中文職業完全放不下。
//
// 右邊那一格有 120 px 的高度，六個人一人 20 px 剛好排得下，一人兩行：
// 上行放編號與完整名字，下行放職業與生命。名字不必截、職業看得到，
// 下面整塊大框也空出來給中文對話。
const (
	// 隊伍面板（右上）。
	PanelX = 222
	PanelY = 6
	PanelW = 314 - PanelX // 92
	PanelH = FPH          // 120

	partyRowH  = PanelH / game.MaxParty // 20
	partyIdxX  = PanelX + 1
	partyNameX = PanelX + 1 + LatinCols*2 // 編號兩格
	partyRight = PanelX + PanelW - 3      // 右靠的基準（框線在 314）
	partyLine2 = 9                        // 第二行相對於這一列的位移
)

// DrawParty 畫隊伍面板：編號、完整名字、職業（或狀況）、生命與法力。
//
// **整塊都走高解析層**，所以要在 Screen.Flush 之後呼叫 —— Flush 會把
// 原版層整片蓋上來，先畫的東西會被洗掉而且不會報錯。
//
// 名字用半形的英數字點陣（一個字 4 個原版像素），十個字 40 px，
// 面板 92 px 綽綽有餘 —— 換掉原版那套 8×8 之後才排得下完整名字。
//
// 生命低於三成塗紅、低於一半塗黃；有法力的人在生命右邊多一格青色法力。
func DrawParty(s *render.Screen, a Assets, p *game.Party) {
	if p == nil {
		return
	}
	for i, c := range p.Members {
		if i >= game.MaxParty {
			break
		}
		y := PanelY + i*partyRowH
		s.DrawText(a.style(egaRGBA(14)), fmt.Sprintf("%d", i+1),
			partyIdxX*render.Scale, y*render.Scale)
		s.DrawText(a.style(egaRGBA(15)), c.Name,
			partyNameX*render.Scale, y*render.Scale)

		// 第二行左邊：職業；狀況不正常時改印狀況，紅色。
		label, col := c.Class.String(), egaRGBA(7) // 亮灰
		if c.Condition != game.CondGood {
			label, col = c.Condition.String(), egaRGBA(12) // 紅
		}
		s.DrawText(a.style(col), label,
			partyIdxX*render.Scale, (y+partyLine2)*render.Scale)

		// 第二行右邊：生命與法力靠右排成一組，兩者用顏色分。
		hpCol := egaRGBA(10) // 綠
		switch {
		case c.MaxHP > 0 && c.HP*10 <= c.MaxHP*3:
			hpCol = egaRGBA(12) // 紅
		case c.MaxHP > 0 && c.HP*2 <= c.MaxHP:
			hpCol = egaRGBA(14) // 黃
		}
		hp, sp := fmt.Sprintf("%d", c.HP), ""
		wide := textCols(hp)
		if c.MaxSP > 0 {
			sp = fmt.Sprintf("%d", c.SP)
			wide += LatinCols + textCols(sp)
		}
		x := partyRight - wide
		s.DrawText(a.style(hpCol), hp, x*render.Scale, (y+partyLine2)*render.Scale)
		if sp != "" {
			s.DrawText(a.style(egaRGBA(11)), sp,
				(x+textCols(hp)+LatinCols)*render.Scale, (y+partyLine2)*render.Scale)
		}
	}
}

