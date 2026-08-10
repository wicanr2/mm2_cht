package game_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 箱子的名字反映裡面有多少東西 —— 品質是算出來的，不是隨機挑的。
func TestChestQualityFollowsContents(t *testing.T) {
	empty := &game.Chest{}
	if got := empty.Quality(); got != 0 {
		t.Errorf("空箱子的品質 %d，預期 0", got)
	}
	rich := &game.Chest{Gold: 500, Gems: 20}
	rich.Items[0] = game.ChestItem{ID: 30, Level: 20}
	rich.Items[1] = game.ChestItem{ID: 31, Level: 8}
	rich.Items[2] = game.ChestItem{ID: 32, Level: 4}
	// 五個來源各一分 + 等級 > 1 一分 + 20/4 = 5 → 11，夾在 8，減一 = 7
	if got := rich.Quality(); got != 7 {
		t.Errorf("塞滿的箱子品質 %d，預期 7", got)
	}
	if empty.Name() == rich.Name() {
		t.Errorf("空箱子與滿箱子同名：%s", empty.Name())
	}
	t.Logf("空 %q／滿 %q", empty.Name(), rich.Name())
}

// 開箱要把東西給隊伍，而且只給一次。
func TestChestOpenGivesLoot(t *testing.T) {
	s := session(t)
	gold := s.Party[0].Gold
	c := &game.Chest{Gold: 250, Gems: 7}
	c.Items[0] = game.ChestItem{ID: 30, Level: 3}

	r := s.Do(c, game.ChestOpen, 0)
	if !r.Opened || !r.Done {
		t.Fatalf("開箱沒有回報：%+v", r)
	}
	if s.Party[0].Gold != gold+250 {
		t.Errorf("金幣 %d → %d，預期加 250", gold, s.Party[0].Gold)
	}
	if s.Party[0].Gems != 7 {
		t.Errorf("寶石 %d，預期 7", s.Party[0].Gems)
	}
	found := false
	for _, it := range s.Party[0].Backpack() {
		if it.ID == 30 {
			found = true
		}
	}
	if !found {
		t.Error("物品沒有進背包")
	}
	// 再開一次不該再給一份
	before := s.Party[0].Gold
	s.Do(c, game.ChestOpen, 0)
	if s.Party[0].Gold != before {
		t.Errorf("開了兩次拿到兩份金幣：%d → %d", before, s.Party[0].Gold)
	}
}

// 陷阱的判定：盜行 0 幾乎一定中，盜行 96 以上仍有 4% 中 —— 技能擋不掉那一段。
func TestChestTrapOdds(t *testing.T) {
	s := session(t)
	count := func(thievery int) int {
		n := 0
		for i := 0; i < 400; i++ {
			s.Party[0].Thievery = thievery
			s.Party[0].Condition = game.CondGood
			s.Party[0].HP = s.Party[0].MaxHP
			c := &game.Chest{Trap: 1}
			if s.Do(c, game.ChestOpen, 0).Sprung {
				n++
			}
		}
		return n
	}
	low, high := count(0), count(100)
	if low < 350 {
		t.Errorf("盜行 0 只中了 %d/400 次陷阱", low)
	}
	if high == 0 {
		t.Error("盜行 100 完全不會中陷阱 —— 那 4% 沒實作")
	}
	if high > 60 {
		t.Errorf("盜行 100 中了 %d/400 次，遠高於預期的 4%%", high)
	}
	t.Logf("盜行 0：%d/400；盜行 100：%d/400", low, high)
}

// 找陷阱做完一定接著開箱（原版 sub_1C824(0FFh)）。
func TestChestFindTrapThenOpens(t *testing.T) {
	s := session(t)
	s.Party[0].Thievery = 100
	c := &game.Chest{Trap: 2, Gold: 100}
	r := s.Do(c, game.ChestFind, 0)
	if !r.Opened {
		t.Errorf("找完陷阱沒有接著開箱：%v", r.Lines)
	}
	if !strings.Contains(strings.Join(r.Lines, "|"), "打開了") {
		t.Errorf("播報裡沒有開箱：%v", r.Lines)
	}
}

// 偵測魔法不擲骰、不會觸發陷阱。
func TestChestDetectIsSafe(t *testing.T) {
	s := session(t)
	s.Party[0].Thievery = 0
	c := &game.Chest{Trap: 5}
	c.Items[0] = game.ChestItem{ID: 30, Level: 1}
	for i := 0; i < 50; i++ {
		if r := s.Do(c, game.ChestDetect, 0); r.Sprung || r.Opened {
			t.Fatalf("偵測魔法出事了：%+v", r)
		}
	}
	line := strings.Join(s.Do(c, game.ChestDetect, 0).Lines, "")
	if !strings.Contains(line, "有魔法") || !strings.Contains(line, "有陷阱") {
		t.Errorf("偵測的播報不完整：%s", line)
	}
}
