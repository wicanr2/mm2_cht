package ui_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

// 功能鍵在任何畫面都是同一件事。這一份把那句話變成可以驗的東西。
//
// 為什麼要有：先前 `F4`／`F5`／`F6` 只寫在探索與戰鬥兩個分支裡，選單與
// 訊息模式一律吃掉不回應 —— 而**按了沒反應與程式當掉，玩家分不出來**。
// 文件（`CONTEXT.md`）早就寫著它們是全域的，只是沒有任何測試在守。

// setups 是幾個「不是探索模式」的畫面。每一個都要說得出怎麼進去。
var setups = []struct {
	name string
	open func(*testing.T, *ui.Session)
	want ui.Mode
}{
	{"選單", func(t *testing.T, s *ui.Session) {
		if !s.Key(ui.KeyCast) || s.Mode != ui.ModeMenu {
			t.Fatalf("進不了選單，模式是 %v", s.Mode)
		}
	}, ui.ModeMenu},
	{"訊息", func(t *testing.T, s *ui.Session) {
		if !s.Key(ui.KeySave) || s.Mode != ui.ModeMessage {
			t.Fatalf("進不了訊息模式，模式是 %v", s.Mode)
		}
	}, ui.ModeMessage},
	{"地圖", func(t *testing.T, s *ui.Session) {
		if !s.Key(ui.KeyMap) || s.Mode != ui.ModeMap {
			t.Fatalf("進不了地圖，模式是 %v", s.Mode)
		}
	}, ui.ModeMap},
	{"世界地圖", func(t *testing.T, s *ui.Session) {
		if !s.Key(ui.KeyWorld) || s.Mode != ui.ModeWorld {
			t.Fatalf("進不了世界地圖，模式是 %v", s.Mode)
		}
	}, ui.ModeWorld},
}

// F4／F5／F6 到哪裡都要有反應，而且**不能把玩家踢出目前的畫面**。
func TestSaveAndAssetKeysWorkEverywhere(t *testing.T) {
	for _, sc := range setups {
		for _, k := range []struct {
			name string
			key  ui.Key
		}{{"F4 存檔", ui.KeySave}, {"F5 樣式", ui.KeyStyle}, {"F6 平台", ui.KeyPlatform}} {
			s := load(t)
			sc.open(t, s)
			if !s.Key(k.key) {
				t.Errorf("%s：%s 沒有反應", sc.name, k.name)
			}
			if s.Mode != sc.want {
				t.Errorf("%s：按了 %s 之後模式變成 %v，應該留在 %v",
					sc.name, k.name, s.Mode, sc.want)
			}
		}
	}
}

// F1／F2 蓋上去之後要關得掉，而且關掉之後回得到原本的畫面。
func TestHelpAndSettingsToggle(t *testing.T) {
	for _, k := range []struct {
		name string
		key  ui.Key
	}{{"F1 說明", ui.KeyHelp}, {"F2 設定", ui.KeySettings}} {
		s := load(t)
		if !s.Key(k.key) || s.Mode != ui.ModeMenu {
			t.Fatalf("%s 打不開，模式是 %v", k.name, s.Mode)
		}
		// 再按一次收起來 —— 開了關不掉的說明頁比沒有說明頁更煩。
		if !s.Key(k.key) || s.Mode != ui.ModeExplore {
			t.Errorf("%s 再按一次沒有收起來，模式是 %v", k.name, s.Mode)
		}
	}
	// 從戰鬥開說明，Esc 之後要回到戰鬥，不是回到探索。
	s := load(t)
	startFight(t, s)
	if !s.Key(ui.KeyHelp) || s.Mode != ui.ModeMenu {
		t.Fatalf("戰鬥中 F1 打不開，模式是 %v", s.Mode)
	}
	if !s.Key(ui.KeyCancel) || s.Mode != ui.ModeCombat {
		t.Errorf("戰鬥中關掉說明之後模式是 %v，應該回戰鬥", s.Mode)
	}
}

