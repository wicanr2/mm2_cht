#!/usr/bin/env python3
"""把 Mega Drive 版的圖形素材**全部**抓出來並編目。

    tools/mdassets.py <ROM> --out workplace/gfx/md-all

先前三支工具各管一種：`mdgfx.py` 管帶區塊頭的 tile 池、`mdview.py` 管第一人稱
視野的點陣圖、`mdlzss_scan.py` 只列出位址不畫圖。這一支把三邊合起來跑一次，
輸出一份總表加全部的 PNG —— 目的是「這顆 ROM 裡還有沒有沒被看過的圖」
這個問題有一個可重跑的答案。

## 三種家族怎麼分

同一套 LZSS（ROM `0x29954`）壓的東西，外面包的頭有三種：

| 家族 | 判準 | 內容 |
|---|---|---|
| `tiles` | 16 bytes 區塊頭 ＋ `F0FF` magic，且 `(rawSize−2)/32` 等於解出來的第一個字 | 8×8 tile 池（怪物、介面、場景插畫）|
| `view` | 來源前 4 bytes 是 `u16 寬(bytes)`、`u16 高`，且 `寬 × 高 == rawSize` | 第一人稱視野的點陣圖（牆、地板、天空）|
| `map` | `rawSize == 4100`（4 bytes 頭 ＋ 64×32 cell × 2）| 整頁介面的 tilemap（建角畫面、遊戲主畫面的框）|
| `pool` | 沒有區塊頭，`rawSize` 是 32 的倍數（裸）或 `32n + 2`（開頭兩個位元組是張數）| tile 池 |
| `text` | 解出來九成以上是可列印 ASCII | 壓縮過的英文劇本 |
| `other` | 只通過 LZSS 的結構條件（解出長度與吃掉長度都與宣告相符）| 還沒歸類的 |

三個判準都是**兩個獨立欄位互相印證**，不是「解出來看起來合理」——
LZSS 餵什麼都吐得出東西，長度以外沒有別的免費驗收條件。

## 調色盤

`--palette-line` 選 ROM 內建那份 CRAM（`0x6FBEA`，四條 16 色）的哪一條。
第一人稱視野依**區域類型**用不同條（`AREA_PAL`），tile 池則各自在區塊頭前面
32 bytes 有自己的調色盤（沒有的沿用上一個，原版就是這樣）。
"""

from __future__ import annotations

import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mdgfx  # noqa: E402
import mdview  # noqa: E402
import mdlzss_scan  # noqa: E402
import mdscene  # noqa: E402

# 區域類型 → ROM 內建 CRAM 的第幾條。取自 `0x8166` 起那一段：
# 依 `-$4C6(a5)`（區域類型）與 `-$4C4(a5)`（戶外旗標）把 32 bytes 從
# `0x6FBEA + N×0x20` 抄進工作區，再乘亮度上傳 CRAM 第 2 條。
AREA_PAL = {0: 1, 1: 2, 2: 0, 5: 0}
OUTDOOR_PAL = 3

# 整頁介面的 tile 池：256 個 tile，沒有區塊頭。三張 4100 bytes 的 tilemap
# （建角畫面、遊戲主畫面的框…）的 tile 索引都落在 1–255、調色盤線都是 0，
# 與這個池的張數剛好對得上。
UI_POOL = 0x06E9A2
MAP_W, MAP_H = 64, 32
MAP_RAW = 4 + MAP_W * MAP_H * 2


def view_header(rom: bytes, src: int):
    """來源指標處是不是第一人稱視野的點陣圖頭。回傳 (寬像素, 高) 或 None。"""
    if src < 8 or src + 8 > len(rom):
        return None
    w = int.from_bytes(rom[src - 4:src - 2], "big")
    h = int.from_bytes(rom[src - 2:src], "big")
    raw = int.from_bytes(rom[src + 4:src + 8], "big")
    if w and h and w * h == raw:
        return w * 2, h
    return None


