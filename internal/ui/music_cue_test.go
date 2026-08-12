package ui

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

func musicCueSession() *Session {
	return &Session{Game: &game.Session{World: &game.World{MapIndex: 5}}}
}

func TestMusicCueBattleHasPriority(t *testing.T) {
	s := musicCueSession()
	s.Mode = ModeCombat
	s.Game.Fight = &game.Encounter{}
	s.Mode = ModeMenu
	s.menuKind = menuTemple
	if got := s.MusicCue(); got != MusicCueBattle {
		t.Fatalf("戰鬥應優先於選單：得到 %q", got)
	}
}

func TestMusicCueFacilityMenus(t *testing.T) {
	tests := []struct {
		name string
		kind menuKind
		want MusicCue
	}{
		{"temple", menuTemple, MusicCueTemple},
		{"temple buy", menuTempleBuy, MusicCueTemple},
		{"inn", menuInn, MusicCueInn},
		{"inn add", menuInnAdd, MusicCueInn},
		{"inn drop", menuInnDrop, MusicCueInn},
		{"smith", menuSmith, MusicCueBlacksmith},
		{"smith sell", menuSmithSell, MusicCueBlacksmith},
		{"smith identify", menuSmithIdent, MusicCueBlacksmith},
		{"tavern", menuTavern, MusicCueTavern},
		{"training", menuTrain, MusicCueTraining},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := musicCueSession()
			s.Mode, s.menuKind = ModeMenu, tt.kind
			if got := s.MusicCue(); got != tt.want {
				t.Fatalf("得到 %q，預期 %q", got, tt.want)
			}
		})
	}
}

func TestMusicCueMapClassification(t *testing.T) {
	for i := 0; i <= 4; i++ {
		s := musicCueSession()
		s.Game.World.MapIndex = i
		if got := s.MusicCue(); got != MusicCueTown {
			t.Errorf("地圖 %d 得到 %q，預期 town", i, got)
		}
	}
	for _, tt := range []struct {
		name   string
		indoor bool
		want   MusicCue
	}{
		{"indoor", true, MusicCueDungeon},
		{"outdoor", false, MusicCueOutside},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := musicCueSession()
			s.Game.Attrs = make([]game.MapAttr, 6)
			s.Game.Attrs[5].Raw[18] = 0
			if tt.indoor {
				s.Game.Attrs[5].Raw[18] = 1
			}
			if got := s.MusicCue(); got != tt.want {
				t.Fatalf("得到 %q，預期 %q", got, tt.want)
			}
		})
	}
}

func TestMusicCueMissingAttrIsConservative(t *testing.T) {
	s := musicCueSession()
	if got := s.MusicCue(); got != MusicCueUnknown {
		t.Fatalf("缺少 ATTRIB 時得到 %q，預期 unknown", got)
	}
	if got := (*Session)(nil).MusicCue(); got != MusicCueUnknown {
		t.Fatalf("nil session 得到 %q，預期 unknown", got)
	}
}
