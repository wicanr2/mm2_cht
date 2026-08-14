package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 世界網格必須是二十張野外圖不重不漏，而且每一格的東鄰與南鄰
// 都與 `ATTRIB.DAT` 自己說的一致 —— 排錯一格這條就會叫。
func TestWorldGridMatchesNeighbors(t *testing.T) {
	attrs := loadAttrs(t)
	grid := game.WorldGrid(attrs)
	if len(grid) != game.WorldRows {
		t.Fatalf("網格 %d 列，預期 %d", len(grid), game.WorldRows)
	}
	seen := map[int]bool{}
	for r := range grid {
		if len(grid[r]) != game.WorldCols {
			t.Fatalf("第 %d 列 %d 行，預期 %d", r, len(grid[r]), game.WorldCols)
		}
		for c, m := range grid[r] {
			if seen[m] {
				t.Errorf("地圖 %d 出現兩次", m)
			}
			seen[m] = true
			east := grid[r][(c+1)%game.WorldCols]
			if got := attrs[m].East(); got != east {
				t.Errorf("(%d,%d) 圖 %d 的東鄰是 %d，網格排的是 %d", r, c, m, got, east)
			}
			south := grid[(r+1)%game.WorldRows][c]
			if got := attrs[m].South(); got != south {
				t.Errorf("(%d,%d) 圖 %d 的南鄰是 %d，網格排的是 %d", r, c, m, got, south)
			}
		}
	}
	if len(seen) != game.WorldRows*game.WorldCols {
		t.Errorf("網格上有 %d 張圖，預期 %d", len(seen), game.WorldRows*game.WorldCols)
	}
}

// C2 是米德格特那一格：外圍地圖 11 的 (7,3) 事件會換到地圖 0。
// 這是整張網格唯一的錨點，錯了其餘十九格就全錯。
func TestWorldGridMiddlegateAnchor(t *testing.T) {
	attrs := loadAttrs(t)
	if got := game.RegionOf(attrs, 11); got != "C2" {
		t.Errorf("地圖 11 的區域是 %q，預期 C2", got)
	}
	// 說明書那一頁的另外幾條地形線索：A1／B1 是凍原、D2／E2 是沙漠、
	// D4 是沼澤。貼圖組碼 9 沙漠、10 海洋、11 沼澤、12 凍原。
	for _, tc := range []struct {
		region string
		want   int
	}{{"A1", 12}, {"B1", 12}, {"D2", 9}, {"E2", 9}, {"D4", 11}, {"C2", 10}} {
		m := regionMap(t, attrs, tc.region)
		if got := game.WorldTileset(attrs, m); got != tc.want {
			t.Errorf("%s（地圖 %d）貼圖組 %d，預期 %d", tc.region, m, got, tc.want)
		}
	}
}

// 城鎮、地城與四個元素領域四面自指，不在世界網格上。
func TestWorldGridExcludesSelfContained(t *testing.T) {
	attrs := loadAttrs(t)
	for _, m := range []int{0, 1, 2, 3, 4, 41, 42, 43, 44} {
		if got := game.RegionOf(attrs, m); got != "" {
			t.Errorf("地圖 %d 不該在網格上，卻排到 %s", m, got)
		}
	}
}

func loadAttrs(t *testing.T) []game.MapAttr {
	t.Helper()
	as, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func regionMap(t *testing.T, attrs []game.MapAttr, region string) int {
	t.Helper()
	grid := game.WorldGrid(attrs)
	for r := range grid {
		for _, m := range grid[r] {
			if game.RegionOf(attrs, m) == region {
				return m
			}
		}
	}
	t.Fatalf("網格上找不到區域 %s", region)
	return -1
}