def referenced(rom: bytes):
    """回傳「ROM 裡出現過的 32 位元值」集合，用來判斷一個區塊有沒有人指到。

    掃描器是拿結構條件（解出長度與吃掉長度都符合宣告）找區塊的，
    三十九萬次嘗試下必然有假陽性。**「有沒有程式指向它」是獨立的第二個條件**
    —— 真素材一定有人載它，位址會以立即值或指標表的形式出現在 ROM 裡。
    這一招在找調色盤時就用過一次（`0x6FBEA` 命中八處、`0x6FBE8` 零命中）。
    """
    import numpy as np

    n = len(rom) // 2 * 2
    words = np.frombuffer(rom[:n], dtype=">u4", count=(n - 2) // 4)
    odd = np.frombuffer(rom[2:n - 2], dtype=">u4", count=(n - 4) // 4)
    return set(words.tolist()) | set(odd.tolist())


def dedupe(found):
    """重疊的候選只留第一個：同一段位元流從不同起點也可能湊出合法解。"""
    kept, end = [], -1
    for rec in found:
        if rec[0] < end:
            continue
        kept.append(rec)
        end = rec[0] + 8 + rec[1]
    return kept


def unpack_tiles(data: bytes, n: int):
    """4bpp tile 資料 → (n, 8, 8) 的索引陣列。"""
    import numpy as np

    body = np.frombuffer(bytes(data[:n * 32]).ljust(n * 32, b"\0"),
                         dtype=np.uint8).reshape(n, 8, 4)
    px = np.empty((n, 8, 8), np.uint8)
    px[:, :, 0::2] = body >> 4
    px[:, :, 1::2] = body & 15
    return px


def save_map(cells: bytes, tiles, pal, path: str, scale: int = 1):
    """把 64×32 的 nametable 用 tile 池畫出來。

    項的低 11 位是 tile 編號、bit12–11 是翻轉、bit14–13 是調色盤線。
    這三張圖的調色盤線全部是 0，所以這裡只用一條。
    """
    import numpy as np

    nt = np.frombuffer(cells[:MAP_W * MAP_H * 2], dtype=">u2").reshape(MAP_H, MAP_W)
    img = np.zeros((MAP_H * 8, MAP_W * 8), np.uint8)
    for y in range(MAP_H):
        for x in range(MAP_W):
            t = int(nt[y, x]) & 0x7FF
            if 0 < t < len(tiles):
                img[y * 8:y * 8 + 8, x * 8:x * 8 + 8] = tiles[t]
    mdscene.save(img, pal, path, scale)


def save_tiles(data: bytes, n: int, pal, path: str, cols: int = 32, scale: int = 2):
    """把 n 個 8×8 tile 畫成總覽圖。

    **tile 數要由呼叫端給**：`mdgfx.decode()` 回傳的資料已經去掉開頭那兩個
    位元組的計數，再從裡面讀一次計數會讀到像素。
    **用 numpy 展開，不要逐像素 `putpixel`** —— 一萬多個 tile 是八十幾萬
    個像素，逐格寫會讓整支工具從兩秒變成跑不完。
    """
    import numpy as np

    if n <= 0:
        return 0
    body = np.frombuffer(bytes(data[:n * 32]).ljust(n * 32, b"\0"),
                         dtype=np.uint8).reshape(n, 8, 4)
    px = np.empty((n, 8, 8), np.uint8)
    px[:, :, 0::2] = body >> 4
    px[:, :, 1::2] = body & 15
    rows = (n + cols - 1) // cols
    sheet = np.zeros((rows * 8, cols * 8), np.uint8)
    for t in range(n):
        ty, tx = divmod(t, cols)
        sheet[ty * 8:ty * 8 + 8, tx * 8:tx * 8 + 8] = px[t]
    mdscene.save(sheet, pal, path, scale)
    return n


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--out", default="workplace/gfx/md-all")
    ap.add_argument("--palette-line", type=int, default=2)
    ap.add_argument("--scale", type=int, default=2)
    args = ap.parse_args()

    rom = open(args.rom, "rb").read()
    out = args.out
    for sub in ("tiles", "view", "map", "pool", "text", "other", "palette"):
        os.makedirs(os.path.join(out, sub), exist_ok=True)

    # 1. tile 池（帶區塊頭）
    pools = mdgfx.blocks(rom)
    pool_at = {b["data"]: b for b in pools}

    # 2. 第一人稱視野的點陣圖：從 `sub_FC38` 的表與地板／天空指標拿，
    #    這一組有明確來源，不必靠掃描。
    scene = {}
    tables = mdscene.area_tables(rom)
    for ai, t in enumerate(tables):
        line = OUTDOOR_PAL if ai == 2 else AREA_PAL.get([0, 2, 3][ai], 0)
        for src in set(v for v in t.values() if v):
            scene[src] = line
    for ai in range(3):
        scene[mdscene.FLOOR[ai]] = OUTDOOR_PAL if ai == 2 else AREA_PAL.get([0, 2, 3][ai], 0)
        scene[mdscene.SKY_DAY[ai]] = OUTDOOR_PAL if ai == 2 else AREA_PAL.get([0, 2, 3][ai], 0)
    scene[mdscene.SKY_NIGHT] = 0

    # 3. 全 ROM 的 LZSS 區塊
    refs = referenced(rom)
    blocks = dedupe(mdlzss_scan.scan(rom))
    # 介面 tilemap 要配的 tile 池
    ui_raw = int.from_bytes(rom[UI_POOL + 4:UI_POOL + 8], "big")
    ui_data = mdgfx.lzss(rom, UI_POOL + 8, ui_raw)[0]
    ui_tiles = unpack_tiles(ui_data[2:], int.from_bytes(ui_data[:2], "big"))
    # 掃描器回傳的 `src` 就是 LZSS 頭（`comp`/`raw`）的位址，位元流在 +8。
    # **這裡不要再加 8** —— 加了之後 `view_header` 會把 `raw` 讀成寬高，
    # 整批視野點陣圖就會被歸到「其他」，而且總表上的位址全部偏 8。
    decoded = {src: data for src, _c, _r, data in blocks}
    print(f"LZSS 區塊 {len(blocks)} 個、tile 池 {len(pools)} 個，開始輸出…")

    rows = []
    # 場景那一批的來源是從程式裡讀出來的，不靠掃描；tile 池自成一家。
    todo = [(s, None) for s in sorted(scene)] + [(b["data"], b) for b in pools]
    todo += [(src, None) for src, _c, _r, _d in blocks]

    done = set()
    n_tiles = n_view = n_other = n_map = n_pool = n_text = 0
    for src, pool in sorted(todo):
        if src in done:
            continue
        done.add(src)
        wh = view_header(rom, src)
        raw_here = int.from_bytes(rom[src + 4:src + 8], "big") if src + 8 <= len(rom) else 0
        if wh is None and src in refs and raw_here != MAP_RAW:
            # 來源有人指到時，高度欄位與 `rawSize` 衝突就以 `rawSize` 為準。
            # **但要先排掉 4100 那一種**：介面 tilemap 的寬度欄位剛好也整除
            # `rawSize`，不擋掉會被當成一張細長的點陣圖。
            got = mdview.decode(rom, src, relaxed=True)
            if got and 0 < got[1] <= 256 and 0 < got[0] <= 512:
                wh = got[0], got[1]
        if pool is not None or src in pool_at:
            b = pool or pool_at[src]
            data = mdgfx.decode(rom, b)
            pal = mdgfx.palette(rom, b["pal"]) if b["pal"] else mdscene.rom_palette(rom, args.palette_line)
            name = f"tiles/{b['data']:06X}-id{b['id']:03d}-{b['tiles']}t.png"
            got = save_tiles(data, b["tiles"], pal, os.path.join(out, name),
                             scale=args.scale)
            rows.append(("tiles", b["data"], f"{got} tiles", f"id {b['id']}", name))
            n_tiles += 1
            continue
        if wh:
            w, h = wh
            raw = int.from_bytes(rom[src + 4:src + 8], "big")
            data = decoded.get(src) or mdgfx.lzss(rom, src + 8, raw)[0]
            line = scene.get(src, args.palette_line)
            pal = mdscene.rom_palette(rom, line)
            name = f"view/{src:06X}-{w}x{h}.png"
            mdscene.save(mdscene.to_indexed(w, h, data), pal,
                         os.path.join(out, name), args.scale)
            rows.append(("view", src, f"{w}×{h}", f"調色盤第 {line} 條", name))
            n_view += 1
            continue
        raw = int.from_bytes(rom[src + 4:src + 8], "big")
        data = decoded.get(src)
        if data is None:
            continue
        printable = sum(1 for b in data if 32 <= b < 127 or b in (9, 10, 13))
        if printable >= len(data) * 9 // 10:
            name = f"text/{src:06X}.txt"
            with open(os.path.join(out, name), "w", encoding="latin1") as fh:
                fh.write(data.decode("latin1"))
            head = data[:40].decode("latin1").replace("|", "｜").strip()
            rows.append(("text", src, f"{len(data)} bytes", f"`{head}…`", name))
            n_text += 1
            continue
        if raw == MAP_RAW:
            name = f"map/{src:06X}.png"
            save_map(data[4:], ui_tiles, mdscene.rom_palette(rom, 0),
                     os.path.join(out, name), args.scale)
            rows.append(("map", src, f"{MAP_W}×{MAP_H} cell",
                         "配 `0x06E9A2` 的 256 個 tile", name))
            n_map += 1
            continue
        if src in refs and raw % 32 in (0, 2):
            # `32n + 2` 的那一種開頭兩個位元組是張數（與帶區塊頭的那一族同慣例），
            # 對得起來才當它是 tile 池 —— 又是兩個獨立欄位互相印證。
            if raw % 32 == 2:
                n = int.from_bytes(data[:2], "big")
                body, note = data[2:], "開頭兩個位元組是張數"
                if n != (raw - 2) // 32:
                    n, body, note = raw // 32, data, "張數欄與長度對不上，照長度切"
            else:
                n, body, note = raw // 32, data, "沒有區塊頭"
            name = f"pool/{src:06X}-{n}t.png"
            save_tiles(body, n, mdscene.rom_palette(rom, 0),
                       os.path.join(out, name), scale=args.scale)
            rows.append(("pool", src, f"{n} tiles", note, name))
            n_pool += 1
            continue
        # 其他：原樣存起來，順便記下它像不像 tile
        name = f"other/{src:06X}-{raw}.bin"
        with open(os.path.join(out, name), "wb") as fh:
            fh.write(data)
        note = []
        if src in refs:
            note.append("**有人指到**")
        if raw % 32 == 0:
            note.append("長度是 32 的倍數（可能是 tile）")
        rows.append(("other", src, f"{raw} bytes", "；".join(note), name))
        n_other += 1

    # 4. 調色盤：全 ROM 掃一遍，畫成色票。
    #
    # 先用 numpy 一次算出「哪些字合法」，再只在合法的長串上做細判 ——
    # 逐位址呼叫 `is_palette` 是三十九萬次 struct.unpack，會拖垮整支工具。
    import numpy as np

    words = np.frombuffer(rom[:len(rom) // 2 * 2], dtype=">u2")
    # 16 位元的補集要自己寫死：numpy 的 `~0x0EEE` 會變成負的 Python 整數，
    # 對 uint16 陣列直接 OverflowError。
    ok = (words & np.uint16(0xF111)) == 0
    pals, i = [], 0
    while i + 16 <= len(words):
        if not ok[i:i + 16].all():
            # 跳到這一段裡最後一個不合法的字之後
            bad = np.nonzero(~ok[i:i + 16])[0][-1]
            i += int(bad) + 1
            continue
        if mdgfx.is_palette(rom, i * 2):
            pals.append(i * 2)
            i += 16
        else:
            i += 1
    if pals:
        from PIL import Image

        sw = np.zeros((len(pals), 16, 3), np.uint8)
        for r, off in enumerate(pals):
            sw[r] = np.array(mdgfx.palette(rom, off), np.uint8)
        sw = np.repeat(np.repeat(sw, 12, axis=0), 12, axis=1)
        Image.fromarray(sw).save(os.path.join(out, "palette", "all.png"))

    with open(os.path.join(out, "index.md"), "w", encoding="utf-8") as fh:
        fh.write("# Mega Drive 圖形素材總表\n\n")
        cited = sum(1 for r in rows if r[0] == "other" and "有人指到" in r[3])
        fh.write(f"由 `tools/mdassets.py` 產生。tile 池 {n_tiles} 個、"
                 f"視野點陣圖 {n_view} 張、介面 tilemap {n_map} 張、"
                 f"tile 池（無區塊頭）{n_pool} 個、文字區塊 {n_text} 個、"
                 f"其他 LZSS 區塊 {n_other} 個"
                 f"（其中 {cited} 個在 ROM 裡有人指到，其餘多半是掃描的假陽性）、"
                 f"16 色調色盤 {len(pals)} 組。\n\n")
        fh.write("| 家族 | 位址 | 尺寸／數量 | 備註 | 檔案 |\n|---|---|---|---|---|\n")
        for fam, addr, size, note, name in sorted(rows, key=lambda r: (r[0], r[1])):
            fh.write(f"| {fam} | `0x{addr:06X}` | {size} | {note} | `{name}` |\n")
        fh.write("\n調色盤位址：" + "、".join(f"`0x{p:06X}`" for p in pals) + "\n")

    print(f"tile 池 {n_tiles}、視野點陣圖 {n_view}、介面 tilemap {n_map}、"
          f"tile 池（無區塊頭）{n_pool}、文字 {n_text}、其他 {n_other}、"
          f"調色盤 {len(pals)} → {out}")


if __name__ == "__main__":
    main()
