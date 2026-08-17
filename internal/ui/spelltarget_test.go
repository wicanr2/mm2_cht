package ui_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

// castOne 讓某位施法者學會指定法術、法力灌滿，回傳那個人的編號。
//
// **順序有意義**：`SetFieldByte` 會改記錄再重新解析，先設好的 SP 會被
// 檔案裡的值蓋回去。
func castOne(t *testing.T, s *ui.Session, school game.SpellSchool, n int) int {
	t.Helper()
	for i := range s.Game.Party {
		c := &s.Game.Party[i]
		if !game.CanCast(c.Class) || game.SpellSchoolOf(c.Class) != school {
			continue
		}
		// 先把已學的位元清光，法術選單才只剩一項 —— 靠名字在選單裡
		// 找項目會被「這個人本來就會的法術」干擾。
		for off := 81; off < 87; off++ {
			c.SetFieldByte(off, 0x00, 0)
		}
		c.SetFieldByte(114, 0x00, 9) // 法力等級 9
		c.Learn(n)
		c.SP, c.MaxSP, c.Gems = 99, 99, 99
		return i
	}
	t.Skip("預設隊伍裡沒有這一系的施法者")
	return -1
}

// openCast 走完「施法 → 選人 → 選法術」，停在法術選好之後的下一步。
func openCast(t *testing.T, s *ui.Session, who, spell int) {
	t.Helper()
	s.Key(ui.KeyCast)
	if s.Menu == nil {
		t.Fatal("按施法沒有開選單")
	}
	// 施法者選單：挑到 who。
	for i := 0; i < len(s.Menu.Items); i++ {
		if strings.Contains(s.Menu.Items[i], s.Game.Party[who].Name) {
			s.Menu.Cur = i
			break
		}
	}
	s.Key(ui.KeyConfirm)
	if s.Menu == nil {
		t.Fatal("選完施法者之後沒有法術選單")
	}
	if len(s.Menu.Items) != 1 {
		t.Fatalf("法術選單有 %d 項，這個人只該會一條：%v", len(s.Menu.Items), s.Menu.Items)
	}
	s.Menu.Cur = 0
	s.Key(ui.KeyConfirm)
}

// 戰鬥中施單體法術要問「打哪一隻」，而且問的是**場上全部**
// —— 原版的提示是 `On which (A-J)?`，同一場戰鬥的近戰是
// `Fight which (A - E)?`（只有前排）。
func TestCombatSpellAsksWhichMonster(t *testing.T) {
	s := load(t)
	startFightN(t, s, 4, 2) // 場上四隻，前排只有兩隻
	who := castOne(t, s, game.SchoolCleric, 11)
	openCast(t, s, who, 11)
	if s.Menu == nil {
		t.Fatal("沒有開目標選單")
	}
	if len(s.Menu.Items) != 4 {
		t.Fatalf("法術列了 %d 個目標，場上有 4 隻（不是前排 2 隻）：%v",
			len(s.Menu.Items), s.Menu.Items)
	}
	if !strings.Contains(s.Menu.Title, "打哪一隻") {
		t.Errorf("標題看不出要選什麼：%q", s.Menu.Title)
	}
}

// 選了第三隻就要打第三隻。
func TestCombatSpellHitsChosenMonster(t *testing.T) {
	s := load(t)
	ms := startFightN(t, s, 3, 3)
	who := castOne(t, s, game.SchoolCleric, 11)
	openCast(t, s, who, 11)
	if s.Menu == nil {
		t.Fatal("沒有開目標選單")
	}
	before := make([]int, len(ms))
	for i, m := range ms {
		before[i] = m.CombatHP()
	}
	s.Menu.Cur = 2
	s.Key(ui.KeyConfirm)
	for i, m := range ms {
		hurt := m.CombatHP() < before[i]
		if (i == 2) != hurt {
			t.Errorf("第 %d 隻 hurt=%v（挑的是第 2 隻）", i, hurt)
		}
	}
}

// 目標提示按 Esc **一分不扣**。原版量到的就是這樣：
// 提示中 SP 200／寶石 99，Esc 之後仍然是 200／99（2026-08-17）。
// 需要數字的那幾條原版會扣，remake 刻意不照做，見 polish-spec D4。
func TestCancelMonsterPromptCostsNothing(t *testing.T) {
	s := load(t)
	startFightN(t, s, 3, 3)
	who := castOne(t, s, game.SchoolCleric, 11)
	sp, gems := s.Game.Party[who].SP, s.Game.Party[who].Gems
	openCast(t, s, who, 11)
	if s.Menu == nil {
		t.Fatal("沒有開目標選單")
	}
	s.Key(ui.KeyCancel)
	if got := s.Game.Party[who].SP; got != sp {
		t.Errorf("取消之後法力 %d，施法前是 %d", got, sp)
	}
	if got := s.Game.Party[who].Gems; got != gems {
		t.Errorf("取消之後寶石 %d，施法前是 %d", got, gems)
	}
}

// 只剩一隻就不問 —— 與攻擊那邊同一條規則（原版 `var_C <= 1`）。
func TestSingleMonsterNeedsNoPrompt(t *testing.T) {
	s := load(t)
	ms := startFightN(t, s, 1, 1)
	who := castOne(t, s, game.SchoolCleric, 11)
	before := ms[0].CombatHP()
	openCast(t, s, who, 11)
	if ms[0].CombatHP() >= before {
		t.Errorf("只有一隻怪，法術應該直接落下去，不必問")
	}
}
