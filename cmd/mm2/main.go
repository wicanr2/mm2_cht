// mm2 是可以玩的那一支：開視窗、吃鍵盤、畫畫面。
//
//	go run ./cmd/mm2 -data workplace/orig/MM2
//
// 按鍵：↑ 前進、↓ 後退、← → 轉向、Enter／空白 推進訊息與打一回合、
// Y／N 回答事件的提問、R 在旅店休息並受訓、Esc 離開。
//
// 遊戲邏輯全部在 internal/ui，這一支只做「Ebiten ↔ ui」的綁定 ——
// 所以同一份互動流程在沒有 GPU 的環境也跑得起來（見 internal/ui 的測試）。
package main

import (
	"flag"
	"image"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

type app struct {
	sess  *ui.Session
	frame *ebiten.Image
	dirty bool
}

// keymap 把實體按鍵對到互動層的按鍵。順序決定同時按下時誰先。
var keymap = []struct {
	key ebiten.Key
	act ui.Key
}{
	{ebiten.KeyArrowUp, ui.KeyForward},
	{ebiten.KeyArrowDown, ui.KeyBack},
	{ebiten.KeyArrowLeft, ui.KeyLeft},
	{ebiten.KeyArrowRight, ui.KeyRight},
	{ebiten.KeyEnter, ui.KeyConfirm},
	{ebiten.KeySpace, ui.KeyConfirm},
	{ebiten.KeyY, ui.KeyYes},
	{ebiten.KeyN, ui.KeyNo},
	{ebiten.KeyR, ui.KeyRest},
}

func (a *app) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	for _, m := range keymap {
		if inpututil.IsKeyJustPressed(m.key) {
			if a.sess.Key(m.act) {
				a.dirty = true
			}
			break
		}
	}
	return nil
}

func (a *app) Draw(dst *ebiten.Image) {
	if a.dirty || a.frame == nil {
		a.frame = toEbiten(a.sess.Draw())
		a.dirty = false
	}
	dst.DrawImage(a.frame, nil)
}

func (a *app) Layout(int, int) (int, int) {
	return render.HiW, render.HiH
}

// toEbiten 把高解析畫布轉成 Ebiten 的圖。
//
// 走的是 `Screen.Hi`（已經把原版像素放大、中文疊上去的那一層），
// 所以中文不會被再縮放一次糊掉。
func toEbiten(s *render.Screen) *ebiten.Image {
	s.Flush()
	rgba := image.NewRGBA(s.Hi.Bounds())
	b := s.Hi.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			rgba.Set(x, y, s.Hi.At(x, y))
		}
	}
	return ebiten.NewImageFromImage(rgba)
}

func main() {
	dataDir := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	flag.Parse()

	sess, err := ui.Load(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	// 視窗尺寸直接用高解析層的大小。**不要在這裡再乘一次倍率** ——
	// `render.Scale` 已經把原版的 320×200 放大過了，外面再乘會讓視窗
	// 超出螢幕、邊緣被裁掉，而且中文那一層會被二次縮放糊掉。
	ebiten.SetWindowSize(render.HiW, render.HiH)
	ebiten.SetWindowTitle("魔法門 II：異界之門")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(&app{sess: sess, dirty: true}); err != nil {
		log.Fatal(err)
	}
}
