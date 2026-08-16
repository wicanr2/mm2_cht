package view

import (
	"image"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 第一人稱視圖區的位置與大小。
//
// 原點是把 `TOWN.16` 的側牆圖樣板比對回原版截圖定出來的：深度 0 的
// 左右側牆（24×92）在 `shots/fpv.png` 的 (8,22) 與 (192,22)，兩張都是
// **100% 逐像素相符**。由此反推 `FPX+sideX[0] = 8`、
// `FPX+FPW-24 = 192`、`FPY+(FPH-92)/2 = 22`，三式同時成立於 (8,8,208,120)。
const (
	FPX, FPY = 8, 8
	FPW, FPH = 208, 120
	// Depth 是往前看幾格。素材只有四個深度的牆面。
	Depth = 4
)

// 側牆的水平配置。素材寬度累加起來剛好等於同深度正牆的左緣
// （24 → 56 → 80 → 96 對上 (208-160)/2、(208-96)/2、(208-48)/2、(208-16)/2），
// 所以貼圖不必再算透視，照這個表擺就對齊了。
var sideX = [Depth]int{0, 24, 56, 80}

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
	// 視圖只有 154×64（DOS 是 208×120），所以整幅置中放進視圖區；
	// 每張牆的落點另有一張表，算不出來 —— 兩邊的透視不是同一套。
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

	hi map[*image.Paletted]*image.Paletted // Scale3x
	up map[*image.Paletted]*image.Paletted // 整數倍 nearest
}

// NewTownSet 準備 DOS 的素材（4bpp packed，EGA 16 色）。
func NewTownSet(walls, floor, torch, sky []gfx.Image) *TownSet {
	conv := func(src []gfx.Image) []*image.Paletted {
		out := make([]*image.Paletted, len(src))
		for i, im := range src {
			out[i] = im.Paletted(gfx.EGAPalette)
		}
		return out
	}
	return &TownSet{Walls: conv(walls), Floor: conv(floor), Torch: conv(torch), Sky: conv(sky),
		Platform: PlatformDOS, clear: 8, torchStride: 4, front: torchFront,
		hi: map[*image.Paletted]*image.Paletted{},
		up: map[*image.Paletted]*image.Paletted{}}
}

