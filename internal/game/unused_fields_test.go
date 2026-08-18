package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/items"
	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// 三個「原版解出來卻沒有用」的欄位，remake 給了它們用途。
// 這一組測試釘的是**資料的形狀**，不是玩法數值 —— 形狀跑掉了就表示
// 位元取錯了，而玩法數值本來就是我們自己定的，改了不該讓測試紅。

// 怪物記錄 b22 的 bit7（異界生物）。
func TestOtherworldlyBit(t *testing.T) {
	mons, err := monsters.Parse(orig(t, "MONSTERS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, m := range mons {
		if m.Otherworldly != (m.Stats[8]&0x80 != 0) {
			t.Fatalf("%s 的 Otherworldly 與 b22 bit7 不一致", m.Name)
		}
		if m.Otherworldly {
			n++
		}
	}
	// 100/256。不是零、也不是全部 —— 兩者都表示位元取錯了。
	if n != 100 {
		t.Errorf("異界生物有 %d 隻，預期 100 隻", n)
	}
}

// 屬性層 bit 5（吃照明的格）：125 格、6 張圖，而且每一格都有牆。
//
// 數量與分佈是版面對不對的護欄；「每一格都有牆」是當初拿來猜語意的
// 觀察，語意本身現在有反組譯根據（`game.AttrDrainsLight`），這條留著
// 是因為它抓得到「位元或平面取錯」。
func TestDrainCellsAllHaveWalls(t *testing.T) {
	w := newWorld(t)
	cells, withWall, maps := 0, 0, 0
	for mi := range w.Maps {
		m := &w.Maps[mi]
		got := 0
		for c := 0; c < len(m.Attr); c++ {
			if m.Attr[c]&game.AttrDrainsLight == 0 {
				continue
			}
			cells++
			got++
			if m.Attr[c]&0x55 != 0 {
				withWall++
			}
		}
		if got > 0 {
			maps++
		}
	}
	if cells != 125 || maps != 6 {
		t.Errorf("bit 5 有 %d 格分佈在 %d 張圖，預期 125 格、6 張圖", cells, maps)
	}
	if withWall != cells {
		t.Errorf("%d/%d 格有牆 —— 應該是全部", withWall, cells)
	}
}

// 進貨的洗牌要是 0–25 的**排列**：二十六個值不重不漏。
func TestRestockPermutation(t *testing.T) {
	r := game.NewRand(0x1234)
	for round := 0; round < 20; round++ {
		p := game.RestockPermutation(r)
		var seen [26]bool
		for _, v := range p {
			if v < 0 || v > 25 {
				t.Fatalf("第 %d 輪出現界外的值 %d", round, v)
			}
			if seen[v] {
				t.Fatalf("第 %d 輪的值 %d 重複 —— 那就不是排列了", round, v)
			}
			seen[v] = true
		}
	}
}

// 貨架：六件、互不相同、全部是有名字的物品，而且同一天問兩次要一樣。
func TestShopShelf(t *testing.T) {
	w := newWorld(t)
	s := game.NewSession(w, nil, nil, 4321)
	tbl, err := items.Parse(orig(t, "ITEMS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s.UseItems(tbl)
	for group := 0; group < 4; group++ {
		for town := 0; town < 5; town++ {
			got := s.ShopShelf(group, town)
			if len(got) != 6 {
				t.Fatalf("第 %d 類第 %d 城上架 %d 件，預期 6 件", group, town, len(got))
			}
			seen := map[int]bool{}
			for _, id := range got {
				if seen[id] {
					t.Errorf("第 %d 類第 %d 城的貨架有重複的 %d", group, town, id)
				}
				seen[id] = true
				if id <= 0 || id >= len(s.Items) || s.Items[id].Name == "" {
					t.Errorf("第 %d 類第 %d 城上架了查不到名字的 %d", group, town, id)
				}
			}
			// 同一天再問一次要一模一樣 —— 逛出去再進來不該換一批。
			again := s.ShopShelf(group, town)
			for i := range got {
				if got[i] != again[i] {
					t.Fatalf("同一天兩次進店拿到不同的貨架")
				}
			}
		}
	}
}
