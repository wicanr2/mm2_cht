// mm2match 把 `.16` 裡的每一張影格拿去原版截圖上滑動比對，找出它被貼在哪。
//
//	go run ./cmd/mm2match -set workplace/orig/MM2/TOWNT.16 \
//	    workplace/dosbox/shots/*.png
//
// 這是「素材位置」這一類問題的通用答案：知道某張圖長什麼樣、但不知道
// 引擎把它貼在畫面的哪裡時，用截圖反推比讀 blit 組語快得多 ——
// 側牆、火炬、地板的座標都是這樣定出來的。
//
// 比對規則：
//
//   - 只比**非透明像素**。原版的貼圖用色號 0 當透空，整張算進去的話
//     背景會主導分數，位置對了反而不像。
//   - 分數 = 相符的非透明像素 / 非透明像素總數。
//   - 截圖是 320×200 的 EGA 畫面，色號用最近的調色盤項回推。
//
// 只印超過門檻的結果；一張都沒中也要印出來，「沒有命中」與「沒有跑」
// 長得不一樣才有診斷價值。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
)

func main() {
	set := flag.String("set", "", "要比對的 .16 檔")
	min := flag.Float64("min", 0.90, "分數門檻")
	only := flag.Int("frame", -1, "只比對這一張影格（預設全部）")
	minPix := flag.Int("minpix", 40, "非透明像素少於這個數就跳過（太小的圖到處都能中）")
	region := flag.String("region", "5,5,212,126", "搜尋區 x,y,w,h；預設是第一人稱視圖那一格")
	flag.Parse()

	var rx, ry, rw, rh int
	if _, err := fmt.Sscanf(*region, "%d,%d,%d,%d", &rx, &ry, &rw, &rh); err != nil {
		log.Fatalf("-region 要寫成 x,y,w,h：%v", err)
	}
	rc := image.Rect(rx, ry, rx+rw, ry+rh)

	if *set == "" || flag.NArg() == 0 {
		log.Fatal("用法：mm2match -set X.16 shot1.png shot2.png ...")
	}
	blob, err := os.ReadFile(*set)
	if err != nil {
		log.Fatal(err)
	}
	imgs, err := gfx.ParseSet(blob)
	if err != nil {
		log.Fatal(err)
	}

	type hit struct {
		frame, x, y, n int
		score          float64
		shot           string
	}
	var hits []hit
	nshot := 0

	for _, path := range flag.Args() {
		scr, err := loadIndexed(path)
		if err != nil {
			continue // 目錄裡混著 .bin，不是錯誤
		}
		nshot++
		for i, im := range imgs {
			if *only >= 0 && i != *only {
				continue
			}
			tpl := im.Paletted(gfx.EGAPalette)
			bx, by, best, n := match(scr, tpl, rc, *min)
			if n < *minPix || best < *min {
				continue
			}
			hits = append(hits, hit{i, bx, by, n, best, filepath.Base(path)})
		}
	}

	sort.Slice(hits, func(a, b int) bool {
		if hits[a].frame != hits[b].frame {
			return hits[a].frame < hits[b].frame
		}
		return hits[a].score > hits[b].score
	})
	fmt.Printf("%s：%d 張影格 × %d 張截圖，門檻 %.0f%%\n",
		filepath.Base(*set), len(imgs), nshot, *min*100)
	if len(hits) == 0 {
		fmt.Println("沒有任何影格命中。")
		return
	}
	for _, h := range hits {
		fmt.Printf("影格 %2d  %-14s (%3d,%3d)  %.1f%%  %d 個像素\n",
			h.frame, h.shot, h.x, h.y, h.score*100, h.n)
	}
}

// match 讓樣板在搜尋區內滑一遍，回傳最佳位置、分數與非透明像素數。
//
// 逐位置全比會炸：一張 24×43 的樣板在 320×200 上有四萬多個落點，
// 每個落點要比一千個像素，乘上 36 張影格與上百張截圖就是幾百億次。
// 兩個手段把它壓回可跑的範圍 ——
//
//   - **限定搜尋區**（`-region`）。找第一人稱視圖裡的東西不必掃訊息框。
//   - **提早放棄**。錯的落點通常前幾個像素就錯了，累積的不符數一旦
//     多到分數不可能超過門檻就跳掉，實測省掉九成九以上的內圈。
//
// 提早放棄用的是 `min` 而不是「目前最佳」，所以結果與全比一致：
// 被砍掉的落點本來就不會印出來。
func match(scr, tpl *image.Paletted, rc image.Rectangle, min float64) (bx, by int, best float64, n int) {
	tw, th := tpl.Bounds().Dx(), tpl.Bounds().Dy()
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if tpl.ColorIndexAt(x, y) != 0 {
				n++
			}
		}
	}
	rc = rc.Intersect(scr.Bounds())
	if n == 0 || tw > rc.Dx() || th > rc.Dy() {
		return 0, 0, 0, n
	}
	// 允許的不符上限：超過這個數，分數必定低於門檻。
	maxBad := n - int(float64(n)*min)
	bx, by = -1, -1
	for oy := rc.Min.Y; oy+th <= rc.Max.Y; oy++ {
		for ox := rc.Min.X; ox+tw <= rc.Max.X; ox++ {
			bad, same := 0, 0
			for y := 0; y < th && bad <= maxBad; y++ {
				for x := 0; x < tw; x++ {
					c := tpl.ColorIndexAt(x, y)
					if c == 0 {
						continue
					}
					if scr.ColorIndexAt(ox+x, oy+y) == c {
						same++
					} else if bad++; bad > maxBad {
						break
					}
				}
			}
			if bad > maxBad {
				continue
			}
			if s := float64(same) / float64(n); s > best {
				best, bx, by = s, ox, oy
			}
		}
	}
	return bx, by, best, n
}

// loadIndexed 讀 PNG 並把每個像素回推成 EGA 色號。
func loadIndexed(path string) (*image.Paletted, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	dst := image.NewPaletted(image.Rect(0, 0, b.Dx(), b.Dy()), gfx.EGAPalette)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bb, _ := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			dst.SetColorIndex(x, y, uint8(nearest(color.RGBA{
				uint8(r >> 8), uint8(g >> 8), uint8(bb >> 8), 0xFF})))
		}
	}
	return dst, nil
}

func nearest(c color.RGBA) int {
	bestI, bestD := 0, 1<<30
	for i, p := range gfx.EGAPalette {
		r, g, b, _ := p.RGBA()
		dr, dg, db := int(r>>8)-int(c.R), int(g>>8)-int(c.G), int(b>>8)-int(c.B)
		if d := dr*dr + dg*dg + db*db; d < bestD {
			bestI, bestD = i, d
		}
	}
	return bestI
}
