package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 走一步過多少時間（root `0x150CE`）：256 步一天、180 天一年。
func TestStepTimeRollsDaysAndYears(t *testing.T) {
	w := newWorld(t)
	w.Globals = map[uint16]byte{}
	w.Party = []game.Character{{Age: 18}}

	if w.Today() != 0 || w.Year() != 0 {
		t.Fatalf("空的全域應該是 0/0，得到 %d/%d", w.Today(), w.Year())
	}
	// 255 步還在第一天。
	for i := 0; i < 255; i++ {
		w.StepTime(1)
	}
	if w.Clock() != 255 || w.Today() != 0 {
		t.Errorf("255 步：時鐘 %d 日 %d，預期 255／0", w.Clock(), w.Today())
	}
	// 第 256 步跨日，時鐘歸 0。
	w.StepTime(1)
	if w.Clock() != 0 || w.Today() != 1 {
		t.Errorf("256 步：時鐘 %d 日 %d，預期 0／1", w.Clock(), w.Today())
	}
	if w.Party[0].AgeDays != 1 {
		t.Errorf("跨一天，年齡天數應該是 1，得到 %d", w.Party[0].AgeDays)
	}

	// 再走 180 天：日期滿 180 之後歸 1，年份加一。
	for i := 0; i < 256*180; i++ {
		w.StepTime(1)
	}
	if w.Today() != 1 {
		t.Errorf("滿一年後日期應該歸 1，得到 %d", w.Today())
	}
	if w.Year() != 1 {
		t.Errorf("年份應該加一，得到 %d", w.Year())
	}
	if w.Party[0].Age != 19 || w.Party[0].AgeDays != 1 {
		t.Errorf("角色應該長一歲，得到 %d 歲第 %d 天", w.Party[0].Age, w.Party[0].AgeDays)
	}
}

// 夜晚只是時鐘過半，**不是原版的東西** —— 這條守的是門檻沒被改掉。
func TestNightIsTheSecondHalfOfTheDay(t *testing.T) {
	w := newWorld(t)
	w.Globals = map[uint16]byte{}
	if game.NightFrom != 128 {
		t.Fatalf("入夜的起點是 %d，預期 128（一天 256 步的一半）", game.NightFrom)
	}
	for i := 0; i < game.NightFrom-1; i++ {
		w.StepTime(1)
	}
	if w.Night() {
		t.Errorf("時鐘 %d 還不算夜晚", w.Clock())
	}
	w.StepTime(1)
	if !w.Night() {
		t.Errorf("時鐘 %d 應該算夜晚了", w.Clock())
	}
}
