package view_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/view"
)

// TestIntroPlaylistMatchesOriginal 守住從 `1MENU1` 的 `ds:18D8` 讀出來的
// 47 步清單，以及每個圖號的落點。這兩張表是原版資料，不是呈現決定。
func TestIntroPlaylistMatchesOriginal(t *testing.T) {
	if n := len(view.IntroPlaylist); n != 47 {
		t.Fatalf("播放清單 %d 步，原版是 47 步（`sub_1C1FA` 的 di < 0x2F）", n)
	}
	if view.IntroPlaylist[0] != 14 {
		t.Errorf("第 0 步是第 %d 張，原版先畫底圖（第 14 張）", view.IntroPlaylist[0])
	}
	for i, n := range view.IntroPlaylist {
		if n < 0 || n >= len(view.IntroSpotAt) {
			t.Fatalf("第 %d 步的圖號 %d 超出十五張", i, n)
		}
	}
	// 底圖以外的每一張都要有落點；重複出現的那幾張落點必須一致。
	for _, tc := range []struct {
		pic, x, y int
	}{
		{0, 160, 161}, {1, 160, 161}, {2, 16, 126}, {3, 16, 126},
		{4, 160, 154}, {5, 256, 74}, {6, 256, 74}, {7, 232, 62},
		{8, 240, 113}, {9, 240, 61}, {10, 232, 125}, {11, 232, 125},
		{12, 96, 95}, {13, 96, 95}, {14, 0, 0},
	} {
		got := view.IntroSpotAt[tc.pic]
		if got.X != tc.x || got.Y != tc.y {
			t.Errorf("第 %d 張的落點是 (%d, %d)，原版表是 (%d, %d)",
				tc.pic, got.X, got.Y, tc.x, tc.y)
		}
	}
}

// TestIntroStepPacing 守住換格週期：原版一步 0.601 秒，remake 的 tick 是
// 7.5 Hz，所以 9 個 tick 該走 2 步、整串 47 步約 211 個 tick（28.2 秒）。
func TestIntroStepPacing(t *testing.T) {
	if got := view.IntroStep(0); got != 0 {
		t.Errorf("第 0 個 tick 在第 %d 步，該是第 0 步", got)
	}
	if got := view.IntroStep(9); got != 2 {
		t.Errorf("第 9 個 tick 在第 %d 步，該是第 2 步", got)
	}
	// 走滿一輪要 47 × 4.5 ≈ 211.5 個 tick；第 212 個回到第 0 步。
	if got := view.IntroStep(212); got != 0 {
		t.Errorf("第 212 個 tick 在第 %d 步，清單該已經繞回開頭", got)
	}
	// 負的 tick 不能讓它取到負索引。
	if got := view.IntroStep(-9); got < 0 || got >= len(view.IntroPlaylist) {
		t.Errorf("負的 tick 算出第 %d 步", got)
	}
}
