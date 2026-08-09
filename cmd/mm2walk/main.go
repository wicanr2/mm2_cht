// mm2walk 用固定的按鍵序列走一段路，每一步輸出一張 PNG。
//
// 不需要視窗與 GPU，所以移動、事件觸發、中文訊息都能在 CI 或
// headless 環境驗收 —— 編譯成功不算視覺測試。
//
//	go run ./cmd/mm2walk -steps "F,F,R,F,F" -out workplace/gfx/walk
//
// 按鍵：F 前進、B 後退、L 左轉、R 右轉。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/cjk"
	"github.com/wicanr2/mm2_cht/internal/assets/font"
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/view"
)

func main() {
	dataDir := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	outDir := flag.String("out", "workplace/gfx/walk", "PNG 輸出目錄")
	steps := flag.String("steps", "F,F,F,F", "按鍵序列：F 前進 / B 後退 / L 左轉 / R 右轉")
	mapIdx := flag.Int("map", 0, "地圖編號")
	startX := flag.Int("x", 8, "起始 X")
	startY := flag.Int("y", 4, "起始 Y")
	lang := flag.String("lang", "translations/zh-Hant.json", "譯文檔；空字串則顯示原文")
	flag.Parse()

	w, err := game.NewWorld(read(*dataDir, "MAP.DAT"), read(*dataDir, "EVENTSI.DAT"))
	if err != nil {
		log.Fatal(err)
	}
	w.MapIndex, w.X, w.Y = *mapIdx, *startX, *startY

	f, err := font.Parse(read(*dataDir, "MM2.CH"))
	if err != nil {
		log.Fatal(err)
	}
	cjkBlob, err := os.ReadFile("assets/font/cjk24.bin")
	if err != nil {
		log.Fatal("讀不到中文 atlas（先跑 tools/build_cjk_font.py）: ", err)
	}
	cf, err := cjk.Parse(cjkBlob)
	if err != nil {
		log.Fatal(err)
	}
	trans := loadTranslations(*lang, *mapIdx)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	assets := view.Assets{ASCII: f, CJK: cf, Town: loadTown(*dataDir)}
	s := view.NewScreen()

	shot := func(n int, label string) {
		msg := w.Message
		if t, ok := trans[msg]; ok && t != "" {
			msg = t
		}
		view.Draw(s, w, assets, msg)
		path := filepath.Join(*outDir, fmt.Sprintf("%02d-%s.png", n, label))
		fh, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(fh, s.Hi); err != nil {
			log.Fatal(err)
		}
		fh.Close()
		note := ""
		if msg != "" {
			note = "  訊息: " + strings.ReplaceAll(msg, "\n", " / ")
		}
		fmt.Printf("%2d %-4s (%2d,%2d) %s%s\n", n, label, w.X, w.Y, w.Face, note)
	}

	shot(0, "start")
	for i, k := range strings.Split(*steps, ",") {
		switch strings.TrimSpace(strings.ToUpper(k)) {
		case "F":
			w.Move(1)
		case "B":
			w.Move(-1)
		case "L":
			w.Turn(-1)
		case "R":
			w.Turn(1)
		default:
			log.Fatalf("未知的按鍵 %q", k)
		}
		shot(i+1, strings.ToLower(strings.TrimSpace(k)))
	}
}

func read(dir, name string) []byte {
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if strings.EqualFold(e.Name(), name) {
				name = e.Name()
				break
			}
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		log.Fatal(err)
	}
	return b
}

// loadTranslations 建原文→譯文的對照。事件字串的 key 帶段號，
// 這裡只取目前地圖那一段，避免不同城鎮的同名設施互相蓋掉。
func loadTranslations(path string, mapIdx int) map[string]string {
	out := map[string]string{}
	if path == "" {
		return out
	}
	work, err := os.ReadFile("translations/strings.json")
	if err != nil {
		return out // 工作檔不入版控，沒有就顯示原文
	}
	var rows []struct{ Key, Source, Target string }
	if err := json.Unmarshal(work, &rows); err != nil {
		return out
	}
	trans := map[string]string{}
	if b, err := os.ReadFile(path); err == nil {
		var t []struct{ Key, Target string }
		if json.Unmarshal(b, &t) == nil {
			for _, e := range t {
				trans[e.Key] = e.Target
			}
		}
	}
	prefix := fmt.Sprintf("indoor.%02d.", mapIdx)
	for _, r := range rows {
		if strings.HasPrefix(r.Key, prefix) {
			if v := trans[r.Key]; v != "" {
				out[r.Source] = v
			}
		}
	}
	return out
}

// loadTown 載入城鎮第一人稱視角需要的三組素材。
func loadTown(dir string) *view.TownSet {
	set := func(name string) []gfx.Image {
		imgs, err := gfx.ParseSet(read(dir, name))
		if err != nil {
			log.Fatalf("解 %s 失敗: %v", name, err)
		}
		return imgs
	}
	return view.NewTownSet(set("TOWN.16"), set("TOWNF.16"), set("TOWNT.16"))
}
