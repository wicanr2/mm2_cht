// mm2diff 把 remake 的畫面與原版在**同一個位置**的畫面逐像素比對。
//
//	go run ./cmd/mm2diff -x 8 -y 0 -face E
//
// 做四件事：用引擎的牆模型排出從進城起點走到指定格的按鍵、驅動 DOSBox
// 走過去並截圖、**讀記憶體確認隊伍真的在那裡**、再與 remake 畫的同一格
// 逐像素比。輸出並排圖與差異圖到 `workplace/diff/`。
//
// 為什麼要有這一支：`internal/view` 的每一個座標都是拿樣板去比對截圖定
// 出來的，而那是一次性的手工。改動繪圖層之後沒有東西會告訴你哪裡跑掉了 ——
// 「編譯成功不是視覺測試」。這支把視覺測試變成可以重跑的一行指令。
//
// 三個非守不可的規矩，都是踩過的：
//
//   - **每一個 `key:` 後面都要跟 `wait:1`。** 連著送 DOSBox 會掉鍵，
//     而掉鍵的症狀（走錯方向、撞牆）與「牆模型算錯」長得一模一樣。
//   - **走完一定要 `dump:` 對座標。** 不對就是路上出過事，這時候比畫面
//     只會得到一個看起來很嚴重、其實無關的差異。
//   - **每一步前面插一個 `key:n`** 取消設施提示。提示會擋住後續按鍵，
//     不取消的話走到第一個招牌就停住了。
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/ui"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// 進城之後隊伍站的地方與朝向，實機用 `dump:` 量出來的
// （見 docs/playtest/01-oracle-timeline.md §7）。
const (
	startX, startY = 7, 3
	startFace      = game.North
	townMap        = 0

	// 北門在米德格特的 (5,15)，面北按 Up 會問 `Exit (y/n)?`，
	// 答 `y` 就換到地圖 11 的 (7,3)（事件 opcode `0x0c`，由
	// `cmd/doorprobe 0` 印得出來）。**出城要回答 `y` 不是 space** ——
	// 與樓梯同一個坑，補 space 只會被當成「不出去」。
	gateX, gateY = 5, 15
	gateMap      = 11
	gateEntryX   = 7
	gateEntryY   = 3
)

// bootTimeline 是從開機走到第一人稱視角的固定前綴。
const bootTimeline = "wait:3;key:Return;wait:2;key:s;wait:4;key:g;wait:5;key:z;wait:4"

// DGROUP 的定位錨點：事件腳本的 opcode 長度表的前十二個值。
//
// 不能拿 `MM2.EXE` 這類字串當錨 —— dump 裡不只一處，而地圖資料在讀檔
// 緩衝裡也有一份。拿表自己當 pattern 才唯一（`tools/dump_tables.py` 同源）。
const anchorOffset = 0x15E8

var anchor = func() []byte {
	var b bytes.Buffer
	for _, v := range []uint16{2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 3, 3} {
		binary.Write(&b, binary.LittleEndian, v)
	}
	return b.Bytes()
}()

// DGROUP 裡的隊伍位置與朝向。
//
// **朝向存的是 ASCII 字母**（`'N'`/`'E'`/`'S'`/`'W'`），不是 0–3 的編號 ——
// `sub_1423E` 逐一 `cmp byte ptr ds:3CFh, 4Eh` 比對字母來決定方向遮罩，
// 狀態列的 `Face= N` 也是直接把這個位元組印出來。
const (
	offMap  = 0x0392
	offPosX = 0x0393
	offPosY = 0x0394
	offFace = 0x03CF
)

