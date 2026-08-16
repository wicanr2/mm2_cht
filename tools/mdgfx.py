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


# 3 位元分量 → 8 位元的實際輸出階（拿 BlastEm 原生截圖逐格反查出來的）。
# **不是線性的** —— 早期版本用 ×17／×36 都畫得出「看起來對」的圖，
# 但逐像素與實機比就全錯。
MD_RAMP = (0, 49, 87, 119, 146, 174, 206, 255)


def palette(d: bytes, off: int):
    """9-bit BGR（`0000 BBB0 GGG0 RRR0`）→ RGB 0–255。"""
    return [tuple(MD_RAMP[((struct.unpack_from(">H", d, off + 2 * i)[0] >> s) & 0xE) >> 1]
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


# 一張怪物圖是 11×11 個 tile（88×88 px），由九個硬體 sprite 拼出來：
# 寬 4/4/3、高 4/4/3（＝ `min(4, 剩餘)` 的通用切法，sprite 最大就是 4×4）。
# sprite 的順序是「一整行由上到下、再換下一行」，而 **sprite 內部的 tile
# 是直向排的** —— Mega Drive 硬體就是這樣存 sprite 的圖案。
PIC_W = 11
PIC_CELLS = PIC_W * PIC_W
NT_BYTES = PIC_CELLS * 2          # 一個影格的 nametable ＝ 242 bytes


def sprite_cells():
    """回傳 nametable 第 k 格畫在 (x, y) 的對照表。

    這張表是從實機的 sprite 屬性表影子（work RAM `0xFFD2A8`）讀出來的：
    九筆 sprite，x 是 204/236/268、y 是 174/206/238，尺寸 4×4/4×4/4×3、
    3×4/3×4/3×3，tile 編號連續。照它排，nametable 才拼得出圖 ——
    照 row-major 排會得到看起來像雜訊、但每一格都合法的東西。
    """
    out, t, x0 = {}, 0, 0
    for w in (4, 4, 3):
        y0 = 0
        for h in (4, 4, 3):
            for k in range(w * h):
                out[t] = (x0 + k // h, y0 + k % h)
                t += 1
            y0 += h
        x0 += w
    return out


def nametables(d: bytes) -> list:
    """掃出全部 nametable 資源（解壓後長度是 242 的倍數）。

    **`raw % 242 == 0` 就是判準**：242 ＝ 11×11×2，一格兩個位元組的
    標準 Mega Drive nametable 項。ROM 裡剛好 75 筆，與 DOS `MONSTERS.16`
    的 75 個槽數量相同。
    """
    out = []
    for p in range(0x20000, len(d) - 16, 2):
        comp, raw = struct.unpack_from(">II", d, p)
        if raw % NT_BYTES or not (16 < comp < 0x20000 and raw < 0x20000):
            continue
        if p + 8 + comp > len(d):
            continue
        try:
            got, used = lzss(d, p + 8, raw)
        except Exception:
            continue
        if len(got) == raw and abs(used - comp) <= 2:
            out.append({"at": p, "frames": raw // NT_BYTES, "data": got})
    return out


def pair_pool(bs, nt):
    """替一張 nametable 找它的 tile 池。

    ROM 裡是 (tile 池, nametable, 調色盤) 依序擺，所以**先往前找最近、
    而且 tile 數蓋得住這張圖用到的最大索引**的那個池；前面找不到才往後找
    （少數幾張共用池，池排在後面）。只取「前面最近」會漏掉那幾張，
    而漏掉的長相是一張全黑的圖 —— 與「這張本來就是空的」分不開。
    """
    need = max(v & 0x7FF for v in struct.unpack_from(">%dH" % PIC_CELLS, nt["data"], 0))
    before = [b for b in bs if b["hdr"] < nt["at"] and b["tiles"] > need]
    if before:
        return before[-1]
    after = [b for b in bs if b["hdr"] >= nt["at"] and b["tiles"] > need]
    return after[0] if after else None


def picture(d: bytes, b, nt, frame: int, pal):
    """把一個影格攤成 (88, 88) 的調色盤索引陣列。"""
    import numpy as np

    px = decode(d, b)
    n = b["tiles"]
    a = np.frombuffer(px[:n * 32], dtype=np.uint8).reshape(n, 8, 4)
    tiles = np.empty((n, 8, 8), np.uint8)
    tiles[:, :, 0::2] = a >> 4
    tiles[:, :, 1::2] = a & 15
    idx = struct.unpack_from(">%dH" % PIC_CELLS, nt["data"], frame * NT_BYTES)
    out = np.zeros((PIC_W * 8, PIC_W * 8), np.uint8)
    lay = sprite_cells()
    for c, v in enumerate(idx):
        t = v & 0x7FF
        if t >= n:
            continue
        g = tiles[t]
        if v & 0x800:
            g = g[:, ::-1]
        if v & 0x1000:
            g = g[::-1, :]
        cx, cy = lay[c]
        out[cy * 8:cy * 8 + 8, cx * 8:cx * 8 + 8] = g
    return out


def export_pics(d: bytes, bs, outdir: str, data_dir: str) -> None:
    """把每張圖的每個影格烘成 PNG，另附 `set.json`（見 `tools/monpack.py`）。

    **烘出來就不必再在執行時走 nametable 了** —— sprite 版面已經定案，
    重建一次就好，之後 remake 直接吃 PNG。
    """
    import os

    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    import monpack

    nts = nametables(d)
    pics, masks, last = [], {}, [(0, 0, 0)] * 16
    for i, nt in enumerate(nts):
        b = pair_pool(bs, nt)
        if b is None:
            continue
        pal = palette(d, b["pal"]) if b["pal"] is not None else last
        last = pal
        frames = [picture(d, b, nt, f, pal) for f in range(nt["frames"])]
        masks[i] = frames[0] != 0
        pics.append((i, pal, frames))
    slot_of = monpack.assign(masks, monpack.dos_masks(data_dir))
    monpack.write(outdir, "Mega Drive (1991)", PIC_W * 8, PIC_W * 8, pics, slot_of)


MONSTER_SLOTS = 75


def slot_map(d: bytes, bs, data_dir: str):
    """回傳 {nametable 索引: (DOS 槽號, 分數)}。對不到的不在字典裡。

    ROM 裡 nametable 的順序與 DOS `MONSTERS.16` 的槽號**是一個排列**，
    所以要用剪影比對再做貪婪一對一指派。見 `export_pics` 的說明。
    """
    import os

    import numpy as np

    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    import mm216

    nts = nametables(d)
    masks = []
    for nt in nts:
        b = pair_pool(bs, nt)
        masks.append(None if b is None else (picture(d, b, nt, 0, None) != 0))
    blob = open(os.path.join(data_dir, "MONSTERS.16"), "rb").read()
    slots = mm216.monster_index(blob)
    scores = []
    for s_i in range(MONSTER_SLOTS):
        if not slots[s_i]:
            continue
        fr, _ = mm216.parse_monster(blob, s_i)
        _, _, w, h, px = fr[0]
        dm = np.frombuffer(px, dtype=np.uint8).reshape(h, w) != mm216.MON_TRANSPARENT
        dd = np.zeros((PIC_W * 8, PIC_W * 8), bool)
        dd[:h, :w] = dm
        for i, mk in enumerate(masks):
            if mk is not None:
                scores.append((float((dd == mk).mean()), s_i, i))
    scores.sort(reverse=True)
    out, used_s, used_i = {}, set(), set()
    for sc, s_i, i in scores:
        if s_i in used_s or i in used_i:
            continue
        out[i] = (s_i, round(sc, 4))
        used_s.add(s_i)
        used_i.add(i)
    return out


def make_pics(d: bytes, bs, out: str, data_dir: str = "", scale: int = 1) -> None:
    """把每張圖的第一個影格畫出來排成總覽圖。

    配對規則：ROM 裡是 (tile 池, nametable, 調色盤) 依序擺，所以取
    **前面最近、而且 tile 數蓋得住這張 nametable 用到的最大索引**的那個池。
    只取「最近」會撞到共用池的那幾張。
    """
    from PIL import Image

    lay = sprite_cells()
    pools = sorted(bs, key=lambda b: b["hdr"])
    # 照 DOS 槽號排，與 dos-monsters.png／amiga-monsters.png 對得起來；
    # 對不到槽的（片頭地球、書、城堡那些 MD 才有的）排到最後。
    order = list(enumerate(nametables(d)))
    smap = {}
    if data_dir:
        try:
            smap = slot_map(d, bs, data_dir)
        except Exception as exc:      # 沒有 MONSTERS.16 就照 ROM 順序
            print(f"（沒對槽號，照 ROM 順序排：{exc}）")
    order.sort(key=lambda t: smap.get(t[0], (MONSTER_SLOTS + t[0], 0))[0])
    cells, last = [], [(0, 0, 0)] * 16
    for _, nt in order:
        idx = struct.unpack_from(">%dH" % PIC_CELLS, nt["data"], 0)
        b = pair_pool(pools, nt)
        if b is None:
            continue
        pal = palette(d, b["pal"]) if b["pal"] is not None else last
        last = pal
        px = decode(d, b)
        im = Image.new("RGB", (PIC_W * 8, PIC_W * 8), (0, 0, 0))
        for c, v in enumerate(idx):
            t = v & 0x7FF
            if t >= b["tiles"]:
                continue
            cx, cy = lay[c]
            for y in range(8):
                sy = 7 - y if v & 0x1000 else y
                for x in range(0, 8, 2):
                    val = px[t * 32 + sy * 4 + x // 2]
                    hi, lo = pal[val >> 4], pal[val & 15]
                    if v & 0x800:
                        im.putpixel((cx * 8 + 7 - x, cy * 8 + y), hi)
                        im.putpixel((cx * 8 + 6 - x, cy * 8 + y), lo)
                    else:
                        im.putpixel((cx * 8 + x, cy * 8 + y), hi)
                        im.putpixel((cx * 8 + x + 1, cy * 8 + y), lo)
        cells.append(im)
    cols = 12
    rows = (len(cells) + cols - 1) // cols
    sheet = Image.new("RGB", (cols * 92 + 4, rows * 92 + 4), (18, 18, 24))
    for i, im in enumerate(cells):
        sheet.paste(im, (4 + (i % cols) * 92, 4 + (i // cols) * 92))
    sheet.resize((sheet.width * scale, sheet.height * scale), Image.NEAREST).save(out)
    print(f"{len(cells)} 張圖 → {out}（{sheet.width * scale}×{sheet.height * scale}）")


# 總覽圖的一格：16 tile 寬（＝128 px），高度取最高的那一塊。
SHEET_COLS = 16
SHEET_GRID = 8


def make_sheet(d: bytes, bs, out: str, scale: int = 1) -> None:
    """把全部區塊排成對齊的格線。

    **這是 tile 集合不是圖。** 每一塊是去重過的 tile，畫面由 nametable
    組出來，而怪物那批的 nametable 還沒定位 —— 所以這裡只保證「每一塊
    佔一格、用自己的調色盤、對齊」，不宣稱看得出是什麼。

    重排救不回來這件事是試過的：所有能整除 tile 數的寬度、直排與橫排、
    1×1 到 4×4 的 sprite 分組全掃過一遍，接縫連續度（跨 tile 邊界的
    像素差 ÷ tile 內部的像素差）最好也只到 1.25，離 1 還很遠。
    """
    from PIL import Image

    cw, pad = SHEET_COLS * 8, 4
    order = sorted(bs, key=lambda x: x["id"])
    # 每一列的高度取那一列最高的那一塊 —— 全部對齊到最高的那一塊會
    # 留下大半張空白，而空白不是資訊。
    rowh = [max((b["tiles"] + SHEET_COLS - 1) // SHEET_COLS for b in order[r:r + SHEET_GRID]) * 8
            for r in range(0, len(order), SHEET_GRID)]
    ytop, y = [], pad
    for h in rowh:
        ytop.append(y)
        y += h + pad
    im = Image.new("RGB", (SHEET_GRID * (cw + pad) + pad, y), (18, 18, 24))
    last = [(0, 0, 0)] * 16
    for i, b in enumerate(order):
        ch = rowh[i // SHEET_GRID]
        pal = palette(d, b["pal"]) if b["pal"] is not None else last
        last = pal
        px = decode(d, b)
        cell = Image.new("RGB", (cw, ch), (18, 18, 24))
        for t in range(b["tiles"]):
            base, tx, ty = t * 32, (t % SHEET_COLS) * 8, (t // SHEET_COLS) * 8
            for y in range(8):
                for x in range(0, 8, 2):
                    v = px[base + y * 4 + x // 2]
                    cell.putpixel((tx + x, ty + y), pal[v >> 4])
                    cell.putpixel((tx + x + 1, ty + y), pal[v & 15])
        gx, gy = i % SHEET_GRID, i // SHEET_GRID
        im.paste(cell, (pad + gx * (cw + pad), ytop[gy]))
    im.resize((im.width * scale, im.height * scale), Image.NEAREST).save(out)
    print(f"{len(bs)} 個區塊 → {out}（{im.width * scale}×{im.height * scale}）")


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--dump", help="把每個區塊解壓成 PNG 放進這個目錄")
    ap.add_argument("--pal", help="把所有調色盤畫成色票 PNG")
    ap.add_argument("--sheet", help="把全部區塊排成一張對齊的總覽圖")
    ap.add_argument("--pics", help="把 75 張怪物圖（tile 池 ＋ nametable）畫成總覽圖")
    ap.add_argument("--export", help="把每張圖的每個影格烘成 PNG ＋ set.json 放進這個目錄")
    ap.add_argument("--data", default="workplace/orig/MM2",
                    help="--export 用：DOS 原版目錄，拿 MONSTERS.16 對出槽號")
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
    if a.export:
        export_pics(d, bs, a.export, a.data)
        return

    if a.pics:
        make_pics(d, bs, a.pics, a.data)
        return

    if a.sheet:
        make_sheet(d, bs, a.sheet)
        return

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
