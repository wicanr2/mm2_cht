// mm2atlas 產生攻略網站要用的地圖：每張圖一份**原創向量圖**（SVG）
// 加一份 **remake 遊戲內俯視圖**（PNG），外加一份索引 JSON。
//
//	go run ./cmd/mm2atlas -data workplace/orig/MM2 -out workplace/site/maps
//
// 為什麼要兩種圖：
//
//   - SVG 是我們自己從 `MAP.DAT`／`ATTRIB.DAT` 畫出來的，**沒有一個像素
//     來自原版美術**，而且格線上標得下攻略提示的編號 —— 讀攻略的人要能
//     一眼看到「那條提示講的是哪一格」。
//   - PNG 是玩家實際按 `M` 會看到的畫面，用來對照「原創圖上的那一格，
//     在遊戲裡長什麼樣」。它走的是 `internal/ui` 那條與視窗無關的路徑，
//     所以沒有 GPU 也產得出來。
//
// **室內與室外是兩套完全不同的畫法**，因為資料本身就不同源：室內的牆
// 位元在屬性層，室外那幾個位元放的是地形碼（見 `internal/game/wall.go`
// 的 `CanMove`）。拿同一套畫法套兩邊，野外會長出一整片假牆。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

// cell 是 SVG 裡一格的邊長（px）。pad 留給座標軸的數字。
const (
	cell   = 34
	padL   = 30
	padT   = 22
	padR   = 12
	padB   = 30
	gridPx = game.MapW * cell
)

// 語意色。全站一組，`94-github-pages-doc-style` §3：一個顏色一個意思。
const (
	colPaper   = "#faf8f4"
	colGrid    = "#d9d2c6" // 結構線：格線
	colWall    = "#3a352d" // 實牆
	colDoor    = "#a8552a" // 門（關鍵路徑色）
	colBarrier = "#9a9384" // 屏障：擋路但看不見
	colEvent   = "#6a8f6a" // 事件格
	colDark    = "#efe7d8" // 吸光的格子
	colPin     = "#a8552a"
	colText    = "#3a352d"
	colMuted   = "#7c7466"
)

// 野外地形類別的填色。只變明度不變色相（同上 §3）。
var terrainFill = map[int]string{
	game.TerrainOpenClass:     "#faf8f4",
	game.TerrainMountainClass: "#cfc6b4",
	game.TerrainForestClass:   "#dfe6d6",
	game.TerrainWaterClass:    "#dbe4ec",
}

var terrainName = map[int]string{
	game.TerrainOpenClass:     "可通行",
	game.TerrainMountainClass: "山區（要兩名登山家）",
	game.TerrainForestClass:   "森林（要兩名探險家）",
	game.TerrainWaterClass:    "水域（要水行術）",
}

// townMaps 是五座城（`docs/formats/06-map.md` §3）。
var townMaps = []int{0, 1, 2, 3, 4}

// outdoorMaps 是二十個野外區，順序即 A1…E4。編號來自飛行術的 5×4 跳表
// （`docs/formats/09-spells.md`，`ds:30BC`）。
var outdoorMaps = []int{
	5, 9, 12, 15, // A1 A2 A3 A4
	6, 10, 13, 16, // B1 B2 B3 B4
	7, 11, 14, 38, // C1 C2 C3 C4
	8, 34, 36, 39, // D1 D2 D3 D4
	33, 35, 37, 40, // E1 E2 E3 E4
}

// townNames 補上沒有攻略條目的城名，讓圖說不會只剩編號。
var townNames = map[int]string{
	0: "米德格特 Middlegate（1 號城）",
	1: "亞特蘭汀 Atlantium（2 號城）",
	2: "桑達拉 Tundara（3 號城）",
	3: "佛卡尼亞 Vulcania（4 號城）",
	4: "桑德索巴 Sansobar（5 號城）",
}

// areaOf 由野外地圖編號反查 A1–E4。
func areaOf(mi int) string {
	for i, m := range outdoorMaps {
		if m == mi {
			return fmt.Sprintf("%c%d", 'A'+i/4, i%4+1)
		}
	}
	return ""
}

