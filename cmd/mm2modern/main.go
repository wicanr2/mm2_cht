// mm2modern 把原版素材烘成高解析的「現代」素材包。
//
//	go run ./cmd/mm2modern -data workplace/orig/MM2
//	go run ./cmd/mm2modern -amiga workplace/amiga -out workplace/modern-amiga
//
// 產出是一疊 PNG 加一份 `set.json`，遊戲把它當成另一個可切換的素材來源。
//
// 為什麼要落地成檔案，而不是執行時算 Scale3x（那條路徑一直都在，按 F5）：
// **落地之後這些圖可以被換掉**。Scale3x 的上限就是原版像素的資訊量，
// 想要真正重畫的美術，得有一個「檔案在這裡、尺寸與命名照這個規矩」的地方
// 讓人放進去。這支工具產生的就是那份骨架，同時也是可以直接用的預設值。
//
// **產出預設落在 `workplace/`，不進版控**：放大過的原版美術仍然是原版美術
// （硬性原則 5）。玩家自己烘一份，或由美術把重畫的圖放進 `assets/modern`
// —— 後者是原創的，那一份才能跟著 repo 走。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/wicanr2/mm2_cht/internal/assets/amiga"
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// manifest 是素材包的規格。遊戲照這裡的值決定怎麼畫，不靠檔名猜。
type manifest struct {
	// Source 是來源平台，只是給人看的。
	Source string `json:"source"`
	// Clear 是透空色的色號。DOS 是 8、Amiga 是 0 —— 猜錯的話牆的邊緣
	// 會多出一圈實心方塊，而且不會有任何錯誤訊息。
	Clear int `json:"clear"`
	// TorchStride 是火炬每一格佔幾張圖（DOS 4、Amiga 3）。
	TorchStride int `json:"torchStride"`
	// Scale 是圖已經放大的倍率。遊戲拿它與 render.Scale 比對，
	// 不符就不載入 —— 倍率不合的素材畫出來是「位置對、大小錯」，
	// 看起來像座標算錯，很難往素材那邊想。
	Scale int `json:"scale"`
}

func main() {
	data := flag.String("data", "workplace/orig/MM2", "DOS 原版資料目錄")
	amigaDir := flag.String("amiga", "", "改用 Amiga 素材當來源")
	out := flag.String("out", "workplace/modern", "輸出目錄")
	flag.Parse()

	var groups map[string][]*image.Paletted
	var mf manifest
	var err error
	if *amigaDir != "" {
		groups, mf, err = fromAmiga(*amigaDir)
	} else {
		groups, mf, err = fromDOS(*data)
	}
	if err != nil {
		log.Fatal(err)
	}
	mf.Scale = render.Scale

	n := 0
	for name, ims := range groups {
		dir := filepath.Join(*out, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}
		for i, im := range ims {
			p := filepath.Join(dir, fmt.Sprintf("%02d.png", i))
			f, err := os.Create(p)
			if err != nil {
				log.Fatal(err)
			}
			if err := png.Encode(f, render.Scale3x(im)); err != nil {
				log.Fatal(err)
			}
			f.Close()
			n++
		}
	}
	b, _ := json.MarshalIndent(mf, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "set.json"), append(b, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s：%d 張，放大 %d 倍 → %s\n", mf.Source, n, mf.Scale, *out)
}

func fromDOS(dir string) (map[string][]*image.Paletted, manifest, error) {
	mf := manifest{Source: "DOS", Clear: 8, TorchStride: 4}
	g := map[string][]*image.Paletted{}
	for name, key := range map[string]string{
		"TOWN.16": "walls", "TOWNF.16": "floor", "TOWNT.16": "torch", "SKY.16": "sky",
	} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, mf, err
		}
		set, err := gfx.ParseSet(b)
		if err != nil {
			return nil, mf, fmt.Errorf("%s: %w", name, err)
		}
		for _, im := range set {
			g[key] = append(g[key], im.Paletted(gfx.EGAPalette))
		}
	}
	return g, mf, nil
}

func fromAmiga(dir string) (map[string][]*image.Paletted, manifest, error) {
	mf := manifest{Source: "Amiga", Clear: amiga.TransparentIndex, TorchStride: 3}
	g := map[string][]*image.Paletted{}
	for name, key := range map[string]string{
		"town.32": "walls", "townf.32": "floor", "townt.32": "torch", "sky.32": "sky",
	} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, mf, err
		}
		set, err := amiga.Parse(b)
		if err != nil {
			return nil, mf, fmt.Errorf("%s: %w", name, err)
		}
		g[key] = set.Images
	}
	return g, mf, nil
}
