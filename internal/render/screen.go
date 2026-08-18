// Package render 管理兩層畫布。
//
// 原版是 320×200 的 EGA 畫面。中文筆畫多，縮到原版的 8×8 字位會糊成一團，
// 所以採「原版像素層維持原解析度、整數倍 nearest 放大，中文另走高解析層」
// 的分工（見 CLAUDE.md §6）。這裡把兩層的座標關係固定下來，
// 避免後續各處各自換算。
package render

import (
	"image"
	"image/color"
)

const (
	// OrigW/OrigH 是原版畫面的邏輯尺寸，不隨顯示倍率改變。
	OrigW, OrigH = 320, 200
	// Scale 是原版像素層放大的整數倍率。非整數倍即使用 nearest 也會
	// 出現寬窄不一的像素，所以只允許整數。
	Scale = 3
	// HiW/HiH 是高解析層（也就是實際輸出）的尺寸。
	HiW, HiH = OrigW * Scale, OrigH * Scale
)

// Screen 疊合兩層：Orig 是原版 EGA 像素，Hi 是放大後的輸出。
// 中文字直接畫進 Hi，不經過 Orig，才不會被放大成馬賽克。
type Screen struct {
	Orig *image.Paletted
	Hi   *image.RGBA

	// hi 是等 Flush 之後才畫的高解析貼圖（見 BlitHi）。
	hi []hiSprite
}

// hiSprite 是一張已經放大好、要蓋在 Hi 層上的貼圖。
// x/y 用**原版座標**，畫的時候才乘 Scale —— 呼叫端因此不必知道倍率。
type hiSprite struct {
	im   *image.Paletted
	x, y int
	key  int // 透空色的索引，-1 表示不透空
	// mask 非 nil 時由它決定哪些位置透空（見 BlitMask）。
	mask *image.Paletted
	// fitW/fitH 非零時表示這張圖要**縮放**填滿 w×h（原版座標），
	// 而不是照它自己的尺寸貼。見 BlitHiFit。
	fitW, fitH int
}

func New(pal color.Palette) *Screen {
	return &Screen{
		Orig: image.NewPaletted(image.Rect(0, 0, OrigW, OrigH), pal),
		Hi:   image.NewRGBA(image.Rect(0, 0, HiW, HiH)),
	}
}

// Clear 把原版層填成指定的調色盤索引。
func (s *Screen) Clear(idx uint8) {
	for i := range s.Orig.Pix {
		s.Orig.Pix[i] = idx
	}
}

// Blit 把一張原版影像畫到原版層的 (x, y)。
func (s *Screen) Blit(src *image.Paletted, x, y int) {
	b := src.Bounds()
	for sy := 0; sy < b.Dy(); sy++ {
		dy := y + sy
		if dy < 0 || dy >= OrigH {
			continue
		}
		for sx := 0; sx < b.Dx(); sx++ {
			dx := x + sx
			if dx < 0 || dx >= OrigW {
				continue
			}
			s.Orig.SetColorIndex(dx, dy, src.ColorIndexAt(sx, sy))
		}
	}
}

// BlitKey 與 Blit 相同，但跳過等於 key 的像素。
//
// **透空色不是全域的，是看貼圖。** 牆的貼圖用色號 8（深灰）標出「這裡
// 看得到後面」——側牆矩形四角那兩塊楔形就是它，露出來的是天空與地板。
// 而同一個色號 8 在地板貼圖裡是真的顏色（灰白相間的格子），所以不能
// 在 `Blit` 裡一律跳過。
//
// 判準：把 `TOWN.16` 的影格 4 對回原版截圖，327 個不符的像素**全部**是
// 「樣板 8 → 截圖是天空或地板」，沒有一個例外。
func (s *Screen) BlitKey(src *image.Paletted, x, y int, key uint8) {
	b := src.Bounds()
	for sy := 0; sy < b.Dy(); sy++ {
		dy := y + sy
		if dy < 0 || dy >= OrigH {
			continue
		}
		for sx := 0; sx < b.Dx(); sx++ {
			dx := x + sx
			if dx < 0 || dx >= OrigW {
				continue
			}
			if c := src.ColorIndexAt(sx, sy); c != key {
				s.Orig.SetColorIndex(dx, dy, c)
			}
		}
	}
}

