package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/mm2_cht/internal/view"
)

// Hints 是攻略提示，來源是 1990 年代《軟體世界》雜誌的 MM2 攻略
// （轉錄在 `docs/research/soft-world/`，整理在該目錄的 `11-walkthrough-index.md`）。
//
// 當年這種資訊只在雜誌上，遊戲裡一個字都沒有。收進來的理由與說明書
// 那幾章一樣：**紙本才有的東西要進遊戲**。
//
// 三件跟著資料一起收的事：
//
//   - **每一條都帶出處**（`檔名:行號`）。沒有出處的沒有收。
//   - **推論等級只有兩級。** `已證實` 是雜誌以外另有本專案內的獨立來源
//     （手冊轉錄、`data/*.json`、反組譯結果）；其餘是 `推論`。
//     雜誌另一期寫了同一件事只算同一個來源，不升級。
//   - **雜誌是二手資料。** 與本專案解出的機制衝突時兩邊都留著，
//     記在 `Conflicts`，不擅自改成哪一邊。
type Hints struct {
	Source         string            `json:"source"`
	Issues         []string          `json:"issues"`
	LevelDefinition map[string]string `json:"levelDefinition"`
	// MapIndexSource 記的是每一類地點的編號從哪裡推出來的（城鎮、野外、地下城）。
	MapIndexSource map[string]string `json:"mapIndexSource"`
	Places         []HintPlace       `json:"places"`
	General        []Hint            `json:"general"`
	Conflicts      []HintConflict    `json:"conflicts"`

	// LoadError 是載入失敗的原因，成功時是空的。
	LoadError string `json:"-"`
}

// HintPlace 是一個地點的提示。
//
// `Map` 是地圖編號，**地下城與城堡是 null** —— 那些圖的編號本專案還沒
// 定出來，所以用 `Area`（世界地圖的格線索引）加 `Entrance`（地表入口座標）
// 定位。編不出來就不編，不要塞一個看起來合理的數字。
type HintPlace struct {
	Map      *int           `json:"map"`
	Area     string         `json:"area"`
	Name     string         `json:"name"`
	Kind     string         `json:"kind"`
	Entrance *HintEntrance  `json:"entrance"`
	Hints    []Hint         `json:"hints"`
}

// HintEntrance 是地下城／城堡在地表的入口：世界地圖的格線索引加座標。
type HintEntrance struct {
	Area string `json:"area"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
}

// String 是「B1-4,12」這種寫法。
func (e *HintEntrance) String() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s-%d,%d", e.Area, e.X, e.Y)
}

// Hint 是一條提示。
type Hint struct {
	Text     string `json:"text"`
	From     string `json:"from"`
	Level    string `json:"level"`
	Coord    string `json:"coord"`
	CrossRef string `json:"crossRef"`
}

// HintConflict 是同一件事在不同來源說法不一的紀錄。
type HintConflict struct {
	Topic   string `json:"topic"`
	Records []struct {
		Claim string `json:"claim"`
		From  string `json:"from"`
	} `json:"records"`
	Note string `json:"note"`
}

// LoadHints 讀 `data/hints.json`。讀不到就回空的 —— 提示是加值，
// 少了它遊戲照樣能玩，不該讓整個載入失敗。
func LoadHints(dir string) *Hints {
	b, err := os.ReadFile(filepath.Join(dir, "hints.json"))
	if err != nil {
		return &Hints{}
	}
	var h Hints
	if err := json.Unmarshal(b, &h); err != nil {
		// **不要靜靜回空的。** 解析失敗與「檔案裡沒有提示」長得一模一樣，
		// 而症狀是畫面上少了一整欄，沒有任何線索指向這裡。
		return &Hints{LoadError: err.Error()}
	}
	return &h
}

// ForMap 回傳這張地圖的提示行。沒有對應的地點時回 nil。
//
// 每一條是「提示本文」加一行縮排的出處與等級 —— 出處要看得到，
// 不然玩家分不出「雜誌這樣寫」與「程式這樣做」。
func (h *Hints) ForMap(idx int) (title string, lines []string) {
	if h == nil {
		return "", nil
	}
	for _, p := range h.Places {
		if p.Map == nil || *p.Map != idx {
			continue
		}
		return p.Name, hintLines(p.Hints)
	}
	return "", nil
}

// GeneralLines 是不綁地點的通用提示。
func (h *Hints) GeneralLines() []string {
	if h == nil {
		return nil
	}
	return hintLines(h.General)
}

func hintLines(hs []Hint) []string {
	out := make([]string, 0, len(hs)*2)
	for _, x := range hs {
		t := x.Text
		if x.Coord != "" {
			t = fmt.Sprintf("（%s）%s", x.Coord, t)
		}
		// 出處那一行用**全形空白起頭**，畫面那邊靠這個前綴分兩種顏色。
		// 不用符號當項目記號 —— `・` 這類字不一定烘得進字型，
		// 而缺字是安靜的：畫面上就是少一個字，不會報錯。
		out = append(out, t, view.HintIndent+x.Level+"／"+x.From)
	}
	return out
}
