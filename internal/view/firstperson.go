package view

import (
	"image"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 第一人稱視圖區的位置與大小（量自原版截圖，見 docs/playtest/01）。
const (
	FPX, FPY = 0, 0
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
	Torch  []gfx.Image // TOWNT.16：火炬動畫
	Pal    []gfx.Image // 保留：地形貼圖
	cached map[int]*image.Paletted
}

// NewTownSet 準備素材並建立解碼快取（每張圖只展開一次）。
func NewTownSet(walls, floor, torch []gfx.Image) *TownSet {
	return &TownSet{Walls: walls, Floor: floor, Torch: torch,
		cached: map[int]*image.Paletted{}}
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
	m := w.CurrentMap()
	if m == nil || t == nil {
		return
	}
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
	}
}

// blitAt 把圖垂直置中貼進視圖區 —— 透視消失點在視圖中央。
func blitAt(s *render.Screen, im *image.Paletted, x int) {
	if im == nil {
		return
	}
	s.Blit(im, x, FPY+(FPH-im.Bounds().Dy())/2)
}