// pin 是畫在格子上的提示編號。
type pin struct {
	N     int    `json:"n"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Text  string `json:"text"`
	From  string `json:"from"`
	Level string `json:"level"`
}

// entry 是索引 JSON 的一筆。
type entry struct {
	Map    int    `json:"map"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Area   string `json:"area"`
	Indoor bool   `json:"indoor"`
	SVG    string `json:"svg"`
	PNG    string `json:"png"`
	Pins   []pin  `json:"pins"`
	// Notes 是**沒有座標**的提示。它們一樣要出現在頁面上 ——
	// 上不了圖不代表不重要，漏掉的話讀者會以為那個地點只有圖釘那幾條。
	Notes []pin `json:"notes"`
	// Stats 是這張圖的幾個數字，寫在圖說裡當量測基準。
	Stats map[string]int `json:"stats"`
}

func main() {
	data := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	dataDir := flag.String("datadir", "data", "衍生資料目錄（要有 hints.json）")
	out := flag.String("out", "workplace/site/maps", "輸出目錄")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	sess, err := ui.Load(*data)
	if err != nil {
		log.Fatal(err)
	}
	hints := ui.LoadHints(*dataDir)
	if hints.LoadError != "" {
		log.Fatalf("讀 hints.json 失敗：%s", hints.LoadError)
	}

	// 只畫「攻略講得到」的圖：有地圖編號的地點。沒有編號的地下城與城堡
	// 用地表入口定位，畫不出格位（`docs/research/soft-world/11-walkthrough-index.md`）。
	byMap := map[int]ui.HintPlace{}
	for _, p := range hints.Places {
		if p.Map != nil {
			byMap[*p.Map] = p
		}
	}
	// **有攻略提示的圖不等於該畫的圖。** 五座城與二十個野外區是玩家一定
	// 會走到的，即使雜誌沒有寫到那一張也要有圖 —— 少一張的長相與
	// 「那張圖不存在」一樣。四號城 Vulcania 就是這個情況。
	want := map[int]bool{}
	for _, i := range townMaps {
		want[i] = true
	}
	for _, i := range outdoorMaps {
		want[i] = true
	}
	for i := range byMap {
		want[i] = true
	}
	idxs := make([]int, 0, len(want))
	for i := range want {
		idxs = append(idxs, i)
	}
	sort.Ints(idxs)

	var index []entry
	for _, mi := range idxs {
		place, hasHints := byMap[mi]
		if place.Name == "" {
			if n, ok := townNames[mi]; ok {
				place.Name, place.Kind = n, "town"
			} else if a := areaOf(mi); a != "" {
				place.Name, place.Kind, place.Area = a+" 區（野外）", "outdoor", a
			} else {
				place.Name = fmt.Sprintf("地圖 %d", mi)
			}
		}
		if place.Area == "" {
			place.Area = areaOf(mi)
		}
		_ = hasHints
		sess.Game.World.MapIndex = mi
		sess.Game.World.X, sess.Game.World.Y = 0, 0
		m := sess.Game.World.CurrentMap()
		if m == nil {
			log.Printf("地圖 %d 讀不到，跳過", mi)
			continue
		}
		pins, notes := makePins(place)
		svgName := fmt.Sprintf("map-%02d.svg", mi)
		pngName := fmt.Sprintf("map-%02d.png", mi)
		if err := os.WriteFile(filepath.Join(*out, svgName),
			[]byte(renderSVG(m, place, pins)), 0o644); err != nil {
			log.Fatal(err)
		}
		if err := writeGameMap(sess, filepath.Join(*out, pngName)); err != nil {
			log.Fatal(err)
		}
		index = append(index, entry{
			Map: mi, Name: place.Name, Kind: place.Kind, Area: place.Area,
			Indoor: m.Indoor, SVG: svgName, PNG: pngName, Pins: pins, Notes: notes,
			Stats: stats(m),
		})
		fmt.Printf("地圖 %2d  %-34s %s  %d 個圖釘\n", mi, place.Name, kindLabel(m), len(pins))
	}
	// **沒有地圖編號的地點也要輸出。** 地下城、城堡與元素界佔了 46 個地點裡的
	// 22 個，它們畫不出格位圖（編號還沒定出來），但提示照樣要進網站；
	// 只輸出畫得出圖的那些，站上就會少掉將近一半的內容。
	var places []map[string]any
	for _, p := range hints.Places {
		pins, notes := makePins(p)
		e := map[string]any{
			"name": p.Name, "kind": p.Kind, "area": p.Area,
			"pins": pins, "notes": notes,
		}
		if p.Map != nil {
			e["map"] = *p.Map
		}
		if p.Entrance != nil {
			e["entrance"] = p.Entrance.String()
		}
		places = append(places, e)
	}
	doc := map[string]any{
		"source":   "MAP.DAT ＋ ATTRIB.DAT（玩家自備的原版），提示來自 data/hints.json",
		"legend":   legend(),
		"maps":     index,
		"places":   places,
		"general":  hints.General,
		"conflict": hints.Conflicts,
	}
	b, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "atlas.json"), append(b, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n%d 張地圖 → %s\n", len(index), *out)
}

