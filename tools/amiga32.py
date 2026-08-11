#!/usr/bin/env python3
"""解 Amiga 版 MM2 的 `.32` 圖形檔容器（影像目錄與調色盤）。

    tools/amiga32.py workplace/amiga/town.32

**先用 `tools/adf.py` 抽檔。** 舊的抽取程式沒有處理 OFS 資料區塊那 24 bytes
的標頭，抽出來的檔案**長度正確、內容從第 1 個位元組就錯**，而且錯得很像
「壓縮過的資料」—— 熵合理、看得到疑似結構。判準只有一個：`mm2` 抽對了
開頭會是 `00 00 03 F3`（Amiga hunk 檔的 magic）。

## 容器

    uint16  count      張數
    uint16  ?          場景檔是 3、sky 是 0
    count × {           每筆 6 bytes，big-endian
        uint16 width
        uint16 height
        uint16 flag     正牆與補牆是 0、側牆是 3
    }
    32    × uint16     調色盤，Amiga 的 12-bit RGB（0x0RGB）
    每張影像：nibble RLE 的位元平面資料（見下）

副檔名的 `32` 是**色數**不是張數 —— 調色盤固定 32 筆。

尺寸與 DOS 的同名檔逐張對得起來（同一批美術），只是 Amiga 的高度少 1–3。
`town.32` 的 32 筆是 16 張 × 兩種變體，與 DOS 的 `TOWN.16` 一模一樣；
`sky.32` 是兩張 208×60，與 DOS 的 `SKY.16` 一致。
"""
import struct
import sys


def parse(d: bytes):
    """目錄就在檔頭，不必掃描。"""
    if len(d) < 4:
        return None
    count, second = struct.unpack_from(">HH", d, 0)
    if not 0 < count <= 200 or 4 + 6 * count + 64 > len(d):
        return None
    ent = [struct.unpack_from(">HHH", d, 4 + 6 * i) for i in range(count)]
    if not all(8 <= w <= 400 and 4 <= h <= 300 for w, h, _ in ent):
        return None
    pal_at = 4 + 6 * count
    return {"dir": 4, "count": count, "field2": second, "entries": ent,
            "pal": pal_at, "colors": palette(d, pal_at), "data": pal_at + 64}


def unrle(d: bytes, pos: int, nwords: int):
    """`sub_33EF2` 的逐行重寫。回傳 (位元平面資料, 讀到哪)。"""
    out = bytearray()
    acc = n = 0
    while len(out) // 2 < nwords and pos < len(d):
        b = d[pos]
        pos += 1
        hi = b & 0xF0
        vals = [b >> 4, b & 0xF] if hi and hi != 0xF0 else [b >> 4] * ((b & 0xF) + 1)
        for v in vals:
            acc = ((acc << 4) | v) & 0xFFFF
            n += 1
            if n == 4:
                out += acc.to_bytes(2, "big")
                acc = n = 0
            if len(out) // 2 >= nwords:
                break
    return bytes(out), pos


def images(d: bytes):
    """把整個 `.32` 解成 [(w, h, flag, 位元平面資料)]。"""
    r = parse(d)
    if r is None:
        return None
    pos = r["data"]
    out = []
    for w, h, fl in r["entries"]:
        bpw = (w + 15) // 16
        px, pos = unrle(d, pos, h * bpw * 5)
        out.append((w, h, fl, px))
    return r, out


def to_png(w: int, h: int, px: bytes, pal, path: str, scale: int = 2):
    from PIL import Image

    bpr = (w + 15) // 16 * 2
    im = Image.new("RGB", (w, h), (255, 0, 255))
    for y in range(h):
        for x in range(w):
            v = 0
            for p in range(5):
                k = (p * h + y) * bpr + x // 8
                if k < len(px) and px[k] >> (7 - x % 8) & 1:
                    v |= 1 << p
            im.putpixel((x, y), pal[v])
    im.resize((w * scale, h * scale), Image.NEAREST).save(path)


