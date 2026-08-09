package view

import (
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 戰鬥畫面：怪物圖疊在第一人稱視圖區上。
//
// **像素與影格已證實**（docs/formats/04 §2）：圖號 → 槽號的回退規則抄自
// `sub_6818`，RLE 逐張解出剛好 w×h，59 張基準圖 render 出來可辨識。
//
// **版面還沒有 oracle。** 原版怎麼把幾隻怪物排在視圖區裡、有沒有依距離
// 縮放、活著與死掉怎麼區分，要有戰鬥畫面的截圖才能定案。這裡先用
// 「由中央往兩側等距排、底邊貼齊視圖底部」，並在文件裡標成未驗證。
// 之後拿到截圖就照 `docs/formats/04` §3 的反推法定位，不要繼續猜。

// MonsterSprite 是畫面上的一隻怪物。
type MonsterSprite struct {
	Pic gfx.MonsterPic
	// Anim 是要播的動畫序列編號，Step 是序列裡的第幾步。
	// 兩者都超出範圍時只畫基準圖。
	Anim, Step int
	// Dead 為真時整隻用灰階畫，讓「還站著的」一眼看得出來。
	Dead bool
}

// DrawMonsters 把一排怪物畫進第一人稱視圖區。
//
// 由中央往外排：一隻置中，兩隻對稱分在兩側，依此類推。超出視圖區的
// 部分會被裁掉而不是折行 —— 原版的視圖區就這麼寬。
func DrawMonsters(s *render.Screen, ms []MonsterSprite) {
	if len(ms) == 0 {
		return
	}
	slot := FPW / len(ms)
	for i := range ms {
		cx := FPX + slot*i + slot/2
		drawMonster(s, &ms[i], cx, FPY+FPH)
	}
}

// drawMonster 把一隻怪物畫在 (cx, bottom)：水平置中、底邊對齊。
//
// 先鋪基準圖（影格 0），再疊上目前動畫步的影格。動畫零件自己帶
// x/y 偏移，那個偏移是相對基準圖左上角的，不是相對畫面。
func drawMonster(s *render.Screen, m *MonsterSprite, cx, bottom int) {
	if len(m.Pic.Frames) == 0 {
		return
	}
	base := m.Pic.Frames[0]
	ox := cx - base.Width/2
	oy := bottom - base.Height
	blitFrame(s, base, ox, oy, m.Dead)

	f := m.currentFrame()
	if f == nil {
		return
	}
	blitFrame(s, *f, ox+f.X, oy+f.Y, m.Dead)
}

// currentFrame 回傳目前動畫步要疊的影格，沒有就回 nil。
func (m *MonsterSprite) currentFrame() *gfx.Frame {
	if m.Anim < 0 || m.Anim >= len(m.Pic.Anims) {
		return nil
	}
	seq := m.Pic.Anims[m.Anim]
	if m.Step < 0 || m.Step >= len(seq) {
		return nil
	}
	i := seq[m.Step].Frame
	if i <= 0 || i >= len(m.Pic.Frames) {
		return nil // 0 是基準圖，已經畫過了
	}
	return &m.Pic.Frames[i]
}

// blitFrame 把一個影格畫上去，跳過透明色。
func blitFrame(s *render.Screen, f gfx.Frame, x, y int, dim bool) {
	for sy := 0; sy < f.Height; sy++ {
		dy := y + sy
		if dy < FPY || dy >= FPY+FPH || dy >= render.OrigH {
			continue
		}
		for sx := 0; sx < f.Width; sx++ {
			dx := x + sx
			if dx < FPX || dx >= FPX+FPW || dx >= render.OrigW {
				continue
			}
			c := f.At(sx, sy)
			if c == gfx.TransparentIndex {
				continue
			}
			if dim {
				c = grey(c)
			}
			s.Orig.SetColorIndex(dx, dy, c)
		}
	}
}

// grey 把顏色換成 EGA 的三階灰（0 黑、8 深灰、7 淺灰、15 白）。
// 死掉的怪物用它畫，不必另備一套素材。
func grey(c byte) byte {
	switch {
	case c == 0:
		return 0
	case c < 8:
		return 8
	case c < 15:
		return 7
	default:
		return 15
	}
}
