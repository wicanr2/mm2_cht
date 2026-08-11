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
			t.Logf("選擇器 %#02x 解不出偏移（出現 %d 次）", sel, used[sel])
			continue
		}
		if f.Offset < 0 || f.Offset >= game.RecordSize {
			t.Errorf("選擇器 %#02x 對到記錄 +%d，超出 %d bytes", sel, f.Offset, game.RecordSize)
		}
	}
	// 128 項裡只有 `0x00` 與 `0x01` 不是「基底 + 位移」。認出腳本庫
	// 那幾段之後兩個都出現了（`0x00` 八次、`0x01` 十次），所以基準是 2。
	if unresolved > 2 {
		t.Errorf("%d 種選擇器解不出偏移，預期最多 2", unresolved)
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

// 加減欄位要照寬度飽和，扣不動時要把結果清成 0。
//
// 「扣不動 → 結果 0」是付款那條路徑的核心：`0x10`／`0x11` 讀的就是它。
func TestFieldAddAndSubtract(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:6]...)
	game.NewSession(w, party, nil, 1)

	const selGold = 0x3E // 4 bytes，記錄 +102；第三個參數是寫回的位元組數
	party[0].SetFieldValue(102, 4, 500)

	// 1f 01 3e 00 <200,0,0> → 第 1 人加 200 黃金
	w.RunScriptForTest([]byte{game.OpAdd, 0x01, selGold, 0x03, 200, 0, 0})
	if got := party[0].FieldValue(102, 4); got != 700 {
		t.Errorf("加完是 %d，預期 700", got)
	}
	// 20 01 3e 00 <100,0,0> → 扣 100
	w.RunScriptForTest([]byte{game.OpSub, 0x01, selGold, 0x03, 100, 0, 0})
	if got := party[0].FieldValue(102, 4); got != 600 {
		t.Errorf("扣完是 %d，預期 600", got)
	}
	if w.Result == 0 {
		t.Error("扣得動卻回報付不出來")
	}
	// 扣超過持有量：欄位不動，結果清 0
	w.RunScriptForTest([]byte{game.OpSub, 0x01, selGold, 0x03, 0x40, 0x9C, 0x00}) // 40000
	if got := party[0].FieldValue(102, 4); got != 600 {
		t.Errorf("扣不動卻改了欄位，變成 %d", got)
	}
	if w.Result != 0 {
		t.Error("扣不動時結果沒有清成 0")
	}

	// 單位元組欄位要飽和在 255，不能繞回去
	const selFood = 0x42 // 記錄 +37
	party[0].SetFieldValue(37, 1, 200)
	w.RunScriptForTest([]byte{game.OpAdd, 0x01, selFood, 0x01, 200, 0, 0})
	if got := party[0].FieldValue(37, 1); got != 255 {
		t.Errorf("加完是 %d，預期飽和在 255", got)
	}
}

// Y／N 詢問要先停在輸入點；答案出現後才寫入結果。
func TestAskWaitsForResponse(t *testing.T) {
	w := newWorld(t)
	game.NewSession(w, nil, nil, 1)
	w.Result = 1
	w.RunScriptForTest([]byte{game.OpAsk})
	if w.Pending == nil || w.Pending.Kind != game.PromptYesNo {
		t.Fatalf("0x09 沒停在 Y/N 輸入：%+v", w.Pending)
	}
	if w.Result != 0 {
		t.Errorf("尚未回答時結果是 %d，預期 0", w.Result)
	}
	if !w.ResumeYesNo(false) {
		t.Fatal("N 無法讓事件續跑")
	}
	if w.Pending != nil || w.Result != 0 {
		t.Errorf("答 N 後 Pending=%+v、結果=%d，預期完成且為 0", w.Pending, w.Result)
	}
	w.RunScriptForTest([]byte{game.OpAsk})
	if !w.ResumeYesNo(true) {
		t.Fatal("Y 無法讓事件續跑")
	}
	if w.Result != 1 {
		t.Errorf("答 Y 之後結果是 %d，預期 1", w.Result)
	}
}

// 走出地圖邊界要換到鄰接的那一張，座標從對邊進來。
//
// 方位由 `sub_1B75E` 定死：X 溢位讀 `+6`（東）、Y 溢位讀 `+5`（北）。
// 這條同時守著「北是 +Y」。
func TestCrossMapEdge(t *testing.T) {
	w := newWorld(t)
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, nil, nil, 1)
	s.UseAttrs(attrs)
	s.EncounterRate = 0

	// 找一張野外圖，它的東鄰不是自己。
	from := -1
	for i := range attrs {
		if !attrs[i].Indoor() && attrs[i].East() != i {
			from = i
			break
		}
	}
	if from < 0 {
		t.Skip("找不到東鄰不是自己的野外圖")
	}
	want := attrs[from].East()

	// 站在東邊界上朝東走。地形擋住就換一列試。
	moved := false
	for y := 0; y < game.MapH && !moved; y++ {
		w.MapIndex, w.X, w.Y, w.Face = from, game.MapW-1, y, game.East
		if ok, _ := s.Step(1); ok {
			moved = true
			if w.MapIndex != want {
				t.Fatalf("往東走到圖 %d，預期 %d", w.MapIndex, want)
			}
			if w.X != 0 || w.Y != y {
				t.Fatalf("換圖後在 (%d,%d)，預期 (0,%d)", w.X, w.Y, y)
			}
		}
	}
	if !moved {
		t.Skipf("圖 %d 的東邊界十六列都過不去", from)
	}
}

