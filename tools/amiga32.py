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


def parse(d: bytes, strict: bool = True):
    """目錄就在檔頭，不必掃描。

    `strict` 是給**掃描**用的尺寸下限（避免把任意資料讀成目錄）。
    `.anm` 的容器起點是算出來的、不是掃出來的，而且它的動畫零件可以小到
    2×1，所以那條路要 `strict=False` —— 它改用「首筆必須是 84×86」與
    「調色盤 32 個字全部 12-bit 合法」把關，兩條都比尺寸下限硬。
    """
    if len(d) < 4:
        return None
    count, second = struct.unpack_from(">HH", d, 0)
    if not 0 < count <= 200 or 4 + 6 * count + 64 > len(d):
        return None
    ent = [struct.unpack_from(">HHH", d, 4 + 6 * i) for i in range(count)]
    if strict and not all(8 <= w <= 400 and 4 <= h <= 300 for w, h, _ in ent):
        return None
    if not all(0 < w <= 400 and 0 < h <= 300 for w, h, _ in ent):
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
    im.resize((im.width * scale, im.height * scale), Image.NEAREST).save(path)


def anm(d: bytes):
    """解 `.anm`（怪物動畫）。回傳 (w, h, 位元平面資料, 調色盤)。

    **`.anm` 的本體就是一個標準的 `.32` 容器，自帶 32 色調色盤。**
    這件事是從 Amiga 執行檔追出來的，不是猜的：`sub_33A18`（動畫載入）
    第一件事就是把檔案交給 `sub_33322` —— 而 `sub_33322` 正是 `.32` 的
    解析器（讀 count、`count×6` 的目錄、64 bytes 調色盤）。

    所以檔案版面是：

        0x00   uint8 w, h 在 +2/+3；+4…+0x2F 是十一組影格矩形
        0x30   uint8 count
        0x31   count−1 bytes 的動畫表      ← `sub_19A30` 讀到這裡為止
        body   uint16 count, uint16 3      ← 從這裡開始是 `.32` 容器
               count × (w, h, flags)
               32 × uint16 調色盤
               各影格的 nibble RLE 位元平面

    `body = 0x31 + d[0x30] − 1`。

    ## 調色盤怎麼套上去

    `A4−0x4F0` 是實際送進硬體的 32 色緩衝（`sub_33ED2` 拿它呼叫
    `LoadRGB4(viewport, 緩衝, 32)`）。`sub_33322` 讀檔時就把這個檔的
    64 bytes 填進去，`sub_33A18` 接著**只把 `0x12`–`0x1F` 從畫面現有的
    色表抄回來**（`move.w #$12,var` 起的迴圈），其餘不動。

    所以一隻怪的顏色是：**自己檔案裡的 `0x00`–`0x11`** ＋ 場景的
    `0x12`–`0x1F`。七十二個檔的 `0x12`–`0x1F` 完全相同（那段本來就會被
    蓋掉），而 `0x03`–`0x11` 每個檔都不一樣 —— 那才是怪物的顏色。

    這同時解掉了先前那條「頂端 14 列雜訊」：像素不是從 `body` 開始，
    是從 `body + 4 + count×6 + 64` 開始，早了六十幾個位元組。
    """
    body = 0x31 + d[0x30] - 1
    r = parse(d[body:], strict=False)
    if r is None or r["colors"] is None:
        return None
    w, h, _ = r["entries"][0]
    if (w, h) != (d[2], d[3]):
        return None
    bpw = (w + 15) // 16
    px, _ = unrle(d, body + r["data"], h * bpw * 5)
    return w, h, px, r["colors"]


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
        scale = int(os.environ.get("ANM_SCALE", "2"))
        n = 0
        for path in sys.argv[3:]:
            got = anm(open(path, "rb").read())
            if got is None:
                print(f"{path}: 解不開")
                continue
            w, h, px, pal = got
            base = os.path.splitext(os.path.basename(path))[0]
            to_png(w, h, px, pal, os.path.join(outdir, f"{base}_{w}x{h}.png"),
                   scale=scale)
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
