// mm2dump 把原版素材解出來輸出成 PNG，不需要視窗。
//
// 用途是讓渲染管線在沒有 GPU 的環境也能驗收 —— 編譯成功不算視覺測試，
// 要有畫面能看。
//
//	go run ./cmd/mm2dump -data workplace/orig/MM2 -out workplace/gfx/go title
//	go run ./cmd/mm2dump -data workplace/orig/MM2 -out workplace/gfx/go sheet TOWNT.16
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/cjk"
	"github.com/wicanr2/mm2_cht/internal/assets/font"
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/render"
)

func main() {
	data := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	out := flag.String("out", "workplace/gfx/go", "PNG 輸出目錄")
	flag.Parse()

	if flag.NArg() == 0 {
		log.Fatal("用法: mm2dump [-data dir] [-out dir] <title|dialog|sheet <檔名>>")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	switch flag.Arg(0) {
	case "title":
		if err := dumpTitle(*data, *out); err != nil {
			log.Fatal(err)
		}
	case "dialog":
		if err := dumpDialog(*data, *out); err != nil {
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

// 對話框：原版城鎮背景 + 中文譯文，驗證中英混排與 '@' 換行。
// 缺字會被明白報出來，不會默默畫成空白。
func dumpDialog(dataDir, outDir string) error {
	f, cf, err := loadFonts(dataDir)
	if err != nil {
		return err
	}
	blob, err := os.ReadFile(findFile(dataDir, "TOWNF.16"))
	if err != nil {
		return err
	}
	imgs, err := gfx.ParseSet(blob)
	if err != nil {
		return err
	}

	lines := []string{}
	trans, err := loadTranslations()
	if err != nil {
		return err
	}
	for _, k := range []string{
		"indoor.00.000", "indoor.00.002", "indoor.00.003", "indoor.00.007",
	} {
		if v := trans[k]; v != "" {
			lines = append(lines, v)
		}
	}
	body := trans["indoor.00.021"]

	s := render.New(gfx.EGAPalette)
	s.Clear(0)
	s.Blit(imgs[0].Paletted(gfx.EGAPalette), (render.OrigW-imgs[0].Width)/2, 8)
	// 對話框底：原版層畫實心塊，中文疊在放大後的畫布上
	for y := 80; y < render.OrigH-4; y++ {
		for x := 8; x < render.OrigW-8; x++ {
			s.Orig.SetColorIndex(x, y, 1)
		}
	}
	s.Flush()

	st := render.TextStyle{ASCII: f, CJK: cf, Color: color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}}
	y := 84 * render.Scale
	y = s.DrawText(st, "中門城 Middlegate", 14*render.Scale, y)
	for _, ln := range lines {
		y = s.DrawText(render.TextStyle{ASCII: f, CJK: cf,
			Color: color.RGBA{0x55, 0xFF, 0x55, 0xFF}}, "  "+ln, 14*render.Scale, y)
	}
	s.DrawText(render.TextStyle{ASCII: f, CJK: cf,
		Color: color.RGBA{0xFF, 0xFF, 0x55, 0xFF}}, body, 14*render.Scale, y+8)

	for _, ln := range append(lines, body) {
		if miss := cf.Missing(ln); len(miss) > 0 {
			fmt.Printf("缺字：%q -> %s（重跑 tools/build_cjk_font.py）\n", ln, string(miss))
		}
	}
	fmt.Printf("中文 %d×%d 點陣，%d 行\n", cf.W, cf.H, len(lines)+2)
	return writePNG(filepath.Join(outDir, "dialog.png"), s)
}

func loadFonts(dataDir string) (*font.Font, *cjk.Font, error) {
	chBlob, err := os.ReadFile(findFile(dataDir, "MM2.CH"))
	if err != nil {
		return nil, nil, err
	}
	f, err := font.Parse(chBlob)
	if err != nil {
		return nil, nil, err
	}
	cjkBlob, err := os.ReadFile("assets/font/cjk24.bin")
	if err != nil {
		return nil, nil, fmt.Errorf("讀不到中文 atlas（先跑 tools/build_cjk_font.py）: %w", err)
	}
	cf, err := cjk.Parse(cjkBlob)
	return f, cf, err
}

func loadTranslations() (map[string]string, error) {
	b, err := os.ReadFile("translations/zh-Hant.json")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Key    string `json:"key"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, r := range rows {
		m[r.Key] = r.Target
	}
	return m, nil
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
