// Package lzw 解開 MM2 資料檔的壓縮。
//
// 演算法讀自 MM2.EXE 的 sub_12242（IDA linear 0x12242），細節與證據見
// docs/formats/03-lzw-compression.md。與 Go 標準庫的 compress/lzw 不相容：
// 原版用固定的 0x100 = CLEAR、0x101 = EOF、0x102 起動態、碼寬 9→12。
package lzw

import (
	"encoding/binary"
	"fmt"
)

const (
	clearCode = 0x100
	eofCode   = 0x101
	firstCode = 0x102
	maxWidth  = 12
	dictSize  = 1 << maxWidth
)

// bitReader 是 LSB-first 的位元流。原版一次讀三個位元組（lodsw + lodsb）
// 再右移到位，所以尾端不足三個位元組時要補零而不是報錯。
type bitReader struct {
	data []byte
	pos  int // 位元位置
}

func (r *bitReader) read(width int) int {
	byteOff, bitOff := r.pos/8, r.pos%8
	var v uint32
	for i := 0; i < 3; i++ {
		if byteOff+i < len(r.data) {
			v |= uint32(r.data[byteOff+i]) << (8 * i)
		}
	}
	r.pos += width
	return int((v >> bitOff) & (1<<width - 1))
}

func (r *bitReader) exhausted() bool { return r.pos/8 >= len(r.data) }

// Decompress 解開一段 LZW 位元流。limit 是段頭宣告的解壓後長度；
// 傳 0 表示一路解到 EOF 碼。回傳解出的資料與消耗掉的壓縮位元組數
// （多段串接的檔案要靠它找下一段）。
func Decompress(data []byte, limit int) (out []byte, used int, err error) {
	r := &bitReader{data: data}
	if limit > 0 {
		out = make([]byte, 0, limit)
	}

	var prefix [dictSize]int
	var suffix [dictSize]byte

	width, threshold, next := 9, 0x200, firstCode
	prev, lastChar := -1, byte(0)
	stack := make([]byte, 0, dictSize)

	for {
		if r.exhausted() {
			return out, (r.pos + 7) / 8, fmt.Errorf("位元流在 EOF 碼之前就用完了（已解 %d bytes）", len(out))
		}
		code := r.read(width)
		if code == eofCode {
			break
		}
		if code == clearCode {
			width, threshold, next = 9, 0x200, firstCode
			code = r.read(width)
			if code == eofCode || code == clearCode {
				break
			}
			out = append(out, byte(code))
			prev, lastChar = code, byte(code)
			if limit > 0 && len(out) >= limit {
				break
			}
			continue
		}

		cur := code
		stack = stack[:0]
		if code >= next { // KwKwK
			stack = append(stack, lastChar)
			code = prev
		}
		if code < 0 {
			return out, (r.pos + 7) / 8, fmt.Errorf("位元流以非 CLEAR 的碼 %d 開頭", cur)
		}
		for code > 0xFF {
			stack = append(stack, suffix[code])
			code = prefix[code]
		}
		stack = append(stack, byte(code))
		lastChar = byte(code)

		for i := len(stack) - 1; i >= 0; i-- {
			out = append(out, stack[i])
		}

		if prev >= 0 && next < dictSize {
			prefix[next] = prev
			suffix[next] = lastChar
			next++
			// 碼寬增長發生在新增字典項之後
			if next >= threshold && width < maxWidth {
				width++
				threshold <<= 1
			}
		}
		prev = cur

		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, (r.pos + 7) / 8, nil
}

// Segment 解開一個帶段頭的段：uint16 解壓後長度 + uint16 0 + LZW 流。
func Segment(blob []byte, off int) ([]byte, error) {
	if off+4 > len(blob) {
		return nil, fmt.Errorf("段起點 %d 超出檔案長度 %d", off, len(blob))
	}
	size := int(binary.LittleEndian.Uint16(blob[off:]))
	// 段頭的第二個 word 在所有 LZW 段都是 0。不是 0 就代表這裡根本不是段頭
	// （SPELLS.DAT、ITEMS.DAT 等未壓縮檔的開頭形狀很像段頭，會被誤解成
	// 「解出剛好正確長度」的垃圾）。
	if w1 := binary.LittleEndian.Uint16(blob[off+2:]); w1 != 0 {
		return nil, fmt.Errorf("偏移 %d 的段頭第二個 word 是 %#x，不是 LZW 段", off, w1)
	}
	out, _, err := Decompress(blob[off+4:], size)
	if err != nil {
		return out, err
	}
	if len(out) != size {
		return out, fmt.Errorf("段頭宣告 %d bytes，實際解出 %d bytes", size, len(out))
	}
	return out, nil
}
