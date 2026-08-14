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

// MapCount 是 MAP.DAT 裡的地圖張數。
const MapCount = 60

// Map 是一張 16×16 的地圖。兩層各 256 bytes：地形層與屬性層。
type Map struct {
	Index   int
	Terrain [MapCells]byte
	Attr    [MapCells]byte

	// Indoor 標記這張圖走室內的通行模型（城鎮與地城）。
	//
	// 原版在 `sub_5E68` 依 `ds:039D` 分兩條路：室內用「每方向 2 位元」
	// 判牆與門，室外走另一套（5 位元碼查 `ds:52B2` 的 32 項表分成 5 類）。
	// 判準用 `ATTRIB.DAT` 的 `+18`（撞門難度）非零 —— 野外沒有門。
	//
	// 這個區分很重要：把兩者混在一起量，門的訊號會從 99.7% 掉到 91.7%。
	Indoor bool
}

// PromptKind 是事件腳本目前等著哪一種玩家輸入。
//
// 這不是 UI 的畫面模式，而是原版 `2PLAY.img` 直譯器暫停的位置：
// `0x07` 等任意確認鍵、`0x09`／`0x0a` 等 Y/N、`0x26` 選隊員、
// `0x2f` 輸入文字。把它留在 game 層，UI 才不會靠「下一步預設答 Y」
// 這類事前設定去猜腳本分支。
type PromptKind string

const (
	PromptKey    PromptKind = "key"
	PromptYesNo  PromptKind = "yes_no"
	PromptMember PromptKind = "member"
	PromptText   PromptKind = "text"
)

// EventPrompt 是一段尚未跑完的事件腳本之可存檔續跑點。
//
// Segment／Script／Offset 都保留原始資料的定位：Offset 是下一條要讀的
// opcode，不是輸入 opcode 自己。其餘欄位是該 opcode 之前已產生、但尚未
// 交給 Session 結算的狀態；讀檔時不能從腳本開頭重跑，否則付款、傳送或
// ConsumeEvent 會再做一次。
//
// 已證實：0x07、0x09、0x0a、0x26、0x2f 都由原版在讀鍵後才往下一條
// opcode 前進（2PLAY.img 的各 handler；見 docs/formats/07-event-script.md）。
type EventPrompt struct {
	Kind    PromptKind `json:"kind"`
	Segment int        `json:"segment"`
	Script  int        `json:"script"`
	Offset  int        `json:"offset"`

	Message    string       `json:"message,omitempty"`
	Result     byte         `json:"result"`
	Selected   int          `json:"selected"`
	TextExpect string       `json:"text_expect,omitempty"`
	Encounter  []int        `json:"encounter,omitempty"`
	Reward     Reward       `json:"reward"`
	Facility   FacilityKind `json:"facility"`
	Sound      int          `json:"sound"`
	Picture    int          `json:"picture"`
	Teleported bool         `json:"teleported"`
	Time       int          `json:"time"`
}

// ParseMaps 解開 MAP.DAT 的全部 60 張地圖。
func ParseMaps(blob []byte) ([]Map, error) {
	const count = MapCount
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

	// 當前地圖兩層的改寫紀錄，見 patchAttr。不進存檔 ——
	// 原版讀檔後也會重讀地圖。
	patchMap   int
	patchInit  bool
	terrainOld map[int]byte
	attrOld    map[int]byte

	MapIndex int
	X, Y     int
	Face     Facing

	// Message 是最近一次觸發的事件文字，空字串表示沒有。
	Message string
	// MessageSegment 是 Message 最後一段原文的來源段號；腳本庫文字不能
	// 用目前地圖的翻譯表查，否則會安靜退回英文。-1 表示尚無來源。
	// 這是 UI 暫態，不納入 State；Pending 已保留同一份原始段號。
	MessageSegment int
	// MessageWait 表示目前腳本停在需要玩家輸入的位置。
	//
	// 顯示字串本身不一定會攔住移動：城鎮招牌是單獨的 `04 NN`，原版會
	// 顯示名稱但仍接受下一個方向鍵。`Pending` 才是輸入閘門；這個旗標
	// 保留給既有呼叫端辨別招牌與可互動訊息，不必靠英文文字或座標猜。
	MessageWait bool
	// Pending 是尚未完成的原版事件輸入；nil 表示腳本已跑到結尾。
	// 它可直接寫進 remake 的 State，讓讀檔後從相同 opcode 續跑。
	Pending *EventPrompt

	// Encounter 是腳本擺出來的固定遭遇（怪物編號），空的表示沒有。
	// 由 opcode `0x12`／`0x13` 設定，呼叫端取走之後要自己清掉。
	Encounter []int

	// Result 是原版的 `ds:042F` —— 腳本的「上一個判斷結果」。
	//
	// 它不只給付款用：`0x15`（測角色欄位）、`0x16`（隊伍有沒有某物品）、
	// `0x32`（交給 root 判斷）都把結果 OR 進來，而 `0x10`／`0x11`
	// 讀它決定跳不跳。所以是同一個位元組，不是三個不同的旗標。
	//
	Result byte

	// Unhandled 記下直譯器沒有分支的 opcode 與次數。
	//
	// 長度表是原版的，所以不認得的 opcode 不會弄壞掃描，只是被跳過 ——
	// 症狀是「那一格少做了一件事」，沒有錯誤也沒有訊息。
	// 稽核（`TestEveryOpcodeInDataIsHandled`）靠它問「文件說全部有解，
	// 程式是不是真的有」。
	Unhandled map[byte]int

	// Reward 是 opcode `0x2a` 擺好的待領獎賞，Pending 為 false 表示沒有。
	Reward Reward

	// Selected 是 opcode `0x26` 選中的隊員（1 起算，0 表示沒選），
	// 對應原版的 `ds:54BE`。「對象 9」讀的就是它。
	Selected int

	// TextExpect 是眼前這道打字謎題的正確答案，沒有謎題時是空的。
	//
	// 答案直接寫在腳本裡（`0x2f` 後面那條 `0x30` 的十個位元組），
	// 所以不必另外建表 —— 資料改了它就跟著改。
	TextExpect string

	// Facility 是這一段腳本要進的設施（opcode `0x0e`），
	// FacilityNone 表示沒有。
	Facility FacilityKind

	// Sound 是這一段腳本最後要求播放的曲子編號（opcode `0x0d`），
	// -1 表示沒有。播放本身由上層決定。
	Sound int

	// Picture 是這一段腳本要顯示的 `monsters.16` 圖號（opcode `0x0b`），
	// 0 表示沒有。畫出來由上層決定。
	Picture int

	// Scene 是場景碼（`ds:039C`），決定 `0x0b` 查哪一張圖號表。
	// 原版由 `sub_1B1D4` 從 `ATTRIB.DAT` 的 `+4` 算出來，而室內圖的
	// `+4` 低 nibble 全是 0 —— 城鎮與地城因此都走第 0 張表。
	// 完整的換算還沒解，預設 0。
	Scene int

	// Time 是 `ds:03C8`：opcode `0x2c` 每次把它加上一個值。
	// 單位未定（日？），`ds:03CA`（世紀）在它隔壁。
	Time int

	// Globals 是遊戲的全域變數，key 是 DGROUP 位址。
	//
	// 腳本用選擇器指名（`globalAddr`），最重要的是 `0x00`–`0x17`：
	// `ds:03F6` 起連續 24 個位元組的劇情旗標。存檔要一起存。
	Globals map[uint16]byte

	// Explored 是走過哪些格。原版沒有這個東西（見 explored.go），
	// 是為了取代紙本地圖而加的，只記玩家親自到過的格。
	Explored Explored

	// Rand 是腳本要擲骰時用的亂數（`0x0c` 的隨機傳送、`0x1c`）。
	// `Session` 建立時接上；沒接的話那幾條隨機分支不執行。
	Rand *Rand

	// Teleported 記錄這一段腳本有沒有把隊伍送走。呼叫端據此知道
	// 位置變了、要重新讀地圖。
	Teleported bool

	// Neighbor 是每張地圖四個方向的鄰接地圖（依 Facing 的順序）。
	// `Session.UseAttrs` 從 `ATTRIB.DAT` 填進來；沒填的話走到邊界就是走不動。
	Neighbor [][4]int

	// Party 是腳本要讀寫角色欄位時用的隊伍。`Session` 建立時接上，
	// 兩邊共用同一個底層陣列 —— 腳本改的就是隊伍實際的資料。
	// 沒接上時 `0x15`／`0x16`／`0x18` 什麼都不做。
	Party []Character

	// Flag 是事件腳本的條件旗標（原版的 `ds:0509`）。
	//
	// **每次移動都會清掉**（`2PLAY.img` 的 `0x4300` 那段，緊接在更新
	// 隊伍座標之後），而**設定它的是戰鬥模組**（`2COMBAT.img` 的
	// `0x9C03`／`0xA64E`／`0xA682`）。所以它的意思是
	// 「自從上一次移動之後打過架」。
	//
	// opcode `0x2b` 讀它決定要不要跳過後面幾個 opcode ——
	// 342 段腳本以它開頭，多半是「已經打過了就別再演一次開場」。
	Flag bool

	// testPromptScript 只讓 RunScriptForTest 的暫停腳本可續跑；真實事件
	// 一律由 EventPrompt 的原始段號／腳本號重新取得，不能靠這個暫存。
	testPromptScript []byte
	// textAnswer 是剛由 ResumeText 交出的答案，只存到下一個 0x30 比對完。
	// 它不是遊戲狀態，因為玩家尚未輸入時才會進存檔中的 PromptText。
	textAnswer string
}

