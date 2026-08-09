// mm2dump 把原版素材解出來輸出成 PNG，不需要視窗。
//
// 用途是讓渲染管線在沒有 GPU 的環境也能驗收 —— 編譯成功不算視覺測試，
// 要有畫面能看。
//
//	go run ./cmd/mm2dump -data workplace/orig/MM2 -out workplace/gfx/go title
//	go run ./cmd/mm2dump -data workplace/orig/MM2 -out workplace/gfx/go sheet TOWNT.16
package main

import (
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/font"
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/render"
)

func main() {
	data := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	out := flag.String("out", "workplace/gfx/go", "PNG 輸出目錄")
	flag.Parse()

	if flag.NArg() == 0 {
		log.Fatal("用法: mm2dump [-data dir] [-out dir] <title|sheet <檔名>>")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	switch flag.Arg(0) {
	case "title":
		if err := dumpTitle(*data, *out); err != nil {
			log.Fatal(err)
		}
	case "sheet":
		if flag.NArg() < 2 {
			log.Fatal("sheet 需要一個 .16 檔名")
		}
		if err := dumpSheet(*data, *out, flag.Arg(1)); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("未知的子命令 %q", flag.Arg(0))
	}
}

// 開場畫面：NWCP.16 疊在原版層，再用原版字型在高解析層寫一行字，
// 驗證兩層的疊合順序正確（Flush 之後才畫文字）。
func dumpTitle(dataDir, outDir string) error {
	blob, err := os.ReadFile(findFile(dataDir, "NWCP.16"))
	if err != nil {
		return err
	}
	imgs, err := gfx.ParseSet(blob)
	if err != nil {
		return err
	}
	chBlob, err := os.ReadFile(findFile(dataDir, "MM2.CH"))
	if err != nil {
		return err
	}
	f, err := font.Parse(chBlob)
	if err != nil {
		return err
	}

	s := render.New(gfx.EGAPalette)
	s.Clear(0)
	logo := imgs[0].Paletted(gfx.EGAPalette)
	s.Blit(logo, (render.OrigW-imgs[0].Width)/2, (render.OrigH-imgs[0].Height)/2)
	s.Flush()
	s.DrawASCIIHi(f, "MIGHT AND MAGIC II", 8*render.Scale, 8*render.Scale,
		color.RGBA{0xFF, 0xFF, 0x55, 0xFF})

	fmt.Printf("開場 %d×%d（原版層 %d×%d，%d 倍）\n",
		render.HiW, render.HiH, render.OrigW, render.OrigH, render.Scale)
	return writePNG(filepath.Join(outDir, "title.png"), s)
}

func dumpSheet(dataDir, outDir, name string) error {
	blob, err := os.ReadFile(findFile(dataDir, name))
	if err != nil {
		return err
	}
	imgs, err := gfx.ParseSet(blob)
	if err != nil {
		return err
	}
	s := render.New(gfx.EGAPalette)
	s.Clear(0)
	x, y, rowH := 0, 0, 0
	for _, im := range imgs {
		if x+im.Width > render.OrigW {
			x, y = 0, y+rowH+1
			rowH = 0
		}
		s.Blit(im.Paletted(gfx.EGAPalette), x, y)
		x += im.Width + 1
		if im.Height > rowH {
			rowH = im.Height
		}
	}
	s.Flush()
	fmt.Printf("%s: %d 張影像\n", name, len(imgs))
	return writePNG(filepath.Join(outDir, strings.Split(name, ".")[0]+"_sheet.png"), s)
}

// 原版 EXE 內的檔名是小寫，實體檔案是大寫。DOS 不分大小寫，
// Linux/macOS 會分，所以存取一律走這裡。
func findFile(dir, name string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return filepath.Join(dir, name)
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name())
		}
	}
	return filepath.Join(dir, name)
}

func writePNG(path string, s *render.Screen) error {
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	if err := png.Encode(fh, s.Hi); err != nil {
		return err
	}
	fmt.Println("->", path)
	return nil
}
