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

// TownSet 是城鎮視角需要的三組素材。
type TownSet struct {
	Walls  []gfx.Image // TOWN.16：0-3 正牆、4-7 左側牆、8-11 右側牆
	Floor  []gfx.Image // TOWNF.16：208×60 的地板
	Torch  []gfx.Image // TOWNT.16：火炬動畫，見 torchSlots
	Pal    []gfx.Image // 保留：地形貼圖
	cached map[int]*image.Paletted
}

// NewTownSet 準備素材並建立解碼快取（每張圖只展開一次）。
func NewTownSet(walls, floor, torch []gfx.Image) *TownSet {
	return &TownSet{Walls: walls, Floor: floor, Torch: torch,
		cached: map[int]*image.Paletted{}}
}

// 火炬。
//
// `TOWNT.16` 的 36 張排成 **3 組 × 3 個深度 × 4 張**，每個深度的四張是
// 「一張含燈桿的底圖 + 三張只有火焰的動畫」；動畫那三張與底圖共用左上角，
// 只蓋掉上半部，燈桿留著不重畫 —— 原版省重繪的老作法。
//
// 位置是樣板比對回原版截圖定出來的（都取 96% 以上）：
//
//	影格 1（24×31）在 shots/22-fpv2.png 的 (8, 42)   → 深度 0 左
//	影格 5（24×13）在 shots/fpv.png     的 (40, 56)  → 深度 1 左
//	影格 17（24×13）在 shots/fpv.png    的 (160, 56) → 深度 1 右
//
// 換成視圖區內座標之後規律很乾淨：**火炬貼在該深度側牆的內側邊緣**
// （左牆靠右、右牆靠左），y 只跟深度有關。深度 2 那一組（影格 8–11、
// 20–23）在現有的截圖裡都沒出現，位置還沒驗證，所以不畫 ——
// 要驗證就找一張深度 2 有側牆的畫面重跑樣板比對。
//
// 第三組（影格 24–35，尺寸與前兩組不同）用途未解。
type torchSlot struct {
	base, first int // 底圖與第一張火焰的影格編號
	y           int // 視圖區內的 y
	w           int // 圖寬，用來算內側邊緣
}

var torchLeft = [2]torchSlot{
	{base: 0, first: 1, y: 34, w: 24},  // 深度 0
	{base: 4, first: 5, y: 48, w: 24},  // 深度 1
}

var torchRight = [2]torchSlot{
	{base: 12, first: 13, y: 34, w: 24},
	{base: 16, first: 17, y: 48, w: 24},
}

// TorchFrames 是火焰動畫的張數。
const TorchFrames = 3

func (t *TownSet) torch(i int) *image.Paletted {
	if i < 0 || i >= len(t.Torch) {
		return nil
	}
	key := 1000 + i
	if im, ok := t.cached[key]; ok {
		return im
	}
	im := t.Torch[i].Paletted(gfx.EGAPalette)
	t.cached[key] = im
	return im
}

// drawTorch 在某個深度的側牆上點一盞火炬。phase 是動畫相位。
func (t *TownSet) drawTorch(s *render.Screen, d int, left bool, phase int) {
	tab := &torchRight
	if left {
		tab = &torchLeft
	}
	if d < 0 || d >= len(tab) {
		return
	}
	sl := tab[d]
	// 內側邊緣：左牆的右緣、右牆的左緣。
	x := FPX + sideX[d+1] - sl.w
	if !left {
		x = FPX + FPW - sideX[d+1]
	}
	if im := t.torch(sl.base); im != nil {
		s.Blit(im, x, FPY+sl.y)
	}
	if im := t.torch(sl.first + phase%TorchFrames); im != nil {
		s.Blit(im, x, FPY+sl.y)
	}
}

func (t *TownSet) wall(i int) *image.Paletted {
	if i < 0 || i >= len(t.Walls) {
		return nil
	}
	if im, ok := t.cached[i]; ok {
		return im
	}
	im := t.Walls[i].Paletted(gfx.EGAPalette)
	t.cached[i] = im
	return im
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
	drawCeiling(s)
	if len(t.Floor) > 0 {
		s.Blit(t.Floor[0].Paletted(gfx.EGAPalette), FPX, FPY+FPH-t.Floor[0].Height)
	}

	left := game.Facing((int(w.Face) + 3) & 3)
	right := game.Facing((int(w.Face) + 1) & 3)
	dx, dy := w.Face.Delta()

	// 先走一趟決定每個深度要畫什麼，再由遠到近貼，近的才會蓋住遠的。
	type slot struct{ l, r, front bool }
	var slots [Depth]slot
	last := -1
	x, y := w.X, w.Y
	for d := 0; d < Depth; d++ {
		if game.Cell(x, y) < 0 {
			break
		}
		slots[d] = slot{
			l:     m.HasWall(x, y, left),
			r:     m.HasWall(x, y, right),
			front: m.HasWall(x, y, w.Face),
		}
		last = d
		if slots[d].front {
			break
		}
		x, y = x+dx, y+dy
	}

	for d := last; d >= 0; d-- {
		if slots[d].l {
			blitAt(s, t.wall(4+d), FPX+sideX[d])
		}
		if slots[d].r {
			if im := t.wall(8 + d); im != nil {
				blitAt(s, im, FPX+FPW-sideX[d]-im.Bounds().Dx())
			}
		}
		if slots[d].front {
			if im := t.wall(d); im != nil {
				blitAt(s, im, FPX+(FPW-im.Bounds().Dx())/2)
			}
		}
		// 火炬畫在側牆上，所以要在側牆之後、下一個更近的深度之前。
		if slots[d].l {
			t.drawTorch(s, d, true, phase)
		}
		if slots[d].r {
			t.drawTorch(s, d, false, phase)
		}
	}
}

// blitAt 把圖垂直置中貼進視圖區 —— 透視消失點在視圖中央。
func blitAt(s *render.Screen, im *image.Paletted, x int) {
	if im == nil {
		return
	}
	s.Blit(im, x, FPY+(FPH-im.Bounds().Dy())/2)
}


// 天花板不是素材，是**程式畫的抖動花紋**。
//
// 量自原版截圖（`shots/fpv.png` 的 y 8–21，208 px 寬）：黑與藍各 1,456 個
// 像素、剛好一半一半，排成**橫向兩格一組、逐列交錯**的棋盤：
//
//	y 偶數  BB..BB..BB..
//	y 奇數  ..BB..BB..BB
//
// 所以規則是 `((x−FPX)/2 + (y−FPY)) 為偶數就塗藍`。先鋪滿整個視圖區，
// 地板與牆再蓋上去 —— 沒被蓋到的地方就是天花板，深度不同露出來的高度
// 自然就不同（`22-fpv2.png` 只露最上面幾列）。
//
// 顏色是 EGA 的 0（黑）與 1（藍）。城堡與地城用的是另一組貼圖，
// 花紋一不一樣還沒對過截圖，所以目前三種場景都畫這一組。
const (
	ceilDark  = 0 // EGA 黑
	ceilLight = 1 // EGA 藍
	ceilCell  = 2 // 橫向兩格一組
)

func drawCeiling(s *render.Screen) {
	for y := 0; y < FPH; y++ {
		for x := 0; x < FPW; x++ {
			idx := uint8(ceilDark)
			if (x/ceilCell+y)%2 == 0 {
				idx = ceilLight
			}
			s.Orig.SetColorIndex(FPX+x, FPY+y, idx)
		}
	}
}
