// mm2 是可以玩的那一支：開視窗、吃鍵盤、畫畫面。
//
//	go run ./cmd/mm2 -data workplace/orig/MM2
//
// 按鍵：↑ 前進、↓ 後退、← → 轉向、Enter／空白 推進訊息與打一回合、
// Y／N 回答事件的提問、R 在旅店休息並受訓、C 施法、I 物品（裝備／卸下）、
// B 撞門、U 開鎖（先挑人）、S 搜尋、M 地圖、W 世界地圖、K／Q 查說明書、
// G 商店、N 建角色、物品選單裡 E 使用、
// 戰鬥中：Enter 攻擊、T 射擊、C 施法、A 抵擋、F 溜跑、P 防護、V 檢視、X 對調、
// F8 快速戰鬥。選單開著時方向鍵改成移游標。
//
// 功能鍵在任何畫面都是同一件事：**F1 說明、F2 設定、F4 存檔、
// F5／F6 換素材、F10 離開（先自動存檔）、Esc 一律是取消**。
//
// 唯一的例外是**有東西正等著按鍵的時候**（事件腳本停在提問、施法停在選目標、
// 正在打字、正在建角色）：那幾個畫面裡 F1–F6 一律不介入，因為那些狀態既回不去
// 也存不起來。`Esc` 取消掉提示之後就恢復。判斷在 `ui.Session.acceptsFunctionKeys`。
//
// 遊戲邏輯全部在 internal/ui，這一支只做「Ebiten ↔ ui」的綁定 ——
// 所以同一份互動流程在沒有 GPU 的環境也跑得起來（見 internal/ui 的測試）。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime"
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
	{ebiten.KeyF4, ui.KeySave},
	// 四個功能鍵是**固定的**，不隨模式改變意義：
	//
	//	F1   說明（指令一覽，與 Q／K 同一頁）
	//	F2   設定
	//	F4   存檔
	//	F10  離開遊戲（先自動存檔，在 Update 裡處理）
	//	Esc  一律是取消
	//
	// 這幾個位置與遊戲指令分開，玩家在任何畫面按下去都是同一件事 ——
	// 「同一個鍵在不同畫面做不同的事」是最容易讓人誤按的設計。
	// **「任何畫面」在 2026-08-20 之前只是文件上的說法**：F1–F6 當時只寫在
	// 探索與戰鬥兩個分支裡，選單與訊息模式一律吃掉不回應。現在由
	// `ui.Session.functionKey` 統一處理（提示中的例外見檔頭）。
	// F1 走 `ui.KeyHelp` 而不是 `ui.KeyRef`：兩者畫的是同一頁，但 F1 在任何
	// 畫面都有效，Q／K 只在探索與戰鬥有效。綁同一個語意鍵就分不出這件事。
	{ebiten.KeyF1, ui.KeyHelp},
	{ebiten.KeyF2, ui.KeySettings},
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
	// 快速戰鬥：一路打到分出結果。原版沒有這個指令 —— 它是 remake 加的
	// 便利功能，走的仍然是同一條回合結算。
	{ebiten.KeyF8, ui.KeyQuickFight},
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

// silence 關掉音樂並記一行，而不是讓遊戲收攤。
//
// 音樂包在載入時就逐首解碼驗過（`newMusicPlayer`），所以跑到這裡的錯誤是
// **裝置層**的：沒有音效卡、ALSA 開不起來、音訊裝置被別的程式佔著。
// 那種機器上遊戲照樣該能玩 —— 關掉音樂繼續跑，不要因為沒聲音就退出。
func (a *app) silence(what string, err error) {
	if a.music == nil {
		return
	}
	a.music = nil
	log.Printf("%s失敗，這一場關閉音樂：%v", what, err)
}

func (a *app) Update() error {
	a.frames++
	if a.frames%torchTicks == 0 {
		a.sess.Tick()
		a.dirty = true
	}
	if err := a.music.Sync(a.sess.MusicCue()); err != nil {
		a.silence("切換背景音樂", err)
	}
	// 一次性音效排在背景樂之後：先讓場景該有的曲子就位，再把事件音疊上去。
	if cue, ok := a.sess.Stinger(); ok {
		if err := a.music.PlayOnce(cue); err != nil {
			a.silence(fmt.Sprintf("播放音效 %s", cue), err)
		}
	}
	// **Esc 一律是取消**，任何畫面都一樣（它照常走 keymap 進 ui.KeyCancel）。
	// 離開遊戲是 F10，而且**先存檔再離開** —— 先前 Esc 兼任離開，
	// 在選單外按一下就直接關掉，進度沒了都不知道發生什麼事。
	if inpututil.IsKeyJustPressed(ebiten.KeyF10) {
		log.Printf("離開前自動存檔：%s", a.sess.Save())
		return ebiten.Termination
	}
	// 命名、事件文字輸入與控制室都直接吃字元，不能讓字母鍵被當成探索指令。
	if a.sess.Mode == ui.ModeName || a.sess.Mode == ui.ModeText || a.sess.Mode == ui.ModeControl {
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
//
// **這裡不准再 Flush 一次。** `Flush` 是把原版層整片重畫進 `Hi`，
// 而中文是 `view` 的各支 Draw 在它們自己 Flush 之後才疊上去的
// （見 `internal/view/view.go` 的 `DrawPhase`）。在這裡補一次 Flush
// 會把整層文字洗掉，畫面只剩空框，而且離屏測試因為沒有這一步而全綠。
func toEbiten(s *render.Screen) *ebiten.Image {
	return ebiten.NewImageFromImage(frameRGBA(s))
}

// frameRGBA 把高解析層抄成一張獨立的 RGBA。
//
// 與 toEbiten 分開是為了測得到：`ebiten.NewImageFromImage` 要有繪圖
// 環境才叫得動，而這條路徑真正會出錯的地方在它前面（見上面那段註解）。
func frameRGBA(s *render.Screen) *image.RGBA {
	rgba := image.NewRGBA(s.Hi.Bounds())
	b := s.Hi.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			rgba.Set(x, y, s.Hi.At(x, y))
		}
	}
	return rgba
}

