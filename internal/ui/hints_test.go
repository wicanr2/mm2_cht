package ui_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/ui"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// 提示載得進來，而且每一條都帶出處。
//
// **出處是這批資料唯一的品質保證** —— 內容出自 1990 年代的雜誌，
// 沒辦法用原版驗，能做的只有「說得出是哪一頁寫的」。
func TestHintsLoad(t *testing.T) {
	h := ui.LoadHints("../../data")
	if h.LoadError != "" {
		t.Fatalf("讀 data/hints.json 失敗：%s", h.LoadError)
	}
	if len(h.Places) == 0 {
		t.Fatal("一個地點都沒有")
	}
	n := 0
	for _, p := range h.Places {
		for _, x := range p.Hints {
			n++
			if x.From == "" {
				t.Errorf("%s 有一條提示沒有出處：%s", p.Name, x.Text)
			}
			if x.Level != "已證實" && x.Level != "推論" {
				t.Errorf("%s 有一條提示的推論等級是 %q", p.Name, x.Level)
			}
		}
	}
	for _, x := range h.General {
		n++
		if x.From == "" {
			t.Errorf("通用提示沒有出處：%s", x.Text)
		}
	}
	if n < 200 {
		t.Errorf("只有 %d 條提示，預期兩百條以上", n)
	}
	t.Logf("%d 個地點、%d 條提示、%d 組衝突", len(h.Places), n, len(h.Conflicts))
}

// 地圖畫面拿得到提示，而且出處那一行帶著約定的前綴。
func TestHintsForMap(t *testing.T) {
	h := ui.LoadHints("../../data")
	title, lines := h.ForMap(0)
	if title == "" || len(lines) == 0 {
		t.Fatal("米德格特（地圖 0）沒有提示")
	}
	if len(lines)%2 != 0 {
		t.Fatalf("提示行數 %d 不是偶數 —— 每一條該是「本文 + 出處」兩行", len(lines))
	}
	for i := 1; i < len(lines); i += 2 {
		if !strings.HasPrefix(lines[i], view.HintIndent) {
			t.Errorf("第 %d 行是出處卻沒有前綴：%q", i, lines[i])
		}
	}
}

// 提示用到的字都要在 atlas 裡。缺字不會報錯，只會安靜地少一個字。
func TestHintGlyphs(t *testing.T) {
	s := load(t)
	f := s.Assets.CJK
	if f == nil {
		t.Skip("沒有中文 atlas")
	}
	h := ui.LoadHints("../../data")
	seen := map[rune]bool{}
	var miss []rune
	add := func(v string) {
		for _, r := range f.Missing(v) {
			if !seen[r] {
				seen[r] = true
				miss = append(miss, r)
			}
		}
	}
	for _, p := range h.Places {
		add(p.Name)
		for _, x := range p.Hints {
			add(x.Text)
		}
	}
	for _, x := range h.General {
		add(x.Text)
	}
	if len(miss) > 0 {
		t.Errorf("atlas 缺 %d 個字：%q（重跑 tools/build_cjk_font.py）",
			len(miss), string(miss))
	}
}