func kindLabel(m *game.Map) string {
	if m.Indoor {
		return "室內"
	}
	return "野外"
}

// stats 是寫在圖說裡的量測基準。**圖說要說得出這張圖數過什麼**，
// 否則讀者分不出「這格沒有標記」與「我們沒有畫那一類」。
func stats(m *game.Map) map[string]int {
	s := map[string]int{}
	for y := 0; y < game.MapH; y++ {
		for x := 0; x < game.MapW; x++ {
			if m.HasEvent(x, y) {
				s["event"]++
			}
			if m.Indoor {
				for _, f := range []game.Facing{game.North, game.East, game.South, game.West} {
					switch {
					case !m.HasWall(x, y, f):
					case m.WallKind(x, y, f) == game.WallDoor:
						s["door"]++
					case m.DrawKind(x, y, f) == game.WallNone:
						s["barrier"]++
					default:
						s["wall"]++
					}
				}
				if m.DrainsLight(x, y) {
					s["dark"]++
				}
				continue
			}
			switch m.TerrainClass(x, y) {
			case game.TerrainMountainClass:
				s["mountain"]++
			case game.TerrainForestClass:
				s["forest"]++
			case game.TerrainWaterClass:
				s["water"]++
			default:
				s["open"]++
			}
		}
	}
	// 每一面牆被兩格各數一次（自己與鄰居），除以二才是實際的面數。
	for _, k := range []string{"door", "barrier", "wall"} {
		s[k] = s[k] / 2
	}
	return s
}

// makePins 把帶座標的提示排成圖釘。沒有座標的提示不上圖，只進正文 ——
// **編一個看起來合理的座標比不標更糟**，讀者會照著去找。
func makePins(p ui.HintPlace) (pins, notes []pin) {
	n := 0
	for _, h := range p.Hints {
		x, y, ok := parseCoord(h.Coord)
		if !ok {
			notes = append(notes, pin{Text: h.Text, From: h.From, Level: h.Level})
			continue
		}
		n++
		pins = append(pins, pin{N: n, X: x, Y: y, Text: h.Text, From: h.From, Level: h.Level})
	}
	return pins, notes
}

func parseCoord(s string) (int, int, bool) {
	// 形式是 "x,y"，也接受 "A1-x,y" 這種前面帶區域碼的。
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s = s[i+1:]
	}
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	x, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	y, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || x < 0 || x >= game.MapW || y < 0 || y >= game.MapH {
		return 0, 0, false
	}
	return x, y, true
}

// px 把地圖座標換成 SVG 座標。**y 是往北增加的**，畫面往下增加，所以要反號。
func px(x, y int) (float64, float64) {
	return float64(padL + x*cell), float64(padT + (game.MapH-1-y)*cell)
}