// BlitMask 把 src 畫上去，**兩張都是透空色的位置才透空**。
// mask 為 nil 時退化成 BlitKey。
//
// 為什麼需要第二張：側牆是透視梯形，梯形之外原版根本沒碰。素牆那一組
// （`TOWN.16` 影格 0–15）的色號 8 剛好就是梯形之外（影格 4 的 327 個
// 色號 8 對回截圖，**全部**是天空或地板，沒有例外），所以拿色號當透空
// 是對的。門那一組（影格 16–31）是**同一面牆畫上門**，門的柵欄也是
// 色號 8 與 0 交錯的網紋，位置卻在牆裡面 —— 一律當透空，柵欄會漏出天空。
//
// 兩張取交集就同時對：柵欄在素牆那張是石頭（畫），梯形之外兩張都是 8
// （不畫），而門比素牆多長出來的那一小段邊緣在門那張是牆（畫）。
// 少了任何一邊都會有一批座標對不上 —— 只看門那張 197 個像素、
// 只看素牆那張 10 個。
func (s *Screen) BlitMask(src, mask *image.Paletted, x, y int, key uint8) {
	b := src.Bounds()
	mb := image.Rectangle{}
	if mask != nil {
		mb = mask.Bounds()
	}
	for sy := 0; sy < b.Dy(); sy++ {
		dy := y + sy
		if dy < 0 || dy >= OrigH {
			continue
		}
		for sx := 0; sx < b.Dx(); sx++ {
			dx := x + sx
			if dx < 0 || dx >= OrigW {
				continue
			}
			c := src.ColorIndexAt(sx, sy)
			if c == key {
				if mask == nil || sx >= mb.Dx() || sy >= mb.Dy() ||
					mask.ColorIndexAt(mb.Min.X+sx, mb.Min.Y+sy) == key {
					continue
				}
			}
			s.Orig.SetColorIndex(dx, dy, c)
		}
	}
}

// BlitHiFit 把一張**原尺寸**的圖以 nearest 縮放填滿高解析層的一塊矩形。
// x/y/w/h 用原版座標，實際畫的是它乘上 Scale 的那塊。
//
// 用途是「自成一套解析度的素材」：MSX 的第一人稱視圖只有 154×64，
// 與 DOS 那個 208×120 的框既不是整數倍也不是同一個長寬比。**先把整幅
// 合成好再縮放一次**，而不是每一塊各自縮放 —— 後者每塊各自四捨五入，
// 相鄰的兩塊之間會裂出一兩個像素的縫，那看起來像素材有破洞。
// **與 BlitHi 一樣要排隊**：`Flush` 之後才畫。直接寫 Hi 的話會被
// `Flush` 從原版層整片蓋掉，症狀是視圖區全黑 —— 那看起來像素材沒載到。
func (s *Screen) BlitHiFit(src *image.Paletted, x, y, w, h int) {
	if src == nil || w <= 0 || h <= 0 {
		return
	}
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return
	}
	// 畫布是共用的（每一格重畫），排隊到 Flush 才用 —— 要留一份自己的。
	cp := image.NewPaletted(image.Rect(0, 0, b.Dx(), b.Dy()), src.Palette)
	copy(cp.Pix, src.Pix)
	s.hi = append(s.hi, hiSprite{im: cp, x: x, y: y, key: -1, fitW: w, fitH: h})
}

// BlitHi 把一張**已經放大 Scale 倍**的貼圖排進高解析層，座標仍用原版座標。
//
// 為什麼要排隊而不是直接畫：`Flush` 會把整個原版層重新蓋到 Hi 上，
// 在它之前畫的東西一律被洗掉。排隊讓呼叫端可以照原本的順序畫（先遠後近、
// 牆疊在地板上），不必把繪圖流程拆成「Flush 前」與「Flush 後」兩段 ——
// 那種拆法會讓遮蔽關係散在兩個地方，加一張新貼圖就要重想一次順序。
func (s *Screen) BlitHi(src *image.Paletted, x, y int) {
	s.blitHi(src, x, y, -1)
}

// BlitHiKey 與 BlitHi 相同，但跳過等於 key 的像素。
func (s *Screen) BlitHiKey(src *image.Paletted, x, y int, key uint8) {
	s.blitHi(src, x, y, int(key))
}

// BlitHiMask 是 BlitMask 的高解析版。mask 要與 src 同倍率。
func (s *Screen) BlitHiMask(src, mask *image.Paletted, x, y int, key uint8) {
	if src == nil {
		return
	}
	s.hi = append(s.hi, hiSprite{im: src, mask: mask, x: x, y: y, key: int(key)})
}

