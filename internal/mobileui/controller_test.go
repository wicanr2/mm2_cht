package mobileui

import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/i18n"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

func load(t *testing.T) (*Controller, string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := cwd
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		next := filepath.Dir(root)
		if next == root {
			t.Fatal("missing project root")
		}
		root = next
	}
	private := t.TempDir()
	for _, name := range []string{"assets", "data", "translations", "workplace"} {
		if err := os.Symlink(filepath.Join(root, name), filepath.Join(private, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chdir(private); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(cwd) })
	s, err := ui.Load(filepath.Join(root, "workplace/orig/MM2"))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.InitPresentation(root, filepath.Join(root, "workplace/hd-amiga-r1"), filepath.Join(root, "workplace/amiga")); err != nil {
		t.Fatal(err)
	}
	p := ui.DefaultPresentation()
	p.HD = true
	p.Platform = "amiga"
	if err = s.SetPresentation(p); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(root, "workplace/hd-amiga-r1/frame.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	frame, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return New(s, frame), root
}

func TestPhoneGesturesAndCheckpoints(t *testing.T) {
	c, root := load(t)
	s := c.Session
	face := s.Game.World.Face
	initialFace := face
	c.Touch(1, 155, 205, true, 0)
	c.Touch(1, 700, 20, true, 10*time.Millisecond)
	c.Touch(1, 155, 205, false, 20*time.Millisecond)
	if s.Game.World.Face != face {
		t.Fatal("dragging out must cancel")
	}
	c.Touch(1, 155, 205, true, 0)
	c.Touch(2, 40, 205, true, 0)
	c.Touch(2, 40, 205, false, 10*time.Millisecond)
	c.Touch(1, 155, 205, false, 20*time.Millisecond)
	if s.Game.World.Face != (face+1)%4 {
		t.Fatal("second finger dispatched or owner lost")
	}
	face = s.Game.World.Face
	c.Touch(1, 155, 205, true, 0)
	c.Touch(1, 155, 205, true, 400*time.Millisecond)
	c.Touch(1, 155, 205, true, 580*time.Millisecond)
	c.Touch(1, 155, 205, false, 600*time.Millisecond)
	if s.Game.World.Face != (face+2)%4 {
		t.Fatal("hold must repeat twice, without extra release action")
	}
	c.Touch(1, 155, 205, true, 0)
	s.Key(ui.KeyMap)
	c.Touch(1, 155, 205, false, 20*time.Millisecond)
	if s.Mode != ui.ModeMap {
		t.Fatal("release leaked into new mode")
	}
	s.Key(ui.KeyCancel)
	c.Act("save")
	s.Key(ui.KeyConfirm)
	before, err := os.ReadFile(ui.SavePath)
	if err != nil {
		t.Fatal(err)
	}
	s.Key(ui.KeyCreate)
	if err = s.Checkpoint(); err == nil {
		t.Fatal("incomplete character cannot checkpoint")
	}
	after, err := os.ReadFile(ui.SavePath)
	if err != nil || string(before) != string(after) {
		t.Fatal("unsafe checkpoint changed the save")
	}
	s.Mode = ui.ModeExplore
	s.Game.World.Face = initialFace
	for _, locale := range []string{i18n.Traditional, i18n.Simplified} {
		p := s.Presentation()
		p.Locale = locale
		s.SetPresentation(p)
		for _, target := range c.Targets() {
			for _, r := range target.Label {
				font := s.DisplayCatalog().Font(locale)
				if r < 128 {
					font = s.DisplayCatalog().Sans
				}
				if !font.Has(r) {
					t.Errorf("missing glyph %U for %s", r, target.ID)
				}
			}
		}
	}
	p := s.Presentation()
	p.Locale = i18n.Traditional
	s.SetPresentation(p)
	if out := os.Getenv("MM2_MOBILE_SHOTS"); out != "" {
		if !filepath.IsAbs(out) {
			out = filepath.Join(root, out)
		}
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for _, state := range []string{"explore", "more", "menu", "create"} {
			s.Mode = ui.ModeExplore
			c.Overlay = ""
			c.Page = 0
			if state == "more" {
				c.Act("more")
			}
			if state == "menu" {
				c.Act("shop")
			}
			if state == "create" {
				c.Act("create")
			}
			f, err := os.Create(filepath.Join(out, state+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, c.Draw())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPhoneInputIsAtomicAndCancelsPreedit(t *testing.T) {
	c, _ := load(t)
	s := c.Session
	s.Mode = ui.ModeText
	s.PromptText = "原文"
	c.InputToken = 9
	if c.Submit("這個輸入字數超過十個字元的限制", 9, true) {
		t.Fatal("overlong input accepted")
	}
	if s.PromptText != "原文" {
		t.Fatal("rejected input mutated committed text")
	}
	if c.Submit("測試@文字", 9, true) {
		t.Fatal("invalid character silently removed")
	}
	if s.PromptText != "原文" {
		t.Fatal("invalid input mutated committed text")
	}
	if !c.Submit("测试組字", 9, false) || s.PromptText != "原文" {
		t.Fatal("cancel submitted preedit")
	}
	if c.Submit("测试", 8, true) {
		t.Fatal("stale native editor accepted")
	}
}

func TestPhoneMenuSelectionUsesConfirmation(t *testing.T) {
	c, _ := load(t)
	s := c.Session
	menu := &ui.Menu{Title: "fixture", Items: make([]string, 12)}
	s.Mode = ui.ModeMenu
	s.Menu = menu
	if !s.SelectMenu(10) || menu.Cur != 10 {
		t.Fatal("direct selection must reach entries beyond digit nine")
	}
	if s.SelectMenu(-1) {
		t.Fatal("invalid row accepted")
	}
}

func TestPhoneMenuTapAndLongPressUseEventTime(t *testing.T) {
	c, _ := load(t)
	s := c.Session
	c.Act("create")
	c.Act("classes")
	for _, target := range c.Targets() {
		if target.Enabled && strings.HasPrefix(target.ID, "class:") {
			c.Act(target.ID)
			break
		}
	}
	before := s.Menu
	// A delayed frame must replay these original event times in order. The
	// MOVE belongs to a short tap and must not focus without confirming.
	c.Touch(1, 422, 124, true, 0)
	c.Touch(1, 422, 124, true, 100*time.Millisecond)
	c.Touch(1, 422, 124, false, 160*time.Millisecond)
	if s.Menu == before || s.Mode != ui.ModeMenu {
		t.Fatal("short menu tap did not confirm")
	}
	before = s.Menu
	c.Touch(1, 422, 124, true, time.Second)
	c.Touch(1, 422, 124, true, 1400*time.Millisecond)
	c.Touch(1, 422, 124, false, 1450*time.Millisecond)
	if s.Menu != before || s.Menu.Cur != 0 {
		t.Fatal("long press should only focus the menu row")
	}
	c.Act("confirm")
	if s.Menu == before {
		t.Fatal("explicit confirmation did not advance the focused row")
	}
}

func TestPhoneTextPagesAndItemFocus(t *testing.T) {
	c, _ := load(t)
	s := c.Session
	s.Mode = ui.ModeMessage
	s.Lines = []string{strings.Repeat("長對話", 180)}
	pages := c.textPages()
	if pages < 2 {
		t.Fatal("fixture must require paging")
	}
	c.Act("confirm")
	if s.Mode != ui.ModeMessage || len(s.Lines) != 1 || c.Page != 1 {
		t.Fatal("reading a page advanced the game")
	}
	for c.Page+1 < pages {
		c.Act("confirm")
	}
	c.Act("confirm")
	if s.Mode != ui.ModeExplore {
		t.Fatal("last page did not confirm the message")
	}
	c.Act("items")
	if !s.ItemsOpen() || len(s.Menu.Items) < 2 {
		t.Fatal("missing inventory fixture")
	}
	before, _ := json.Marshal(s.Game.State())
	c.Act("menu:1")
	after, _ := json.Marshal(s.Game.State())
	if !s.ItemsOpen() || s.Menu.Cur != 1 || string(before) != string(after) {
		t.Fatal("touch selection executed an item action")
	}
}

// Controlled remake fixture, shared with ui/settings_test.go. This checks the
// phone adapter against the keyboard path, not original DOS combat parity.
func TestPhoneCombatActionsKeepKeyboardResults(t *testing.T) {
	for _, quick := range []bool{false, true} {
		var expected []byte
		for _, phone := range []bool{false, true} {
			t.Run(fmt.Sprintf("quick=%v/phone=%v", quick, phone), func(t *testing.T) {
				c, _ := load(t)
				s := c.Session
				var definition monsters.Monster
				definition.Index, definition.HP, definition.SpecialUses, definition.Speed, definition.AC = 0x21, 1, 1, 1, 1
				definition.DropBand, definition.GoldMode, definition.GemDrop = 1, 1, true
				party := []game.Combatant{}
				for i := range s.Game.Party {
					party = append(party, &s.Game.Party[i])
				}
				s.Game.Fight = &game.Encounter{Party: party, Monsters: []game.Combatant{game.NewMonster(definition)}, Front: 1}
				s.Game.Rand = game.NewRand(0x1234)
				s.Mode = ui.ModeCombat
				for _, target := range c.Targets() {
					if (target.ID == "forward" || target.ID == "items") && target.Enabled {
						t.Fatal("unavailable combat command enabled")
					}
				}
				if quick {
					if phone {
						c.Act("more")
						c.Act("quick")
					} else {
						s.Key(ui.KeyQuickFight)
					}
				} else {
					for n := 0; n < 20 && s.Mode == ui.ModeCombat; n++ {
						if phone {
							c.Act("attack")
						} else {
							s.Key(ui.KeyConfirm)
						}
					}
				}
				if s.Mode == ui.ModeCombat || s.Chest == nil {
					t.Fatal("combat did not settle through the existing reward path")
				}
				result, _ := json.Marshal(struct {
					State game.State
					Chest *game.Chest
					Mode  ui.Mode
				}{s.Game.State(), s.Chest, s.Mode})
				if !phone {
					expected = result
				} else if string(expected) != string(result) {
					t.Fatal("phone adapter changed combat results")
				}
			})
		}
	}
}
