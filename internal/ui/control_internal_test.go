package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 結局控制室的正常玩家路徑：踩上去 → 打贏守門那一場 → 打中止碼 →
// 照畫面附的密碼打進去 → 看到結算。
//
// **不用傳送、不發道具、不強制勝利**以外的捷徑只有一個：戰鬥直接判定
// 隊伍獲勝（把怪物打到 0），因為 Sheltem 那一場要打很多回合，
// 而這條測試要驗的是戰鬥「之後」的那條鏈。
func TestControlRoomFullPath(t *testing.T) {
	s := loadUI(t)
	if !s.openDevice(game.DeviceControlRoom) {
		t.Fatal("踩到 `0e fd` 沒有開出任何畫面")
	}
	if s.Mode != ModeCombat {
		t.Fatalf("進控制室之前應該先開打，實際是 %v", s.Mode)
	}
	enc := s.Game.Fight
	if enc == nil {
		t.Fatal("沒有擺出守門的那一場")
	}
	if len(enc.Monsters) != 5 {
		t.Fatalf("守門的有 %d 隻，預期 5 隻（Sheltem ＋ 四元素）", len(enc.Monsters))
	}
	// 名字這裡已經是譯名了，所以編號的驗收放在 game 那一層
	// （TestControlRoomEncounterIsSheltemAndElementals）。
	if got := enc.Monsters[0].CombatName(); got == "" {
		t.Error("第一隻沒有名字")
	}

	// 打完那一場：把守門的全部打到 0，剩下的收尾走正常那條路。
	for i := range enc.Monsters {
		enc.Monsters[i].TakeDamage(enc.Monsters[i].CombatHP())
	}
	if !s.fightRound() {
		t.Fatal("戰鬥沒有收尾")
	}
	if s.Mode != ModeControl {
		t.Fatalf("打贏之後是 %v，預期進控制室", s.Mode)
	}

	// 守門旁白 → 中止碼。
	s.Key(KeyConfirm)
	if s.controlStage != stageAbort {
		t.Fatalf("翻過旁白之後是 %v，預期等中止碼", s.controlStage)
	}
	for _, r := range "WAFE" {
		if !s.TypeRune(r) {
			t.Fatalf("打不進 %q", r)
		}
	}
	// 沒有天選者旗標時會被請出去；先給第一個人 `+129` bit 5。
	s.Game.Party[0].SetFieldByte(129, 0xFF, 0x20)
	s.Key(KeyConfirm)
	if s.controlStage != stageBrief {
		t.Fatalf("中止碼過了之後是 %v，預期看預錄訊息", s.controlStage)
	}
	s.Key(KeyConfirm)
	if s.controlStage != stageCipher {
		t.Fatalf("翻過訊息之後是 %v，預期密碼題", s.controlStage)
	}

	// 畫面上要同時看得到密文、原文與譯文，而且提示裡附著答案。
	page := s.controlPage()
	want := s.control.Expect
	if !strings.Contains(page.Prompt, want) {
		t.Fatalf("提示 %q 裡沒有附上答案 %q", page.Prompt, want)
	}
	body := strings.Join(page.Lines, "\n")
	for _, k := range []string{"密文", "原文", "譯文"} {
		if !strings.Contains(body, k) {
			t.Errorf("密碼題那一頁少了「%s」那一段", k)
		}
	}
	if page.Clock == "" {
		t.Error("密碼題沒有顯示倒數")
	}

	// 照提示打進去。
	for _, r := range want {
		if !s.TypeRune(r) {
			t.Fatalf("打不進 %q", r)
		}
	}
	if s.controlInput != want {
		t.Fatalf("輸入欄是 %q，預期 %q", s.controlInput, want)
	}
	s.Key(KeyConfirm)
	if s.controlStage != stageWin {
		t.Fatalf("打對之後是 %v，預期通關", s.controlStage)
	}
	if s.controlScore == 0 {
		t.Error("最終分數是 0")
	}
	s.Key(KeyConfirm)
	if s.controlStage != stageScore {
		t.Fatalf("翻過賀詞之後是 %v，預期結算", s.controlStage)
	}
	score := s.controlPage()
	if !strings.Contains(strings.Join(score.Lines, "\n"), "最終分數") {
		t.Error("結算那一頁沒有印分數")
	}
	s.Key(KeyConfirm)
	if s.Mode == ModeControl {
		t.Error("結算翻完之後還留在控制室")
	}
	// 獎勵要真的發下去。
	for i := range s.Game.Party {
		if s.Game.Party[i].FieldByte(129)&0x08 == 0 {
			t.Errorf("第 %d 個人沒有拿到 +129 bit 3", i+1)
		}
	}
}

// 中止碼打錯就被請出去，而且不會偷偷放行到密碼題。
func TestControlRoomWrongAbortCode(t *testing.T) {
	s := loadUI(t)
	s.openControlRoom()
	s.Key(KeyConfirm) // 翻過旁白
	for _, r := range "WAFF" {
		s.TypeRune(r)
	}
	s.Key(KeyConfirm)
	if s.controlStage != stageOut {
		t.Fatalf("打錯之後是 %v，預期被請出去", s.controlStage)
	}
	s.Key(KeyConfirm)
	if s.Mode == ModeControl {
		t.Error("被請出去之後還留在控制室")
	}
}