func main() {
	x := flag.Int("x", 8, "目標格的 X")
	y := flag.Int("y", 0, "目標格的 Y")
	faceS := flag.String("face", "E", "目標朝向 N/E/S/W")
	dataDir := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	outDir := flag.String("out", "workplace/diff", "輸出目錄")
	mapIdx := flag.Int("map", townMap, "目標地圖：0 米德格特、11 北門出去那張野外圖")
	shotOnly := flag.Bool("reuse", false, "不跑 DOSBox，直接用上一次的截圖與 dump")
	tlOnly := flag.Bool("timeline", false, "只印 timeline 就結束（Go 在容器裡、DOSBox 在 host 時分兩段跑）")
	oracle := flag.Bool("oracle", false, "拿上一次存下來的 -orig.png 當基準，完全不碰 DOSBox")
	flag.Parse()

	face, ok := parseFace(*faceS)
	if !ok {
		log.Fatalf("-face 只能是 N/E/S/W，收到 %q", *faceS)
	}

	// --- 1. 排路線 ---
	s, err := ui.Load(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	w := s.Game.World
	name := fmt.Sprintf("m%d-%d-%d-%v", *mapIdx, *x, *y, face)
	rc := image.Rect(view.FPX, view.FPY, view.FPX+view.FPW, view.FPY+view.FPH)

	// `-oracle` 拿上一次存下來的 208×120 基準圖直接比，路線、DOSBox、
	// 記憶體對位全部跳過。
	//
	// 為什麼要有這條路：一次 DOSBox 驗證要一分鐘，而改繪圖層時要問的問題
	// （這一塊是誰畫的、那張表對不對）常常要問十幾次 —— 慢迴圈不是拿來
	// 一直用的，能變快就先變快。基準圖是同一支程式存下來的，
	// 只要不換座標就與實機那次完全相同。
	if *oracle {
		orig, err := loadCrop(filepath.Join(*outDir, name+"-orig.png"), rc)
		if err != nil {
			log.Fatalf("讀不到基準圖：%v（先不帶 -oracle 跑一次，把它存下來）", err)
		}
		report(s, w, orig, rc, *mapIdx, *x, *y, face, *outDir, name)
		return
	}
	if *mapIdx != townMap && *mapIdx != gateMap {
		log.Fatalf("目前只排得出地圖 %d 與 %d 的路線（北門那條）", townMap, gateMap)
	}
	w.MapIndex = townMap
	m := w.CurrentMap()
	if m == nil {
		log.Fatal("載不到城鎮地圖")
	}
	// 野外那張要先走到北門、面北、按 Up 再答 `y`；出去之後落在 (7,3) 面北，
	// 後半段的路線從那裡再排一次。
	var walk string
	if *mapIdx == townMap {
		steps, err := route(m, startX, startY, *x, *y)
		if err != nil {
			log.Fatal(err)
		}
		keys, moves := keySequence(startFace, face, steps)
		fmt.Printf("路線：(%d,%d) → (%d,%d) 面 %v，%d 步、%d 個按鍵\n",
			startX, startY, *x, *y, face, moves, keys)
		walk = timeline(startFace, face, steps)
	} else {
		toGate, err := route(m, startX, startY, gateX, gateY)
		if err != nil {
			log.Fatal(err)
		}
		w.MapIndex = *mapIdx
		out := w.CurrentMap()
		if out == nil {
			log.Fatalf("載不到地圖 %d", *mapIdx)
		}
		inside, err := route(out, gateEntryX, gateEntryY, *x, *y)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("路線：城裡 %d 步到北門 → 出城 → 野外 %d 步到 (%d,%d) 面 %v\n",
			len(toGate), len(inside), *x, *y, face)
		walk = timelineTail(startFace, game.North, toGate, false) +
			";key:Up;wait:4;key:y;wait:6;" +
			timeline(game.North, face, inside)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	// --- 2. 驅動 DOSBox ---
	shotName, dumpName := "diff-shot", "diff-pos"
	// **Go 在容器裡跑不動 DOSBox**（那是 host 上的另一個 docker）。
	// `-timeline` 只把 timeline 印出來，由 host 執行 `tools/dosbox_run.sh`，
	// 跑完再用 `-reuse` 回來比對。
	if *tlOnly {
		fmt.Println(bootTimeline + ";" + walk +
			";wait:2;dump:" + dumpName + ";shot:" + shotName)
		return
	}
	if !*shotOnly {
		tl := bootTimeline + ";" + walk +
			";wait:2;dump:" + dumpName + ";shot:" + shotName
		fmt.Printf("timeline 共 %d 個步驟，DOSBox 大約要跑 %d 秒\n",
			strings.Count(tl, ";")+1, estimateSeconds(tl))
		cmd := exec.Command("tools/dosbox_run.sh", "ega", tl)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("DOSBox 跑失敗：%v", err)
		}
	}

	// --- 3. 對座標。不對就沒得比 ---
	shots := filepath.Join("workplace", "dosbox", "shots")
	mem, err := os.ReadFile(filepath.Join(shots, dumpName+".bin"))
	if err != nil {
		log.Fatalf("讀不到記憶體 dump：%v", err)
	}
	gotMap, gotX, gotY, gotFace, err := readPos(mem)
	if err != nil {
		log.Fatal(err)
	}
	// **朝向與座標一樣要驗。** 到得了那一格不代表面向對的方向 ——
	// 走到終點時跳出來的提示會把後面的轉向鍵吃掉，而畫面照樣是一張
	// 看起來很正常的第一人稱視圖，只是朝著別的方向。
	want := strings.ToUpper(*faceS)[0]
	if gotMap != *mapIdx || gotX != *x || gotY != *y || gotFace != want {
		log.Fatalf("實機停在地圖 %d 的 (%d,%d) 面 %c，不是目標 (%d,%d) 面 %c ——"+
			" 路上掉過鍵或被提示擋住，這時候比畫面沒有意義",
			gotMap, gotX, gotY, rune(gotFace), *x, *y, rune(want))
	}
	fmt.Printf("實機座標與朝向都對上了：地圖 %d (%d,%d) 面 %c\n",
		gotMap, gotX, gotY, rune(gotFace))

	orig, err := loadShot(filepath.Join(shots, shotName+".png"))
	if err != nil {
		log.Fatal(err)
	}
	report(s, w, orig, rc, *mapIdx, *x, *y, face, *outDir, name)
}

// report 畫 remake 的同一格、與基準逐像素比、寫圖，差一個像素就非零離開。
func report(s *ui.Session, w *game.World, orig *image.Paletted, rc image.Rectangle,
	mapIdx, x, y int, face game.Facing, outDir, name string) {
	// **三個火焰相位都試。** 火炬是動畫，截圖抓到的是哪一張取決於按下
	// 截圖鍵的那一刻。固定用相位 0 去比，會把「火焰正好在別張」報成差異 ——
	// 那是工具的問題不是繪圖的問題，而兩者長得一樣。
	w.MapIndex, w.X, w.Y, w.Face = mapIdx, x, y, face
	var mine *image.Paletted
	var diff *image.Gray
	n, best := 0, -1
	for p := 0; p < view.TorchFrames; p++ {
		s.TorchPhase = p
		cand := s.Draw().Orig
		d, cn := compare(orig, cand, rc)
		if best < 0 || cn < n {
			mine, diff, n, best = cand, d, cn, p
		}
	}
	if view.TorchFrames > 1 {
		fmt.Printf("火焰相位取 %d（三個裡最相符的）\n", best)
	}

	total := rc.Dx() * rc.Dy()
	fmt.Printf("第一人稱視圖 %d×%d：%d 個像素不同（%.1f%%）\n",
		rc.Dx(), rc.Dy(), n, 100*float64(n)/float64(total))
	if n > 0 {
		fmt.Println("差異集中在：", hotspots(diff, rc))
	}

	writePNG(filepath.Join(outDir, name+"-orig.png"), crop(orig, rc))
	writePNG(filepath.Join(outDir, name+"-mine.png"), crop(mine, rc))
	writePNG(filepath.Join(outDir, name+"-diff.png"), sideBySide(orig, mine, diff, rc))
	fmt.Printf("圖寫到 %s/%s-{orig,mine,diff}.png\n", outDir, name)
	if n > 0 {
		os.Exit(1)
	}
}

func parseFace(s string) (game.Facing, bool) {
	switch strings.ToUpper(s) {
	case "N":
		return game.North, true
	case "E":
		return game.East, true
	case "S":
		return game.South, true
	case "W":
		return game.West, true
	}
	return 0, false
}

// route 用引擎的牆模型 BFS 排出一串方向。
func route(m *game.Map, sx, sy, gx, gy int) ([]game.Facing, error) {
	type node struct{ x, y int }
	start, goal := node{sx, sy}, node{gx, gy}
	if start == goal {
		return nil, nil
	}
	prev := map[node]node{}
	dir := map[node]game.Facing{}
	seen := map[node]bool{start: true}
	queue := []node{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for f := 0; f < 4; f++ {
			if !m.CanMove(cur.x, cur.y, game.Facing(f)) {
				continue
			}
			dx, dy := game.Facing(f).Delta()
			n := node{cur.x + dx, cur.y + dy}
			if n.x < 0 || n.x > 15 || n.y < 0 || n.y > 15 || seen[n] {
				continue
			}
			seen[n] = true
			prev[n], dir[n] = cur, game.Facing(f)
			if n == goal {
				var out []game.Facing
				for c := goal; c != start; c = prev[c] {
					out = append([]game.Facing{dir[c]}, out...)
				}
				return out, nil
			}
			queue = append(queue, n)
		}
	}
	return nil, fmt.Errorf("牆模型走不到 (%d,%d)", gx, gy)
}

// turns 回傳從 from 轉到 to 要按幾次哪個鍵。
//
// 原版只有左轉右轉，沒有「直接面向某方位」——三次同向轉比一次反向轉多
// 兩個按鍵，而每個按鍵都是一次掉鍵的機會，所以取短的那邊。
func turns(from, to game.Facing) (key string, n int) {
	d := (int(to) - int(from) + 4) % 4
	switch d {
	case 0:
		return "", 0
	case 1:
		return "Right", 1
	case 3:
		return "Left", 1
	}
	return "Right", 2
}

// timeline 把一串方向翻成 DOSBox 的 timeline 片段。
func timeline(start, end game.Facing, steps []game.Facing) string {
	return timelineTail(start, end, steps, true)
}

// timelineTail 的 cancel 決定走完之後要不要補一次 `n`。
//
// **出城那一步不能補**：終點 (5,15) 的提示就是 `Exit (y/n)?`，
// 補一個 `n` 等於當場回答「不出去」，之後再按 `Up` 也不會再問 ——
// 症狀是「座標停在門口、畫面完全正常」，看不出是被自己關掉的。
func timelineTail(start, end game.Facing, steps []game.Facing, cancel bool) string {
	var b strings.Builder
	cur := start
	emit := func(k string) {
		if b.Len() > 0 {
			b.WriteByte(';')
		}
		fmt.Fprintf(&b, "key:%s;wait:1", k)
	}
	for _, f := range steps {
		k, n := turns(cur, f)
		for i := 0; i < n; i++ {
			emit(k)
		}
		cur = f
		// 設施提示會擋住後面的按鍵，所以每走一步之前先取消一次。
		emit("n")
		emit("Up")
	}
	// 走到終點會跳出該格的提示（樓梯、招牌），不取消的話後面的轉向鍵
	// 會被吃掉 —— 症狀是「座標對了但朝向不對」，而畫面看起來完全正常。
	if cancel && len(steps) > 0 {
		emit("n")
	}
	k, n := turns(cur, end)
	for i := 0; i < n; i++ {
		emit(k)
	}
	return b.String()
}

func keySequence(start, end game.Facing, steps []game.Facing) (keys, moves int) {
	return strings.Count(timeline(start, end, steps), "key:"), len(steps)
}

// estimateSeconds 粗估這條 timeline 要跑多久，開跑前先講。
func estimateSeconds(tl string) int {
	n := 0
	for _, part := range strings.Split(tl, ";") {
		switch {
		case strings.HasPrefix(part, "wait:"):
			var v int
			fmt.Sscanf(part, "wait:%d", &v)
			n += v
		default:
			n++ // 按鍵與截圖各算一秒的餘裕
		}
	}
	return n + 10 // DOSBox 起停
}

// readPos 從記憶體 dump 讀出隊伍的地圖與座標。
func readPos(mem []byte) (mapIdx, x, y int, face byte, err error) {
	i := bytes.Index(mem, anchor)
	if i < 0 {
		return 0, 0, 0, 0, fmt.Errorf("dump 裡找不到定位用的 pattern —— 截圖時可能還沒進到遊戲中")
	}
	if bytes.Index(mem[i+1:], anchor) >= 0 {
		fmt.Fprintln(os.Stderr, "警告：定位 pattern 命中多處，取第一個")
	}
	dg := i - anchorOffset
	if dg < 0 || dg+offFace >= len(mem) {
		return 0, 0, 0, 0, fmt.Errorf("算出來的 DGROUP 落在 dump 之外")
	}
	return int(mem[dg+offMap]), int(mem[dg+offPosX]), int(mem[dg+offPosY]),
		mem[dg+offFace], nil
}

// loadShot 讀 DOSBox 的截圖並回推成 EGA 色號。
//
// 截圖是 1024×768 的視窗擷取，遊戲畫面**以 1:1 畫在左上角** ——
// 不是縮放過的，所以像素座標就是 320×200 的遊戲座標。
func loadShot(path string) (*image.Paletted, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	if b.Dx() < render.OrigW || b.Dy() < render.OrigH {
		return nil, fmt.Errorf("截圖只有 %d×%d，放不下 %d×%d 的遊戲畫面",
			b.Dx(), b.Dy(), render.OrigW, render.OrigH)
	}
	dst := image.NewPaletted(image.Rect(0, 0, render.OrigW, render.OrigH), gfx.EGAPalette)
	for y := 0; y < render.OrigH; y++ {
		for x := 0; x < render.OrigW; x++ {
			r, g, bb, _ := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			dst.SetColorIndex(x, y, uint8(nearest(dst.Palette,
				color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bb >> 8), 0xFF})))
		}
	}
	return dst, nil
}

