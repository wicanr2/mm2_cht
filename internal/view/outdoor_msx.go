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

// MSXBandCode 是 DOS 那套碼裡的「地平線地形帶」。**只有在讀不到 MSX
// 自己的地圖資料時才用得到** —— 兩套碼不是同一件事，見 SetMSXCells。
const MSXBandCode = 4

// 一格要畫什麼（`SetMSXCells` 的回傳）。
const (
	MSXCellNone = iota
	// MSXCellFeature：畫擋路物，arg 是第幾組（0–2）。
	MSXCellFeature
	// MSXCellBand：畫地平線的地形帶，arg 是變體。
	MSXCellBand
)

// SetMSXOutdoor 掛上 MSX 的戶外：背景，加一個「這一格畫哪幾塊」的查詢。
//
// 傳函式而不是傳整張表，是為了讓 `internal/view` 不必認識 MSX 的素材格式
// —— 切圖與快取留在載入端，這裡只拿得到「圖 ＋ 落點」。
//
// **兩個查詢都吃地圖號**：擋路物 A 與地平線的地形帶各有兩張素材，
// 背景的地面那一半在地圖 41–44 還會被換掉，三者都是換圖時決定的。
func (t *TownSet) SetMSXOutdoor(bg func(mapIdx int) *image.Paletted, depths [Depth]int,
	pieces func(mapIdx, set, depth, v int) []MSXOutPiece) {
	t.msxBack, t.msxSpan, t.msxPieces = bg, depths, pieces
}

// SetMSXBand 掛上地平線的地形帶。與擋路物分開是因為它在原版就是另一支
// （`sub_E3E`，由碼 4／5 觸發），而且**先畫** —— 它是地平線，擋路物疊在上面。
func (t *TownSet) SetMSXBand(band func(mapIdx, variant, depth, v int) []MSXOutPiece) {
	t.msxBand = band
}

// SetMSXCells 掛上「這一格畫什麼」的查詢，資料來自**MSX 自己的地圖檔**。
//
// 為什麼不沿用 DOS 的碼：兩套碼長得像但不是同一件事。地圖 11 的 256 格
// 有 255 格相同，看起來可以共用；地圖 5 只剩 50 格，地圖 41–44 一格都不同
// （DOS 全是 4、MSX 全是 1）。差異來自每張地圖自己的分派表 —— 同一個
// MSX 碼在不同地圖指向不同的常式，所以碼本身沒有跨地圖的意義。
//
// 回 ok = false（讀不到那張圖的資料）時退回 DOS 的碼，畫得出東西，
// 但地圖 41–44 那種「原版什麼都不畫、只露地面」的情形會多畫一層帶。
func (t *TownSet) SetMSXCells(f func(mapIdx, x, y int) (kind, arg int, ok bool)) {
	t.msxCell = f
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
		if bg := t.msxBack(w.MapIndex); bg != nil {
			t.blit(s, bg, FPX, FPY)
		}
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
			kind, arg := t.msxCellAt(m, w.MapIndex, x, y)
			switch kind {
			case MSXCellBand:
				if t.msxBand == nil {
					continue
				}
				for _, p := range t.msxBand(w.MapIndex, arg, d, v) {
					t.blitKey(s, p.Im, FPX+p.DXK*v+p.DX, FPY+p.DY, int(t.clear))
				}
			case MSXCellFeature:
				for _, p := range t.msxPieces(w.MapIndex, arg, d, v) {
					t.blitKey(s, p.Im, FPX+p.DXK*v+p.DX, FPY+p.DY, int(t.clear))
				}
			}
		}
	}
	return true
}

// msxCellAt 問「這一格畫什麼」，讀不到 MSX 的地圖資料就退回 DOS 的碼。
func (t *TownSet) msxCellAt(m *game.Map, mapIdx, x, y int) (kind, arg int) {
	if t.msxCell != nil {
		if k, a, ok := t.msxCell(mapIdx, x, y); ok {
			return k, a
		}
	}
	switch code := m.OutdoorCode(x, y); {
	case code == MSXBandCode:
		return MSXCellBand, 0
	case code >= 1 && code <= 3:
		return MSXCellFeature, code - 1
	}
	return MSXCellNone, 0
}