// ClearTravelEffects 清除原版休息／換圖生命週期會一起清掉的暫時效果。
//
// `2MISC` 的 `sub_1CD8A` 會把 `ds:03D5`–`ds:03E1` 整段寫成 0；其中
// `ds:03D9` 是水行術（`2CAST1` 的 `sub_1C8C8`）。這裡保留 DGROUP 位址
// 原樣，不把任何欄位改名成船、游泳或其他未證實語意。
func (w *World) ClearTravelEffects() {
	if w.Globals == nil {
		return
	}
	for addr := uint16(0x03D5); addr <= 0x03E1; addr++ {
		w.Globals[addr] = 0
	}
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
	return &World{Maps: maps, Events: segs, Face: North, MessageSegment: -1}, nil
}

// CurrentMap 回傳目前所在的地圖。
func (w *World) CurrentMap() *Map {
	if w.MapIndex < 0 || w.MapIndex >= len(w.Maps) {
		return nil
	}
	switch {
	case !w.patchInit:
		w.patchMap, w.patchInit = w.MapIndex, true
	case w.patchMap != w.MapIndex:
		w.restorePatch()
		w.patchMap = w.MapIndex
	}
	return &w.Maps[w.MapIndex]
}

// patchTerrain／patchAttr 改寫當前地圖的一格，並記下改寫前的原值。
//
// **原版的地形層與屬性層只有當前地圖一份**（`ds:59D6`／`ds:5AD6`，各 256
// bytes），`2PLAY sub_1BE24` 在地圖編號改變時整層從 `MAP.DAT` 重讀 ——
// 所以撞開的門、用掉的事件、腳本改過的格，**離開那張圖就全部回到檔案裡的
// 樣子**。見 docs/formats/06-map.md「開門狀態的生存期」。
//
// remake 把六十張圖都留在記憶體裡，沒有「只有一份」這個天然限制，
// 所以改用「離開時還原」達到同一個效果。持久的走訪紀錄是另一份
// （原版 `ds:968C` 的位元陣列，remake 在 explored.go），兩者不要混在一起。
func (w *World) patchTerrain(m *Map, c int, v byte) {
	if c < 0 || c >= MapCells {
		return
	}
	if w.terrainOld == nil {
		w.terrainOld = map[int]byte{}
	}
	if _, ok := w.terrainOld[c]; !ok {
		w.terrainOld[c] = m.Terrain[c]
	}
	m.Terrain[c] = v
}

func (w *World) patchAttr(m *Map, c int, v byte) {
	if c < 0 || c >= MapCells {
		return
	}
	if w.attrOld == nil {
		w.attrOld = map[int]byte{}
	}
	if _, ok := w.attrOld[c]; !ok {
		w.attrOld[c] = m.Attr[c]
	}
	m.Attr[c] = v
}

// restorePatch 把 patchMap 那張圖的兩層還原成 MAP.DAT 裡的樣子。
func (w *World) restorePatch() {
	if w.patchMap >= 0 && w.patchMap < len(w.Maps) {
		m := &w.Maps[w.patchMap]
		for c, v := range w.terrainOld {
			m.Terrain[c] = v
		}
		for c, v := range w.attrOld {
			m.Attr[c] = v
		}
	}
	w.terrainOld, w.attrOld = nil, nil
}

