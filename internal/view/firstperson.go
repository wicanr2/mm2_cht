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
	Torch  []gfx.Image // TOWNT.16：火炬動畫，見 torchSlot
	Sky    []gfx.Image // SKY.16：兩張 208×60，見 drawSky
	cached map[int]*image.Paletted
}

// NewTownSet 準備素材並建立解碼快取（每張圖只展開一次）。
func NewTownSet(walls, floor, torch, sky []gfx.Image) *TownSet {
	return &TownSet{Walls: walls, Floor: floor, Torch: torch, Sky: sky,
		cached: map[int]*image.Paletted{}}
}

// 火炬。
//
// `TOWNT.16` 的 36 張排成 **3 組 × 3 個深度 × 4 張**，每個深度的四張是
// 「一張含燈桿的底圖 + 三張只有火焰的動畫」；動畫那三張與底圖共用左上角，
// 只蓋掉上半部，燈桿留著不重畫 —— 原版省重繪的老作法。第三張與底圖的
// 火焰相同（樣板比對兩張都取 100%），所以動畫是「靜、動、動、回靜」。
//
// 三組分別是**左側牆、右側牆、正牆**：前兩組的燈桿斜著畫（牆是斜的），
// 第三組直立。位置全部用 `cmd/mm2match` 把每一張影格滑過 126 張原版截圖
// 定出來，每一張在整批截圖裡都只有唯一一個 100% 的落點：
//
//	組 A 左側牆   影格  0/ 4/ 8   (8,42) (40,56) (64,59)
//	組 B 右側牆   影格 12/16/20   (192,42) (160,56) —
//	組 C 正牆     影格 24/28/32   (104,44) (8,54)+(200,54) (104,60)
//
// 換算成視圖區內座標寫進下面三張表。兩個例外要記著：
//
//   - **右側牆深度 2（影格 20–23）在 126 張截圖裡一次都沒出現。**
//     用左右鏡射補（`FPW − x − w`），這條規則在深度 0 與 1 上都成立。
//   - **正牆深度 1（影格 28–31）量到的落點在視圖最左與最右**，不在中央，
//     出處是神殿與酒館那兩張整幅插畫 —— 那不是走廊算出來的位置。
//     x 照另外兩階的中央對齊補，y 用量到的 46。
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

var torchFront = [Depth - 1]torchSlot{
	{base: 24, first: 25, x: 96, y: 36},
	{base: 28, first: 29, x: 96, y: 46},
	{base: 32, first: 33, x: 96, y: 52},
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

// torchSide 選出某個深度、某一面牆該用哪一格。
func torchSide(d int, side game.Facing, face game.Facing) *torchSlot {
	if d < 0 || d >= Depth-1 {
		return nil
	}
	switch {
	case side == face:
		return &torchFront[d]
	case side == game.Facing((int(face)+3)&3):
		return &torchLeft[d]
	case side == game.Facing((int(face)+1)&3):
		return &torchRight[d]
	}
	return nil
}

// drawTorch 在某個深度的某一面牆上點一盞火炬。phase 是動畫相位。
func (t *TownSet) drawTorch(s *render.Screen, d int, side, face game.Facing, phase int) {
	sl := torchSide(d, side, face)
	if sl == nil {
		return
	}
	if im := t.torch(sl.base); im != nil {
		s.Blit(im, FPX+sl.x, FPY+sl.y)
	}
	if im := t.torch(sl.first + phase%TorchFrames); im != nil {
		s.Blit(im, FPX+sl.x, FPY+sl.y)
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
	t.drawSky(s)
	if len(t.Floor) > 0 {
		s.Blit(t.Floor[0].Paletted(gfx.EGAPalette), FPX, FPY+FPH-t.Floor[0].Height)
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
			blitAt(s, t.wall(wallImage(slots[d].l, 4+d)), FPX+sideX[d])
		}
		if slots[d].r != game.WallNone {
			if im := t.wall(wallImage(slots[d].r, 8+d)); im != nil {
				blitAt(s, im, FPX+FPW-sideX[d]-im.Bounds().Dx())
			}
		}
		if slots[d].front != game.WallNone {
			if im := t.wall(wallImage(slots[d].front, d)); im != nil {
				blitAt(s, im, FPX+(FPW-im.Bounds().Dx())/2)
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

// blitAt 把圖垂直置中貼進視圖區 —— 透視消失點在視圖中央。
func blitAt(s *render.Screen, im *image.Paletted, x int) {
	if im == nil {
		return
	}
	s.Blit(im, x, FPY+(FPH-im.Bounds().Dy())/2)
}


// 視圖上半是 `SKY.16` 的兩張 208×60 之一，貼在視圖區的左上角。
//
// **這不是程式畫的花紋，是素材。** 影格 0 是白雲藍天（208×60 全不透明）、
// 影格 1 只有一半的像素不透明，露出底色之後就是那個深藍與黑交錯的棋盤 ——
// 先前把它當成「抖動出來的天花板」而用程式重畫，兩者長得一樣但來源不同。
//
// 兩張都用樣板比對釘在 `(FPX, FPY)`：`shots/p5.png` 與比對用的
// `diff-shot.png` 命中影格 0，`shots/fpv.png` 與 `22-fpv2.png` 命中影格 1
// （分數低於 100% 是因為牆蓋掉了下半部）。
//
// **哪一張什麼時候用還沒解。** 四張截圖裡兩張各半，而且與「正牆在第幾格」
// 對不起來（`fpv.png` 與 `diff-shot.png` 的正牆都在深度 1，用的卻不同張）。
// 白天黑夜是目前最像的猜測，但沒有證據，所以先固定用影格 0，
// 不編一個看起來合理的規則。
const skyDay = 0

func (t *TownSet) drawSky(s *render.Screen) {
	if len(t.Sky) <= skyDay {
		return
	}
	s.Blit(t.Sky[skyDay].Paletted(gfx.EGAPalette), FPX, FPY)
}
