package view

import (
	"image"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 野外的第一人稱是**另一條繪圖路徑**，不是「換一組牆貼圖」。
//
// 原版 `_2play_e10` 在野外（場景碼 3／4／6）只載地板 `OUTF.16` 與
// 三組 `OUTDOOR1-3.16`，**不載「32 張牆」那一組**；地形檔
// （`DESERT`／`OCEAN`／`SWAMP`／`TUNDRA`）由 `_2play_e09` 依 `ATTRIB +4`
// 另外載。畫的是三支自己的常式：
//
//	sub_18AD0(d)        正面的擋路物，只畫**最近的一個**
//	sub_18B0C(d, limit) 左側，由遠到近
//	sub_18BEC(d, limit) 右側，由遠到近
//
// 每一格畫什麼由**地形值查 `ds:52B2`** 決定（`game.Map.OutdoorCode`）：
// 0 不畫、1–3 是三組擋路物之一、4 是地平線的地形帶。
//
// 落點與影格全部是 DGROUP 的小表，值抄在下面。推導與位址見
// `docs/formats/04-graphics.md`「野外是另一條路徑」。

// outdoorSets 是野外要用的四組素材：0–2 是 `OUTDOOR1-3`、3 是地形檔。
// 原版是四個遠指標 `ds:036E`／`0372`／`0376`／`037A`，碼減一就是索引。
const outdoorSets = 4

// 三支常式各自的影格與落點表。x／y 已經是**畫面座標**（含視圖原點 8）。
var (
	outFrontFrame = [Depth]int{0, 0, 1, 2}
	outFrontX     = [Depth]int{40, 40, 64, 88}
	outFrontY     = [Depth]int{21, 21, 42, 50}

	outLeftFrame = [Depth]int{4, 5, 2, 3}
	outLeftX     = [Depth]int{8, 16, 32, 88}
	outSideY     = [Depth]int{36, 46, 50, 58}

	outRightFrame = [Depth]int{6, 7, 2, 3}
	outRightX     = [Depth]int{176, 152, 136, 120}

	// 地平線的地形帶走自己的常式（原版 `sub_189B8`），影格與落點都與
	// 擋路物那三支不同 —— 拿擋路物的表去畫 208 寬的橫帶會衝出視圖右緣，
	// 蓋到旁邊的隊伍面板。
	//
	//	正面是帶   → 整條 208 寬（影格 d）貼在 x=8
	//	           左也是帶 → 影格 d+12 貼 x=8；右也是帶 → 影格 d+16 貼 outBandRightX
	//	正面不是帶 → 左半（影格 d+4）貼 x=8；右半（影格 d+8）貼 x=112
	outBandY      = [Depth]int{108, 93, 78, 68} // ＝ 128 − ds:159E
	outBandRightX = [Depth]int{184, 160, 136, 112}
)

// OutdoorPiece 是野外一筆貼圖的落點，供測試檢查它落在視圖裡。
type OutdoorPiece struct {
	What  string
	Frame int
	X, Y  int
	Band  bool // true ＝ 取自地形檔（208 寬的橫帶），false ＝ 擋路物
}

// OutdoorGeometry 把野外那三支常式與地形帶的落點全部列出來。
//
// 匯出它只有一個用途：讓測試拿真的素材尺寸去驗「有沒有畫到視圖外」。
func OutdoorGeometry() []OutdoorPiece {
	var out []OutdoorPiece
	for d := 0; d < Depth; d++ {
		out = append(out,
			OutdoorPiece{"正面", outFrontFrame[d], outFrontX[d], outFrontY[d], false},
			OutdoorPiece{"左", outLeftFrame[d], outLeftX[d], outSideY[d], false},
			OutdoorPiece{"右", outRightFrame[d], outRightX[d], outSideY[d], false},
			OutdoorPiece{"帶 整條", d, 8, outBandY[d], true},
			OutdoorPiece{"帶 左半", d + 4, 8, outBandY[d], true},
			OutdoorPiece{"帶 右半", d + 8, 112, outBandY[d], true},
			OutdoorPiece{"帶 窄左", d + 12, 8, outBandY[d], true},
			OutdoorPiece{"帶 窄右", d + 16, outBandRightX[d], outBandY[d], true},
		)
	}
	return out
}

// Outdoor 回報這一套素材有沒有野外那四組。沒有就退回室內那條路徑
// （畫面會是城鎮的牆，與 2026-08-17 之前相同）。
func (t *TownSet) Outdoor() bool { return len(t.Out) == outdoorSets }

// SetOutdoor 掛上野外的四組素材與地板。
func (t *TownSet) SetOutdoor(sets [][]*image.Paletted, floor []*image.Paletted) {
	if len(sets) != outdoorSets {
		return
	}
	t.Out = sets
	t.OutFloor = floor
}

// drawOutdoor 畫野外的第一人稱。回傳 false 表示這一套素材做不到，
// 呼叫端要退回室內那條。
func (t *TownSet) drawOutdoor(s *render.Screen, w *game.World, phase int) bool {
	m := w.CurrentMap()
	if m == nil || !t.Outdoor() {
		return false
	}
	// 地板與天空：原版 `_2play_e03` 開頭那兩筆，地板貼在視圖下半。
	if len(t.OutFloor) > 0 {
		_, h := t.size(t.OutFloor[0])
		t.blit(s, t.OutFloor[0], FPX, FPY+FPH-h)
	}
	t.drawSky(s, w)

	// 三行 × 四個深度的地形碼。`d` 是離隊伍幾格，0 是腳下那一格。
	dx, dy := w.Face.Delta()
	lx, ly := game.Facing((int(w.Face) + 3) & 3).Delta()
	var front, left, right [Depth]int
	for d := 0; d < Depth; d++ {
		x, y := w.X+dx*d, w.Y+dy*d
		front[d] = m.OutdoorCode(x, y)
		left[d] = m.OutdoorCode(x+lx, y+ly)
		right[d] = m.OutdoorCode(x-lx, y-ly)
	}

	// 地形帶（碼 4）先畫 —— 它是地平線，擋路物疊在上面。
	for d := 0; d < Depth; d++ {
		band := func(v int) bool { return v == outBandCode }
		switch {
		case band(front[d]):
			t.blitBand(s, d, 8, outBandY[d])
			if band(left[d]) {
				t.blitBand(s, d+12, 8, outBandY[d])
			}
			if band(right[d]) {
				t.blitBand(s, d+16, outBandRightX[d], outBandY[d])
			}
		default:
			if band(left[d]) {
				t.blitBand(s, d+4, 8, outBandY[d])
			}
			if band(right[d]) {
				t.blitBand(s, d+8, 112, outBandY[d])
			}
		}
	}

	// **正面只畫最近的那一個**（原版 `sub_18CC6`：找第一個非空的深度就停）。
	// 兩側畫到那個深度為止 —— 更遠的被擋住了。
	limit := Depth - 1
	for d := 0; d < Depth; d++ {
		if front[d] != 0 && front[d] != outBandCode {
			t.blitOut(s, front[d], outFrontFrame[d], outFrontX[d], outFrontY[d])
			if d < limit {
				limit = d
			}
			break
		}
	}
	// 由遠到近：近的蓋住遠的。
	//
	// **最深那一格若同時有正面擋路物，兩側要接上去**（原版 `sub_18B0C`
	// 與 `sub_18BEC` 的「連續」分支）：改用**前一個深度**的影格與 x、
	// y 改用正面那一張的 —— 那是把相鄰兩格的樹接成一片。再前一格也有
	// 東西的話這一筆整個不畫，否則會疊出兩層樹冠。
	joined := limit > 0 && front[limit] != 0 && front[limit] != outBandCode
	for d := limit; d >= 0; d-- {
		frame, x, y := outLeftFrame[d], outLeftX[d], outSideY[d]
		if joined && d == limit {
			if left[d-1] != 0 {
				continue
			}
			frame, x, y = outLeftFrame[d-1], outLeftX[d-1], outFrontY[d]
		}
		// 深度 1 的左側一律貼齊左緣；深度 2 只有在它就是最深那一格時才貼齊。
		if d == 1 || (d == 2 && d == limit) {
			x = 8
		}
		t.blitOut(s, left[d], frame, x, y)
	}
	for d := limit; d >= 0; d-- {
		frame, x, y := outRightFrame[d], outRightX[d], outSideY[d]
		if joined && d == limit {
			if right[d-1] != 0 {
				continue
			}
			frame, x, y = outRightFrame[d-1], outRightX[d-1], outFrontY[d]
		}
		if d == 1 && right[0] == 0 {
			x = 176
			if d != limit {
				x = 152
			}
		}
		t.blitOut(s, right[d], frame, x, y)
	}
	return true
}

// outBandCode 是「這一格是地平線的地形帶」那個碼（`ds:52B2` 的值 4）。
const outBandCode = 4

// blitBand 從地形檔（第四組）貼一條地平線橫帶。
func (t *TownSet) blitBand(s *render.Screen, frame, x, y int) {
	set := t.Out[outdoorSets-1]
	if frame < 0 || frame >= len(set) {
		return
	}
	t.blitKey(s, set[frame], x, y, int(t.clear))
}

// blitOut 依地形碼挑素材組再貼一張。碼 0 是「什麼都沒有」，
// 碼 4 是地形帶（走 blitBand，不走這裡）。
func (t *TownSet) blitOut(s *render.Screen, code, frame, x, y int) {
	if code <= 0 || code >= outBandCode {
		return
	}
	set := t.Out[code-1]
	if frame < 0 || frame >= len(set) {
		return
	}
	// 表裡的值已經是**畫面座標**（含視圖原點 8），不要再加一次 FPX。
	t.blitKey(s, set[frame], x, y, int(t.clear))
}
