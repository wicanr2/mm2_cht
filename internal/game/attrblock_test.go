package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 屬性區是**六格**（`+16`–`+21`），不是七格。
//
// 角色創建的擲骰程序（`sub_189EE`）對七格清 0 並各填 3d7（3–21），
// 一度讓人以為記錄的屬性區也是七格。名冊四十筆一比就分開了：
// `+16`–`+21` 每格都有三成以上落在 3–21、平均 15–17，
// 而 `+22` 只有 1/40 落在區間內、平均 6.0。
//
// 所以那個七格的區塊是創建時的暫存，第七項（運氣）另外存到 `+115`。
func TestAttrBlockWidth(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for off := 16; off <= 24; off++ {
		var min, max, sum, n, inRange int
		min = 999
		for i := range cs {
			if cs[i].Empty() {
				continue
			}
			v := int(cs[i].FieldByte(off))
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
			if v >= 3 && v <= 21 {
				inRange++
			}
			sum += v
			n++
		}
		if n == 0 {
			continue
		}
		t.Logf("+%d：範圍 %d–%d 平均 %.1f，落在 3–21 的 %d/%d",
			off, min, max, float64(sum)/float64(n), inRange, n)
		switch {
		case off <= 21 && inRange*10 < n*7:
			t.Errorf("+%d 只有 %d/%d 落在 3–21，屬性該有七成以上", off, inRange, n)
		case off == 22 && inRange*10 > n*3:
			t.Errorf("+22 有 %d/%d 落在 3–21，它不該是屬性", inRange, n)
		}
	}
}
