// mm2 是可以玩的那一支：開視窗、吃鍵盤、畫畫面。
//
//	go run ./cmd/mm2 -data workplace/orig/MM2
//
// 按鍵：↑ 前進、↓ 後退、← → 轉向、Enter／空白 推進訊息與打一回合、
// Y／N 回答事件的提問、R 在旅店休息並受訓、C 施法、I 物品（裝備／卸下）、
// B 撞門、U 開鎖（先挑人）、S 搜尋、M 地圖、W 世界地圖、K／Q 查說明書、
// G 商店、N 建角色、F2 存檔、物品選單裡 E 使用、
// 戰鬥中：Enter 攻擊、T 射擊、C 施法、A 抵擋、F 溜跑、P 防護、V 檢視、X 對調、
// F5／F6 換素材、Esc 離開（選單裡是取消）。選單開著時方向鍵改成移游標。
//
// 遊戲邏輯全部在 internal/ui，這一支只做「Ebiten ↔ ui」的綁定 ——
// 所以同一份互動流程在沒有 GPU 的環境也跑得起來（見 internal/ui 的測試）。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/mm2_cht/internal/music"
	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

type app struct {
	sess   *ui.Session
	music  *musicPlayer
	frame  *ebiten.Image
	dirty  bool
	frames int
}

// keymap 把實體按鍵對到互動層的按鍵。順序決定同時按下時誰先。
var keymap = []struct {
	key ebiten.Key
	act ui.Key
}{
	{ebiten.KeyEnter, ui.KeyConfirm},
	{ebiten.KeySpace, ui.KeyConfirm},
	{ebiten.KeyEscape, ui.KeyCancel},
	{ebiten.KeyY, ui.KeyYes},
	{ebiten.KeyN, ui.KeyNo},
	{ebiten.KeyR, ui.KeyRest},
	{ebiten.KeyC, ui.KeyCast},
	{ebiten.KeyI, ui.KeyItems},
	// B 撞門、Q 快速檢視都照原版（`2PLAY` 分派 `0x18166`／`0x181C8`）。
	// 商店是 remake 才有的入口，放在原版沒用到的 G。
	{ebiten.KeyB, ui.KeyBash},
	{ebiten.KeyG, ui.KeyShop},
	{ebiten.KeyK, ui.KeyRef},
	{ebiten.KeyQ, ui.KeyRef},
	{ebiten.KeyU, ui.KeyUnlock},
	// S 照原版給搜尋（`2PLAY` 分派 `0x181E8`）。存檔挪到功能鍵：原版的存檔
	// 藏在 `O` 指令視窗裡，本來就不佔字母鍵。
	{ebiten.KeyS, ui.KeySearch},
	{ebiten.KeyF2, ui.KeySave},
	{ebiten.KeyF, ui.KeyRun},
	{ebiten.KeyA, ui.KeyBlock},
	{ebiten.KeyT, ui.KeyShoot},
	{ebiten.KeyE, ui.KeyUse},
	{ebiten.KeyP, ui.KeyProt},
	{ebiten.KeyV, ui.KeyView},
	{ebiten.KeyX, ui.KeyExch},
	{ebiten.KeyM, ui.KeyMap},
	// W 是 remake 加的世界地圖頁（原版沒用到 W）。
	{ebiten.KeyW, ui.KeyWorld},
	// F3 與 M 同一件事。地圖畫面同時顯示這張圖的攻略提示，
	// 而 M 已經被「地圖」佔著，多給一個功能鍵讓它好記。
	{ebiten.KeyF3, ui.KeyMap},
	// F5 切換牆面的放大方式。放在功能鍵而不是字母鍵：字母鍵是遊戲指令，
	// 這一個是顯示設定，兩者混在一起會搶掉未來的指令字母。
	{ebiten.KeyF5, ui.KeyStyle},
	{ebiten.KeyF6, ui.KeyPlatform},
	{ebiten.KeyN, ui.KeyCreate},
}

// 方向鍵在探索與選單下是兩件事：走路 vs 移游標。
// 對應由這裡分，`internal/ui` 收到的是已經分好的語意鍵。
// digits 是 1–9，索引 0 對應 '1'。
var digits = []ebiten.Key{
	ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4,
	ebiten.KeyDigit5, ebiten.KeyDigit6, ebiten.KeyDigit7, ebiten.KeyDigit8,
	ebiten.KeyDigit9,
}

var arrows = []struct {
	key            ebiten.Key
	walk, navigate ui.Key
}{
	{ebiten.KeyArrowUp, ui.KeyForward, ui.KeyUp},
	{ebiten.KeyArrowDown, ui.KeyBack, ui.KeyDown},
	{ebiten.KeyArrowLeft, ui.KeyLeft, ui.KeyUp},
	{ebiten.KeyArrowRight, ui.KeyRight, ui.KeyDown},
}

// torchTicks 是火炬換一張要幾個更新影格。Ebiten 預設 60 fps，
// 8 影格約 7.5 fps —— 原版的火焰是慢慢跳的，不是抖動。
const torchTicks = 8

