package game

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

// ATTRIB.DAT：每張地圖 64 bytes，60 張，解壓後 3,840 bytes。
//
// 定錨靠第一個位元組：六十筆的 `+0` 剛好是 0…59，一個不差。
// 這也是為什麼 stride 只能是 64 —— 換成 15、16 或 26，那個遞增序列就散了。
const (
	MapAttrSize  = 64
	MapAttrCount = 60
)

// 鄰接欄位在 `+5`…`+8`。哪一對是南北、哪一對是東西，由對向性定：
// 「`+5` 的鄰居其 `+7` 指回自己」六十張全中，`+6` 與 `+8` 同理，
// 而交叉配對（+5/+6、+5/+8…）只有 40/60。
//
// `+6` 是東：地圖 5 的 `+6` 是 6，而地圖 6 的 `+8` 是 5，
// 編號沿著這個方向遞增。`+5` 與 `+7` 其一是北、其一是南，還沒定 ——
// 要對照世界地圖的實際方位才能斷。
// BashDifficulty 是撞門的難度門檻（`+18`）。
//
// 位置由 `2MISC.img` 的 `0xC1E4` 定出：撞門時拿隊伍力量與 `ds:5998` 比，
// 而 `ds:5998` 落在地圖屬性那 64 bytes 的 `+18`（`ds:5986` 是起點）。
//
// 值全是十的倍數、0–100：五座城鎮 10/30/20/40/30（中門最低），
// 野外二十張全是 0（沒有門），地城 20–100 隨深度上升。
const attrBashDifficulty = 18

const (
	neighborAxis1A = 5 // 與 axis1B 對向
	neighborAxis1B = 7
	neighborEast   = 6
	neighborWest   = 8
)

// MapAttr 是一張地圖的屬性。
type MapAttr struct {
	Index int
	Raw   [MapAttrSize]byte
}

// BashDifficulty 回傳這張地圖撞門的難度門檻。
func (a *MapAttr) BashDifficulty() int { return int(a.Raw[attrBashDifficulty]) }

// Neighbor 回傳某個鄰接欄位指到的地圖編號。
func (a *MapAttr) Neighbor(field int) int { return int(a.Raw[field]) }

// East、West 回傳東西向的鄰接地圖。
func (a *MapAttr) East() int { return a.Neighbor(neighborEast) }
func (a *MapAttr) West() int { return a.Neighbor(neighborWest) }

// Axis1 回傳南北向的一對鄰接地圖。兩者的方位還沒定，所以不叫 North/South。
func (a *MapAttr) Axis1() (int, int) {
	return a.Neighbor(neighborAxis1A), a.Neighbor(neighborAxis1B)
}

// SelfContained 回報這張圖四面都指向自己 —— 城鎮就是這樣，
// 走到邊界不會接到別張圖。前五張（五座主要城鎮）全是如此。
func (a *MapAttr) SelfContained() bool {
	for _, f := range []int{neighborAxis1A, neighborEast, neighborAxis1B, neighborWest} {
		if a.Neighbor(f) != a.Index {
			return false
		}
	}
	return true
}

// ParseMapAttrs 解開 ATTRIB.DAT。
func ParseMapAttrs(blob []byte) ([]MapAttr, error) {
	raw, err := lzw.Segment(blob, 0)
	if err != nil {
		return nil, fmt.Errorf("解壓 ATTRIB.DAT: %w", err)
	}
	if len(raw) != MapAttrSize*MapAttrCount {
		return nil, fmt.Errorf("解出 %d bytes，預期 %d", len(raw), MapAttrSize*MapAttrCount)
	}
	out := make([]MapAttr, 0, MapAttrCount)
	for i := 0; i < MapAttrCount; i++ {
		m := MapAttr{Index: i}
		copy(m.Raw[:], raw[i*MapAttrSize:(i+1)*MapAttrSize])
		out = append(out, m)
	}
	return out, nil
}
