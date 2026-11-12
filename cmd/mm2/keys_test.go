package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

func TestNKeyUsesItsAcceptedContext(t *testing.T) {
	s := &ui.Session{Game: game.NewSession(&game.World{}, nil, nil, 0x1234)}
	if !dispatchKey(s, ebiten.KeyN) || s.Mode != ui.ModeCreate {
		t.Fatal("N did not open character creation")
	}
	s.Mode = ui.ModeMenu
	s.Menu = &ui.Menu{Items: []string{"fixture"}}
	if !dispatchKey(s, ebiten.KeyN) || s.Mode != ui.ModeExplore {
		t.Fatal("N did not keep its cancel meaning in menus")
	}
}
