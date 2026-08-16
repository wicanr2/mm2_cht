package ui_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/ui"
)

// 片頭要載得起來、畫得出東西、按任意鍵就進遊戲。
//
// **不驗「開起來就是片頭」** —— 片頭是前端的呈現決定（`cmd/mm2` 明講
// `ShowIntro()`），Session 的預設狀態仍是一場進行中的遊戲。
func TestIntroShowAndDismiss(t *testing.T) {
	s := load(t)
	if !s.ShowIntro() {
		t.Fatal("ShowIntro 失敗：片頭素材沒載進來")
	}
	if s.Mode != ui.ModeIntro {
		t.Fatalf("模式是 %v，不是片頭", s.Mode)
	}
	if got := s.MusicCue(); got != ui.MusicCueIntro {
		t.Errorf("片頭的音樂角色是 %q，預期 %q", got, ui.MusicCueIntro)
	}

	scr := s.Draw()
	scr.Flush()
	nonZero := 0
	for _, v := range scr.Orig.Pix {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero < 30000 {
		t.Errorf("片頭只有 %d 個非背景像素，看起來沒畫出標題", nonZero)
	}

	s.Key(ui.KeyConfirm)
	if s.Mode == ui.ModeIntro {
		t.Error("按鍵之後還停在片頭")
	}
}

// 動畫要真的在動：走幾格之後畫面必須與第 0 格不同。
//
// 比的是整張畫面而不是某一塊 —— 落點寫在 `view.IntroSpotAt`，
// 這裡要驗的是「動畫有接上」，不是把那張表再抄一次。
func TestIntroAnimates(t *testing.T) {
	s := load(t)
	if !s.ShowIntro() {
		t.Skip("片頭素材沒載進來")
	}
	first := append([]uint8(nil), s.Draw().Orig.Pix...)
	moved := false
	for i := 0; i < 16 && !moved; i++ {
		s.Tick()
		cur := s.Draw().Orig.Pix
		for j := range cur {
			if cur[j] != first[j] {
				moved = true
				break
			}
		}
	}
	if !moved {
		t.Error("走了 16 格畫面都沒變，片頭動畫沒在動")
	}
}
