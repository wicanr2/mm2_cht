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

// trigger 更新 Message：踩到有事件記錄的格子就顯示對應字串。
//
// 事件記錄的 Index 欄位語意未定（見 docs/formats/02-data-files.md §4），
// 目前把它當成字串序號用：原版的 sub_18FD0 就是「從目前位置往後跳過
// N 個 0xFF」，與序號的行為一致。等級：假設待驗 —— 要拿原版逐格對照。
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
	if i := int(ev.Index) - 1; i >= 0 && i < len(seg.Strings) {
		w.Message = seg.Strings[i]
	}
}
