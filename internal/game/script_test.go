package game_test

import (
	"sort"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
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

// 0x2f／0x30 的結果要進條件暫存器：答對 1、答錯 0。
func TestMatchText(t *testing.T) {
	w := newWorld(t)
	for _, tc := range []struct {
		name   string
		answer string
		want   byte
	}{
		{"答錯", "WRONG", 0},
		{"答對", "cartograph", 1},
	} {
		w.Result = 0xFF
		script := append([]byte{game.OpAskText, game.OpMatchText}, textCipher("CARTOGRAPH")...)
		script = append(script, 0x00)
		w.RunScriptForTest(script)
		if w.Pending == nil || w.Pending.Kind != game.PromptText {
			t.Fatalf("%s：0x2f 沒停在文字輸入：%+v", tc.name, w.Pending)
		}
		if !w.ResumeText(tc.answer) {
			t.Fatalf("%s：文字輸入無法續跑", tc.name)
		}
		if w.Result != tc.want {
			t.Errorf("%s：ds:042F 是 %d，該是 %d", tc.name, w.Result, tc.want)
		}
	}
}

func textCipher(answer string) []byte {
	out := make([]byte, 10)
	for i := range out {
		out[i] = 0xFA // 原版十格緩衝區的空白填充
	}
	for i := 0; i < len(answer) && i < len(out); i++ {
		out[i] = byte(0x11A - int(answer[i]))
	}
	return out
}

// 0x24／0x25 是全隊湊錢：湊得出才扣，湊不出一毛都不動。
func TestPayOpcodes(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		op    byte
		off   int
		width int
	}{
		{"金錢", 0x24, 102, 4},
		{"寶石", 0x25, 92, 2},
	} {
		w := newWorld(t)
		party := append([]game.Character(nil), cs[:3]...)
		w.Party = party
		for i := range party {
			party[i].SetFieldValue(tc.off, tc.width, 100)
		}

		// 湊得出：三個人各 100，要 250 → 前兩人被掏空、第三人剩 50。
		w.Result = 0
		w.RunScriptForTest([]byte{tc.op, 0xFA, 0x00, 0x00})
		if w.Result != 1 {
			t.Errorf("%s：湊得出 250 卻回 %d", tc.name, w.Result)
		}
		got := []uint32{
			party[0].FieldValue(tc.off, tc.width),
			party[1].FieldValue(tc.off, tc.width),
			party[2].FieldValue(tc.off, tc.width),
		}
		if got[0] != 0 || got[1] != 0 || got[2] != 50 {
			t.Errorf("%s：扣完剩 %v，該是 [0 0 50]", tc.name, got)
		}

		// 湊不出：剩 50，要 250 → 條件 0，而且一毛都不能動。
		w.Result = 1
		w.RunScriptForTest([]byte{tc.op, 0xFA, 0x00, 0x00})
		if w.Result != 0 {
			t.Errorf("%s：湊不出卻回 %d", tc.name, w.Result)
		}
		if v := party[2].FieldValue(tc.off, tc.width); v != 50 {
			t.Errorf("%s：湊不出卻扣了，剩 %d", tc.name, v)
		}
	}
}

