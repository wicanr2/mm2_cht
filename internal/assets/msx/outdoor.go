package msx

// 戶外的第一人稱。
//
// 與室內是**兩條路徑、兩組素材**，而且共用同一片 VRAM 座標 ——
// 先前 remake 的室內牆表就是抄成了這一條（見 `docs/todo.md` A8）。
// 完整的機制與位址在 `docs/research/02-other-platforms.md`
// 「MSX 的戶外第一人稱」，這裡只留實作要用的部分。
//
// 原版把整幅畫在一個 154×64 的工作頁上再拷到畫面，所以下面的落點
// 全部是**視圖座標**（原版的目的座標減掉工作頁原點 (0,256)）。

// 戶外要用的四張表。`OutSheetID` 是磁片上的檔案 id。
type OutSheet int

const (
	// SheetBack 是背景：天空與地面，整幅 154×64，不透空先鋪上去。
	SheetBack OutSheet = iota
	// SheetFeatureA 是第一組擋路物（原版 `sub_1103` 讀的那張）。
	SheetFeatureA
	// SheetFeatureB 是第二、三組共用的那張：左半 x 0–97 是第二組、
	// 右半 x 98–195 是第三組，所以兩組的落點表只差 98。
	SheetFeatureB
	// SheetBand 是地平線的地形帶，154×168 ＝ **三個 154×56 的變體疊起來**，
	// 由每張地圖自己的位元組挑（`+0x204` 的高 nibble）。
	SheetBand
)

// OutSheetID 是每一張表在磁片上的 id。
//
// 有兩張擋路物 A（`0x2042`／`0x2043`）與兩張地形帶（`0x2045`／`0x2046`），
// 挑哪一張由每張地圖的位元組決定；remake 目前固定用第一張 ——
// **那是取捨不是還原**，選擇規則還沒解（見 docs/research/02）。
var OutSheetID = map[OutSheet]uint16{
	SheetBack:     0x2041,
	SheetFeatureA: 0x2042,
	SheetFeatureB: 0x2044,
	SheetBand:     0x2045,
}

// OutView 是戶外視圖的大小，與室內同一個工作頁。
var OutView = [2]int{154, 64}

// OutPiece 是一塊貼圖：從某張表的 (SX,SY) 取 W×H，貼到視圖的
// (DXK·v + DX, DY)。DXK 為 0 表示落點與 v 無關。
type OutPiece struct {
	Sheet        OutSheet
	SX, SY, W, H int
	DX, DXK      int
	DY           int
}

// OutDepthRange 是每個深度要列舉的橫向偏移範圍（±這個值）。
//
// 由 `sub_2B0A` 的迴圈邊界讀出來：深度 3 是 −5…5、深度 2 是 −2…2、
// 深度 1 與 0 是 −1…1。**視野隨距離變寬**，與 DOS 那條一樣。
var OutDepthRange = [4]int{1, 1, 2, 5}

// 地形帶（原版 `sub_E3E`）。
//
// 觸發它的是格子碼 4 與 5（`sub_297A`），與 DOS 的地形帶同一個位置。
// 規則比擋路物簡單：**那張帶本身就是一整幅與視圖對齊的畫**，每一格
// 只是把自己那一段複製過來 —— 所以來源 x 與目的 x 相同，表裡只有一個 X。
//
// `SheetBand` 是 154×168 ＝ **三個 154×56 的變體疊起來**，由每張地圖的
// 位元組挑（`+0x204` 的高 nibble 是 9 或 0x0A 用第一個）。remake 目前
// 固定用第一個 —— 那是取捨不是還原，選擇規則還沒解。
type OutBandPiece struct {
	X, XK    int // 視圖 x ＝ XK·v + X，來源 x 同值
	SY, W, H int // SY 還要加上變體位移
	DY       int
}

// OutBandVariant 是三個變體在 SheetBand 裡的列位移。
var OutBandVariant = [3]int{0, 56, 112}

// OutBandCode 是「這一格是地平線的地形帶」那個碼。
//
// DOS 走 `ds:52B2` 的分類表得到同一個 4（見 `internal/game.Map.OutdoorCode`），
// 所以兩個平台在這一格上是同一件事。
const OutBandCode = 4
