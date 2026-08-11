// Package amiga 解 Amiga 版 MM2 的 `.32` 圖形檔。
//
// 與 DOS 的 `.16` 是**完全不同的容器**（只有命名沿用），但影像的張數與
// 排列一一對應：`town.32` 與 `TOWN.16` 都是 32 張、同樣的深度與側牆順序，
// 所以同一套第一人稱幾何可以直接吃 Amiga 的素材。差別只在
// 高度少 1–3 像素（螢幕比例不同）與火炬的張數。
//
// 格式（big-endian）：
//
//	uint16  count
//	uint16  ?              場景檔是 3、單張的是 0
//	count × { uint16 width, uint16 height, uint16 flag }
//	32    × uint16         調色盤，Amiga 的 12-bit RGB（0x0RGB）
//	每張影像：nibble RLE 的 5 個位元平面
//
// 逆向過程與證據見 `docs/research/02-other-platforms.md`。
package amiga

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// Planes 是位元平面數。程式裡寫死 5（32 色），不是從檔案讀的。
const Planes = 5

// TransparentIndex 是牆面的透空色。
//
// **與 DOS 不同**：DOS 的 `.16` 用色號 8，Amiga 用 0。判準是側牆
// （旗標 3）四角那兩塊楔形 —— 取樣 32 個角落像素有 27 個是 0。
const TransparentIndex = 0

// Set 是一個 `.32` 檔解出來的東西。
type Set struct {
	Palette color.Palette
	Images  []*image.Paletted
	// Flags 是目錄裡那一欄：正牆與補牆是 0、側牆是 3。
	// 側牆需要透空，正牆不需要。
	Flags []int
}

// Parse 解一整個 `.32`。
func Parse(d []byte) (*Set, error) {
	if len(d) < 4 {
		return nil, fmt.Errorf("amiga: 檔案太短（%d bytes）", len(d))
	}
	count := int(binary.BigEndian.Uint16(d))
	dirEnd := 4 + 6*count
	if count == 0 || count > 200 || dirEnd+64 > len(d) {
		return nil, fmt.Errorf("amiga: 張數 %d 不合理", count)
	}

	pal := make(color.Palette, 32)
	for i := range pal {
		v := binary.BigEndian.Uint16(d[dirEnd+2*i:])
		if v > 0x0FFF {
			return nil, fmt.Errorf("amiga: 第 %d 格不是 12-bit RGB（%04x）", i, v)
		}
		// 每個分量 4 bit，乘 17 展開到 0–255。
		pal[i] = color.RGBA{uint8(v >> 8 & 15 * 17), uint8(v >> 4 & 15 * 17), uint8(v & 15 * 17), 0xFF}
	}

	set := &Set{Palette: pal}
	pos := dirEnd + 64
	for i := 0; i < count; i++ {
		w := int(binary.BigEndian.Uint16(d[4+6*i:]))
		h := int(binary.BigEndian.Uint16(d[4+6*i+2:]))
		flag := int(binary.BigEndian.Uint16(d[4+6*i+4:]))
		bpr := (w + 15) / 16 * 2
		px, next, err := unrle(d, pos, h*bpr*Planes)
		if err != nil {
			return nil, fmt.Errorf("amiga: 第 %d 張: %w", i, err)
		}
		set.Images = append(set.Images, planar(px, w, h, bpr, pal))
		set.Flags = append(set.Flags, flag)
		pos = next
	}
	return set, nil
}

// unrle 是 `mm2` 的 `sub_33EF2`。
//
//	高 nibble 是 0 或 F  →  重複那個 nibble（低 nibble + 1）次
//	其他                 →  這個位元組就是兩個字面 nibble
//
// **只有 0 與 15 有 run**，因為位元平面裡連續的 0 位元與 1 位元最多。
func unrle(d []byte, pos, n int) ([]byte, int, error) {
	out := make([]byte, 0, n)
	var acc, bits int
	emit := func(v byte) {
		acc = acc<<4 | int(v)
		bits++
		if bits == 4 {
			out = append(out, byte(acc>>8), byte(acc))
			acc, bits = 0, 0
		}
	}
	for len(out) < n {
		if pos >= len(d) {
			return nil, pos, fmt.Errorf("資料在解完之前用光（%d/%d）", len(out), n)
		}
		b := d[pos]
		pos++
		if hi := b & 0xF0; hi != 0 && hi != 0xF0 {
			emit(b >> 4)
			if len(out) < n {
				emit(b & 0xF)
			}
			continue
		}
		v, cnt := b>>4, int(b&0xF)+1
		for k := 0; k < cnt && len(out) < n; k++ {
			emit(v)
		}
	}
	return out, pos, nil
}

// planar 把 5 個位元平面攤成索引色影像。
//
// 平面是**逐平面連續**（plane 0 的整張、再 plane 1…），不是逐列交錯 ——
// 兩種擺法在單列上看不出差別，整張畫出來才會露餡。
func planar(px []byte, w, h, bpr int, pal color.Palette) *image.Paletted {
	im := image.NewPaletted(image.Rect(0, 0, w, h), pal)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var v byte
			for p := 0; p < Planes; p++ {
				k := (p*h+y)*bpr + x/8
				if k < len(px) && px[k]>>(7-x%8)&1 != 0 {
					v |= 1 << p
				}
			}
			im.SetColorIndex(x, y, v)
		}
	}
	return im
}
