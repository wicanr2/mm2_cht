#!/usr/bin/env python3
"""盤點 Mega Drive 版 MM2 的圖形區塊。

    tools/mdgfx.py workplace/genesis/*.md                    列出所有區塊
    tools/mdgfx.py *.md --pal workplace/gfx/md/pal.png       調色盤色票
    tools/mdgfx.py *.md --raw 0x06D800 0x06E400 --out t.png  畫未壓縮的那一段

ROM 沒有檔案系統也沒有圖形指標表（32-bit 遞增位址表全檔只有一處，10 筆，
指向它自己後面）。找圖形區塊的入口是**調色盤**。

## 怎麼認調色盤

Mega Drive 的色是 9-bit BGR 塞在 16-bit 裡：`0000 BBB0 GGG0 RRR0`，
也就是 `值 & 0x0EEE == 值`。隨機資料連續 16 個字全中這個遮罩的機率是
`(1/128)^16`，所以命中就是真的，不必再旁證。再加兩個條件濾掉全零與
低變化的假陽性：相異值 ≥ 9、非零 ≥ 10。

`0x30000`–`0x72000` 之間掃出 **74 組**。

## 區塊頭：調色盤後面 16 bytes

	uint16  id
	uint32  compSize     壓縮後大小
	uint32  rawSize      解壓後大小 ＝ tiles × 32 + 2
	uint8   flag         0x27 / 0x47 / 0x87 / 0xC7
	uint16  tiles        tile 數
	uint8   ?
	uint16  0xF0FF       magic，74 組全部都有

**驗收條件是 `(rawSize − 2) / 32 == tiles`** —— 兩個獨立欄位互相印證，
74 組裡 62 組通過。這不是「長度剛好對」那種弱證據：欄位位置與換算關係
都要同時成立才會過。剩下 12 組是掃描起點偏了兩個位元組的那幾組
（調色盤前面剛好也有合法的色值），不是格式不同。

62 組合計 **10,120 個 tile、323,964 bytes**，那是整套 Mega Drive 素材。

## 像素編碼：還沒解

壓縮率 0.51–0.69。已經排除（每一項的驗收條件都是「解出來的長度等於
rawSize、吃掉的位元組等於 compSize」，兩個精確數字，不是目視）：

  - Kosinski（Mega Drive 最常見的那一套）—— 第一個 match 就偏移越界
  - `MONSTERS.16` 的 nibble RLE —— 吃掉的位元組只有 compSize 的三分之一
  - flag-bit LZSS 家族 96 種組合：旗標單位 byte／word(LE)／word(BE) ×
    位元序 LSB／MSB × 字面值極性 × 四種 match 編碼 × 最短長度 2／3

資料流固定以 `F0 FF` 之後的兩個位元組開始，56 組是 `02 0F`、3 組 `01 0D`、
2 組 `FE FA`、1 組 `FF FB`。

**下一步是反 68000 找解壓常式**，錨點是區塊頭的欄位位移：解壓常式會讀
`+2`（compSize）與 `+6`（rawSize），而 VDP 的三個硬體位址
（`$C00000` ×13、`$C00004` ×61、`$C00008` ×1）給出寫 VRAM 的所有地點。
"""
import argparse
import glob
import struct
import sys

PAL_MASK = 0x0EEE
HDR = 16          # 調色盤之後的區塊頭長度
MAGIC = 0xF0FF


def is_palette(d: bytes, off: int) -> bool:
    if off + 32 > len(d):
        return False
    v = struct.unpack_from(">16H", d, off)
    if any(x & ~PAL_MASK for x in v):
        return False
    return len(set(v)) >= 9 and sum(1 for x in v if x) >= 10


def palette(d: bytes, off: int):
    """9-bit BGR → RGB 0–255。每個分量只有 3 個有效位元，乘 17 展開。"""
    return [tuple(((struct.unpack_from(">H", d, off + 2 * i)[0] >> s) & 0xE) * 17
                  for s in (0, 4, 8))
            for i in range(16)]


