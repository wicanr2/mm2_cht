package ui_test

import (
	"testing"

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