// loadCrop 把上一次存下來的 208×120 基準圖放回它在整幅畫面裡的位置。
//
// 存的是裁好的視圖，比對走的是整幅座標，所以要貼回 `rc.Min` ——
// 貼錯位置的症狀是「整幅都不同」，看起來像繪圖層全壞了。
func loadCrop(path string, rc image.Rectangle) (*image.Paletted, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	if b.Dx() != rc.Dx() || b.Dy() != rc.Dy() {
		return nil, fmt.Errorf("基準圖是 %d×%d，視圖是 %d×%d", b.Dx(), b.Dy(), rc.Dx(), rc.Dy())
	}
	dst := image.NewPaletted(image.Rect(0, 0, render.OrigW, render.OrigH), gfx.EGAPalette)
	for y := 0; y < rc.Dy(); y++ {
		for x := 0; x < rc.Dx(); x++ {
			r, g, bb, _ := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			dst.SetColorIndex(rc.Min.X+x, rc.Min.Y+y, uint8(nearest(dst.Palette,
				color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bb >> 8), 0xFF})))
		}
	}
	return dst, nil
}

func nearest(pal color.Palette, c color.RGBA) int {
	bestI, bestD := 0, 1<<30
	for i, p := range pal {
		r, g, b, _ := p.RGBA()
		dr, dg, db := int(r>>8)-int(c.R), int(g>>8)-int(c.G), int(b>>8)-int(c.B)
		if d := dr*dr + dg*dg + db*db; d < bestD {
			bestI, bestD = i, d
		}
	}
	return bestI
}

