package render

import "image"

// Scale3x 把索引色影像放大三倍，沿著色塊邊界補出斜角。
//
// 為什麼是三倍而不是別的倍率：整個畫布的放大倍率就是 `Scale = 3`
// （見 screen.go），所以 Scale3x 的輸出**剛好**填滿高解析層對應的區域，
// 不需要再縮放一次。任何「先放大 N 倍再縮到 3 倍」的做法都會把
// 剛補出來的邊界重新糊掉。
//
// 演算法是 AdvMAME3x：只看 3×3 鄰域，兩個對角相同而另一對不同時，
// 把角落換成那個對角的顏色。它**不產生新顏色**，所以輸出仍是索引色，
// 透明色（牆面的色號 8）不會被混成半透明的雜色 —— 這是選它而不選
// 雙線性內插的原因，不是因為它比較快。
//
// 邊界的鄰居取最近的邊緣像素（clamp）。用環繞或補透明都會讓圖的四周
// 長出一圈假邊：牆面是整塊貼上去的，那一圈會變成看得見的框。
func Scale3x(src *image.Paletted) *image.Paletted {
	if src == nil {
		return nil
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewPaletted(image.Rect(0, 0, w*3, h*3), src.Palette)

	at := func(x, y int) uint8 {
		if x < 0 {
			x = 0
		} else if x >= w {
			x = w - 1
		}
		if y < 0 {
			y = 0
		} else if y >= h {
			y = h - 1
		}
		return src.ColorIndexAt(b.Min.X+x, b.Min.Y+y)
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a, bb, c := at(x-1, y-1), at(x, y-1), at(x+1, y-1)
			d, e, f := at(x-1, y), at(x, y), at(x+1, y)
			g, hh, i := at(x-1, y+1), at(x, y+1), at(x+1, y+1)

			var o [9]uint8
			for k := range o {
				o[k] = e
			}
			// 四個角：一組對角相同、另一組不同，就補成那個對角的顏色。
			db := d == bb && d != hh && bb != f
			bf := bb == f && bb != d && f != hh
			hd := hh == d && hh != f && d != bb
			fh := f == hh && f != bb && hh != d
			if db {
				o[0] = d
			}
			if bf {
				o[2] = f
			}
			if hd {
				o[6] = d
			}
			if fh {
				o[8] = f
			}
			// 四個邊：兩側的角都想擴張時才填，否則細線會被抹粗一格。
			if (db && e != c) || (bf && e != a) {
				o[1] = bb
			}
			if (hd && e != a) || (db && e != g) {
				o[3] = d
			}
			if (bf && e != i) || (fh && e != c) {
				o[5] = f
			}
			if (fh && e != g) || (hd && e != i) {
				o[7] = hh
			}

			for k, v := range o {
				dst.SetColorIndex(x*3+k%3, y*3+k/3, v)
			}
		}
	}
	return dst
}
