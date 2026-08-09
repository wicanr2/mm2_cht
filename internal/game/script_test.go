package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 把全部地圖的每一個事件格都踩一遍。
//
// 腳本直譯器只認得三個 opcode，其餘靠 opLen 跳過 —— 長度表一旦有誤，
// 就會把參數當成 opcode，或是把索引算到字串陣列外面去。這條掃過
// 兩千多格，任何越界都會在這裡炸出來。
func TestTriggerEveryEventCell(t *testing.T) {
	mb := orig(t, "MAP.DAT")
	for _, f := range []string{"EVENTSI.DAT", "EVENTSO.DAT"} {
		w, err := game.NewWorld(mb, orig(t, f))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		cells, withMsg := 0, 0
		for mi := 0; mi < len(w.Maps); mi++ {
			w.MapIndex = mi
			if w.EventSegment() == nil {
				continue
			}
			for y := 0; y < game.MapH; y++ {
				for x := 0; x < game.MapW; x++ {
					if w.EventAt(x, y) == nil {
						continue
					}
					cells++
					w.X, w.Y = x, y
					w.Trigger()
					if w.Message != "" {
						withMsg++
					}
				}
			}
		}
		if cells == 0 {
			t.Errorf("%s 一個事件格都沒有", f)
			continue
		}
		t.Logf("%s：事件格 %d，有訊息 %d（%.1f%%）",
			f, cells, withMsg, float64(withMsg)*100/float64(cells))
	}
}

// 長度表要能把每一段腳本走到剛好結束。
//
// 這條是長度表的驗收面：從 handler 靜態數出來的版本只有 83.7% 走得完，
// 補上迴圈與條件分支之後是 100%。掉下來就表示表被改壞了。
func TestOpcodeLengthsConsumeWholeScript(t *testing.T) {
	for _, f := range []string{"EVENTSI.DAT", "EVENTSO.DAT"} {
		w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, f))
		if err != nil {
			t.Fatal(err)
		}
		total, clean := 0, 0
		for i := range w.Events {
			for _, sc := range w.Events[i].Scripts {
				if len(sc) == 0 {
					continue
				}
				total++
				p := 0
				for p < len(sc) {
					n := game.OpLen(sc[p])
					if n < 1 || p+n > len(sc) {
						break
					}
					p += n
				}
				if p == len(sc) {
					clean++
				}
			}
		}
		if total == 0 {
			t.Fatalf("%s 沒有腳本", f)
		}
		if r := float64(clean) / float64(total); r < 1.0 {
			t.Errorf("%s：%d/%d（%.1f%%）的腳本走到剛好結束，預期 100%%",
				f, clean, total, r*100)
		}
	}
}

// 0x30 的結果要進條件暫存器：答對 1、答錯或沒答 0。
func TestMatchText(t *testing.T) {
	w := newWorld(t)
	answer := []byte("CARTOGRAP")
	for _, tc := range []struct {
		name string
		hook func([]byte) bool
		want byte
	}{
		{"沒有回答", nil, 0},
		{"答錯", func([]byte) bool { return false }, 0},
		{"答對", func(e []byte) bool { return string(e[:9]) == string(answer) }, 1},
	} {
		w.TextAnswer = tc.hook
		w.Result = 0xFF
		script := append([]byte{0x30}, answer...)
		script = append(script, 0x00, 0x00) // 湊滿十個位元組的答案欄再接結束
		w.RunScriptForTest(script)
		if w.Result != tc.want {
			t.Errorf("%s：ds:042F 是 %d，該是 %d", tc.name, w.Result, tc.want)
		}
	}
}

// 0x24／0x25 都要先把條件暫存器清成 0，再由述詞決定設不設 1。
func TestPartyTestOpcodes(t *testing.T) {
	w := newWorld(t)
	for _, op := range []byte{0x24, 0x25} {
		w.PartyTest = nil
		w.Result = 0xFF
		w.RunScriptForTest([]byte{op, 0x34, 0x12, 0x00})
		if w.Result != 0 {
			t.Errorf("%#02x 沒有述詞時 ds:042F 是 %d，該清成 0", op, w.Result)
		}

		var got uint16
		w.PartyTest = func(_ byte, arg uint16) bool { got = arg; return true }
		w.Result = 0
		w.RunScriptForTest([]byte{op, 0x34, 0x12, 0x00})
		if got != 0x1234 {
			t.Errorf("%#02x 的運算元讀成 %#04x，該是 0x1234（低位在前）", op, got)
		}
		if w.Result != 1 {
			t.Errorf("%#02x 述詞成立時 ds:042F 是 %d", op, w.Result)
		}
	}
}