func renderSVG(m *game.Map, p ui.HintPlace, pins []pin) string {
	w := padL + gridPx + padR
	h := padT + gridPx + padB
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-label="%s 的地圖">`,
		w, h, w, h, html.EscapeString(p.Name))
	fmt.Fprintf(&b, `<style>
text{font-family:"Noto Sans TC","PingFang TC","Microsoft JhengHei",system-ui,sans-serif}
.ax{font-size:10px;fill:%s;font-variant-numeric:tabular-nums}
.pn{font-size:13px;font-weight:660;fill:%s}
</style>`, colMuted, colPaper)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`, w, h, colPaper)

	// 格子底色：室外按地形類別，室內只標「吸光」那些。
	for y := 0; y < game.MapH; y++ {
		for x := 0; x < game.MapW; x++ {
			cx, cy := px(x, y)
			fill := ""
			if m.Indoor {
				if m.DrainsLight(x, y) {
					fill = colDark
				}
			} else {
				fill = terrainFill[m.TerrainClass(x, y)]
			}
			if fill != "" && fill != colPaper {
				fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="%d" height="%d" fill="%s"/>`,
					cx, cy, cell, cell, fill)
			}
		}
	}
	// 格線
	for i := 0; i <= game.MapW; i++ {
		x := float64(padL + i*cell)
		fmt.Fprintf(&b, `<line x1="%.0f" y1="%d" x2="%.0f" y2="%d" stroke="%s" stroke-width="1"/>`,
			x, padT, x, padT+gridPx, colGrid)
		y := float64(padT + i*cell)
		fmt.Fprintf(&b, `<line x1="%d" y1="%.0f" x2="%d" y2="%.0f" stroke="%s" stroke-width="1"/>`,
			padL, y, padL+gridPx, y, colGrid)
	}
	// 座標軸
	for i := 0; i < game.MapW; i++ {
		cx, _ := px(i, 0)
		fmt.Fprintf(&b, `<text class="ax" x="%.0f" y="%d" text-anchor="middle">%d</text>`,
			cx+cell/2, padT-7, i)
		_, cy := px(0, i)
		fmt.Fprintf(&b, `<text class="ax" x="%d" y="%.0f" text-anchor="end">%d</text>`,
			padL-6, cy+cell/2+4, i)
	}

	if m.Indoor {
		renderWalls(&b, m)
		// 圖框：**地圖外圍一律過不去**（`Cell` 出界時 `HasWall` 直接回 true），
		// 而外圈格子自己的牆位元不一定有設。少了這一圈，室內圖看起來像是
		// 四邊敞開的，讀者會以為走得出去。
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" fill="none" stroke="%s" stroke-width="3"/>`,
			padL, padT, gridPx, gridPx, colWall)
	}
	// 事件格：一個小方塊，不填滿整格 —— 填滿會蓋掉地形色。
	for y := 0; y < game.MapH; y++ {
		for x := 0; x < game.MapW; x++ {
			if !m.HasEvent(x, y) {
				continue
			}
			cx, cy := px(x, y)
			fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="6" height="6" fill="%s"/>`,
				cx+float64(cell)/2-3, cy+float64(cell)/2-3, colEvent)
		}
	}
	// 圖釘畫在最上層。
	for _, pn := range pins {
		cx, cy := px(pn.X, pn.Y)
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="10" fill="%s"/>`,
			cx+float64(cell)/2, cy+float64(cell)/2, colPin)
		fmt.Fprintf(&b, `<text class="pn" x="%.1f" y="%.1f" text-anchor="middle">%d</text>`,
			cx+float64(cell)/2, cy+float64(cell)/2+4.5, pn.N)
	}
	// 方位：北在上。原版的 y 往北增加，這一行是給讀者的提醒。
	fmt.Fprintf(&b, `<text class="ax" x="%d" y="%d">北在上　x 往東增加　y 往北增加</text>`,
		padL, padT+gridPx+18)
	b.WriteString(`</svg>`)
	return b.String()
}

