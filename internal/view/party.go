package view

import (
	"fmt"
	"image/color"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 角色面板在右上，接在第一人稱視圖區右邊 —— 原版就是這個版面。
const (
	PanelX = FPW            // 208
	PanelW = render.OrigW - FPW // 112
	PanelH = FPH            // 120
	rowH   = 19
)

// DrawParty 畫出隊伍面板：每人一列，名字加目前／上限 HP。
//
// HP 低於三成塗紅，這是原版就有的視覺提示；名字用原版 8×8 字型，
// 中文名字要等譯文接進來之後才走高解析層。
func DrawParty(s *render.Screen, a Assets, p *game.Party) {
	if p == nil || len(p.Members) == 0 {
		return
	}
	for i, c := range p.Members {
		if i >= game.MaxParty {
			break
		}
		y := 2 + i*rowH
		if y+rowH > PanelH {
			break
		}
		name := c.Name
		if len(name) > 12 {
			name = name[:12]
		}
		s.DrawASCII(a.ASCII, fmt.Sprintf("%d %s", i+1, name), PanelX+2, y, 15)

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
		s.DrawASCII(a.ASCII, hp, PanelX+8, y+9, idx)
	}
}

// DrawPartyText 補上面板裡的中文（職業名）。
//
// 中文走高解析層，所以要在 Screen.Flush 之後畫 —— Flush 會把原版層放大
// 覆蓋上去，先畫的中文會被洗掉。
func DrawPartyText(s *render.Screen, a Assets, p *game.Party) {
	if p == nil || a.CJK == nil {
		return
	}
	st := render.TextStyle{ASCII: a.ASCII, CJK: a.CJK,
		Color: color.RGBA{0xAA, 0xAA, 0xAA, 0xFF}}
	for i, c := range p.Members {
		if i >= game.MaxParty {
			break
		}
		y := 2 + i*rowH
		if y+rowH > PanelH {
			break
		}
		s.DrawText(st, c.Class.String(), (PanelX+56)*render.Scale, (y+9)*render.Scale)
	}
}