// OpenDoor 把 (x, y) 朝 f 那一面的門打開。
//
// 原版是 root `sub_13A64`，overlay 走 thunk `0x1765A`；唯二呼叫點是
// `2MISC` 的撞門成功（`0x1C20F`）與開鎖成功（`0x1C326`）：
//
//	al  = 地形層[格] & 方向遮罩      ; 遮罩是那個方向的兩個位元
//	al >>= 1                        ; 門的值 2 移成牆位元的 1
//	al ^= 當前格的屬性位元組
//	屬性層[格] = al                  ; 該方向的牆位元被翻掉
//
// 所以「開門」改的是**屬性層的牆位元**，地形層那兩個位元不動 ——
// 門還是門（`DrawKind` 照樣畫門），只是不再擋路。
func (w *World) OpenDoor(x, y int, f Facing) {
	m := w.CurrentMap()
	c := Cell(x, y)
	if m == nil || c < 0 {
		return
	}
	bit := wallBit[f&3]
	w.patchAttr(m, c, m.Attr[c]^(m.Terrain[c]&(3<<bit))>>1)
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

// libraryScriptForFacility 解析 `0e NN` 的特殊設施轉派。一般設施由
// FacilityByCode 留給 Session 開選單；其餘已證實的代碼會切換到沒有事件
// 表的腳本庫段。回傳的腳本索引已是 events.Segment.Scripts 的零起算索引。
func (w *World) libraryScriptForFacility(code int) (*events.Segment, int, []byte, bool) {
	segment, script, ok := LibraryScriptForFacility(code)
	if !ok {
		return nil, 0, nil, false
	}
	for i := range w.Events {
		seg := &w.Events[i]
		if seg.Index != segment || !seg.Library {
			continue
		}
		if script < 0 || script >= len(seg.Scripts) {
			return nil, 0, nil, false
		}
		return seg, script, seg.Scripts[script], true
	}
	return nil, 0, nil, false
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
	if m == nil {
		return false
	}
	dx, dy := f.Delta()
	nx, ny := w.X+dx, w.Y+dy
	if nx < 0 || nx >= MapW || ny < 0 || ny >= MapH {
		// 室內圖的外圈也有牆。先檢查來源格的牆，再決定能否走
		// crossEdge；否則 Middlegate 的西牆會被「出界就換圖」捷徑繞過。
		// CanMove 對出界本身回 false，所以只有這條邊界分支要先取牆。
		if m.Indoor && !m.CanMove(w.X, w.Y, f) {
			return false
		}
		return w.crossEdge(f, nx, ny)
	}
	if !m.CanMove(w.X, w.Y, f) {
		return false
	}
	w.X, w.Y = nx, ny
	w.Trigger()
	return true
}

// MarkExplored 記下目前這一格走過了。
func (w *World) MarkExplored() {
	if w.Explored == nil {
		w.Explored = Explored{}
	}
	w.Explored.Mark(w.MapIndex, w.X, w.Y)
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

	// 另外三個也是顯示字串，差別只在版面：
	//
	//	03 NN  sub_190F2  設 ds:0430 |= 3 之後轉呼叫 0x02 的 handler
	//	05 NN  sub_19160  自己取字串（sub_18FD0 + sub_19016）再畫
	//	06 NN  sub_191EC  開一個 0x12×9 的方框，把字串裡的 '-' 換成
	//	                  框線字元（`0x2D` → `0x7B`）再畫進去
	//
	// 六個一起認之後，會顯示訊息的事件格從 57.0% 升到 69.5%。
	OpShowStringBoxed   = 0x06
	OpShowStringPlain   = 0x05
	OpShowStringWindow2 = 0x03

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

	// OpTestField、OpSetField 是同一支程式（`sub_19A02`）的兩個入口，
	// 差別只在要不要寫回：
	//
	//	15 對象 欄位 遮罩        讀 → `結果 |= 值 & 遮罩`（遮罩 0 時不遮）
	//	18 對象 欄位 遮罩 新值   寫 → `欄位 = 欄位 & 遮罩 | 新值`
	//
	// 「對象」的規則（原版 `sub_19A02` 前段）：
	//
	//	bit 7 設起來 → 寫入值改用**進來時的 `ds:042F`**，不是參數
	//	低 7 位元 0  → 全隊，由最後一人往前
	//	低 7 位元 9  → 前一個判斷選中的那一位
	//	超過隊伍人數 → 改成第 1 人
	//
	// 「欄位」是選擇器，經 `sub_1AA00` 的 128 項跳表換成角色記錄的偏移，
	// 見 `data/fields.json`。
	OpTestField = 0x15
	OpSetField  = 0x18

	// OpHasItem 檢查隊伍裡有沒有人帶著某件物品（`sub_19ABC`）：
	// 逐人掃已裝備的六格（記錄 +40）與背包的六格（+58），
	// 命中就把數量加進 `ds:042F` 並停止。讀兩個參數，只用第二個 ——
	// 原版把第一個讀進同一個區域變數又立刻覆寫掉。
	//
	// 這同時反向印證了物品區的排法：`+0x28` 與 `+0x3A` 各六格。
	OpHasItem = 0x16

	// OpWaitKey 等玩家按鍵（`sub_193B8`，出現 226 次）。
	// 它是訊息的分頁點 —— remake 一次顯示整段，所以只當段落界線。
	OpWaitKey = 0x07

	// OpTeleport 換地圖並移動到指定座標（`sub_194D4`，出現 212 次）：
	//
	//	0c 目標 座標
	//	目標 & 0x40 → 目標 = rand(1,20) + 5；≥ 0x11 再加 0x10，然後 |= 0x80
	//	目標 ≥ 0x80 → 座標 = rand(1,255)
	//	地圖 = 目標 & 0x3F      座標低 nibble = X、高 nibble = Y
	//
	// X／Y 的分派不是猜的：`sub_142DE` 前進一步時把 `sub_1428C` 給的兩個
	// 增量分別加到 `ds:0393` 與 `ds:0394`，而 `sub_1428C` 對 `'N'` 給
	// (0, +1)、`'E'` 給 (+1, 0)。所以 `ds:0393` 是 X、`ds:0394` 是 Y，
	// 且**北是 +Y** —— 與第 0 列在南邊一致。
	OpTeleport = 0x0c

	// OpRoll 擲一次 `rand(1, N)` 放進結果（`sub_19C5E`）。
	OpRoll = 0x1c

	// OpAtLeast 是門檻檢查（`sub_19C40`）：結果小於 N 就清成 0。
	// 配著 `0x15`（讀欄位）用，就是「這個欄位有沒有到 N」。
	OpAtLeast = 0x1b

	// OpRedraw 只設「需要重畫」旗標（`sub_1A19A`：`ds:0395 = 1`），
	// 出現 206 次。remake 每一格都重畫，所以不必做事。
	OpRedraw = 0x29

	// OpAdd、OpSub 對角色的某個欄位做加減（`sub_19E40(0)` 與 `(1)`）：
	//
	//	1f 對象 欄位 模式 值低 值中 值高    加，飽和在該欄位的寬度上限
	//	20 對象 欄位 模式 值低 值中 值高    減，不夠扣就設 0 並把結果清成 0
	//
	// 值是 **3 個位元組的小端序**（`sub_18DF8` 讀兩個再補一個高位），
	// 依欄位寬度（1／2／4）截斷。對象的 bit 7 設起來時改用進來時的結果
	// 當運算元，那三個位元組跳過。
	//
	// `0x20` 扣不動時把結果清成 0 —— 那正是 `0x10`／`0x11` 讀的「付款成功
	// 與否」。付款不是另一組 opcode，就是這一個。
	OpAdd = 0x1f
	OpSub = 0x20

	// OpAsk 問玩家 Y／N（`sub_1941E`），答 `Y` 才把結果設成 1。
	// 出現 203 次，配 `0x10`／`0x11` 用。
	OpAsk = 0x09

	// OpAsk2 也是問 Y／N（`sub_1946E`）。原版與 `0x09` 差在取鍵的路徑
	// （它先把讀取模式設成 `0FDh` 再呼叫 `sub_1941E(1)`），
	// 對腳本的效果相同：答 `Y` 把結果設成 1。
	OpAsk2 = 0x0a

	// OpGiveItem 給隊伍一件物品（`sub_19B44`）：
	//
	//	19 對象 編號 次數 屬性
	//
	// 逐人掃背包六格（記錄 `+58`），找到第一個空的就把三個值分別寫進
	// `+58`（編號）、`+64`（可用次數）、`+70`（屬性），並把結果設成 1。
	// 對象 ≥ `0x80` 時**編號改從結果暫存器取**，與 `0x18` 同一個約定。
	// 全隊背包都滿就掉在地上（`ds:6950` 那兩格，並把 `ds:0434` 設成
	// `0FFh` —— 那正是 `0x2a` 的 `Treasure!` 領取路徑）。
	OpGiveItem = 0x19

	// OpScriptLoop 把直譯器的讀取模式切成 `0FDh`（`sub_1940E`），
	// 在那個模式下終止碼 `0FFh` 變成「位置歸零重讀」。
	//
	// **remake 不實作**：它改的是主迴圈的讀取行為，要動整條迴圈才有意義，
	// 而 remake 的腳本是一次讀完的切片。列在這裡是為了讓
	// 「認得但不做」與「不認得」分得開。
	OpScriptLoop = 0x08

	// OpSound 播放第 N 首曲子（`sub_19560` → root `sub_57E0`）。
	// 十首曲子的指標表在 `ds:5214`，音高表 `ds:5144`、時值表 `ds:51F4`，
	// 曲子以 `0xFF` 收尾。
	OpSound = 0x0d

	// OpShowPicture 顯示一張 `monsters.16` 的圖（`sub_1947E`）：
	// 第一個參數經 `sub_18EE6` 換成圖號 —— 參數 ≥ 0x80 一律是 75，
	// 否則參數減一後依場景碼查表。那些圖號落在高號段，正是房舍、
	// 城堡、店家、寶箱那幾張。
	OpShowPicture = 0x0b

	// OpRedrawView 設「需要重畫」旗標再重畫視圖（`sub_198C8`）。
	// 出現 278 次，remake 每一格都重畫，所以不必做事。
	OpRedrawView = 0x0f

	// OpTakeItem 從隊伍的背包裡拿走一件物品（`sub_1A126`）：
	// 逐人掃背包六格（記錄 `+58`），找到就用 root 的 `sub_13766`
	// 把那一格移掉、後面往前補，並把結果加一。
	//
	// 與 `0x16` 的差別是 `0x16` 只看不拿，而且 `0x16` 連已裝備的也算。
	OpTakeItem = 0x28

	// OpAdvanceTime 讓時間前進：`ds:03C8 += N`（`sub_1A202`）。
	// `ds:03CA`（世紀）就在它隔壁兩個位元組。
	OpAdvanceTime = 0x2c

	// OpPause 是可被按鍵中斷的停頓。remake 一次顯示整段，所以不做事。
	//
	//	1d N  停 (7N+1) × 2 個單位（`sub_19C72` → root `sub_14EFE`）
	//	1e N  停 N 次，每次 10 個單位（`sub_19C8A`）
	OpPauseScaled = 0x1d
	OpPauseCount  = 0x1e

	// OpReadGlobal、OpWriteGlobal 讀寫遊戲的全域變數（`sub_19B20` 與
	// `sub_19C1A`），選擇器經 `sub_18E22` 換成 DGROUP 位址：
	//
	//	17 選擇器 ??   結果 = *全域[選擇器]（第二個參數讀了沒用）
	//	1a 選擇器 值   *全域[選擇器] = 值
	//
	// 選擇器 0x00–0x17 是連續的 24 個位元組（`ds:03F6` 起）——
	// 那是全遊戲的劇情旗標區，與角色記錄尾端那 12 bytes 各管一半。
	OpReadGlobal  = 0x17
	OpWriteGlobal = 0x1a

	// OpInRange 檢查 `ds:03CA` 落不落在 [下限, 上限]（`sub_19F90`）。
	// 那個全域也可以用選擇器 `0x84` 讀寫。
	OpInRange = 0x22

	// OpFacility 進入城鎮設施（`sub_19716`）：讀一個子命令，
	// 1–6 分別是旅店、訓練基地、酒館、神殿、法師公會、鐵匠。
	// 對照關係見 `FacilityByCode`。
	OpFacility = 0x0e

	// OpPayGold 與 OpPayGems 是「全隊湊錢」（`sub_1A01E` → `sub_15188`
	// 與 `sub_1A04C` → `sub_15262`，各長 3 個位元組）。
	//
	// 運算元是 16 位元的金額（低位在前）。程序先把 `ds:042F` 清成 0，
	// 掃過 `ds:0426` 個隊伍成員把該欄位加總 —— 金錢是 `+102`（32 位元）、
	// 寶石是 `+92`（16 位元）—— 湊得出就依序扣掉並設 `ds:042F = 1`，
	// 湊不出就維持 0 且**一毛都不扣**。
	//
	// 掃描時跳過 `ds:0416[i] >= 24` 的位置：那一排是每個隊伍位置對應的
	// 名冊索引，名冊只有 24 筆，超出就是空位。
	OpPayGold = 0x24
	OpPayGems = 0x25

	// OpHasMember 問「隊伍裡有沒有符合條件的人」（`sub_1A21E`，長 3）。
	//
	// 第一個運算元同時帶旗標與值：
	//
	//	bit 7 → 比種族（記錄 +14）
	//	bit 6 → 比性別（記錄 +12）
	//	bit 5 → 比職業（記錄 +15）
	//	低 nibble → 要比的值
	//
	// 三個旗標可以同時成立，**任何一項對上就算數**。結果寫進 `ds:042F`
	// （程序開頭先清成 0）。第二個運算元只在三個旗標全為 0 時才被讀，
	// 那條路徑上三個比較都不會執行 —— 照抄但不特別處理。
	OpHasMember = 0x2d

	// OpTeachSpell 教會某一系的隊員一個法術（`sub_1A386`，長 3）。
	//
	//	運算元 1 bit 7 → 牧師系（職業 3 牧師、1 聖騎士）
	//	          否則 → 巫師系（職業 4 巫師、2 弓箭手）
	//	運算元 1 低七位 → 要寫哪一個位元組：記錄 `+81 + (值 - 110)`
	//	運算元 2 → 要 OR 進去的位元遮罩
	//
	// 原版的算式是 `值 - 6Eh + 51h`，也就是 `值 - 29`；`值 = 110` 時
	// 正好落在 `+81`，那是會不會某條法術的位元圖。
	//
	// 兩個職業一組這件事與施法能力對得上：巫師與弓箭手共用巫師系、
	// 牧師與聖騎士共用牧師系。
	OpTeachSpell = 0x2e

	// OpHarm 對隊員造成傷害（`sub_1A4BC` → `sub_13928`，長 4）。
	//
	//	運算元 1 → 對象（同 scriptTargets 的約定）
	//	           bit 7 表示「傷害改用 ds:042F 的值」
	//	運算元 2–3 → 傷害（word）
	//
	// `sub_13928` 的前段是抗性判定：狀況 `>= 80h`（重症）直接跳過；
	// 否則擲 `rand(1, 100)`，**擲值小於 `記錄 +22` 加上 `ds:03D6`
	// 就完全擋下**。那個加總正好是「抗性百分比 ＋ 全隊抗性加成」——
	// `+22` 是抗性這件事因此有了第二個獨立的用處（第一個是欄位表）。
	OpHarm = 0x31

	// OpSetReward 擺好一份待領的獎賞（`sub_1A1A0`，長 **15**）。
	//
	//	3 bytes → 金錢（與 0x1f／0x20 同一種 3 位元組小端序）→ ds:695C
	//	2 bytes → 寶石 → ds:695A
	//	3 × 3 bytes → 三件物品的編號／兩個附屬欄 → ds:6950／6956／6953
	//	最後 ds:0434 = 0FFh 標成「有東西待領」
	//
	// 三件物品用三個平行陣列存，與角色記錄的背包（`+58`／`+64`／`+70`）
	// 同一種排法。真正發給隊伍的程式碼在別處，由 `ds:0434` 觸發。
	OpSetReward = 0x2a

	// OpDateCond 判斷今天符不符合日期條件（`loc_19FC6`，長 3）。
	//
	//	今天 = ds:03A2[ds:03CA × 2]
	//	運算元 1 == 0B5h → 單數日才成立
	//	運算元 1 == 0B6h → 偶數日才成立
	//	否則             → 今天落在閉區間 [運算元 1, 運算元 2] 才成立
	//
	// 成立就 `ds:042F = 1`（程序開頭先清 0）。
	OpDateCond = 0x23

	// OpSetCell 改寫某一格的兩個平面（`loc_19F44`，長 4）。
	//
	//	運算元 1 → 格子索引（低 nibble X、高 nibble Y，與其他座標同一種打包）
	//	運算元 2 → 寫進 ds:59D6[格] ＝ Terrain
	//	運算元 3 → 寫進 ds:5AD6[格] ＝ Attr
	//	ds:0430 |= 1                 ; 標記地圖被改過
	//
	// `ds:59D6` 與 `ds:5AD6` 相隔 0x100，正是 `MAP.DAT` 一張圖的兩個
	// 256 bytes 平面依序載入的結果 —— 所以第一個運算元對 Terrain、
	// 第二個對 Attr。**這是牆會在遊戲中改變的機制**（暗門之類）。
	OpSetCell = 0x21

	// OpPickMember 請玩家選一名隊員（`sub_1A082`，長 1）。
	//
	// 迴圈讀按鍵直到選到為止：`1`–`9` 對應隊員（減一之後是索引），
	// 超出人數的重來，狀況 `>= 81h`（死亡／石化那一類）也重來，
	// `ESC`（`1Bh`）取消。選中的人存進 `ds:54BE` ——
	// **就是「對象 9」讀的那一格**。
	OpPickMember = 0x26

	// OpAskText 讓玩家輸入一串字（`sub_1A404`）。
	//
	// 緩衝區是 `ds:54C4`，十個位元組（`sub_16EE6(54C4h, 10)`），
	// 讀到空的就重來。**它與 `0x30` 之間誰是題目、誰是答案還沒定** ——
	// 引擎只把它當成「準備比對」，實際字串交給 TextAnswer。
	OpAskText = 0x2f

	// OpMatchText 把腳本裡接著的**十個位元組**與玩家輸入的字串比對，
	// 結果寫進 `ds:042F`（`sub_1A45A`）。
	//
	// 十個字元逐一比，全部相同才 `ds:042F = 1`，任何一個不同就是 0。
	// 答案內嵌在腳本裡 —— 這條 opcode 長 11 個位元組，是全表最長的一條。
	OpMatchText = 0x30

	// OpCountSkill 數隊伍裡具備某項第二技能的人數（`sub_1A570` →
	// root `sub_36A6`），結果進 `ds:042F`。
	//
	// `sub_36A6` 就是野外通行判定用的那一支：山區要 `0x0B`（登山家）
	// 兩人、森林要 `0x0D`（探險家）兩人。腳本用它問「隊伍裡有沒有人會 X」。
	OpCountSkill = 0x32

	// OpSkipIfFlag 是條件跳躍：讀一個位元組 N，條件旗標成立就跳過 N 個 opcode。
	// handler 在 `2PLAY.img` 的 `0xa1e2`，旗標是 `ds:0509`。
	OpSkipIfFlag = 0x2b
)

// Trigger 更新目前格的暫時事件輸出；踩到有事件記錄的格子就執行對應腳本段。
//
// 原版會跳到第 Index 段腳本、執行 50 種 opcode（見
// docs/formats/07-event-script.md）。直譯器依原始長度表前進；遇到原版
// 會讀玩家輸入的 opcode 時，保存 EventPrompt 並停在下一條 opcode 前。
func (w *World) Trigger() {
	// 有事件輸入尚未完成時，原版不會接受新的移動再覆蓋它。
	if w.Pending != nil {
		return
	}
	// 每次位置改變都會經過這裡（走路、跨圖、傳送、腳本搬人），
	// 所以探索記錄放在這裡才不會漏掉某一條路徑。
	w.MarkExplored()
	w.Message = ""
	w.MessageSegment = -1
	w.MessageWait = false
	w.TextExpect = ""
	w.testPromptScript = nil
	// 設施是「目前這一格」的腳本輸出，不是玩家離開後仍持續的狀態。
	// 若這格沒有事件，必須先清掉前一格留下的值；否則下一次正常移動會
	// 再度打開剛離開的設施選單。
	w.Facility = FacilityNone
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
	w.Teleported = false
	w.Sound = -1
	w.Picture = 0
	w.Message = w.run(seg, idx, seg.Scripts[idx], 0)
}

// run 從 start 執行一段腳本，回傳這一頁要顯示的文字。
//
// 已實作：三個顯示字串的 opcode，以及條件跳躍 `0x2b`。
// 其餘靠 opLen 跳過 —— 所以一段腳本裡「先做別的事、後面才顯示訊息」
// 的情形也讀得到。長度未知的 opcode 才會中斷：再往下走就是把參數當指令解釋。
//
// 遇到輸入 opcode 時**不能**繼續掃：後面的付款、傳送或跳躍都依玩家答案
// 而定。pause 把原始段／腳本／位移留下，Resume* 才會從那個精確位置續跑。
func (w *World) run(seg *events.Segment, scriptIndex int, script []byte, start int) string {
	return w.runWithMessages(seg, scriptIndex, script, start, nil)
}

// runWithMessages 在切到腳本庫時保留前段已顯示的文字。原版 `0x0e` 的
// handler 會設直譯器停止旗標；若它是特殊設施，外層再換段執行對應的腳本
// 庫。因此不能把 `0x0e` 後面的位元組繼續當成同一段腳本執行。
func (w *World) runWithMessages(seg *events.Segment, scriptIndex int, script []byte, start int, msg []string) string {
	for p := start; p < len(script); {
		op := script[p]
		n := OpLen(op)
		if n < 1 || p+n > len(script) {
			break
		}
		switch op {
		case OpShowStringLeft, OpShowStringWindow, OpShowString,
			OpShowStringWindow2, OpShowStringPlain, OpShowStringBoxed:
			if i := int(script[p+1]) - 1; i >= 0 && i < len(seg.Strings) {
				msg = append(msg, seg.Strings[i])
				w.MessageSegment = seg.Index
			}
		case OpSkipIfFlag:
			// `2b N`：條件旗標成立就跳過接下來 N 個 opcode。
			// 342 段腳本以它開頭 —— 多半是「這件事這一趟已經處理過了」。
			if w.Flag {
				p = skipOps(script, p+n, int(script[p+1]))
				continue
			}
		case OpSkipIfPaid:
			if w.Result != 0 {
				p = skipOps(script, p+n, int(script[p+1]))
				continue
			}
		case OpSkipIfUnpaid:
			if w.Result == 0 {
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
		case OpTestField:
			w.testField(script[p+1], script[p+2], script[p+3])
		case OpSetField:
			w.setField(script[p+1], script[p+2], script[p+3], script[p+4])
		case OpHasItem:
			w.hasItem(int(script[p+2]))
		case OpTeleport:
			w.teleport(script[p+1], script[p+2])
		case OpRoll:
			if w.Rand != nil {
				w.Result = byte(w.Rand.Range(1, int(script[p+1])))
			}
		case OpAtLeast:
			if w.Result < script[p+1] {
				w.Result = 0
			}
		case OpAdd:
			w.addField(script[p+1], script[p+2], script[p+3], operand3(script[p+4:]), false)
		case OpSub:
			w.addField(script[p+1], script[p+2], script[p+3], operand3(script[p+4:]), true)
		case OpAsk, OpAsk2:
			// 原版 handler 讀到 Y／N 才會把 ds:042F 寫成 0／1；先清掉
			// 舊結果，避免後面的條件跳躍看見上一段腳本的殘值。
			w.Result = 0
			return w.pause(PromptYesNo, seg, scriptIndex, p+n, msg)
		case OpGiveItem:
			if p+5 <= len(script) {
				w.giveItem(script[p+1], script[p+2], script[p+3], script[p+4])
			}
		case OpScriptLoop:
			// 見 OpScriptLoop 的說明：認得，但 remake 的腳本模型下不做事。
		case OpSound:
			w.Sound = int(script[p+1])
		case OpFacility:
			code := int(script[p+1])
			if k := FacilityByCode(code); k != FacilityNone {
				w.Facility = k
				return strings.Join(msg, "\n")
			}
			if library, libraryIndex, libraryScript, ok := w.libraryScriptForFacility(code); ok {
				return w.runWithMessages(library, libraryIndex, libraryScript, 0, msg)
			}
			// `sub_19716` 不論代碼能否在 remake 完整實作，都會中斷目前
			// 腳本；保留這個邊界，不能把後面的資料誤當成可繼續執行。
			return strings.Join(msg, "\n")
		case OpReadGlobal:
			w.Result = w.Global(int(script[p+1]))
		case OpWriteGlobal:
			w.SetGlobal(int(script[p+1]), script[p+2])
		case OpInRange:
			lo, hi := script[p+1], script[p+2]
			w.Result = 0
			if v := w.Global(globalSelCentury); v >= lo && v <= hi {
				w.Result = 1
			}
		case OpTakeItem:
			w.takeItem(int(script[p+2]))
		case OpAdvanceTime:
			w.Time += int(script[p+1])
		case OpPauseScaled, OpPauseCount:
			// 停頓在 remake 沒有對應動作。
		case OpPayGold, OpPayGems:
			w.Result = 0
			if p+3 <= len(script) {
				amount := int(script[p+1]) | int(script[p+2])<<8
				if w.pay(script[p] == OpPayGold, amount) {
					w.Result = 1
				}
			}
		case OpHasMember:
			w.Result = 0
			if p+3 <= len(script) {
				if w.hasMember(script[p+1]) {
					w.Result = 1
				}
			}
		case OpTeachSpell:
			if p+3 <= len(script) {
				w.teachSpell(script[p+1], script[p+2])
			}
		case OpHarm:
			if p+4 <= len(script) {
				dmg := int(script[p+2]) | int(script[p+3])<<8
				if script[p+1]&0x80 != 0 {
					dmg = int(w.Result)
				}
				for _, i := range w.scriptTargets(script[p+1]) {
					w.harm(i, dmg)
				}
			}
		case OpSetReward:
			if p+15 <= len(script) {
				r := Reward{Pending: true,
					Gold: operand3(script[p+1 : p+4]),
					Gems: uint16(script[p+4]) | uint16(script[p+5])<<8}
				for i := 0; i < 3; i++ {
					b := script[p+6+i*3:]
					r.Items[i] = [3]byte{b[0], b[1], b[2]}
				}
				w.Reward = r
			}
		case OpDateCond:
			w.Result = 0
			if p+3 <= len(script) && w.dateCond(script[p+1], script[p+2]) {
				w.Result = 1
			}
		case OpSetCell:
			if p+4 <= len(script) {
				if m := w.CurrentMap(); m != nil {
					c := int(script[p+1])
					w.patchTerrain(m, c, script[p+2])
					w.patchAttr(m, c, script[p+3])
				}
			}
		case OpPickMember:
			// 原版的輸入迴圈會一直讀到合法的 1–9 或 ESC；選單層送回
			// 1 起算的成員號，0 表示 ESC 取消。
			w.Selected = 0
			return w.pause(PromptMember, seg, scriptIndex, p+n, msg)
		case OpAskText:
			// 只是把輸入準備好，狀態改變都在 0x30。
			// 順便把**正確答案**解出來 —— 它就寫在後面那條 0x30 裡。
			w.TextExpect = expectedAnswer(script, p+n)
			return w.pause(PromptText, seg, scriptIndex, p+n, msg)
		case OpMatchText:
			w.Result = 0
			if p+11 <= len(script) && textAnswerMatches(w.textAnswer, script[p+1:p+11]) {
				w.Result = 1
			}
		case OpCountSkill:
			w.Result = byte(w.countSkill(int(script[p+1])))
		case OpShowPicture:
			if data != nil {
				w.Picture = data.Pictures.Picture(w.Scene, int(script[p+1]))
			}
		case OpRedraw, OpRedrawView:
			// remake 每一格都重畫，所以不必做事。
		case OpWaitKey:
			return w.pause(PromptKey, seg, scriptIndex, p+n, msg)
		default:
			// 沒有分支的 opcode 會被安靜地跳過 —— 長度表是原版的，
			// 所以掃描不會壞，畫面上只呈現為「那一格少做了一件事」。
			// 記下來，稽核才問得出「文件說全部有解，程式是不是真的有」。
			if w.Unhandled == nil {
				w.Unhandled = map[byte]int{}
			}
			w.Unhandled[op]++
		}
		p += n
	}
	return strings.Join(msg, "\n")
}

// pause 擷取中途輸入前已經執行的狀態。這些欄位在輸入掛著時不應再變，
// 因而可以和續跑點一起寫進 State；讀檔後不必重跑前半段腳本。
func (w *World) pause(kind PromptKind, seg *events.Segment, script, offset int, msg []string) string {
	message := strings.Join(msg, "\n")
	p := &EventPrompt{
		Kind: kind, Segment: seg.Index, Script: script, Offset: offset,
		Message: message, Result: w.Result, Selected: w.Selected,
		TextExpect: w.TextExpect, Reward: w.Reward, Facility: w.Facility,
		Sound: w.Sound, Picture: w.Picture, Teleported: w.Teleported, Time: w.Time,
	}
	if len(w.Encounter) > 0 {
		p.Encounter = append([]int(nil), w.Encounter...)
	}
	w.Pending = p
	w.MessageWait = true
	return message
}

// ResumeKey 用一個確認鍵跨過 `0x07` 的分頁點。
func (w *World) ResumeKey() bool {
	return w.resume(PromptKey, func() {})
}

// ResumeYesNo 把事件的 Y／N 輸入寫入 ds:042F，接著從保存的位移續跑。
func (w *World) ResumeYesNo(yes bool) bool {
	return w.resume(PromptYesNo, func() {
		if yes {
			w.Result = 1
		} else {
			w.Result = 0
		}
	})
}

// ResumeMember 把 1 起算的隊員編號交給 `0x26`；0 等同原版 ESC。
func (w *World) ResumeMember(member int) bool {
	return w.resume(PromptMember, func() {
		w.Selected = 0
		if member >= 1 && member <= len(w.Party) {
			if c := &w.Party[member-1]; c.CondBits < CondPetrified {
				w.Selected = member
			}
		}
	})
}

// ResumeText 交出 `0x2f` 的文字輸入。比對實際發生在續跑遇到的 `0x30`；
// 這樣兩個 opcode 中間夾著其他指令時仍維持原本資料流。
func (w *World) ResumeText(answer string) bool {
	return w.resume(PromptText, func() { w.textAnswer = answer })
}

func (w *World) resume(kind PromptKind, apply func()) bool {
	p := w.Pending
	if p == nil || p.Kind != kind {
		return false
	}
	seg, script, ok := w.promptSource(p)
	if !ok {
		return false
	}
	w.Pending = nil
	w.Message = ""
	w.MessageSegment = -1
	w.MessageWait = false
	if kind == PromptText {
		w.TextExpect = ""
		defer func() { w.textAnswer = "" }()
	}
	apply()
	w.Message = w.run(seg, p.Script, script, p.Offset)
	return true
}

// promptSource 以 EventPrompt 留下的原始定位取回腳本。直接跑測試腳本時
// 沒有資料檔段號，才使用 testPromptScript；它永遠不會進入正式存檔。
func (w *World) promptSource(p *EventPrompt) (*events.Segment, []byte, bool) {
	if p.Segment < 0 {
		if len(w.testPromptScript) == 0 {
			return nil, nil, false
		}
		return &events.Segment{Index: p.Segment}, w.testPromptScript, true
	}
	for i := range w.Events {
		seg := &w.Events[i]
		if seg.Index != p.Segment {
			continue
		}
		if p.Script < 0 || p.Script >= len(seg.Scripts) {
			return nil, nil, false
		}
		return seg, seg.Scripts[p.Script], true
	}
	return nil, nil, false
}

// validPrompt 確認讀檔提供的續跑點仍指向該種類的輸入 opcode 之後。
// State 是 JSON，不能相信 Offset 恰好落在 opcode 邊界；否則手改壞的檔案
// 可能把參數誤當指令執行。
func (w *World) validPrompt(p *EventPrompt) bool {
	if p == nil || p.Segment < 0 {
		return false
	}
	_, script, ok := w.promptSource(p)
	if !ok || p.Offset < 1 || p.Offset > len(script) {
		return false
	}
	for at := 0; at < len(script); {
		n := OpLen(script[at])
		if n < 1 || at+n > len(script) {
			return false
		}
		if at+n == p.Offset {
			switch script[at] {
			case OpWaitKey:
				return p.Kind == PromptKey
			case OpAsk, OpAsk2:
				return p.Kind == PromptYesNo
			case OpPickMember:
				return p.Kind == PromptMember
			case OpAskText:
				return p.Kind == PromptText
			default:
				return false
			}
		}
		if at+n > p.Offset {
			return false
		}
		at += n
	}
	return false
}

// ConsumeEvent 把目前這一格的事件旗標關掉，之後再走過來就不會再觸發。
func (w *World) ConsumeEvent() {
	m := w.CurrentMap()
	if m == nil {
		return
	}
	if c := Cell(w.X, w.Y); c >= 0 {
		w.patchAttr(m, c, m.Attr[c]&^AttrHasEvent)
	}
}

// skipOps 從 p 開始跳過 count 個 opcode，回傳新的位置。
// 對應原版的 `sub_18F64`：它也是逐個查長度表前進。
func skipOps(script []byte, p, count int) int {
	for i := 0; i < count && p < len(script); i++ {
		// 長度 0 的只有 `0x00`，它就是結束標記 —— 原版的長度表用 0
		// 讓直譯器停下來，不需要另外的終止判斷。
		n := OpLen(script[p])
		if n < 1 {
			return len(script)
		}
		p += n
	}
	return p
}

// StartMiddlegate 是原版從角色選擇畫面按 Z 進入第一人稱視角時的起始狀態。
//
// 已證實：DOSBox 的 `key:g;key:z` 流程以記憶體 dump 量到地圖 0、(7,3)、面北；
// 見 docs/playtest/01-oracle-timeline.md §2、§7。這不是 ATTRIB.DAT +14 的
// 傳送預設入口 (7,5)，兩條進入路徑不同。
var StartMiddlegate = struct {
	Map, X, Y int
	Face      Facing
}{0, 7, 3, North}

// ── 角色欄位的讀寫（opcode 0x15 / 0x18 / 0x16）────────────────────────────

// scriptTargets 把 opcode 的「對象」參數換成要處理的隊員索引（0 起算）。
//
// 原版 `sub_19A02` 的規則：0 是全隊（由最後一人往前）、超過人數的一律
// 改成第 1 人、9 是「前一個判斷選中的那一位」。
//
// 9 那一路原版查 `ds:54BE`，而填 `ds:54BE` 的正是 opcode `0x26`
// （`sub_1A082`：迴圈讀按鍵直到選到活著的隊員）。`0x31`
// （`sub_1A4BC`）把退路寫得最完整：
//
//	who--
//	if who == 8 {           // 也就是原值 9
//	    who = ds:54BE
//	    if who == 0 { who = ds:042F }   // 退回條件暫存器
//	    if who != 0 { who-- }
//	}
//
// 所以 `ds:54BE` 為 0 時退回條件暫存器的值，再為 0 才輪到第 1 人。
func (w *World) scriptTargets(who byte) []int {
	n := len(w.Party)
	if n == 0 {
		return nil
	}
	k := int(who & 0x7F)
	switch {
	case k == 0:
		out := make([]int, n)
		for i := range out {
			out[i] = n - 1 - i // 由最後一人往前，與原版的遞減迴圈同序
		}
		return out
	case k == 9:
		// ds:54BE → 條件暫存器 → 第 1 人，逐級退。
		sel := w.Selected
		if sel == 0 {
			sel = int(w.Result)
		}
		if sel >= 1 && sel <= n {
			return []int{sel - 1}
		}
		return []int{0}
	case k > n:
		return []int{0}
	}
	return []int{k - 1}
}

// fieldOffset 把欄位選擇器換成角色記錄的偏移。
func fieldOffset(sel byte) (int, bool) {
	if data == nil {
		return 0, false
	}
	f, ok := data.Fields.Lookup(int(sel))
	if !ok {
		return 0, false
	}
	return f.Offset, true
}

// testField 是 opcode `0x15`：讀欄位，結果 OR 進 `ds:042F`。
func (w *World) testField(who, sel, mask byte) {
	off, ok := fieldOffset(sel)
	if !ok {
		return
	}
	for _, i := range w.scriptTargets(who) {
		v := w.Party[i].FieldByte(off)
		if mask != 0 {
			v &= mask
		}
		w.Result |= v
	}
}

// setField 是 opcode `0x18`：`欄位 = 欄位 & 遮罩 | 新值`。
//
// 對象的 bit 7 設起來時，寫入值改用**進來時**的 `ds:042F`，
// 也就是上一個判斷的結果 —— 原版用它把「找到的那個值」搬進欄位。
func (w *World) setField(who, sel, mask, val byte) {
	off, ok := fieldOffset(sel)
	if !ok {
		return
	}
	if who&0x80 != 0 {
		val = w.Result
	}
	w.Result = 0
	for _, i := range w.scriptTargets(who) {
		w.Party[i].SetFieldByte(off, mask, val)
	}
}

// hasItem 是 opcode `0x16`：隊伍裡有人帶著這件物品就把數量加進 `ds:042F`。
// 原版一找到人就停，不會把整隊掃完。
func (w *World) hasItem(id int) {
	w.Result = 0
	if id == 0 {
		return
	}
	for i := range w.Party {
		n := 0
		for _, s := range w.Party[i].Items {
			if s.ID == id {
				n++
			}
		}
		if n > 0 {
			w.Result = byte(n)
			return
		}
	}
}

// RunScriptForTest 直接執行一段腳本，不需要真的踩到事件格。
// 只給測試用 —— 正式流程一律走 Trigger。
func (w *World) RunScriptForTest(script []byte) string {
	w.Pending = nil
	w.Message = ""
	w.MessageWait = false
	w.TextExpect = ""
	w.testPromptScript = append(w.testPromptScript[:0], script...)
	w.Message = w.run(&events.Segment{Index: -1}, -1, w.testPromptScript, 0)
	return w.Message
}

// hasMember 是 opcode `0x2d`：隊伍裡有沒有符合條件的人。
func (w *World) hasMember(spec byte) bool {
	want := spec & 0x0F
	for i := range w.Party {
		c := &w.Party[i]
		if c.Empty() {
			continue
		}
		if spec&0x40 != 0 && c.FieldByte(offSex) == want {
			return true
		}
		if spec&0x80 != 0 && c.FieldByte(offRace) == want {
			return true
		}
		if spec&0x20 != 0 && c.FieldByte(offClass) == want {
			return true
		}
	}
	return false
}

// teachSpell 是 opcode `0x2e`。
func (w *World) teachSpell(spec, bits byte) {
	var a, b byte = 4, 2 // 巫師、弓箭手
	if spec >= 0x80 {
		a, b = 3, 1 // 牧師、聖騎士
		spec &= 0x7F
	}
	off := offSpells + int(spec) - 110
	if off < 0 || off >= RecordSize {
		return
	}
	for i := range w.Party {
		c := &w.Party[i]
		if c.Empty() {
			continue
		}
		if cl := c.FieldByte(offClass); cl != a && cl != b {
			continue
		}
		c.SetFieldByte(off, 0xFF, bits)
	}
}

// Reward 是 opcode `0x2a` 擺好的獎賞。
//
// Items 的三個位元組依原版的讀取順序是「編號、`ds:6956` 那欄、
// `ds:6953` 那欄」—— 後兩欄對應背包的充能與屬性，但哪一欄是哪一個
// 還沒對過，所以原樣保留。
type Reward struct {
	Pending bool
	Gold    uint32
	Gems    uint16
	Items   [3][3]byte
}

// SeedGlobals 把全域變數的初值從 `MM2.EXE` 的尾部資料區抄進來。
//
// 那一段**就是執行時的 DGROUP 初值** —— 檔案裡放的是什麼，開機後
// 記憶體裡就是什麼。不抄的話所有計數器都從 0 開始，畫面上的年份會是 0
// 而不是 900，而且遇敵率、難度那些旗標也全錯。
//
// 只抄 `0x0300`–`0x0500` 這一段：目前解出來的全域變數全在裡面，
// 整段 21 KB 抄進 map 沒有必要。
func (w *World) SeedGlobals(exe []byte) {
	const base = 0x8630 // 尾部資料區在檔案裡的位移
	if len(exe) < base+globalSeedEnd {
		return
	}
	if w.Globals == nil {
		w.Globals = map[uint16]byte{}
	}
	for a := globalSeedStart; a < globalSeedEnd; a++ {
		if v := exe[base+a]; v != 0 {
			w.Globals[uint16(a)] = v
		}
	}
}

const (
	globalSeedStart = 0x0300
	globalSeedEnd   = 0x0500
)

// Year 是目前的年份（`ds:03C8`）。
//
// 位址是這樣定的：原版一開始顯示 `Year= 900`，而整個尾部資料區裡
// **值等於 900 的 word 只有一個**，就是 `ds:03C8`；它又緊鄰日期計數器
// （`ds:03A2`）與那排計數器的索引（`ds:03CA`）。等級：強推論。
func (w *World) Year() int {
	return int(w.Globals[globalYearAddr]) | int(w.Globals[globalYearAddr+1])<<8
}

const globalYearAddr = 0x03C8

// Today 回傳目前的日子：`ds:03A2` 那排計數器裡由 `ds:03CA` 選中的那一格。
func (w *World) Today() byte {
	return w.Globals[dayTable+uint16(w.Globals[dayIndex])*2]
}

// `ds:03A2` 是一排計數器、`ds:03CA` 選其中一格。索引 9 指到的
// 正是 `ds:03B4` —— 自然之門直接寫死那個位址，所以它才要求索引是 9。
const (
	dayTable = 0x03A2
	dayIndex = 0x03CA
)

// dateCond 是 opcode `0x23`。
func (w *World) dateCond(a, b byte) bool {
	day := w.Today()
	switch a {
	case 0xB5:
		return day&1 != 0
	case 0xB6:
		return day&1 == 0
	}
	return a <= day && day <= b
}

// harm 是 opcode `0x31` 對單一隊員的部分（`sub_13928` 的前段）。
//
// 重症的一律跳過；否則擲 `rand(1, 100)`，擲值小於
// 「`記錄 +22` ＋ `ds:03D6`」就完全擋下。
func (w *World) harm(i, dmg int) {
	if i < 0 || i >= len(w.Party) {
		return
	}
	c := &w.Party[i]
	if c.CondBits >= CondBitSevere {
		return
	}
	resist := int(c.FieldByte(offResist)) + int(w.Globals[0x03D6])
	if w.Rand != nil && w.Rand.Range(1, 100) < resist {
		return
	}
	c.addHP(-dmg)
}

// pay 是 opcode `0x24`／`0x25` 的收款：全隊湊得出才扣。
//
// 湊不出來時**一毛都不動** —— 原版先加總比對過才進扣款迴圈。
func (w *World) pay(gold bool, amount int) bool {
	get := func(c *Character) int {
		if gold {
			return int(c.FieldValue(offGold, 4))
		}
		return int(c.FieldValue(offGems, 2))
	}
	set := func(c *Character, v int) {
		if gold {
			c.SetFieldValue(offGold, 4, uint32(v))
			return
		}
		c.SetFieldValue(offGems, 2, uint32(v))
	}
	total := 0
	for i := range w.Party {
		if w.Party[i].Empty() {
			continue
		}
		total += get(&w.Party[i])
	}
	if total < amount {
		return false
	}
	left := amount
	for i := range w.Party {
		if left == 0 {
			break
		}
		if w.Party[i].Empty() {
			continue
		}
		have := get(&w.Party[i])
		take := have
		if take > left {
			take = left
		}
		set(&w.Party[i], have-take)
		left -= take
	}
	return true
}

// teleport 是 opcode `0x0c`：換地圖並移動到指定座標。
//
// 座標 0xFF 在原版是「用這張地圖的預設進入位置」（`ATTRIB.DAT` 的 `+14`，
// 同樣是低 nibble = X、高 nibble = Y）。那條路徑要地圖屬性，
// 所以放在 `Session.Teleport`，這裡只做腳本自己給了座標的情形。
func (w *World) teleport(target, pos byte) {
	if target&0x40 != 0 {
		if w.Rand == nil {
			return
		}
		v := byte(w.Rand.Range(1, 0x14) + 5)
		if v >= 0x11 {
			v += 0x10
		}
		target = v | 0x80
	}
	if target >= 0x80 {
		if w.Rand == nil {
			return
		}
		pos = byte(w.Rand.Range(1, 255))
	}
	m := int(target & 0x3F)
	if m >= len(w.Maps) {
		return
	}
	w.MapIndex = m
	w.X, w.Y = int(pos&0x0F), int(pos>>4)
	w.ClearTravelEffects()
	w.Teleported = true
}

// operand3 讀腳本裡的 3 位元組小端序運算元（`sub_18DF8`）。
func operand3(b []byte) uint32 {
	if len(b) < 3 {
		return 0
	}
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

// addField 是 opcode `0x1f`／`0x20`：對欄位加上或扣掉一個值。
//
// 加法飽和在該欄位寬度的上限。減法不夠扣時**不寫回**，只把結果清成 0 ——
// 原版在寫回之前有一行 `cmp ds:042F, 0 / je 跳過`，所以「付不出來」
// 不會順手把欄位歸零。
//
// `store` 是第三個參數：寫回幾個位元組（原版 `repnz movs` 的長度，
// 值 3 會先變成 4，值 0 表示不寫）。運算的寬度則來自選擇器。
func (w *World) addField(who, sel, store byte, v uint32, sub bool) {
	if data == nil {
		return
	}
	f, ok := data.Fields.Lookup(int(sel))
	if !ok {
		return
	}
	if who&0x80 != 0 {
		v = uint32(w.Result)
	}
	width := int(store)
	if width == 3 {
		width = 4
	}
	for _, i := range w.scriptTargets(who) {
		c := &w.Party[i]
		cur := c.FieldValue(f.Offset, f.Width)
		var next uint32
		if sub {
			if cur < v {
				w.Result = 0
				continue // 不寫回
			}
			next = cur - v
			w.Result = 1
		} else {
			next = cur + v
			if max := fieldMax(f.Width); next > max || next < cur {
				next = max
			}
			w.Result = 1
		}
		if width > 0 {
			c.SetFieldValue(f.Offset, width, next)
		}
	}
}

// fieldMax 是某個寬度的欄位放得下的最大值。
func fieldMax(width int) uint32 {
	switch width {
	case 1:
		return 0xFF
	case 2:
		return 0xFFFF
	default:
		return 0xFFFFFFFF
	}
}

// crossEdge 走出地圖邊界時換到鄰接的那一張。
//
// 抄自 `sub_1B75E`（每次移動後呼叫）：
//
//	X == 0x10 → 新地圖 = ATTRIB +6（東）    X == 0xFF → +8（西）
//	Y == 0x10 → 新地圖 = ATTRIB +5（北）    Y == 0xFF → +7（南）
//	然後 X &= 0x0F、Y &= 0x0F
//
// `& 0x0F` 讓 16 變 0、-1（0xFF）變 15，也就是從對邊進來。城鎮四面都
// 指向自己，所以那個「換圖」其實是繞回同一張的另一邊 —— 不過城鎮外圈
// 六十四面全是牆，走不到邊界。
func (w *World) crossEdge(f Facing, nx, ny int) bool {
	if w.MapIndex < 0 || w.MapIndex >= len(w.Neighbor) {
		return false
	}
	next := w.Neighbor[w.MapIndex][f&3]
	if next < 0 || next >= len(w.Maps) {
		return false
	}
	w.MapIndex = next
	w.X, w.Y = nx&0x0F, ny&0x0F
	// 原版換圖生命週期會清除 `ds:03D5`–`ds:03E1`，包含水行術
	// `ds:03D9`；先換圖再觸發新圖事件。
	w.ClearTravelEffects()
	w.Trigger()
	return true
}

// countSkill 數隊伍裡具備某項第二技能的人數（原版 `sub_36A6`）。
// 每個人兩項，各佔記錄 `+80` 的一個 nibble；同一個人不重複計。
func (w *World) countSkill(skill int) int {
	n := 0
	for i := range w.Party {
		if w.Party[i].Empty() {
			continue
		}
		for _, k := range w.Party[i].Skills {
			if k == skill {
				n++
				break
			}
		}
	}
	return n
}

// ScriptMessageForTest 執行一段腳本並回傳它會顯示的文字。
// 只給測試與盤點工具用。
func ScriptMessageForTest(seg *events.Segment, script []byte) string {
	var w World
	return w.run(seg, -1, script, 0)
}

// ── 全域變數（opcode 0x17 / 0x1a / 0x22）─────────────────────────────────

// globalSelCentury 是 `ds:03CA` 的選擇器。`0x22` 拿它跟腳本給的範圍比 ——
// 遊戲有跨世紀的時間旅行，這個值最可能是目前的世紀。**語意未定案。**
const globalSelCentury = 0x84

// globalAddr 把選擇器換成 DGROUP 位址，0 表示沒有這一項。
//
// 抄自 `sub_18E22`。不是連續的一張表，是一串 `cmp`／`jne`：
//
//	0x00–0x17 → 0x03F6 + N     連續 24 個位元組（劇情旗標）
//	0x23      → 0x03D8
//	0x27–0x2A → 0x03B5 + N     即 0x03DC–0x03DF
//	0x2B      → 0x03E0
//	0x2C      → 0x03E1
//	0x32      → 0x03EA
//	0x33      → 0x03F1
//	0x3B–0x3E → 0x03B7 + N     即 0x03F2–0x03F5
//	0x80–0x83 → 0x036C + N     即 0x03EC–0x03EF
//	0x84      → 0x03CA
func globalAddr(sel int) uint16 {
	switch {
	case sel >= 0x00 && sel < 0x18:
		return uint16(0x03F6 + sel)
	case sel == 0x23:
		return 0x03D8
	case sel >= 0x27 && sel <= 0x2A:
		return uint16(0x03B5 + sel)
	case sel == 0x2B:
		return 0x03E0
	case sel == 0x2C:
		return 0x03E1
	case sel == 0x32:
		return 0x03EA
	case sel == 0x33:
		return 0x03F1
	case sel >= 0x3B && sel <= 0x3E:
		return uint16(0x03B7 + sel)
	case sel >= 0x80 && sel < 0x84:
		return uint16(0x036C + sel)
	case sel == 0x84:
		return 0x03CA
	}
	return 0
}

// Global 讀一個全域變數。沒有這一項或還沒設過都回 0。
func (w *World) Global(sel int) byte {
	a := globalAddr(sel)
	if a == 0 {
		return 0
	}
	return w.Globals[a]
}

// SetGlobal 寫一個全域變數。
func (w *World) SetGlobal(sel int, v byte) {
	a := globalAddr(sel)
	if a == 0 {
		return
	}
	if w.Globals == nil {
		w.Globals = map[uint16]byte{}
	}
	w.Globals[a] = v
}

// takeItem 是 opcode `0x28`：從隊伍的背包裡拿走一件物品。
//
// 原版逐人掃背包六格，找到就移除並讓 `ds:042F` 加一，然後停止 ——
// 一次只拿走一件。
// giveItem 是 opcode `0x19`：給隊伍一件物品（`sub_19B44`）。
//
// 逐人找第一個空的背包格，找到就寫進去並把結果設成 1；全隊都滿就
// 掉在地上 —— 原版寫進 `ds:6950` 那兩格再把 `ds:0434` 設成 `0FFh`，
// 而 `0FFh` 那條路正是 `0x2a` 的 `Treasure!` 領取路徑，所以這裡用
// 同一個 `Reward` 表示，撿起來的程式碼不必分兩套。
//
// 對象 ≥ `0x80` 時**編號改從結果暫存器取**（`cmp [bp+var_6], 80h`），
// 與 `0x18` 是同一個約定。
//
// 地上那份的三個位元組順序與背包相反（`ds:6953` 放屬性、`ds:6956` 放
// 可用次數），與 `ChestFromReward` 讀 `Reward.Items` 的順序一致。
func (w *World) giveItem(who, id, charge, attr byte) {
	if who >= 0x80 {
		id = w.Result
	}
	w.Result = 0
	if id == 0 {
		return
	}
	slot := ItemSlot{ID: int(id), Charge: charge, Attr: attr}
	for i := range w.Party {
		if w.Party[i].GivePackItem(slot) {
			w.Result = 1
			return
		}
	}
	// 全隊背包都滿：掉在地上，等玩家去撿。
	w.Reward = Reward{Pending: true, Items: [3][3]byte{{id, charge, attr}}}
}

func (w *World) takeItem(id int) {
	w.Result = 0
	if id == 0 {
		return
	}
	for i := range w.Party {
		for slot, s := range w.Party[i].Backpack() {
			if s.ID != id {
				continue
			}
			w.Party[i].RemovePackItem(slot)
			w.Result = 1
			return
		}
	}
}

// expectedAnswer 把 `0x2f` 後面那條 `0x30` 的運算元還原成明文。
//
// 編碼是 `明文 = 0x11A − 位元組`（`sub ax, 11Ah` / `neg ax`，`2PLAY` 的
// `0x1A47C`）。`0xFA` 還原成空白，是十格固定長度的填充。
//
// 中間可以夾別的 opcode（原版有 `2f` 與 `30` 不相鄰的段），所以往後掃到
// 第一條 `0x30` 為止，掃不到就回空字串。
func expectedAnswer(script []byte, from int) string {
	for p := from; p < len(script); {
		n := OpLen(script[p])
		if n < 1 || p+n > len(script) {
			return ""
		}
		if script[p] == OpMatchText && p+11 <= len(script) {
			return decodeTextAnswer(script[p+1 : p+11])
		}
		if script[p] == OpAskText {
			return "" // 下一題了，這一題沒有答案
		}
		p += n
	}
	return ""
}

// textAnswerMatches 是 `sub_1A45A` 的十格答案比對在 UI 文字輸入上的等價。
// 原版編輯器把字母收成大寫；remake 的文字輸入保留玩家輸入，再以不分大小寫
// 的比較還原同一個效果。尾端空白是原版固定十格緩衝區的填充。
func textAnswerMatches(answer string, encoded []byte) bool {
	return strings.EqualFold(strings.TrimSpace(answer), decodeTextAnswer(encoded))
}

func decodeTextAnswer(encoded []byte) string {
	out := make([]byte, 0, len(encoded))
	for _, b := range encoded {
		out = append(out, byte(textCipherBase-int(b)))
	}
	return strings.TrimRight(string(out), " ")
}

// textCipherBase 是打字答案的編碼基數（`sub ax, 11Ah` 之後 `neg`）。
const textCipherBase = 0x11A
