package ui_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/i18n"
	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

func TestToolbarLabelsKeepVisibleHeight(t *testing.T) {
	s := load(t)
	if err := s.InitPresentation(".", "workplace/hd-amiga-r1", ""); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"amiga", "megadrive"} {
		if !s.HasPlatform(platform) {
			t.Fatalf("missing toolbar platform fixture %s", platform)
		}
		for _, percent := range []int{100, 125, 150} {
			p := s.Presentation()
			p.Platform = platform
			p.HD = false
			p.ToolbarPercent = percent
			if err := s.SetPresentation(p); err != nil {
				t.Fatal(err)
			}
			bar := ui.NewToolbar(s)
			screen := &render.Screen{Hi: image.NewRGBA(image.Rect(0, 0, 960, 640))}
			bar.Draw(screen)
			inkBounds := func(x0, x1 int) image.Rectangle {
				var b image.Rectangle
				for y := 8; y < bar.RowHeight()-8; y++ {
					for x := x0; x < x1; x++ {
						c := screen.Hi.RGBAAt(x, y)
						if c.R > 100 || c.G > 110 || c.B > 120 {
							b = b.Union(image.Rect(x, y, x+1, y+1))
						}
					}
				}
				return b
			}
			ref := inkBounds(20, 110)
			// Keep long platform names out of the dropdown arrow's column.
			for y := 8; y < bar.RowHeight()-8; y++ {
				if y >= bar.RowHeight()/2-2 && y <= bar.RowHeight()/2+1 {
					continue
				}
				for x := 527; x <= 533; x++ {
					c := screen.Hi.RGBAAt(x, y)
					if c.R > 100 || c.G > 110 || c.B > 120 {
						t.Fatalf("%s %d%%: label overlaps dropdown caret at %d,%d", platform, percent, x, y)
					}
				}
			}
			for _, b := range []image.Rectangle{inkBounds(320, 522), inkBounds(688, 772)} {
				if b.Empty() || b.Dy() < ref.Dy()-1 || b.Dy() > ref.Dy()+1 || b.Min.Y < ref.Min.Y-1 || b.Min.Y > ref.Min.Y+1 {
					t.Fatalf("%s %d%%: toolbar ink %v differs from CJK %v", platform, percent, b, ref)
				}
			}
			if dir := os.Getenv("MM2_TOOLBAR_RECEIPTS"); dir != "" {
				file, err := os.Create(filepath.Join(dir, fmt.Sprintf("toolbar-%s-%d.png", platform, percent)))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(file, screen.Hi.SubImage(image.Rect(0, 0, 960, bar.RowHeight())))
				closeErr := file.Close()
				if err != nil {
					t.Fatal(err)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
			}
		}
	}
}

