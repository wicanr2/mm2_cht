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

	MapIndex int
	X, Y     int
	Face     Facing

	// Message 是最近一次觸發的事件文字，空字串表示沒有。
	Message string

	// Encounter 是腳本擺出來的固定遭遇（怪物編號），空的表示沒有。
	// 由 opcode `0x12`／`0x13` 設定，呼叫端取走之後要自己清掉。
	Encounter []int

	// Result 是原版的 `ds:042F` —— 腳本的「上一個判斷結果」。
	//
	// 它不只給付款用：`0x15`（測角色欄位）、`0x16`（隊伍有沒有某物品）、
	// `0x32`（交給 root 判斷）都把結果 OR 進來，而 `0x10`／`0x11`
	// 讀它決定跳不跳。所以是同一個位元組，不是三個不同的旗標。
	//
	// 付款那組 opcode 還沒實作，所以「有沒有付」那一路仍恆為 0。
	Result byte

	// Rand 是腳本要擲骰時用的亂數（`0x0c` 的隨機傳送、`0x1c`）。
	// `Session` 建立時接上；沒接的話那幾條隨機分支不執行。
	Rand *Rand

	// Teleported 記錄這一段腳本有沒有把隊伍送走。呼叫端據此知道
	// 位置變了、要重新讀地圖。
	Teleported bool

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
	w.Teleported = false
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
		case OpRedraw, OpWaitKey:
			// 重畫與等按鍵在 remake 沒有對應動作。列出來是為了
			// 「認得但不做」與「不認得」分得開。
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

// ── 角色欄位的讀寫（opcode 0x15 / 0x18 / 0x16）────────────────────────────

// scriptTargets 把 opcode 的「對象」參數換成要處理的隊員索引（0 起算）。
//
// 原版 `sub_19A02` 的規則：0 是全隊（由最後一人往前）、超過人數的一律
// 改成第 1 人、9 是「前一個判斷選中的那一位」。9 那一路原版查的是
// `ds:54BE`，填它的程式碼還沒找到，這裡當成第 1 人 —— 這是**假設**。
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
	case k == 9 || k > n:
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
	return w.run(&events.Segment{}, script)
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
	w.Teleported = true
}
