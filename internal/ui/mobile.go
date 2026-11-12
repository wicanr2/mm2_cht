package ui

import (
	"fmt"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// SelectMenu uses the same confirmation path as keyboard/digit selection.
func (s *Session) SelectMenu(index int) bool {
	if s.Mode != ModeMenu || s.Menu == nil || index < 0 || index >= len(s.Menu.Items) {
		return false
	}
	s.Menu.Cur = index
	return s.Key(KeyConfirm)
}

func (s *Session) SelectParty(index int) bool {
	if index < 0 || index >= len(s.Game.Party) || (s.Mode != ModeExplore && s.Mode != ModeCombat) {
		return false
	}
	s.who = index
	return true
}

func (s *Session) PartyDetails(index int) []string { return s.viewLines(index) }
func (s *Session) LocationLabel() string           { return s.mapTitle() }
func (s *Session) ItemsOpen() bool                 { return s.Mode == ModeMenu && s.menuKind == menuItems }

// A checkpoint intentionally excludes continuations which the JSON does not
// preserve. Suspending a process in another mode retains its in-memory state.
func (s *Session) CanCheckpoint() bool {
	return s.Mode == ModeExplore && s.Game != nil && s.Game.World != nil &&
		s.Game.World.Pending == nil && s.Game.Fight == nil && s.Chest == nil
}

func (s *Session) Checkpoint() error {
	if !s.CanCheckpoint() {
		return fmt.Errorf("current interaction is not a checkpoint")
	}
	return s.saveFiles()
}

func (s *Session) InputLimit() int {
	if s.Mode == ModeControl && s.controlStage == stageCipher {
		return controlInputMax
	}
	return maxNameLen
}

func (s *Session) MobileTextPage() (view.TextPage, bool) {
	switch s.Mode {
	case ModeControl:
		return s.controlPage(), true
	case ModeDead:
		return s.deadPage(), true
	}
	return view.TextPage{}, false
}
