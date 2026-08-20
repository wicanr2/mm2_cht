package ui_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

// 底下那兩塊框（`'O' 選項`／`Day=`／`Year=`／`Face=` 那一列，以及訊息大框）
// 是**只有高解析層才畫得到東西**的區域：原版像素層在那裡只有黑底與紅框。
// 所以「Hi 與原版層放大後的結果有差」等價於「文字真的畫上去了」。
//
// 為什麼要有這一條：先前 `cmd/mm2` 在 `Session.Draw()` 之後又 `Flush()`
// 一次，把整層文字洗掉，畫面只剩空框 —— 而當時所有測試都是綠的，
// 因為離屏測試直接讀 `Draw()` 的結果，沒有那一步。這個測試把「文字有沒有
// 進到最終畫布」變成看得見的訊號，而不是靠人去看截圖。
const (
	textZoneY0, textZoneY1 = 133, 187 // 原版座標，橫列上緣到大框下緣
	textZoneX0, textZoneX1 = 5, 314
)

// hiOverlayPixels 回傳指定原版座標矩形內，高解析層與「原版層放大」不同的像素數。
func hiOverlayPixels(scr *render.Screen) int {
	ref := render.New(scr.Orig.Palette)
	copy(ref.Orig.Pix, scr.Orig.Pix)
	ref.Flush()
	n := 0
	for y := textZoneY0 * render.Scale; y < textZoneY1*render.Scale; y++ {
		for x := textZoneX0 * render.Scale; x < textZoneX1*render.Scale; x++ {
			if ref.Hi.RGBAAt(x, y) != scr.Hi.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

func TestFrameKeepsHiResText(t *testing.T) {
	s := load(t)
	scr := s.Draw()
	if got := hiOverlayPixels(scr); got < 500 {
		t.Fatalf("下方兩塊框的高解析文字只有 %d 個像素，等於沒畫出來", got)
	}
}

// 再 Flush 一次就是那個 bug 本身：這裡把它的後果寫成斷言，
// 免得日後有人「順手」在送畫面之前補一次 Flush。
func TestExtraFlushWipesText(t *testing.T) {
	s := load(t)
	scr := s.Draw()
	before := hiOverlayPixels(scr)
	scr.Flush()
	if after := hiOverlayPixels(scr); after >= before {
		t.Fatalf("Flush 之後文字像素從 %d 變成 %d，"+
			"與 render.Screen.Flush 的契約（一律在畫中文之前呼叫）不符", before, after)
	}
}

// 選單開著時，選單文字也走同一層。
func TestMenuTextReachesHiLayer(t *testing.T) {
	s := load(t)
	s.Key(ui.KeyCast)
	if s.Mode != ui.ModeMenu {
		t.Fatal("沒有進選單模式")
	}
	if got := hiOverlayPixels(s.Draw()); got < 500 {
		t.Fatalf("選單開著時下方兩塊框只有 %d 個高解析像素", got)
	}
}
