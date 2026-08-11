package game_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 解開再寫回，必須與原檔一個位元組不差。
//
// 這條守著兩件事：已解欄位的寫回位置正確，以及**未解欄位沒被洗掉**。
// remake 只解了 15/21 個欄位，如果寫回時把沒解的清成 0，
// 存檔就會毀掉原版的資料。
func TestRoundTripIsByteExact(t *testing.T) {
	for _, f := range []string{"DEFAULT.DAT", "ROSTER.DAT"} {
		blob := orig(t, f)
		cs, err := game.ParseCharacters(blob)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		got, err := game.EncodeRoster(cs, blob)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if !bytes.Equal(got, blob) {
			n := 0
			var first int = -1
			for i := range blob {
				if got[i] != blob[i] {
					n++
					if first < 0 {
						first = i
					}
				}
			}
			t.Errorf("%s：%d 個位元組不同，第一個在 +%d（記錄 %d 的 +%d）：%#x → %#x",
				f, n, first, first/game.RecordSize, first%game.RecordSize,
				blob[first], got[first])
			continue
		}
		t.Logf("%s：%d bytes 完全一致", f, len(blob))
	}
}

// 改過的欄位要真的寫進去。
func TestEncodeAppliesChanges(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	c := cs[0]
	c.HP = 7
	c.Level = 12
	c.Food = 3
	c.Base[game.Might] = 19
	back, err := game.ParseCharacters(c.Encode())
	if err != nil {
		t.Fatal(err)
	}
	g := back[0]
	if g.HP != 7 || g.Level != 12 || g.Food != 3 || g.Base[game.Might] != 19 {
		t.Errorf("寫回後是 HP=%d 等級=%d 食物=%d 力量=%d",
			g.HP, g.Level, g.Food, g.Base[game.Might])
	}
	if g.Name != c.Name || g.Class != c.Class {
		t.Errorf("名稱或職業跑掉了：%q %v", g.Name, g.Class)
	}
}

// 走一段路、打一場、存檔、讀回來 —— 狀態要對得上。
func TestSessionRoundTrip(t *testing.T) {
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	w.MapIndex, w.X, w.Y, w.Face = 0, 7, 8, game.South
	blob := orig(t, "DEFAULT.DAT")
	party, err := game.ParseCharacters(blob)
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, party, nil, 4321)
	for i := 0; i < 2; i++ {
		s.Step(1)
	}
	// 讓第一個人受傷，看存檔有沒有帶上
	s.Party[0].TakeDamage(5)
	hp := s.Party[0].HP

	out, err := game.EncodeRoster(s.Party, blob)
	if err != nil {
		t.Fatal(err)
	}
	back, err := game.ParseCharacters(out)
	if err != nil {
		t.Fatal(err)
	}
	if back[0].HP != hp {
		t.Errorf("讀回來的 HP 是 %d，存的時候是 %d", back[0].HP, hp)
	}
	// 沒動過的角色要與原檔一致
	origParty, _ := game.ParseCharacters(blob)
	for i := 1; i < len(back); i++ {
		if back[i].HP != origParty[i].HP || back[i].Name != origParty[i].Name {
			t.Errorf("第 %d 個角色被改動了", i)
		}
	}
}

// 遊玩狀態要能原樣往返：位置、朝向、種子、全域旗標。
//
// 全域旗標是全遊戲的劇情進度（`ds:03F6` 起 24 個位元組）與世紀
// （`ds:03CA`）。漏存等於進度沒存。
func TestSessionStateRoundTrip(t *testing.T) {
	w := newWorld(t)
	s := game.NewSession(w, nil, nil, 4321)
	w.MapIndex, w.X, w.Y, w.Face = 7, 3, 11, game.West
	s.Rand.Range(1, 100) // 讓種子走幾步，確認存的是當下的值
	for sel, v := range map[int]byte{0x00: 1, 0x05: 0x2A, 0x17: 0xFF, 0x84: 9} {
		w.SetGlobal(sel, v)
	}
	before := s.State()

	b, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	var after game.State
	if err := json.Unmarshal(b, &after); err != nil {
		t.Fatal(err)
	}

	w2 := newWorld(t)
	s2 := game.NewSession(w2, nil, nil, 1)
	if err := s2.LoadState(after); err != nil {
		t.Fatal(err)
	}
	if w2.MapIndex != 7 || w2.X != 3 || w2.Y != 11 || w2.Face != game.West {
		t.Errorf("讀回來在圖 %d 的 (%d,%d) 面 %v", w2.MapIndex, w2.X, w2.Y, w2.Face)
	}
	if s2.Rand.Seedof() != before.Seed {
		t.Errorf("種子讀回來是 %d，存的是 %d", s2.Rand.Seedof(), before.Seed)
	}
	for sel, want := range map[int]byte{0x00: 1, 0x05: 0x2A, 0x17: 0xFF, 0x84: 9} {
		if got := w2.Global(sel); got != want {
			t.Errorf("全域 %#02x 讀回來是 %#02x，存的是 %#02x", sel, got, want)
		}
	}

	// 同一顆種子必然給出同一串數列 —— 這是「存檔可重播」的條件。
	a1 := []int{s.Rand.Range(1, 1000), s.Rand.Range(1, 1000), s.Rand.Range(1, 1000)}
	a2 := []int{s2.Rand.Range(1, 1000), s2.Rand.Range(1, 1000), s2.Rand.Range(1, 1000)}
	for i := range a1 {
		if a1[i] != a2[i] {
			t.Fatalf("讀檔之後亂數序列就分岔了：%v vs %v", a1, a2)
		}
	}
}