func (s *Screen) blitHi(src *image.Paletted, x, y, key int) {
	if src == nil {
		return
	}
	s.hi = append(s.hi, hiSprite{im: src, x: x, y: y, key: key})
}

// Flush 把原版層以 nearest-neighbor 放大到高解析層，再補上排隊中的
// 高解析貼圖。一律在畫中文之前呼叫：中文層畫完再 Flush 會把中文洗掉。
func (s *Screen) Flush() {
	defer s.flushHi()
	pal := s.Orig.Palette
	for y := 0; y < OrigH; y++ {
		for x := 0; x < OrigW; x++ {
			c := pal[s.Orig.ColorIndexAt(x, y)]
			r, g, b, a := c.RGBA()
			px := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
			for dy := 0; dy < Scale; dy++ {
				for dx := 0; dx < Scale; dx++ {
					s.Hi.SetRGBA(x*Scale+dx, y*Scale+dy, px)
				}
			}
		}
	}
}

// flushHi 把排隊中的高解析貼圖畫上去並清空佇列。
//
// **清空是必要的**：不清的話下一影格會把上一影格的貼圖再畫一次，
// 而症狀是「畫面看起來對，記憶體慢慢長大」——不會有任何錯誤訊息。
func (s *Screen) flushHi() {
	for _, sp := range s.hi {
		b := sp.im.Bounds()
		if sp.fitW > 0 && sp.fitH > 0 {
			s.fitInto(sp)
			continue
		}
		for sy := 0; sy < b.Dy(); sy++ {
			dy := sp.y*Scale + sy
			if dy < 0 || dy >= HiH {
				continue
			}
			for sx := 0; sx < b.Dx(); sx++ {
				dx := sp.x*Scale + sx
				if dx < 0 || dx >= HiW {
					continue
				}
				ci := sp.im.ColorIndexAt(b.Min.X+sx, b.Min.Y+sy)
				if sp.key >= 0 && int(ci) == sp.key {
					m := sp.mask
					if m == nil {
						continue
					}
					mb := m.Bounds()
					if sx >= mb.Dx() || sy >= mb.Dy() ||
						int(m.ColorIndexAt(mb.Min.X+sx, mb.Min.Y+sy)) == sp.key {
						continue
					}
				}
				r, g, bl, a := sp.im.Palette[ci].RGBA()
				s.Hi.SetRGBA(dx, dy,
					color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(a >> 8)})
			}
		}
	}
	s.hi = s.hi[:0]
}

// fitInto 把一張圖以 nearest 縮放填滿 fitW×fitH（原版座標）那一塊。
func (s *Screen) fitInto(sp hiSprite) {
	b := sp.im.Bounds()
	dw, dh := sp.fitW*Scale, sp.fitH*Scale
	for dy := 0; dy < dh; dy++ {
		ty := sp.y*Scale + dy
		if ty < 0 || ty >= HiH {
			continue
		}
		sy := b.Min.Y + dy*b.Dy()/dh
		for dx := 0; dx < dw; dx++ {
			tx := sp.x*Scale + dx
			if tx < 0 || tx >= HiW {
				continue
			}
			sx := b.Min.X + dx*b.Dx()/dw
			r, g, bl, a := sp.im.Palette[sp.im.ColorIndexAt(sx, sy)].RGBA()
			s.Hi.SetRGBA(tx, ty,
				color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(a >> 8)})
		}
	}
}

// Fit 算出把整張畫面塞進 w×h 的等比例倍率與置中位移。
//
// 取兩軸較小的倍率，所以**長寬比永遠不變**（多出來的一邊留黑邊）。
// 原版像素與中文疊加層是同一張圖，倍率與位移對兩層一致 ——
// 中文不可能相對原版像素跑掉。這是把疊加層做在畫布裡而不是做在
// 視窗上的直接好處。
func Fit(w, h int) (scale, ox, oy float64) {
	if w < 1 || h < 1 {
		return 1, 0, 0
	}
	scale = float64(w) / float64(HiW)
	if sy := float64(h) / float64(HiH); sy < scale {
		scale = sy
	}
	if scale <= 0 {
		scale = 1
	}
	return scale, (float64(w) - float64(HiW)*scale) / 2,
		(float64(h) - float64(HiH)*scale) / 2
}
