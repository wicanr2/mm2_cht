package view

import (
	"fmt"

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
//	隊伍           x 222..313   y   5..130   見 party.go
//	一條橫列       x   5..313   y 133..146   'O' 選項 / Day / Year / Face
//	下方大框       x   5..313   y 149..186   訊息；沒有訊息時放狀態
const (
	frameRed = 4 // EGA 紅

	barY, barH         = 133, 14 // 那條橫列的內容區
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
	st := a.white()
	for i, c := range cols {
		s.DrawText(st, c, barCols[i]*render.Scale, (barY+2)*render.Scale)
	}
}

// StatusLines 是沒有訊息時填進下方大框的那一行。
//
// 原版把這幾個值直立擺在右上角那一格（`Protection`：`Light (N)`、
// `Magic N%`、`Forces N%`）。那一格改放隊伍之後，這裡橫排成一行 ——
// 它們平常都是 0，只有法術開著時才需要看，佔一整格並不划算。
//
// 三個值的來源在 root `sub_147D8`（`0x1486B` 起），逐列印出來：
//
//	第 10 列  ds:03D5  Light    ← 照明術 +1、持續照明術 +20
//	第 11 列  ds:03D6  Magic%   ← 魔法防護術（等級 + 10）
//	第 12 列  ds:03D7  Forces%  ← 拒絕傷害（等級 + 20）
//	第 13 列  ds:03D8 非零才印  ← 飄浮術
//
// 每個值前面的 `cmp 0Ah` / `cmp 64h` 是**欄位寬度**（值到十位、百位各左移
// 一格把數字靠右對齊），不是門檻。第四行原版只在飄浮術開著時才出現，
// 這裡照做。
func StatusLines(w *game.World, place string) []string {
	if w == nil {
		return nil
	}
	if place == "" {
		place = fmt.Sprintf("地圖 %d", w.MapIndex)
	}
	line := fmt.Sprintf("照明 %d　魔法 %d%%　力場 %d%%",
		int(w.Globals[globalLight]),
		int(w.Globals[globalMagicProt]),
		int(w.Globals[globalForceProt]))
	if w.Globals[globalLevitate] != 0 {
		line += "　飄浮"
	}
	return []string{
		line,
		fmt.Sprintf("%s　X %d　Y %d", place, w.X, w.Y),
	}
}

// 這四個是 root `sub_147D8` 逐列印的那幾格，寫入端見 docs/formats/09 §計數型。
const (
	globalLight     = 0x03D5
	globalMagicProt = 0x03D6
	globalForceProt = 0x03D7
	globalLevitate  = 0x03D8
)

// DrawTextBox 把訊息畫進下方那一塊大框，最多三行。
//
// 有訊息就顯示訊息、沒有才顯示隊伍名單 —— 原版就是這樣兩用的。
func DrawTextBox(s *render.Screen, a Assets, lines []string) {
	st := a.white()
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