// compare 回傳差異遮罩與不同的像素數。
func compare(a, b *image.Paletted, rc image.Rectangle) (*image.Gray, int) {
	d := image.NewGray(rc)
	n := 0
	for y := rc.Min.Y; y < rc.Max.Y; y++ {
		for x := rc.Min.X; x < rc.Max.X; x++ {
			if a.ColorIndexAt(x, y) != b.ColorIndexAt(x, y) {
				d.SetGray(x, y, color.Gray{0xFF})
				n++
			}
		}
	}
	return d, n
}

// hotspots 把差異切成 8×8 的格子，回報最密集的幾塊。
//
// 「不同 412 個像素」沒有診斷價值，「都集中在左上角那一塊」才有。
func hotspots(d *image.Gray, rc image.Rectangle) string {
	const cell = 8
	type box struct {
		x, y, n int
	}
	var boxes []box
	for gy := rc.Min.Y; gy < rc.Max.Y; gy += cell {
		for gx := rc.Min.X; gx < rc.Max.X; gx += cell {
			n := 0
			for y := gy; y < gy+cell && y < rc.Max.Y; y++ {
				for x := gx; x < gx+cell && x < rc.Max.X; x++ {
					if d.GrayAt(x, y).Y != 0 {
						n++
					}
				}
			}
			if n > 0 {
				boxes = append(boxes, box{gx, gy, n})
			}
		}
	}
	for i := 1; i < len(boxes); i++ {
		for j := i; j > 0 && boxes[j].n > boxes[j-1].n; j-- {
			boxes[j], boxes[j-1] = boxes[j-1], boxes[j]
		}
	}
	var parts []string
	for i, b := range boxes {
		if i >= 5 {
			parts = append(parts, fmt.Sprintf("…另外 %d 塊", len(boxes)-5))
			break
		}
		parts = append(parts, fmt.Sprintf("(%d,%d) %d 點", b.x, b.y, b.n))
	}
	return strings.Join(parts, "、")
}

