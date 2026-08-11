#!/usr/bin/env python3
"""解 Amiga 版 MM2 的 `.32` 圖形檔**容器層**（影像目錄與調色盤）。

    tools/amiga32.py workplace/amiga/town.32

像素的編碼還沒解（見 docs/research/02-other-platforms.md），這支到目錄與
調色盤為止：張數、每一張的寬高與旗標、32 個色。有這兩樣就能回答
「這個檔裡有幾張、多大、什麼配色」，而那已經足以與 DOS 版逐張對照。

## 容器

`.32` 是**記憶體映像**，不是串流：整份載進固定位址的緩衝區，程式知道
目錄在 buffer + 固定偏移。所以目錄不在檔頭也不在檔尾，而在中間，
而且同一類檔案的目錄偏移完全相同（`town`／`cave`／`castle` 都在 `0x875C`，
三個檔的大小卻是 57,125／57,205／57,098）。

    <像素資料，壓縮，墊到固定長度>
    uint16  count      張數
    uint16  ?          三個場景檔都是 3
    count × {           每筆 6 bytes，big-endian
        uint16 width
        uint16 height
        uint16 flag     正牆與補牆是 0、側牆是 3
    }
    32    × uint16     調色盤，Amiga 的 12-bit RGB（0x0RGB）
    <更多像素資料>

副檔名的 `32` 是**色數**不是張數 —— 調色盤固定 32 筆。判準是每個 word 的
高 nibble 全為 0，而後面的資料立刻破功（`0xFFFF`）。

尺寸與 DOS 的同名檔逐張對得起來（同一批美術），只是 Amiga 的高度少 1–3 ——
螢幕比例不同。`town.32` 的 32 筆是 16 張 × 兩種變體，與 DOS 的 `TOWN.16`
一模一樣。

## 找目錄的方法

不要用固定偏移。掃全檔，找「連續 N 筆都落在合理值域」的位置：
寬 8–320、高 4–200、旗標 ≤ 8。`town.32` 這樣掃只會命中一處。
"""
import struct
import sys


def find_dir(d: bytes, need: int = 8):
    """回傳 (目錄偏移, 筆數)。找不到回 None。"""
    i = 0
    while i < len(d) - 6 * need:
        n = 0
        while True:
            k = i + 6 * n
            if k + 6 > len(d):
                break
            w, h, f = struct.unpack_from(">HHH", d, k)
            if 8 <= w <= 320 and 4 <= h <= 200 and f <= 8:
                n += 1
            else:
                break
        if n >= need:
            return i, n
        i += 2
    return None


def parse(d: bytes):
    hit = find_dir(d)
    if hit is None:
        return None
    off, n = hit
    # 目錄前面兩個 uint16 是張數與那個未解的欄位。掃描的起點可能早了一兩筆
    # （前面的位元組剛好也落在值域內），所以以「宣告的張數」為準往回對齊。
    for back in range(0, 6 * 4, 2):
        cand = off + back
        count = struct.unpack_from(">H", d, cand - 4)[0]
        second = struct.unpack_from(">H", d, cand - 2)[0]
        if 0 < count <= 200 and cand + 6 * count <= len(d):
            ent = [struct.unpack_from(">HHH", d, cand + 6 * i) for i in range(count)]
            if all(8 <= w <= 320 and 4 <= h <= 200 for w, h, _ in ent):
                pal_at = cand + 6 * count
                return {"dir": cand, "count": count, "field2": second,
                        "entries": ent, "pal": pal_at, "colors": palette(d, pal_at)}
    return None


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