// 遭遇機率要跟著地圖走：城鎮 200、野外多半 100。
func TestEncounterRateComesFromMap(t *testing.T) {
	attrs, err := game.ParseMapAttrs(orig(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	if got := attrs[0].EncounterRate(); got != 200 {
		t.Errorf("Middlegate 的遭遇分母是 %d，預期 200", got)
	}
	lo, hi := 255, 0
	for i := range attrs {
		v := attrs[i].EncounterRate()
		if v == 0 {
			t.Errorf("圖 %d 的遭遇分母是 0", i)
		}
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if lo < 50 || hi > 250 {
		t.Errorf("遭遇分母的值域是 %d–%d，預期落在 50–250", lo, hi)
	}
}

// 全域變數要讀得回寫進去的值，而且腳本實際用到的選擇器都要有對應位址。
func TestGlobalVariables(t *testing.T) {
	d := testData(t)
	w := newWorld(t)
	game.NewSession(w, nil, nil, 1)

	// 1a 05 2a → 全域 5 設成 0x2A；17 05 00 → 讀回來
	w.RunScriptForTest([]byte{game.OpWriteGlobal, 0x05, 0x2A})
	w.Result = 0
	w.RunScriptForTest([]byte{game.OpReadGlobal, 0x05, 0x00})
	if w.Result != 0x2A {
		t.Errorf("讀回來是 %#02x，預期 0x2A", w.Result)
	}

	// 0x22 檢查 ds:03CA 的範圍。先把它設成 9，再問 8–10 與 1–3。
	w.RunScriptForTest([]byte{game.OpWriteGlobal, 0x84, 9})
	w.RunScriptForTest([]byte{game.OpInRange, 8, 10})
	if w.Result != 1 {
		t.Error("9 落在 8–10 卻回報不在範圍內")
	}
	w.RunScriptForTest([]byte{game.OpInRange, 1, 3})
	if w.Result != 0 {
		t.Error("9 不在 1–3 卻回報在範圍內")
	}

	// 腳本用到的選擇器都要對得到位址。
	bad := map[byte]int{}
	used := map[byte]int{}
	forEachScript(t, func(script []byte) {
		for p := 0; p < len(script); {
			op := script[p]
			n := d.OpLen(op)
			if n < 1 || p+n > len(script) {
				return
			}
			if op == game.OpReadGlobal || op == game.OpWriteGlobal {
				sel := script[p+1]
				used[sel]++
				w.SetGlobal(int(sel), 0x5A)
				if w.Global(int(sel)) != 0x5A {
					bad[sel]++
				}
				w.SetGlobal(int(sel), 0)
			}
			p += n
		}
	})
	if len(used) == 0 {
		t.Fatal("腳本裡一個全域選擇器都沒看到")
	}
	if len(bad) > 0 {
		t.Errorf("%d 種選擇器對不到位址：%v", len(bad), bad)
	}
	t.Logf("腳本用到 %d 種全域選擇器", len(used))
}

// 0x22 的參數值域要像世紀（一位數），不是任意位元組。
func TestInRangeArgsLookLikeCenturies(t *testing.T) {
	d := testData(t)
	lo, hi := 255, 0
	n := 0
	forEachScript(t, func(script []byte) {
		for p := 0; p < len(script); {
			op := script[p]
			l := d.OpLen(op)
			if l < 1 || p+l > len(script) {
				return
			}
			if op == game.OpInRange {
				n++
				for _, v := range []int{int(script[p+1]), int(script[p+2])} {
					if v < lo {
						lo = v
					}
					if v > hi {
						hi = v
					}
				}
			}
			p += l
		}
	})
	if n == 0 {
		t.Skip("腳本裡沒有 0x22")
	}
	t.Logf("0x22 出現 %d 次，參數值域 %d–%d", n, lo, hi)
	if hi > 20 {
		t.Errorf("參數上限是 %d，不像世紀", hi)
	}
}

// 0x28 要真的把背包裡的物品拿走，而且只拿一件。
//
// 移除時三個平行陣列要一起往前補（編號 +58、充能 +64、屬性 +70）——
// 只搬編號的話，剩下的物品會配到別人的充能與屬性。
func TestTakeItemRemovesFromBackpack(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:6]...)
	game.NewSession(w, party, nil, 1)

	const packID, packCharge, packAttr = 58, 64, 70
	// 背包放三件：目標在中間，後面那件帶著自己的充能與屬性。
	for i, v := range []byte{0x11, 0xA7, 0x33} {
		party[1].SetFieldByte(packID+i, 0x00, v)
		party[1].SetFieldByte(packCharge+i, 0x00, byte(0x50+i))
		party[1].SetFieldByte(packAttr+i, 0x00, byte(0x60+i))
	}
	// 另一個人也帶一件同編號的，確認只拿一件。
	party[3].SetFieldByte(packID, 0x00, 0xA7)

	w.RunScriptForTest([]byte{game.OpTakeItem, 0, 0xA7})
	if w.Result == 0 {
		t.Fatal("背包裡有 0xA7 卻沒拿到")
	}
	pack := party[1].Backpack()
	if pack[0].ID != 0x11 || pack[1].ID != 0x33 || pack[2].ID != 0 {
		t.Errorf("拿走後背包是 %d,%d,%d，預期 0x11,0x33,0", pack[0].ID, pack[1].ID, pack[2].ID)
	}
	// 後面那件的充能與屬性要跟著往前補。
	if pack[1].Charge != 0x52 || pack[1].Attr != 0x62 {
		t.Errorf("補上來的那件充能 %#02x 屬性 %#02x，預期 0x52／0x62",
			pack[1].Charge, pack[1].Attr)
	}
	if party[3].Backpack()[0].ID != 0xA7 {
		t.Error("另一個人的那件也被拿走了，原版一次只拿一件")
	}
}

// 0x2c 讓時間前進。
func TestAdvanceTime(t *testing.T) {
	w := newWorld(t)
	game.NewSession(w, nil, nil, 1)
	w.RunScriptForTest([]byte{game.OpAdvanceTime, 7})
	w.RunScriptForTest([]byte{game.OpAdvanceTime, 3})
	if w.Time != 10 {
		t.Errorf("時間是 %d，預期 10", w.Time)
	}
}

// 0x0b 的圖號要全部落在 monsters.16 的 75 個槽內，而且解得到圖。
//
// 表裡的值若解錯（位址差幾個位元組），立刻會冒出 0 或 75 以上的號碼。
func TestShowPictureResolves(t *testing.T) {
	d := testData(t)
	w := newWorld(t)
	game.NewSession(w, nil, nil, 1)

	seen := map[int]int{}
	zero := 0
	forEachScript(t, func(script []byte) {
		for p := 0; p < len(script); {
			op := script[p]
			n := d.OpLen(op)
			if n < 1 || p+n > len(script) {
				return
			}
			if op == game.OpShowPicture {
				w.RunScriptForTest(script[p : p+n])
				if w.Picture == 0 {
					zero++
				} else {
					seen[w.Picture]++
				}
			}
			p += n
		}
	})
	if len(seen) == 0 {
		t.Fatal("一個圖號都沒解出來")
	}
	for pic := range seen {
		if pic < 1 || pic > 75 {
			t.Errorf("圖號 %d 超出 1–75", pic)
		}
	}
	if zero > 0 {
		t.Errorf("有 %d 次查不到圖號", zero)
	}
	t.Logf("0x0b 用到 %d 種圖號", len(seen))
}

// opcode `0x19`（給予物品）要真的把東西放進背包。
//
// 它先前只是被長度表跳過 —— 掃描不會壞、也不會有錯誤訊息，
// 玩家看到的只是「劇情說給了你一把劍，背包裡沒有」。
func TestGiveItemFillsBackpack(t *testing.T) {
	w := newWorld(t)
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	party := append([]game.Character(nil), cs[:2]...)
	game.NewSession(w, party, nil, 1)

	// 先清空兩個人的背包，讓落點是確定的。
	for i := range party {
		for s := 0; s < 6; s++ {
			party[i].SetFieldByte(58+s, 0x00, 0)
		}
	}
	const id, charge, attr = 0xA7, 3, 0x11
	w.RunScriptForTest([]byte{game.OpGiveItem, 0, id, charge, attr})
	if w.Result != 1 {
		t.Fatalf("結果暫存器是 %d，預期 1（放進去了）", w.Result)
	}
	got := party[0].Backpack()[0]
	if got.ID != id || got.Charge != charge || got.Attr != attr {
		t.Errorf("背包第一格是 %+v，預期 ID %d／次數 %d／屬性 %d",
			got, id, charge, attr)
	}

	// 對象 ≥ 0x80 時編號改從結果暫存器取（與 0x18 同一個約定）。
	w.Result = 0x42
	w.RunScriptForTest([]byte{game.OpGiveItem, 0x80, 0, 1, 0})
	if got := party[0].Backpack()[1]; got.ID != 0x42 {
		t.Errorf("對象 0x80 應該用結果暫存器裡的編號，實際放進 %d", got.ID)
	}

	// 全隊背包塞滿之後掉在地上，走 Treasure! 那條領取路徑。
	for i := range party {
		for s := 0; s < 6; s++ {
			party[i].SetFieldByte(58+s, 0x00, 0x50)
		}
	}
	w.Reward = game.Reward{}
	w.RunScriptForTest([]byte{game.OpGiveItem, 0, id, charge, attr})
	if w.Result != 0 {
		t.Errorf("背包全滿時結果應該是 0，實際 %d", w.Result)
	}
	if !w.Reward.Pending || w.Reward.Items[0][0] != id {
		t.Errorf("背包全滿時東西該掉在地上，Reward = %+v", w.Reward)
	}
}
