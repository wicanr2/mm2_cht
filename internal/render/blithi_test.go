package render_test

import (
	"image"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/render"
)

func solid(idx uint8, w, h int) *image.Paletted {
	im := image.NewPaletted(image.Rect(0, 0, w, h), pal4())
	for i := range im.Pix {
		im.Pix[i] = idx
	}
	return im
}

// 高解析貼圖要蓋在 Flush 的結果上。
//
// 這是整個機制唯一會出錯的地方：順序反了畫面照樣出得來，只是那一塊
// 變回馬賽克 —— 不會有錯誤訊息，看起來像「放大沒生效」。
func TestBlitHiCoversFlush(t *testing.T) {
	s := render.New(pal4())
	s.Clear(0)
	s.BlitHi(solid(1, 3, 3), 5, 7) // 一個原版像素大小的高解析貼圖
	s.Flush()

	if got := s.Hi.RGBAAt(5*render.Scale, 7*render.Scale); got != pal4()[1] {
		t.Fatalf("貼圖沒蓋上去，(%d,%d) 是 %v", 5*render.Scale, 7*render.Scale, got)
	}
	if got := s.Hi.RGBAAt(0, 0); got != pal4()[0] {
		t.Fatalf("貼圖溢出到 (0,0)：%v", got)
	}
}

// 佇列每次 Flush 之後要清空，否則下一影格會重畫上一影格的東西。
func TestBlitHiQueueClears(t *testing.T) {
	s := render.New(pal4())
	s.Clear(0)
	s.BlitHi(solid(1, 3, 3), 5, 7)
	s.Flush()

	s.Clear(2)
	s.Flush()
	if got := s.Hi.RGBAAt(5*render.Scale, 7*render.Scale); got != pal4()[2] {
		t.Fatalf("上一影格的貼圖留下來了：%v", got)
	}
}

// 透空色要跳過，露出底下 Flush 的結果。
func TestBlitHiKeySkips(t *testing.T) {
	s := render.New(pal4())
	s.Clear(3)
	im := solid(1, 3, 3)
	im.SetColorIndex(0, 0, 2) // 這一格是透空色
	s.BlitHiKey(im, 4, 4, 2)
	s.Flush()

	if got := s.Hi.RGBAAt(4*render.Scale, 4*render.Scale); got != pal4()[3] {
		t.Fatalf("透空色沒被跳過：%v", got)
	}
	if got := s.Hi.RGBAAt(4*render.Scale+1, 4*render.Scale); got != pal4()[1] {
		t.Fatalf("非透空的像素沒畫上去：%v", got)
	}
}

// 座標用原版座標，倍率由 Screen 換算 —— 呼叫端不必知道 Scale。
func TestBlitHiUsesOrigCoordinates(t *testing.T) {
	s := render.New(pal4())
	s.Clear(0)
	s.BlitHi(solid(1, render.Scale, render.Scale), 10, 20)
	s.Flush()

	for _, p := range [][2]int{{10*render.Scale - 1, 20 * render.Scale},
		{10 * render.Scale, 20*render.Scale - 1}} {
		if got := s.Hi.RGBAAt(p[0], p[1]); got != pal4()[0] {
			t.Fatalf("(%d,%d) 不該被畫到：%v", p[0], p[1], got)
		}
	}
	if got := s.Hi.RGBAAt(10*render.Scale, 20*render.Scale); got != pal4()[1] {
		t.Fatalf("左上角沒對齊原版座標 ×Scale：%v", got)
	}
}
