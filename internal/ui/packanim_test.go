package ui

import "testing"

// TestPackFrameAtFollowsPlaylist 守住「照原版的動畫表播」這件事。
//
// 清單取自 Amiga `01.anm`（蜘蛛）：序列 5 是 9、10、11、10、9、0 各停 5 個
// tick，整串 30 個 tick 循環一次。等速輪播會在第 5 個 tick 換第二格，
// 這個測試就是要讓那種退化被抓到。
func TestPackFrameAtFollowsPlaylist(t *testing.T) {
	play := [][2]int{{9, 5}, {10, 5}, {11, 5}, {10, 5}, {9, 5}, {0, 5}}
	want := map[int]int{0: 9, 4: 9, 5: 10, 9: 10, 10: 11, 20: 9, 25: 0, 29: 0}
	for tick, frame := range want {
		if got := packFrameAt(play, tick); got != frame {
			t.Fatalf("tick %d：拿到影格 %d，該是 %d", tick, got, frame)
		}
	}
	// 循環：第 30 個 tick 回到開頭。
	for tick := 0; tick < 30; tick++ {
		if packFrameAt(play, tick) != packFrameAt(play, tick+30) {
			t.Fatalf("tick %d 與 %d 不同，清單沒有循環", tick, tick+30)
		}
	}
}

// TestPackFrameAtSurvivesBadPlaylist 是壞掉的 set.json 不能讓遊戲卡死。
func TestPackFrameAtSurvivesBadPlaylist(t *testing.T) {
	if got := packFrameAt(nil, 7); got != 0 {
		t.Fatalf("空清單該回 0，拿到 %d", got)
	}
	if got := packFrameAt([][2]int{{3, 0}, {4, -2}}, 1); got != 4 {
		t.Fatalf("停留次數 0 與負數都該當成 1，拿到 %d", got)
	}
}