// 密碼題的鐘走完就結束，不會卡在輸入畫面。
func TestControlRoomClockEndsTheRun(t *testing.T) {
	s := loadUI(t)
	s.openControlRoom()
	s.Key(KeyConfirm)
	s.Game.Party[0].SetFieldByte(129, 0xFF, 0x20)
	for _, r := range "WAFE" {
		s.TypeRune(r)
	}
	s.Key(KeyConfirm)
	s.Key(KeyConfirm) // 翻過預錄訊息
	if s.controlStage != stageCipher {
		t.Fatalf("沒走到密碼題，停在 %v", s.controlStage)
	}
	for i := 0; i < 100000 && s.controlStage == stageCipher; i++ {
		s.Tick()
	}
	if s.controlStage != stageOut {
		t.Fatalf("鐘走完之後是 %v，預期結束", s.controlStage)
	}
}

// 中止碼十格、密碼八格，超過就打不進去（原版兩個緩衝區的長度）。
func TestControlRoomInputWidths(t *testing.T) {
	s := loadUI(t)
	s.openControlRoom()
	s.Key(KeyConfirm)
	for i := 0; i < 20; i++ {
		s.TypeRune('A')
	}
	if len(s.controlInput) != game.ControlAbortWidth {
		t.Errorf("中止碼欄收了 %d 個字，預期 %d", len(s.controlInput), game.ControlAbortWidth)
	}
	s.controlStage = stageCipher
	s.controlInput = ""
	for i := 0; i < 20; i++ {
		s.TypeRune('A')
	}
	if len(s.controlInput) != controlInputMax {
		t.Errorf("密碼欄收了 %d 個字，預期 %d", len(s.controlInput), controlInputMax)
	}
	// 中文打不進去 —— 兩個答案都是拉丁字母。
	s.controlInput = ""
	if s.TypeRune('光') {
		t.Error("中文竟然打得進去")
	}
}

// 留下每一頁的畫面。**編譯成功不是視覺測試** —— 控制室的版面是自己排的，
// 行數、輸入列與倒數擠在一起的樣子只有看圖才知道。
func TestControlRoomScreenshots(t *testing.T) {
	s := loadUI(t)
	s.Game.Party[0].SetFieldByte(129, 0xFF, 0x20)
	s.openControlRoom()
	out := filepath.Join("workplace", "gfx", "ui")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	shot := func(name string) {
		f, err := os.Create(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, s.Draw().Hi); err != nil {
			t.Fatal(err)
		}
	}
	shot("control-guard.png")
	s.Key(KeyConfirm)
	shot("control-abort.png")
	for _, r := range "WAFE" {
		s.TypeRune(r)
	}
	s.Key(KeyConfirm)
	shot("control-brief.png")
	s.Key(KeyConfirm)
	shot("control-cipher.png")
	for _, r := range s.control.Expect {
		s.TypeRune(r)
	}
	s.Key(KeyConfirm)
	shot("control-win.png")
	s.Key(KeyConfirm)
	shot("control-score.png")
	t.Logf("答案 %q，鐘面 %s", s.control.Expect, s.control.Clock())
}

// 全滅不是死路：按 Enter 要回到最後投宿的旅店。
//
// 這條守的是一個真的會卡住玩家的洞 —— 在此之前 ModeDead 把所有按鍵
// 都吃掉，全滅之後畫面就不動了。
func TestDeadReturnsToLastInn(t *testing.T) {
	s := loadUI(t)
	// 先在 Sansobar（城 4）登記入住，再走遠。
	s.Game.World.MapIndex = 4
	s.Game.CheckInAtInn()
	s.Game.World.MapIndex, s.Game.World.X, s.Game.World.Y = 11, 1, 1

	page := s.deadPage()
	if len(page.Lines) != 10 {
		t.Fatalf("全滅那一頁有 %d 行，預期 10 行", len(page.Lines))
	}
	if page.Lines[0] == "" {
		t.Error("第一行是空的（`Death Strikes!` 那一行沒接上譯文）")
	}

	s.Mode = ModeDead
	if s.Key(KeyLeft) {
		t.Error("全滅時方向鍵不該有作用")
	}
	if !s.Key(KeyConfirm) {
		t.Fatal("全滅時按 Enter 沒有反應 —— 玩家會卡死")
	}
	w := s.Game.World
	want := game.TownStart[4]
	if w.MapIndex != 4 || w.X != want.X || w.Y != want.Y || w.Face != want.Face {
		t.Fatalf("回到 (圖%d, %d, %d) 面 %v，預期 (圖4, %d, %d) 面 %v",
			w.MapIndex, w.X, w.Y, w.Face, want.X, want.Y, want.Face)
	}
	if s.Mode == ModeDead {
		t.Error("按完 Enter 還停在全滅畫面")
	}
}

// 登記入住要寫進存檔，否則讀檔之後全滅會回錯地方。
func TestLastInnSurvivesSave(t *testing.T) {
	s := loadUI(t)
	s.Game.World.MapIndex = 3
	s.Game.CheckInAtInn()
	if s.Game.LastInn != 3 {
		t.Fatalf("登記入住之後 LastInn 是 %d，預期 3", s.Game.LastInn)
	}
	// 整隊的記錄 +11 也要跟著寫（原版 `loc_1C1BD`）。
	for i := range s.Game.Party {
		if got := s.Game.Party[i].FieldByte(0x0B) & 0x7F; got != 4 {
			t.Errorf("第 %d 人的 +11 是 %d，預期 4（城 3 + 1）", i+1, got)
		}
	}
	st := s.Game.State()
	s.Game.LastInn = 0
	if err := s.Game.LoadState(st); err != nil {
		t.Fatal(err)
	}
	if s.Game.LastInn != 3 {
		t.Fatalf("讀檔之後 LastInn 是 %d，預期 3", s.Game.LastInn)
	}
}
