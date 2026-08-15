// Package monpack 讀烘好的怪物素材包（Mega Drive、Amiga）。
//
// 素材包由 `tools/mdgfx.py --export` 與 `tools/amiga32.py --export-monsters`
// 產生，格式相同：每張圖的每個影格一個索引色 PNG（索引 0 透空），
// 外加一份 `set.json`。尺寸由 `set.json` 宣告 —— Mega Drive 是 88×88、
// Amiga 是 84×86，不寫死。
//
// **為什麼是烘好的檔案而不是執行時解 ROM**：一張圖在 ROM 裡是「去重過的
// tile 池 ＋ 一張 nametable」，而 nametable 的格子順序是**硬體 sprite 的
// 順序**（九個 sprite，寬 4/4/3、高 4/4/3，sprite 內部直向排），不是
// row-major。那個版面是從實機的 sprite 屬性表影子（work RAM `0xFFD2A8`）
// 讀出來的 —— 既然已經定案，重建一次就好。推導見
// `docs/research/02-other-platforms.md`。
//
// 槽號用 DOS `MONSTERS.16` 的剪影對出來的：ROM 裡的順序與 DOS 的槽號
// **是一個排列不是恆等**，所以 `set.json` 的 `slot` 才是能用的鍵，
// PNG 的檔名編號不是。
package monpack

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// TransparentIndex 是透空的調色盤索引。ROM 裡每張圖的第 0 格就是背景色，
// 烘出來的 PNG 也把 0 標成 transparency。
const TransparentIndex = 0

// Set 是一整包，以 DOS 槽號為鍵。
type Set struct {
	// Source 是來源平台，給訊息用。
	Source string
	// W、H 是一張圖的原版像素尺寸。
	W, H int
	// Pics[槽號] 是那一槽的全部影格，至少一張。
	Pics map[int][]*image.Paletted
	// Extra 是對不到 DOS 槽的圖（片頭的旋轉地球、書、城堡那些）。
	// 留著是為了「解出來的東西不丟掉」，遊戲用不到。
	Extra [][]*image.Paletted
}

// Slots 回傳有圖的槽號個數。
func (s *Set) Slots() int { return len(s.Pics) }

type manifest struct {
	Source   string `json:"source"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Pictures []struct {
		Pic    int      `json:"pic"`
		Slot   int      `json:"slot"`
		Match  float64  `json:"match"`
		Frames []string `json:"frames"`
	} `json:"pictures"`
}

// Load 讀一個素材包目錄。目錄不存在時回傳錯誤 —— 呼叫端把它當成
// 「這個平台沒有怪物素材」處理，不是失敗。
func Load(dir string) (*Set, error) {
	b, err := os.ReadFile(filepath.Join(dir, "set.json"))
	if err != nil {
		return nil, err
	}
	var mf manifest
	if err := json.Unmarshal(b, &mf); err != nil {
		return nil, fmt.Errorf("set.json 解不開：%w", err)
	}
	if mf.Width <= 0 || mf.Height <= 0 {
		return nil, fmt.Errorf("set.json 沒宣告尺寸")
	}
	out := &Set{Source: mf.Source, W: mf.Width, H: mf.Height,
		Pics: map[int][]*image.Paletted{}}
	for _, p := range mf.Pictures {
		frames := make([]*image.Paletted, 0, len(p.Frames))
		for _, name := range p.Frames {
			im, err := loadPaletted(filepath.Join(dir, name), mf.Width, mf.Height)
			if err != nil {
				return nil, err
			}
			frames = append(frames, im)
		}
		if len(frames) == 0 {
			continue
		}
		if p.Slot < 0 {
			out.Extra = append(out.Extra, frames)
			continue
		}
		out.Pics[p.Slot] = frames
	}
	if len(out.Pics) == 0 {
		return nil, fmt.Errorf("%s 裡沒有對到 DOS 槽號的圖", dir)
	}
	return out, nil
}

// loadPaletted 讀一張索引色 PNG。
//
// **一定要是索引色**。存成 RGBA 也畫得出來，但畫面上的東西會少一層資訊：
// 兩個剛好同色的調色盤索引會被併成一個，而 `render` 的高解析貼圖走的是
// `*image.Paletted`。所以這裡直接把型別當契約檢查。
func loadPaletted(path string, w, h int) (*image.Paletted, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s：%w", filepath.Base(path), err)
	}
	p, ok := im.(*image.Paletted)
	if !ok {
		return nil, fmt.Errorf("%s 不是索引色 PNG（是 %T）", filepath.Base(path), im)
	}
	if b := p.Bounds(); b.Dx() != w || b.Dy() != h {
		return nil, fmt.Errorf("%s 是 %d×%d", filepath.Base(path), b.Dx(), b.Dy())
	}
	return p, nil
}
