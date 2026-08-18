package view

import (
	"image"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// MSX 的戶外第一人稱是**第三條繪圖路徑**（DOS 室內、DOS 野外之外的那一條）。
//
// 與 DOS 野外的差別不在素材而在模型：DOS 每個深度只有「正面／左／右」
// 三個位置，MSX 是**一整排格子** —— 每個深度列舉一段橫向偏移 v，
// 每一格自己決定畫什麼。所以這裡不能沿用 `drawOutdoor` 那套表。
//
// 落點與來源由 `internal/assets/msx` 提供（那張表是 `tools/msxout.go`
// 從反組譯產生的），這個檔只負責「走訪哪些格、疊圖的順序」。
// 機制見 `docs/research/02-other-platforms.md`「MSX 的戶外第一人稱」。

// MSXOutPiece 是一塊已經切好的戶外貼圖與它的落點。
// 視圖 x ＝ DXK·v + DX。
type MSXOutPiece struct {
	Im      *image.Paletted
	DX, DXK int
	DY      int
}

// MSXBandCode 是「這一格是地平線的地形帶」那個碼，與 DOS 同一個值。
const MSXBandCode = 4

// SetMSXOutdoor 掛上 MSX 的戶外：背景整幅，加一個「這一格畫哪幾塊」的查詢。
//
// 傳函式而不是傳整張表，是為了讓 `internal/view` 不必認識 MSX 的素材格式
// —— 切圖與快取留在載入端，這裡只拿得到「圖 ＋ 落點」。
func (t *TownSet) SetMSXOutdoor(bg *image.Paletted, depths [Depth]int,
	pieces func(set, depth, v int) []MSXOutPiece) {
	t.msxBack, t.msxSpan, t.msxPieces = bg, depths, pieces
}

// SetMSXBand 掛上地平線的地形帶。與擋路物分開是因為它在原版就是另一支
// （`sub_E3E`，由碼 4／5 觸發），而且**先畫** —— 它是地平線，擋路物疊在上面。
func (t *TownSet) SetMSXBand(band func(depth, v int) []MSXOutPiece) {
	t.msxBand = band
}

// MSXOutdoor 回報這一套素材畫不畫得出戶外。
func (t *TownSet) MSXOutdoor() bool { return t != nil && t.msxPieces != nil }

// drawMSXOutdoor 畫 MSX 的戶外。回傳 false 表示這一套做不到。
func (t *TownSet) drawMSXOutdoor(s *render.Screen, w *game.World) bool {
	m := w.CurrentMap()
	if m == nil || t.msxPieces == nil {
		return false
	}
	// 背景（天空與地面）整幅不透空鋪上去，原版就是這一步。
	if t.msxBack != nil {
		t.blit(s, t.msxBack, FPX, FPY)
	}
	dx, dy := w.Face.Delta()
	rx, ry := game.Facing((int(w.Face) + 1) & 3).Delta()
	// **由遠到近**：近的蓋住遠的，與其他兩條路徑同一個順序。
	for d := Depth - 1; d >= 0; d-- {
		span := t.msxSpan[d]
		for v := -span; v <= span; v++ {
			x, y := w.X+dx*d+rx*v, w.Y+dy*d+ry*v
			if game.Cell(x, y) < 0 {
				continue
			}
			// 碼 0 什麼都不畫、1–3 是三組擋路物之一、4 是地平線的地形帶。
			code := m.OutdoorCode(x, y)
			if code == MSXBandCode && t.msxBand != nil {
				for _, p := range t.msxBand(d, v) {
					t.blitKey(s, p.Im, FPX+p.DXK*v+p.DX, FPY+p.DY, int(t.clear))
				}
				continue
			}
			if code < 1 || code > 3 {
				continue
			}
			for _, p := range t.msxPieces(code-1, d, v) {
				t.blitKey(s, p.Im, FPX+p.DXK*v+p.DX, FPY+p.DY, int(t.clear))
			}
		}
	}
	return true
}
