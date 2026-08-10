package view

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 版面座標量自原版截圖的紅色框線（`shots/fpv.png`）。
//
// 橫線落在 y = 3/4、131/132、147/148、187/188；直線落在 x = 2–4、
// 217–221（只在最上面那一區）、314–316。切出來四塊：
//
//	y   5..130   x   5..216   第一人稱視圖（見 firstperson.go 的 FPX/FPY）
//	y   5..130   x 222..313   狀態框（原版放 Protection：照明／魔法／力場）
//	y 133..146   x   5..313   一條橫列（'O' Options、Day、Year、Face）
//	y 149..186   x   5..313   隊伍名單，兩欄各三列
//
// 狀態框裡那三行的來源還沒解，所以先空著；隊伍名單照原版擺在下面。
const (
	// PanelX/PanelW/PanelH 是右上那一格狀態框。
	PanelX = 222
	PanelW = 314 - PanelX // 92
	PanelH = FPH          // 120

	// 隊伍名單那一區。
	rosterY    = 149
	rosterH    = 187 - rosterY // 38
	rosterX    = 5
	rosterW    = 314 - rosterX // 309
	rosterCols = 2
	rosterRows = game.MaxParty / rosterCols // 3
	rosterRowH = rosterH / rosterRows       // 12
	rosterColW = rosterW / rosterCols       // 154
)

// DrawParty 畫出隊伍名單：兩欄各三列，每列是「編號 名字 生命」。
//
// HP 低於三成塗紅、低於一半塗黃 —— 原版就有的視覺提示。
// 名字走原版 8×8 字型；職業是中文，走高解析層（見 DrawPartyText）。
func DrawParty(s *render.Screen, a Assets, p *game.Party) {
	if p == nil || len(p.Members) == 0 {
		return
	}
	for i, c := range p.Members {
		if i >= game.MaxParty {
			break
		}
		x, y := rosterCell(i)
		// 一欄 rosterColW 寬、一個字 8 px：編號兩格、生命值右靠七格，
		// 名字拿中間剩下的。不截就會畫到隔壁欄上 —— 畫超界不會報錯，
		// 只會疊在別人身上。
		const hpCols = 7
		const nameCols = (rosterColW-4)/8 - 2 - hpCols
		name := c.Name
		if len(name) > nameCols {
			name = name[:nameCols]
		}
		s.DrawASCII(a.ASCII, fmt.Sprintf("%d %s", i+1, name), x, y, 15)

		idx := uint8(10) // 綠
		switch {
		case c.MaxHP > 0 && c.HP*10 <= c.MaxHP*3:
			idx = 12 // 紅
		case c.MaxHP > 0 && c.HP*2 <= c.MaxHP:
			idx = 14 // 黃
		}
		hp := fmt.Sprintf("%d/%d", c.HP, c.MaxHP)
		if c.MaxSP > 0 {
			hp = fmt.Sprintf("%d %d", c.HP, c.SP)
		}
		// 生命值靠這一欄的右緣，與原版一樣。
		s.DrawASCII(a.ASCII, hp, x+rosterColW-10-len(hp)*8, y, idx)
	}
}

// rosterCell 是第 i 名隊員那一格的左上角。先排完一欄再換下一欄，
// 與原版一樣（左欄 1、3、5，右欄 2、4、6 是**逐列**排的，所以照列走）。
func rosterCell(i int) (x, y int) {
	col, row := i%rosterCols, i/rosterCols
	return rosterX + col*rosterColW, rosterY + row*rosterRowH
}

// DrawPartyText 是名單那一區的高解析層。
//
// **職業名不放在這裡** —— 一欄只有 154 px，編號、名字、生命值就把它填滿了，
// 再塞兩三個中文字一定疊上去。原版那一列也只有名字與生命值。
// 職業去「檢視」（戰鬥中的 V）與物品選單裡看。
//
// 這支留著是因為譯名接進來之後，中文名字要走高解析層畫在這裡。
func DrawPartyText(*render.Screen, Assets, *game.Party) {}
