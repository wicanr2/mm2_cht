// Package events 解析 EVENTSI.DAT / EVENTSO.DAT。
//
// 兩個檔的結構相同，原版依旗標把檔名第 7 個字元填成 i 或 o
// （indoor / outdoor）走同一條載入路徑。
//
//	uint32 offsets[71]     段索引，0 表示空槽
//	每段（LZW）：
//	    事件表           3 bytes/筆，第一個 byte 是格位置，遞增
//	    腳本區           0xFF 分隔的變長序列
//	    0xFF 0xFF        字串表起點標記
//	    字串表           0xFF 分隔
//
// 事件表與腳本區的欄位語意尚未確定（見 CONTEXT.md §3），
// 但字串表的邊界在 71 個段上都由 0xFF 0xFF 準確界定，中文化可以先動。
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

// stripHighBit 對應原版的 and 0x7F。實測 EVENTSI/EVENTSO 的 1,308 條字串
// 沒有任何位元組帶 bit7，所以這一步在目前的資料上是 no-op；照原版做是為了
// 換一份資料時不會默默解錯。
func stripHighBit(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		out[i] = c & 0x7F
	}
	return out
}

// Segment 是一個地點（城鎮或區域）的事件資料。
// Body 保留字串表之前的全部位元組原樣，未解的欄位才能原樣往返。
type Segment struct {
	Index   int
	Body    []byte   // 事件表 + 腳本區，含 0xFF 0xFF 標記
	Strings []string // 以 0xFF 分隔
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
	p := bytes.Index(raw, []byte{0xFF, 0xFF})
	if p < 0 {
		return Segment{}, fmt.Errorf("段 %d 找不到字串表起點標記 FF FF", idx)
	}
	seg := Segment{Index: idx, Body: raw[:p+2]}
	for _, s := range bytes.Split(raw[p+2:], []byte{Terminator}) {
		if len(s) > 0 {
			seg.Strings = append(seg.Strings, string(stripHighBit(s)))
		}
	}
	return seg, nil
}

// Rebuild 把段組回未壓縮的位元組，供改寫字串後重新打包。
// Body 原樣寫回，只有字串表被換掉。
func (s Segment) Rebuild(strs []string) []byte {
	out := make([]byte, 0, len(s.Body)+64)
	out = append(out, s.Body...)
	for i, str := range strs {
		if i > 0 {
			out = append(out, 0xFF)
		}
		out = append(out, str...)
	}
	return out
}
