package ui

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 五種設施各對到一個 key 前綴，其餘設施沒有描述。
func TestMDFlavorKinds(t *testing.T) {
	want := map[game.FacilityKind]string{
		game.FacilityInn:        "inn",
		game.FacilityBlacksmith: "blacksmith",
		game.FacilityTavern:     "tavern",
		game.FacilityTemple:     "temple",
		game.FacilityTraining:   "training",
		game.FacilityMageGuild:  "",
		game.FacilityArena:      "",
		game.FacilityNone:       "",
	}
	for k, w := range want {
		if got := mdFlavorKind(k); got != w {
			t.Errorf("%v 對到 %q，預期 %q", k, got, w)
		}
	}
}

// 開關關著就什麼都不講；打開才用 key 去查。
func TestMDFlavorNeedsTheSetting(t *testing.T) {
	s := &Session{
		Game:     &game.Session{World: &game.World{}, Facility: game.FacilityTemple},
		mdFlavor: map[string]string{"md.temple.0": "英文原文"},
	}
	if got := s.mdFlavorLine(); got != "" {
		t.Errorf("開關關著卻講了 %q", got)
	}
	s.Game.MDFlavor = true
	if got := s.mdFlavorLine(); got != "英文原文" {
		t.Errorf("沒有譯文時應該退回原文，得到 %q", got)
	}
	// 五座城之外（地城、野外）不講。
	s.Game.World.MapIndex = 11
	if got := s.mdFlavorLine(); got != "" {
		t.Errorf("城鎮之外卻講了 %q", got)
	}
}

// 譯文檔要**五種設施 × 五座城**一格不缺 —— 缺一格的症狀是那一間設施
// 突然冒出英文，而且只有走到那一座城才看得到。
func TestMDFlavorTranslationsAreComplete(t *testing.T) {
	b, err := os.ReadFile("../../translations/md-flavor.json")
	if err != nil {
		t.Skipf("沒有譯文檔：%v", err)
	}
	var rows []struct {
		Key    string `json:"key"`
		Hash   string `json:"src_sha8"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range rows {
		if r.Target == "" {
			t.Errorf("%s 沒有譯文", r.Key)
		}
		if len(r.Hash) != 8 {
			t.Errorf("%s 的原文雜湊是 %q，應該是 8 碼", r.Key, r.Hash)
		}
		got[r.Key] = true
	}
	for _, kind := range []string{"inn", "blacksmith", "tavern", "temple", "training"} {
		for town := 0; town < mdFlavorTowns; town++ {
			k := "md." + kind + "." + string(rune('0'+town))
			if !got[k] {
				t.Errorf("譯文檔缺 %s", k)
			}
		}
	}
	if len(rows) != 5*mdFlavorTowns {
		t.Errorf("譯文 %d 條，預期 %d 條", len(rows), 5*mdFlavorTowns)
	}
}
