package game

// Rand 是原版的隨機數產生器，逐行照 `sub_11C88` 重寫。
//
//	mov     ax, ds:4A14h      ; 種子
//	add     ax, 9248h
//	ror     ax, 1  ×3
//	xor     ax, 9248h
//	add     ax, 11h
//	mov     ds:4A14h, ax      ; 寫回
//	and     ax, 7FFFh
//	...
//	sub     cx, bx            ; cx = hi - lo
//	inc     cx                ; +1
//	xor     dx, dx
//	idiv    cx                ; dx = 餘數
//	add     dx, bx            ; + lo
//
// 種子是 16-bit，開機時由 DOS 時鐘播種（`sub_11CCC`：`int 21h` AH=2Ch，
// 把 DX 與 CX 加進種子）。同一顆種子必然給出同一串數列 ——
// remake 用同一套算法，隨機行為就與原版一致，存檔重播也對得起來。
type Rand struct {
	seed uint16
}

// NewRand 用指定的種子建立產生器。
func NewRand(seed uint16) *Rand { return &Rand{seed: seed} }

// Seed 重設種子，對應原版緊接在 `sub_11C88` 之後那個只做
// `mov ds:4A14h, ax` 的小程序。
func (r *Rand) Seed(s uint16) { r.seed = s }

// SeedFromClock 模擬 `sub_11CCC` 的播種：把時鐘的兩個值加進種子。
// 原版取的是 DOS 時間的 DX（秒與百分秒）與 CX（時與分）。
func (r *Rand) SeedFromClock(dx, cx uint16) { r.seed += dx; r.seed += cx }

// next 推進種子並回傳 15-bit 的隨機值。
func (r *Rand) next() uint16 {
	ax := r.seed
	ax += 0x9248
	ax = ax>>3 | ax<<13 // ror ax, 1 做三次
	ax ^= 0x9248
	ax += 0x11
	r.seed = ax
	return ax & 0x7FFF
}

// Range 回傳 [lo, hi] 之間的隨機數，兩端都含。
//
// 原版的三個邊界處理照抄：hi 為 0 時當成 1、lo 大於 hi 時對調、
// 範圍大小是 hi-lo+1。
func (r *Rand) Range(lo, hi int) int {
	v := int(r.next())
	if hi == 0 {
		hi = 1
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	return v%(hi-lo+1) + lo
}

// Seedof 回傳目前的種子，存檔時要一併寫進去才能重播。
func (r *Rand) Seedof() uint16 { return r.seed }