def anm(d: bytes):
    """解 `.anm`（怪物動畫）的基準影格。回傳 (w, h, 位元平面資料)。

    容器與 `.32` 不同（`sub_19A30` 開檔後讀 48 bytes 標頭、1 byte 計數、
    再讀 `計數−1` bytes 的動畫表），但**像素用同一套 nibble RLE**。

        d[2], d[3]        影格寬高（72 個檔全部是 84×86）
        0x31 + d[0x30]−1  像素起點

    **調色盤不在檔案裡**，也不在 `.anm` 的檔頭或尾端（32 個 word 的
    12-bit 值域檢驗整個檔案都不通過）。場景檔的 32 格只有前 16 格有色，
    後 16 格留給怪物在執行時另外設 —— 設的來源還沒定位。

    總覽圖用 `book.32` 那份（31 格有色），是十一個候選裡畫出來最自然的
    一個（紅披風、藍衣、灰甲、橘靴）。**這是假設不是已證實**：
    `throw.32` 的 31 格是一條紅色漸層，整批怪物會變成同一個紅，
    先前就是拿它畫的。
    """
    w, h = d[2], d[3]
    if not (8 <= w <= 320 and 8 <= h <= 200):
        return None
    start = 0x31 + d[0x30] - 1
    px, _ = unrle(d, start, h * ((w + 15) // 16) * 5)
    return w, h, px


def palette(d: bytes, off: int, n: int = 32):
    """讀 Amiga 的 12-bit RGB 調色盤，回傳 [(r, g, b), …]（各 0–255）。

    值域檢查（高 nibble 必須為 0）不通過就回 None —— 少了它，任何一段資料
    都能被讀成「調色盤」，而畫出來的顏色錯得很像對的。
    """
    if off + 2 * n > len(d):
        return None
    words = struct.unpack_from(">%dH" % n, d, off)
    if any(w > 0x0FFF for w in words):
        return None
    return [((w >> 8 & 15) * 17, (w >> 4 & 15) * 17, (w & 15) * 17) for w in words]


def main() -> None:
    if len(sys.argv) < 2:
        raise SystemExit(__doc__)
    if sys.argv[1] == "--anm":
        import os

        outdir = sys.argv[2]
        os.makedirs(outdir, exist_ok=True)
        palfile = os.environ.get("ANM_PAL", "workplace/amiga/book.32")
        pal = parse(open(palfile, "rb").read())["colors"]
        n = 0
        for path in sys.argv[3:]:
            got = anm(open(path, "rb").read())
            if got is None:
                print(f"{path}: 解不開")
                continue
            w, h, px = got
            base = os.path.splitext(os.path.basename(path))[0]
            to_png(w, h, px, pal, os.path.join(outdir, f"{base}_{w}x{h}.png"))
            n += 1
        print(f"{n} 個 .anm 的基準影格")
        return

    if sys.argv[1] == "--dump":
        import os

        outdir = sys.argv[2]
        os.makedirs(outdir, exist_ok=True)
        for path in sys.argv[3:]:
            d = open(path, "rb").read()
            got = images(d)
            if got is None:
                print(f"{path}: 解不開")
                continue
            r, ims = got
            base = os.path.splitext(os.path.basename(path))[0]
            for i, (w, h, fl, px) in enumerate(ims):
                to_png(w, h, px, r["colors"],
                       os.path.join(outdir, f"{base}_{i:02d}_{w}x{h}.png"))
            print(f"{path}: {len(ims)} 張")
        return
    for path in sys.argv[1:]:
        d = open(path, "rb").read()
        r = parse(d)
        if r is None:
            print(f"{path}: {len(d)} bytes —— 找不到影像目錄")
            continue
        px = sum(w * h for w, h, _ in r["entries"])
        data = len(d) - 4 - 6 * r["count"] - (64 if r["colors"] else 0)
        print(f"{path}: {len(d)} bytes　目錄 @0x{r['dir']:X}　"
              f"{r['count']} 張　第二欄 {r['field2']}")
        print(f"   像素段 {data} bytes / {px} px = {data * 8 / px:.2f} bit/px"
              f"（32 色未壓縮要 5.00，所以壓過）")
        if r["colors"]:
            print(f"   調色盤 @0x{r['pal']:X}  " +
                  " ".join("%02x%02x%02x" % c for c in r["colors"][:8]) + " …")
        else:
            print("   目錄後面不是調色盤（高 nibble 值域檢查沒過）")
        for i, (w, h, f) in enumerate(r["entries"]):
            end = "\n" if i % 4 == 3 else "  "
            print(f"   {i:2d} {w:3d}×{h:<3d} f{f}", end=end)
        if r["count"] % 4:
            print()


if __name__ == "__main__":
    main()
