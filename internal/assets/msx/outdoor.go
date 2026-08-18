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

// 地形帶（原版 `sub_E3E`）**還沒解，所以不畫**。
//
// 已知的部分：它讀 `SheetBand`，那張是三個 154×56 的變體疊起來，
// 由每張地圖的位元組挑（`+0x204` 的高 nibble 是 9 或 0x0A 用第一個）；
// 觸發它的是格子碼 4 與 5（`sub_297A`），與 DOS 的地形帶同一個位置。
// 深度 3 那一段也讀出來了：高 3 列、貼在視圖 y=36，來源列是變體位移
// 加 0 或 28。
//
// 卡在哪：`sub_E3E` 的落點是**堆疊上的區域變數**（`sub_5020`／`sub_5014`
// 那一族是「讀 SP+N」），要先把它的堆疊框架模型建起來才讀得出來 ——
// `tools/msxout.py` 目前只做暫存器的符號執行。**寧可少畫一條地平線，
// 也不要把猜出來的座標寫成表**：那種表看起來像解出來了，
// 而畫面上「有一條帶」與「那條帶在對的位置」分不出來。