// **有東西正等著這一顆按鍵的時候，功能鍵一律不介入。** 戰鬥中的「打哪一隻」
// 就是這種畫面：蓋一個說明選單上去之後那一回合的狀態沒有回得去的路，
// 存檔則會寫下一份讀回來就少了那個提示的狀態。
//
// 這不是當掉 —— `Esc` 取消掉提示之後功能鍵就恢復，最後那一段驗的就是這件事。
func TestOverlayKeysRefuseWhileChoosingTarget(t *testing.T) {
	s := load(t)
	// 前排要有兩隻以上才會問「打哪一隻」（原版 `var_C <= 1` 就不問）。
	startFightN(t, s, 3, 3)
	if !s.Key(ui.KeyConfirm) || s.Mode != ui.ModeMenu {
		t.Fatalf("攻擊沒有問「打哪一隻」，模式是 %v", s.Mode)
	}
	if s.Key(ui.KeyHelp) {
		t.Error("選目標的時候 F1 竟然蓋得上去")
	}
	if s.Key(ui.KeySettings) {
		t.Error("選目標的時候 F2 竟然蓋得上去")
	}
	if s.Mode != ui.ModeMenu {
		t.Errorf("模式變成 %v，選目標那一層被弄掉了", s.Mode)
	}
	// F4 也一樣擋掉：提示不是可存檔的狀態，讀回來就少了那個提示
	// （`TestTargetedSpellPromptCanCancelBeforeCost` 守的是同一件事）。
	if s.Key(ui.KeySave) {
		t.Error("選目標的時候 F4 竟然存了檔")
	}
	// 選目標那一層還在，數字鍵照樣選得到。
	if !s.PressDigit(1) {
		t.Error("擋掉功能鍵之後連數字鍵都不動了")
	}

	// 取消掉提示之後功能鍵要恢復 —— 「擋住」與「壞掉」的差別就在這裡。
	s2 := load(t)
	startFightN(t, s2, 3, 3)
	s2.Key(ui.KeyConfirm)
	if !s2.Key(ui.KeyCancel) || s2.Mode != ui.ModeCombat {
		t.Fatalf("Esc 沒有取消選目標，模式是 %v", s2.Mode)
	}
	if !s2.Key(ui.KeySave) || s2.Mode != ui.ModeCombat {
		t.Errorf("取消之後 F4 還是沒反應，或離開了戰鬥（%v）", s2.Mode)
	}
}

// Esc 一律是取消，**永遠不是離開遊戲**。
//
// 離開是 F10，而且會先自動存檔（`cmd/mm2` 的 `Update`）。先前 Esc 兼任離開，
// 在選單外按一下就直接關掉，進度沒了都不知道發生什麼事。
func TestEscapeCancelsAndNeverQuits(t *testing.T) {
	s := load(t)
	// 探索模式：沒有東西可以取消，什麼都不該發生（更不該離開）。
	if s.Key(ui.KeyCancel) {
		t.Error("探索模式按 Esc 竟然有反應")
	}
	if s.Mode != ui.ModeExplore {
		t.Errorf("探索模式按 Esc 之後模式變成 %v", s.Mode)
	}
	// 選單：關掉選單，回到探索。
	s.Key(ui.KeyCast)
	if s.Mode != ui.ModeMenu {
		t.Fatalf("進不了選單，模式是 %v", s.Mode)
	}
	if !s.Key(ui.KeyCancel) || s.Mode != ui.ModeExplore {
		t.Errorf("選單按 Esc 之後模式是 %v，應該回探索", s.Mode)
	}
	// 連按幾次也只是取消，遊戲照樣在。
	for i := 0; i < 5; i++ {
		s.Key(ui.KeyCancel)
	}
	if s.Mode != ui.ModeExplore {
		t.Errorf("連按 Esc 之後模式是 %v", s.Mode)
	}
	if !s.Key(ui.KeyForward) && !s.Key(ui.KeyRight) {
		t.Error("連按 Esc 之後走不動也轉不了 —— 輸入被吃掉了")
	}
}

// Esc 要收得掉訊息。
//
// 訊息模式只吃確認鍵的話，玩家按 Esc 什麼事都不會發生，而方向鍵在這個模式下
// 同樣沒反應 —— 兩件事加起來就是「畫面卡住了」。實跑抓到的案例：對空的
// 物品格按 Enter 會印「那一格是空的。」，接著八十次方向鍵全部落空。
func TestEscapeDismissesMessages(t *testing.T) {
	s := load(t)
	if !s.Key(ui.KeySave) || s.Mode != ui.ModeMessage {
		t.Fatalf("進不了訊息模式，模式是 %v", s.Mode)
	}
	if !s.Key(ui.KeyCancel) {
		t.Fatal("訊息模式按 Esc 沒有反應")
	}
	if s.Mode != ui.ModeExplore {
		t.Errorf("Esc 之後模式是 %v，訊息沒收掉", s.Mode)
	}
	// 收掉之後走得動 —— 「收掉了」與「還卡著」的差別就在這裡。
	if !s.Key(ui.KeyForward) && !s.Key(ui.KeyRight) {
		t.Error("訊息收掉之後還是走不動")
	}
}

