package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// loadUI 開一場遊玩，沒有原版資料就跳過。與 session_test.go 的 load 同一件事，
// 但那支在 ui_test 套件裡，內部測試用不到。
func loadUI(t *testing.T) *Session {
	t.Helper()
	const data = "workplace/orig/MM2"
	if _, err := os.Stat(filepath.Join(data, "MAP.DAT")); err != nil {
		if _, err := os.Stat(filepath.Join("..", "..", data, "MAP.DAT")); err != nil {
			t.Skip("沒有原版資料，跳過")
		}
		wd, _ := os.Getwd()
		if err := os.Chdir(filepath.Join("..", "..")); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chdir(wd) })
	}
	s, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// 年代之門開得了門才給選單；選完要真的把隊伍送走。
func TestDeviceEraGateMenu(t *testing.T) {
	s := loadUI(t)

	// 沒有旗標：只回一句話，不開選單。
	if !s.openDevice(game.DeviceEraGate) {
		t.Fatal("年代之門沒有畫面")
	}
	if s.Mode == ModeMenu {
		t.Error("開不了的門不該給選單")
	}

	s.Game.Party[0].SetFieldByte(128, 0xFF, 0x02)
	s.Lines = nil
	if !s.openDevice(game.DeviceEraGate) || s.Mode != ModeMenu {
		t.Fatalf("有旗標時應該開選單，模式是 %v", s.Mode)
	}
	if got := len(s.Menu.Items); got != 9 {
		t.Errorf("選單有 %d 項，預期 8 個年代加離開", got)
	}
	s.Menu.Cur = 5 // 第 6 個年代 → 地圖 37 (5,5)
	s.choose()
	if s.Game.World.MapIndex != 37 {
		t.Errorf("選完地圖是 %d，預期 37", s.Game.World.MapIndex)
	}
	if s.Mode == ModeMenu {
		t.Error("選完之後選單還開著")
	}
}

// 捐獻選單挑人之後要真的換到經驗。
func TestDeviceDonateMenu(t *testing.T) {
	s := loadUI(t)
	c := &s.Game.Party[0]
	c.Gems, c.Exp = 5, 0

	if !s.openDevice(game.DeviceGemExp) || s.Mode != ModeMenu {
		t.Fatalf("捐寶石應該開選單，模式是 %v", s.Mode)
	}
	s.Menu.Cur = 0
	s.choose()
	if c.Exp != 50 || c.Gems != 0 {
		t.Errorf("捐 5 顆之後 Exp=%d Gems=%d，預期 50／0", c.Exp, c.Gems)
	}
}

// 座標傳送機收兩個數字，格式不對就留在輸入畫面。
func TestDeviceMagicLocationInput(t *testing.T) {
	s := loadUI(t)
	s.Game.World.MapIndex = 2
	if !s.openDevice(game.DeviceTeleport) || s.Mode != ModeText {
		t.Fatalf("座標傳送機應該進文字輸入，模式是 %v", s.Mode)
	}

	s.PromptText = "十三"
	if s.Key(KeyConfirm) {
		t.Error("看不懂的輸入不該被接受")
	}
	s.PromptText = "9 12"
	if !s.Key(KeyConfirm) {
		t.Fatal("`9 12` 應該被接受")
	}
	w := s.Game.World
	if w.X != 9 || w.Y != 12 {
		t.Errorf("座標是 (%d,%d)，預期 (9,12)", w.X, w.Y)
	}
	if w.MapIndex != 2 {
		t.Errorf("不該換地圖，卻變成 %d", w.MapIndex)
	}
	if !strings.Contains(strings.Join(s.Lines, "\n"), "9") {
		t.Errorf("沒有回報新座標：%v", s.Lines)
	}
}

// 兩位領主的任務走完整條路：接任務 → 打死目標 → 回來結算。
func TestDeviceQuestFlow(t *testing.T) {
	s := loadUI(t)
	if !s.openDevice(game.DeviceSlayer) || s.Mode != ModeMenu {
		t.Fatalf("斯萊爾領主應該開選單，模式是 %v", s.Mode)
	}
	if got := len(s.Menu.Items); got != 5 {
		t.Errorf("選單有 %d 項，預期四個難度加離開", got)
	}
	s.Menu.Cur = 0 // 侍童任務
	s.choose()
	target, lord := game.QuestTarget(&s.Game.Party[0])
	if target == 0 || lord != game.LordSlayer {
		t.Fatalf("沒有接到任務：目標 %d、委託人 %v", target, lord)
	}

	// 再踩一次應該回「已經接下了」，不是又給一個新的。
	s.Lines = nil
	s.openDevice(game.DeviceSlayer)
	if s.Mode == ModeMenu {
		t.Error("任務中卻又給了選單")
	}
	if got, _ := game.QuestTarget(&s.Game.Party[0]); got != target {
		t.Errorf("目標被換掉了：%d → %d", target, got)
	}

	s.Game.MarkQuestKillForTest(target)
	s.Lines = nil
	s.openDevice(game.DeviceSlayer)
	if len(s.Lines) == 0 {
		t.Fatal("打死目標之後應該結算")
	}
	if got, _ := game.QuestTarget(&s.Game.Party[0]); got != 0 {
		t.Errorf("結算後目標沒清掉：%d", got)
	}
}

// 競技賽的入口是 `0e 08` 那一格，不是酒館選單。
//
// 原版的酒館（`2BRAIN` 的 `_2brain_e02`）五個選單項裡沒有競技賽 ——
// 先前 remake 把它掛在酒館下面，那是發明出來的玩法。
func TestArenaIsNotInTavernMenu(t *testing.T) {
	items := tavernMenu().Items
	for _, it := range items {
		if strings.Contains(it, "競技") {
			t.Errorf("酒館選單裡還有競技賽：%v", items)
		}
	}
	if got := game.FacilityByCode(8); got != game.FacilityArena {
		t.Errorf("`0e 08` 是 %v，預期競技場", got)
	}
}