// 中途事件不能只存座標：Y/N 已經顯示、但後半段還沒執行時，讀檔必須回到
// 同一個原始腳本位移，而不是從頭重跑付款或把答案預先套進分支。
func TestPendingEventStateRoundTrip(t *testing.T) {
	w := newWorld(t)
	s := game.NewSession(w, nil, nil, 4321)
	if !triggerSandsobarPrompt(w) {
		t.Fatal("中門特殊設施沒有停在 Sandsobar 的原始 Y/N 提問")
	}
	if w.Pending == nil || w.Pending.Kind != game.PromptYesNo ||
		w.Pending.Segment != 61 || w.Pending.Script != 0 {
		t.Fatalf("沒有停在 Y/N 輸入：%+v", w.Pending)
	}
	before := s.State()
	if before.Version != game.StateVersion || before.Pending == nil {
		t.Fatalf("State 沒帶出續跑點：%+v", before)
	}

	b, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	var after game.State
	if err := json.Unmarshal(b, &after); err != nil {
		t.Fatal(err)
	}
	w2 := newWorld(t)
	s2 := game.NewSession(w2, nil, nil, 1)
	if err := s2.LoadState(after); err != nil {
		t.Fatalf("讀回中途事件：%v", err)
	}
	if w2.Pending == nil || w2.Pending.Kind != game.PromptYesNo ||
		w2.Pending.Segment != w.Pending.Segment || w2.Pending.Script != w.Pending.Script ||
		w2.Pending.Offset != w.Pending.Offset {
		t.Fatalf("續跑定位不一致：原=%+v，讀回=%+v", w.Pending, w2.Pending)
	}
	if w2.Message != w.Message || !w2.MessageWait {
		t.Errorf("讀回提問文字／鎖定錯誤：%q wait=%v", w2.Message, w2.MessageWait)
	}
	if !w2.ResumeYesNo(false) {
		t.Fatal("讀檔後 N 無法讓事件續跑")
	}
	if w2.Pending != nil {
		t.Errorf("讀檔後回答完仍有續跑點：%+v", w2.Pending)
	}
}

// triggerSandsobarPrompt 走的是原始 Middlegate 的 `0e 11` 特殊設施：
// 它必須轉派到 EVENTSI 腳本庫段 61，而不是測試自行塞一段 Y/N 腳本。
func triggerSandsobarPrompt(w *game.World) bool {
	w.MapIndex, w.X, w.Y = 0, 0, 5
	w.Trigger()
	return w.Pending != nil && w.Pending.Kind == game.PromptYesNo
}

// 第 1 版還沒有 Pending 欄；升級版必須能正常開啟這些舊檔。
func TestLoadStateAcceptsLegacyV1(t *testing.T) {
	w := newWorld(t)
	s := game.NewSession(w, nil, nil, 1)
	st := s.State()
	st.Version = 1
	st.Pending = nil
	if err := s.LoadState(st); err != nil {
		t.Fatalf("第 1 版存檔不該被拒絕：%v", err)
	}
}

// 存檔的欄位要擋得住壞值，不能默默把隊伍放到不存在的地圖上。
func TestLoadStateRejectsBadValues(t *testing.T) {
	w := newWorld(t)
	s := game.NewSession(w, nil, nil, 1)
	for _, st := range []game.State{
		{Version: game.StateVersion, Map: 999},
		{Version: game.StateVersion, Map: 0, X: 99, Y: 0},
		{Version: game.StateVersion + 1},
	} {
		if err := s.LoadState(st); err == nil {
			t.Errorf("%+v 應該被擋下來", st)
		}
	}
}