// NewSceneSet 準備非 DOS 平台的素材（已經是索引色，帶自己的調色盤）。
func NewSceneSet(p Platform, walls, floor, torch, sky []*image.Paletted,
	clear uint8, torchStride int) *TownSet {
	return &TownSet{Walls: walls, Floor: floor, Torch: torch, Sky: sky,
		Platform: p, clear: clear, torchStride: torchStride, front: torchFront,
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
	t.origin = image.Pt((FPW-view.X)/2, (FPH-view.Y)/2)
	if bg != nil {
		t.Sky = []*image.Paletted{bg}
	}
	return t
}

// wallPos 回傳第 i 張牆素材該貼的位置。
//
// 有落點表就用表。沒有的話照 DOS 的算法：水平位置由呼叫端給、
// 垂直置中（透視消失點在視圖中央）。
func (t *TownSet) wallPos(i int, im *image.Paletted, defX int) (int, int) {
	if i >= 0 && i < len(t.place) {
		p := t.place[i]
		return FPX + t.origin.X + p.X, FPY + t.origin.Y + p.Y
	}
	_, h := t.size(im)
	return defX, FPY + (FPH-h)/2
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
	if im == nil {
		return
	}
	if t.Platform == PlatformDOS && t.Style != StyleModern {
		if key < 0 {
			s.Blit(im, x, y)
		} else {
			s.BlitKey(im, x, y, uint8(key))
		}
		return
	}
	up := t.upscaled(im)
	if key < 0 {
		s.BlitHi(up, x, y)
	} else {
		s.BlitHiKey(up, x, y, uint8(key))
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
//	組 B 右側牆   影格 12/16/20   (192,42) (160,56) —
//	組 C 正面     影格 24/28/32   (104,44) (8,54)+(200,54) (104,60)
//
// 換算成視圖區內座標寫進下面幾張表。兩個要記著的：
//
//   - **右側牆深度 2（影格 20–23）在 126 張截圖裡一次都沒出現。**
//     用左右鏡射補（`FPW − x − w`），這條規則在深度 0 與 1 上都成立。
//   - **組 C 的中間那一階（影格 28–31）落在視圖的最左與最右，不在中央** ——
//     它不是正牆的火炬，是**補牆**的（見 flankTorch）。
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
	{base: 20, first: 21, x: 136, y: 51}, // 鏡射：208 − 56 − 16
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

// flankTorch 是深度 1 補牆上的那一對火炬（影格 28–31，16×28）。
//
// 落點量了兩次都一致：神殿與酒館那兩張整幅插畫的 (8,54)／(200,54)，
// 以及 `shots/diff-shot.png`（(7,5) 面北）的同兩點，四筆都 100%。
// 換算成視圖內座標是 (0,46) 與 (192,46) —— **貼在視圖的最左與最右**，
// 不是置中，所以它屬於補牆不屬於正牆。
//
// 兩側各一盞，而且與側牆的火炬條件無關：(7,5) 的東西兩面在兩個平面上
// 都是空的，原版照樣點著這兩盞。**補牆畫出來就有火炬。**
var flankTorch = torchSlot{base: 28, first: 29, x: 0, y: 46}

// TorchFrames 是火焰動畫的張數。
const TorchFrames = 3

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

// 正牆兩側的補牆。
//
// 正牆比視圖窄，兩側露出來的部分由一對專用的圖填滿。寬度加起來剛好是
// 視圖寬，這是判斷它們用途的依據：
//
//	正牆深度 0   160 寬 → 兩側各 24   影格 12／13，24×92   24+160+24 = 208
//	正牆深度 1    96 寬 → 兩側各 56   影格 14／15，56×56   56+ 96+56 = 208
//
// 深度 2 與 3 的正牆更窄（48 與 16），兩側要 80 與 96 —— `TOWN.16` 裡
// 沒有那兩對，看得夠遠的時候露出來的部分由各深度的側牆補。
//
// 位置量自 `shots/diff-shot.png`（(7,5) 面北，正牆是深度 1 的門）：
// 影格 14 在視圖 (0,32)、影格 15 在 (152,32)，都取 95.7%。
// **補牆一律用石牆那一組**，即使正牆是門 —— 同一張截圖裡門版的
// 影格 30／31 在同一個位置只有 62–68%。
var frontFlank = [2][2]int{
	{12, 13},
	{14, 15},
}

// drawFrontFlank 補上正牆兩側。正牆不在深度 0 或 1 時什麼都不做。
func (t *TownSet) drawFrontFlank(s *render.Screen, d, phase int) {
	if d < 0 || d >= len(frontFlank) {
		return
	}
	if im := t.wall(frontFlank[d][0]); im != nil {
		t.blitAt(s, im, FPX)
	}
	if im := t.wall(frontFlank[d][1]); im != nil {
		w, _ := t.size(im)
		t.blitAt(s, im, FPX+FPW-w)
	}
	if d == flankTorchDepth {
		t.blitTorch(s, &flankTorch, FPX+flankTorch.x, phase)
		t.blitTorch(s, &flankTorch, FPX+FPW-flankTorchW, phase)
	}
}

// flankTorchDepth 是唯一有補牆火炬的那一階，flankTorchW 是那張圖的寬。
const (
	flankTorchDepth = 1
	flankTorchW     = 16
)

// blitTorch 貼一盞火炬：底圖加上這一個相位的火焰。
func (t *TownSet) blitTorch(s *render.Screen, sl *torchSlot, x, phase int) {
	base, first := t.torchFrames(sl)
	if first < 0 {
		return
	}
	frame := phase % TorchFrames
	if t.torchStride < TorchFrames {
		// 一格只有一張的平台（MSX）沒有動畫可播，硬加相位會索引到隔壁那盞。
		frame = 0
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
	m := w.CurrentMap()
	if m == nil || t == nil {
		return
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
		lt, rt, ft  bool
	}
	var slots [Depth]slot
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
			lt:    m.HasTorch(x, y, left),
			rt:    m.HasTorch(x, y, right),
			ft:    m.HasTorch(x, y, w.Face),
		}
		last = d
		if slots[d].front != game.WallNone {
			break
		}
		x, y = x+dx, y+dy
	}

	for d := last; d >= 0; d-- {
		if slots[d].l != game.WallNone {
			if i := wallImage(slots[d].l, 4+d); t.wall(i) != nil {
				t.blitSlot(s, i, FPX+sideX[d])
			}
		}
		if slots[d].r != game.WallNone {
			if i := wallImage(slots[d].r, 8+d); t.wall(i) != nil {
				w, _ := t.size(t.wall(i))
				t.blitSlot(s, i, FPX+FPW-sideX[d]-w)
			}
		}
		if slots[d].front != game.WallNone {
			// 補牆要在正牆之前 —— 它們與正牆同高，重疊的部分由正牆蓋掉。
			t.drawFrontFlank(s, d, phase)
			if i := wallImage(slots[d].front, d); t.wall(i) != nil {
				w, _ := t.size(t.wall(i))
				t.blitSlot(s, i, FPX+(FPW-w)/2)
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

// blitAt 把牆垂直置中貼進視圖區 —— 透視消失點在視圖中央。
//
// 牆用**色號 8 當透空色**（見 render.BlitKey）：側牆矩形四角的楔形是
// 「這裡看得到後面」，不是灰色的牆。用一般的 Blit 會在畫面四角留下
// 兩塊灰，而那正是先前對不上原版的地方。
// blitSlot 貼第 i 張牆素材，位置由 wallPos 決定。
func (t *TownSet) blitSlot(s *render.Screen, i, defX int) {
	im := t.wall(i)
	if im == nil {
		return
	}
	x, y := t.wallPos(i, im, defX)
	t.blitKey(s, im, x, y, int(t.clear))
}

func (t *TownSet) blitAt(s *render.Screen, im *image.Paletted, x int) {
	if im == nil {
		return
	}
	_, h := t.size(im)
	t.blitKey(s, im, x, FPY+(FPH-h)/2, int(t.clear))
}

// wallClear 是 DOS 牆貼圖的透空色。各平台的值存在 TownSet.clear。
const wallClear = 8

// 視圖上半是 `SKY.16` 的兩張 208×60 之一，貼在視圖區的左上角。
//
// **這不是程式畫的花紋，是素材。** 影格 0 是白雲藍天（208×60 全不透明）、
// 影格 1 只有一半的像素不透明，露出底色之後就是那個深藍與黑交錯的棋盤 ——
// 先前把它當成「抖動出來的天花板」而用程式重畫，兩者長得一樣但來源不同。
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