// 0x2d：三個旗標各比一個欄位，任一項對上就算數。
func TestHasMember(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld(t)
	party := append([]game.Character(nil), cs[:3]...)
	w.Party = party
	party[0].SetFieldByte(12, 0x00, 1) // 性別 1
	party[0].SetFieldByte(14, 0x00, 2) // 種族 2
	party[0].SetFieldByte(15, 0x00, 3) // 職業 3
	for i := 1; i < 3; i++ {
		party[i].SetFieldByte(12, 0x00, 0)
		party[i].SetFieldByte(14, 0x00, 0)
		party[i].SetFieldByte(15, 0x00, 0)
	}
	for _, tc := range []struct {
		name   string
		spec   byte
		second byte
		want   byte
	}{
		// bit 5 沒設 ＝ 找「沒對上」的人，所以 `2d 40 00`（性別 0 ＝ 男）
		// 問的是「隊伍裡有沒有女性」。
		{"有人性別不是 1", 0x40 | 1, 0, 1},
		{"有人性別不是 5", 0x40 | 5, 0, 1},
		{"有人種族不是 2", 0x80 | 2, 0, 1},
		{"三個人性別都是 0 或 1，沒有人不是 0 或 1 以外", 0x40 | 0, 0, 1},
		// bit 5 設 ＝ 找「對上」的人。
		{"有職業 3 的人", 0x20 | 3, 0, 1},
		{"沒有職業 7 的人", 0x20 | 7, 0, 0},
		{"種族 3 沒人（bit 7 設了就不比職業）", 0xA0 | 3, 0, 0},
		{"有種族 2 的人", 0xA0 | 2, 0, 1},
		// 種族與性別都沒要比 → 比職業，值取自**第二個運算元**。
		// 八個職業考驗的閘門就是這一型（`2d 0N 05`）。
		{"有人職業不是 5", 0x04, 0x05, 1},
		{"三個人職業都不是 9，所以有人不是 9", 0x04, 0x09, 1},
		{"兩個值都放行：職業 0 與 3 就是全隊，沒有人不符合", 0x00, 0x03, 0},
	} {
		w.Result = 0xFF
		w.RunScriptForTest([]byte{0x2d, tc.spec, tc.second, 0x00})
		if w.Result != tc.want {
			t.Errorf("%s：ds:042F 是 %d，該是 %d", tc.name, w.Result, tc.want)
		}
	}
}

// 八個職業考驗的閘門 `2d 0N 05`：**兩個值都放行**（該職業與盜賊），
// 而且沒有 bit 5 表示它找的是「不符合的人」—— 成立代表擋下來。
//
// 舊的讀法（沒有旗標就一律不成立）會讓八個閘門永遠不成立，等於門戶大開。
func TestHasMemberClassTrialGate(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld(t)
	party := append([]game.Character(nil), cs[:3]...)
	w.Party = party

	// 全隊都是巫師（4）→ 沒有人不符合 → 不擋。
	for i := range party {
		party[i].SetFieldByte(15, 0x00, 4)
	}
	w.Result = 0xFF
	w.RunScriptForTest([]byte{0x2d, 0x04, 0x05, 0x00})
	if w.Result != 0 {
		t.Error("全隊巫師卻被擋下來了")
	}

	// 巫師帶盜賊（5）也放行 —— 第二個值就是為了這個。
	party[2].SetFieldByte(15, 0x00, 5)
	w.Result = 0xFF
	w.RunScriptForTest([]byte{0x2d, 0x04, 0x05, 0x00})
	if w.Result != 0 {
		t.Error("巫師帶盜賊被擋下來了")
	}

	// 混進一個騎士（0）就擋。
	party[1].SetFieldByte(15, 0x00, 0)
	w.Result = 0
	w.RunScriptForTest([]byte{0x2d, 0x04, 0x05, 0x00})
	if w.Result != 1 {
		t.Error("隊伍裡有騎士卻沒被擋")
	}
}

// 0x2e：只教對系的職業，位元是 OR 進去的。
func TestTeachSpell(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld(t)
	party := append([]game.Character(nil), cs[:4]...)
	w.Party = party
	party[0].SetFieldByte(15, 0x00, 4) // 巫師
	party[1].SetFieldByte(15, 0x00, 2) // 弓箭手
	party[2].SetFieldByte(15, 0x00, 3) // 牧師
	party[3].SetFieldByte(15, 0x00, 0) // 騎士
	for i := range party {
		party[i].SetFieldByte(81, 0x00, 0x01) // 先各留一個位元
	}

	// 巫師系：運算元 1 = 110（落在 +81），位元 0x40。
	w.RunScriptForTest([]byte{0x2e, 110, 0x40, 0x00})
	if got := party[0].FieldByte(81); got != 0x41 {
		t.Errorf("巫師的 +81 是 %#02x，該是 0x41（OR 進去，不是覆蓋）", got)
	}
	if got := party[1].FieldByte(81); got != 0x41 {
		t.Errorf("弓箭手的 +81 是 %#02x，該跟巫師同系", got)
	}
	if got := party[2].FieldByte(81); got != 0x01 {
		t.Errorf("牧師的 +81 被巫師系的教學動到了：%#02x", got)
	}
	if got := party[3].FieldByte(81); got != 0x01 {
		t.Errorf("騎士的 +81 被動到了：%#02x", got)
	}

	// 牧師系：bit 7 打開。
	w.RunScriptForTest([]byte{0x2e, 110 | 0x80, 0x08, 0x00})
	if got := party[2].FieldByte(81); got != 0x09 {
		t.Errorf("牧師的 +81 是 %#02x，該是 0x09", got)
	}
	if got := party[0].FieldByte(81); got != 0x41 {
		t.Errorf("巫師被牧師系的教學動到了：%#02x", got)
	}
}

