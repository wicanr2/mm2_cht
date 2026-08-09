package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 每一筆的 +0 就是自己的編號 —— 這條同時釘住 stride 64 與筆數 60。
// 換成別的 stride，那個 0…59 的遞增序列立刻散掉。
func TestMapAttrIndexIsSelfDescribing(t *testing.T) {
	as, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != game.MapAttrCount {
		t.Fatalf("解出 %d 筆，預期 %d", len(as), game.MapAttrCount)
	}
	for i, a := range as {
		if got := int(a.Raw[0]); got != i {
			t.Errorf("第 %d 筆的 +0 是 %d", i, got)
		}
	}
}

// 鄰接是雙向的：往東走到的那張圖，往西要走得回來。
// 兩軸都必須是六十張全中 —— 交叉配對只有 40/60，差距很大。
func TestMapNeighborsAreMutual(t *testing.T) {
	as, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	ew, ns := 0, 0
	for i := range as {
		if e := as[i].East(); e < len(as) && as[e].West() == i {
			ew++
		}
		a1, _ := as[i].Axis1()
		if a1 < len(as) {
			if _, b := as[a1].Axis1(); b == i {
				ns++
			}
		}
	}
	if ew != len(as) {
		t.Errorf("東西向互指 %d/%d", ew, len(as))
	}
	if ns != len(as) {
		t.Errorf("南北向互指 %d/%d", ns, len(as))
	}
}

// 五座主要城鎮四面都指向自己，走到邊界不會接到別張圖。
func TestTownsAreSelfContained(t *testing.T) {
	as, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if !as[i].SelfContained() {
			t.Errorf("地圖 %d 是城鎮，卻連到別張圖", i)
		}
	}
	// 野外不該是自封的，否則整張世界地圖走不通
	open := 0
	for i := 5; i < len(as); i++ {
		if !as[i].SelfContained() {
			open++
		}
	}
	if open == 0 {
		t.Error("第 5 張之後沒有任何一張連到別的地圖")
	}
	t.Logf("城鎮 5 張自封，其餘 %d 張裡有 %d 張與別圖相連", len(as)-5, open)
}
