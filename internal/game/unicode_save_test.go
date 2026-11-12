package game_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

func TestUnicodeCharacterSnapshotAndLegacyBoundary(t *testing.T) {
	party, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	w.SeedGlobals(orig(t, "MM2.EXE"))
	w.MarkExplored()
	s := game.NewSession(w, party, nil, 1234)
	s.Party[0].Name = "简体冒险者龙剑侠客甲"
	if s.Party[0].Empty() {
		t.Fatal("native Unicode character is empty")
	}
	for _, update := range []func(*game.Character){
		func(c *game.Character) { c.SetFieldByte(11, 0x80, 1) },
		func(c *game.Character) { c.SetFieldValue(98, 4, 1234) },
		func(c *game.Character) { c.Learn(48) },
		func(c *game.Character) { c.GivePackItem(game.ItemSlot{ID: 1}) },
		func(c *game.Character) { c.RemovePackItem(0) },
	} {
		update(&s.Party[0])
		if s.Party[0].Name != "简体冒险者龙剑侠客甲" {
			t.Fatal("legacy field update lost Unicode name")
		}
	}
	s.Party[0].HP = 7
	s.Roster = append(s.Roster, s.Party[0])
	before := s.State()
	blob, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	var saved game.State
	if err = json.Unmarshal(blob, &saved); err != nil {
		t.Fatal(err)
	}
	other := game.NewSession(w, nil, nil, 99)
	if err = other.LoadState(saved); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(other.State())
	if !bytes.Equal(blob, after) {
		t.Fatal("Unicode snapshot changed after load")
	}
	if other.Party[0].Name != s.Party[0].Name || other.Party[0].HP != 7 || len(other.Roster) != 1 {
		t.Fatal("character data lost")
	}
	if &other.World.Party[0] != &other.Party[0] {
		t.Fatal("world party disconnected")
	}
	legacy := other.Party[0].Encode()
	parsed, err := game.ParseCharacters(legacy)
	if err != nil || len(legacy) != game.RecordSize || parsed[0].Name != "PLAYER" {
		t.Fatal("legacy projection is not compatible")
	}
	bad := append([]byte(nil), legacy...)
	bad[0] = 0xff
	parsed, err = game.ParseCharacters(bad)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed[0].Empty() || !bytes.Equal(parsed[0].Encode(), bad) {
		t.Fatal("invalid legacy slot changed")
	}
	for _, version := range []int{1, 2} {
		old := before
		old.Version = version
		old.Characters = nil
		if err := other.LoadState(old); err != nil {
			t.Fatalf("old version %d: %v", version, err)
		}
	}
	valid, _ := json.Marshal(other.State())
	broken := saved
	broken.Characters = &game.SavedCharacters{Party: append([]game.Character(nil), saved.Characters.Party...)}
	broken.Characters.Party[0].Raw = []byte{1}
	if err := other.LoadState(broken); err == nil {
		t.Fatal("broken snapshot accepted")
	}
	unchanged, _ := json.Marshal(other.State())
	if !bytes.Equal(valid, unchanged) {
		t.Fatal("failed load partially mutated session")
	}
}
