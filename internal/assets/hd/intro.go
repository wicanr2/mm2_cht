package hd

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
)

// The Amiga clip locations are registered against matching pixels in intro.32.
// Timing is a remake arrangement of the DOS action sequence, not Amiga parity.
var introSpots = [...]image.Point{{166, 164}, {166, 164}, {32, 124}, {167, 155}, {262, 73}, {262, 73}, {247, 107}, {223, 57}, {243, 129}, {243, 129}, {92, 96}}
var introActions = [...]int{-1, 0, 1, 0, 2, 4, 1, -2, 5, 0, 4, 1, -3, 0, 1, 0, 2, 6, 1, -3, 0, -2, 1, 0, 7, 1, -3, 0, 2, 1, 0, -2, 8, 1, 9, 0, -3, 1, 0, 2, 10, 1, 0, -2, -4, 1, 3}
var introGroups = [...]int{0, 0, 1, 2, 3, 3, 4, 5, 6, 6, 7}

// DrawIntro reconstructs every active layer from the background, so restored
// poses never leave trails. Soft patch edges join the independently painted art.
func (b *Bundle) DrawIntro(dst *image.RGBA, tick int) bool {
	if b == nil || b.entries["intro-000"] == nil {
		return false
	}
	draw.Draw(dst, dst.Bounds(), b.entries["intro-000"], image.Point{}, draw.Src)
	if tick < 0 {
		tick = -tick
	}
	step := (tick * 2 / 9) % len(introActions)
	layers := [8]int{-1, -1, -1, -1, -1, -1, -1, -1}
	for _, n := range introActions[:step+1] {
		switch n {
		case -1:
			layers = [8]int{-1, -1, -1, -1, -1, -1, -1, -1}
		case -2:
			layers[1] = -1
		case -3:
			for i := 3; i <= 6; i++ {
				layers[i] = -1
			}
		case -4:
			layers[7] = -1
		default:
			layers[introGroups[n]] = n
		}
	}
	for _, n := range layers {
		if n < 0 {
			continue
		}
		// The painted background has continuous hair and fur. Animate those
		// surfaces in place instead of exposing a rectangular replacement patch.
		if n == 2 {
			b.warpIntro(dst, image.Rect(90, 330, 210, 546), math.Sin(float64(tick)*math.Pi/9)*4, false)
			continue
		}
		if n == 0 || n == 1 {
			amount := 1.5
			if n == 1 {
				amount = -1.5
			}
			b.warpIntro(dst, image.Rect(490, 480, 566, 550), amount, true)
			continue
		}
		art := b.entries[fmt.Sprintf("introclips-%03d", n)]
		if art == nil {
			continue
		}
		bounds := art.Bounds()
		if b.introMasks[n] == nil {
			mask := image.NewAlpha(bounds)
			for y := 0; y < bounds.Dy(); y++ {
				for x := 0; x < bounds.Dx(); x++ {
					edge := min(x, y, bounds.Dx()-1-x, bounds.Dy()-1-y)
					mask.SetAlpha(x, y, color.Alpha{A: uint8(min(255, edge*255/9))})
				}
			}
			b.introMasks[n] = mask
		}
		p := introSpots[n].Mul(3)
		draw.DrawMask(dst, bounds.Add(p), art, bounds.Min, b.introMasks[n], bounds.Min, draw.Over)
	}
	return true
}

func (b *Bundle) warpIntro(dst *image.RGBA, box image.Rectangle, amount float64, vertical bool) {
	base := b.entries["intro-000"]
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			weight := math.Sin(math.Pi*float64(x-box.Min.X)/float64(box.Dx())) * math.Sin(math.Pi*float64(y-box.Min.Y)/float64(box.Dy()))
			d := int(math.Round(amount * weight))
			sx, sy := x, y
			if vertical {
				sy += d
			} else {
				sx += d
			}
			dst.Set(x, y, base.At(sx, sy))
		}
	}
}