// renderWalls 畫室內的牆。
//
// 每一面牆由兩格共用，所以只畫「北側」與「西側」兩個方向，再補最外圈的
// 南緣與東緣 —— 四個方向都畫的話每條線會疊兩次，門的顏色會被實牆蓋掉。
func renderWalls(b *strings.Builder, m *game.Map) {
	line := func(x1, y1, x2, y2 float64, kind game.WallKind, drawn game.WallKind) {
		col, wd, dash := colWall, 3.0, ""
		switch {
		case kind == game.WallDoor:
			col, wd = colDoor, 3.5
		case drawn == game.WallNone:
			// 擋路但看不見：原版撞上去印 `Barrier!`。虛線代表「走不過去，
			// 但畫面上什麼都沒有」—— 實線會讓讀者以為那裡有一堵牆。
			col, wd, dash = colBarrier, 2.0, ` stroke-dasharray="4 3"`
		}
		fmt.Fprintf(b, `<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="%s" stroke-width="%.1f"%s stroke-linecap="square"/>`,
			x1, y1, x2, y2, col, wd, dash)
	}
	for y := 0; y < game.MapH; y++ {
		for x := 0; x < game.MapW; x++ {
			cx, cy := px(x, y)
			if m.HasWall(x, y, game.North) {
				line(cx, cy, cx+float64(cell), cy,
					m.WallKind(x, y, game.North), m.DrawKind(x, y, game.North))
			}
			if m.HasWall(x, y, game.West) {
				line(cx, cy, cx, cy+float64(cell),
					m.WallKind(x, y, game.West), m.DrawKind(x, y, game.West))
			}
			if y == 0 && m.HasWall(x, y, game.South) {
				line(cx, cy+float64(cell), cx+float64(cell), cy+float64(cell),
					m.WallKind(x, y, game.South), m.DrawKind(x, y, game.South))
			}
			if x == game.MapW-1 && m.HasWall(x, y, game.East) {
				line(cx+float64(cell), cy, cx+float64(cell), cy+float64(cell),
					m.WallKind(x, y, game.East), m.DrawKind(x, y, game.East))
			}
		}
	}
}

// writeGameMap 拍一張玩家按 `M` 會看到的俯視圖。
//
// **要先把整張標成走過。** remake 的自動地圖只畫走過的格子，剛換過去的圖
// 一格都沒走過 —— 拍出來是整片黑，而整片黑與「這張圖沒有東西」在截圖上
// 分不出來。城鎮圖看起來正常只是因為它們在 `Mapped` 清單裡（手冊附了全圖），
// 野外圖不在。這一步等於玩家施了定位術之後的畫面。
func writeGameMap(s *ui.Session, path string) error {
	s.Game.World.Explored.MarkMap(s.Game.World.MapIndex)
	s.Mode = ui.ModeMap
	scr := s.Draw()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, scr.Hi); err != nil {
		return err
	}
	s.Mode = ui.ModeExplore
	return nil
}

// legend 把圖例連同色碼一起輸出，網站產生器直接用這一份 ——
// **圖例與畫圖的程式必須同一個來源**，各寫一套就會漂移，
// 而漂移的症狀是「圖上是這個顏色，圖例說是另一回事」。
func legend() []map[string]string {
	l := []map[string]string{
		{"color": colWall, "label": "實牆", "shape": "line"},
		{"color": colDoor, "label": "門（上鎖）", "shape": "line"},
		{"color": colBarrier, "label": "屏障：擋路但畫面上看不見", "shape": "dash"},
		{"color": colEvent, "label": "事件格", "shape": "dot"},
		{"color": colDark, "label": "吸光的格子", "shape": "fill"},
		{"color": colPin, "label": "攻略提示（編號對應正文）", "shape": "pin"},
	}
	for _, c := range []int{
		game.TerrainOpenClass, game.TerrainMountainClass,
		game.TerrainForestClass, game.TerrainWaterClass,
	} {
		l = append(l, map[string]string{
			"color": terrainFill[c], "label": terrainName[c], "shape": "fill",
		})
	}
	return l
}
