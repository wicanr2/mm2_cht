package game_test

import (
	"encoding/json"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// findWaterCell 找一個真實 MAP.DAT 的野外水域格，避免用人工地形碼把
// typed data→地形分類→通行 gate 這條鏈切斷。
func findWaterCell(t *testing.T, s *game.Session) (int, int, int) {
	t.Helper()
	for mi := range s.World.Maps {
		if mi >= len(s.Attrs) || s.Attrs[mi].Indoor() {
			continue
		}
		m := &s.World.Maps[mi]
		for c := 0; c < game.MapCells; c++ {
			x, y := c%game.MapW, c/game.MapW
			if m.TerrainClass(x, y) == game.TerrainWaterClass {
				return mi, x, y
			}
		}
	}
	t.Fatal("原版地圖找不到野外水域格")
	return 0, 0, 0
}

func TestWaterGateUsesSceneAndWalkOnWaterFlag(t *testing.T) {
	s := session(t)
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s.UseAttrs(attrs)
	mi, x, y := findWaterCell(t, s)
	s.World.MapIndex = mi
	s.World.X, s.World.Y = x, y
	if s.World.Globals == nil {
		s.World.Globals = map[uint16]byte{}
	}
	s.World.Globals[0x03D9] = 0

	if attrs[mi].Scene() == 0x0A {
		if ok, msg := s.EnterOutdoor(x, y); ok || msg != "不會游泳！" {
			t.Fatalf("場景 0x0A 未施法應阻擋，ok=%v msg=%q", ok, msg)
		}
	} else {
		if ok, msg := s.EnterOutdoor(x, y); !ok || msg != "" {
			t.Fatalf("非 0x0A 場景不應走水域 Can't swim 分支，ok=%v msg=%q", ok, msg)
		}
	}
	s.World.Globals[0x03D9] = 1
	if ok, msg := s.EnterOutdoor(x, y); !ok || msg != "" {
		t.Fatalf("水行術後應可進水域，ok=%v msg=%q", ok, msg)
	}
}

func TestWaterWalkSpellRestCrossMapAndSaveLifecycle(t *testing.T) {
	s := session(t)
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s.UseAttrs(attrs)
	mi, x, y := findWaterCell(t, s)
	s.World.MapIndex = mi
	s.World.X, s.World.Y = x, y
	if s.World.Globals == nil {
		s.World.Globals = map[uint16]byte{}
	}

	who := -1
	for i := range s.Party {
		if s.Party[i].Class == game.Cleric || s.Party[i].Class == game.Sorcerer {
			who = i
			break
		}
	}
	if who < 0 {
		t.Fatal("DEFAULT.DAT 沒有可施法角色")
	}
	// engine index 19 是牧師第 20 條水行術；用既有角色資料學會它，
	// 仍經過 Cast 的資格／代價／效果垂直鏈。
	n := 20
	if s.Party[who].Class == game.Sorcerer {
		t.Skip("DEFAULT.DAT 的巫師系沒有對應水行術測試角色")
	}
	s.Party[who].Learn(n)
	s.Party[who].SL = 9
	s.Party[who].SP, s.Party[who].Gems = 99, 99
	if r := s.Cast(who, n); !r.OK {
		t.Fatalf("水行術施法失敗：%s", r.Reason)
	}
	if s.World.Globals[0x03D9] != 1 {
		t.Fatalf("施法後 ds:03D9=%d，預期 1", s.World.Globals[0x03D9])
	}
	if ok, msg := s.EnterOutdoor(x, y); !ok || msg != "" {
		t.Fatalf("施法後水域仍被擋，ok=%v msg=%q", ok, msg)
	}

	// 休息是已證實的清除邊界。
	s.RestAtInn()
	if s.World.Globals[0x03D9] != 0 {
		t.Fatalf("休息後 ds:03D9=%d，預期清零", s.World.Globals[0x03D9])
	}

	// 存檔格式是 remake 自己的 JSON；原版 save/load 仍未知，這裡只守
	// remake 的狀態往返，不把它宣稱成 DOS parity。
	s.World.Globals[0x03D9] = 1
	before := s.State()
	var after game.State
	b, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &after); err != nil {
		t.Fatal(err)
	}
	s2 := session(t)
	if err := s2.LoadState(after); err != nil {
		t.Fatal(err)
	}
	if s2.World.Globals[0x03D9] != 1 {
		t.Fatalf("remake 存檔重載遺失 ds:03D9=%d", s2.World.Globals[0x03D9])
	}
}

func TestWaterWalkClearsOnCrossMap(t *testing.T) {
	s := session(t)
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s.UseAttrs(attrs)
	mi := -1
	for i := range attrs {
		if !attrs[i].Indoor() {
			mi = i
			break
		}
	}
	if mi < 0 {
		t.Fatal("找不到野外地圖")
	}
	s.World.MapIndex, s.World.X, s.World.Y, s.World.Face = mi, game.MapW-1, 8, game.East
	s.World.Neighbor = make([][4]int, len(s.World.Maps))
	s.World.Neighbor[mi][game.East] = (mi + 1) % len(s.World.Maps)
	s.World.Globals = map[uint16]byte{}
	s.World.Globals[0x03D9] = 1
	if !s.World.Move(1) {
		t.Fatal("野外邊界換圖測試未成功")
	}
	if s.World.Globals[0x03D9] != 0 {
		t.Fatalf("換圖後 ds:03D9=%d，預期清零", s.World.Globals[0x03D9])
	}
}