func (a *app) Update() error {
	a.frames++
	if a.frames%torchTicks == 0 {
		a.sess.Tick()
		a.dirty = true
	}
	if err := a.music.Sync(a.sess.MusicCue()); err != nil {
		return fmt.Errorf("切換背景音樂：%w", err)
	}
	// 一次性音效排在背景樂之後：先讓場景該有的曲子就位，再把事件音疊上去。
	if cue, ok := a.sess.Stinger(); ok {
		if err := a.music.PlayOnce(cue); err != nil {
			return fmt.Errorf("播放音效 %s：%w", cue, err)
		}
	}
	// Esc 在選單裡是「取消」，不在選單裡才是「離開遊戲」。
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && a.sess.Mode != ui.ModeMenu && a.sess.Mode != ui.ModeText {
		return ebiten.Termination
	}
	// 命名與事件文字輸入都直接吃字元，不能讓字母鍵被當成探索指令。
	if a.sess.Mode == ui.ModeName || a.sess.Mode == ui.ModeText {
		for _, r := range ebiten.AppendInputChars(nil) {
			if a.sess.TypeRune(r) {
				a.dirty = true
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && a.sess.TypeRune('\b') {
			a.dirty = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && a.sess.Key(ui.KeyConfirm) {
			a.dirty = true
		}
		return nil
	}
	for i, k := range digits {
		if inpututil.IsKeyJustPressed(k) {
			if a.sess.PressDigit(i + 1) {
				a.dirty = true
			}
			return nil
		}
	}
	inMenu := a.sess.Mode == ui.ModeMenu || a.sess.Mode == ui.ModeCreate
	for _, m := range arrows {
		if inpututil.IsKeyJustPressed(m.key) {
			act := m.walk
			if inMenu {
				act = m.navigate
			}
			if a.sess.Key(act) {
				a.dirty = true
			}
			return nil
		}
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

// Draw 把畫面等比例放大到視窗大小，置中、四周留黑邊。
//
// **兩層一起縮放**：中文不是另外畫在視窗上的，是先疊進同一張
// `render.Screen.Hi` 再整張送出去 —— 所以縮放時中文與原版像素
// 不可能各縮各的、也不會位移。這是把疊加層做在畫布裡而不是
// 做在視窗上的直接好處。
func (a *app) Draw(dst *ebiten.Image) {
	if a.dirty || a.frame == nil {
		a.frame = toEbiten(a.sess.Draw())
		a.dirty = false
	}
	scale, ox, oy := render.Fit(dst.Bounds().Dx(), dst.Bounds().Dy())
	op := &ebiten.DrawImageOptions{}
	// 最近鄰取樣：像素風放大不能內插，否則整片糊掉。
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(ox, oy)
	dst.DrawImage(a.frame, op)
}

// Layout 回傳視窗實際大小，縮放交給 Draw 自己算 ——
// 固定 Layout 會讓 Ebiten 自己拉伸，長寬比與視窗不同時會變形。
func (a *app) Layout(outsideW, outsideH int) (int, int) {
	if outsideW < 1 {
		outsideW = render.HiW
	}
	if outsideH < 1 {
		outsideH = render.HiH
	}
	return outsideW, outsideH
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
	amigaDir := flag.String("amiga-dir", "", "Amiga 素材目錄（空值沿用 workplace/amiga）")
	msxDir := flag.String("msx-dir", "", "MSX 磁片素材目錄（空值沿用 workplace/msx）")
	modernDir := flag.String("modern-dir", "", "Modern 素材包目錄（空值沿用 assets/modern、workplace/modern）")
	theme := flag.String("theme", "dos", "初始素材主題：dos、amiga、msx、modern")
	musicPack := flag.String("music-pack", "", "本機音樂包 manifest.json（原版音檔不得進版控）")
	musicTheme := flag.String("music-theme", "", "音樂主題：megadrive、msx、amiga、dos、off（空值採 manifest）")
	flag.Parse()
	requestedMusicTheme := music.Theme(strings.ToLower(strings.TrimSpace(*musicTheme)))
	switch requestedMusicTheme {
	case "", music.ThemeMegaDrive, music.ThemeMSX, music.ThemeAmiga, music.ThemeDOS, music.ThemeOff:
	default:
		log.Fatalf("未知音樂主題 %q", *musicTheme)
	}

	var modernDirs []string
	if *modernDir != "" {
		modernDirs = []string{*modernDir}
	}
	sess, err := ui.LoadWithOptions(*dataDir, ui.LoadOptions{
		AmigaDir:   *amigaDir,
		MSXDir:     *msxDir,
		ModernDirs: modernDirs,
		Theme:      *theme,
	})
	if err != nil {
		log.Fatal(err)
	}
	if sess.Restore() {
		log.Printf("已接續 %s", ui.SavePath)
	}
	// 開起來先看片頭，按任意鍵進遊戲 —— 原版也是這樣。
	sess.ShowIntro()
	var bgm *musicPlayer
	if requestedMusicTheme != music.ThemeOff && *musicPack != "" {
		pack, err := music.LoadManifest(*musicPack, requestedMusicTheme)
		if err != nil {
			log.Fatal(err)
		}
		bgm, err = newMusicPlayer(pack)
		if err != nil {
			log.Fatal(err)
		}
	}
	// 視窗尺寸直接用高解析層的大小。**不要在這裡再乘一次倍率** ——
	// `render.Scale` 已經把原版的 320×200 放大過了，外面再乘會讓視窗
	// 超出螢幕、邊緣被裁掉，而且中文那一層會被二次縮放糊掉。
	// 開起來就是一比一；之後拉大拉小都會等比例縮放（見 app.Draw）。
	ebiten.SetWindowSize(render.HiW, render.HiH)
	ebiten.SetWindowTitle("魔法門 II：異界之門")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(&app{sess: sess, music: bgm, dirty: true}); err != nil {
		log.Fatal(err)
	}
}