def blocks(d: bytes, lo=0x30000, hi=0x72000):
    """回傳通過結構驗證的區塊。驗收條件見模組說明。"""
    out, i = [], lo
    while i < hi:
        if not is_palette(d, i):
            i += 2
            continue
        h = i + 32
        comp, raw = struct.unpack_from(">II", d, h + 2)
        tiles = struct.unpack_from(">H", d, h + 11)[0]
        magic = struct.unpack_from(">H", d, h + 14)[0]
        if raw > 2 and (raw - 2) % 32 == 0 and (raw - 2) // 32 == tiles \
                and 0 < comp < raw * 4 and magic == MAGIC:
            out.append({"pal": i, "hdr": h, "data": h + HDR, "id": struct.unpack_from(">H", d, h)[0],
                        "comp": comp, "raw": raw, "tiles": tiles, "flag": d[h + 10]})
        i += 32
    return out


def draw_tiles(d: bytes, off: int, n: int, pal, cols=32, scale=3):
    from PIL import Image

    rows = (n + cols - 1) // cols
    im = Image.new("RGB", (cols * 8, rows * 8), (255, 0, 255))
    for t in range(n):
        b = off + t * 32
        tx, ty = (t % cols) * 8, (t // cols) * 8
        for y in range(8):
            for x in range(0, 8, 2):
                k = b + y * 4 + x // 2
                if k >= len(d):
                    return im.resize((im.width * scale, im.height * scale), Image.NEAREST)
                v = d[k]
                im.putpixel((tx + x, ty + y), pal[v >> 4])
                im.putpixel((tx + x + 1, ty + y), pal[v & 15])
    return im.resize((im.width * scale, im.height * scale), Image.NEAREST)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--pal", help="把所有調色盤畫成色票 PNG")
    ap.add_argument("--raw", nargs=2, help="畫某一段未壓縮的 tile（起訖，十六進位）")
    ap.add_argument("--palette-at", type=lambda s: int(s, 0), default=0x06D03C,
                    help="--raw 用哪一組調色盤")
    ap.add_argument("--out", default="md.png")
    a = ap.parse_args()

    paths = glob.glob(a.rom)
    if not paths:
        raise SystemExit(f"找不到 {a.rom}")
    d = open(paths[0], "rb").read()
    bs = blocks(d)

    if a.raw:
        lo, hi = int(a.raw[0], 0), int(a.raw[1], 0)
        draw_tiles(d, lo, (hi - lo) // 32, palette(d, a.palette_at), cols=24).save(a.out)
        print(f"畫到 {a.out}（{(hi - lo) // 32} 個 tile）")
        return

    if a.pal:
        from PIL import Image

        im = Image.new("RGB", (16 * 16, len(bs) * 16))
        for r, b in enumerate(bs):
            for c, col in enumerate(palette(d, b["pal"])):
                for y in range(16):
                    for x in range(16):
                        im.putpixel((c * 16 + x, r * 16 + y), col)
        im.save(a.pal)
        print(f"{len(bs)} 組調色盤畫到 {a.pal}")
        return

    print(f"{paths[0]}：{len(d)} bytes，通過結構驗證的圖形區塊 {len(bs)} 個")
    print(f"合計 {sum(b['tiles'] for b in bs)} 個 tile、"
          f"解壓後 {sum(b['raw'] for b in bs)} bytes（壓縮後 {sum(b['comp'] for b in bs)}）")
    print(f"{'調色盤':>10} {'資料':>9} {'id':>5} {'comp':>7} {'raw':>7} {'tiles':>6} {'flag':>5} 壓縮率")
    for b in bs:
        print(f"  0x{b['pal']:06X}  0x{b['data']:06X} {b['id']:5d} {b['comp']:7d} "
              f"{b['raw']:7d} {b['tiles']:6d}  0x{b['flag']:02X}  {b['comp'] / b['raw']:.2f}")


if __name__ == "__main__":
    sys.exit(main())
