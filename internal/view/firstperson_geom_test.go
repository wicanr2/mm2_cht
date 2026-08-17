package view_test

import (
	"encoding/binary"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/exetext"
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// townWalls 解出 `TOWN.16` 的 32 張牆。
func townWalls(t *testing.T) []gfx.Image {
	t.Helper()
	chdirRoot(t)
	im, err := gfx.ParseSet(origAt(t, "TOWN.16"))
	if err != nil {
		t.Fatalf("TOWN.16 解不開：%v", err)
	}
	return im
}

// 每一張牆都要落在視圖裡。
func TestIndoorPiecesStayInsideView(t *testing.T) {
	walls := townWalls(t)
	for _, g := range view.IndoorGeometry() {
		if g.Frame < 0 || g.Frame >= len(walls) {
			t.Fatalf("%s 用影格 %d，`TOWN.16` 只有 %d 張", g.What, g.Frame, len(walls))
		}
		im := walls[g.Frame]
		if g.X < view.FPX || g.X+im.Width > view.FPX+view.FPW {
			t.Errorf("%s 影格 %d：x %d 寬 %d → %d，視圖是 %d–%d",
				g.What, g.Frame, g.X, im.Width, g.X+im.Width, view.FPX, view.FPX+view.FPW)
		}
		if g.Y < view.FPY || g.Y+im.Height > view.FPY+view.FPH {
			t.Errorf("%s 影格 %d：y %d 高 %d → %d，視圖是 %d–%d",
				g.What, g.Frame, g.Y, im.Height, g.Y+im.Height, view.FPY, view.FPY+view.FPH)
		}
	}
}

// 同一個深度的「左側牆 ＋ 正牆 ＋ 右側牆」要**剛好鋪滿視圖寬**。
//
// 這是那幾張落點表最強的結構性條件：三塊接縫對不上就會露出一條後面的
// 天空或地板，而**那條縫只有一兩個像素寬** —— 畫面上看起來像素材有雜點，
// 不像座標表抄錯。正牆之所以能整塊不透空地蓋上去，靠的也是這個條件。
func TestIndoorRowTilesTheView(t *testing.T) {
	walls := townWalls(t)
	g := view.IndoorGeometry()
	pick := func(what string, d int) view.IndoorPiece { return g[d*5+slotOf(what)] }
	for d := 0; d < view.Depth; d++ {
		left, front, right := pick("左側牆", d), pick("正牆", d), pick("右側牆", d)
		if got := left.X + walls[left.Frame].Width; got != front.X {
			t.Errorf("深度 %d：左側牆右緣 %d，正牆左緣 %d", d, got, front.X)
		}
		if got := front.X + walls[front.Frame].Width; got != right.X {
			t.Errorf("深度 %d：正牆右緣 %d，右側牆左緣 %d", d, got, right.X)
		}
		// 整列以視圖中線左右對稱：左緣 ＋ 右緣 ＝ 兩倍中線。
		// 深度 0 那一列剛好鋪滿視圖，更深的往中間縮。
		mid := 2*view.FPX + view.FPW
		if got := left.X + right.X + walls[right.Frame].Width; got != mid {
			t.Errorf("深度 %d：左緣 %d ＋ 右緣 %d ＝ %d，對稱的話該是 %d",
				d, left.X, right.X+walls[right.Frame].Width, got, mid)
		}
	}
}

// 縱列牆與正牆的接縫：左邊那一片的右緣要等於正牆的左緣，
// 右邊那一片的左緣要等於右側牆的左緣（原版四個深度都是同一個 x）。
func TestIndoorColumnsMeetTheFrontWall(t *testing.T) {
	walls := townWalls(t)
	g := view.IndoorGeometry()
	pick := func(what string, d int) view.IndoorPiece { return g[d*5+slotOf(what)] }
	for d := 0; d < view.Depth; d++ {
		lc, front := pick("左縱列牆", d), pick("正牆", d)
		if got := lc.X + walls[lc.Frame].Width; got != front.X {
			t.Errorf("深度 %d：左縱列牆右緣 %d，正牆左緣 %d", d, got, front.X)
		}
		if rc, right := pick("右縱列牆", d), pick("右側牆", d); rc.X != right.X {
			t.Errorf("深度 %d：右縱列牆 x %d，右側牆 x %d —— 原版兩者同一個 x",
				d, rc.X, right.X)
		}
	}
}

// 落點表要與 `MM2.EXE` 裡的那幾張 DGROUP 表逐格相同。
//
// **這是唯一擋得住「垂直置中」那類公式的檢查。** 落點只要差 1 px，畫面
// 照樣像對的，前一次是靠 DOSBox 逐像素比才發現（側牆深度 1 與正牆深度 3
// 各差一格）。EXE 是一手資料，比對它不必等一分鐘的實機驗證。
//
// 位址取自 `docs/formats/04-graphics.md`：`sub_18558`／`sub_185B4`／
// `sub_1867C` 查的就是這幾張，`EXE 偏移 = DGROUP 偏移 + 0x8630`。
func TestIndoorTablesMatchTheEXE(t *testing.T) {
	chdirRoot(t)
	exe := origAt(t, "MM2.EXE")
	if len(exe) < exetext.DGroupBase+0x2000 {
		t.Skip("MM2.EXE 太小，沒有 DGROUP 初值段")
	}
	read := func(dgroup int) [view.Depth]int {
		var out [view.Depth]int
		for d := 0; d < view.Depth; d++ {
			off := exetext.DGroupBase + dgroup + d*2
			out[d] = int(int16(binary.LittleEndian.Uint16(exe[off : off+2])))
		}
		return out
	}
	g := view.IndoorGeometry()
	pick := func(what string, d int) view.IndoorPiece { return g[d*5+slotOf(what)] }

	// 六張表連在一起，左半 0x153E–0x1571、右半 0x1572–0x1595。
	// 落點是 word，影格是 **byte**（每組四格塞在 x 表前面那四個位元組）。
	for _, c := range []struct {
		what        string
		frame, x, y int // DGROUP 偏移；frame 為 0 表示影格是「深度 ＋ 固定值」不查表
		base        int // frame 為 0 時的影格起點
	}{
		{what: "正牆", x: 0x153E, y: 0x1546, base: 0},
		{what: "左側牆", x: 0x1552, y: 0x155A, base: 4},
		{what: "右側牆", x: 0x1576, y: 0x157E, base: 8},
		{what: "左縱列牆", frame: 0x154E, x: 0x1562, y: 0x156A},
		{what: "右縱列牆", frame: 0x1572, x: 0x1586, y: 0x158E},
	} {
		wantX, wantY := read(c.x), read(c.y)
		for d := 0; d < view.Depth; d++ {
			p := pick(c.what, d)
			if p.X != wantX[d] {
				t.Errorf("%s 深度 %d 的 x：程式 %d，EXE %d", c.what, d, p.X, wantX[d])
			}
			if p.Y != wantY[d] {
				t.Errorf("%s 深度 %d 的 y：程式 %d，EXE %d", c.what, d, p.Y, wantY[d])
			}
			want := c.base + d
			if c.frame != 0 {
				want = int(exe[exetext.DGroupBase+c.frame+d])
			}
			if p.Frame != want {
				t.Errorf("%s 深度 %d 的影格：程式 %d，EXE %d", c.what, d, p.Frame, want)
			}
		}
	}
}

// slotOf 是 IndoorGeometry 每個深度那五筆的順序。
func slotOf(what string) int {
	for i, s := range []string{"正牆", "左側牆", "右側牆", "左縱列牆", "右縱列牆"} {
		if s == what {
			return i
		}
	}
	return -1
}
