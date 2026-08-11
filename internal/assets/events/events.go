// Package events 解析 EVENTSI.DAT / EVENTSO.DAT。
//
// 兩個檔的結構相同，原版依旗標把檔名第 7 個字元填成 i 或 o
// （indoor / outdoor）走同一條載入路徑。
//
//	uint32 offsets[71]     段索引，0 表示空槽
//	每段（LZW 解開後）：
//	    事件表      3 bytes/筆，以 00 00 00 結束
//	    uint16      skip，從事件表結束處算到字串區的距離
//	    腳本區      事件表結束 +2 起，0xFF 分隔的變長序列
//	    字串區      事件表結束 + skip 起，0xFF 分隔
//
// 段的佈局讀自 2PLAY.OVL 的 sub_1A85C：它從偏移 0 開始每次讀 3 個位元組，
// 三個都為 0 才跳出，接著讀兩個位元組組成 skip，用
// 「事件表結束 + skip」算出字串區起點（word_1042C），
// 「事件表結束 + 2」是腳本區起點（word_155BC）。
package events

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

// SegmentCount 是索引表的固定筆數。
const SegmentCount = 71

// LineBreak 是字串裡的換行符。原版用 '@'，例如
// "Drink from the fountain of@clairvoyance (y/n)?"。
//
// 出處是 2PLAY.OVL 的字串讀取常式（sub_19016）：逐位元組讀，0xFF 結束，
// 讀出來先 and 0x7F 再判斷是不是 '@'，是就換成 0x0A。
const LineBreak = '@'

// Terminator 是字串的結束位元組。
const Terminator = 0xFF

// Event 是事件表的一筆。Cell 與 Index 已由原版讀取端確認；Kind 的完整
// 語意仍待釐清。
type Event struct {
	Cell  byte // 格位置，0–255 對應 16×16；同一段內遞增
	// Index 是 `sub_1A606` 的腳本段號。一般事件段的腳本區先有一個
	// 0xFF，故 Scripts[Index] 就是原版跳過 Index 個分隔符後的位置。
	// 腳本庫則由特殊設施的 1 起算 selector 呼叫，解析器已去掉那個
	// 開頭分隔符，兩種索引不可混用。
	Index byte
	Kind  byte // 觀察到的值都是 16 的倍數，低 nibble 恆為 0；高 nibble 是類型
}

// Segment 是一個地點（城鎮或區域）的事件資料。
// Script 保留原樣，未解的欄位才能原樣往返。
type Segment struct {
	Index   int
	Events  []Event
	Script  []byte   // 腳本區原樣，0xFF 分隔的變長序列
	// Scripts 是腳本區切成段。一般事件段的 Events[i].Index 可直接當下標；
	// Library 段則由 game.LibraryScriptForFacility 回傳零起算下標。
	Scripts [][]byte
	Strings []string // 字串區，0xFF 分隔
	Raw     []byte   // 解壓後的原始位元組，供未解結構的段原樣往返

	// Library 標記這是一段**沒有事件表的腳本庫**：`uint16 skip` 之後
	// 直接是 `0xFF` 分隔的腳本，字串區在 `skip` 指到的地方。
	//
	// 編號 60 以上的段都是這樣，它們不對應地圖 —— 程式碼把 `ds:0392`
	// 暫時換成該段號、載入、用完換回來（`2PLAY.img` `0x196F6`）。
	// 城鎮裡那些「付 N 金幣就送你去某地 (y/n)」之類的共用互動就住在這裡。
	//
	// **先前這些段被歸進 Irregular 而整段跳過，於是所有掃腳本的工具
	// 都看不到它們。** 掃描類的工具要跳過某一段之前，先想清楚跳掉的
	// 是「解不出來的」還是「還沒學會解的」。
	Library bool

	// Irregular 標記這一段不符合 sub_1A85C 的佈局：事件表掃不到
	// 00 00 00 終止，或 skip 指到緩衝外。實測 71 個非空段裡有 4 段如此
	// （EVENTSI 的 63、67，EVENTSO 的 65、68），編號都在後段。
	// 這種段只抽得出字串，事件表與腳本區維持未解 —— 不猜、不硬套。
	Irregular bool
}

// Parse 解開整個事件檔。
func Parse(blob []byte) ([]Segment, error) {
	if len(blob) < SegmentCount*4 {
		return nil, fmt.Errorf("檔案只有 %d bytes，放不下 %d 筆索引", len(blob), SegmentCount)
	}
	var segs []Segment
	for i := 0; i < SegmentCount; i++ {
		off := int(binary.LittleEndian.Uint32(blob[i*4:]))
		if off == 0 {
			continue // 空槽
		}
		raw, err := lzw.Segment(blob, off)
		if err != nil {
			return nil, fmt.Errorf("段 %d @%d: %w", i, off, err)
		}
		seg, err := parseSegment(i, raw)
		if err != nil {
			return nil, err
		}
		segs = append(segs, seg)
	}
	return segs, nil
}

