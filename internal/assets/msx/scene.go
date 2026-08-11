package msx

import "image"

// 第一人稱視圖的尺寸。原版在 VRAM 的 (0,256) 組好之後整塊搬到畫面的
// (16,40) —— `sub_24FC` 那句 `154×64 → (16,40)` 就是視圖的大小與位置。
const (
	ViewW = 154
	ViewH = 64
)

// Piece 是一塊貼圖：素材表裡的矩形，貼到視圖內的哪裡。
//
// 兩個座標系都不是 VRAM：`SX`／`SY` 已經換算成 `0x2020` 那張 462×128
// 素材表的檔內座標，`DX`／`DY` 是視圖內座標。**remake 不需要 VRAM。**
type Piece struct {
	SX, SY, W, H int
	DX, DY       int
}

// scene 是室內第一人稱的貼圖表，索引沿用 DOS 那一套
// （0-3 正牆、4-7 左側牆、8-11 右側牆），這樣幾何邏輯可以共用。
//
// 值從 `f004` 的貼圖呼叫抽出來（`tools/msxblits.py`），三件事是確定的：
//
//   - **左右**由算術定：成對的目的 x 滿足 `右 = 154 − 寬`，中央的滿足
//     `x = (154 − 寬)/2`。這不是判斷是計算。
//   - **深度**由高度定：透視讓牆高隨距離遞減（62 → 42 → 20 → 11），
//     四階分得很開，沒有模稜兩可的。
//   - 同一格有好幾筆不同寬度的，取**最寬**那筆：窄的是被更近的牆擋住時
//     的裁切版（56 寬的牆被擋成 49 或 21，來源起點跟著往右移）。
//     remake 靠由遠到近的疊畫順序處理遮蔽，用不到裁切版。
//
// 側牆用 `sub_1A40` 那一組（`sub_1C2B` 是同一批牆的另一種材質）。
var scene = map[int]Piece{
	0: {308, 64, 126, 42, 14, 13}, // 正牆 深度0
	1: {182, 64, 98, 41, 28, 13},  // 正牆 深度1
	// 深度 2、3 的正牆還沒定位，見 docs/research/02-other-platforms.md

	// 門是另一種牆（DOS 的槽位 +16），`sub_C48`／`sub_D10`／`sub_DA7`。
	16: {350, 26, 42, 34, 56, 20}, // 正面的門 深度0
	20: {308, 0, 21, 51, 0, 11},   // 左側牆的門 深度0
	21: {350, 0, 21, 26, 35, 26},  // 左側牆的門 深度1
	22: {392, 15, 14, 11, 56, 33}, // 左側牆的門 深度2
	24: {329, 0, 21, 51, 133, 11}, // 右側牆的門 深度0
	25: {371, 0, 21, 26, 98, 26},
	26: {406, 15, 14, 11, 84, 33},

	4: {315, 0, 49, 62, 0, 2},    // 左側牆 深度0
	5: {371, 0, 35, 42, 0, 12},   // 左側牆 深度1
	6: {371, 42, 21, 20, 0, 24},  // 左側牆 深度2
	7: {441, 85, 21, 11, 0, 28},  // 左側牆 深度3
	8: {308, 0, 49, 62, 105, 2},  // 右側牆 深度0
	9: {364, 0, 35, 42, 119, 12}, // 右側牆 深度1
	10: {364, 42, 21, 20, 133, 24},
	11: {434, 85, 21, 11, 133, 28},
}

// background 是天空與地板，一整塊蓋滿視圖（`sub_24FC` 的
// `154×64 從 (0,320)`，換算成素材表內是 (0,64)）。
var background = Piece{0, 64, ViewW, ViewH, 0, 0}

// TorchFrames 是每個火炬位置的影格數，見 flicker。
const TorchFrames = 3

