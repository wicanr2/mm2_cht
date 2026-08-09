package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 同一顆種子必然給出同一串數列 —— 這是能重播原版行為的前提。
func TestRandIsDeterministic(t *testing.T) {
	a, b := game.NewRand(0x1234), game.NewRand(0x1234)
	for i := 0; i < 200; i++ {
		if x, y := a.Range(1, 100), b.Range(1, 100); x != y {
			t.Fatalf("第 %d 次分歧：%d vs %d", i, x, y)
		}
	}
	if a.Seedof() != b.Seedof() {
		t.Errorf("種子分歧：%#x vs %#x", a.Seedof(), b.Seedof())
	}
}

// 邊界照原版處理：兩端都含、lo 大於 hi 要對調、hi 為 0 當成 1。
func TestRandRangeBounds(t *testing.T) {
	r := game.NewRand(1)
	seen := map[int]bool{}
	for i := 0; i < 2000; i++ {
		v := r.Range(3, 7)
		if v < 3 || v > 7 {
			t.Fatalf("Range(3,7) 給出 %d", v)
		}
		seen[v] = true
	}
	for v := 3; v <= 7; v++ {
		if !seen[v] {
			t.Errorf("Range(3,7) 兩千次都沒出現過 %d", v)
		}
	}
	// lo > hi 要對調
	for i := 0; i < 100; i++ {
		if v := r.Range(9, 4); v < 4 || v > 9 {
			t.Fatalf("Range(9,4) 給出 %d", v)
		}
	}
	// hi = 0 當成 1，所以範圍是 [0,1]
	for i := 0; i < 100; i++ {
		if v := r.Range(0, 0); v < 0 || v > 1 {
			t.Fatalf("Range(0,0) 給出 %d", v)
		}
	}
}

// 種子每次呼叫都要推進，不能卡住。
func TestRandAdvancesSeed(t *testing.T) {
	r := game.NewRand(0)
	prev := r.Seedof()
	stuck := 0
	for i := 0; i < 500; i++ {
		r.Range(1, 6)
		if r.Seedof() == prev {
			stuck++
		}
		prev = r.Seedof()
	}
	if stuck > 0 {
		t.Errorf("種子有 %d 次沒有推進", stuck)
	}
}

// 分佈要夠平：一顆六面骰擲六千次，每面都該落在合理區間。
// 這條擋的是「照抄組語時 ror 的位數或遮罩寫錯」——
// 那類錯誤會讓某些值永遠出不來，或分佈嚴重偏斜。
func TestRandDistribution(t *testing.T) {
	r := game.NewRand(0xBEEF)
	const n = 6000
	count := map[int]int{}
	for i := 0; i < n; i++ {
		count[r.Range(1, 6)]++
	}
	for v := 1; v <= 6; v++ {
		c := count[v]
		if c < n/6*7/10 || c > n/6*13/10 {
			t.Errorf("值 %d 出現 %d 次，離期望值 %d 太遠", v, c, n/6)
		}
	}
}