// **但有 Y/N 提問掛著時，Esc 不能被偷當成其中一邊。**
func TestEscapeDoesNotAnswerYesNoPrompts(t *testing.T) {
	s := load(t)
	walkToMiddlegateGate(t, s)
	w := s.World()
	if s.Mode != ui.ModeMessage || w.Pending == nil {
		t.Fatalf("沒有走到原版的 Y/N 提問：模式=%v，續跑=%+v", s.Mode, w.Pending)
	}
	before := s.Mode
	if s.Key(ui.KeyCancel) {
		t.Error("Y/N 提問掛著時 Esc 竟然有反應")
	}
	if s.Mode != before || w.Pending == nil {
		t.Errorf("模式從 %v 變成 %v，續跑=%+v", before, s.Mode, w.Pending)
	}
	// 答得出來才算 —— 擋掉 Esc 不能把提問本身弄壞。
	if !s.Key(ui.KeyNo) || w.Pending != nil {
		t.Errorf("擋掉 Esc 之後連 N 都答不了，續跑=%+v", w.Pending)
	}
}

// 施法的擋人條件**戰鬥內外不一樣，原版自己就不一樣**：
//
//	戰鬥外（角色卡 `C`，root `sub_158B0`）  昏迷／麻痺／沈睡／石化以上
//	戰鬥中（`2COMBAT sub_1929A`）           沈默、法力等級 0、SP 0
//
// 沈默在戰鬥中擋、戰鬥外不擋（社群的 Bug 2）；沈睡與麻痺反過來。
// 這一份把「不一致」本身釘住 —— 下一輪很容易有人覺得那是筆誤而統一掉。
func casterIndex(t *testing.T, s *ui.Session) int {
	t.Helper()
	for i := range s.Game.Party {
		if c := &s.Game.Party[i]; !c.Empty() && game.CanCast(c.Class) && c.SP > 0 {
			return i
		}
	}
	t.Skip("隊伍裡沒有法力還在的施法者")
	return -1
}

// castMenuHas 回報施法者選單裡有沒有第 i 個隊員。
func castMenuHas(s *ui.Session, i int) bool {
	for _, c := range s.Casters() {
		if c == i {
			return true
		}
	}
	return false
}

func TestSleepBlocksCastingOutsideCombatOnly(t *testing.T) {
	s := load(t)
	i := casterIndex(t, s)
	c := &s.Game.Party[i]

	c.CondBits = game.CondBitAsleep
	if !s.Key(ui.KeyCast) {
		t.Fatal("施法選單打不開")
	}
	if castMenuHas(s, i) {
		t.Error("戰鬥外沈睡的人還列在施法者裡")
	}
	s.Key(ui.KeyCancel)

	// 麻痺同理。
	c.CondBits = game.CondBitParalyzed
	s.Key(ui.KeyCast)
	if castMenuHas(s, i) {
		t.Error("戰鬥外麻痺的人還列在施法者裡")
	}
	s.Key(ui.KeyCancel)

	// **沈默在戰鬥外不擋** —— 這是原版的行為，照抄。
	c.CondBits = game.CondBitSilenced
	s.Key(ui.KeyCast)
	if !castMenuHas(s, i) {
		t.Error("戰鬥外沈默的人被擋掉了 —— 原版不擋這一項")
	}
	s.Key(ui.KeyCancel)
}

func TestSilenceBlocksCastingInCombatOnly(t *testing.T) {
	s := load(t)
	i := casterIndex(t, s)
	c := &s.Game.Party[i]
	startFight(t, s)

	c.CondBits = game.CondBitSilenced
	if !s.Key(ui.KeyCast) {
		t.Fatal("戰鬥中施法選單打不開")
	}
	if castMenuHas(s, i) {
		t.Error("戰鬥中沈默的人還列在施法者裡")
	}
	s.Key(ui.KeyCancel)

	// **沈睡在戰鬥中不擋** —— 戰鬥的輪替只看 `狀況 & 0xC0`。
	c.CondBits = game.CondBitAsleep
	s.Key(ui.KeyCast)
	if !castMenuHas(s, i) {
		t.Error("戰鬥中沈睡的人被擋掉了 —— 原版的戰鬥閘門不收這一項")
	}
	s.Key(ui.KeyCancel)

	// 法力用完也不給按（`cmp word ptr [bx+58h], 0`）。
	c.CondBits, c.SP = 0, 0
	s.Key(ui.KeyCast)
	if castMenuHas(s, i) {
		t.Error("戰鬥中沒有法力的人還列在施法者裡")
	}
}
