package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 職業、種族、陣營的顯示名來自 data/labels.json，不再寫在 Go 原始碼裡。
// 沒設定譯文時顯示原文；設了就顯示譯文。
func TestLabelsComeFromData(t *testing.T) {
	game.UseText(nil)
	if got := game.ClassName(int(game.Knight)); got != "Knight" {
		t.Errorf("沒有譯文時應該顯示原文 Knight，實際 %q", got)
	}
	game.UseText(fakeCatalog{"exe.003E": "武士"})
	if got := game.ClassName(int(game.Knight)); got != "武士" {
		t.Errorf("有譯文時應該顯示武士，實際 %q", got)
	}
	if got := game.RaceName(int(game.Human)); got != "Human" {
		t.Errorf("沒對到 key 時應該退回原文 Human，實際 %q", got)
	}
	game.UseText(nil)
}

type fakeCatalog map[string]string

func (f fakeCatalog) Or(key, fallback string) string {
	if s, ok := f[key]; ok {
		return s
	}
	return fallback
}
