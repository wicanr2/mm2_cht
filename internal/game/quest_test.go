package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 指派的目標要落在該難度的範圍裡：Slayer 三段怪物編號、
// Hoardall 六個裝備類別。擲很多次才驗得出範圍。
func TestAssignQuestRanges(t *testing.T) {
	slayer := [3][2]int{{32, 79}, {80, 143}, {144, 191}}
	hoard := [3][][2]int{
		{{1, 24}, {66, 78}, {92, 97}, {115, 117}, {127, 134}, {155, 156}},
		{{25, 53}, {79, 84}, {98, 104}, {118, 124}, {135, 149}, {157, 158}},
		{{54, 65}, {85, 91}, {105, 114}, {125, 126}, {150, 154}, {159, 159}},
	}
	in := func(v int, rs [][2]int) bool {
		for _, r := range rs {
			if v >= r[0] && v <= r[1] {
				return true
			}
		}
		return false
	}
	for d := 0; d < 3; d++ {
		for n := 0; n < 60; n++ {
			s := caveSession(t)
			s.AssignQuest(game.LordSlayer, game.QuestDifficulty(d))
			got, lord := game.QuestTarget(&s.Party[0])
			if lord != game.LordSlayer {
				t.Fatalf("難度 %d 的委託人是 %v", d, lord)
			}
			if got < slayer[d][0] || got > slayer[d][1] {
				t.Fatalf("Slayer 難度 %d 擲出 %d，超出 %v", d, got, slayer[d])
			}

			s2 := caveSession(t)
			s2.AssignQuest(game.LordHoardall, game.QuestDifficulty(d))
			got2, lord2 := game.QuestTarget(&s2.Party[0])
			if lord2 != game.LordHoardall {
				t.Fatalf("難度 %d 的委託人是 %v", d, lord2)
			}
			if !in(got2, hoard[d]) {
				t.Fatalf("Hoardall 難度 %d 擲出 %d，不在六個類別裡", d, got2)
			}
		}
	}
}

// 全隊拿到同一個目標，而且已經在任務中就不再指派。
func TestAssignQuestOncePerParty(t *testing.T) {
	s := caveSession(t)
	if lines := s.AssignQuest(game.LordSlayer, game.QuestPage); len(lines) == 0 {
		t.Fatal("第一次指派應該要有回話")
	}
	want, _ := game.QuestTarget(&s.Party[0])
	for i := range s.Party {
		if got, _ := game.QuestTarget(&s.Party[i]); got != want {
			t.Errorf("第 %d 人的目標是 %d，預期全隊都是 %d", i+1, got, want)
		}
	}
	if lines := s.AssignQuest(game.LordSlayer, game.QuestKnight); len(lines) != 0 {
		t.Error("已經在任務中卻又指派了一次")
	}
	if s.QuestPending(game.LordSlayer) == "" {
		t.Error("應該回報任務進行中")
	}
}

// 打死指派的那隻才算數，獎勵依怪物編號分級。
func TestSlayerTurnIn(t *testing.T) {
	s := caveSession(t)
	s.AssignQuest(game.LordSlayer, game.QuestPage)
	target, _ := game.QuestTarget(&s.Party[0])
	before := s.Party[0].Exp

	// 還沒打死：不給獎。
	if lines := s.TurnInQuest(game.LordSlayer); len(lines) != 0 {
		t.Errorf("還沒打死就給獎了：%v", lines)
	}

	// 打死別隻也不算。
	s.MarkQuestKillForTest(target + 1)
	if lines := s.TurnInQuest(game.LordSlayer); len(lines) != 0 {
		t.Errorf("打死別隻也算：%v", lines)
	}

	s.MarkQuestKillForTest(target)
	if lines := s.TurnInQuest(game.LordSlayer); len(lines) == 0 {
		t.Fatal("打死目標之後應該領得到獎")
	}
	// 難度 A 的怪物編號 32–79，門檻表對到 2,000 或 4,000。
	got := s.Party[0].Exp - before
	if got != 2000 && got != 4000 && got != 5000 {
		t.Errorf("獎勵是 %d，不在門檻表的前三級", got)
	}
	if tgt, _ := game.QuestTarget(&s.Party[0]); tgt != 0 {
		t.Errorf("結算後目標沒清掉：%d", tgt)
	}
}

// Hoardall 要把東西交出來，獎勵是那件物品的價格。
func TestHoardallTurnIn(t *testing.T) {
	s := caveSession(t)
	s.AssignQuest(game.LordHoardall, game.QuestPage)
	target, _ := game.QuestTarget(&s.Party[0])
	before := s.Party[0].Exp

	if lines := s.TurnInQuest(game.LordHoardall); len(lines) != 0 {
		t.Errorf("身上沒有那件東西卻給獎了：%v", lines)
	}
	s.Party[0].SetFieldByte(58, 0, byte(target))
	if lines := s.TurnInQuest(game.LordHoardall); len(lines) == 0 {
		t.Fatal("交出東西之後應該領得到獎")
	}
	if s.Party[0].Exp <= before {
		t.Errorf("經驗沒有增加：%d → %d", before, s.Party[0].Exp)
	}
	if got := s.Party[0].FieldByte(58); int(got) == target {
		t.Error("東西沒有被收走")
	}
}

// 領主任務：三把劍湊齊才算，獎勵是固定的 100,000。
func TestLordQuestSwords(t *testing.T) {
	s := caveSession(t)
	if lines := s.AssignQuest(game.LordHoardall, game.QuestFinal); len(lines) == 0 {
		t.Fatal("領主任務應該接得下來")
	}
	if !game.QuestActive(&s.Party[0]) {
		t.Fatal("領主任務沒有標成進行中")
	}
	before := s.Party[0].Exp

	// 只有兩把不算。
	s.Party[0].SetFieldByte(58, 0, 226)
	s.Party[0].SetFieldByte(59, 0, 227)
	if lines := s.TurnInQuest(game.LordHoardall); len(lines) != 0 {
		t.Errorf("只有兩把劍就給獎了：%v", lines)
	}
	s.Party[0].SetFieldByte(60, 0, 228)
	if lines := s.TurnInQuest(game.LordHoardall); len(lines) == 0 {
		t.Fatal("三把湊齊應該領得到獎")
	}
	if got := s.Party[0].Exp - before; got != 100000 {
		t.Errorf("獎勵是 %d，預期 100,000", got)
	}
	if !game.QuestDone(&s.Party[0], game.LordHoardall) {
		t.Error("沒有標成完成")
	}
	if game.QuestActive(&s.Party[0]) {
		t.Error("完成之後還標著進行中")
	}
	// 完成過就不再接。
	if lines := s.AssignQuest(game.LordHoardall, game.QuestFinal); len(lines) != 0 {
		t.Error("完成過的領主任務又接了一次")
	}
}
