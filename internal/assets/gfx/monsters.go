package gfx

import (
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

// monsters.16 是唯一一個帶自己索引表的 .16。整條鏈是：
//
//	MONSTERS.DAT 記錄 +21 的低 7 位元 = 圖號（1–60）
//	→ 索引 = 圖號 − 1
//	→ monsters.16 開頭 75 個 uint32 的索引表；值為 0 就往後找下一個非零，
//	  掃到第 75 項回繞 0
//	→ 該偏移是一個 LZW 段
//	→ 段內：uint16 影格數、uint16 影格偏移[n]、動畫表、影格
//	→ 每個影格：uint8 x, y, w, h ＋ RLE 像素
//
// 索引表與回退規則抄自 root 的 `sub_6818`（`2PLAY.img` 0x6818）：它 open
// `ds:04C2` 指的檔名（`monsters.16`）、讀 0x12C = 300 bytes 的索引表進
// `ds:9E48`，然後
//
//	si = 索引 × 4
//	dx = word[9E48+si]；cx = word[9E48+si+2]
//	兩個都是 0 → si += 4、索引 += 1、索引 < 0x4B 就再試一次，否則歸零重來
//
// 也就是說**圖號指到空槽不是錯誤，是往後借下一張圖**。75 個槽裡有 16 個空的，
// 所以像「老守財奴」（圖號 7）與「哥布林」（圖號 8）會共用同一張。
// 只看檔案猜不出這條規則 —— 空槽長得像壞掉的資料。

// MonsterPicCount 是索引表的項數。
const MonsterPicCount = 75

// monsterIndexBytes 是索引表的長度，原版一次讀這麼多（`mov cx, 0x12C`）。
const monsterIndexBytes = MonsterPicCount * 4

// TransparentIndex 是怪物圖的背景色。59 張基準圖的左上角全部是 5，
// 而 5 在 EGA 標準調色盤是洋紅 —— 戰鬥畫面上不會有整片洋紅背景。
const TransparentIndex = 5

// Frame 是怪物圖的一個影格。像素已經展開成「一個位元組一個顏色索引」，
// 不是 4bpp packed —— 影格要疊在別的影格上，packed 反而礙事。
type Frame struct {
	X, Y          int
	Width, Height int
	Pixels        []byte
}

// At 回傳影格內 (x, y) 的顏色索引，出界回傳透明色。
func (f Frame) At(x, y int) byte {
	if x < 0 || y < 0 || x >= f.Width || y >= f.Height {
		return TransparentIndex
	}
	i := y*f.Width + x
	if i >= len(f.Pixels) {
		return TransparentIndex
	}
	return f.Pixels[i]
}

// AnimStep 是動畫的一步：顯示哪個影格、停留多久。
type AnimStep struct {
	Frame int
	Hold  int
	// Flag 是影格編號的 bit 7。59 個槽裡有 24 個用到，語意未知 ——
	// 可能是水平翻轉或音效觸發。原樣留著，不要當成編號的一部分。
	Flag bool
}

// MonsterPic 是一個槽的全部內容。
//
// Frames[0] 是 84×86 的基準圖，其餘是疊在上面的動畫零件
// （各自帶 X/Y 偏移，尺寸小得多）。
type MonsterPic struct {
	Slot   int
	Frames []Frame
	Anims  [][]AnimStep

	// AnimHeader 是動畫表第一個 0xFF 之前的位元組，**語意未知**。
	// 長度不固定（2 到 14 都有），常以 `8X 00` 收尾，但槽 9 是
	// `2F 0A 10 3B 06 04 83 00`，套 (影格, 停留) 讀會得到 47 號影格。
	// 原樣留著，不要猜。
	AnimHeader []byte
}

// MonsterIndex 讀 monsters.16 開頭的 75 項索引表。值 0 表示空槽。
func MonsterIndex(blob []byte) ([]int, error) {
	if len(blob) < monsterIndexBytes {
		return nil, fmt.Errorf("monsters.16 只有 %d bytes，放不下 %d bytes 的索引表",
			len(blob), monsterIndexBytes)
	}
	out := make([]int, MonsterPicCount)
	for i := range out {
		out[i] = int(binary.LittleEndian.Uint32(blob[i*4:]))
		if out[i] > len(blob) {
			return nil, fmt.Errorf("索引表第 %d 項是 %d，超出檔案長度 %d", i, out[i], len(blob))
		}
	}
	if out[0] != monsterIndexBytes {
		return nil, fmt.Errorf("索引表第 0 項是 %d，預期 %d（表本身的長度）",
			out[0], monsterIndexBytes)
	}
	return out, nil
}

// ResolveMonsterPic 把記錄裡的圖號換成索引表的槽號，含原版的回退。
// 整張表都是空的才回傳 -1。
func ResolveMonsterPic(index []int, pic int) int {
	if len(index) == 0 {
		return -1
	}
	i := (pic - 1) % len(index)
	if i < 0 {
		i += len(index)
	}
	for n := 0; n < len(index); n++ {
		if index[i] != 0 {
			return i
		}
		i = (i + 1) % len(index)
	}
	return -1
}

// ParseMonsterPic 解出一個槽的影格與動畫。slot 是索引表的項次，
// 要先用 ResolveMonsterPic 把圖號換過來。
func ParseMonsterPic(blob []byte, slot int) (MonsterPic, error) {
	index, err := MonsterIndex(blob)
	if err != nil {
		return MonsterPic{}, err
	}
	if slot < 0 || slot >= len(index) || index[slot] == 0 {
		return MonsterPic{}, fmt.Errorf("槽 %d 是空的", slot)
	}
	raw, err := lzw.Segment(blob, index[slot])
	if err != nil {
		return MonsterPic{}, fmt.Errorf("槽 %d 解壓失敗：%w", slot, err)
	}
	pic := MonsterPic{Slot: slot}
	if len(raw) < 4 {
		return pic, fmt.Errorf("槽 %d 只解出 %d bytes", slot, len(raw))
	}
	n := int(binary.LittleEndian.Uint16(raw))
	if n == 0 || 2+n*2 > len(raw) {
		return pic, fmt.Errorf("槽 %d 宣告 %d 個影格，與 %d bytes 不合", slot, n, len(raw))
	}
	offs := make([]int, n)
	for i := range offs {
		offs[i] = int(binary.LittleEndian.Uint16(raw[2+i*2:]))
	}
	for i, off := range offs {
		if off+4 > len(raw) {
			return pic, fmt.Errorf("槽 %d 影格 %d 的偏移 %d 超出 %d bytes", slot, i, off, len(raw))
		}
		end := len(raw)
		if i+1 < len(offs) && offs[i+1] > off && offs[i+1] <= len(raw) {
			end = offs[i+1]
		}
		f := Frame{
			X: int(raw[off]), Y: int(raw[off+1]),
			Width: int(raw[off+2]), Height: int(raw[off+3]),
		}
		f.Pixels = DecodeRLE(raw[off+4:end], f.Width, f.Height)
		pic.Frames = append(pic.Frames, f)
	}
	tbl := raw[2+n*2 : offs[0]]
	if i := indexByte(tbl, 0xFF); i >= 0 {
		pic.AnimHeader = tbl[:i]
		pic.Anims = parseAnims(tbl[i:])
	} else {
		pic.AnimHeader = tbl
	}
	return pic, nil
}

// DecodeRLE 解怪物圖的像素：一個位元組一段，**高 nibble + 1 是長度、
// 低 nibble 是顏色**，列優先鋪滿 w×h。
//
// 驗收條件是 433 個影格的像素數逐張等於 `w × h`：279 張剛好用完，其餘
// 最後一段稍微超出（超出的部分丟掉）。這條規則一度被自己的驗收方式判死 ——
// 影格與下一個偏移之間還有幾個位元組的填充，把整段吃完再比對，
// 每一張都會多出一點。**驗收條件本身也要驗**。
func DecodeRLE(src []byte, w, h int) []byte {
	need := w * h
	if need <= 0 {
		return nil
	}
	out := make([]byte, 0, need)
	for _, b := range src {
		n := int(b>>4) + 1
		c := b & 0x0F
		if len(out)+n > need {
			n = need - len(out)
		}
		for i := 0; i < n; i++ {
			out = append(out, c)
		}
		if len(out) >= need {
			break
		}
	}
	// 資料不足就用透明色補滿，讓呼叫端永遠拿到完整的 w×h。
	for len(out) < need {
		out = append(out, TransparentIndex)
	}
	return out
}

// parseAnims 解動畫表從第一個 0xFF 起的部分。
//
//	FF （影格, 停留）… FF （影格, 停留）… FF FF
//
// `FF` 開一段、`FF FF` 收尾，段內是成對的位元組。停留值集中在
// 7/10/15/20/30，是影格數。
//
// 自洽條件：每一段用到的影格編號都要落在該槽宣告的影格數內。
// 59 個槽共 181 段，只有槽 9 的第三段有一個編號 6（它宣告 6 個影格），
// 而且這一部分沒有任何一步設 bit 7 —— bit 7 只出現在前面那段未解的表頭。
// 槽 0 的五段用到 {0,1,3}、{0,2,4}、{0,5,7}、{0,6,8}、{0,9,10,11}，
// 而它宣告 12 個影格。
func parseAnims(b []byte) [][]AnimStep {
	var out [][]AnimStep
	var cur []AnimStep
	for i := 0; i < len(b); {
		if b[i] == 0xFF {
			if i+1 < len(b) && b[i+1] == 0xFF {
				break
			}
			if cur != nil {
				out = append(out, cur)
			}
			cur = []AnimStep{}
			i++
			continue
		}
		if cur == nil || i+1 >= len(b) {
			break
		}
		cur = append(cur, AnimStep{
			Frame: int(b[i] & 0x7F),
			Flag:  b[i]&0x80 != 0,
			Hold:  int(b[i+1]),
		})
		i += 2
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}
