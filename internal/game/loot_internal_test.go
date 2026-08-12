package game

import "testing"

// `sub_19A3C` 的每列第一格是基礎 ID，後三格才是各 band 的擲骰範圍。
// 這條守住 slice 偏移，避免把基礎 ID 誤當成範圍上限。
func TestRollLootBandItemUsesRangeAfterBase(t *testing.T) {
	band := []int{100, 2, 4, 6}
	for column, span := range band[1:] {
		for seed := uint16(1); seed < 64; seed++ {
			got := rollLootBandItem(NewRand(seed), band, column)
			if got < 101 || got > 100+span {
				t.Fatalf("column %d span %d 產生 ID %d；應為 101..%d",
					column, span, got, 100+span)
			}
		}
	}
}
