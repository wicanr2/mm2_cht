package view

import (
	"image"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 第一人稱視圖區的位置與大小。
//
// 原點由 `cmd/mm2match` 把 `TOWN.16` 每一張影格滑過原版截圖定出來：
// 深度 0 的左右側牆（影格 4／8，24×120）落在 (8,8) 與 (192,8)，
// 兩張都是 **100% 逐像素相符**，而 208 寬的地板貼在視圖底邊 ——
// 三個條件同時成立於 (8,8,208,120)。
const (
	FPX, FPY = 8, 8
	FPW, FPH = 208, 120
	// Depth 是往前看幾格。素材只有四個深度的牆面。
	Depth = 4
)

// 牆面的落點是**表**，不是算出來的。
//
// 四個深度各一組，值全部是畫面座標（含視圖原點 8），抄自 DGROUP：
// 正牆 x `ds:153E`／y `ds:1546`，左側牆 x `ds:1552`／y `ds:155A`，
// 右側牆 x `ds:1576`／y `ds:157E`。`internal/view` 的測試會拿 `MM2.EXE`
// 逐格對這幾張表。
//
// **不要改回「水平置中、垂直置中」。** 那個公式在八個落點裡對六個，
// 錯的兩個各差 1 px：側牆深度 1（表 22、置中 21）與正牆深度 3
// （表 62、置中 63）。1 px 的垂直位移落在石牆這種高頻紋理上，
// 整塊 32×94 幾乎每個像素都不同 —— 畫面看起來完全正常，
// `cmd/mm2diff` 卻量到 9.3%，而且差異集中在「那面牆」，
// 看起來像挑錯影格，不像差一個像素。
var (
	frontX = [Depth]int{32, 64, 88, 104}
	frontY = [Depth]int{22, 40, 54, 62}

	sideLeftX  = [Depth]int{8, 32, 64, 88}
	sideRightX = [Depth]int{192, 160, 136, 120}
	sideY      = [Depth]int{8, 22, 40, 54}
)

// IndoorPiece 是室內第一人稱一筆貼圖的影格與落點（畫面座標）。
type IndoorPiece struct {
	What  string
	Frame int
	X, Y  int
}

// IndoorGeometry 把室內四個深度的正牆、左右側牆、左右縱列牆全部列出來。
//
// 匯出它只有一個用途：讓測試拿真的素材尺寸去驗那幾張表 ——
// 表是手抄 DGROUP 來的，抄錯一格畫面照樣「像對的」，只有逐像素比得出來，
// 而逐像素要跑 DOSBox。這條讓大部分的抄錯在單元測試就攔下來。
func IndoorGeometry() []IndoorPiece {
	var out []IndoorPiece
	for d := 0; d < Depth; d++ {
		out = append(out,
			IndoorPiece{"正牆", d, frontX[d], frontY[d]},
			IndoorPiece{"左側牆", 4 + d, sideLeftX[d], sideY[d]},
			IndoorPiece{"右側牆", 8 + d, sideRightX[d], sideY[d]},
			IndoorPiece{"左縱列牆", colLeftFrame[d], colLeftX[d], colY[d]},
			IndoorPiece{"右縱列牆", colRightFrame[d], colRightX[d], colY[d]},
		)
	}
	return out
}

// Platform 是素材來自哪一個原版平台。
//
// 幾何、遮蔽、火炬相位這些邏輯**完全共用** —— 三個平台的牆面素材張數與
// 排列一一對應（`TOWN.16` 與 `town.32` 都是 32 張、同樣的深度與側牆順序），
// 所以換平台只是換素材來源，不動任何規則。
type Platform int

const (
	// PlatformDOS 是原版 EGA 16 色，走原版像素層。
	PlatformDOS Platform = iota
	// PlatformAmiga 是 32 色的 5 位元平面素材（1989，`.32`）。
	PlatformAmiga
	// PlatformMSX 是 MSX2 版（Starcraft 1989 日版）。
	//
	// 視圖只有 154×64（DOS 是 208×120），所以**先合成到 154×64 的離屏
	// 畫布，再整幅縮放填滿視圖框**；每張牆的落點另有一張表，算不出來
	// —— 兩邊的透視不是同一套。
	PlatformMSX
	// PlatformModern 是烘好的高解析素材包（`assets/modern`，見 cmd/mm2modern）。
	//
	// 與「原版素材 ＋ Scale3x」的差別是**它是檔案**：可以被換成重畫的美術，
	// 而 Scale3x 的上限就是原版像素的資訊量。預設內容就是烘好的 Scale3x，
	// 所以沒有人換圖之前兩者看起來一樣。
	PlatformModern
	// PlatformMegaDrive 是 Mega Drive 版（1991）。
	//
	// 視圖同樣是 208×120（與 DOS 逐格相同，見 `docs/research/02`），
	// 但**牆面的切法不一樣**：DOS 每個深度各一張左右側牆，Mega Drive 把
	// 一整根側牆柱烘成一張 120 高的圖，所以落點要另給一張表。
	// 火炬不是獨立素材，是畫在牆面素材裡的。
	PlatformMegaDrive
)

// String 是顯示用的名字。
func (p Platform) String() string {
	switch p {
	case PlatformAmiga:
		return "Amiga"
	case PlatformMSX:
		return "MSX"
	case PlatformMegaDrive:
		return "Mega Drive"
	case PlatformModern:
		return "現代"
	}
	return "DOS"
}

// Style 是場景素材的呈現方式。
//
// 兩種都畫**同一批原版素材**，差別只在放大的方式，所以幾何、遮蔽、
// 火炬相位這些邏輯共用一套程式碼 —— 換風格不會換出不同的畫面內容。
type Style int

const (
	// StyleClassic 走原版路徑：畫進 320×200 的原版層，整數倍 nearest 放大。
	StyleClassic Style = iota
	// StyleModern 用 Scale3x 補斜角之後直接畫進高解析層。
	//
	// 動機是中文層：中文走 24×24 點陣，牆面走 8×8 放大三倍，兩者的
	// 像素密度差三倍，並排時牆面明顯較粗。Scale3x 讓牆的邊界跟上中文的
	// 解析度，而**不引入新顏色**，所以仍然是原版的配色。
	StyleModern
)

// TownSet 是一個平台的第一人稱素材。
//
// 影像在載入時就展開成索引色（各平台帶自己的調色盤），所以繪圖路徑
// 不必知道來源是 4bpp packed 還是 5 個位元平面。
type TownSet struct {
	Walls []*image.Paletted // 0-3 正牆、4-7 左側牆、8-11 右側牆、12-15 補牆
	Floor []*image.Paletted // 208×60 的地板
	Torch []*image.Paletted // 火炬動畫，見 torchSlot
	Sky   []*image.Paletted // 兩張 208×60，見 drawSky

	// Platform 與 Style 都可以隨時改，快取以來源指標為鍵，不必跟著重建。
	Platform Platform
	Style    Style

	// clear 是這一批素材的透空色。**各平台不同**：DOS 是 8、Amiga 是 0。
	clear uint8
	// flames 是一組火炬有幾個動畫相位。DOS 是 3（底圖之外那三張），
	// 其他平台等於 torchStride（沒有底圖）：MSX 1、Mega Drive 9。
	flames int

	// torchStride 是火炬每一格佔幾張圖。DOS 是 4（含燈桿底圖），
	// Amiga 是 3（沒有底圖，每個深度只有三張火焰）。
	torchStride int
	// preScaled 表示素材本身已經是放大好的（素材包）。再放大一次會
	// 得到「位置對、大小錯」的畫面 —— 那看起來像座標算錯，不像素材問題。
	preScaled bool

	// place 是每張素材的固定落點（視圖內座標）。有表就用表，取代
	// 置中與 sideX 的計算 —— 各平台的透視不同，算不出別人的位置。
	place []image.Point
	// torchPlace 與 Torch 同索引，非空時取代 torchSlot 的座標。
	torchPlace []image.Point

	// front 是正牆三階各用哪一組火炬影格。預設抄 torchFront，
	// 素材包可以用 SetFrontTorchGroup 覆蓋。
	front [Depth - 1]torchSlot
	// origin 是這一套素材的視圖原點（視圖比 DOS 小的時候用來置中）。
	origin image.Point
	// native 非零時表示這一套素材有**自己的視圖解析度**（MSX 是 154×64）。
	// 那種素材先合成到 canvas 再整幅縮放填滿視圖框，見 fitCanvas。
	native image.Point
	// canvas 是合成用的離屏畫布，非 nil 時所有貼圖改畫到它上面。
	canvas *image.Paletted

	// MSX 的戶外那條路徑（見 outdoor_msx.go）。
	msxBack   *image.Paletted
	msxSpan   [Depth]int
	msxPieces func(set, depth, v int) []MSXOutPiece
	msxBand   func(depth, v int) []MSXOutPiece

	// stencils 是每張素材自己的 1-bit 透空遮罩（原版存在影像集裡，
	// 見 internal/assets/gfx）。以素材指標為鍵，沒有登記就是整塊不透空。
	// **只有 DOS 的素材有**：其他平台的容器沒有這個欄位，那邊仍走透空色。
	stencils map[*image.Paletted]*image.Alpha

	hi map[*image.Paletted]*image.Paletted // Scale3x
	up map[*image.Paletted]*image.Paletted // 整數倍 nearest
	us map[*image.Alpha]*image.Alpha       // 遮罩的整數倍 nearest

	// Out 是野外那條路徑的四組素材（`OUTDOOR1-3` ＋ 該圖的地形檔），
	// OutFloor 是野外的地板（`OUTF.16`）。空的話野外就退回室內那條畫。
	Out      [][]*image.Paletted
	OutFloor []*image.Paletted

	// variants 是同一個平台的**場景變體**：原版每種場景各一套牆面素材
	// （DOS 的 `town`／`cave`／`castle`，MSX 的四張表），依場景碼挑。
	// 取用一律走 `For`，沒有登記變體就回自己。
	variants map[int]*TownSet
}

// SetVariant 登記某個場景碼要用的素材。
//
// **場景碼是算出來的**（`game.World.Scene()` 由地圖編號導出），所以這裡
// 只要登記，不必有人在換圖時記得換 —— 換圖忘了換素材那條漏在結構上
// 就不存在。原版是每次換圖比對 `ds:039C` 再重載，行為相同。
func (t *TownSet) SetVariant(scene int, v *TownSet) {
	if v == nil || v == t {
		return
	}
	if t.variants == nil {
		t.variants = map[int]*TownSet{}
	}
	t.variants[scene] = v
}

// For 回傳這個場景碼該用的素材。沒有變體就是自己。
//
// 風格（原版像素／Scale3x）跟著主素材走 —— 玩家按 `F5` 改的是整套的
// 顯示方式，不是某一種場景的。
func (t *TownSet) For(scene int) *TownSet {
	if t == nil {
		return nil
	}
	v, ok := t.variants[scene]
	if !ok {
		return t
	}
	v.Style = t.Style
	return v
}

// NewTownSet 準備 DOS 的素材（4bpp packed，EGA 16 色）。
func NewTownSet(walls, floor, torch, sky []gfx.Image) *TownSet {
	st := map[*image.Paletted]*image.Alpha{}
	conv := func(src []gfx.Image) []*image.Paletted {
		out := make([]*image.Paletted, len(src))
		for i, im := range src {
			out[i] = im.Paletted(gfx.EGAPalette)
			if m := im.Stencil(); m != nil {
				st[out[i]] = m
			}
		}
		return out
	}
	return &TownSet{Walls: conv(walls), Floor: conv(floor), Torch: conv(torch), Sky: conv(sky),
		Platform: PlatformDOS, clear: 8, torchStride: 4, flames: TorchFrames, front: torchFront,
		stencils: st,
		hi:       map[*image.Paletted]*image.Paletted{},
		up:       map[*image.Paletted]*image.Paletted{},
		us:       map[*image.Alpha]*image.Alpha{}}
}

// NewSceneSet 準備非 DOS 平台的素材（已經是索引色，帶自己的調色盤）。
func NewSceneSet(p Platform, walls, floor, torch, sky []*image.Paletted,
	clear uint8, torchStride int) *TownSet {
	return &TownSet{Walls: walls, Floor: floor, Torch: torch, Sky: sky,
		Platform: p, clear: clear, torchStride: torchStride, flames: torchStride, front: torchFront,
		hi: map[*image.Paletted]*image.Paletted{},
		up: map[*image.Paletted]*image.Paletted{}}
}

// NewPackSet 準備烘好的高解析素材包（已經放大 render.Scale 倍）。
func NewPackSet(p Platform, walls, floor, torch, sky []*image.Paletted,
	clear uint8, torchStride int) *TownSet {
	t := NewSceneSet(p, walls, floor, torch, sky, clear, torchStride)
	t.preScaled = true
	return t
}

// AddStencils 登記一批 DOS 素材自己的 1-bit 透空遮罩，src 與 dst 一一對應。
//
// 給「不是從 NewTownSet 進來」的素材用（野外的三組擋路物與地形檔就是
// 這一類：它們與牆同一種容器，也各自帶遮罩，只是走另一條載入路徑）。
func (t *TownSet) AddStencils(src []gfx.Image, dst []*image.Paletted) {
	if t.stencils == nil {
		t.stencils = map[*image.Paletted]*image.Alpha{}
	}
	for i := range src {
		if i >= len(dst) || dst[i] == nil {
			continue
		}
		if m := src[i].Stencil(); m != nil {
			t.stencils[dst[i]] = m
		}
	}
}

// Fixed 表示這一套素材是烘好的高解析圖，換風格不會有任何效果。
func (t *TownSet) Fixed() bool { return t.preScaled }

// size 回傳一張素材在**原版座標空間**的寬高。
//
// 幾何全部算在原版座標上（置中、貼齊底邊、鏡射都要用到寬高），
// 而素材包的圖已經放大過 —— 直接拿 `Bounds()` 會把每一個「除以二」
// 算成三倍大，症狀是整幅畫面往上往左散開，看起來像座標公式寫錯。
func (t *TownSet) size(im *image.Paletted) (int, int) {
	if im == nil {
		return 0, 0
	}
	b := im.Bounds()
	if t.preScaled {
		return b.Dx() / render.Scale, b.Dy() / render.Scale
	}
	return b.Dx(), b.Dy()
}

// NewPlacedSet 準備「落點另有一張表」的素材（MSX 與 Mega Drive）。
//
// view 是這一套素材自己的視圖大小，比 FPW×FPH 小的話整幅置中。
func NewPlacedSet(p Platform, walls, torches []*image.Paletted,
	place, torchPlace []image.Point,
	bg *image.Paletted, clear uint8, stride int, view image.Point) *TownSet {
	t := NewSceneSet(p, walls, nil, torches, nil, clear, stride)
	t.place = place
	t.torchPlace = torchPlace
	// **視圖比 DOS 小的話拉滿，不要置中。** 置中的話四周留一圈黑：
	// MSX 的 154×64 放進 208×120 只佔寬 74%、高 53%，看起來像畫面破了。
	// 拉滿的代價是長寬比差 8%（MSX 的像素是半寬，154×64 在 4:3 螢幕上
	// 實際約 1.33:1，DOS 那框是 1.44:1）—— 看不出來，而留白看得出來。
	if view.X > 0 && view.Y > 0 && (view.X != FPW || view.Y != FPH) {
		t.native = view
	}
	if bg != nil {
		t.Sky = []*image.Paletted{bg}
	}
	return t
}

// canvasFor 準備（或重用）離屏畫布。調色盤取自這一套素材的任何一張圖 ——
// 各平台自帶調色盤，拿 EGA 那張會整幅變色。
func (t *TownSet) canvasFor() *image.Paletted {
	if t.native.X <= 0 || t.native.Y <= 0 {
		return nil
	}
	if t.canvas == nil {
		var pal []*image.Paletted
		pal = append(pal, t.Sky...)
		pal = append(pal, t.Walls...)
		pal = append(pal, t.Torch...)
		for _, im := range pal {
			if im != nil {
				t.canvas = image.NewPaletted(image.Rect(0, 0, t.native.X, t.native.Y),
					im.Palette)
				break
			}
		}
		if t.canvas == nil {
			return nil
		}
	}
	for i := range t.canvas.Pix {
		t.canvas.Pix[i] = 0
	}
	return t.canvas
}

// wallPos 回傳第 i 張牆素材該貼的位置。
//
// MSX 與 Mega Drive 的透視自成一套，落點另有一張表（`place`）；
// 其餘素材走 DOS 的落點表，由呼叫端查好傳進來。
func (t *TownSet) wallPos(i int, defX, defY int) (int, int) {
	if i >= 0 && i < len(t.place) {
		p := t.place[i]
		return FPX + t.origin.X + p.X, FPY + t.origin.Y + p.Y
	}
	return defX, defY
}

// blit 依風格把一張原版素材畫上去，座標一律是原版座標。
//
// **所有貼圖都要走這裡。** 直接呼叫 `s.Blit` 的地方在切到 StyleModern
// 之後會留在原版層，被 Flush 蓋掉的高解析貼圖壓在下面，症狀是
// 「換了風格之後某一塊還是馬賽克」——而那一塊看起來只是沒放大成功。
func (t *TownSet) blit(s *render.Screen, im *image.Paletted, x, y int) {
	t.blitKey(s, im, x, y, -1)
}

// blitKey 與 blit 相同，但 key ≥ 0 時跳過該色號。
//
// **只有「DOS ＋ 原版像素」走原版層**，那條路徑要與原版逐像素相同
// （`cmd/mm2diff` 守著）。其餘情形一律走高解析層：Amiga 的素材有自己的
// 32 色調色盤，塞不進原版層那張 EGA 調色盤的畫布。
func (t *TownSet) blitKey(s *render.Screen, im *image.Paletted, x, y, key int) {
	t.blitMasked(s, im, nil, x, y, key)
}

// blitMasked 與 blitKey 相同，但透空的形狀由 mask 決定（見 render.BlitMask）。
func (t *TownSet) blitMasked(s *render.Screen, im, mask *image.Paletted, x, y, key int) {
	if im == nil {
		return
	}
	// 原版存出來的遮罩優先：它是這張圖的透空**資料**，而 key／mask
	// 是沒有遮罩的平台才需要的推測。
	if st := t.stencils[im]; st != nil {
		t.blitStencil(s, im, st, x, y)
		return
	}
	// 有離屏畫布就畫到畫布上，座標扣掉視圖原點（畫布的左上角是視圖左上角）。
	if t.canvas != nil {
		drawOnto(t.canvas, im, mask, x-FPX, y-FPY, key)
		return
	}
	if t.Platform == PlatformDOS && t.Style != StyleModern {
		if key < 0 {
			s.Blit(im, x, y)
		} else {
			s.BlitMask(im, mask, x, y, uint8(key))
		}
		return
	}
	up := t.upscaled(im)
	if key < 0 {
		s.BlitHi(up, x, y)
		return
	}
	var upMask *image.Paletted
	if mask != nil {
		upMask = t.upscaled(mask)
	}
	s.BlitHiMask(up, upMask, x, y, uint8(key))
}

// blitStencil 照原版的 1-bit 遮罩貼一張素材，三條路徑與 blitMasked 相同。
func (t *TownSet) blitStencil(s *render.Screen, im *image.Paletted, st *image.Alpha, x, y int) {
	if t.canvas != nil {
		drawStencil(t.canvas, im, st, x-FPX, y-FPY)
		return
	}
	if t.Platform == PlatformDOS && t.Style != StyleModern {
		s.BlitStencil(im, st, x, y)
		return
	}
	s.BlitHiStencil(t.upscaled(im), t.upStencil(st), x, y)
}

// upStencil 把遮罩放大到與 upscaled 相同的倍率（一律 nearest：遮罩是
// 「畫或不畫」，插值出來的中間值沒有意義）。
func (t *TownSet) upStencil(st *image.Alpha) *image.Alpha {
	if t.preScaled || st == nil {
		return st
	}
	if v, ok := t.us[st]; ok {
		return v
	}
	b := st.Bounds()
	dst := image.NewAlpha(image.Rect(0, 0, b.Dx()*render.Scale, b.Dy()*render.Scale))
	for y := 0; y < b.Dy()*render.Scale; y++ {
		for x := 0; x < b.Dx()*render.Scale; x++ {
			dst.SetAlpha(x, y, st.AlphaAt(b.Min.X+x/render.Scale, b.Min.Y+y/render.Scale))
		}
	}
	t.us[st] = dst
	return dst
}

// drawStencil 是 blitStencil 的離屏畫布版本。
func drawStencil(dst, src *image.Paletted, st *image.Alpha, x, y int) {
	b, db, sb := src.Bounds(), dst.Bounds(), st.Bounds()
	for sy := 0; sy < b.Dy(); sy++ {
		dy := y + sy
		if dy < 0 || dy >= db.Dy() {
			continue
		}
		for sx := 0; sx < b.Dx(); sx++ {
			dx := x + sx
			if dx < 0 || dx >= db.Dx() {
				continue
			}
			if sx < sb.Dx() && sy < sb.Dy() && st.AlphaAt(sb.Min.X+sx, sb.Min.Y+sy).A == 0 {
				continue
			}
			dst.SetColorIndex(db.Min.X+dx, db.Min.Y+dy, src.ColorIndexAt(b.Min.X+sx, b.Min.Y+sy))
		}
	}
}

// drawOnto 把一張圖畫進離屏畫布，透空規則與 render.BlitMask 相同。
//
// 為什麼不共用 render 那一支：那邊寫的是 `Screen.Orig`，這裡寫的是
// 一張自己的畫布。規則相同、目的地不同 —— 兩邊都很短，各寫一份比
// 為了共用而把目的地抽象化清楚。
func drawOnto(dst, src, mask *image.Paletted, x, y, key int) {
	b, db := src.Bounds(), dst.Bounds()
	mb := image.Rectangle{}
	if mask != nil {
		mb = mask.Bounds()
	}
	for sy := 0; sy < b.Dy(); sy++ {
		dy := y + sy
		if dy < 0 || dy >= db.Dy() {
			continue
		}
		for sx := 0; sx < b.Dx(); sx++ {
			dx := x + sx
			if dx < 0 || dx >= db.Dx() {
				continue
			}
			c := src.ColorIndexAt(b.Min.X+sx, b.Min.Y+sy)
			if key >= 0 && int(c) == key {
				if mask == nil || sx >= mb.Dx() || sy >= mb.Dy() ||
					int(mask.ColorIndexAt(mb.Min.X+sx, mb.Min.Y+sy)) == key {
					continue
				}
			}
			dst.SetColorIndex(db.Min.X+dx, db.Min.Y+dy, c)
		}
	}
}

// upscaled 把一張素材放大到高解析層的倍率，依風格選演算法。
func (t *TownSet) upscaled(im *image.Paletted) *image.Paletted {
	if t.preScaled {
		return im
	}
	if t.Style == StyleModern {
		return t.scaled(im)
	}
	if v, ok := t.up[im]; ok {
		return v
	}
	v := render.ScaleN(im, render.Scale)
	t.up[im] = v
	return v
}

// AmigaTorchStride 是 Amiga 火炬每一格佔的張數（三張火焰，沒有底圖）。
const AmigaTorchStride = 3

// Prepare 預先把兩種風格要用的放大版本算好。
//
// 不呼叫也能跑（放大是照需要算的），差別在**第一次切換的那一格**：
// 七十幾張素材要重算，會掉一格。切換素材是玩家隨手按的動作，
// 掉格會被當成當掉，所以成本挪到載入時付。
func (t *TownSet) Prepare() {
	for _, v := range t.variants {
		v.Prepare()
	}
	if t.preScaled {
		return
	}
	st := t.Style
	for _, style := range []Style{StyleClassic, StyleModern} {
		t.Style = style
		for _, g := range [][]*image.Paletted{t.Walls, t.Floor, t.Torch, t.Sky} {
			for _, im := range g {
				if im == nil || (t.Platform == PlatformDOS && style == StyleClassic) {
					continue // DOS ＋ 原版像素直接走原版層，不必放大
				}
				t.upscaled(im)
			}
		}
	}
	t.Style = st
}

// scaled 回傳放大三倍的版本，以來源指標為鍵快取。
//
// 快取有效的前提是**來源指標穩定** —— 所有素材都經過 cached 那張表，
// 每張圖只展開一次。若哪天有人改成每格重新 `Paletted()`，這裡會變成
// 每格都重算 Scale3x，而且是安靜地變慢，不會壞掉。
func (t *TownSet) scaled(im *image.Paletted) *image.Paletted {
	if im == nil {
		return nil
	}
	if v, ok := t.hi[im]; ok {
		return v
	}
	v := render.Scale3x(im)
	t.hi[im] = v
	return v
}

// 火炬。
//
// `TOWNT.16` 的 36 張排成 **3 組 × 3 個深度 × 4 張**，每個深度的四張是
// 「一張含燈桿的底圖 + 三張只有火焰的動畫」；動畫那三張與底圖共用左上角，
// 只蓋掉上半部，燈桿留著不重畫 —— 原版省重繪的老作法。第三張與底圖的
// 火焰相同（樣板比對兩張都取 100%），所以動畫是「靜、動、動、回靜」。
//
// 三組分別是**左側牆、右側牆、正面**：前兩組的燈桿斜著畫（牆是斜的），
// 第三組直立。位置全部用 `cmd/mm2match` 把每一張影格滑過原版截圖定出來，
// 每一張在整批截圖裡都只有唯一一個 100% 的落點：
//
//	組 A 左側牆   影格  0/ 4/ 8   (8,42) (40,56) (64,59)
//	組 B 右側牆   影格 12/16/20   (192,42) (160,56) (136,57)
//	組 C 正面     影格 24/28/32   (104,44) (8,54)+(200,54) (104,60)
//
// 換算成視圖區內座標寫進下面幾張表。兩件事要記著：
//
//   - **落點是量出來的，不是鏡射出來的。** 右側牆深度 2（影格 20–23）在
//     那 126 張截圖裡一次都沒出現，先前用左右鏡射補成 (136,51)；
//     `(9,4)` 面北量到的是 **(128,49)**，四張影格都 100%。同一個深度的
//     左右兩盞 y 差了 2 —— 鏡射在深度 0 與 1 成立，不代表深度 2 也成立。
//   - **組 C 不是「正牆的火炬」，是「正面朝向的火炬」。** 縱列牆是隔壁那格
//     的正牆，朝向一樣，所以也用這一組（見 colTorchLeft）；深度 1 的
//     (8,54)／(200,54) 就是那一對，位置在視圖最左與最右，不在中央。
type torchSlot struct {
	base, first int // 底圖與第一張火焰的影格編號
	x, y        int // 視圖區內的左上角
}

var torchLeft = [Depth - 1]torchSlot{
	{base: 0, first: 1, x: 0, y: 34},
	{base: 4, first: 5, x: 32, y: 48},
	{base: 8, first: 9, x: 56, y: 51},
}

var torchRight = [Depth - 1]torchSlot{
	{base: 12, first: 13, x: 184, y: 34},
	{base: 16, first: 17, x: 152, y: 48},
	{base: 20, first: 21, x: 128, y: 49}, // (9,4) 面北實測，四張影格都 100%
}

// 正牆上的火炬，直立燈桿、置中。
//
// **DOS 的深度 1 那一階沒有素材** —— 影格 28–31 不是正牆的火炬，是補牆的
// （見 flankTorch）。別的平台不見得缺：Mega Drive 的 `sub_3836` 四個深度
// 都畫，所以這張表是每一套素材各一份（`TownSet.front`），不是全域固定的。
var torchFront = [Depth - 1]torchSlot{
	{base: 24, first: 25, x: 96, y: 36}, // 正牆深度 0（`shots/s5.png` 目視確認）
	{base: -1, first: -1},               // 深度 1：DOS 沒有這一階
	{base: 32, first: 33, x: 96, y: 52}, // 正牆深度 2（`shots/p5.png`）
}

// TorchStride 回傳一組火炬佔幾張圖。只有素材包的驗收會用到。
func (t *TownSet) TorchStride() int { return t.torchStride }

// TorchPos 回傳第 i 張火炬影格在視圖裡的落點；沒有落點表就回 (0,0)。
// 只有素材包的驗收會用到。
func (t *TownSet) TorchPos(i int) image.Point {
	if i < 0 || i >= len(t.torchPlace) {
		return image.Point{}
	}
	return t.torchPlace[i]
}

// SetFrontTorchGroup 指定正牆第 d 階用第 group 組火炬影格。
//
// group 沿用 DOS 的分組編號（影格 `group × 4` 起），落點由素材包自己的
// `torchPlace` 決定，所以這裡不必給座標。DOS 沒有的組（例如深度 1）
// 可以給素材包自己新增的組號。
func (t *TownSet) SetFrontTorchGroup(d, group int) {
	if d < 0 || d >= len(t.front) || group < 0 {
		return
	}
	t.front[d] = torchSlot{base: group * 4, first: group*4 + 1}
}

// 縱列牆上的火炬，視圖內座標。
//
// 用的是那個深度的**組 C**（正面朝向）影格，不是側牆的 —— 縱列牆本來就是
// 隔壁那格的正牆，朝向一樣，所以燈桿是直立的那一組（24／28／32）。
//
// 四筆全部是實測，**左右各一張表，不鏡射**：
//
//	深度 1  (0,46)／(192,46)   神殿與酒館兩張整幅插畫、(7,5) 面北、(7,4) 面西
//	深度 2  (48,50)／(144,52)  (9,4) 面北的左邊、(6,6) 面南的右邊
//
// 深度 2 的左右 y 差 2，與側牆那一組（左 51、右 49）一樣不對稱，
// 所以不能拿一邊推另一邊。深度 0 與 3 還沒量到，留空不畫 ——
// **空著會少一盞，猜錯會多一盞在錯的地方**，前者比較容易被下一次量到。
var (
	colTorchLeft = [Depth]torchSlot{
		{base: -1, first: -1},
		{base: 28, first: 29, x: 0, y: 46},
		{base: 32, first: 33, x: 48, y: 50},
		{base: -1, first: -1},
	}
	colTorchRight = [Depth]torchSlot{
		{base: -1, first: -1},
		{base: 28, first: 29, x: 192, y: 46},
		{base: 32, first: 33, x: 144, y: 52},
		{base: -1, first: -1},
	}
)

// drawColTorch 在縱列牆上點一盞。
func (t *TownSet) drawColTorch(s *render.Screen, d int, isLeft bool, phase int) {
	if d < 0 || d >= Depth || len(t.place) > 0 {
		return // 有落點表的素材（MSX／Mega Drive）沒有這一組
	}
	sl := colTorchRight[d]
	if isLeft {
		sl = colTorchLeft[d]
	}
	if sl.base < 0 {
		return
	}
	t.blitTorch(s, &sl, FPX+sl.x, phase)
}

// TorchFrames 是 DOS 火焰動畫的張數。
const TorchFrames = 3

// TorchCycle 是相位計數器的循環長度。
//
// 各平台的相位數不同（DOS 3、MSX 1、Mega Drive 9），計數器是共用的 ——
// 循環長度要能被每一個整除，否則換素材的那一刻相位會跳。
const TorchCycle = 36

func (t *TownSet) torch(i int) *image.Paletted {
	if i < 0 || i >= len(t.Torch) {
		return nil
	}
	return t.Torch[i]
}

// torchFrames 把 DOS 的影格編號換算成這個平台的。
//
// DOS 每一格四張（底圖 + 三張火焰），Amiga 只有三張火焰。少了底圖就
// 沒有燈桿可畫 —— 這是素材本身的差異，不是解錯了。
func (t *TownSet) torchFrames(sl *torchSlot) (base, first int) {
	if sl.base < 0 {
		return -1, -1
	}
	if t.torchStride == 4 {
		return sl.base, sl.first
	}
	return -1, sl.base / 4 * t.torchStride
}

// torchSide 選出某個深度、某一面牆該用哪一格。
func (t *TownSet) torchSide(d int, side game.Facing, face game.Facing) *torchSlot {
	if d < 0 || d >= Depth-1 {
		return nil
	}
	switch {
	case side == face:
		if t.front[d].base < 0 {
			return nil
		}
		return &t.front[d]
	case side == game.Facing((int(face)+3)&3):
		return &torchLeft[d]
	case side == game.Facing((int(face)+1)&3):
		return &torchRight[d]
	}
	return nil
}

// drawTorch 在某個深度的某一面牆上點一盞火炬。phase 是動畫相位。
func (t *TownSet) drawTorch(s *render.Screen, d int, side, face game.Facing, phase int) {
	if sl := t.torchSide(d, side, face); sl != nil {
		t.blitTorch(s, sl, FPX+sl.x, phase)
	}
}

// wallImage 把「哪一種牆」加「哪一格」換成 `TOWN.16` 的影格編號。
//
// 那 32 張是 16 張 × 兩種變體：前 16 張是石牆、後 16 張是**同一組牆畫上門**
// （見 docs/formats/04 §3.5）。所以門只是換一個變體，位置與尺寸完全一樣。
//
// 種類 3 沒有第三套貼圖，用石牆那一套 —— 原版的撞牆訊息也把它折成實牆，
// 差別只在牆上點不點火炬（見 `game.HasTorch`）。
func wallImage(k game.WallKind, slot int) int {
	if k == game.WallDoor {
		return slot + doorVariant
	}
	return slot
}

// doorVariant 是門那一組貼圖在 `TOWN.16` 裡的起始索引。
const doorVariant = 16

// 左右那一縱列的正牆（原版 `sub_185B4`／`sub_1867C` 的 bit7 分支）。
//
// **這不是「正牆兩側的補牆」**：畫的是隔壁那一格朝我這邊的牆，所以
// 條件是「這一側沒有側牆，而隔壁那一格有正面的牆」，不是「正牆存在」。
// 先前無條件跟著正牆畫，`cmd/mm2diff` 在 (8,0) 面東量到 9.9% 的差異
// 就是這條 —— 走廊中距離的內緣少一塊、又多畫一塊。
//
// 影格與落點是 DGROUP 的六張表：左邊 `ds:154E`（影格，**byte**）／
// `ds:1562`（x）／`ds:156A`（y），右邊 `ds:1572`／`1586`／`158E`。
// 深度 0／1 用 `TOWN.16` 的 12／14（左）與 13／15（右），
// 深度 2／3 直接用正牆的影格 2／3，只是貼在偏左或偏右。
var (
	colLeftFrame  = [Depth]int{12, 14, 2, 3}
	colRightFrame = [Depth]int{13, 15, 2, 3}
	colLeftX      = [Depth]int{8, 8, 40, 88}
	colRightX     = [Depth]int{192, 160, 136, 120}
	colY          = [Depth]int{22, 40, 54, 62}
)

// blitColumn 貼一張「隔壁縱列的正牆」。x／y 是**畫面座標**（含視圖原點）。
//
// 與側牆同樣是斜看過去的面，透空的形狀同樣取自素牆那一張。有落點表的
// 素材（MSX／Mega Drive）沒有這一組，落點交給 wallPos 決定。
func (t *TownSet) blitColumn(s *render.Screen, frame, x, y int) {
	t.blitWall(s, frame, x, y)
}

// blitTorch 貼一盞火炬：底圖加上這一個相位的火焰。
func (t *TownSet) blitTorch(s *render.Screen, sl *torchSlot, x, phase int) {
	base, first := t.torchFrames(sl)
	if first < 0 {
		return
	}
	// 相位數是**每一套素材各自的**：DOS 三張、MSX 一張（沒有動畫可播，
	// 硬加相位會索引到隔壁那盞）、Mega Drive 九張。用固定的 3 取模，
	// Mega Drive 的九個相位就只會播到前三個。
	frame := 0
	if t.flames > 0 {
		frame = phase % t.flames
	}
	y := FPY + sl.y
	if first < len(t.torchPlace) {
		p := t.torchPlace[first]
		x, y = FPX+t.origin.X+p.X, FPY+t.origin.Y+p.Y
	}
	// **有落點表的素材包，火炬要吃透空色。**
	//
	// DOS 那四張是整塊不透明的（燈桿與火焰各佔滿自己那一張），照抄就對；
	// MSX 與 Mega Drive 不是 —— Mega Drive 一塊 16×32 裡有 407 個像素是
	// 索引 0，原版讓底下的牆從那些位置透出來。用一般的 Blit 會在牆上留
	// 一個黑方塊，而那個黑方塊看起來只像「這面牆有個壁龕」。
	//
	// 判準用「有沒有落點表」不用平台：烘好的高解析包也不是 DOS，但它的
	// 火炬是 DOS 素材放大的，色號 8 在那上面是燈桿的顏色不是透空色。
	key := -1
	if len(t.torchPlace) > 0 {
		key = int(t.clear)
	}
	t.blitKey(s, t.torch(base), x, y, key)
	t.blitKey(s, t.torch(first+frame), x, y, key)
}

// floor 與 sky 也走同一張快取 —— 不只是省解碼，Scale3x 的快取以來源指標
// 為鍵，來源每格重新配置的話那張表會無限長大。
func (t *TownSet) floor() *image.Paletted {
	if len(t.Floor) == 0 {
		return nil
	}
	return t.Floor[0]
}

func (t *TownSet) sky(i int) *image.Paletted {
	if i < 0 || i >= len(t.Sky) {
		return nil
	}
	return t.Sky[i]
}

func (t *TownSet) wall(i int) *image.Paletted {
	if i < 0 || i >= len(t.Walls) {
		return nil
	}
	return t.Walls[i]
}

// DrawFirstPerson 畫出從 (w.X, w.Y) 朝 w.Face 看出去的畫面。
//
// 由遠到近疊：先鋪地板，再從最深的一格往回畫側牆，遇到正面有牆就畫上
// 正牆並停止 —— 後面的東西被擋住了，不必畫。
func DrawFirstPerson(s *render.Screen, w *game.World, t *TownSet) {
	DrawFirstPersonAt(s, w, t, 0)
}

// DrawFirstPersonAt 與 DrawFirstPerson 相同，但指定火炬的動畫相位。
func DrawFirstPersonAt(s *render.Screen, w *game.World, t *TownSet, phase int) {
	// 場景變體在這裡解析，呼叫端一律傳「這個平台的素材」就好。
	m := w.CurrentMap()
	// 素材由**一個鍵**挑：室內用場景碼（0–6），野外用貼圖組碼（9–12）。
	// 兩個碼域不重疊，所以共用一張變體表；`ATTRIB` 沒載入時
	// `TileSet` 是 0，退回場景碼那條，與先前相同。
	key := w.Scene()
	if m != nil && m.TileSet != 0 {
		key = m.TileSet
	}
	t = t.For(key)
	if m == nil || t == nil {
		return
	}
	// 自成一套解析度的素材（MSX）先合成到離屏畫布，最後整幅縮放填滿
	// 視圖框。**縮放只做一次** —— 每一塊各自縮放會在接縫裂出縫。
	if c := t.canvasFor(); c != nil {
		t.canvas = c
		defer func() {
			t.canvas = nil
			s.BlitHiFit(c, FPX, FPY, FPW, FPH)
		}()
	}
	// 野外是另一條繪圖路徑（原版連「32 張牆」那組都不載）。
	// MSX 的戶外又是自己一條 —— 它是一整排格子，不是「正面／左／右」。
	if m.TileSet != 0 {
		if t.drawMSXOutdoor(s, w) || t.drawOutdoor(s, w, phase) {
			return
		}
	}
	t.drawSky(s, w)
	if len(t.Floor) > 0 {
		_, h := t.size(t.floor())
		t.blit(s, t.floor(), FPX, FPY+FPH-h)
	}

	left := game.Facing((int(w.Face) + 3) & 3)
	right := game.Facing((int(w.Face) + 1) & 3)
	dx, dy := w.Face.Delta()

	// 先走一趟決定每個深度要畫什麼，再由遠到近貼，近的才會蓋住遠的。
	//
	// **決定畫不畫的是 `DrawKind` 不是 `HasWall`** —— 兩者會不一致：
	// 設施的門走得過去但看得見，屏障走不過去但看不見。視線也因此要走到
	// 第一面**畫得出來**的正牆才停，不是第一面擋路的。
	type slot struct {
		l, r, front game.WallKind
		// lc／rc 是**左右那一縱列的正牆**：這一側沒有側牆時，看得到的是
		// 隔壁那一格朝我這邊的牆。原版 `sub_1BEBA` 每個深度就是這樣填的
		// —— 先看「我這一格的左／右面」，沒有才看「隔壁那一格的正面」。
		lc, rc     game.WallKind
		lt, rt, ft bool
		// lct／rct 是那一片縱列牆上點不點火炬。條件與側牆同一條
		// （牆種類 3，見 game.HasTorch）—— 縱列牆是隔壁那格的正牆，
		// 火炬跟著**那一面**走，不是跟著「有沒有畫縱列牆」走。
		// (6,6) 面南的左邊是一扇門（種類 2），原版沒點；先前無條件跟著
		// 縱列牆畫，那一格就多出一盞。
		lct, rct bool
	}
	lx, ly := left.Delta()
	rx, ry := right.Delta()
	// **每一格都要先設成 `WallNone`。** `WallNone` 是 `0xFF`（原版的訊息
	// 編號用完了才輪到它），所以結構的零值是 `WallBarrier` ——「沒填到的
	// 那一格」與「這裡有一面屏障」數值上一模一樣，繪圖迴圈會把它當成牆畫。
	// 症狀極輕：多畫的那幾片幾乎都被近處的牆蓋掉，(8,3) 面東只漏出
	// **3 個像素**，看起來像素材差一列，不像整個深度多畫了一層。
	var slots [Depth]slot
	for d := range slots {
		slots[d] = slot{l: game.WallNone, r: game.WallNone, front: game.WallNone,
			lc: game.WallNone, rc: game.WallNone}
	}
	last := -1
	x, y := w.X, w.Y
	for d := 0; d < Depth; d++ {
		if game.Cell(x, y) < 0 {
			break
		}
		slots[d] = slot{
			l:     m.DrawKind(x, y, left),
			r:     m.DrawKind(x, y, right),
			front: m.DrawKind(x, y, w.Face),
			lc:    game.WallNone,
			rc:    game.WallNone,
			lt:    m.HasTorch(x, y, left),
			rt:    m.HasTorch(x, y, right),
			ft:    m.HasTorch(x, y, w.Face),
		}
		if slots[d].l == game.WallNone {
			slots[d].lc = m.DrawKind(x+lx, y+ly, w.Face)
			slots[d].lct = m.HasTorch(x+lx, y+ly, w.Face)
		}
		if slots[d].r == game.WallNone {
			slots[d].rc = m.DrawKind(x+rx, y+ry, w.Face)
			slots[d].rct = m.HasTorch(x+rx, y+ry, w.Face)
		}
		last = d
		if slots[d].front != game.WallNone {
			// 正面有牆就到底了 —— 但**再遠一格的隔壁縱列還看得到**
			// （原版在同一段裡多填一格），前提是這一側兩種都空。
			if d+1 < Depth {
				nx, ny := x+dx, y+dy
				if slots[d].l == game.WallNone && slots[d].lc == game.WallNone {
					slots[d+1].lc = m.DrawKind(nx+lx, ny+ly, w.Face)
					slots[d+1].lct = m.HasTorch(nx+lx, ny+ly, w.Face)
				}
				if slots[d].r == game.WallNone && slots[d].rc == game.WallNone {
					slots[d+1].rc = m.DrawKind(nx+rx, ny+ry, w.Face)
					slots[d+1].rct = m.HasTorch(nx+rx, ny+ry, w.Face)
				}
				if slots[d+1].lc != game.WallNone || slots[d+1].rc != game.WallNone {
					last = d + 1
				}
			}
			break
		}
		x, y = x+dx, y+dy
	}

	for d := last; d >= 0; d-- {
		// 左半：側牆，沒有就畫左邊那一縱列的正牆（原版 `sub_185B4`
		// 的兩個分支，bit7 那個走另一組影格與落點）。
		if slots[d].l != game.WallNone {
			if i := wallImage(slots[d].l, 4+d); t.wall(i) != nil {
				t.blitSlot(s, i, sideLeftX[d], sideY[d])
			}
		} else if slots[d].lc != game.WallNone {
			t.blitColumn(s, wallImage(slots[d].lc, colLeftFrame[d]), colLeftX[d], colY[d])
			if slots[d].lct {
				t.drawColTorch(s, d, true, phase)
			}
		}
		// 右半同理（`sub_1867C`）。
		if slots[d].r != game.WallNone {
			if i := wallImage(slots[d].r, 8+d); t.wall(i) != nil {
				t.blitSlot(s, i, sideRightX[d], sideY[d])
			}
		} else if slots[d].rc != game.WallNone {
			t.blitColumn(s, wallImage(slots[d].rc, colRightFrame[d]), colRightX[d], colY[d])
			if slots[d].rct {
				t.drawColTorch(s, d, false, phase)
			}
		}
		if slots[d].front != game.WallNone {
			if i := wallImage(slots[d].front, d); t.wall(i) != nil {
				t.blitFront(s, i, frontX[d], frontY[d])
			}
		}
		// 火炬畫在牆上，所以要在牆之後、下一個更近的深度之前。
		if slots[d].lt {
			t.drawTorch(s, d, left, w.Face, phase)
		}
		if slots[d].rt {
			t.drawTorch(s, d, right, w.Face, phase)
		}
		if slots[d].ft {
			t.drawTorch(s, d, w.Face, w.Face, phase)
		}
	}
}

// blitSlot 貼第 i 張側牆素材，落點由 wallPos 決定。
func (t *TownSet) blitSlot(s *render.Screen, i, defX, defY int) {
	t.blitWall(s, i, defX, defY)
}

// blitFront 貼正牆，**整塊不透空**。
//
// 這不是推論：正牆那八張（影格 0–3 與 16–19）在容器裡的遮罩指標就是 0，
// 而側牆那十六張各有一份遮罩。原因看得出來 —— 正牆的寬度剛好等於同深度
// 兩片側牆之間的縫（160/96/48/16 對上 24+24／32+32／24+24／16+16 的兩側），
// 整塊蓋上去也蓋不到別人。
func (t *TownSet) blitFront(s *render.Screen, i, defX, defY int) {
	im := t.wall(i)
	if im == nil {
		return
	}
	x, y := t.wallPos(i, defX, defY)
	t.blitKey(s, im, x, y, -1)
}

// blitWall 貼一張斜看過去的牆（側牆或縱列牆）。
//
// DOS 的素材自己帶遮罩，blitMasked 會優先用它，這裡算出來的 mask 用不到。
// 剩下的是**沒有遮罩的平台**：那邊只能拿色號推，而門那一組（同一面牆
// 畫上門）的柵欄也是透空色與 0 交錯的網紋、位置卻在牆裡面，拿自己當遮罩
// 會漏出後面的天空 —— 所以改用素牆那一張的形狀。
func (t *TownSet) blitWall(s *render.Screen, i, defX, defY int) {
	im := t.wall(i)
	if im == nil {
		return
	}
	x, y := t.wallPos(i, defX, defY)
	var mask *image.Paletted
	if len(t.place) == 0 {
		// 有落點表的素材（MSX／Mega Drive）是從一張大圖切下來的，
		// 素牆與門那兩塊不是同一個矩形，拿來當遮罩沒有意義。
		mask = t.wall(i - doorVariant)
	}
	t.blitMasked(s, im, mask, x, y, int(t.clear))
}

// wallClear 是 DOS 牆貼圖的透空色。各平台的值存在 TownSet.clear。
const wallClear = 8

// 視圖上半是 `SKY.16` 的兩張 208×60 之一，貼在視圖區的左上角。
//
// **這不是程式畫的花紋，是素材。** 影格 0 是白雲藍天、影格 1 是深藍與黑
// 交錯的棋盤，兩張都是 208×60 且整塊不透空（容器裡的遮罩指標都是 0），
// 棋盤是畫在素材裡的，不是漏出底色 —— 先前把它當成「抖動出來的天花板」
// 而用程式重畫，兩者長得一樣但來源不同。
//
// **哪一張由隊伍所在的格決定**：`2PLAY _2play_e02`（`0x18517`）查
// `ATTRIB.DAT` 的 `+32`…`+63` 那張 16×16 位元圖，有天花板回 1、沒有回 0，
// `_2play_e03` 直接拿它當影格編號（`0x18773`）。所以影格 1 其實是**天花板**
// 不是另一種天空 —— 24 張野外圖那塊位元圖全是 0，地城與城堡全是 1，
// 五座城鎮混合（街道露天、屋簷底下不是）。
//
// 先前對不起來是因為拿「正牆在第幾格」去對 —— 它跟牆無關，只看腳下那一格。
func (t *TownSet) drawSky(s *render.Screen, w *game.World) {
	frame := 0
	if m := w.CurrentMap(); m != nil {
		if c := game.Cell(w.X, w.Y); c >= 0 && m.Ceiling[c] {
			frame = 1
		}
	}
	if len(t.Sky) <= frame {
		frame = 0
	}
	if len(t.Sky) <= frame {
		return
	}
	t.blit(s, t.sky(frame), FPX+t.origin.X, FPY+t.origin.Y)
}
