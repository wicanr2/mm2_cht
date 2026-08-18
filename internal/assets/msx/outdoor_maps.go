// 這個檔是 tools/msxmaps.py 產生的，不要手改。
//
// 來源：MSX 版引擎 f002 的 `0x431`，24 筆 8 bytes 的記錄。
// 機制見 docs/research/02-other-platforms.md「MSX 的戶外第一人稱」。

package msx

// OutMap 是一張野外圖要用哪幾張戶外素材。
type OutMap struct {
	// FeatureA 是擋路物 A 的素材編號：0x2042 或 0x2043，兩張同尺寸同落點。
	FeatureA uint16
	// Band 是地平線地形帶的素材編號：0x2045 或 0x2046。
	Band uint16
	// Terrain 是地形碼（9 沙漠、10 海洋、11 沼澤、12 凍原）。
	// 0 表示這張圖的地面由地圖號另外挑，見 OutGroundID。
	Terrain int
}

// OutdoorMaps 是野外圖的素材選擇，鍵是地圖號。不在表裡的是室內圖。
var OutdoorMaps = map[int]OutMap{
	5:  {FeatureA: 0x2043, Band: 0x2045, Terrain: 12},
	6:  {FeatureA: 0x2043, Band: 0x2045, Terrain: 12},
	7:  {FeatureA: 0x2042, Band: 0x2045, Terrain: 10},
	8:  {FeatureA: 0x2042, Band: 0x2045, Terrain: 10},
	9:  {FeatureA: 0x2043, Band: 0x2045, Terrain: 12},
	10: {FeatureA: 0x2043, Band: 0x2045, Terrain: 12},
	11: {FeatureA: 0x2042, Band: 0x2045, Terrain: 10},
	12: {FeatureA: 0x2043, Band: 0x2045, Terrain: 10},
	13: {FeatureA: 0x2043, Band: 0x2045, Terrain: 10},
	14: {FeatureA: 0x2042, Band: 0x2045, Terrain: 10},
	15: {FeatureA: 0x2043, Band: 0x2045, Terrain: 10},
	16: {FeatureA: 0x2042, Band: 0x2045, Terrain: 10},
	33: {FeatureA: 0x2042, Band: 0x2046, Terrain: 9},
	34: {FeatureA: 0x2042, Band: 0x2046, Terrain: 9},
	35: {FeatureA: 0x2042, Band: 0x2046, Terrain: 9},
	36: {FeatureA: 0x2042, Band: 0x2046, Terrain: 9},
	37: {FeatureA: 0x2042, Band: 0x2046, Terrain: 9},
	38: {FeatureA: 0x2042, Band: 0x2046, Terrain: 11},
	39: {FeatureA: 0x2042, Band: 0x2046, Terrain: 11},
	40: {FeatureA: 0x2042, Band: 0x2046, Terrain: 0},
	41: {FeatureA: 0x2043, Band: 0x2046, Terrain: 0},
	42: {FeatureA: 0x2043, Band: 0x2046, Terrain: 0},
	43: {FeatureA: 0x2043, Band: 0x2046, Terrain: 0},
	44: {FeatureA: 0x2043, Band: 0x2046, Terrain: 0},
}
