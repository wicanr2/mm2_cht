package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 沒有照明就看不見的規則（root `sub_13FFC`）：照明計數為 0，而且
// 這張圖沒有光源或這一格吃照明。
func TestDarkNeedsLightAndADarkPlace(t *testing.T) {
	w := newWorld(t)
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range w.Maps {
		if i < len(attrs) {
			w.Maps[i].Dark = attrs[i].Dark()
		}
	}
	w.Globals = map[uint16]byte{}

	// 十三張沒有光源的圖全部落在 17–32 那一段（室內／地城）。
	var dark []int
	for i, a := range attrs {
		if a.Dark() {
			dark = append(dark, i)
		}
	}
	if len(dark) != 13 {
		t.Errorf("沒有光源的圖有 %d 張，預期 13 張：%v", len(dark), dark)
	}
	for _, i := range dark {
		if i < 17 || i > 32 {
			t.Errorf("地圖 %d 有「沒有光源」的位元，但不在 17–32 那一段", i)
		}
	}

	w.MapIndex, w.X, w.Y = dark[0], 7, 7
	if !w.Dark() {
		t.Errorf("地圖 %d 照明 0 應該是暗的", dark[0])
	}
	w.Globals[game.LightAddr] = 1
	if w.Dark() {
		t.Errorf("地圖 %d 照明 1 就不該是暗的", dark[0])
	}

	// 有光源的圖照明 0 也看得見。
	w.Globals[game.LightAddr] = 0
	w.MapIndex = 0
	if w.Dark() {
		t.Error("城鎮不該因為沒有照明就看不見")
	}
}

// 吃照明的格（屬性層 bit 5）：走上去燒一點，燒完就看不見。
func TestDrainCellsBurnLight(t *testing.T) {
	w := newWorld(t)
	w.Globals = map[uint16]byte{}
	// 找一張有這種格子、而且走得過去的圖。
	var mi, cx, cy int = -1, 0, 0
	for i := range w.Maps {
		m := &w.Maps[i]
		for y := 0; y < game.MapH && mi < 0; y++ {
			for x := 0; x < game.MapW; x++ {
				if m.DrainsLight(x, y) {
					mi, cx, cy = i, x, y
					break
				}
			}
		}
		if mi >= 0 {
			break
		}
	}
	if mi < 0 {
		t.Skip("沒有吃照明的格，跳過")
	}
	w.MapIndex, w.X, w.Y = mi, cx, cy
	if !w.Dark() {
		t.Errorf("圖 %d (%d,%d) 吃照明且照明 0，應該是暗的", mi, cx, cy)
	}
	w.Globals[game.LightAddr] = 3
	if w.Dark() {
		t.Error("照明還有 3 就不該是暗的")
	}
}
