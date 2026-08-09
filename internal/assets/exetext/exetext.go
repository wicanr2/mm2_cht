// Package exetext 讀 MM2.EXE 尾部的字串。
//
// `MM2.EXE` 的 MZ header 只宣告 34,320 bytes，實體檔案有 77,824 ——
// 尾部那 43,472 bytes 不在 MZ image 內，是 **DGROUP 的初值段**：
// 城鎮名、職業、種族、陣營、次要技能、陷阱訊息、戰鬥播報都在這裡，
// 遊戲的查表也在這裡。
//
// 對應關係是 `EXE 檔內偏移 = DGROUP 偏移 + 0x8630`，跨九張表驗過，
// 每張在整個 EXE 裡都只命中一處（docs/formats/01 §2.9）。
//
// 這一段在 IDA 裡看起來是 BSS（`db N dup(?)`），因為 IDA 只載入 MZ image。
// 判斷某塊資料有沒有初值，要看實體檔案，不是看反組譯的段落屬性。
package exetext

import (
	"errors"
	"fmt"
	"strings"
)

// DGroupBase 是 DGROUP 初值段在 MM2.EXE 裡的起點。
const DGroupBase = 0x8630

// MinSize 是含尾部資料區的 MM2.EXE 應有的最小長度。
const MinSize = DGroupBase + 0x1000

// String 是一條字串與它的 DGROUP 偏移。
//
// 用 DGROUP 偏移當識別，不用序號 —— 抽取規則哪天調整了，序號會整批位移，
// 偏移不會。
type String struct {
	Offset int
	Text   string
}

// Key 回傳穩定的識別字串，格式 `exe.XXXX`。
func (s String) Key() string { return fmt.Sprintf("exe.%04X", s.Offset) }

// Parse 抽出尾部資料區裡所有 NUL 結尾的可見字串。
//
// 條件：長度至少 2、全部是可列印 ASCII、至少含一個英文字母、
// 而且不是檔名。
//
// 後兩條擋掉不是「文字」的東西：純標點的欄位（像 UI 用的點線
// `............`）與遊戲自己要開的檔名（`monsters.16`、`eventsi.dat`）。
// 檔名翻了會讓遊戲開不了檔，留在待譯清單裡則永遠翻不完。
func Parse(exe []byte) ([]String, error) {
	if len(exe) < MinSize {
		return nil, errors.New("檔案太短，沒有尾部資料區；這不是完整的 MM2.EXE")
	}
	tail := exe[DGroupBase:]
	var out []String
	for i := 0; i < len(tail); {
		j := indexByte(tail, i, 0)
		if j < 0 {
			break
		}
		if s := tail[i:j]; qualifies(s) {
			out = append(out, String{Offset: i, Text: string(s)})
		}
		i = j + 1
	}
	if len(out) == 0 {
		return nil, errors.New("尾部資料區裡一條字串都沒有，位移可能不對")
	}
	return out, nil
}

// At 回傳指定 DGROUP 偏移的字串。遊戲要用指標表取字串時走這裡。
func At(exe []byte, off int) (string, error) {
	i := DGroupBase + off
	if i < 0 || i >= len(exe) {
		return "", fmt.Errorf("DGROUP 偏移 %#x 超出檔案範圍", off)
	}
	j := indexByte(exe, i, 0)
	if j < 0 {
		return "", fmt.Errorf("DGROUP 偏移 %#x 之後沒有結尾符", off)
	}
	return string(exe[i:j]), nil
}

func indexByte(b []byte, from int, c byte) int {
	for i := from; i < len(b); i++ {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func qualifies(s []byte) bool {
	if len(s) < 2 {
		return false
	}
	letter := false
	for _, c := range s {
		if c < 0x20 || c > 0x7E {
			return false
		}
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
			letter = true
		}
	}
	return letter && !isFilename(string(s))
}

// dataExt 是遊戲自己會去開的副檔名，全部小寫 —— EXE 裡的檔名是小寫的，
// 只有 overlay 名是大寫（見 CLAUDE.md §3）。
var dataExt = []string{".16", ".dat", ".drv", ".ch", ".ovl", ".exe", ".com"}

func isFilename(s string) bool {
	if strings.ContainsAny(s, " \t") {
		return false
	}
	low := strings.ToLower(s)
	for _, ext := range dataExt {
		if strings.HasSuffix(low, ext) {
			return true
		}
	}
	return false
}
