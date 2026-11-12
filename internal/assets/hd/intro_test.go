package hd

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestIntroRestoresPreviousLayers(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 960, 600))
	draw.Draw(base, base.Bounds(), image.NewUniform(color.RGBA{10, 20, 30, 255}), image.Point{}, draw.Src)
	tail := image.NewRGBA(image.Rect(0, 0, 144, 108))
	draw.Draw(tail, tail.Bounds(), image.NewUniform(color.RGBA{240, 20, 30, 255}), image.Point{}, draw.Src)
	b := &Bundle{entries: map[string]image.Image{"intro-000": base, "introclips-004": tail}}
	dst := image.NewRGBA(base.Bounds())
	p := introSpots[4].Mul(3).Add(image.Pt(30, 30))
	b.DrawIntro(dst, 23) // step5: pointing figure
	if dst.RGBAAt(p.X, p.Y) == base.RGBAAt(p.X, p.Y) {
		t.Fatal("figure did not animate")
	}
	b.DrawIntro(dst, 54) // step12: restore tree
	if dst.RGBAAt(p.X, p.Y) != base.RGBAAt(p.X, p.Y) {
		t.Fatal("animation left a trail")
	}
	seen := map[int]bool{}
	for _, n := range introActions {
		if n >= 0 {
			seen[n] = true
		}
	}
	if len(seen) != 11 {
		t.Fatalf("only %d of11 source poses are used", len(seen))
	}
}