// torch 是火炬，索引照 DOS 的槽位除以四（左 0-2、右 3-5、正面 6-8）。
//
// **原版每個位置只有一張**：整批貼圖裡沒有 DOS 那種「底圖 ＋ 三張火焰」
// 的動畫組，`sub_E3E` 那支查表貼圖畫的是地面條不是火焰。少的是素材
// 不是解碼 —— 動畫影格由 flicker 產生。
var torch = map[int]Piece{
	0: {420, 0, 21, 22, 0, 14},    // 左 深度0
	1: {420, 22, 14, 10, 35, 27},  // 左 深度1
	3: {441, 0, 21, 22, 133, 14},  // 右 深度0
	4: {434, 22, 14, 10, 105, 27}, // 右 深度1
	6: {448, 22, 14, 18, 70, 20},  // 正面 深度0
}

// SceneID 是四套室內場景的素材表 id。
var SceneID = []uint16{0x2020, 0x2021, 0x2022, 0x2023}

// flicker 由一張火炬做出動畫影格。
//
// **原版沒有這個。** 做法是把火焰（上半部）整體左右各位移一個像素，
// 底部的燈座不動；沒有新增任何像素，只是把既有的挪位置。這是 remake
// 自己加的效果，不是還原 —— 標在這裡是因為「看起來像原版」正是最容易
// 被誤當成考證結果的那種東西。
//
// 位移量刻意只有一像素：火炬在視圖裡最大也才 21×22，再多就變成整支
// 火把在晃，那不是火在燒。
func flicker(src *image.Paletted, dx int) *image.Paletted {
	if src == nil || dx == 0 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewPaletted(image.Rect(0, 0, w, h), src.Palette)
	// 火焰佔**有內容的那一段**上面六成，不是整張圖的上面六成 ——
	// 貼圖四周常有整列透空，拿整張的比例去切會切在空白上，
	// 位移出來的三張完全相同，而畫面上只是「火炬不會動」。
	top, bot := h, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if src.ColorIndexAt(b.Min.X+x, b.Min.Y+y) != 0 {
				if y < top {
					top = y
				}
				bot = y
				break
			}
		}
	}
	if bot < top {
		return src
	}
	flame := top + (bot-top+1)*6/10
	for y := 0; y < h; y++ {
		shift := 0
		if y < flame {
			shift = dx
		}
		for x := 0; x < w; x++ {
			sx := x - shift
			if sx < 0 || sx >= w {
				continue // 移出去的位置留透空
			}
			out.SetColorIndex(x, y, src.ColorIndexAt(b.Min.X+sx, b.Min.Y+y))
		}
	}
	return out
}

// Scene 從一張素材表切出第一人稱要用的貼圖。
//
// 回傳的 walls 與 place 同索引，空的格子是 nil —— 呼叫端照 DOS 的槽位
// 取用，取到 nil 就不畫。
func Scene(sheet *image.Paletted) (walls, torches []*image.Paletted,
	place, torchPlace []image.Point, bg *image.Paletted) {
	cut := func(p Piece) *image.Paletted {
		r := image.Rect(p.SX, p.SY, p.SX+p.W, p.SY+p.H)
		if !r.In(sheet.Bounds()) {
			return nil
		}
		// SubImage 會共用底層像素，之後放大時會照著原圖的 Stride 走 ——
		// 這裡要獨立的一張，所以複製。
		out := image.NewPaletted(image.Rect(0, 0, p.W, p.H), sheet.Palette)
		for y := 0; y < p.H; y++ {
			for x := 0; x < p.W; x++ {
				out.SetColorIndex(x, y, sheet.ColorIndexAt(p.SX+x, p.SY+y))
			}
		}
		return out
	}
	walls = make([]*image.Paletted, 32)
	place = make([]image.Point, 32)
	for i, p := range scene {
		walls[i] = cut(p)
		place[i] = image.Pt(p.DX, p.DY)
	}
	// 每個位置佔 TorchFrames 格，與 DOS 的排法一致（一格一組影格），
	// 這樣火炬的相位邏輯不必為 MSX 另寫一套。
	torches = make([]*image.Paletted, 9*TorchFrames)
	torchPlace = make([]image.Point, 9*TorchFrames)
	for i, p := range torch {
		base := cut(p)
		for f, dx := range [TorchFrames]int{0, 1, -1} {
			torches[i*TorchFrames+f] = flicker(base, dx)
		}
		torchPlace[i*TorchFrames] = image.Pt(p.DX, p.DY)
	}
	return walls, torches, place, torchPlace, cut(background)
}
