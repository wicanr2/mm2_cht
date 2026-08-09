// mm2mon 把怪物圖畫成 PNG，供肉眼驗收。
//
// 不需要視窗與 GPU。三種模式：
//
//	go run ./cmd/mm2mon -sheet            59 個槽的基準圖排成一張
//	go run ./cmd/mm2mon -pic 1 -anim 0    單一圖號的動畫，一步一張
//	go run ./cmd/mm2mon -fight 5 -count 3 戰鬥畫面：一排怪物疊在視圖區
//
// `-pic` 收的是**怪物記錄裡的圖號**（1–60），不是索引表的槽號 ——
// 指到空槽時會照原版往後借（見 docs/formats/04 §2.1）。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/view"
)

func main() {
	dataDir := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	outDir := flag.String("out", "workplace/gfx/mon", "PNG 輸出目錄")
	sheet := flag.Bool("sheet", false, "輸出 59 個槽的基準圖總覽")
	pic := flag.Int("pic", 0, "怪物圖號（1–60），輸出它的影格")
	anim := flag.Int("anim", -1, "動畫序列編號，配合 -pic 使用")
	fight := flag.Int("fight", -1, "怪物編號（0–255），輸出戰鬥畫面")
	count := flag.Int("count", 3, "戰鬥畫面上的怪物數量")
	flag.Parse()

	blob, err := os.ReadFile(filepath.Join(*dataDir, "MONSTERS.16"))
	if err != nil {
		log.Fatal(err)
	}
	index, err := gfx.MonsterIndex(blob)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	switch {
	case *sheet:
		writeSheet(blob, index, *outDir)
	case *pic > 0:
		writePic(blob, index, *pic, *anim, *outDir)
	case *fight >= 0:
		writeFight(blob, index, *dataDir, *fight, *count, *outDir)
	default:
		flag.Usage()
		os.Exit(2)
	}
}

// writeSheet 把每個非空槽的基準圖排成一張 10 欄的總覽。
func writeSheet(blob []byte, index []int, outDir string) {
	const cw, ch, cols = 84, 86, 10
	var pics []gfx.MonsterPic
	for slot, v := range index {
		if v == 0 {
			continue
		}
		p, err := gfx.ParseMonsterPic(blob, slot)
		if err != nil {
			log.Fatal(err)
		}
		pics = append(pics, p)
	}
	rows := (len(pics) + cols - 1) / cols
	dst := image.NewPaletted(image.Rect(0, 0, cols*cw, rows*ch), gfx.EGAPalette)
	for i := range dst.Pix {
		dst.Pix[i] = gfx.TransparentIndex
	}
	for i, p := range pics {
		ox, oy := (i%cols)*cw, (i/cols)*ch
		f := p.Frames[0]
		for y := 0; y < f.Height && y < ch; y++ {
			for x := 0; x < f.Width && x < cw; x++ {
				dst.SetColorIndex(ox+x, oy+y, f.At(x, y))
			}
		}
	}
	save(filepath.Join(outDir, "sheet.png"), dst)
	fmt.Printf("%d 個槽寫進 %s/sheet.png\n", len(pics), outDir)
}

// writePic 輸出一個圖號的全部影格；給了 -anim 就照那一段動畫逐步輸出。
func writePic(blob []byte, index []int, pic, anim int, outDir string) {
	slot := gfx.ResolveMonsterPic(index, pic)
	if slot < 0 {
		log.Fatalf("圖號 %d 解不到槽", pic)
	}
	p, err := gfx.ParseMonsterPic(blob, slot)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("圖號 %d → 槽 %d：%d 個影格、%d 段動畫\n",
		pic, slot, len(p.Frames), len(p.Anims))
	if anim < 0 {
		for i, f := range p.Frames {
			save(filepath.Join(outDir, fmt.Sprintf("pic%02d_f%02d.png", pic, i)), frameImage(f))
		}
		return
	}
	if anim >= len(p.Anims) {
		log.Fatalf("槽 %d 只有 %d 段動畫", slot, len(p.Anims))
	}
	for step := range p.Anims[anim] {
		s := render.New(gfx.EGAPalette)
		s.Clear(gfx.TransparentIndex)
		view.DrawMonsters(s, []view.MonsterSprite{{Pic: p, Anim: anim, Step: step}})
		save(filepath.Join(outDir, fmt.Sprintf("pic%02d_a%d_s%02d.png", pic, anim, step)), s.Orig)
	}
	fmt.Printf("動畫 %d 共 %d 步\n", anim, len(p.Anims[anim]))
}

// writeFight 用真的怪物編號組一排，畫成戰鬥畫面。
func writeFight(blob []byte, index []int, dataDir string, first, count int, outDir string) {
	b, err := os.ReadFile(filepath.Join(dataDir, "MONSTERS.DAT"))
	if err != nil {
		log.Fatal(err)
	}
	defs, err := monsters.Parse(b)
	if err != nil {
		log.Fatal(err)
	}
	var sprites []view.MonsterSprite
	for i := 0; i < count && first+i < len(defs); i++ {
		d := defs[first+i]
		slot := gfx.ResolveMonsterPic(index, d.Sprite)
		p, err := gfx.ParseMonsterPic(blob, slot)
		if err != nil {
			log.Fatal(err)
		}
		sprites = append(sprites, view.MonsterSprite{Pic: p, Anim: -1})
		fmt.Printf("  %-16s 圖號 %2d → 槽 %2d\n", d.Name, d.Sprite, slot)
	}
	s := render.New(gfx.EGAPalette)
	s.Clear(0)
	view.DrawMonsters(s, sprites)
	save(filepath.Join(outDir, fmt.Sprintf("fight%03d.png", first)), s.Orig)
	fmt.Printf("寫進 %s/fight%03d.png\n", outDir, first)
}

func frameImage(f gfx.Frame) *image.Paletted {
	dst := image.NewPaletted(image.Rect(0, 0, f.Width, f.Height), gfx.EGAPalette)
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			dst.SetColorIndex(x, y, f.At(x, y))
		}
	}
	return dst
}

func save(path string, im image.Image) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, im); err != nil {
		log.Fatal(err)
	}
}