// 0x31：抗性 0 的人一定吃滿，抗性 100 的人一定擋下，重症的碰都不碰。
func TestHarmOpcode(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld(t)
	w.Rand = game.NewRand(99) // 抗性判定要擲骰，沒有產生器就一律不擋
	party := append([]game.Character(nil), cs[:3]...)
	w.Party = party
	for i := range party {
		party[i].SetFieldByte(38, 0x00, 0)
		party[i].SetFieldValue(94, 2, 200)  // 目前生命
		party[i].SetFieldValue(96, 2, 200)  // 基礎生命上限
		party[i].SetFieldValue(116, 2, 200) // 有效生命上限
	}
	party[0].SetFieldByte(22, 0x00, 0)   // 沒有抗性
	party[1].SetFieldByte(22, 0x00, 100) // 擲不過
	party[2].SetFieldByte(22, 0x00, 0)
	party[2].SetFieldByte(38, 0x00, game.CondPetrified) // 重症

	// 對象 1 = 第 1 人，傷害 30。
	for i := 0; i < 20; i++ {
		party[0].SetFieldValue(94, 2, 200)
		w.RunScriptForTest([]byte{0x31, 1, 30, 0, 0x00})
		if got := party[0].FieldValue(94, 2); got != 170 {
			t.Fatalf("抗性 0 卻只掉到 %d，該是 170", got)
		}
	}
	// 抗性 100 擋的是 rand(1,100) < 100，也就是擲出 100 才會被打中 ——
	// 是 99%，不是 100%。測統計而不是絕對。
	hit := 0
	for i := 0; i < 200; i++ {
		party[1].SetFieldValue(94, 2, 200)
		w.RunScriptForTest([]byte{0x31, 2, 30, 0, 0x00})
		if party[1].FieldValue(94, 2) != 200 {
			hit++
		}
	}
	if hit > 10 {
		t.Errorf("抗性 100 在 200 次裡被打中 %d 次，預期個位數", hit)
	}
	w.RunScriptForTest([]byte{0x31, 3, 30, 0, 0x00})
	if got := party[2].FieldValue(94, 2); got != 200 {
		t.Errorf("重症的隊員被扣到 %d，該完全不碰", got)
	}
}

// 0x2a：15 個位元組的獎賞，金錢是 3 位元組小端序。
func TestSetReward(t *testing.T) {
	w := newWorld(t)
	script := []byte{0x2a,
		0x40, 0x9C, 0x00, // 金錢 40000
		0x2C, 0x01, // 寶石 300
		10, 11, 12,
		20, 21, 22,
		30, 31, 32,
		0x00}
	w.RunScriptForTest(script)
	r := w.Reward
	if !r.Pending {
		t.Fatal("獎賞沒有被標成待領")
	}
	if r.Gold != 40000 {
		t.Errorf("金錢是 %d，該是 40000（3 位元組小端序）", r.Gold)
	}
	if r.Gems != 300 {
		t.Errorf("寶石是 %d，該是 300", r.Gems)
	}
	want := [3][3]byte{{10, 11, 12}, {20, 21, 22}, {30, 31, 32}}
	if r.Items != want {
		t.Errorf("三件物品是 %v，該是 %v", r.Items, want)
	}
}

