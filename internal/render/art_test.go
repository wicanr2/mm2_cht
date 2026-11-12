package render

import (
	"image"
	"image/color"
	"testing"
)

func TestRGBAArtKeepsOriginalMaskAndQueueOrder(t *testing.T) {
	pal := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}}
	src := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	src.SetColorIndex(0, 0, 1)
	door := image.NewPaletted(src.Bounds(), pal)
	door.SetColorIndex(1, 0, 1)
	mask := SourceMask(src, door, 0)
	art := image.NewRGBA(mask.Bounds())
	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			art.SetRGBA(x, y, color.RGBA{200, 80, 40, 255})
		}
	}
	s := New(pal)
	s.Clear(1)
	s.BlitHiArt(art, mask, 4, 5)
	s.Flush()
	if got := s.Hi.RGBAAt(16, 16); got != (color.RGBA{200, 80, 40, 255}) {
		t.Fatalf("door's opaque interior lost: %v", got)
	}
	if got := s.Hi.RGBAAt(12, 19); got != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("transparent source overwrote background: %v", got)
	}
	s.Clear(0)
	s.Flush()
	if s.Hi.RGBAAt(12, 15) != (color.RGBA{0, 0, 0, 255}) {
		t.Fatal("art queue leaked into next frame")
	}
}
