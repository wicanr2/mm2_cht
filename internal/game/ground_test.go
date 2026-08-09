package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// ATTRIB +22/+24 是「地面出口」：野外圖沒有（+22 = 0、+24 指自己），
// 室內圖一定有，而且指向一張野外圖。這一條抓得到欄位認錯位置。
func TestGroundExit(t *testing.T) {
	as, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	// 五張例外：野外 41–44 的 `+24` 是 0 而不是自己，室內 59 沒有出口。
	// 這五張的整筆屬性與相鄰的圖比對過，看得出是沒用到的空欄位。
	except := map[int]bool{41: true, 42: true, 43: true, 44: true, 59: true}
	indoor, outdoor := 0, 0
	for i := range as {
		a := as[i]
		_, _, ok := a.GroundPos()
		if a.Raw[18] != 0 { // 室內（撞門難度非零）
			indoor++
			if !ok && !except[i] {
				t.Errorf("室內圖 %d 沒有地面出口", i)
			}
			if g := a.GroundMap(); g == i {
				t.Errorf("室內圖 %d 的地面出口指向自己", i)
			}
			continue
		}
		outdoor++
		if ok {
			t.Errorf("野外圖 %d 竟然有地面出口 %#02x", i, a.Raw[22])
		}
		if g := a.GroundMap(); g != i && !except[i] {
			t.Errorf("野外圖 %d 的地面地圖是 %d，該是自己", i, g)
		}
	}
	if indoor == 0 || outdoor == 0 {
		t.Fatalf("室內 %d 張、野外 %d 張，分不出來", indoor, outdoor)
	}
	t.Logf("室內 %d 張、野外 %d 張", indoor, outdoor)
}