// 0x23：0B5h 單數日、0B6h 偶數日、其餘是閉區間。
func TestDateCond(t *testing.T) {
	w := newWorld(t)
	if w.Globals == nil {
		w.Globals = map[uint16]byte{}
	}
	w.Globals[0x03CA] = 9 // 選 ds:03B4 那一格
	for _, tc := range []struct {
		day  byte
		a, b byte
		want byte
	}{
		{41, 0xB5, 0, 1}, // 單數
		{40, 0xB5, 0, 0},
		{40, 0xB6, 0, 1}, // 偶數
		{41, 0xB6, 0, 0},
		{50, 40, 60, 1}, // 區間內
		{40, 40, 60, 1}, // 下界含
		{60, 40, 60, 1}, // 上界含
		{39, 40, 60, 0},
		{61, 40, 60, 0},
	} {
		w.Globals[0x03B4] = tc.day
		w.Result = 0xFF
		w.RunScriptForTest([]byte{0x23, tc.a, tc.b, 0x00})
		if w.Result != tc.want {
			t.Errorf("第 %d 天、條件 %#02x/%d：ds:042F 是 %d，該是 %d",
				tc.day, tc.a, tc.b, w.Result, tc.want)
		}
	}
}

// 0x21：改寫一格的兩個平面，格子索引是 nibble 打包的 Y/X。
func TestSetCell(t *testing.T) {
	w := newWorld(t)
	m := w.CurrentMap()
	if m == nil {
		t.Skip("沒有地圖")
	}
	// 0x57 = X 7、Y 5 → 索引 5*16+7 = 87。
	const cell = 0x57
	m.Terrain[cell], m.Attr[cell] = 0, 0
	w.RunScriptForTest([]byte{0x21, cell, 0xAA, 0x55, 0x00})
	if m.Terrain[cell] != 0xAA {
		t.Errorf("Terrain 是 %#02x，該是 0xAA", m.Terrain[cell])
	}
	if m.Attr[cell] != 0x55 {
		t.Errorf("Attr 是 %#02x，該是 0x55", m.Attr[cell])
	}
	// 別的格子不能被動到。
	if m.Terrain[cell+1] == 0xAA && m.Attr[cell+1] == 0x55 {
		t.Error("隔壁格也被改了")
	}
}

// 0x26 選人，之後「對象 9」就指到選中的那位。
func TestPickMember(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld(t)
	w.Rand = game.NewRand(7)
	party := append([]game.Character(nil), cs[:4]...)
	w.Party = party
	for i := range party {
		party[i].SetFieldByte(38, 0x00, 0)
		party[i].SetFieldByte(22, 0x00, 0) // 沒有抗性，傷害一定進得去
		party[i].SetFieldValue(94, 2, 100)
		party[i].SetFieldValue(96, 2, 100)
		party[i].SetFieldValue(116, 2, 100)
	}

	// 選第 3 人，然後用「對象 9」打他。0x26 必須先停住，不能在
	// 腳本啟動前預設選人。
	w.RunScriptForTest([]byte{0x26, 0x31, 9, 10, 0, 0x00})
	if w.Pending == nil || w.Pending.Kind != game.PromptMember {
		t.Fatalf("0x26 沒停在選人：%+v", w.Pending)
	}
	if got := party[2].FieldValue(94, 2); got != 100 {
		t.Fatalf("尚未選人就傷到第 3 人：生命 %d", got)
	}
	if !w.ResumeMember(3) {
		t.Fatal("選第 3 人無法讓事件續跑")
	}
	if w.Selected != 3 {
		t.Fatalf("選中的是 %d，該是 3", w.Selected)
	}
	if got := party[2].FieldValue(94, 2); got != 90 {
		t.Errorf("第 3 人的生命是 %d，該是 90", got)
	}
	for _, i := range []int{0, 1, 3} {
		if got := party[i].FieldValue(94, 2); got != 100 {
			t.Errorf("第 %d 人被誤傷到 %d", i+1, got)
		}
	}

	// 選死人不算數：狀況 >= 81h 一律拒絕。
	party[1].SetFieldByte(38, 0x00, game.CondPetrified)
	w.RunScriptForTest([]byte{0x26, 0x00})
	if !w.ResumeMember(2) {
		t.Fatal("選石化隊員時事件無法續跑")
	}
	if w.Selected != 0 {
		t.Errorf("選了石化的隊員卻回 %d", w.Selected)
	}
}

