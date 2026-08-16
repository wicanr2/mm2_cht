package lzw

import "encoding/binary"

// bitWriter 是 LSB-first 的位元流，與 bitReader 對稱。
type bitWriter struct {
	data []byte
	pos  int // 位元位置
}

func (w *bitWriter) write(v, width int) {
	for i := 0; i < width; i++ {
		if v&(1<<i) != 0 {
			byteOff := (w.pos + i) / 8
			for len(w.data) <= byteOff {
				w.data = append(w.data, 0)
			}
			w.data[byteOff] |= 1 << ((w.pos + i) % 8)
		}
	}
	w.pos += width
	for len(w.data) < (w.pos+7)/8 {
		w.data = append(w.data, 0)
	}
}

// Compress 壓成原版解得開的 LZW 位元流。
//
// **這不是「還原出與原版逐位元組相同的輸出」** —— 壓縮端的選擇（何時送
// CLEAR、字典滿了怎麼辦）在解壓端看不出來，所以同一份資料有很多種合法
// 編碼。這裡的驗收條件是**「原版的解壓常式解得回去」**，由
// `Decompress` 逐段往返驗證（`compress_test.go` 對 EVENTSI／EVENTSO 的
// 全部段各跑一次）。
//
// 三件事要與 Decompress 對稱，錯一個就會在解壓的中途才炸開，
// 而且症狀是「解出來的長度不對」而不是「這裡有問題」：
//
//   - 碼寬增長發生在**新增字典項之後**，判準是 `next >= threshold`；
//   - 字典滿了（0x1000）就**不再新增**，碼寬停在 12，不送 CLEAR；
//   - 開頭先送一個 CLEAR，接著第一個位元組當字面碼送出。
func Compress(src []byte) []byte {
	w := &bitWriter{}
	if len(src) == 0 {
		w.write(clearCode, 9)
		w.write(eofCode, 9)
		return w.data
	}

	// 字典：把 (前綴碼, 尾字元) 對到碼。最多 4096 項，用 map 比開
	// 4096×256 的陣列省。
	type key struct {
		prefix int
		suffix byte
	}
	dict := make(map[key]int, dictSize)

	// **兩個計數器不是同一個。** `next` 是這一端的字典編號；`dnext` 是
	// 解壓端的，它比這一端**慢一個碼** —— 解壓端要讀到下一個碼才知道
	// 前一項的尾字元，所以字典項是在「讀完第 k 個碼之後」才補上的。
	// 碼寬跟著解壓端走，跟錯就會在中途多讀／少讀一位，症狀是「解出來的
	// 長度不對」而不是「這裡有問題」。
	width, threshold := 9, 0x200
	dnext, next, emitted := firstCode, firstCode, 0

	put := func(code int) {
		if emitted >= 2 && dnext < dictSize {
			dnext++
			if dnext >= threshold && width < maxWidth {
				width++
				threshold <<= 1
			}
		}
		w.write(code, width)
		emitted++
	}

	w.write(clearCode, width) // CLEAR 本身不算一個碼：解壓端讀完就重置
	prev := int(src[0])

	for _, c := range src[1:] {
		k := key{prev, c}
		if code, ok := dict[k]; ok {
			prev = code
			continue
		}
		put(prev)
		if next < dictSize {
			dict[k] = next
			next++
		}
		prev = int(c)
	}
	put(prev)
	put(eofCode)
	return w.data
}

// PackSegment 包成帶段頭的一段：uint16 解壓後長度 + uint16 0 + LZW 流。
func PackSegment(src []byte) []byte {
	out := make([]byte, 4, 4+len(src))
	binary.LittleEndian.PutUint16(out[0:], uint16(len(src)))
	binary.LittleEndian.PutUint16(out[2:], 0)
	return append(out, Compress(src)...)
}
