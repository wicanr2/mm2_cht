package ui_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/ui"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// F6 要能繞完所有載到的素材再回到 DOS。
//
// 這條同時守住「素材包安靜地沒載到」：載入失敗是**正常路徑**
// （玩家不一定有 Amiga 版），所以不會有任何錯誤訊息 —— 只有選項變少。
// 少了一套與「本來就只有一套」在畫面上長得一模一樣。
func TestPlatformCycleReturnsToStart(t *testing.T) {
	s := load(t)
	if s.Assets.Town == nil {
		t.Skip("沒有場景素材")
	}
	first := s.Assets.Town.Platform
	if first != view.PlatformDOS {
		t.Fatalf("第一套應該是 DOS，卻是 %v", first)
	}
	seen := []view.Platform{first}
	for i := 0; i < 8; i++ {
		s.Key(ui.KeyPlatform)
		s.Mode = ui.ModeExplore // 訊息模式會吃掉下一次按鍵
		p := s.Assets.Town.Platform
		if p == first {
			break
		}
		for _, q := range seen {
			if q == p {
				t.Fatalf("平台 %v 出現兩次，沒有繞成一圈：%v", p, seen)
			}
		}
		seen = append(seen, p)
	}
	if s.Assets.Town.Platform != first {
		t.Fatalf("繞了八次還沒回到 %v", first)
	}
	t.Logf("可切換的素材：%v", seen)
}

// 換平台不該把玩家選好的風格重設掉 —— 兩個設定是正交的。
func TestPlatformKeepsStyle(t *testing.T) {
	s := load(t)
	if s.Assets.Town == nil {
		t.Skip("沒有場景素材")
	}
	s.Key(ui.KeyStyle)
	s.Mode = ui.ModeExplore
	if s.Assets.Town.Style != view.StyleModern {
		t.Fatal("F5 沒有切到 Scale3x")
	}
	s.Key(ui.KeyPlatform)
	s.Mode = ui.ModeExplore
	if s.Assets.Town.Fixed() {
		return // 素材包沒有風格可言
	}
	if s.Assets.Town.Style != view.StyleModern {
		t.Fatal("換平台之後風格被重設了")
	}
}
