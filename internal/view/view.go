// Package view 把遊戲狀態畫成畫面。
//
// 與輸入和視窗無關，所以 Ebiten 主程式與 headless 的 PNG 產生器可以共用
// 同一份繪製程式碼 —— 沒有 GPU 的環境也驗得了畫面。
package view

import (
	"image/color"
	"strings"

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
	// Latin 是高解析的英數字點陣（半形）。沒載到就退回原版 8×8 放大，
	// 畫面仍然可讀，只是英文比中文粗一截。
	Latin *cjk.Font
	Town  *TownSet
	Party *game.Party
	// Monsters 是戰鬥中要畫在視圖裡的怪物。
	//
	// **一定要在 Flush 之前畫** —— Flush 把原版像素層整片蓋到高解析層，
	// 之後再往原版層畫的東西不會出現在畫面上，而且不會有任何錯誤。
	Monsters []MonsterSprite
	// Place 是目前地點的名字，沒有訊息時顯示在下方。地名由呼叫端提供
	// （只有五座城鎮查得到名字，見 ui.Session.mapTitle）。
	Place string
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
	DrawFrame(s)
	if menu != nil {
		drawMenuBox(s)
	}

	s.Flush()

	DrawBar(s, w, a)
	// 隊伍面板整塊走高解析層，所以在 Flush 之後畫。
	DrawParty(s, a, a.Party)
	if menu != nil {
		drawMenuText(s, a, menu)
	}
	// 下方那一塊大框整塊給訊息；沒有訊息時才擺狀態。
	lines := msgLines(msg)
	if len(lines) == 0 {
		lines = StatusLines(w, a.Place)
	}
	DrawTextBox(s, a, lines)
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

// style 是這套素材的文字樣式。**所有文字都走這一支** ——
// 少接一個字型欄位的症狀是「那一塊的英文變粗」，沒有錯誤訊息。
func (a Assets) style(c color.RGBA) render.TextStyle {
	return render.TextStyle{ASCII: a.ASCII, CJK: a.CJK, Latin: a.Latin, Color: c}
}

// white 是最常用的白色文字樣式。
func (a Assets) white() render.TextStyle {
	return a.style(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
}

// egaRGBA 把 EGA 調色盤索引換成高解析層要用的顏色，
// 讓兩層的同一個顏色名指到同一個值。
func egaRGBA(idx int) color.RGBA {
	if idx < 0 || idx >= len(gfx.EGAPalette) {
		return color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	}
	r, g, b, a := gfx.EGAPalette[idx].RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
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

// MenuCols 是選單一行放得下幾個字。
//
// 排選單時拿它當上限：超過的行會被折成兩行，看起來就像版面壞掉
// （價格掉到下一行那種）。折行不會報錯，所以要靠這個數字自己守住。
func MenuCols() int {
	_, _, w, _ := menuBox()
	return (w - 8) / LatinCols
}

// drawMenuText 寫選單的字，走高解析層，所以必須在 Flush 之後。
func drawMenuText(s *render.Screen, a Assets, lines []string) {
	x0, y0, w, h := menuBox()
	st := a.white()
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
// 不會知道少了什麼。
//
// 斷行的位置照中英混排的慣例：中文每個字都可以斷，英文只在空白處斷
// （不然 `Luxus` 會變成 `Lu` 加 `xus`），句讀不落在行首（不然
// 「，」「。」會孤零零地開一行）。長到一行放不下的英文字才硬切。
func wrap(text string, limit int) []string {
	if text == "" {
		return []string{""}
	}
	var out []string
	line, used := "", 0
	lastTok, lastW := "", 0
	flush := func() {
		out = append(out, line)
		line, used, lastTok, lastW = "", 0, "", 0
	}
	for _, tok := range wrapTokens(text) {
		w := textCols(tok)
		if tok == " " {
			if used == 0 && len(out) > 0 {
				// 折出來的續行不留行首空白；**原文自己的縮排要留**，
				// 選單那一欄靠它對齊（未選中的項目前面是兩個空白）。
				continue
			}
			if used+w > limit {
				flush()
				continue
			}
			line, used, lastTok, lastW = line+tok, used+w, "", 0
			continue
		}
		if used > 0 && used+w > limit {
			if lastW > 0 && lastW < used && (noLineStart(tok) || noLineEnd(lastTok)) {
				// 句讀不能開頭、開括號不能收尾，把前一個字一起帶到下一行。
				keep := lastTok
				line, used = line[:len(line)-len(keep)], used-lastW
				flush()
				line, used = keep, textCols(keep)
			} else {
				flush()
			}
		}
		// 單一個單位就超過整行寬 —— 只可能是很長的英文字，硬切。
		// 英文一個字元一個位元組，按位元組切不會切壞。
		for w > limit && limit >= LatinCols {
			n := limit / LatinCols
			line, used = tok[:n], n*LatinCols
			flush()
			tok, w = tok[n:], textCols(tok[n:])
		}
		line, used = line+tok, used+w
		lastTok, lastW = tok, w
	}
	if line != "" || len(out) == 0 {
		out = append(out, line)
	}
	return out
}

// wrapTokens 把一行切成可以斷行的單位：一個中文字算一個，
// 一串不含空白的英數字算一個，空白自成一個。
func wrapTokens(text string) []string {
	var out []string
	start := -1
	end := func(i int) {
		if start >= 0 {
			out = append(out, text[start:i])
			start = -1
		}
	}
	for i, r := range text {
		switch {
		case r > 0x7F:
			end(i)
			out = append(out, string(r))
		case r == ' ':
			end(i)
			out = append(out, " ")
		default:
			if start < 0 {
				start = i
			}
		}
	}
	end(len(text))
	return out
}

// noLineStart 是不能排在行首的字（中文的禁則處理，取常用的那一批）。
func noLineStart(tok string) bool {
	return single(tok, "。，、．：；！？」』）〕｝》〉”’─")
}

// noLineEnd 是不能排在行尾的字：開括號後面接的是它要括住的東西，
// 拆在兩行讀起來會斷掉。
func noLineEnd(tok string) bool {
	return single(tok, "「『（〔｛《〈“‘")
}

func single(tok, set string) bool {
	r := []rune(tok)
	return len(r) == 1 && strings.ContainsRune(set, r[0])
}

// 一個字佔幾個原版像素。**漢字全形、拉丁半形**：中文 atlas 是 24 px 寬、
// 英數字 12 px，而 render.Scale 是 3，換算成原版像素就是 8 與 4。
//
// **估算一定要跟著 render.TextStyle.Advance 走。** 兩邊各算一套的話，
// 畫出來的位置與折行／補位算出來的位置會漂移，而漂移的症狀
// （欄位對不齊、行提早斷）看起來像版面沒調好，不像有兩套定義在打架。
const (
	CJKCols   = 8 // 一個漢字
	LatinCols = 4 // 一個英數字
)

// runeCols 是一個字佔幾個原版像素。
func runeCols(r rune) int {
	if r < 0x80 {
		return LatinCols
	}
	return CJKCols
}

// textCols 是一段字畫出來佔幾個原版像素。
func textCols(s string) int {
	n := 0
	for _, r := range s {
		n += runeCols(r)
	}
	return n
}
