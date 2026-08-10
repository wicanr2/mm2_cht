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
// 下面整塊大框也空出來給中文對話 —— 一行中文佔 8 個原版像素，
// 三行 38 px 的框本來就吃緊，再分一半給名單只會兩邊都不夠。
const (
	// 隊伍面板（右上）。
	PanelX = 222
	PanelY = 6
	PanelW = 314 - PanelX // 92
	PanelH = FPH          // 120

	partyRowH  = PanelH / game.MaxParty // 20
	partyIdxX  = PanelX + 1
	partyNameX = PanelX + 9  // 編號佔一格
	partyRight = PanelX + 89 // 右靠的基準（框線在 314）
	partyLine2 = 9           // 第二行相對於這一列的位移
)

// DrawParty 畫隊伍面板走原版像素層的部分：編號、名字、生命與法力。
//
// 名字用原版 8×8 字型 —— 英文名字配原版字型比較搭，而且十個字剛好是
// 角色記錄裡名字欄的長度（`offName` 10 bytes），排得進 80 px。
// 職業與狀態是中文，走高解析層，見 DrawPartyText。
//
// 生命低於三成塗紅、低於一半塗黃；有法力的人在生命左邊多一格青色法力。
func DrawParty(s *render.Screen, a Assets, p *game.Party) {
	if p == nil {
		return
	}
	for i, c := range p.Members {
		if i >= game.MaxParty {
			break
		}
		y := PanelY + i*partyRowH
		s.DrawASCII(a.ASCII, fmt.Sprintf("%d", i+1), partyIdxX, y, 14)
		s.DrawASCII(a.ASCII, truncASCII(c.Name, 10), partyNameX, y, 15)

		idx := uint8(10) // 綠
		switch {
		case c.MaxHP > 0 && c.HP*10 <= c.MaxHP*3:
			idx = 12 // 紅
		case c.MaxHP > 0 && c.HP*2 <= c.MaxHP:
			idx = 14 // 黃
		}
		// 生命與法力靠右排成一組，生命在左、法力在右，兩者用顏色分。
		hp, sp := fmt.Sprintf("%d", c.HP), ""
		wide := len(hp) * 8
		if c.MaxSP > 0 {
			sp = fmt.Sprintf("%d", c.SP)
			wide += 8 + len(sp)*8
		}
		x := partyRight - wide
		s.DrawASCII(a.ASCII, hp, x, y+partyLine2, idx)
		if sp != "" {
			s.DrawASCII(a.ASCII, sp, x+len(hp)*8+8, y+partyLine2, 11) // 青
		}
	}
}

// truncASCII 把名字切到 n 個位元組。名字是 ASCII（原版記錄就是），
// 所以按位元組切不會切壞字；真的長到要切表示資料有問題，切了才看得出來。
func truncASCII(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// DrawPartyText 畫隊伍面板的中文：職業，或狀況不正常時改印狀況。
//
// 中文走高解析層，所以**必須在 Screen.Flush 之後呼叫** ——
// Flush 會把原版層整片蓋上來，先畫的中文會被洗掉而且不會報錯。
func DrawPartyText(s *render.Screen, a Assets, p *game.Party) {
	if p == nil || a.CJK == nil {
		return
	}
	for i, c := range p.Members {
		if i >= game.MaxParty {
			break
		}
		y := PanelY + i*partyRowH + partyLine2
		label, col := c.Class.String(), egaRGBA(7) // 亮灰
		if c.Condition != game.CondGood {
			label, col = c.Condition.String(), egaRGBA(12) // 紅
		}
		st := render.TextStyle{ASCII: a.ASCII, CJK: a.CJK, Color: col}
		s.DrawText(st, label, partyIdxX*render.Scale, y*render.Scale)
	}
}
