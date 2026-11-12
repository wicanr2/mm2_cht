package ui_test

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/i18n"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

func reachUnicodeName(t *testing.T, s *ui.Session) {
	t.Helper()
	s.Key(ui.KeyCreate)
	// A rejected class choice must not remain on subsequent valid input pages.
	s.PressDigit(9)
	if len(s.Lines) == 0 {
		t.Fatal("invalid class did not report an error")
	}
	choice := 0
	for n := 0; n < 200 && choice == 0; n++ {
		for i, ok := range game.EligibleClasses(s.New.Attr) {
			if ok {
				choice = i + 1
				break
			}
		}
		if choice == 0 {
			s.Key(ui.KeyConfirm)
		}
	}
	if choice == 0 {
		t.Fatal("no eligible class")
	}
	s.PressDigit(choice)
	for i := 0; i < 3; i++ {
		s.Key(ui.KeyConfirm)
	}
	if s.Mode != ui.ModeName {
		t.Fatalf("not at name input: %v", s.Mode)
	}
	if len(s.Lines) != 0 {
		t.Fatalf("stale class error at name input: %v", s.Lines)
	}
}

func TestUnicodeNameThroughCreateSaveAndReload(t *testing.T) {
	s := load(t)
	// Save/Restore use a relative player folder. Keep this test's writes and
	// cleanup away from any existing player save, including targeted test runs.
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	runRoot := t.TempDir()
	for _, name := range []string{"assets", "translations", "data", "workplace"} {
		if err := os.Symlink(filepath.Join(repoRoot, name), filepath.Join(runRoot, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chdir(runRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(repoRoot); err != nil {
			t.Error(err)
		}
	})
	if err := s.InitPresentation(".", "workplace/hd-amiga-r1", ""); err != nil {
		t.Fatal(err)
	}
	p := s.Presentation()
	p.Locale = i18n.Simplified
	if err := s.SetPresentation(p); err != nil {
		t.Fatal(err)
	}
	reachUnicodeName(t, s)
	s.ReplaceInput("甲\b乙")
	if s.New.Name != "甲乙" {
		t.Fatal("committed control text acted as an edit command")
	}
	s.ReplaceInput("")
	for _, r := range "龍劍俠" {
		if !s.TypeRune(r) {
			t.Fatalf("traditional name rune rejected %c", r)
		}
	}
	for range 3 {
		s.TypeRune('\b')
	}
	name := "简体冒险者龙剑侠客甲"
	for _, r := range name {
		if !s.TypeRune(r) {
			t.Fatalf("name rune rejected %c", r)
		}
	}
	if s.TypeRune('乙') {
		t.Fatal("more than ten characters accepted")
	}
	s.TypeRune('\b')
	s.TypeRune('甲')
	before, _ := json.Marshal(s.Game.State())
	s.SetInputPreview("正在組字", true)
	if s.New.Name != name {
		t.Fatal("preedit changed committed name")
	}
	after, _ := json.Marshal(s.Game.State())
	if !bytes.Equal(before, after) {
		t.Fatal("preedit changed save")
	}
	s.SetInputPreview("", false)
	if s.TypeRune('@') || s.TypeRune('\n') || s.TypeRune('\x7f') {
		t.Fatal("text control accepted")
	}
	if dir := os.Getenv("MM2_DISPLAY_RECEIPTS"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "input-name-hans.png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, s.Draw().Hi)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	s.Key(ui.KeyConfirm)
	if len(s.Game.Roster) != 1 || s.Game.Roster[0].Name != name || s.Game.Roster[0].Empty() {
		t.Fatal("Unicode character was not created")
	}
	if _, ok := s.Game.RemoveFromParty(0); !ok {
		t.Fatal("remove member")
	}
	if _, ok := s.Game.AddToParty(0); !ok {
		t.Fatal("add Unicode member")
	}
	if s.Game.World.Party[len(s.Game.Party)-1].Name != name {
		t.Fatal("world party not updated")
	}
	s.Game.Party[len(s.Game.Party)-1].Gold = 321
	want, _ := json.Marshal(s.Game.State())
	if !strings.Contains(s.Save(), "已存檔") {
		t.Fatal("save failed")
	}
	t.Cleanup(func() { os.RemoveAll("save") })
	other := load(t)
	if !other.Restore() {
		t.Fatal("cannot restore")
	}
	got, _ := json.Marshal(other.Game.State())
	if !bytes.Equal(want, got) {
		t.Fatal("Unicode name/party/roster changed after reload")
	}
	if other.Game.Party[len(other.Game.Party)-1].Name != name {
		t.Fatal("name truncated")
	}
	if err := other.InitPresentation(".", "workplace/hd-amiga-r1", ""); err != nil {
		t.Fatal(err)
	}
	other.Draw()
	if !other.Assets.CJK.Has('甲') || !other.DisplayCatalog().SC.Has('简') {
		t.Fatal("input font fallback missing")
	}
}

func TestLocalizedAnswerContinuesOriginalEvent(t *testing.T) {
	for _, tc := range []struct {
		answer string
		result byte
	}{{"KEYS", 1}, {"鑰匙", 1}, {"钥匙", 1}, {"ＫＥＹＳ", 1}, {"德鲁伊", 0}, {"宝剑", 0}} {
		s := load(t)
		if err := s.InitPresentation(".", "workplace/hd-amiga-r1", ""); err != nil {
			t.Fatal(err)
		}
		w := s.Game.World
		w.RunScriptForTest([]byte{0x2f, 0x30, 0xcf, 0xd5, 0xc1, 0xc7, 0xfa, 0xfa, 0xfa, 0xfa, 0xfa, 0xfa})
		s.Mode = ui.ModeText
		for _, r := range tc.answer {
			if !s.TypeRune(r) {
				t.Fatalf("answer rune rejected %c", r)
			}
		}
		s.Key(ui.KeyConfirm)
		if w.Pending != nil || w.Result != tc.result {
			t.Fatalf("answer %q: pending=%v result=%d", tc.answer, w.Pending, w.Result)
		}
	}
}
