// Package items 讀 ITEMS.DAT。
//
// 256 筆定長記錄，每筆 20 bytes：前 12 是名稱（空白補齊、直接 ASCII，
// 不像 MONSTERS.DAT 那樣整串加 0x80），其後 8 bytes 是屬性。
//
// 屬性的欄位還沒解，所以整筆 Raw 都留著 —— 未知的位元組要能原樣送回去。
package items

import (
	"errors"
	"strings"
)

const (
	// RecordSize 是一筆物品記錄的長度。
	RecordSize = 20
	// Count 是物品筆數。
	Count = 256
	// nameSize 是名稱欄的長度。
	nameSize = 12
)

// Item 是一件物品。
type Item struct {
	Index int
	Name  string
	Raw   [RecordSize]byte
}

// Attrs 回傳名稱之後那 8 個還沒解的位元組。
func (it Item) Attrs() []byte { return it.Raw[nameSize:] }

// Parse 解開 ITEMS.DAT。檔案未壓縮。
func Parse(blob []byte) ([]Item, error) {
	if len(blob) != RecordSize*Count {
		return nil, errors.New("ITEMS.DAT 長度不對，應該是 5120 bytes")
	}
	out := make([]Item, Count)
	for i := range out {
		it := Item{Index: i}
		copy(it.Raw[:], blob[i*RecordSize:(i+1)*RecordSize])
		it.Name = decodeName(it.Raw[:nameSize])
		out[i] = it
	}
	return out, nil
}

func decodeName(b []byte) string {
	if i := indexZero(b); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(string(b))
}

func indexZero(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i
		}
	}
	return -1
}
