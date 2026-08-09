package game_test

import (
	"bytes"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 解開再寫回，必須與原檔一個位元組不差。
//
// 這條守著兩件事：已解欄位的寫回位置正確，以及**未解欄位沒被洗掉**。
// remake 只解了 15/21 個欄位，如果寫回時把沒解的清成 0，
// 存檔就會毀掉原版的資料。
func TestRoundTripIsByteExact(t *testing.T) {
	for _, f := range []string{"DEFAULT.DAT", "ROSTER.DAT"} {
		blob := orig(t, f)
		cs, err := game.ParseCharacters(blob)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		got, err := game.EncodeRoster(cs, blob)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if !bytes.Equal(got, blob) {
			n := 0
			var first int = -1
			for i := range blob {
				if got[i] != blob[i] {
					n++
					if first < 0 {
						first = i
					}
				}
			}
			t.Errorf("%s：%d 個位元組不同，第一個在 +%d（記錄 %d 的 +%d）：%#x → %#x",
				f, n, first, first/game.RecordSize, first%game.RecordSize,
				blob[first], got[first])
			continue
		}
		t.Logf("%s：%d bytes 完全一致", f, len(blob))
	}
}

// 改過的欄位要真的寫進去。
func TestEncodeAppliesChanges(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	c := cs[0]
	c.HP = 7
	c.Level = 12
	c.Food = 3
	c.Base[game.Might] = 19
	back, err := game.ParseCharacters(c.Encode())
	if err != nil {
		t.Fatal(err)
	}
	g := back[0]
	if g.HP != 7 || g.Level != 12 || g.Food != 3 || g.Base[game.Might] != 19 {
		t.Errorf("寫回後是 HP=%d 等級=%d 食物=%d 力量=%d",
			g.HP, g.Level, g.Food, g.Base[game.Might])
	}
	if g.Name != c.Name || g.Class != c.Class {
		t.Errorf("名稱或職業跑掉了：%q %v", g.Name, g.Class)
	}
}

// 走一段路、打一場、存檔、讀回來 —— 狀態要對得上。
func TestSessionRoundTrip(t *testing.T) {
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	w.MapIndex, w.X, w.Y, w.Face = 0, 7, 8, game.South
	blob := orig(t, "DEFAULT.DAT")
	party, err := game.ParseCharacters(blob)
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, party, nil, 4321)
	for i := 0; i < 2; i++ {
		s.Step(1)
	}
	// 讓第一個人受傷，看存檔有沒有帶上
	s.Party[0].TakeDamage(5)
	hp := s.Party[0].HP

	out, err := game.EncodeRoster(s.Party, blob)
	if err != nil {
		t.Fatal(err)
	}
	back, err := game.ParseCharacters(out)
	if err != nil {
		t.Fatal(err)
	}
	if back[0].HP != hp {
		t.Errorf("讀回來的 HP 是 %d，存的時候是 %d", back[0].HP, hp)
	}
	// 沒動過的角色要與原檔一致
	origParty, _ := game.ParseCharacters(blob)
	for i := 1; i < len(back); i++ {
		if back[i].HP != origParty[i].HP || back[i].Name != origParty[i].Name {
			t.Errorf("第 %d 個角色被改動了", i)
		}
	}
}
