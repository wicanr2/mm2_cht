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

	// Answer 是腳本問 Y／N 時（opcode `0x09`）的回答。nil 一律當成 N ——
	// 「沒有人按 Y」與原版在玩家還沒回答時的狀態一致。
	Answer func() bool

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
	if m == nil {
		return false
	}
	dx, dy := f.Delta()
	nx, ny := w.X+dx, w.Y+dy
	if nx < 0 || nx >= MapW || ny < 0 || ny >= MapH {
		return w.crossEdge(f, nx, ny)
	}
	if !m.CanMove(w.X, w.Y, f) {
		return false
	}
	w.X, w.Y = nx, ny
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

	// 另外三個也是顯示字串，差別只在版面：
	//
	//	03 NN  sub_190F2  設 ds:0430 |= 3 之後轉呼叫 0x02 的 handler
	//	05 NN  sub_19160  自己取字串（sub_18FD0 + sub_19016）再畫
	//	06 NN  sub_191EC  開一個 0x12×9 的方框，把字串裡的 '-' 換成
	//	                  框線字元（`0x2D` → `0x7B`）再畫進去
	//
	// 六個一起認之後，會顯示訊息的事件格從 57.0% 升到 69.5%。
	OpShowStringBoxed  = 0x06
	OpShowStringPlain  = 0x05
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
	w.Sound = -1
	w.Picture = 0
	w.Facility = FacilityNone
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
		case OpShowStringLeft, OpShowStringWindow, OpShowString,
			OpShowStringWindow2, OpShowStringPlain, OpShowStringBoxed:
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
		case OpAdd:
			w.addField(script[p+1], script[p+2], script[p+3], operand3(script[p+4:]), false)
		case OpSub:
			w.addField(script[p+1], script[p+2], script[p+3], operand3(script[p+4:]), true)
		case OpAsk:
			w.Result = 0
			if w.Answer != nil && w.Answer() {
				w.Result = 1
			}
		case OpSound:
			w.Sound = int(script[p+1])
		case OpFacility:
			if k := FacilityByCode(int(script[p+1])); k != FacilityNone {
				w.Facility = k
			}
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
		case OpCountSkill:
			w.Result = byte(w.countSkill(int(script[p+1])))
		case OpShowPicture:
			if data != nil {
				w.Picture = data.Pictures.Picture(w.Scene, int(script[p+1]))
			}
		case OpRedraw, OpRedrawView, OpWaitKey:
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
	return w.run(seg, script)
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
