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
//
// 影格編號**大於等於影格數時原版畫影格 0**（root `0x1578E` 的
// `cmp al, ds:9F76h` 後接 `xor al, al`），不是壞資料。
type AnimStep struct {
	Frame int
	Hold  int
}

// ScriptStep 是播放腳本的一項：要播哪一段、之後停留多久。
//
// Random 是原始位元組的 bit 7：設起來時原版改成 `rand(1, Seq)` 隨機挑一段
// （root `0x15735`–`0x1573D`，呼叫的正是亂數產生器 `sub_11C88`）；
// 沒設就固定播第 Seq 段。
type ScriptStep struct {
	Seq    int
	Random bool
	Hold   int
}

// MonsterPic 是一個槽的全部內容。
//
// Frames[0] 是 84×86 的基準圖，其餘是疊在上面的動畫零件
// （各自帶 X/Y 偏移，尺寸小得多）。
type MonsterPic struct {
	Slot   int
	Frames []Frame

	// Script 是動畫表的**第一段**：播放腳本，每一項挑一段來播。
	// Seq 是 1 起算的段編號，對到 Anims[Seq-1]。
	Script []ScriptStep

	// Anims 是實際的（影格, 停留）序列，也就是腳本之後的每一段。
	Anims [][]AnimStep
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
	if slot == patchedSlot {
		tbl = patchSlot9(tbl)
	}
	pic.Script, pic.Anims = parseAnims(tbl)
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

// patchedSlot 與 slot9Patch 是原版對自家壞資料的執行時修補。
//
// `MONSTERS.16` 索引 9 的動畫表第一段是 `2F 0A 10 3B 06 04 83 00`，
// 套 (影格, 停留) 讀會得到 47 號影格，而那個槽只有 6 個影格；第三段還有
// 一個越界的 6。root `0x12711` 對索引 9 特判，從 `ds:4B92` 搬 36 bytes
// 蓋上去 —— 中間 22 bytes 與檔案完全相同，差別只有壞掉的第一段換成
// `(1,1),(2,1),(3,1)`，以及第三段的 `06` 換成 `02`。
//
// 見 docs/formats/04-graphics.md §2.3。**這不是我們的修正，是原廠的。**
const patchedSlot = 9

var slot9Patch = []byte{
	0x01, 0x01, 0x02, 0x01, 0x03, 0x01, 0xFF,
	0x01, 0x05, 0x02, 0x05, 0x01, 0x05, 0x00, 0x05, 0xFF,
	0x03, 0x05, 0x04, 0x05, 0x05, 0x05, 0x00, 0x05, 0x05, 0x05, 0x00, 0x05, 0xFF,
	0x01, 0x05, 0x02, 0x05, 0x00, 0x05, 0xFF,
}

// 整張換掉，不是疊上去：修補版本身就是一張完整的表（四段，最後一段以
// `FF` 結束），而檔案版比它長 3 bytes。只蓋前 36 bytes 會留下
// `05 FF` 這種殘尾，被讀成第五段的一步。
func patchSlot9(tbl []byte) []byte {
	if len(tbl) < len(slot9Patch) {
		return tbl
	}
	return slot9Patch
}

// parseAnims 解動畫表，回傳（播放腳本, 各動畫序列）。
//
//	播放腳本 FF 序列 1 FF 序列 2 … FF
//
// **`FF` 是每一段的結束標記，不是分隔符**，表以一個空段（連續兩個 `FF`）
// 收尾。照這個讀法切，59 個槽共 240 段，每一段的長度都是偶數，零例外。
//
// **第一段不是動畫，是播放腳本。** 原版由 root `0x15715` 起的迴圈走它，
// 每一對是（段編號, 停留）：
//
//	al = 腳本的段編號位元組
//	if al & 0x80: al = rand(1, al & 0x7F)   ; sub_11C88，隨機挑一段
//	sub_15772(al)                            ; 從表頭往前跳過 al 個 FF，
//	                                         ; 之後逐對讀（影格, 停留）
//
// 所以 bit 7 是「隨機挑一段」，低 7 位是段編號（隨機時當上限）。
// 資料側全部對得上：59 個槽共 136 個腳本項（31 項帶 bit 7），
// **段編號一律落在 1..段數-1，零例外**。
//
// 先前把第一段當成動畫序列讀，會得到 47、131、134 這種「越界影格」——
// 那些是段編號，不是影格編號。
func parseAnims(b []byte) ([]ScriptStep, [][]AnimStep) {
	var segs [][]byte
	cur := []byte{}
	for i := 0; i+1 < len(b); {
		if b[i] == 0xFF {
			if len(cur) == 0 {
				break // 空段 = 表結束
			}
			segs = append(segs, cur)
			cur = []byte{}
			i++
			continue
		}
		cur = append(cur, b[i], b[i+1])
		i += 2
	}
	if len(cur) > 0 {
		segs = append(segs, cur)
	}
	if len(segs) == 0 {
		return nil, nil
	}

	var script []ScriptStep
	for i := 0; i+1 < len(segs[0]); i += 2 {
		script = append(script, ScriptStep{
			Seq:    int(segs[0][i] & 0x7F),
			Random: segs[0][i]&0x80 != 0,
			Hold:   int(segs[0][i+1]),
		})
	}
	anims := make([][]AnimStep, 0, len(segs)-1)
	for _, seg := range segs[1:] {
		steps := make([]AnimStep, 0, len(seg)/2)
		for i := 0; i+1 < len(seg); i += 2 {
			steps = append(steps, AnimStep{Frame: int(seg[i]), Hold: int(seg[i+1])})
		}
		anims = append(anims, steps)
	}
	return script, anims
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}