// audioUsable 回答「這台機器開得起音訊裝置嗎」。
//
// **必須在 `audio.NewContext` 之前問。** Ebiten 的音訊 context 一建立就會去開
// 裝置，開不起來時錯誤是從 `RunGame` 冒出來的 —— 那時遊戲迴圈已經收攤，
// 攔不住，也沒有公開 API 事後問得到。所以改成事前看裝置在不在：沒有就
// 根本不建 context，遊戲照樣跑，只是沒聲音。
//
// Linux 以外一律當成可用 —— Windows 與 macOS 沒有對應的檔案節點，而它們的
// 音訊層在沒有裝置時本來就會靜靜地退化，不會把遊戲拖下水。
func audioUsable() bool {
	if runtime.GOOS != "linux" {
		return true
	}
	if _, err := os.Stat("/dev/snd"); err == nil {
		return true
	}
	// 沒有 /dev/snd 不代表沒有聲音：PulseAudio／PipeWire 走 socket，
	// 容器或遠端桌面常常是這一種。
	if os.Getenv("PULSE_SERVER") != "" {
		return true
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "pulse", "native")); err == nil {
			return true
		}
	}
	return false
}

// defaultMusicPacks 是沒給 `-music-pack` 時要找的位置，**依序**試。
//
// **Mega Drive 排在最前面**：四個平台裡只有它的十六首場景曲目是逐首從
// ROM 擷取、逐一對到場景的（見 docs/music.md），所以它是預設的配樂。
// 找不到就往下試，都沒有就靜音 —— 音檔是玩家自備的，缺了不是錯誤。
//
// `workplace/` 那兩個是開發樹裡的位置，`music/` 是釋出包的版面
// （見 tools/package_container.sh）。
var defaultMusicPacks = []string{
	"workplace/genesis/music/manifest.json",
	"workplace/music/manifest.json",
	"music/manifest.json",
}

// findMusicPack 找第一個存在的預設音樂包，找不到回空字串。
//
// 也看執行檔旁邊：釋出包解開之後工作目錄不見得是包的根目錄，
// 而玩家會直接雙擊執行檔。
func findMusicPack() string {
	var roots []string
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe), filepath.Dir(filepath.Dir(exe)))
	}
	roots = append(roots, ".")
	for _, root := range roots {
		for _, rel := range defaultMusicPacks {
			p := filepath.Join(root, rel)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	return ""
}

func main() {
	dataDir := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	amigaDir := flag.String("amiga-dir", "", "Amiga 素材目錄（空值沿用 workplace/amiga）")
	msxDir := flag.String("msx-dir", "", "MSX 磁片素材目錄（空值沿用 workplace/msx）")
	mdSceneDir := flag.String("md-scene-dir", "",
		"Mega Drive 場景素材目錄（空值沿用 workplace/md-scene，由 tools/mdscene.py --export 烘）")
	modernDir := flag.String("modern-dir", "", "Modern 素材包目錄（空值沿用 assets/modern、workplace/modern）")
	theme := flag.String("theme", "dos", "初始素材主題：dos、amiga、msx、megadrive、modern")
	musicPack := flag.String("music-pack", "", "本機音樂包 manifest.json（空值自動找，見 defaultMusicPacks）")
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
		MDSceneDir: *mdSceneDir,
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
	packPath := *musicPack
	if packPath == "" {
		packPath = findMusicPack()
	}
	if requestedMusicTheme != music.ThemeOff && packPath != "" && !audioUsable() {
		log.Printf("找不到可用的音訊裝置，這一場不放音樂（音樂包：%s）", packPath)
		packPath = ""
	}
	if requestedMusicTheme != music.ThemeOff && packPath != "" {
		pack, err := music.LoadManifest(packPath, requestedMusicTheme)
		if err != nil {
			log.Fatal(err)
		}
		bgm, err = newMusicPlayer(pack)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("音樂：%s（%s）", pack.Theme, packPath)
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