func TestPresentationDoesNotChangeGameState(t *testing.T) {
	s := load(t)
	if err := s.InitPresentation(".", "workplace/hd-amiga-r1", ""); err != nil {
		t.Fatal(err)
	}
	s.Draw()
	before, _ := json.Marshal(s.Game.State())
	frame := sha256.Sum256(s.Draw().Hi.Pix)
	p := s.Presentation()
	original := p
	for _, locale := range []string{i18n.Simplified, i18n.Traditional} {
		p.Locale = locale
		p.TextPercent = 80
		if err := s.SetPresentation(p); err != nil {
			t.Fatal(err)
		}
		s.Draw()
		after, _ := json.Marshal(s.Game.State())
		if !bytes.Equal(before, after) {
			t.Fatal("display changed game save")
		}
	}
	if s.HDReady() {
		p.HD = true
		p.TextPercent = 100
		if err := s.SetPresentation(p); err != nil {
			t.Fatal(err)
		}
		s.Draw()
		after, _ := json.Marshal(s.Game.State())
		if !bytes.Equal(before, after) {
			t.Fatal("HD changed game save")
		}
	}
	if err := s.SetPresentation(original); err != nil {
		t.Fatal(err)
	}
	if got := sha256.Sum256(s.Draw().Hi.Pix); got != frame {
		t.Fatal("original frame did not restore exactly")
	}
	path := filepath.Join(t.TempDir(), "settings", "display.json")
	if err := ui.WritePresentation(path, original); err != nil {
		t.Fatal(err)
	}
	got, err := ui.ReadPresentation(path)
	if err != nil || got != original {
		t.Fatalf("preferences round trip: %+v %v", got, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

// This is a controlled remake fixture, not a DOS oracle comparison.
func TestHDCombatAndQuickFightKeepResults(t *testing.T) {
	for _, quick := range []bool{false, true} {
		var expectedDrawing, expectedResult []byte
		for _, hdMode := range []bool{false, true} {
			s := load(t)
			if err := s.InitPresentation(".", "workplace/hd-amiga-r1", ""); err != nil {
				t.Fatal(err)
			}
			if !s.HDReady() {
				t.Skipf("complete optional HD pack unavailable: %v", s.HDError())
			}
			p := s.Presentation()
			p.HD = hdMode
			p.Locale = i18n.Simplified
			if err := s.SetPresentation(p); err != nil {
				t.Fatal(err)
			}
			startFight(t, s)
			s.Game.Rand = game.NewRand(0x1234)
			for n := 0; n < 12; n++ {
				s.Draw()
				s.Tick()
			}
			frame := s.Draw().Hi
			if dir := os.Getenv("MM2_DISPLAY_RECEIPTS"); dir != "" && hdMode {
				name := "battle-normal-hans.png"
				if quick {
					name = "battle-quick-hans.png"
				}
				f, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, frame)
				closeErr := f.Close()
				if err != nil {
					t.Fatal(err)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
			}
			after, _ := json.Marshal(s.Game.State())
			if !hdMode {
				expectedDrawing = after
			} else if !bytes.Equal(expectedDrawing, after) {
				t.Fatal("HD changed animation RNG consumption")
			}
			if quick {
				s.Key(ui.KeyQuickFight)
			} else {
				for n := 0; n < 20 && s.Mode == ui.ModeCombat; n++ {
					s.Key(ui.KeyConfirm)
					s.Draw()
				}
			}
			if s.Mode == ui.ModeCombat || s.Chest == nil {
				t.Fatalf("combat branch quick=%v did not settle: %v", quick, s.Lines)
			}
			result, _ := json.Marshal(struct {
				State game.State
				Chest *game.Chest
				Mode  ui.Mode
			}{s.Game.State(), s.Chest, s.Mode})
			if !hdMode {
				expectedResult = result
			} else if !bytes.Equal(expectedResult, result) {
				t.Fatalf("HD changed combat results, quick=%v", quick)
			}
			p.HD = false
			if err := s.SetPresentation(p); err != nil {
				t.Fatal(err)
			}
			s.Draw()
		}
	}
}

func TestAmigaMenuBackingFollowsQueuedArtwork(t *testing.T) {
	s := load(t)
	if err := s.InitPresentation(".", "workplace/hd-amiga-r1", ""); err != nil {
		t.Fatal(err)
	}
	if !s.HasPlatform("amiga") {
		t.Skip("Amiga source unavailable")
	}
	p := s.Presentation()
	p.Platform = "amiga"
	if err := s.SetPresentation(p); err != nil {
		t.Fatal(err)
	}
	s.Key(ui.KeySettings)
	if got := s.Draw().Hi.RGBAAt(360, 336); got != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("scene painted over menu backing: %v", got)
	}
}

func TestToolbarPointerOwnershipAndSelections(t *testing.T) {
	s := load(t)
	if err := s.InitPresentation(".", "workplace/hd-amiga-r1", ""); err != nil {
		t.Fatal(err)
	}
	bar := ui.NewToolbar(s)
	bar.Pointer(30, 20, true)
	bar.Pointer(30, 20, false)
	if !bar.MenuOpen() {
		t.Fatal("language dropdown did not open")
	}
	bar.Pointer(30, 106, true)
	bar.Pointer(30, 106, false)
	if s.Presentation().Locale != i18n.Simplified {
		t.Fatal("simplified selection did not apply")
	}
	bar.Pointer(1, 1, false)
	for _, percent := range []int{125, 150, 100} {
		bar.Pointer(935, 20, true)
		_, owned := bar.Pointer(500, 580, false)
		if !owned {
			t.Fatal("toolbar lost dragged release")
		}
		if s.Presentation().ToolbarPercent != percent {
			t.Fatalf("toolbar percent %d", s.Presentation().ToolbarPercent)
		}
		bar.Pointer(1, 1, false)
	}
	bar.Pointer(160, 20, true)
	bar.Pointer(160, 20, false)
	if !bar.CloseMenu() || bar.MenuOpen() {
		t.Fatal("Escape must close only the toolbar menu")
	}
	bar.Pointer(400, 580, false)
	if bar.Visible {
		t.Fatal("toolbar did not hide below its bounds")
	}
}
