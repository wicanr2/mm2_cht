package game_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/gamedata"
)

// testData 讀 repo 根目錄的 data/，與 TestMain 同一份。
func testData(t *testing.T) *gamedata.Data {
	t.Helper()
	dir := gamedata.Dir()
	if _, err := os.Stat(dir); err != nil {
		dir = filepath.Join("..", "..", "data")
	}
	d, err := gamedata.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// 腳本實際用到的欄位選擇器都要對得到角色記錄裡的位置。
//
// 選擇器表出自原版 `sub_1AA00` 的 128 項跳表。這條掃真的資料，
// 確認「表解對了」不是只在少數幾項上成立。
func TestScriptFieldSelectorsResolve(t *testing.T) {
	d := testData(t)
	used := map[byte]int{}
	forEachScript(t, func(script []byte) {
		for p := 0; p < len(script); {
			op := script[p]
			n := d.OpLen(op)
			if n < 1 || p+n > len(script) {
				return
			}
			if (op == game.OpTestField || op == game.OpSetField) && p+2 < len(script) {
				used[script[p+2]]++
			}
			p += n
		}
	})
	if len(used) < 20 {
		t.Fatalf("只看到 %d 種選擇器，資料掃得不對", len(used))
	}
	unresolved := 0
	for sel := range used {
		f, ok := d.Fields.Lookup(int(sel))
		if !ok {
			unresolved++
			continue
		}
		if f.Offset < 0 || f.Offset >= game.RecordSize {
			t.Errorf("選擇器 %#02x 對到記錄 +%d，超出 %d bytes", sel, f.Offset, game.RecordSize)
		}
	}
	// 128 項裡只有 0x00 與 0x01 不是「基底 + 位移」，腳本用到的是 0x01。
	if unresolved > 1 {
		t.Errorf("%d 種選擇器解不出偏移，預期最多 1", unresolved)
	}
}

// 把每一段腳本都跑一遍，確認寫入不會跑出記錄、而且真的有腳本在改隊伍。
//
// 「不會 panic」擋不住「一段腳本都沒改到東西」——所以同時數改了幾段。
func TestScriptFieldWritesTakeEffect(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:6]...)
	game.NewSession(w, party, nil, 1)

	before := snapshot(party)
	changed := 0
	forEachScript(t, func(script []byte) {
		w.RunScriptForTest(script)
		if now := snapshot(party); now != before {
			changed++
			before = now
		}
	})
	if changed == 0 {
		t.Error("跑完全部腳本，隊伍一個位元組都沒被改到")
	}
	t.Logf("有 %d 段腳本改動了隊伍資料", changed)
	for i := range party {
		if len(party[i].Raw) != game.RecordSize {
			t.Fatalf("第 %d 人的記錄變成 %d bytes", i, len(party[i].Raw))
		}
	}
}

// 隊伍持有物品的判斷要同時看已裝備與背包兩區。
//
// 這兩區的位置（記錄 +40 與 +58）是從 `sub_CE12` 推出來的，
// 而原版的 `sub_19ABC` 掃的正是 `+0x28` 與 `+0x3A` —— 互相印證。
func TestHasItemChecksBothAreas(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:6]...)
	game.NewSession(w, party, nil, 1)

	const equipSlot0, packSlot0 = 40, 58
	for _, tc := range []struct {
		off  int
		name string
	}{{equipSlot0, "已裝備"}, {packSlot0, "背包"}} {
		party[2].SetFieldByte(tc.off, 0x00, 0xA7)
		w.Result = 0
		w.RunScriptForTest([]byte{game.OpHasItem, 0, 0xA7})
		if w.Result == 0 {
			t.Errorf("%s區放了物品 0xA7，卻沒被找到", tc.name)
		}
		party[2].SetFieldByte(tc.off, 0x00, 0)
	}
	w.Result = 0
	w.RunScriptForTest([]byte{game.OpHasItem, 0, 0xA7})
	if w.Result != 0 {
		t.Error("物品拿掉之後還是回報找得到")
	}
}

func snapshot(party []game.Character) string {
	b := make([]byte, 0, len(party)*game.RecordSize)
	for i := range party {
		b = append(b, party[i].Raw...)
	}
	return string(b)
}

// forEachScript 把 EVENTSI/EVENTSO 的每一段腳本交給 fn。
func forEachScript(t *testing.T, fn func(script []byte)) {
	t.Helper()
	for _, name := range []string{"EVENTSI.DAT", "EVENTSO.DAT"} {
		segs, err := events.Parse(orig(t, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range segs {
			for _, script := range s.Scripts {
				fn(script)
			}
		}
	}
}

// 腳本裡寫死的傳送目標都要是存在的地圖。
//
// 這是「地圖 = 目標 & 0x3F」解對了的驗收條件：位元遮罩取錯的話，
// 立刻會出現指向第 60 張以後的目標。
func TestTeleportTargetsExist(t *testing.T) {
	d := testData(t)
	targets := map[int]int{}
	random := 0
	forEachScript(t, func(script []byte) {
		for p := 0; p < len(script); {
			op := script[p]
			n := d.OpLen(op)
			if n < 1 || p+n > len(script) {
				return
			}
			if op == game.OpTeleport && p+1 < len(script) {
				v := script[p+1]
				if v&0x40 != 0 || v >= 0x80 {
					random++ // 隨機目標，沒有寫死的地圖編號
				} else {
					targets[int(v&0x3F)]++
				}
			}
			p += n
		}
	})
	if len(targets) < 10 {
		t.Fatalf("只看到 %d 種傳送目標，資料掃得不對", len(targets))
	}
	for m := range targets {
		if m >= game.MapCount {
			t.Errorf("傳送指向第 %d 張地圖，但只有 %d 張", m, game.MapCount)
		}
	}
	t.Logf("寫死的傳送目標 %d 種、隨機傳送 %d 次", len(targets), random)
}

// 傳送要真的把隊伍送過去。
func TestTeleportMovesParty(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	game.NewSession(w, append([]game.Character(nil), cs[:6]...), nil, 1)
	w.MapIndex, w.X, w.Y = 0, 1, 1

	// 0c 05 a3 → 第 5 張地圖的 (3, 10)
	w.RunScriptForTest([]byte{game.OpTeleport, 0x05, 0xA3})
	if w.MapIndex != 5 || w.X != 3 || w.Y != 10 {
		t.Errorf("傳送後在圖 %d 的 (%d,%d)，預期圖 5 的 (3,10)", w.MapIndex, w.X, w.Y)
	}
	if !w.Teleported {
		t.Error("Teleported 沒有設起來")
	}
}
