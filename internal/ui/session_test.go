package ui_test

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/ui"
)

func load(t *testing.T) *ui.Session {
	t.Helper()
	dir := filepath.Join("..", "..", "workplace", "orig", "MM2")
	if _, err := os.Stat(filepath.Join(dir, "MAP.DAT")); err != nil {
		t.Skip("沒有原版資料，跳過")
	}
	// 素材路徑（中文 atlas）是相對於 repo 根目錄找的。
	wd, _ := os.Getwd()
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	s, err := ui.Load(filepath.Join("workplace", "orig", "MM2"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// 開場就要能畫出東西，而且畫的是第一人稱視角不是空畫布。
func TestSessionDrawsSomething(t *testing.T) {
	s := load(t)
	scr := s.Draw()
	scr.Flush()
	nonZero := 0
	for _, v := range scr.Orig.Pix {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero < 1000 {
		t.Errorf("畫面只有 %d 個非背景像素，看起來是空的", nonZero)
	}
}

// 轉向要真的轉，前進要真的走 —— 而且撞牆時位置不變。
func TestMovementChangesState(t *testing.T) {
	s := load(t)
	w := s.World()
	face := w.Face
	if !s.Key(ui.KeyRight) {
		t.Error("右轉沒有回報變化")
	}
	if w.Face == face {
		t.Error("右轉之後朝向沒變")
	}

	// 找一個走得動的方向：四個方向各試一次。
	moved := false
	for i := 0; i < 4 && !moved; i++ {
		x, y := w.X, w.Y
		s.Key(ui.KeyForward)
		if w.X != x || w.Y != y {
			moved = true
			break
		}
		s.Key(ui.KeyConfirm) // 撞牆會留一條訊息，推掉
		s.Key(ui.KeyRight)
	}
	if !moved {
		t.Error("四個方向都走不動")
	}
}

// 訊息模式會攔住方向鍵 —— 原版也是這樣，訊息掛著時走不動。
func TestMessageModeBlocksMovement(t *testing.T) {
	s := load(t)
	// 撞牆一定會產生訊息：先轉到面壁的方向。
	w := s.World()
	for i := 0; i < 4; i++ {
		x, y := w.X, w.Y
		s.Key(ui.KeyForward)
		if w.X == x && w.Y == y && s.Mode == ui.ModeMessage {
			break
		}
		s.Key(ui.KeyConfirm)
		s.Key(ui.KeyRight)
	}
	if s.Mode != ui.ModeMessage {
		t.Skip("這個起點四面都走得動，換一個測法")
	}
	x, y := w.X, w.Y
	if s.Key(ui.KeyForward) {
		t.Error("訊息掛著時方向鍵不該被接受")
	}
	if w.X != x || w.Y != y {
		t.Error("訊息掛著時隊伍卻移動了")
	}
	if !s.Key(ui.KeyConfirm) {
		t.Error("確認鍵沒有推進訊息")
	}
	if s.Mode != ui.ModeExplore {
		t.Errorf("推完訊息之後是 %v，預期回到探索", s.Mode)
	}
}

// 走一段路並把每一格畫出來存成 PNG。
//
// **編譯成功不是視覺測試**（CLAUDE.md §8）——這一支產生的檔案是拿來
// 肉眼比對的，測試本身只保證「每一格都畫得出來、而且畫面確實會變」。
func TestWalkFramesRender(t *testing.T) {
	s := load(t)
	out := filepath.Join("workplace", "gfx", "ui")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	keys := []ui.Key{
		ui.KeyRight, ui.KeyForward, ui.KeyConfirm,
		ui.KeyForward, ui.KeyConfirm,
		ui.KeyLeft, ui.KeyForward, ui.KeyConfirm,
		ui.KeyForward, ui.KeyConfirm,
	}
	var prev []byte
	changed := 0
	for i, k := range keys {
		s.Key(k)
		scr := s.Draw()
		scr.Flush()
		cur := make([]byte, len(scr.Orig.Pix))
		copy(cur, scr.Orig.Pix)
		if prev != nil && !equalBytes(prev, cur) {
			changed++
		}
		prev = cur

		f, err := os.Create(filepath.Join(out, framePath(i, k)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, scr.Hi); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
	if changed == 0 {
		t.Error("走了一整段畫面都沒變過")
	}
	t.Logf("輸出 %d 張到 %s，其中 %d 次畫面有變", len(keys), out, changed)
}

func framePath(i int, k ui.Key) string {
	names := map[ui.Key]string{
		ui.KeyForward: "forward", ui.KeyBack: "back",
		ui.KeyLeft: "left", ui.KeyRight: "right",
		ui.KeyConfirm: "confirm", ui.KeyYes: "yes", ui.KeyNo: "no",
	}
	n := names[k]
	if n == "" {
		n = "key"
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10)) + "-" + n + ".png"
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
