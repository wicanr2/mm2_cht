// 這個檔是 tools/msxout.py 產生的，不要手改。
//
// 來源：MSX 版 f004 的 sub_1103／sub_1A40／sub_1C2B，
// 也就是戶外三組擋路物。機制見 docs/research/02-other-platforms.md
// 「MSX 的戶外第一人稱」。

package msx

// OutdoorPieces 回傳第 set 組擋路物在深度 depth、橫向偏移 v 要畫的每一塊。
// set 0／1／2 依序是 sub_1103／sub_1A40／sub_1C2B。
func OutdoorPieces(set, depth, v int) []OutPiece {
	switch set {
	case 0: // sub_1103
		switch depth {
		case 3:
			switch v {
			case -5:
				return []OutPiece{{SheetFeatureA, 133, 21, 21, 11, 0, 0, 28}}
			case 5:
				return []OutPiece{{SheetFeatureA, 126, 21, 21, 11, 133, 0, 28}}
			}
			return []OutPiece{{SheetFeatureA, 126, 21, 28, 11, 63, 14, 28}}
		case 2:
			switch v {
			case -2:
				return []OutPiece{{SheetFeatureA, 147, 0, 35, 21, 0, 0, 23}}
			case 2:
				return []OutPiece{{SheetFeatureA, 126, 0, 35, 21, 119, 0, 23}}
			}
			return []OutPiece{{SheetFeatureA, 126, 0, 56, 21, 49, 35, 23}}
		case 1:
			switch v {
			case -1:
				return []OutPiece{{SheetFeatureA, 70, 0, 56, 42, 0, 0, 13}}
			case 1:
				return []OutPiece{{SheetFeatureA, 0, 0, 56, 42, 98, 0, 13}}
			}
			return []OutPiece{{SheetFeatureA, 0, 0, 126, 42, 14, 0, 13}}
		case 0:
			switch v {
			case -1:
				return []OutPiece{{SheetFeatureA, 0, 42, 42, 60, 0, 0, 4}}
			case 1:
				return []OutPiece{{SheetFeatureA, 42, 42, 42, 60, 112, 0, 4}}
			}
			return []OutPiece{{SheetFeatureA, 0, 0, 126, 16, 14, 0, 48}}
		}
	case 1: // sub_1A40
		switch depth {
		case 3:
			return []OutPiece{{SheetFeatureB, 84, 42, 14, 11, 70, 14, 28}}
		case 2:
			switch v {
			case -2:
				return []OutPiece{{SheetFeatureB, 63, 42, 21, 20, 0, 0, 24}, {SheetFeatureB, 56, 42, 28, 20, 7, 0, 24}}
			case -1:
				return []OutPiece{{SheetFeatureB, 56, 42, 28, 20, 21, 0, 24}, {SheetFeatureB, 56, 42, 28, 20, 35, 0, 24}}
			case 0:
				return []OutPiece{{SheetFeatureB, 56, 42, 28, 20, 49, 0, 24}, {SheetFeatureB, 56, 42, 28, 20, 63, 0, 24}, {SheetFeatureB, 56, 42, 28, 20, 77, 0, 24}}
			case 1:
				return []OutPiece{{SheetFeatureB, 56, 42, 28, 20, 91, 0, 24}, {SheetFeatureB, 56, 42, 28, 20, 105, 0, 24}}
			}
			return []OutPiece{{SheetFeatureB, 56, 42, 28, 20, 119, 0, 24}, {SheetFeatureB, 56, 42, 21, 20, 133, 0, 24}}
		case 1:
			switch v {
			case -1:
				return []OutPiece{{SheetFeatureB, 84, 0, 14, 42, 0, 0, 12}, {SheetFeatureB, 63, 0, 35, 42, 0, 0, 12}, {SheetFeatureB, 56, 0, 42, 42, 14, 0, 12}}
			case 1:
				return []OutPiece{{SheetFeatureB, 56, 0, 14, 42, 140, 0, 12}, {SheetFeatureB, 56, 0, 35, 42, 119, 0, 12}, {SheetFeatureB, 56, 0, 42, 42, 98, 0, 12}}
			}
			return []OutPiece{{SheetFeatureB, 56, 0, 42, 42, 35, 0, 12}, {SheetFeatureB, 56, 0, 42, 42, 56, 0, 12}, {SheetFeatureB, 56, 0, 42, 42, 77, 0, 12}}
		case 0:
			switch v {
			case -1:
				return []OutPiece{{SheetFeatureB, 35, 0, 21, 62, 0, 0, 2}, {SheetFeatureB, 7, 0, 49, 62, 0, 0, 2}}
			case 1:
				return []OutPiece{{SheetFeatureB, 0, 0, 21, 62, 133, 0, 2}, {SheetFeatureB, 0, 0, 49, 62, 105, 0, 2}}
			}
			return []OutPiece{{SheetFeatureB, 0, 0, 56, 62, 21, 0, 2}, {SheetFeatureB, 0, 0, 56, 62, 49, 0, 2}, {SheetFeatureB, 0, 0, 56, 62, 77, 0, 2}}
		}
	case 2: // sub_1C2B
		switch depth {
		case 3:
			return []OutPiece{{SheetFeatureB, 182, 42, 14, 11, 70, 14, 28}}
		case 2:
			switch v {
			case -2:
				return []OutPiece{{SheetFeatureB, 161, 42, 21, 20, 0, 0, 24}, {SheetFeatureB, 154, 42, 28, 20, 7, 0, 24}}
			case -1:
				return []OutPiece{{SheetFeatureB, 154, 42, 28, 20, 21, 0, 24}, {SheetFeatureB, 154, 42, 28, 20, 35, 0, 24}}
			case 0:
				return []OutPiece{{SheetFeatureB, 154, 42, 28, 20, 49, 0, 24}, {SheetFeatureB, 154, 42, 28, 20, 63, 0, 24}, {SheetFeatureB, 154, 42, 28, 20, 77, 0, 24}}
			case 1:
				return []OutPiece{{SheetFeatureB, 154, 42, 28, 20, 91, 0, 24}, {SheetFeatureB, 154, 42, 28, 20, 105, 0, 24}}
			}
			return []OutPiece{{SheetFeatureB, 154, 42, 28, 20, 119, 0, 24}, {SheetFeatureB, 154, 42, 21, 20, 133, 0, 24}}
		case 1:
			switch v {
			case -1:
				return []OutPiece{{SheetFeatureB, 182, 0, 14, 42, 0, 0, 12}, {SheetFeatureB, 161, 0, 35, 42, 0, 0, 12}, {SheetFeatureB, 154, 0, 42, 42, 14, 0, 12}}
			case 1:
				return []OutPiece{{SheetFeatureB, 154, 0, 14, 42, 140, 0, 12}, {SheetFeatureB, 154, 0, 35, 42, 119, 0, 12}, {SheetFeatureB, 154, 0, 42, 42, 98, 0, 12}}
			}
			return []OutPiece{{SheetFeatureB, 154, 0, 42, 42, 35, 0, 12}, {SheetFeatureB, 154, 0, 42, 42, 56, 0, 12}, {SheetFeatureB, 154, 0, 42, 42, 77, 0, 12}}
		case 0:
			switch v {
			case -1:
				return []OutPiece{{SheetFeatureB, 133, 0, 21, 62, 0, 0, 2}, {SheetFeatureB, 105, 0, 49, 62, 0, 0, 2}}
			case 1:
				return []OutPiece{{SheetFeatureB, 98, 0, 21, 62, 133, 0, 2}, {SheetFeatureB, 98, 0, 49, 62, 105, 0, 2}}
			}
			return []OutPiece{{SheetFeatureB, 98, 0, 56, 62, 21, 0, 2}, {SheetFeatureB, 98, 0, 56, 62, 49, 0, 2}, {SheetFeatureB, 98, 0, 56, 62, 77, 0, 2}}
		}
	}
	return nil
}

