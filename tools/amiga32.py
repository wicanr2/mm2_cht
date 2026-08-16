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


def _unpack(px: bytes, w: int, h: int):
    """5 個位元平面 → (h, w) 的索引陣列。"""
    import numpy as np

    bpr = (w + 15) // 16 * 2
    a = np.frombuffer(bytes(px).ljust(h * bpr * 5, b"\0"), dtype=np.uint8)
    out = np.zeros((h, w), np.uint8)
    for pl in range(5):
        b = a[pl * h * bpr:(pl + 1) * h * bpr].reshape(h, bpr)
        out |= np.unpackbits(b, axis=1)[:, :w] << pl
    return out


def anm_parts(d: bytes):
    """解一個 `.anm` 的全部零件。回傳 dict，解不開回 None。

    容器的第 0 張是基準圖（84×86），**其餘每一張都是一個動畫零件**，
    貼在基準圖上檔頭給的位置：`+4` 起每四個位元組是 `(x, y, w, h)`，
    第 n 組對應容器的第 n+1 張，`w`／`h` 與容器目錄逐一相符。
    七十二個檔全部解得完、剛好把檔案讀到底、沒有剩餘位元組。

    ## 動畫表

    `0x31` 起的 `count−1` bytes 由 `sub_197A2`（每次畫面更新呼叫一次）
    與 `sub_19888` 走訪，格式是**以 `0xFF` 分段的清單**：

        段 0（控制清單）  (序列號, 延遲) 反覆，跑完回到開頭重來
        段 1..N（序列）   (影格號, 延遲) 反覆

    控制清單的序列號**第 7 位若為 1**，換序列時另外呼叫 `-$7BB4(a4)`
    （低 7 位是參數），序列號本身取低 7 位。序列號 N ＝ 從表頭往後跳過
    N 個 `0xFF`。影格號超過張數時當成 0（基準圖）。延遲的單位是畫面更新
    次數，`0` 表示下一次就換，所以停留 `max(延遲, 1)` 次。
    """
    body = 0x31 + d[0x30] - 1
    r = parse(d[body:], strict=False)
    if r is None or r["colors"] is None:
        return None
    w, h, _ = r["entries"][0]
    if (w, h) != (d[2], d[3]):
        return None
    pos = body + r["data"]
    raw = []
    for fw, fh, _fl in r["entries"]:
        px, pos = unrle(d, pos, fh * ((fw + 15) // 16) * 5)
        raw.append((fw, fh, px))
    rects = [tuple(d[4 + 4 * i:8 + 4 * i]) for i in range(11)]
    table = list(d[0x31:0x31 + d[0x30] - 1])
    return {"w": w, "h": h, "colors": r["colors"], "raw": raw,
            "rects": rects, "table": table}


def anm_sequences(table):
    """把動畫表切成 [控制清單, 序列 1, 序列 2, …]，每段是 (值, 延遲) 的串。

    分隔的 `0xFF` **只佔一個位元組**（不是一對），所以要逐位元組走，
    不能兩個兩個跳 —— 跳著走會在第一個奇數位置的分隔符之後整段錯位。
    """
    segs, cur, pos = [], [], 0
    while pos < len(table):
        if table[pos] == 0xFF:
            segs.append(cur)
            cur = []
            pos += 1
            continue
        if pos + 1 >= len(table):
            break
        cur.append((table[pos], table[pos + 1]))
        pos += 2
    if cur:
        segs.append(cur)
    return segs


def anm_playlist(table, nframes: int):
    """把控制清單展開成一串 (影格號, 停留次數)。

    這是 `sub_197A2` ＋ `sub_19888` 的靜態展開：控制清單逐項挑序列，
    序列逐項挑影格。控制清單自己的延遲加在該序列的最後一格上。
    """
    segs = anm_sequences(table)
    if not segs:
        return []
    out = []
    for sel, gap in segs[0]:
        idx = sel & 0x7F
        if idx <= 0 or idx >= len(segs):
            continue
        for fr, hold in segs[idx]:
            if fr >= nframes:
                fr = 0
            out.append([fr, max(hold, 1)])
        if out and gap:
            out[-1][1] += gap
    return out


def anm_frames(d: bytes):
    """把一個 `.anm` 攤成影格。回傳 (w, h, 調色盤, [索引陣列, …], 播放清單)。

    第 0 張是基準圖，其餘每一張是把零件貼上去之後的完整畫面 ——
    貼的是**整塊矩形**（原版就是把那一塊蓋掉，不做透空合成）。
    """
    got = anm_parts(d)
    if got is None:
        return None
    w, h = got["w"], got["h"]
    base = _unpack(got["raw"][0][2], w, h)
    frames = [base]
    for i, (fw, fh, px) in enumerate(got["raw"][1:]):
        x, y, rw, rh = got["rects"][i]
        if x >= w or y >= h:
            continue
        part = _unpack(px, fw, fh)
        comp = base.copy()
        cy, cx = min(fh, h - y), min(fw, w - x)
        comp[y:y + cy, x:x + cx] = part[:cy, :cx]
        frames.append(comp)
    return w, h, got["colors"], frames, anm_playlist(got["table"], len(frames))


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

    if sys.argv[1] == "--export-monsters":
        import os

        outdir, data_dir = sys.argv[2], sys.argv[3]
        sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
        import monpack

        pics, masks = [], {}
        for i, path in enumerate(sorted(sys.argv[4:])):
            got = anm_frames(open(path, "rb").read())
            if got is None:
                print(f"{path}: 解不開")
                continue
            w, h, pal, frames, play = got
            masks[i] = frames[0] != 0
            pics.append((i, pal, frames, play))
        slot_of = monpack.assign(masks, monpack.dos_masks(data_dir))
        monpack.write(outdir, "Amiga (1989)", w, h, pics, slot_of)
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
