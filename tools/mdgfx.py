#!/usr/bin/env python3
"""解 Mega Drive 版 MM2 的圖形區塊。

    tools/mdgfx.py workplace/genesis/*.md                    列出所有區塊
    tools/mdgfx.py *.md --dump workplace/gfx/md               整批解壓成 PNG
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

	uint16  id           1–75 的稀疏編號（另有一個 158）
	uint32  compSize     壓縮後大小
	uint32  rawSize      解壓後大小 ＝ tiles × 32 + 2
	uint32  flagTiles    高位元組是 flag（0x27/0x47/0x87/0xC7），
	                     中間兩個位元組是 tile 數，最低位元組是 0
	uint16  0xF0FF       magic，62 組全部都有

`flagTiles` 要當成一個長字讀，**不能把 tile 數當成 `+11` 的字** ——
那是奇數位址，68000 讀字會觸發位址錯誤，所以原版不可能那樣讀。
兩種讀法算出來的數字相同，錯的讀法要靠「這台機器做不到」才排除得掉。

**驗收條件是 `(rawSize − 2) / 32 == tiles`** —— 兩個獨立欄位互相印證，
74 組裡 62 組通過。這不是「長度剛好對」那種弱證據：欄位位置與換算關係
都要同時成立才會過。剩下 12 組是掃描起點偏了兩個位元組的那幾組
（調色盤前面剛好也有合法的色值），不是格式不同。

62 組合計 **10,120 個 tile、323,964 bytes**，那是整套 Mega Drive 素材。

`id` 落在 1–75（外加一個 158），而 DOS 的 `MONSTERS.16` 正好是 75 個槽，
所以這批多半是怪物圖，`id` 就是槽號。**待驗**：要等解得開才能逐張比對。

## 壓縮：LZSS，4096 環形緩衝，初值 0x20

解壓常式在 **ROM `0x29954`**，由 `jsr $C(a5)` 呼叫（A5 = `0x312`，
那裡是一張 6 bytes 一格的 `jmp abs.l` 跳躍表）。逐行重寫在 `lzss()`。

    ring = 0x20 × 4096      r = 0xFEE
    旗標位元組，LSB 先，1 = 字面值
    match: lo, hi 兩個位元組
           off = lo | ((hi & 0xF0) << 4)     ← 環形緩衝的**絕對位置**
           len = (hi & 0x0F) + 3

呼叫慣例是 `decompress(src, dest)`，而 **`src` 指的是 `compSize` 那個欄位**
（不是 id）。常式讀 `4(a0)` 當輸出長度、`8(a0)` 起是位元流。

三件先前猜錯而卡了很久的事，寫下來免得重踩：

  - **偏移是環形緩衝的絕對位置，不是「往回幾個位元組」。** 兩者在資料
    中段等價，但在開頭不等價 —— 緩衝預先填了 0x20，所以串流一開始就能
    引用「還沒輸出過」的內容。用回退距離實作會在第一個 match 就越界，
    而那個症狀看起來像「參數猜錯了」。
  - **輸出長度在 `src+4`、位元流在 `src+8`。** 先前把 `+8` 的 16-bit 當成
    rawSize（值一樣，因為 `+4..5` 是零），於是位元流的起點整整差了 6 bytes。
  - `F0FF` 不是 magic，是**位元流本身**的一部分。程式碼裡從來沒有比較過它。

解出來的輸出是 `uint16 tileCount` 接 `tileCount × 32` bytes 的 4bpp tile，
所以 `rawSize = tiles × 32 + 2`。DMA 進 VRAM 時從 `dest+2` 開始搬，
那 2 bytes 的計數不進 VRAM。

62 個區塊全部解得開：長度精確等於 `rawSize`，而且解出來的第一個字
等於區塊頭宣告的 tile 數 —— 兩個獨立來源互相印證。
合計 **10,120 個 tile**。
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


def blocks(d: bytes, lo=0, hi=None):
    """回傳通過結構驗證的區塊。驗收條件見模組說明。

    **不要求前面一定有調色盤**，也不要把掃描範圍框在圖形區：九個區塊
    前面沒有調色盤（沿用上一次設的），另外五個落在原本框的範圍外。
    只用調色盤當入口會漏掉它們，而漏掉的長相與「不存在」一模一樣。

    結構條件三個一起看就夠嚴：`(raw−2)/32 == tiles`（宣告的 tile 數要與
    輸出長度算出來的相同）、`0 < comp < raw×4`、結尾 magic。
    """
    out, i = [], lo
    hi = hi or len(d) - 16
    while i < hi:
        h = i
        comp, raw = struct.unpack_from(">II", d, h + 2)
        tiles = (struct.unpack_from(">I", d, h + 10)[0] >> 8) & 0xFFFF
        magic = struct.unpack_from(">H", d, h + 14)[0]
        if magic == MAGIC and raw > 2 and (raw - 2) % 32 == 0 and (raw - 2) // 32 == tiles \
                and 0 < comp < raw * 4:
            pal = h - 32 if h >= 32 and is_palette(d, h - 32) else None
            out.append({"pal": pal, "hdr": h, "data": h + HDR,
                        "id": struct.unpack_from(">H", d, h)[0],
                        "comp": comp, "raw": raw, "tiles": tiles, "flag": d[h + 10]})
            i = h + HDR + comp
            continue
        i += 2
    return out


def lzss(d: bytes, src: int, out_len: int):
    """ROM 0x29954 的逐行重寫。回傳 (輸出, 吃掉的位元組數)。"""
    ring = bytearray(b"\x20" * 4096)
    r = 0xFEE
    out = bytearray()
    p = src
    flags = 0
    while len(out) < out_len:
        flags >>= 1
        if not flags & 0x100:
            # 原版用 `ori.w #$FF00,d6` 把高位元組填滿當計數器：
            # 低 8 位用完之後 0x100 那一位就會是 0，不必另外數。
            flags = d[p] | 0xFF00
            p += 1
        if flags & 1:
            b = d[p]
            p += 1
            out.append(b)
            ring[r] = b
            r = (r + 1) & 0xFFF
        else:
            lo, hi = d[p], d[p + 1]
            p += 2
            off = lo | ((hi & 0xF0) << 4)
            for k in range((hi & 0x0F) + 3):
                b = ring[(off + k) & 0xFFF]
                out.append(b)
                ring[r] = b
                r = (r + 1) & 0xFFF
                if len(out) >= out_len:
                    break
    return bytes(out), p - src


def decode(d: bytes, b) -> bytes:
    """回傳這個區塊的 tile 像素（已去掉開頭那 2 bytes 的計數）。"""
    src = b["hdr"] + 2
    raw = struct.unpack_from(">I", d, src + 4)[0]
    out, _ = lzss(d, src + 8, raw)
    return out[2:]


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
    ap.add_argument("--dump", help="把每個區塊解壓成 PNG 放進這個目錄")
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

    last_pal = [(0,0,0)]*16
    if a.dump:
        import os

        from PIL import Image

        os.makedirs(a.dump, exist_ok=True)
        for b in bs:
            px = decode(d, b)
            # 沒有自己的調色盤就沿用上一塊的 —— 原版就是這樣，
            # 調色盤是另外設定的狀態，不是每塊圖都自帶。
            pal = palette(d, b["pal"]) if b["pal"] is not None else last_pal
            last_pal = pal
            cols = 16
            rows = (b["tiles"] + cols - 1) // cols
            im = Image.new("RGB", (cols * 8, rows * 8), (255, 0, 255))
            for t in range(b["tiles"]):
                base, tx, ty = t * 32, (t % cols) * 8, (t // cols) * 8
                for y in range(8):
                    for x in range(0, 8, 2):
                        v = px[base + y * 4 + x // 2]
                        im.putpixel((tx + x, ty + y), pal[v >> 4])
                        im.putpixel((tx + x + 1, ty + y), pal[v & 15])
            im.resize((im.width * 3, im.height * 3), Image.NEAREST).save(
                os.path.join(a.dump, f"id{b['id']:04X}_{b['hdr']:06X}_{b['tiles']}t.png"))
        print(f"{len(bs)} 個區塊解壓成 PNG 放進 {a.dump}")
        return

    if a.pal:
        from PIL import Image

        im = Image.new("RGB", (16 * 16, len(bs) * 16))
        for r, b in enumerate(bs):
            for c, col in enumerate(palette(d, b["pal"]) if b["pal"] is not None else last_pal):
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
        print(f"  {'—     ' if b['pal'] is None else format(b['pal'],'06X')}  0x{b['data']:06X} {b['id']:5d} {b['comp']:7d} "
              f"{b['raw']:7d} {b['tiles']:6d}  0x{b['flag']:02X}  {b['comp'] / b['raw']:.2f}")


if __name__ == "__main__":
    sys.exit(main())
