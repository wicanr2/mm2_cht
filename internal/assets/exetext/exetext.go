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
	"sort"
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
	for _, off := range knownExtra {
		if off < 0 || off >= len(tail) {
			continue
		}
		j := indexByte(tail, off, 0)
		if j < 0 {
			continue
		}
		if s := tail[off:j]; qualifies(s) {
			out = append(out, String{Offset: off, Text: string(s)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Offset < out[j].Offset })
	if len(out) == 0 {
		return nil, errors.New("尾部資料區裡一條字串都沒有，位移可能不對")
	}
	return out, nil
}

// knownExtra 是掃不到、但已經從反組譯確認位置的字串。
//
// 掃描是「NUL 到 NUL 之間全部可列印才算一條」。指標表的位元組夾在中間時
// 整段都會被丟掉，連帶把**接在指標表後面的那條字串**一起丟掉 ——
// 而那條字串在遊戲裡照樣會顯示，只是翻譯管線看不到它，沒有任何徵兆。
//
// **只有位置已經被證明的才可以進這張表。** 試過兩種自動補救都不行：
// 「取最後一個不可列印之後的字」會把指標表的高位元組黏在開頭（`" Wooden Crate `），
// 位移差一個位元組，譯文就永遠對不上；「推進到某個指標的目標」會停在
// 字串中間（`on Spells`、`ell Book`），因為那些位置只是碰巧被指到。
// 位移錯一個位元組的條目比沒有這條目更糟：它看起來翻好了，實際上不會生效。
var knownExtra = []int{
	// 箱子名稱表 `ds:28A2` 的第一格。表在 `2MISC.OVL` 的 `_2misc_e02` 被讀，
	// 其餘 39 格都掃得到，只有這一格接在指標表後面。
	0x22BA,

	// 建角色那一頁（`1MENU2.OVL`）。四條都接在二進位後面：
	//
	//	07A4  "A - Might.......="  七個屬性標籤的第一條。證據是 `ds:0876`
	//	                           那張七筆指標表的第一項就是 07A4，
	//	                           其餘六條（07B6…0810）間隔 18 也吻合
	//	0884  "Exchange Stat (x) with stat (A-G)?  "
	//	                           接在上面那張七筆表之後：0876 + 14 = 0884
	//	08AC  "Class= "            接在 `ds:08AA` 那個指向 0884 的 word 之後
	//	095F  "(Create New Characters)"
	//	                           前面是一段旗標，最後一個位元組 0xA0 不可列印
	0x07A4,
	0x0884,
	0x08AC,
	0x095F,

	// 4DFB  "Darkness"  視野全黑時印在視圖中間那一行。
	//
	// 它接在四張視窗矩形表（`ds:4DD4`／`4DDE`／`4DE8`／`4DF2`，各 10 個
	// 位元組）後面，所以掃描器連著那段二進位一起丟掉。位置是**指出來的**
	// 不是推的：root `sub_13FFC` 的 `0x14054` 就是
	// `mov ax, 4DFBh : push ax : call sub_11726`，而同一支在那之前先清掉
	// 視窗 8（＝ 第一人稱那一塊）。
	0x4DFB,
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
		// 換行是遊戲真的會印的字元（主選單那幾條、輸入姓名的提示都帶 `\n`）。
		// 把它一併擋掉會讓那些字串從此不存在，而且沒有任何徵兆。
		if c == '\n' {
			continue
		}
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
