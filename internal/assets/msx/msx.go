// Package msx 讀 MSX2 版 MM2 的磁片（Starcraft 1989 日版）。
//
// 磁片**沒有可用的檔案系統**：BPB 那組參數合法（1440 磁區 × 512
// ＝ 737,280），但 FAT 與根目錄整片是零，遊戲繞過檔案系統直接讀磁區。
// 索引是兩張各 192 筆的表，在磁區 1–3 與磁區 4–6：
//
//	uint16  id
//	uint16  起始磁區
//	uint16  長度（bytes）
//	uint16  0
//
// 圖形檔在第二張表裡（`0x20xx`／`0x21xx`／`0x30xx`），每個檔的前 4 bytes
// 就是 VDP 要的 NX／NY，後面接 RLE 的 4bpp 像素。
//
// 逆向過程與證據見 `docs/research/02-other-platforms.md`。
package msx

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

const (
	sectorSize = 512
	// tableEntries 是一張表的筆數。來自查表迴圈的計數 `ld bc,0C0h`
	// —— 三個磁區除以 8 bytes。
	tableEntries = 192
	// ResidentID 是常駐引擎，載入位址 0x6800。調色盤在它裡面。
	ResidentID = 0xFFF0
	// palAddr 是調色盤在常駐引擎裡的執行位址（`ld hl,0D2F9h : otir`）。
	palAddr = 0xD2F9
	// residentBase 是常駐引擎的載入位址。
	residentBase = 0x6800
)

// Disk 是一片磁片。
type Disk struct {
	raw   []byte
	index map[uint16]entry
	order []uint16
}

type entry struct{ sector, length int }

// Open 讀一片 `.dsk`。
func Open(raw []byte) (*Disk, error) {
	if len(raw) < 8*sectorSize {
		return nil, fmt.Errorf("msx: 磁片太小（%d bytes）", len(raw))
	}
	d := &Disk{raw: raw, index: map[uint16]entry{}}
	// 兩張表。只讀第一張的話圖形檔一個都看不到，而第一張本身
	// 完全解得出一批合法檔案 —— 不會有任何症狀。
	for _, base := range []int{1 * sectorSize, 4 * sectorSize} {
		for i := 0; i < tableEntries; i++ {
			off := base + 8*i
			id := binary.LittleEndian.Uint16(raw[off:])
			sec := int(binary.LittleEndian.Uint16(raw[off+2:]))
			ln := int(binary.LittleEndian.Uint16(raw[off+4:]))
			if ln == 0 || sec*sectorSize+ln > len(raw) {
				continue
			}
			if _, dup := d.index[id]; !dup {
				d.order = append(d.order, id)
			}
			d.index[id] = entry{sec, ln}
		}
	}
	if len(d.index) == 0 {
		return nil, fmt.Errorf("msx: 兩張表都是空的，這不是 MM2 的磁片")
	}
	return d, nil
}

// IDs 回傳這片磁片上的檔案 id，照表的順序。
func (d *Disk) IDs() []uint16 { return d.order }

// File 取一個檔案，沒有就回 nil。
func (d *Disk) File(id uint16) []byte {
	e, ok := d.index[id]
	if !ok {
		return nil
	}
	return d.raw[e.sector*sectorSize : e.sector*sectorSize+e.length]
}

// Palette 從常駐引擎取 16 色調色盤。
//
// MSX2 的調色盤是可程式化的，所以顏色不在圖形檔裡也不在硬體裡，
// 而在程式送給 VDP 的那 32 bytes（`0RRR0BBB` / `00000GGG`，每色 3 bit）。
func (d *Disk) Palette() (color.Palette, error) {
	res := d.File(ResidentID)
	off := palAddr - residentBase
	if len(res) < off+32 {
		return nil, fmt.Errorf("msx: 這片沒有常駐引擎（id %04X），取不到調色盤", ResidentID)
	}
	pal := make(color.Palette, 16)
	for i := range pal {
		hi, lo := res[off+2*i], res[off+2*i+1]
		// 每個分量 3 bit，乘 36 展開到 0–252。
		pal[i] = color.RGBA{(hi >> 4 & 7) * 36, (lo & 7) * 36, (hi & 7) * 36, 0xFF}
	}
	return pal, nil
}

// Image 解一個圖形檔。
//
//	uint16  NX     寬（像素）
//	uint16  NY     高
//	RLE 的 4bpp 像素，解出來剛好 (NX/2) × NY bytes
//
// 檔頭不是「格式」是「命令參數」：載圖那一段把前 4 bytes 原地 `otir`
// 進 VDP 的暫存器 40–43，像素則一個位元組一個位元組餵給 HMMC 命令。
func (d *Disk) Image(id uint16, pal color.Palette) (*image.Paletted, error) {
	blob := d.File(id)
	if len(blob) < 8 {
		return nil, fmt.Errorf("msx: 檔案 %04X 不存在或太短", id)
	}
	nx := int(binary.LittleEndian.Uint16(blob))
	ny := int(binary.LittleEndian.Uint16(blob[2:]))
	if nx < 8 || nx > 512 || ny < 4 || ny > 512 || nx%2 != 0 {
		return nil, fmt.Errorf("msx: 檔案 %04X 的檔頭 %d×%d 不是圖", id, nx, ny)
	}
	need := nx / 2 * ny
	px := unrle(blob[4:], need)
	if len(px) != need {
		return nil, fmt.Errorf("msx: 檔案 %04X 解出 %d bytes，檔頭說要 %d", id, len(px), need)
	}
	im := image.NewPaletted(image.Rect(0, 0, nx, ny), pal)
	for y := 0; y < ny; y++ {
		row := px[y*(nx/2):]
		for x := 0; x < nx; x++ {
			b := row[x/2]
			if x%2 == 0 {
				im.SetColorIndex(x, y, b>>4)
			} else {
				im.SetColorIndex(x, y, b&15)
			}
		}
	}
	return im, nil
}

// unrle 是常駐引擎 `0xC51A` 那支解碼器。
//
//	count = c & 0x7F        （0 當 0x80）
//	c & 0x80  →  接 count 個字面值
//	否則      →  接一個位元組，重複 count 次
//
// 原版寫成「每呼叫一次吐一個位元組」的產生器（計數器藏在 `exx` 的替身
// 暫存器組裡），因為資料是逐位元組餵給 VDP 的，不先解到緩衝區。
func unrle(d []byte, n int) []byte {
	out := make([]byte, 0, n)
	for pos := 0; len(out) < n && pos < len(d); {
		c := d[pos]
		pos++
		cnt := int(c & 0x7F)
		if cnt == 0 {
			cnt = 0x80
		}
		if c&0x80 != 0 {
			if pos+cnt > len(d) {
				cnt = len(d) - pos
			}
			out = append(out, d[pos:pos+cnt]...)
			pos += cnt
		} else {
			if pos >= len(d) {
				break
			}
			for k := 0; k < cnt && len(out) < n; k++ {
				out = append(out, d[pos])
			}
			pos++
		}
	}
	return out
}
