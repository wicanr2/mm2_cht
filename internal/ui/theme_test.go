package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// 指定初始平台後，F6 必須從該平台的索引繼續，而不是從 DOS 的索引 0
// 開始。Modern 會在 loader 最後加入，因此下一套應回到 DOS；再循環一圈
// 必須回到 Modern。這條測試選擇 helper 與正常 F6 路徑，不是直接改 setIdx。
func TestConfiguredThemeStartsCycleAtSelectedSet(t *testing.T) {
	dos := &view.TownSet{Platform: view.PlatformDOS}
	amiga := &view.TownSet{Platform: view.PlatformAmiga}
	modern := &view.TownSet{Platform: view.PlatformModern}
	sets := []*view.TownSet{dos, amiga, modern}
	selected, err := selectTheme(sets, "modern", nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected != modern {
		t.Fatalf("指定 Modern 後未選到現代素材：%v", selected)
	}
	s := &Session{Assets: view.Assets{Town: selected}, sets: sets, setIdx: themeSetIndex(sets, selected)}
	if !s.Key(KeyPlatform) {
		t.Fatal("F6 沒有切換平台")
	}
	s.Mode = ModeExplore // F6 的訊息模式不應影響下一個循環按鍵
	if got := s.Assets.Town.Platform; got != view.PlatformDOS {
		t.Fatalf("Modern 後第一套應是 DOS，實際是 %v（初始 setIdx 未同步）", got)
	}
	for n := 0; n < 8 && s.Assets.Town.Platform != view.PlatformModern; n++ {
		if !s.Key(KeyPlatform) {
			t.Fatal("F6 循環中按鍵沒有作用")
		}
		s.Mode = ModeExplore
	}
	if s.Assets.Town.Platform != view.PlatformModern {
		t.Fatal("F6 循環未回到指定的 Modern 素材")
	}
}

func TestSelectThemeKeepsDOSAsSafeDefault(t *testing.T) {
	dos := &view.TownSet{Platform: view.PlatformDOS}
	modern := &view.TownSet{Platform: view.PlatformModern}
	if got, err := selectTheme([]*view.TownSet{dos, modern}, "", nil); err != nil || got != dos {
		t.Fatalf("空主題應安全回 DOS，got=%v err=%v", got, err)
	}
	if got, err := selectTheme([]*view.TownSet{modern, dos}, "dos", nil); err != nil || got != dos {
		t.Fatalf("dos 主題應明確選 DOS，got=%v err=%v", got, err)
	}
	if got, err := selectTheme([]*view.TownSet{dos, modern}, "modern", nil); err != nil || got != modern {
		t.Fatalf("modern 主題未選到現代素材，got=%v err=%v", got, err)
	}
	if _, err := selectTheme([]*view.TownSet{dos}, "modern", nil); err == nil {
		t.Fatal("指定但未載入的主題應明確失敗")
	}
}

func TestLoadWithOptionsRejectsUnknownThemeBeforeReadingData(t *testing.T) {
	if _, err := LoadWithOptions(filepath.Join(t.TempDir(), "missing"), LoadOptions{Theme: "amiga-ish"}); err == nil {
		t.Fatal("未知 theme 應在讀取原版資料前失敗")
	}
}

func TestLoadPackTownRejectsIncompleteGroup(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "set.json"), []byte(`{"scale":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "walls"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeThemePNG(filepath.Join(dir, "walls", "00.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPackTown(dir); err == nil {
		t.Fatal("缺少牆面影格時不應加入部分素材組")
	}
}

func writeThemePNG(path string) error {
	im := image.NewPaletted(image.Rect(0, 0, render.Scale, render.Scale), color.Palette{color.Black})
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, im)
}
