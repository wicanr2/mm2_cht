// Package monsters 解析 MONSTERS.DAT。
package monsters

import (
	"fmt"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

// RecordSize 是一筆怪物記錄的長度。
//
// 解壓後 6,656 bytes = 26 × 256。stride 26 而不是別的整除數，
// 是因為名稱欄位在每筆的同一個位置對得整整齊齊 —— 換成 52 就會變成
// 「一筆裡有兩個名字」。
const RecordSize = 26

// Count 是怪物筆數。
const Count = 256

// Monster 是一筆怪物資料。
type Monster struct {
	Index int
	Name  string

	// Stats 是名稱之後那 12 個位元組，語意尚未解出。
	// 已知 +0x15（Stats[7]）在前幾筆是遞增的小數字，很可能是
	// MONSTERS.16 的影像索引，待驗。
	Stats [12]byte
}

// Parse 解出全部 256 筆。
//
// 名稱是**每個位元組加了 0x80** 的 ASCII，14 bytes、空格填充。
// 這也是為什麼直接 grep `Goblin` 找不到東西 —— 檔案裡存的是 `0xc7 0xef...`。
func Parse(blob []byte) ([]Monster, error) {
	raw, err := lzw.Segment(blob, 0)
	if err != nil {
		return nil, fmt.Errorf("解壓 MONSTERS.DAT: %w", err)
	}
	if len(raw) != RecordSize*Count {
		return nil, fmt.Errorf("解出 %d bytes，預期 %d", len(raw), RecordSize*Count)
	}
	out := make([]Monster, 0, Count)
	for i := 0; i < Count; i++ {
		r := raw[i*RecordSize : (i+1)*RecordSize]
		m := Monster{Index: i, Name: decodeName(r[:14])}
		copy(m.Stats[:], r[14:])
		out = append(out, m)
	}
	return out, nil
}

// decodeName 把 +0x80 的名稱還原成 ASCII。
func decodeName(b []byte) string {
	s := make([]byte, len(b))
	for i, c := range b {
		if c >= 0x80 {
			c -= 0x80
		}
		s[i] = c
	}
	return strings.TrimRight(string(s), " ")
}
