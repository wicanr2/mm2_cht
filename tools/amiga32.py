#!/usr/bin/env python3
"""解 Amiga 版 MM2 的 `.32` 圖形檔**容器層**（影像目錄）。

    tools/amiga32.py workplace/amiga/town.32

像素的編碼還沒解（見 docs/research/02-other-platforms.md），這支只到目錄為止：
張數、每一張的寬高與旗標。有目錄就能回答「這個檔裡有幾張、多大」，
而那已經足以與 DOS 版逐張對照。

## 容器

`.32` 是**記憶體映像**，不是串流：整份載進固定位址的緩衝區，程式知道
目錄在 buffer + 固定偏移。所以目錄不在檔頭也不在檔尾，而在中間，
而且同一類檔案的目錄偏移完全相同（`town`／`cave`／`castle` 都在 `0x875C`）。

    <像素資料>
    uint16  count      張數
    uint16  ?          三個場景檔都是 3
    count × {           每筆 6 bytes，big-endian
        uint16 width
        uint16 height
        uint16 flag     正牆與補牆是 0、側牆是 3
    }
    <更多像素資料>

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
                return {"dir": cand, "count": count, "field2": second, "entries": ent}
    return None


def main() -> None:
    if len(sys.argv) < 2:
        raise SystemExit(__doc__)
    for path in sys.argv[1:]:
        d = open(path, "rb").read()
        r = parse(d)
        if r is None:
            print(f"{path}: {len(d)} bytes —— 找不到影像目錄")
            continue
        used = sum(((w + 15) // 16 * 2) * 5 * h for w, h, _ in r["entries"])
        print(f"{path}: {len(d)} bytes　目錄 @0x{r['dir']:X}　"
              f"{r['count']} 張　第二欄 {r['field2']}")
        print(f"   5 平面未壓縮需要 {used} bytes，檔案只有 {len(d)} —— "
              f"{'裝得下' if used <= len(d) else '裝不下，所以壓過'}")
        for i, (w, h, f) in enumerate(r["entries"]):
            end = "\n" if i % 4 == 3 else "  "
            print(f"   {i:2d} {w:3d}×{h:<3d} f{f}", end=end)
        if r["count"] % 4:
            print()


if __name__ == "__main__":
    main()
