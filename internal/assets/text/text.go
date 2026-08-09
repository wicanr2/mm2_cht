// Package text 解析 STR.DAT：遊戲的長文字（劇情、對話、選單、結局、謎題）。
//
// 格式與驗證見 docs/formats/05-text-system.md。
package text

import (
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

// Shift 是 STR.DAT 的位移量。加上去之後整份直接可讀 —— 大小寫、標點、
// 引號都正確。用別的位移量也能掃出一堆英文單字（例如 −4 會讓字母落在
// 大寫 ASCII 範圍），但句首會變成 `!`/`#`/`)` 這類符號，那是解錯的徵兆。
const Shift = 0x1C

// LineBreak 是解密後唯一剩下的控制碼。
const LineBreak = 0x1D

// Parse 解出 STR.DAT 的每一行。空行是訊息之間的分隔，一併保留 ——
// 訊息的索引方式還沒追到，行號是目前唯一穩定的定位方式。
func Parse(blob []byte) ([]string, error) {
	raw, err := lzw.Segment(blob, 0)
	if err != nil {
		return nil, err
	}
	dec := make([]byte, len(raw))
	for i, b := range raw {
		dec[i] = b + Shift
	}
	return strings.Split(string(dec), string(rune(LineBreak))), nil
}
