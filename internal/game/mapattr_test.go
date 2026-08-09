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

// 撞門的難度門檻：值全是十的倍數、0–100，野外那二十張是 0（沒有門）。
func TestBashDifficulty(t *testing.T) {
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	for _, a := range attrs {
		d := a.BashDifficulty()
		if d < 0 || d > 100 || d%10 != 0 {
			t.Errorf("地圖 %d 的門難度 %d 不是 0–100 的十的倍數", a.Index, d)
		}
		if d == 0 {
			zero++
		}
	}
	if zero < 15 {
		t.Errorf("只有 %d 張地圖的門難度是 0，野外那些應該都是", zero)
	}
	// 中門是起始城鎮，門應該最好撞。
	if attrs[0].BashDifficulty() > attrs[3].BashDifficulty() {
		t.Errorf("中門的門難度 %d 高於地圖 3 的 %d", attrs[0].BashDifficulty(), attrs[3].BashDifficulty())
	}
}

// 開鎖那一擲的門檻（+19）形狀應該與撞門難度（+18）一致：
// 十的倍數、野外為 0。
func TestLockDifficulty(t *testing.T) {
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	for _, a := range attrs {
		d := a.LockDifficulty()
		if d < 0 || d > 100 || d%10 != 0 {
			t.Errorf("地圖 %d 的鎖難度 %d 不是 0–100 的十的倍數", a.Index, d)
		}
		if d == 0 {
			zero++
		}
		// 沒有門的地圖（撞門難度 0）鎖難度也該是 0。
		if a.BashDifficulty() == 0 && d != 0 {
			t.Errorf("地圖 %d 沒有門（撞門難度 0）卻有鎖難度 %d", a.Index, d)
		}
	}
	if zero < 15 {
		t.Errorf("只有 %d 張地圖的鎖難度是 0", zero)
	}
}

// 野外地形的分類分佈要合理：多數可通行、水域是次多、山與森林各佔少數。
// 這一條同時釘住「地形碼取自 Attr 層而不是 Terrain 層」——
// 取錯層的話水域會變成 52.8%，一片汪洋。
func TestOutdoorTerrainDistribution(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip(err)
	}
	w := newWorld(t)
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	count := map[int]int{}
	total := 0
	for i := range w.Maps {
		if i >= len(attrs) || attrs[i].Indoor() {
			continue
		}
		for y := 0; y < game.MapH; y++ {
			for x := 0; x < game.MapW; x++ {
				count[w.Maps[i].TerrainClass(x, y)]++
				total++
			}
		}
	}
	if total == 0 {
		t.Fatal("一張野外圖都沒有")
	}
	open := float64(count[0]) / float64(total)
	water := float64(count[4]) / float64(total)
	if open < 0.5 {
		t.Errorf("可通行只佔 %.1f%%，地形碼可能取錯層", open*100)
	}
	if water > 0.35 {
		t.Errorf("水域佔 %.1f%%，地形碼可能取錯層（取 Terrain 層會得到 52.8%%）", water*100)
	}
	if count[1] == 0 || count[3] == 0 {
		t.Errorf("山區 %d 格、森林 %d 格，其中一種是 0", count[1], count[3])
	}
}

// 野外的通行檢查要真的擋住隊伍，而且要看得出技能有沒有用。
//
// 隨便找一張野外圖上的山區格與森林格，站在它旁邊往那邊走：
// 沒技能的隊伍走不過去，把兩個人的第二技能改成對應技能就過得去。
func TestOutdoorSkillGatesMovement(t *testing.T) {
	w := newWorld(t)
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := cs[:6]

	for _, tc := range []struct {
		class int
		skill int
		name  string
	}{
		{game.TerrainMountainClass, game.SkillMountaineer, "山區"},
		{game.TerrainForestClass, game.SkillPathfinder, "森林"},
	} {
		mi, x, y, from := findTerrain(t, w, attrs, tc.class)
		s := game.NewSession(w, party, nil, 1)
		s.UseAttrs(attrs)
		s.EncounterRate = 0
		w.MapIndex, w.X, w.Y, w.Face = mi, from[0], from[1], game.Facing(from[2])

		// 先把全隊的第二技能清空，確保擋得住。
		for i := range s.Party {
			s.Party[i].Skills = [2]int{0, 0}
		}
		if moved, _ := s.Step(1); moved {
			t.Fatalf("%s：沒技能卻走進 (%d,%d)", tc.name, x, y)
		}
		if s.CountSkill(tc.skill) != 0 {
			t.Fatalf("%s：技能應該清光了", tc.name)
		}

		// 只給一個人 → 仍然不夠（原版要兩人）。
		s.Party[0].Skills[0] = tc.skill
		w.X, w.Y = from[0], from[1]
		if moved, _ := s.Step(1); moved {
			t.Fatalf("%s：只有一人具備技能就走過去了", tc.name)
		}

		// 兩個人 → 過得去。
		s.Party[1].Skills[0] = tc.skill
		w.X, w.Y = from[0], from[1]
		if moved, _ := s.Step(1); !moved {
			t.Fatalf("%s：兩人具備技能卻走不過去（%v）", tc.name, s.Log)
		}
		if w.X != x || w.Y != y {
			t.Fatalf("%s：走完位置是 (%d,%d)，預期 (%d,%d)", tc.name, w.X, w.Y, x, y)
		}
	}
}

// findTerrain 找一張野外圖上某個地形類別的格子，並回傳一個可以走過去的
// 相鄰格與朝向。相鄰格必須是可通行類別，否則測的就不是我們要測的那一格。
func findTerrain(t *testing.T, w *game.World, attrs []game.MapAttr, class int) (mi, x, y int, from [3]int) {
	t.Helper()
	for i := range w.Maps {
		if i < len(attrs) && attrs[i].Indoor() {
			continue
		}
		m := &w.Maps[i]
		for cy := 1; cy < game.MapH-1; cy++ {
			for cx := 1; cx < game.MapW-1; cx++ {
				if m.TerrainClass(cx, cy) != class {
					continue
				}
				// 從西邊往東走進去。
				if m.TerrainClass(cx-1, cy) == game.TerrainOpenClass {
					return i, cx, cy, [3]int{cx - 1, cy, int(game.East)}
				}
			}
		}
	}
	t.Fatalf("六十張圖裡找不到類別 %d 的格子", class)
	return
}
