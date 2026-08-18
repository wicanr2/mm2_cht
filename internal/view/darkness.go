package view

import "github.com/wicanr2/mm2_cht/internal/render"

// DrawDarkness 畫「看不見」的視圖：整塊留黑，中間一行字。
//
// 原版 root `sub_13FFC` 在照明計數為 0 且這張圖沒有光源（或這一格吃照明）
// 時**整支返回**，第一人稱一筆都不畫；在那之前先清掉視窗 8
// （文字格 (1,1) 起 26×15 ＝ 像素 (8,8)–(216,128)，剛好是視圖那一塊），
// 再把游標移到文字格 (10,8) 印 `Darkness`。
//
// remake 這邊視圖區在 `DrawPhase` 一開始就被 `Clear(0)` 清成黑的，
// 所以只要**不畫**第一人稱就等於原版那次清除；這裡只補那一行字。
//
// **落點照原版的意思而不是照它的像素**：原版那一行從文字格 (10,8) 起，
// 也就是像素 x=80 —— `Darkness` 八個字 × 8 px 剛好讓它在 8–216 這塊裡置中。
// 中文字寬不同，所以這裡直接算置中，垂直維持原版那一列（像素 y=64）。
func DrawDarkness(s *render.Screen, a Assets) {
	text := a.DarkText
	if text == "" {
		text = "Darkness"
	}
	st := a.white()
	w := 0
	for _, r := range text {
		w += st.Advance(r)
	}
	x := (FPX+FPW/2)*render.Scale - w/2
	s.DrawText(st, text, x, darknessRow*render.Scale)
}

// darknessRow 是原版印那一行的像素列：文字格第 8 列 × 8 px。
const darknessRow = 64
