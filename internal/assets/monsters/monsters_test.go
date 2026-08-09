package monsters_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
)

func orig(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "workplace", "orig", "MM2", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("找不到原版檔案 %s（玩家自備合法原版，解到 workplace/orig/）", path)
	}
	return b
}

func TestParseAllMonsters(t *testing.T) {
	ms, err := monsters.Parse(orig(t, "MONSTERS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != monsters.Count {
		t.Fatalf("解出 %d 筆，預期 %d", len(ms), monsters.Count)
	}
	// 名稱必須全部是可讀的 ASCII —— 減 0x80 減錯了會立刻現形。
	for _, m := range ms {
		if m.Name == "" {
			t.Errorf("第 %d 筆沒有名稱", m.Index)
			continue
		}
		for _, c := range m.Name {
			if c < 32 || c > 126 {
				t.Errorf("第 %d 筆的名稱有不可見字元 %q: %q", m.Index, c, m.Name)
				break
			}
		}
	}
}

// 首尾各定錨一筆。最後五筆是四位元素領主與 Sheltem，
// 與遊戲劇情的排序一致 —— 這同時守著 stride 26 與 256 筆。
func TestMonsterAnchors(t *testing.T) {
	ms, err := monsters.Parse(orig(t, "MONSTERS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []struct {
		i    int
		name string
	}{
		{0, "Creepy Crawler"},
		{3, "Kobold"},
		{17, "Orc"},
		{251, "Shalwend"},
		{252, "Pyrannaste"},
		{253, "Acwalandar"},
		{254, "Gralkor"},
		{255, "Sheltem"},
	} {
		if ms[w.i].Name != w.name {
			t.Errorf("第 %d 筆是 %q，預期 %q", w.i, ms[w.i].Name, w.name)
		}
	}
}