func crop(src *image.Paletted, rc image.Rectangle) image.Image {
	dst := image.NewPaletted(image.Rect(0, 0, rc.Dx(), rc.Dy()), src.Palette)
	for y := 0; y < rc.Dy(); y++ {
		for x := 0; x < rc.Dx(); x++ {
			dst.SetColorIndex(x, y, src.ColorIndexAt(rc.Min.X+x, rc.Min.Y+y))
		}
	}
	return dst
}

// sideBySide 疊成一張「原版｜remake｜差異」，差異用洋紅畫在原版上。
func sideBySide(a, b *image.Paletted, d *image.Gray, rc image.Rectangle) image.Image {
	const gap = 4
	w, h := rc.Dx(), rc.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w*3+gap*2, h))
	put := func(ox int, f func(x, y int) color.Color) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				out.Set(ox+x, y, f(rc.Min.X+x, rc.Min.Y+y))
			}
		}
	}
	put(0, func(x, y int) color.Color { return a.At(x, y) })
	put(w+gap, func(x, y int) color.Color { return b.At(x, y) })
	put(w*2+gap*2, func(x, y int) color.Color {
		if d.GrayAt(x, y).Y != 0 {
			return color.RGBA{0xFF, 0x00, 0xFF, 0xFF}
		}
		return a.At(x, y)
	})
	return out
}

func writePNG(path string, im image.Image) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, im); err != nil {
		log.Fatal(err)
	}
}