// 稽核：兩個事件檔裡實際出現的每個 opcode，直譯器都要有對應的分支。
//
// 這條守的是「文件說全部有解、程式卻少一個」這種漂移。opcode 的長度表
// 是原版的（`data/opcodes.json`），所以掃描器走得完；真正要問的是
// **走到那個 opcode 的時候有沒有事發生** —— 沒有分支的 opcode 會被
// 安靜地跳過，畫面上只呈現為「那一格少做了一件事」。
func TestEveryOpcodeInDataIsHandled(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skipf("載不到 opcode 長度表：%v", err)
	}
	if game.OpLen(0x04) < 1 {
		t.Fatal("正對照失敗：0x04 的長度是 0，掃描器不會動")
	}
	w := &game.World{}
	seen := map[byte]bool{}
	for _, name := range []string{"EVENTSI.DAT", "EVENTSO.DAT"} {
		segs, err := events.Parse(orig(t, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, sg := range segs {
			for _, sc := range sg.Scripts {
				for p := 0; p < len(sc); {
					n := game.OpLen(sc[p])
					if n < 1 || p+n > len(sc) {
						break
					}
					seen[sc[p]] = true
					p += n
				}
			}
			// 整段丟進直譯器，沒有分支的 opcode 會落進 default 被記下來。
			for _, sc := range sg.Scripts {
				w.RunScriptForTest(sc)
				drainEventPrompt(t, w)
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("一個 opcode 都沒掃到")
	}
	t.Logf("腳本裡出現 %d 種 opcode", len(seen))
	if len(w.Unhandled) > 0 {
		var ops []int
		for op := range w.Unhandled {
			ops = append(ops, int(op))
		}
		sort.Ints(ops)
		t.Errorf("有 %d 種 opcode 沒有分支：%x", len(ops), ops)
	}
}

// 打字謎題的答案就寫在腳本裡（`0x2f` 後面那條 `0x30`）。
//
// 原版的謎底靠英文文字遊戲，翻成中文之後線索與答案對不起來 ——
// 所以 remake 把答案附在提問後面，而答案要從資料解出來、不是另外建表。
func TestTextPuzzleAnswers(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skipf("載不到 opcode 長度表：%v", err)
	}
	want := map[string]bool{
		"46": true, "23": true, "64": true, "32": true,
		"MEENU": true, "KEYS": true, "DRUIDS": true,
	}
	got := map[string]bool{}
	w := &game.World{}
	for _, name := range []string{"EVENTSI.DAT", "EVENTSO.DAT"} {
		segs, err := events.Parse(orig(t, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, sg := range segs {
			for _, sc := range sg.Scripts {
				w.TextExpect = ""
				w.RunScriptForTest(sc)
				for n := 0; w.Pending != nil && n < 64; n++ {
					if w.TextExpect != "" {
						got[w.TextExpect] = true
					}
					switch w.Pending.Kind {
					case game.PromptKey:
						w.ResumeKey()
					case game.PromptYesNo:
						w.ResumeYesNo(false)
					case game.PromptMember:
						w.ResumeMember(0)
					case game.PromptText:
						w.ResumeText(w.TextExpect)
					}
				}
				if w.TextExpect != "" {
					got[w.TextExpect] = true
				}
			}
		}
	}
	for a := range want {
		if !got[a] {
			t.Errorf("解不出答案 %q", a)
		}
	}
	for a := range got {
		if !want[a] {
			t.Errorf("多解出一個答案 %q —— 解碼可能錯了", a)
		}
	}
}

// drainEventPrompt 只給 opcode 稽核使用：真正 UI 不會預設答案，而這條測試
// 的目的是讓直譯器掃到每一段後半，確認沒有漏掉 handler。
func drainEventPrompt(t *testing.T, w *game.World) {
	t.Helper()
	for n := 0; w.Pending != nil; n++ {
		if n >= 64 {
			t.Fatalf("事件輸入連續停了超過 64 次：%+v", w.Pending)
		}
		switch w.Pending.Kind {
		case game.PromptKey:
			if !w.ResumeKey() {
				t.Fatal("0x07 無法續跑")
			}
		case game.PromptYesNo:
			if !w.ResumeYesNo(false) {
				t.Fatal("Y/N 無法續跑")
			}
		case game.PromptMember:
			if !w.ResumeMember(0) {
				t.Fatal("選人無法續跑")
			}
		case game.PromptText:
			if !w.ResumeText(w.TextExpect) {
				t.Fatal("文字輸入無法續跑")
			}
		default:
			t.Fatalf("未知輸入種類 %q", w.Pending.Kind)
		}
	}
}
