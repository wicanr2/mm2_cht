package view_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/view"
)

func testWorld(t *testing.T) *game.World {
	t.Helper()
	// 換到 repo 根，`data/` 才找得到（EnsureData 走相對路徑）。
	// 同一個測試裡呼叫兩次也要能用，所以先探當前目錄。
	if _, err := os.Stat("data"); err != nil {
		wd, _ := os.Getwd()
		if err := os.Chdir(filepath.Join("..", "..")); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chdir(wd) })
	}
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：" + err.Error())
	}
	w, err := game.NewWorld(origAt(t, "MAP.DAT"), origAt(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	// 室內／室外的標記在 ATTRIB.DAT，`NewWorld` 不會自己補。少了它所有
	// 地圖都算野外，而野外的地形層放的是地形碼不是牆的種類 ——
	// 火炬與門會整批消失。正式路徑在 game.Session 補，測試也要補。
	attrs, err := game.ParseMapAttrs(origAt(t, "ATTRIB.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range w.Maps {
		if i < len(attrs) {
			w.Maps[i].Indoor = attrs[i].Indoor()
		}
	}
	return w
}

// origAt 在換過目錄之後讀原版檔。
func origAt(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("workplace", "orig", "MM2", name))
	if err != nil {
		t.Skip("沒有原版 " + name + "，跳過")
	}
	return b
}

// testAssets 只需要字型 —— 地圖畫面不畫第一人稱視角。
func testAssets(t *testing.T) view.Assets {
	t.Helper()
	return view.Assets{}
}

// testTown 載入城鎮視角的三組素材。
func testTown(t *testing.T) *view.TownSet {
	t.Helper()
	set := func(name string) []gfx.Image {
		b, err := os.ReadFile(filepath.Join("workplace", "orig", "MM2", name))
		if err != nil {
			t.Skip("沒有原版 " + name)
		}
		im, err := gfx.ParseSet(b)
		if err != nil {
			t.Fatal(err)
		}
		return im
	}
	return view.NewTownSet(set("TOWN.16"), set("TOWNF.16"), set("TOWNT.16"), set("SKY.16"))
}

// 地圖上的朝向標記要指對邊：北在上、東在右。
func TestMapFacingMarker(t *testing.T) {
	w := testWorld(t)
	a := testAssets(t)
	w.MapIndex, w.X, w.Y = 0, 8, 8
	s := view.NewScreen()

	mark := func(f game.Facing) (x, y int) {
		w.Face = f
		view.DrawMap(s, w, a, view.MapInfo{})
		// 找出朝向標記那個顏色（12）的重心
		sx, sy, n := 0, 0, 0
		b := s.Orig.Bounds()
		for py := b.Min.Y; py < b.Max.Y; py++ {
			for px := b.Min.X; px < b.Max.X; px++ {
				if s.Orig.ColorIndexAt(px, py) == 12 {
					sx, sy, n = sx+px, sy+py, n+1
				}
			}
		}
		if n == 0 {
			t.Fatalf("朝向 %v 找不到標記", f)
		}
		return sx / n, sy / n
	}
	nx, ny := mark(game.North)
	sx, sy := mark(game.South)
	ex, ey := mark(game.East)
	wx, wy := mark(game.West)
	if ny >= sy {
		t.Errorf("北的標記沒有比南高：北 y=%d 南 y=%d", ny, sy)
	}
	if ex <= wx {
		t.Errorf("東的標記沒有比西右：東 x=%d 西 x=%d", ex, wx)
	}
	if nx != sx {
		t.Errorf("南北的標記 x 不同：%d vs %d", nx, sx)
	}
	if ey != wy {
		t.Errorf("東西的標記 y 不同：%d vs %d", ey, wy)
	}
}

// 火炬要真的在動：三張火焰互不相同，而且都畫在同一個位置。
func TestTorchAnimates(t *testing.T) {
	w := testWorld(t)
	town := testTown(t)
	if town == nil {
		t.Skip("沒有城鎮素材")
	}
	// 站在兩側都有牆的地方 —— 沒有側牆就沒有火炬，測不到東西。
	w.MapIndex = 0
	m := w.CurrentMap()
	found := false
	for c := 0; c < game.MapCells && !found; c++ {
		for f := 0; f < 4 && !found; f++ {
			x, y := c%game.MapW, c/game.MapW
			face := game.Facing(f)
			left := game.Facing((f + 3) & 3)
			right := game.Facing((f + 1) & 3)
			// 火炬不是每面牆都有（見 game.HasTorch），所以要找的是
			// 「側牆點著火炬」的格。兩側同時點著的很少，一側就夠測。
			if m.HasTorch(x, y, left) || m.HasTorch(x, y, right) {
				w.X, w.Y, w.Face = x, y, face
				found = true
			}
		}
	}
	if !found {
		t.Fatal("城鎮圖裡一面點著火炬的側牆都沒有 —— HasTorch 的條件挑錯了")
	}
	t.Logf("站在 (%d,%d) 面 %v", w.X, w.Y, w.Face)

	var frames [][]byte
	for p := 0; p < view.TorchFrames; p++ {
		s := view.NewScreen()
		s.Clear(0)
		view.DrawFirstPersonAt(s, w, town, p)
		b := make([]byte, len(s.Orig.Pix))
		copy(b, s.Orig.Pix)
		frames = append(frames, b)
	}
	same := 0
	for i := 1; i < len(frames); i++ {
		if string(frames[i]) == string(frames[0]) {
			same++
		}
	}
	if same == len(frames)-1 {
		t.Error("三個相位畫出來一模一樣，火炬沒在動")
	}
	// 差異要侷限在小範圍 —— 整片都在變表示畫錯位置了
	diff := 0
	for i := range frames[0] {
		if frames[0][i] != frames[1][i] {
			diff++
		}
	}
	if diff == 0 {
		t.Error("相位 0 與 1 完全相同")
	}
	if diff > len(frames[0])/10 {
		t.Errorf("換一張火焰動了 %d 個像素（全畫面 %d），太多了",
			diff, len(frames[0]))
	}
	t.Logf("換一張火焰動 %d 個像素", diff)
}

// 版面要照原版的四塊紅框切，而且紅框真的畫得出來。
func TestFrameMatchesOriginal(t *testing.T) {
	w := testWorld(t)
	s := view.NewScreen()
	view.DrawWith(s, w, view.Assets{}, "", nil)

	red := 0
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			if s.Orig.ColorIndexAt(x, y) == 4 {
				red++
			}
		}
	}
	if red < 3000 {
		t.Errorf("紅框只有 %d 個像素，版面沒畫出來", red)
	}
	// 量到的四條橫線位置都該是紅的
	for _, y := range []int{3, 131, 147, 187} {
		if s.Orig.ColorIndexAt(160, y) != 4 {
			t.Errorf("y=%d 那一條橫線不見了", y)
		}
	}
	// 中間那條直線只在最上面那一區
	if s.Orig.ColorIndexAt(219, 60) != 4 {
		t.Error("上半部中間的分隔線不見了")
	}
	if s.Orig.ColorIndexAt(219, 160) == 4 {
		t.Error("中間的分隔線畫到下方大框裡了 —— 那一塊是整片的")
	}
}
