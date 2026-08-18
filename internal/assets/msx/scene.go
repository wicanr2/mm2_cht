package msx

import (
	"image"
	"sort"
)

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
//
// 原版的貼圖參數是 VRAM 絕對座標，而 VRAM 上同一片座標由兩組素材輪流
// 佔用（室內是這張 462 寬的表，戶外是好幾個檔案的鑲嵌）。換算時**要先
// 決定是哪一組**，否則超出這張表的就會被誤判成「素材不存在」。
type Piece struct {
	SX, SY, W, H int
	DX, DY       int
}

// scene 是室內第一人稱的貼圖表，索引沿用 DOS 那一套
// （0-3 正牆、4-7 左側牆、8-11 右側牆、+16 是門），幾何邏輯因此共用。
//
// **值全部抄自室內那條渲染路徑**：`sub_24FC` 是室內的視圖驅動，它呼叫
// `sub_3B83`（正牆 `sub_1E16` ＋ 門 `sub_C48`／`sub_D10`／`sub_DA7`）與
// `sub_4752`（側牆 `sub_1F4D`／`sub_1FF9` ＋ 火炬）。另外那條
// `sub_2B0A` → `sub_2A57` → `sub_1103`／`sub_1A40`／`sub_1C2B` 是**戶外**的，
// 它讀的是戶外那組素材（`0x2044`、`0x2042`／`0x2043`），拿室內這張表去切
// 會切到門與岩塊 —— 症狀是整條走廊變成一排門，而畫面照樣「像有東西」。
//
// 抽參數的工具**兩個入口都要掃**：`0x685D` 是透空搬移、`0x6857` 是不透空。
// 正牆整條牆帶走的是不透空那個，只掃 `685D` 會完全看不到它
// （`tools/msxblits.py` 的 `--vram` 與 `entry=` 參數）。
//
// 三件事是算出來的不是猜的：
//
//   - **左右**成對的目的 x 滿足 `右 = 154 − 寬 − 左`
//   - **深度**由落點遞進定：側牆是 0 → 28 → 56（與 DOS 的 `sideX`
//     0/24/56 同一個形狀），正牆是整條牆帶逐階變窄變矮
//   - 置中的滿足 `x = (154 − 寬)/2`
var scene = map[int][]Piece{
	// 正牆是 `sub_1E16` 畫的**不透空岩帶**，一階三塊拼滿視圖寬度。
	// 原版分三階（深度 0/1/2），沒有第四階。
	0: {
		{154, 64, 28, 41, 0, 13},
		{182, 64, 98, 41, 28, 13},
		{260, 64, 28, 41, 126, 13},
	},
	1: {
		{154, 105, 14, 18, 0, 27},
		{210, 105, 42, 18, 56, 27}, // 來源與落點都是算式，n = 0 是置中
		{284, 105, 14, 18, 130, 27},
	},
	2: {{224, 33, 14, 8, 70, 33}}, // `14n + 70`，n = 0 ＝ (154−14)/2

	// 門（DOS 的槽位 +16）。深度 1、2 的目的 x 是算式（`42n + 63`、
	// `14n + 70`），`n = 0` 正好等於置中。**深度 3 沒有貼圖。**
	16: {{350, 26, 42, 34, 56, 20}}, // 正面的門 深度0
	17: {{392, 0, 28, 15, 63, 30}},  // 正面的門 深度1
	18: {{392, 26, 14, 5, 70, 35}},  // 正面的門 深度2
	20: {{308, 0, 21, 51, 0, 11}},   // 左側牆的門 深度0
	21: {{350, 0, 21, 26, 35, 26}},  // 左側牆的門 深度1
	22: {{392, 15, 14, 11, 56, 33}}, // 左側牆的門 深度2
	24: {{329, 0, 21, 51, 133, 11}}, // 右側牆的門 深度0
	25: {{371, 0, 21, 26, 98, 26}},
	26: {{406, 15, 14, 11, 84, 33}},

	// 側牆是 `sub_1F4D`（左）／`sub_1FF9`（右）畫的楔形，三個深度，
	// 落點 0 → 28 → 56 與鏡射的 126 → 98 → 84。**深度 3 沒有貼圖。**
	4:  {{154, 0, 28, 64, 0, 0}},    // 左側牆 深度0
	5:  {{182, 13, 28, 41, 28, 13}}, // 左側牆 深度1
	6:  {{210, 27, 14, 18, 56, 27}}, // 左側牆 深度2
	8:  {{280, 0, 28, 64, 126, 0}},  // 右側牆 深度0
	9:  {{252, 13, 28, 41, 98, 13}},
	10: {{238, 27, 14, 18, 84, 27}},
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
// 深度 2 那三個與正面深度 1 的**目的 x 是算出來的**，不是立即值：
// `sub_1956` 是 `14n + 56`、`sub_19CB` 是 `14n + 84`、`sub_18AD` 深度 1 是
// `42n + 70`、深度 2 是 `14n + 70`（`n` ＝ 橫向索引）。正前方那一格 `n = 0`
// 就是下面填的值，而左右仍然滿足 `右 = 154 − 寬 − 左`（56 ↔ 84）。
// 掃立即值的工具在這幾筆會抓到 `SY` 當成目的 x（那是最後一個 `ld hl, imm`），
// **看起來像「這個位置在 288」而不像「抓錯了」** —— 要回去讀呼叫點。
var torch = map[int]Piece{
	0: {420, 0, 21, 22, 0, 14},    // 左 深度0
	1: {420, 22, 14, 10, 35, 27},  // 左 深度1
	2: {420, 32, 14, 5, 56, 33},   // 左 深度2
	3: {441, 0, 21, 22, 133, 14},  // 右 深度0
	4: {434, 22, 14, 10, 105, 27}, // 右 深度1
	5: {434, 32, 14, 5, 84, 33},   // 右 深度2
	6: {448, 22, 14, 18, 70, 20},  // 正面 深度0
	7: {434, 37, 14, 9, 70, 29},   // 正面 深度1
	8: {420, 37, 14, 4, 70, 35},   // 正面 深度2
}

// SceneID 是四套室內場景的素材表 id。
var SceneID = []uint16{0x2020, 0x2021, 0x2022, 0x2023}

// SceneScene 是「DOS 場景碼 → MSX 素材表」。
//
// MSX 挑表的條件寫死在 `f002` 的 `sub_31DA`：地圖 < 5 載 `0x2020`、
// 17–32 載 `0x2021`、45–54 載 `0x2022`、55–59 載 `0x2023`，其餘區間
// （5–16、33–40、41–44 共 24 張）走戶外那組鑲嵌。**那四段區間與 DOS 的
// 場景碼 0／1／5／2 逐段相同**（`_2play_e11` 的區間表，見 `polish-spec` P6），
// 所以兩個平台共用 `World.Scene()`，不必為 MSX 另寫一套區間。
//
// 場景 0 是主表（`SceneID[0]`），不列在這裡。
var SceneScene = map[int]uint16{
	1: 0x2021, // 地圖 17–32
	5: 0x2022, // 地圖 45–54
	2: 0x2023, // 地圖 55–59
}

// WallSlots 是 MSX 這一套**實際定位得出來**的牆面槽位，由 `scene` 直接算出。
//
// MSX 的槽位天生是稀疏的（見上面各筆註解與 `docs/todo.md` A8），所以
// 「32 格全滿」那條 DOS 契約套不上去。用它做完整性檢查的人是
// `internal/ui` 的 `requireTownSet` —— 兩邊各寫一份的話，這裡補上一個
// 深度、那裡忘了改，症狀會是**整套 MSX 素材安靜地從 `F6` 循環裡消失**，
// 而那與「玩家沒有 MSX 磁片」長得一模一樣。踩過一次。
var WallSlots = sortedKeys(scene)

// TorchSlots 是有圖的火炬**影格**索引（位置 × TorchFrames）。
// 九個位置全部定位得出來（2026-08-17 補上深度 2 那三個與正面深度 1）。
var TorchSlots = func() []int {
	out := make([]int, 0, len(torch)*TorchFrames)
	for _, i := range sortedKeys(torch) {
		for f := 0; f < TorchFrames; f++ {
			out = append(out, i*TorchFrames+f)
		}
	}
	return out
}()

func sortedKeys[V any](m map[int]V) []int {
	out := make([]int, 0, len(m))
	for i := range m {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

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
// Cut 從素材表切下一塊。超出表外回 nil ——「切不到」與「切到一片空白」
// 要分得出來，後者在畫面上看起來像素材本來就是空的。
func Cut(sheet *image.Paletted, sx, sy, w, h int) *image.Paletted {
	if sheet == nil {
		return nil
	}
	if !image.Rect(sx, sy, sx+w, sy+h).In(sheet.Bounds()) {
		return nil
	}
	// SubImage 會共用底層像素，之後放大時會照著原圖的 Stride 走 ——
	// 這裡要獨立的一張，所以複製。
	out := image.NewPaletted(image.Rect(0, 0, w, h), sheet.Palette)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.SetColorIndex(x, y, sheet.ColorIndexAt(sx+x, sy+y))
		}
	}
	return out
}

// Overlay 把 src 蓋在 dst 的 (x,y) 上，回傳新的一張（**不改 dst**）。
//
// 用途是地圖 41–44 的地面：原版把那張 154×28 載到 VRAM 的 (0,356)，
// 也就是背景那張 154×64 的下緣 28 列上，之後整幅拷走 —— 效果等同於
// 「同一張背景換掉地面那一半」。整塊覆蓋，不透空。
func Overlay(dst, src *image.Paletted, x, y int) *image.Paletted {
	if dst == nil {
		return nil
	}
	b := dst.Bounds()
	out := image.NewPaletted(image.Rect(0, 0, b.Dx(), b.Dy()), dst.Palette)
	for py := 0; py < b.Dy(); py++ {
		for px := 0; px < b.Dx(); px++ {
			out.SetColorIndex(px, py, dst.ColorIndexAt(b.Min.X+px, b.Min.Y+py))
		}
	}
	if src == nil {
		return out
	}
	sb := src.Bounds()
	for py := 0; py < sb.Dy(); py++ {
		dy := y + py
		if dy < 0 || dy >= b.Dy() {
			continue
		}
		for px := 0; px < sb.Dx(); px++ {
			dx := x + px
			if dx < 0 || dx >= b.Dx() {
				continue
			}
			out.SetColorIndex(dx, dy, src.ColorIndexAt(sb.Min.X+px, sb.Min.Y+py))
		}
	}
	return out
}

func Scene(sheet *image.Paletted) (walls, torches []*image.Paletted,
	place, torchPlace []image.Point, bg *image.Paletted) {
	cut := func(p Piece) *image.Paletted {
		return Cut(sheet, p.SX, p.SY, p.W, p.H)
	}
	// 一個槽位可能是好幾塊拼出來的（正牆那條岩帶是左中右三塊），
	// 拼好之後對外仍然是「一張圖 ＋ 一個落點」，呼叫端不必知道。
	// 取不到來源的那一塊直接讓整個槽位作廢 —— 半條牆比沒有牆更難看，
	// 而且「少一塊」在畫面上看起來像牆上破了個洞，不像素材缺了。
	join := func(ps []Piece) (*image.Paletted, image.Point) {
		var box image.Rectangle
		tiles := make([]*image.Paletted, len(ps))
		for i, p := range ps {
			if tiles[i] = cut(p); tiles[i] == nil {
				return nil, image.Point{}
			}
			r := image.Rect(p.DX, p.DY, p.DX+p.W, p.DY+p.H)
			if i == 0 {
				box = r
			} else {
				box = box.Union(r)
			}
		}
		if len(ps) == 1 {
			return tiles[0], box.Min
		}
		out := image.NewPaletted(image.Rect(0, 0, box.Dx(), box.Dy()), sheet.Palette)
		for i, p := range ps {
			for y := 0; y < p.H; y++ {
				for x := 0; x < p.W; x++ {
					out.SetColorIndex(p.DX-box.Min.X+x, p.DY-box.Min.Y+y,
						tiles[i].ColorIndexAt(x, y))
				}
			}
		}
		return out, box.Min
	}
	walls = make([]*image.Paletted, 32)
	place = make([]image.Point, 32)
	for i, ps := range scene {
		walls[i], place[i] = join(ps)
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