// OutdoorBand 回傳地平線地形帶在深度 depth、橫向偏移 v 要畫的每一塊。
// SY 還要加上變體位移（見 OutBandVariant）。
func OutdoorBand(depth, v int) []OutBandPiece {
	switch depth {
	case 3:
		switch v {
		case 0:
			return []OutBandPiece{{70, 0, 28, 14, 3, 36}}
		case -1:
			return []OutBandPiece{{56, 0, 0, 21, 3, 36}}
		case 1:
			return []OutBandPiece{{77, 0, 0, 21, 3, 36}}
		}
		return []OutBandPiece{{70, 14, 0, 14, 3, 36}}
	case 2:
		switch v {
		case 0:
			return []OutBandPiece{{56, 0, 31, 42, 5, 39}}
		case -1:
			return []OutBandPiece{{28, 0, 3, 42, 5, 39}}
		case 1:
			return []OutBandPiece{{84, 0, 3, 42, 5, 39}}
		case -2:
			return []OutBandPiece{{0, 0, 3, 28, 5, 39}}
		case 2:
			return []OutBandPiece{{126, 0, 3, 28, 5, 39}}
		}
		return nil
	case 1:
		switch v {
		case 0:
			return []OutBandPiece{{28, 0, 36, 98, 10, 44}}
		case -1:
			return []OutBandPiece{{0, 0, 8, 56, 10, 44}}
		case 1:
			return []OutBandPiece{{98, 0, 8, 56, 10, 44}}
		}
		return nil
	case 0:
		switch v {
		case 0:
			return []OutBandPiece{{0, 0, 46, 154, 10, 54}}
		case -1:
			return []OutBandPiece{{0, 0, 18, 28, 10, 54}}
		case 1:
			return []OutBandPiece{{126, 0, 18, 28, 10, 54}}
		}
		return nil
	}
	return nil
}
