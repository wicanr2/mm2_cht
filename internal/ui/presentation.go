package ui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/cjk"
	"github.com/wicanr2/mm2_cht/internal/assets/hd"
	"github.com/wicanr2/mm2_cht/internal/i18n"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// Presentation is stored separately from the game save.
type Presentation struct {
	Locale         string `json:"locale"`
	Platform       string `json:"platform"`
	HD             bool   `json:"hd"`
	TextPercent    int    `json:"text_percent"`
	ToolbarPercent int    `json:"toolbar_percent"`
}

type presentationState struct {
	current                  Presentation
	display                  *i18n.Display
	art                      *hd.Bundle
	artError                 error
	classicCJK, classicLatin *cjk.Font
	monsterMasks             map[*image.Paletted]*image.Alpha
	inputPreview             string
	inputPreviewActive       bool
	inputError               string
}

func DefaultPresentation() Presentation {
	return Presentation{Locale: i18n.Traditional, Platform: "amiga", TextPercent: 100, ToolbarPercent: 100}
}

// RuntimeAssetPath lets packaged read-only assets live outside the save folder.
// Repository runs retain the existing workplace layout by default.
func RuntimeAssetPath(relative string) string {
	root := os.Getenv("MM2_ASSET_ROOT")
	if root == "" {
		root = "workplace"
	}
	return filepath.Join(root, relative)
}

func (s *Session) InitPresentation(root, artDir, amigaPath string) error {
	d, err := i18n.LoadDisplay(root)
	if err != nil {
		return err
	}
	s.presentation = presentationState{display: d, classicCJK: s.Assets.CJK.WithFallback(d.InputTC), classicLatin: s.Assets.Latin, monsterMasks: map[*image.Paletted]*image.Alpha{}}
	if artDir == "" {
		artDir = RuntimeAssetPath("hd-amiga-r1")
	}
	if amigaPath == "" {
		amigaPath = RuntimeAssetPath("amiga")
	}
	s.presentation.art, s.presentation.artError = hd.Load(artDir, amigaPath, RuntimeAssetPath("amiga-monsters"))
	p := DefaultPresentation()
	p.HD = s.HDReady()
	if !s.HasPlatform(p.Platform) {
		p.Platform = "dos"
	}
	return s.SetPresentation(p)
}

func (s *Session) DisplayCatalog() *i18n.Display { return s.presentation.display }
func (s *Session) HDReady() bool                 { return s.presentation.art != nil && s.HasPlatform("amiga") }
func (s *Session) HDError() error                { return s.presentation.artError }
func (s *Session) Presentation() Presentation    { return s.presentation.current }

var platformIDs = map[view.Platform]string{view.PlatformDOS: "dos", view.PlatformAmiga: "amiga", view.PlatformMSX: "msx", view.PlatformMegaDrive: "megadrive", view.PlatformModern: "modern"}

func (s *Session) Platforms() []string {
	var out []string
	seen := map[string]bool{}
	for _, set := range s.sets {
		if set != nil {
			id := platformIDs[set.Platform]
			if !seen[id] {
				out = append(out, id)
				seen[id] = true
			}
		}
	}
	return out
}
func (s *Session) HasPlatform(id string) bool {
	for _, p := range s.Platforms() {
		if p == id {
			return true
		}
	}
	return false
}

// SetPresentation validates everything before changing any visible component.
func (s *Session) SetPresentation(p Presentation) error {
	if s.presentation.display == nil {
		return fmt.Errorf("display catalog is not initialized")
	}
	if !i18n.ValidLocale(p.Locale) || (p.TextPercent != 80 && p.TextPercent != 90 && p.TextPercent != 100) || (p.ToolbarPercent != 100 && p.ToolbarPercent != 125 && p.ToolbarPercent != 150) {
		return fmt.Errorf("invalid display preferences")
	}
	if p.HD {
		p.Platform = "amiga"
		if !s.HDReady() {
			return fmt.Errorf("HD unavailable: %v", s.presentation.artError)
		}
	}
	index := -1
	for i, set := range s.sets {
		if set != nil && platformIDs[set.Platform] == p.Platform {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("unavailable platform %q", p.Platform)
	}
	for _, set := range s.sets {
		if set != nil {
			set.HD = false
		}
	}
	s.setIdx = index
	s.Assets.Town = s.sets[index]
	s.Assets.Town.HD = p.HD
	if p.HD {
		s.Assets.Town.Artwork = s.presentation.art.Art
	}
	s.Assets.CJK, s.Assets.Latin = s.presentation.classicCJK, s.presentation.classicLatin
	if p.HD || p.Locale == i18n.Simplified || p.TextPercent < 100 {
		s.Assets.CJK = s.presentation.display.Font(p.Locale)
		s.Assets.Latin = s.presentation.display.Sans
	}
	if p.HD {
		s.Assets.Latin = s.presentation.display.Serif
	}
	s.presentation.current = p
	return nil
}

func (s *Session) prepareDisplay() {
	d := s.presentation.display
	if d == nil {
		return
	}
	p := s.presentation.current
	s.scr.Text = func(text string) string { return d.Convert(p.Locale, text) }
	s.scr.TextPercent = p.TextPercent
	// F6 remains a valid keyboard path; keep toolbar state synchronized.
	if s.Assets.Town != nil && platformIDs[s.Assets.Town.Platform] != p.Platform {
		p.Platform = platformIDs[s.Assets.Town.Platform]
		p.HD = false
		_ = s.SetPresentation(p)
	}
}

func (s *Session) composeDisplay() {
	if s.presentation.current.HD && s.presentation.art != nil {
		page := s.Mode == ModeMap || s.Mode == ModeWorld || s.Mode == ModeIntro || s.Mode == ModeControl || s.Mode == ModeDead
		s.presentation.art.Compose(s.scr.Hi, page)
	}
}

func (s *Session) amigaMonsterPackIndex() int {
	for i, pack := range s.monSets {
		if pack != nil && strings.Contains(strings.ToLower(pack.Source), "amiga") {
			return i
		}
	}
	return -1
}
