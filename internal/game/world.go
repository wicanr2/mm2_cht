// Package game 是遊戲規則層：地圖、隊伍位置、移動、事件觸發。
//
// 這一層不碰畫面與輸入，只處理狀態轉移，才能用固定輸入寫確定性測試。
// 依賴方向是單向的：platform/UI → game → assets。
package game

import (
	"fmt"

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

// Delta 回傳朝這個方向前進一格的位移。地圖的第 0 列在北邊，
// 所以往北是列號減一。
func (f Facing) Delta() (dx, dy int) {
	switch f & 3 {
	case North:
		return 0, -1
	case East:
		return 1, 0
	case South:
		return 0, 1
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

// AttrHasEvent 是屬性層的 bit3：這格有事件。
//
// 依據是事件表的 Cell 與屬性層的交集 —— 五座城的事件格 100% 都設了這個位元
// （見 docs/formats/06-map.md §2）。反向不成立：有些設了 bit3 的格子沒有
// 對應的事件記錄，所以「有 bit3」不等於「一定會觸發事件」。
const AttrHasEvent = 0x80

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
}

// NewWorld 載入地圖與事件。MAP 段 k 對應 EVENTSI 段 k
// （見 docs/formats/06-map.md §3）。
func NewWorld(mapBlob, eventBlob []byte) (*World, error) {
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

// eventSegment 找出目前地圖對應的事件段。
func (w *World) eventSegment() *events.Segment {
	for i := range w.Events {
		if w.Events[i].Index == w.MapIndex {
			return &w.Events[i]
		}
	}
	return nil
}

// EventAt 回傳 (x, y) 的事件記錄，沒有就回 nil。
func (w *World) EventAt(x, y int) *events.Event {
	seg := w.eventSegment()
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
// 走出地圖邊界就原地不動。回傳是否真的移動了。
func (w *World) Move(step int) bool {
	dx, dy := w.Face.Delta()
	nx, ny := w.X+dx*step, w.Y+dy*step
	if Cell(nx, ny) < 0 {
		return false
	}
	w.X, w.Y = nx, ny
	w.trigger()
	return true
}

// Turn 左轉（-1）或右轉（+1）。
func (w *World) Turn(dir int) {
	w.Face = Facing((int(w.Face) + dir + 4) & 3)
}

// OpShowString 是「顯示第 N 條字串」的腳本 opcode。
//
// Middlegate 的腳本段幾乎都是兩個位元組 `04 NN`，其中 NN 與該事件的
// Index 相同 —— 所以「Index 當字串序號」這個近似會得到正確結果，
// 但走的是碰巧對上的路。這裡改成真的跑腳本。
const OpShowString = 0x04

// trigger 更新 Message：踩到有事件記錄的格子就執行對應的腳本段。
//
// 原版的完整行為是跳到第 Index 段腳本、執行 50 種 opcode
// （見 docs/formats/07-event-script.md）。這裡只實作 OpShowString，
// 其餘 opcode 尚未解出，遇到就不顯示訊息而不是亂猜。
func (w *World) trigger() {
	w.Message = ""
	ev := w.EventAt(w.X, w.Y)
	if ev == nil {
		return
	}
	seg := w.eventSegment()
	if seg == nil {
		return
	}
	idx := int(ev.Index)
	if idx < 0 || idx >= len(seg.Scripts) {
		return
	}
	script := seg.Scripts[idx]
	if len(script) >= 2 && script[0] == OpShowString {
		if n := int(script[1]) - 1; n >= 0 && n < len(seg.Strings) {
			w.Message = seg.Strings[n]
		}
	}
}

// StartMiddlegate 是原版的起始位置。
//
// 推導方式：原版從起點面北走四步會進神殿，而 Gateway Temple 的事件在
// 格 103 = (7,6)，往南四格就是 (7,10)。從這裡走四步確實會顯示「門戶神殿」，
// 與原版一致（見 docs/playtest/01）。
var StartMiddlegate = struct {
	Map, X, Y int
	Face      Facing
}{0, 7, 10, North}
