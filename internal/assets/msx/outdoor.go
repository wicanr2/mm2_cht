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
	// 由該圖的地形碼挑（見 OutBandVariantFor）。
	SheetBand
	// SheetGround 是地圖 41–44 的地面：154×28，蓋掉背景**下緣那 28 列**。
	SheetGround
)

// OutSheetID 是每一張表在磁片上的 id。**擋路物 A 與地形帶各有兩張**
// （`0x2042`／`0x2043`、`0x2045`／`0x2046`），哪一張由地圖號查
// `OutdoorMaps`；這裡列的是預設那張，實際要用的以那張表為準。
var OutSheetID = map[OutSheet]uint16{
	SheetBack:     0x2041,
	SheetFeatureA: 0x2042,
	SheetFeatureB: 0x2044,
	SheetBand:     0x2045,
}

// OutFeatureAID 與 OutBandID 是兩個換得掉的槽各自的兩張。
var (
	OutFeatureAID = [2]uint16{0x2042, 0x2043}
	OutBandID     = [2]uint16{0x2045, 0x2046}
)

// OutGroundID 是地圖 41–44 各自的地面素材。
//
// 原版 `sub_390E` 是四格的 `cp` 分派（41→`0x204A`、42→`0x2047`、
// 43→`0x2048`、44→`0x2049`），載到 VRAM (0,356)，也就是背景那張
// 154×64 的**下緣 28 列**上；`sub_2B0A` 之後整幅拷走，所以效果是
// 「同一張背景換掉地面那一半」。順序不是 9/10/11/12 —— 對上 DOS 那四張
// 圖的貼圖組碼（41 是凍原、42 沙漠、43 沼澤、44 海洋）才排得出來。
var OutGroundID = map[int]uint16{41: 0x204A, 42: 0x2047, 43: 0x2048, 44: 0x2049}

// OutGroundRow 是地面素材蓋在背景上的第一列（背景 154×64，地面 154×28）。
const OutGroundRow = 36

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
// `SheetBand` 是 154×168 ＝ 三個 154×56 的變體疊起來，由地形碼挑，
// 見 OutBandVariantFor。
type OutBandPiece struct {
	X, XK    int // 視圖 x ＝ XK·v + X，來源 x 同值
	SY, W, H int // SY 還要加上變體位移
	DY       int
}

// OutBandVariant 是三個變體在 SheetBand 裡的列位移。
//
// **第三個沒有人用**：`sub_E3E` 的四個呼叫點傳的是 0 或 1，一次都沒有 2。
// 留著是因為那張表確實是 168 列高 —— 第三段存在，只是這條路徑不畫它。
var OutBandVariant = [3]int{0, 56, 112}

// OutBandVariantFor 依地形碼回傳要用哪一個變體。
//
// `sub_297A`：沙漠（9）與海洋（0x0A）用第一個，其餘用第二個。
// 地圖 40–44 的地形碼是 0，落在「其餘」——它們的地面另外由
// OutGroundID 換掉，地平線那一段仍走這條。
func OutBandVariantFor(terrain int) int {
	if terrain == 9 || terrain == 10 {
		return 0
	}
	return 1
}

// OutBandCode 是「這一格是地平線的地形帶」那個碼。
//
// DOS 走 `ds:52B2` 的分類表得到同一個 4（見 `internal/game.Map.OutdoorCode`），
// 所以兩個平台在這一格上是同一件事。
const OutBandCode = 4
