package ui_test

import (
	"crypto/sha256"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// MSX 的火炬影格是 remake 產生的（原版每個位置只有一張），所以要有測試
// 守著「相位真的換了圖」—— 沒換的話畫面照樣正常，只是火炬不會動，
// 而那看起來像「原版本來就這樣」。
//
// 判準是「**地圖上存在**會動的位置」不是「這一格會動」：多數格子看不到
// 火炬，挑錯格子會得到「火炬壞了」這個錯誤結論（踩過）。
func TestMSXTorchPhasesDiffer(t *testing.T) {
	s := load(t)
	for i := 0; i < 4 && (s.Assets.Town == nil || s.Assets.Town.Platform != view.PlatformMSX); i++ {
		s.Key(ui.KeyPlatform)
		s.Mode = ui.ModeExplore
	}
	if s.Assets.Town == nil || s.Assets.Town.Platform != view.PlatformMSX {
		t.Skip("沒有 MSX 素材")
	}
	moving := 0
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			for f := 0; f < 4; f++ {
				s.Game.World.X, s.Game.World.Y, s.Game.World.Face = x, y, game.Facing(f)
				seen := map[[32]byte]bool{}
				for p := 0; p < 3; p++ {
					scr := view.NewScreen()
					view.DrawFirstPersonAt(scr, s.Game.World, s.Assets.Town, p)
					scr.Flush()
					seen[sha256.Sum256(scr.Hi.Pix)] = true
				}
				if len(seen) > 1 {
					moving++
				}
			}
		}
	}
	if moving == 0 {
		t.Fatal("整張地圖沒有一個視角的火炬會動")
	}
	t.Logf("1024 個視角裡有 %d 個看得到火炬在動", moving)
}