func parseSegment(idx int, raw []byte) (Segment, error) {
	seg := Segment{Index: idx, Raw: raw}

	// 事件表的結構條件只有一條：**Cell 在段內嚴格遞增**。加上「掃得到
	// `00 00 00` 終止」與「`skip` 落在段內」，三者一起就足以認出佈局。
	//
	// **不要拿「Kind 的低 nibble 恆為 0」當判準。** 那是從已經解得開的
	// 段觀察來的統計，而整份資料有一筆反例（`EVENTSO` 段 37 的格 98，
	// `Kind = 0xF1`）—— 拿它當過濾器，會為了一個位元丟掉一整段
	// 47 筆事件、14 條腳本、可讀的字串。低 nibble 的語意還未知。
	p, terminated := 0, false
	var evs []Event
	lastCell := -1
	for p+3 <= len(raw) {
		a, b, c := raw[p], raw[p+1], raw[p+2]
		p += 3
		if a == 0 && b == 0 && c == 0 {
			terminated = true
			break
		}
		if int(a) <= lastCell {
			break // 不符合事件表的樣子，terminated 維持 false
		}
		lastCell = int(a)
		evs = append(evs, Event{Cell: a, Index: b, Kind: c})
	}

	strAt := -1
	if terminated && p+2 <= len(raw) {
		skip := int(binary.LittleEndian.Uint16(raw[p:]))
		// skip 可以小於 2（實測 EVENTSI 段 60、EVENTSO 段 64 是 0），
		// 表示沒有腳本區、字串區緊接事件表。原版就是直接
		// word_1042C = si + skip，不做下限檢查。
		if at := p + skip; at <= len(raw) {
			strAt = at
			seg.Events = evs
			if strAt > p+2 {
				seg.Script = raw[p+2 : strAt]
				seg.Scripts = bytes.Split(seg.Script, []byte{Terminator})
			}
		}
	}

	if strAt < 0 {
		// 沒有事件表的那些段走另一套佈局：`uint16 skip` 之後直接是
		// `0xFF` 分隔的腳本區，字串區在 `skip` 指到的地方。編號 60 以上
		// 的段都是這樣 —— 它們不對應地圖，是**腳本庫**，由程式碼把
		// `ds:0392` 暫時換成該段號再載入（`2PLAY.img` `0x196F6`），
		// 用完換回來。
		//
		// 認出來的判準是位置而不是內容：`raw[2] == 0xFF`（skip 之後
		// 立刻是分隔符），而事件表佈局的第三個位元組是 Kind，
		// 低 nibble 恆為 0，不可能是 `0xFF`。
		if len(raw) > 3 && raw[2] == Terminator {
			skip := int(binary.LittleEndian.Uint16(raw))
			if at := skip; at > 3 && at <= len(raw) {
				seg.Library = true
				strAt = at
				seg.Script = raw[3:strAt]
				seg.Scripts = bytes.Split(seg.Script, []byte{Terminator})
			}
		}
	}

	if strAt < 0 {
		// 兩套佈局都不符。仍然要把字串抽出來給中文化用，
		// 但事件表與腳本區留白，標記成 Irregular。
		seg.Irregular = true
		if i := bytes.Index(raw, []byte{Terminator, Terminator}); i >= 0 {
			strAt = i + 1
		} else {
			return seg, fmt.Errorf("段 %d 既不符合事件表佈局，也找不到字串區", idx)
		}
	}

	for _, s := range bytes.Split(raw[strAt:], []byte{Terminator}) {
		if len(s) > 0 {
			seg.Strings = append(seg.Strings, string(stripHighBit(s)))
		}
	}
	return seg, nil
}

// stripHighBit 對應原版的 and 0x7F。實測 EVENTSI/EVENTSO 的字串沒有任何
// 位元組帶 bit7，所以這一步在目前的資料上是 no-op；照原版做是為了換一份
// 資料時不會默默解錯。
func stripHighBit(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		out[i] = c & 0x7F
	}
	return out
}

// Rebuild 把段組回未壓縮的位元組，供改寫字串後重新打包。
// 事件表與腳本區原樣寫回，只有字串區被換掉；skip 依腳本區長度重算。
func (s Segment) Rebuild(strs []string) []byte {
	out := make([]byte, 0, len(s.Script)+256)
	for _, e := range s.Events {
		out = append(out, e.Cell, e.Index, e.Kind)
	}
	out = append(out, 0, 0, 0)
	skip := 2 + len(s.Script)
	out = binary.LittleEndian.AppendUint16(out, uint16(skip))
	out = append(out, s.Script...)
	for i, str := range strs {
		if i > 0 {
			out = append(out, Terminator)
		}
		out = append(out, str...)
	}
	return out
}
