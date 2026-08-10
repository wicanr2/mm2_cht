package view

import (
	"fmt"
	"image/color"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 原版的畫面是四個紅框圍出來的（量自 `shots/fpv.png`）：
//
//	橫線 y = 3/4、131/132、147/148、187/188
//	直線 x = 2–4（左）、217–221（只在最上面那一區）、314–316（右）
//	下方橫列的分隔線 x = 98–100、170–172、250–252
//
// 切出來的四塊：
//
//	第一人稱視圖   x   5..216   y   5..130
//	狀態框         x 222..313   y   5..130
//	一條橫列       x   5..313   y 133..146   'O' Options / Day / Year / Face
//	下方大框       x   5..313   y 149..186   隊伍名單**或**當前訊息
//
// 最後那一塊兩用是原版的行為：`shots/fpv.png` 放的是隊伍名單，
// `shots/c2.png`（神殿門口）放的是對話。所以訊息不必另外找地方擺。
const (
	frameRed = 4 // EGA 紅

	barY, barH       = 133, 14  // 那條橫列的內容區
	textBoxY, textBoxH = 149, 38 // 下方大框的內容區
	textBoxX, textBoxW = 5, 309
	// 下方大框一行 12 px，剛好三行 —— 與原版的三行對話一致。
	textRowH = 12
)

// barCols 是橫列四格的左緣（分隔線量出來的）。
var barCols = [4]int{8, 104, 176, 258}

// DrawFrame 畫出原版的紅框。
func DrawFrame(s *render.Screen) {
	// 四條橫線
	for _, y := range []int{3, 131, 147, 187} {
		fill(s, 2, y, 315, 2, frameRed)
	}
	// 左右兩條直線
	fill(s, 2, 3, 3, 186, frameRed)
	fill(s, 314, 3, 3, 186, frameRed)
	// 上面那一區中間的分隔
	fill(s, 217, 3, 5, 130, frameRed)
	// 橫列的三條分隔
	for _, x := range []int{98, 170, 250} {
		fill(s, x, 131, 3, 18, frameRed)
	}
	// 狀態框中間那一條（原版 y 75/76）
	fill(s, 220, 75, 96, 2, frameRed)
}

// DrawBar 畫那條橫列：指令提示、日期、年份、朝向。
//
// 原版是 `'O' Options`、`Day= 13`、`Year= 900`、`Face= N`。
// 日期與年份取自全域計數器（`World.Today`）。
func DrawBar(s *render.Screen, w *game.World, a Assets) {
	face := "N"
	if w != nil {
		face = w.Face.String()
	}
	day, year := 0, 0
	if w != nil {
		day, year = int(w.Today()), w.Year()
	}
	cols := [4]string{
		"'O' 選項",
		fmt.Sprintf("Day= %d", day),
		fmt.Sprintf("Year= %d", year),
		fmt.Sprintf("Face= %s", face),
	}
	st := render.TextStyle{ASCII: a.ASCII, CJK: a.CJK,
		Color: color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}}
	for i, c := range cols {
		s.DrawText(st, c, barCols[i]*render.Scale, (barY+2)*render.Scale)
	}
}

// DrawStatus 畫右上那一格狀態框。
//
// 原版放的是 `Protection`：標題、一塊方格底紋，底下三行
// `Light (N)`、`Magic N%`、`Forces N%`。
//
// 照明是全域計數器 `ds:03D5`（照明術 +1、持續照明術 +20，見
// docs/formats/09 §計數型）。**魔法與力場那兩行的來源還沒解** ——
// 原版一開始顯示 0%，這裡也顯示 0，不編一個看起來合理的數字。
func DrawStatus(s *render.Screen, w *game.World, a Assets) {
	st := render.TextStyle{ASCII: a.ASCII, CJK: a.CJK,
		Color: color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}}
	s.DrawText(st, "防護", (statusX+26)*render.Scale, (statusBoxY+2)*render.Scale)

	light := 0
	if w != nil {
		light = int(w.Globals[globalLight])
	}
	rows := []string{
		fmt.Sprintf("照明 (%d)", light),
		"魔法 0%",
		"力場 0%",
	}
	for i, r := range rows {
		s.DrawText(st, r, (statusX+2)*render.Scale,
			(statusRowY+i*12)*render.Scale)
	}
}

const (
	statusX    = 222
	statusBoxY = 5
	// 原版在 y 75/76 有一條把這一格切兩半的橫線，三行資訊在下半部。
	statusRowY  = 80
	globalLight = 0x03D5
)

// DrawTextBox 把訊息畫進下方那一塊大框，最多三行。
//
// 有訊息就顯示訊息、沒有才顯示隊伍名單 —— 原版就是這樣兩用的。
func DrawTextBox(s *render.Screen, a Assets, lines []string) {
	st := render.TextStyle{ASCII: a.ASCII, CJK: a.CJK,
		Color: color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}}
	row := 0
	for _, l := range lines {
		for _, seg := range wrap(l, textBoxW-6) {
			if row*textRowH+10 > textBoxH {
				return
			}
			s.DrawText(st, seg, (textBoxX+3)*render.Scale,
				(textBoxY+row*textRowH)*render.Scale)
			row++
		}
	}
}
