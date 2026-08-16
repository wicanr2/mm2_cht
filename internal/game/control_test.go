package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
	"github.com/wicanr2/mm2_cht/internal/assets/text"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// 結局控制室（`0e fd`）的驗收。機制見 docs/re/05-2smith-control-room.md。

// 控制室那一格真的在資料裡，而且分派得到裝置。
//
// 這條掃的是原版事件檔，不是 remake 自己的表 —— 分派表寫對了但資料裡
// 沒有那一格，玩家永遠走不到，而兩者長得一模一樣。
func TestControlRoomCellExistsInEvents(t *testing.T) {
	segs, err := events.Parse(orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for i := range segs {
		for _, sc := range segs[i].Scripts {
			for p := 0; p < len(sc); {
				n := game.OpLen(sc[p])
				if sc[p] == game.OpFacility && p+1 < len(sc) && sc[p+1] == 0xFD {
					found++
				}
				if n <= 0 {
					break
				}
				p += n
			}
		}
	}
	if found == 0 {
		t.Fatal("EVENTSI 裡找不到 `0e fd`，控制室永遠走不到")
	}
	s := caveSession(t)
	s.World.RunScriptForTest([]byte{game.OpFacility, 0xFD})
	if s.World.Device != game.DeviceControlRoom {
		t.Fatalf("`0e fd` 分派到 %v，預期 DeviceControlRoom", s.World.Device)
	}
	t.Logf("`0e fd` 出現 %d 次", found)
}

// 替代字母表是 0..25 的排列，而且大小寫各自代換、非字母原樣。
func TestControlCipherIsPermutation(t *testing.T) {
	cr := game.NewControlRoom(game.NewRand(7))
	var seen [26]bool
	for _, v := range cr.Alphabet {
		if v >= 26 {
			t.Fatalf("字母表出現 %d，超出 0–25", v)
		}
		if seen[v] {
			t.Fatalf("字母表重複用了 %d，不是排列", v)
		}
		seen[v] = true
	}
	got := cr.Encode("Ab, z!")
	if len(got) != len("Ab, z!") {
		t.Fatalf("編碼後長度變了：%q", got)
	}
	if got[2:4] != ", " || got[5] != '!' {
		t.Errorf("非字母被動到了：%q", got)
	}
	if got[0] < 'A' || got[0] > 'Z' {
		t.Errorf("大寫沒有換成大寫：%q", got)
	}
	if got[1] < 'a' || got[1] > 'z' {
		t.Errorf("小寫沒有換成小寫：%q", got)
	}
}

// 每次進控制室重抽一張表 —— 兩次的答案不該總是一樣。
func TestControlCipherRerollsPerVisit(t *testing.T) {
	r := game.NewRand(7)
	a := game.NewControlRoom(r).Expect
	same := 0
	for i := 0; i < 8; i++ {
		if game.NewControlRoom(r).Expect == a {
			same++
		}
	}
	if same == 8 {
		t.Fatalf("連抽八次答案都是 %q，字母表沒有重抽", a)
	}
}

// 中止碼：`WAFE` 加上補到十格的空白才過，而且要有天選者。
func TestControlAbortGate(t *testing.T) {
	for _, tc := range []struct {
		in     string
		chosen bool
		want   game.ControlPhase
	}{
		{"WAFE", true, game.ControlCryptogram},
		{"WAFE      ", true, game.ControlCryptogram},
		// 沒有 +129 bit 5、大小寫不同、多一個字元、空的、十一格
		// （超過原版那個十格緩衝區）—— 五種都該被擋下。
		{"WAFE", false, game.ControlRejected},
		{"wafe", true, game.ControlRejected},
		{"WAFEX", true, game.ControlRejected},
		{"", true, game.ControlRejected},
		{"WAFE       ", true, game.ControlRejected},
	} {
		cr := game.NewControlRoom(game.NewRand(1))
		if got := cr.SubmitAbort(tc.in, tc.chosen); got != tc.want {
			t.Errorf("SubmitAbort(%q, %v) = %v，預期 %v", tc.in, tc.chosen, got, tc.want)
		}
	}
}

// 密碼：只認編碼後那八個字元，打錯不罰、可以一直重打。
func TestControlCodeAcceptsOnlyEncodedAnswer(t *testing.T) {
	cr := game.NewControlRoom(game.NewRand(3))
	cr.SubmitAbort("WAFE", true)
	if len(cr.Expect) != 8 {
		t.Fatalf("答案是 %q，長度 %d，預期 8", cr.Expect, len(cr.Expect))
	}
	// 明文本身不是答案（除非字母表剛好是恆等排列，那不該發生）。
	if cr.Expect == "Preamble" {
		t.Fatal("字母表是恆等排列，等於沒有加密")
	}
	if got := cr.SubmitCode("Preamble"); got != game.ControlCryptogram {
		t.Errorf("打明文竟然過了：%v", got)
	}
	if got := cr.SubmitCode(cr.Expect[:7]); got != game.ControlCryptogram {
		t.Errorf("長度不足竟然過了：%v", got)
	}
	if got := cr.SubmitCode(cr.Expect); got != game.ControlWon {
		t.Errorf("打對了卻回 %v", got)
	}
}

// 時間到就是時間到：鐘走完之後階段變成逾時，鐘面歸零。
func TestControlClockRunsOut(t *testing.T) {
	cr := game.NewControlRoom(game.NewRand(5))
	cr.SubmitAbort("WAFE", true)
	if cr.Clock() != "15:00:00" {
		t.Fatalf("起始鐘面是 %q，預期 15:00:00", cr.Clock())
	}
	for i := 0; i < 100000; i++ {
		if cr.Tick() {
			break
		}
	}
	if cr.Phase != game.ControlTimeout {
		t.Fatalf("鐘走完之後是 %v，預期 ControlTimeout", cr.Phase)
	}
	if cr.Clock() != "00:00:00" {
		t.Errorf("鐘面是 %q，預期 00:00:00", cr.Clock())
	}
	if got := cr.SubmitCode(cr.Expect); got != game.ControlTimeout {
		t.Errorf("時間到之後還收密碼：%v", got)
	}
}

// 通關獎勵：每人五千萬，旗標擋重複，分數是全隊經驗總和。
func TestControlRewardOncePerCharacter(t *testing.T) {
	s := caveSession(t)
	var before uint32
	for i := range s.Party {
		before += uint32(s.Party[i].Exp)
	}
	_, score := s.ControlRoomReward()
	var after uint32
	for i := range s.Party {
		after += uint32(s.Party[i].Exp)
	}
	want := before + uint32(len(s.Party))*50000000
	if after != want {
		t.Fatalf("發完獎之後全隊經驗 %d，預期 %d", after, want)
	}
	if score != after {
		t.Errorf("最終分數 %d 與全隊經驗 %d 不同", score, after)
	}
	// 再領一次不該加。
	_, score2 := s.ControlRoomReward()
	if score2 != score {
		t.Errorf("第二次領獎又加了：%d → %d", score, score2)
	}
}

// 勝敗場數要累積，結局那一行才不會永遠印 0。
func TestBattleCountersSurviveSave(t *testing.T) {
	s := caveSession(t)
	s.BattlesWon, s.BattlesLost = 12, 3
	st := s.State()
	s.BattlesWon, s.BattlesLost = 0, 0
	if err := s.LoadState(st); err != nil {
		t.Fatal(err)
	}
	if s.BattlesWon != 12 || s.BattlesLost != 3 {
		t.Fatalf("讀回來是 %d 勝 %d 敗，預期 12 勝 3 敗", s.BattlesWon, s.BattlesLost)
	}
}

// 密文是拿 STR.DAT 的英文明文算的，不是拿譯文 —— 中文沒有字母可以代換。
//
// 這條同時守住「原文真的被載進來了」：`Strings` 是空的時候整段密文會
// 消失，而畫面上少一段與「這一段本來就沒有」長得一模一樣。
func TestControlCipherUsesOriginalEnglish(t *testing.T) {
	s := caveSession(t)
	lines, err := text.Parse(orig(t, "STR.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s.UseStrings(lines)
	cr := game.NewControlRoom(game.NewRand(11))
	out := s.ControlCipherLines(cr)
	if len(out) == 0 {
		t.Fatal("密碼題那一頁是空的")
	}
	plain := lines[302]
	want := "  " + cr.Encode(plain)
	if !containsLine(out, want) {
		t.Fatalf("找不到第 302 行的密文 %q\n實際內容：%q", want, out)
	}
	// 原文那一份也要在，否則玩家看不出字母怎麼換。
	if !containsLine(out, "  "+plain) {
		t.Errorf("原文那一行不見了：%q", plain)
	}
}

func containsLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

// 守門的那一場是 Sheltem 加四隻元素生物，編號照 `ds:9680` 的五個位元組。
func TestControlRoomEncounterIsSheltemAndElementals(t *testing.T) {
	s := caveSession(t)
	enc := s.ControlRoomEncounter()
	if enc == nil {
		t.Fatal("擺不出守門的那一場")
	}
	want := []int{255, 225, 194, 193, 224}
	if len(enc.Monsters) != len(want) {
		t.Fatalf("有 %d 隻，預期 %d 隻", len(enc.Monsters), len(want))
	}
	for i, id := range want {
		got := enc.Monsters[i].CombatName()
		if exp := s.Bestiary[id].Name; got != exp {
			t.Errorf("第 %d 隻是 %q，預期編號 %d 的 %q", i+1, got, id, exp)
		}
	}
}
