// Package game 是遊戲規則層：地圖、隊伍位置、移動、事件觸發。
//
// 這一層不碰畫面與輸入，只處理狀態轉移，才能用固定輸入寫確定性測試。
// 依賴方向是單向的：platform/UI → game → assets。
package game

import (
	"fmt"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

// 地圖是 16×16 格，格位置 row-major，與事件表的 Cell 欄位同一套座標。
// 見 docs/formats/06-map.md。
const (
	MapW, MapH = 16, 16
	MapCells   = MapW * MapH
)

// Facing 是隊伍朝向。原版狀態列顯示 Face= N/E/S/W。
type Facing byte

const (
	North Facing = iota
	East
	South
	West
)

func (f Facing) String() string {
	return [...]string{"N", "E", "S", "W"}[f&3]
}

// Delta 回傳朝這個方向前進一格的位移。
//
// **地圖的第 0 列在南邊**，往北是列號加一。依據是手冊的城鎮地圖：
// 座標軸畫在左下角、y 由下往上，而圖上標記的設施位置與事件表的格編號
// 在 y 上對得起來（十處抽驗有八處完全相同，見 docs/formats/06-map.md §5）。
func (f Facing) Delta() (dx, dy int) {
	switch f & 3 {
	case North:
		return 0, 1
	case East:
		return 1, 0
	case South:
		return 0, -1
	default:
		return -1, 0
	}
}

// Map 是一張 16×16 的地圖。兩層各 256 bytes：地形層與屬性層。
type Map struct {
	Index   int
	Terrain [MapCells]byte
	Attr    [MapCells]byte
}

// ParseMaps 解開 MAP.DAT 的全部 60 張地圖。
func ParseMaps(blob []byte) ([]Map, error) {
	const count = 60
	if len(blob) < count*2 {
		return nil, fmt.Errorf("MAP.DAT 只有 %d bytes，放不下 %d 筆索引", len(blob), count)
	}
	maps := make([]Map, 0, count)
	for i := 0; i < count; i++ {
		off := int(blob[i*2]) | int(blob[i*2+1])<<8
		raw, err := lzw.Segment(blob, off)
		if err != nil {
			return nil, fmt.Errorf("地圖 %d @%d: %w", i, off, err)
		}
		if len(raw) != MapCells*2 {
			return nil, fmt.Errorf("地圖 %d 解出 %d bytes，預期 %d", i, len(raw), MapCells*2)
		}
		m := Map{Index: i}
		copy(m.Terrain[:], raw[:MapCells])
		copy(m.Attr[:], raw[MapCells:])
		maps = append(maps, m)
	}
	return maps, nil
}

// Cell 把座標換成格編號。超出範圍回 -1。
func Cell(x, y int) int {
	if x < 0 || x >= MapW || y < 0 || y >= MapH {
		return -1
	}
	return y*MapW + x
}

// HasEvent 回報 (x, y) 的屬性層有沒有設事件位元。
func (m *Map) HasEvent(x, y int) bool {
	c := Cell(x, y)
	return c >= 0 && m.Attr[c]&AttrHasEvent != 0
}

// World 是一次遊玩的狀態。
type World struct {
	Maps   []Map
	Events []events.Segment

	MapIndex int
	X, Y     int
	Face     Facing

	// Message 是最近一次觸發的事件文字，空字串表示沒有。
	Message string

	// Encounter 是腳本擺出來的固定遭遇（怪物編號），空的表示沒有。
	// 由 opcode `0x12`／`0x13` 設定，呼叫端取走之後要自己清掉。
	Encounter []int

	// Paid 是付款旗標（原版的 `ds:042F`）。付款那組 opcode 還沒實作，
	// 所以它恆為 false ——「付不出來」那一邊，與原版在沒付款時一致。
	Paid bool

	// Flag 是事件腳本的條件旗標（原版的 `ds:0509`）。
	//
	// **每次移動都會清掉**（`2PLAY.img` 的 `0x4300` 那段，緊接在更新
	// 隊伍座標之後），由 opcode `0x19`／`0x32` 與戰鬥模組設定。
	// opcode `0x2b` 讀它決定要不要跳過後面幾個 opcode。
	//
	// 設定它的那兩個 opcode 語意還沒解，所以現在它恆為 false ——
	// 也就是條件分支一律走「不跳過」那一邊，與原版在旗標未設時一致。
	Flag bool
}

// NewWorld 載入地圖與事件。MAP 段 k 對應 EVENTSI 段 k
// （見 docs/formats/06-map.md §3）。
func NewWorld(mapBlob, eventBlob []byte) (*World, error) {
	if err := EnsureData(); err != nil {
		return nil, err
	}
	maps, err := ParseMaps(mapBlob)
	if err != nil {
		return nil, err
	}
	segs, err := events.Parse(eventBlob)
	if err != nil {
		return nil, err
	}
	return &World{Maps: maps, Events: segs, Face: North}, nil
}

// CurrentMap 回傳目前所在的地圖。
func (w *World) CurrentMap() *Map {
	if w.MapIndex < 0 || w.MapIndex >= len(w.Maps) {
		return nil
	}
	return &w.Maps[w.MapIndex]
}

// EventSegment 找出目前地圖對應的事件段。
func (w *World) EventSegment() *events.Segment {
	for i := range w.Events {
		if w.Events[i].Index == w.MapIndex {
			return &w.Events[i]
		}
	}
	return nil
}

// EventAt 回傳 (x, y) 的事件記錄，沒有就回 nil。
func (w *World) EventAt(x, y int) *events.Event {
	seg := w.EventSegment()
	c := Cell(x, y)
	if seg == nil || c < 0 {
		return nil
	}
	for i := range seg.Events {
		if int(seg.Events[i].Cell) == c {
			return &seg.Events[i]
		}
	}
	return nil
}

// Move 依目前朝向前進（step=1）或後退（step=-1）一格。
// 撞牆或走出地圖邊界就原地不動。回傳是否真的移動了。
func (w *World) Move(step int) bool {
	// 原版在更新座標之後就把條件旗標清掉。
	w.Flag = false
	f := w.Face
	if step < 0 {
		f = Facing((int(f) + 2) & 3) // 後退看的是背後那面牆
	}
	m := w.CurrentMap()
	if m == nil || !m.CanMove(w.X, w.Y, f) {
		return false
	}
	dx, dy := f.Delta()
	w.X, w.Y = w.X+dx, w.Y+dy
	w.Trigger()
	return true
}

// Turn 左轉（-1）或右轉（+1）。
func (w *World) Turn(dir int) {
	w.Face = Facing((int(w.Face) + dir + 4) & 3)
}

// 顯示字串的兩個 opcode。兩者都是「讀下一個位元組當序號 N，顯示第 N 條
// 字串」，差別只在版面：
//
//	01 NN  sub_1905E  → loc_16E79，靠左
//	02 NN  sub_19074  → loc_16EFD，開一個 0x26×0x16 的視窗
//	04 NN  sub_19110  → loc_16F52，先算長度再置中（`sub cx, 1Ch` / `neg` / `shr 1`）
//
// 兩者共用 `sub_18FD0`（把字串指標重設到區塊開頭再往後跳 N 條）與
// `sub_19016`（讀到 0xFF 為止，存進 0x54D0）。
// 對應的 handler 表見 docs/formats/07-event-script.md。
const (
	OpShowStringLeft   = 0x01
	OpShowStringWindow = 0x02
	OpShowString       = 0x04

	// OpSkipIfPaid、OpSkipIfUnpaid 是同一個旗標（`ds:042F`）的正反配對。
	// 旗標由「付款」那組函式設定：`2PLAY.img` 的 `sub_5188` 把全隊的黃金
	// 加總、夠付就扣掉並設旗標；另外兩支同型的處理寶石與食物。
	//
	//	0x10 N  付款成功就跳過 N 個 opcode（後面那 N 個是失敗分支）
	//	0x11 N  付款失敗就跳過 N 個 opcode（後面那 N 個是成功分支）
	OpSkipIfPaid   = 0x10
	OpSkipIfUnpaid = 0x11

	// OpEncounter、OpEncounterPlain 擺一場固定遭遇：讀十個怪物編號寫進
	// `ds:9680`，也就是戰鬥模組解包怪物時索引的那個陣列。
	// `0x12` 多讀兩個位元組（`ds:968A` 與 `ds:0508`），`0x13` 沒有 ——
	// 這正好對得上長度表的 13 與 11。
	OpEncounter      = 0x12
	OpEncounterPlain = 0x13

	// OpConsumeEvent 把這一格的事件旗標關掉：原版是清掉當前格在事件層的
	// bit 7（`2PLAY.img` 的 `0x9990`）。出現 555 次，是最常見的 opcode ——
	// 「這件事只發生一次」。
	OpConsumeEvent = 0x14

	// OpSkipIfFlag 是條件跳躍：讀一個位元組 N，條件旗標成立就跳過 N 個 opcode。
	// handler 在 `2PLAY.img` 的 `0xa1e2`，旗標是 `ds:0509`。
	OpSkipIfFlag = 0x2b
)

// Trigger 更新 Message：踩到有事件記錄的格子就執行對應的腳本段。
//
// 原版的完整行為是跳到第 Index 段腳本、執行 50 種 opcode
// （見 docs/formats/07-event-script.md）。這裡只實作 OpShowString，
// 其餘 opcode 尚未解出，遇到就不顯示訊息而不是亂猜。
func (w *World) Trigger() {
	w.Message = ""
	ev := w.EventAt(w.X, w.Y)
	if ev == nil {
		return
	}
	seg := w.EventSegment()
	if seg == nil {
		return
	}
	idx := int(ev.Index)
	if idx < 0 || idx >= len(seg.Scripts) {
		return
	}
	w.Message = w.run(seg, seg.Scripts[idx])
}

// run 執行一段腳本，回傳要顯示的文字。
//
// 已實作：三個顯示字串的 opcode，以及條件跳躍 `0x2b`。
// 其餘靠 opLen 跳過 —— 所以一段腳本裡「先做別的事、後面才顯示訊息」
// 的情形也讀得到。長度未知的 opcode 才會中斷：再往下走就是把參數當指令解釋。
func (w *World) run(seg *events.Segment, script []byte) string {
	var msg []string
	for p := 0; p < len(script); {
		op := script[p]
		n := OpLen(op)
		if n < 1 || p+n > len(script) {
			break
		}
		switch op {
		case OpShowStringLeft, OpShowStringWindow, OpShowString:
			if i := int(script[p+1]) - 1; i >= 0 && i < len(seg.Strings) {
				msg = append(msg, seg.Strings[i])
			}
		case OpSkipIfFlag:
			// `2b N`：條件旗標成立就跳過接下來 N 個 opcode。
			// 342 段腳本以它開頭 —— 多半是「這件事這一趟已經處理過了」。
			if w.Flag {
				p = skipOps(script, p+n, int(script[p+1]))
				continue
			}
		case OpSkipIfPaid:
			if w.Paid {
				p = skipOps(script, p+n, int(script[p+1]))
				continue
			}
		case OpSkipIfUnpaid:
			if !w.Paid {
				p = skipOps(script, p+n, int(script[p+1]))
				continue
			}
		case OpEncounter, OpEncounterPlain:
			w.Encounter = nil
			for i := 1; i <= 10 && p+i < len(script); i++ {
				if id := int(script[p+i]); id != 0 {
					w.Encounter = append(w.Encounter, id)
				}
			}
		case OpConsumeEvent:
			w.ConsumeEvent()
		}
		p += n
	}
	return strings.Join(msg, "\n")
}

// ConsumeEvent 把目前這一格的事件旗標關掉，之後再走過來就不會再觸發。
func (w *World) ConsumeEvent() {
	m := w.CurrentMap()
	if m == nil {
		return
	}
	if c := Cell(w.X, w.Y); c >= 0 {
		m.Attr[c] &^= AttrHasEvent
	}
}

// skipOps 從 p 開始跳過 count 個 opcode，回傳新的位置。
// 對應原版的 `sub_18F64`：它也是逐個查長度表前進。
func skipOps(script []byte, p, count int) int {
	for i := 0; i < count && p < len(script); i++ {
		n := OpLen(script[p])
		if n < 1 {
			return len(script)
		}
		p += n
	}
	return p
}

// StartMiddlegate 是 Middlegate 的暫定起始位置。
//
// **未定案**：真正的起點要用原版截圖對照，在那之前這個值只當測試的預設位置。
// 已知確定的是神廟在 (7,6) —— 事件表 Index=4 的格 103，
// 手冊的城鎮地圖也把神廟標在同一格（見 docs/formats/06-map.md §5）。
// 從這裡面南走兩步就會到。
var StartMiddlegate = struct {
	Map, X, Y int
	Face      Facing
}{0, 7, 8, South}
