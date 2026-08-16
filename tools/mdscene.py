#!/usr/bin/env python3
"""解 Mega Drive 版第一人稱視野的**版面**：哪一張貼在哪裡，以及怎麼組成一格畫面。

    tools/mdscene.py <ROM> --sheet workplace/gfx/mdscene   # 每個區域類型的素材總覽
    tools/mdscene.py <ROM> --compose workplace/gfx/mdscene # 照牆值組出整幅視野
    tools/mdscene.py <ROM> --export workplace/md-scene     # 烘成 remake 吃的素材包

貼圖本身的格式（LZSS ＋ 寬高在來源前面）在 `tools/mdview.py`，這一支只管版面。

## 版面怎麼來的

`sub_FC38(區域類型)` 把一整組貼圖指標填進 RAM `0xFFCD40` 的 52 個長字。
使用端（`sub_3DE2` 與 `sub_34E8`）的索引算式都是

    索引 = 牆值 × 0x50 + (位置 − 20) × 4

牆值就是視野格子陣列 `0xFFF3CA` 那一格的 2-bit 值（1–3，0 ＝ 沒有牆），
所以一個牆值佔 20 個位置，整張表 **60 個長字**。執行時的 RAM 傾印顯示
**第 40–59 格與第 0–19 格逐項相同** —— 也就是**牆值 3 用的是牆值 1 的圖**，
差別在另外加畫的火炬（`sub_34E8` 那一路）。`sub_3DE2` 裡二十個貼圖點的目的位址都是立即值，
組合緩衝區從 `0xFF0340` 起、列距 `0x68`（104 bytes ＝ 208 像素 4bpp），
換算出來就是 `PLACE`。

**填表有一個會讓掃描漏東西的寫法**：位置 7、8 與 10、11 不是 `move.l #imm`，
而是 `move.l d16(a5),d16(a5)` 把位置 6、9 抄過去。只掃立即值會得到
「那幾格沒填」，而那與「原版真的沒填」長得一模一樣。

## 二十個位置

四個深度的正面牆各有左／中／右三張（位置 0–11），加上八根 120 高的側牆柱
（位置 12–19）。側牆柱的 x 邊界 0/24/56/80/96 ｜ 112/128/152/184/208 正好是
同深度正牆的左右緣 —— 與 DOS 版「側牆寬度累加 24→56→80→96 等於同深度正牆
左緣」是同一套幾何，只是 Mega Drive 把一整根柱子烘成一張圖。

畫的順序是**由遠而近**（`sub_3DE2` 的程式碼順序）：最遠的正牆 → 最內側的
側牆柱 → 遠 → 次內柱 → 中 → 次外柱 → 近 → 最外柱。照這個順序疊才對，
反過來近的會被遠的蓋掉。

## 驗過了：與執行時的緩衝區逐像素相同

拿 BlastEm 傾印的視野格子陣列餵這一支，組出來的 208×120 與模擬器自己的
組合緩衝區 **24,960 個像素全部相同**（非透空的 9,866 個也全中）。
重跑方式見 `docs/research/02-other-platforms.md`。

**目的位址是線性的，記憶體不是。** `dest − 0xFF0340` 除以 `0x68` 得列、
餘數乘 2 得行 —— 這樣算出來的落點是對的（上面那個 100% 就是證據）。
但緩衝區在 RAM 裡是 **VDP 的 tile 順序**（32 bytes 一個 8×8 tile、26 個一列、
共 16 列），把它當成「每列 104 bytes 的點陣圖」畫出來是雜訊。
兩件事都成立，因為做轉換的是 blitter。
"""

from __future__ import annotations

import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mdview  # noqa: E402

# 組合緩衝區
BUF = 0xFF0340
STRIDE = 0x68          # bytes/列
VIEW_W = STRIDE * 2    # 208 像素
VIEW_H = 120

# 位置 → (x, y)，由 sub_3DE2 的目的位址立即值換算。
PLACE = [
    (0, 14), (24, 14), (184, 14),          # 0–2   近距 左／正／右
    (0, 32), (56, 32), (152, 32),          # 3–5   中距
    (32, 45), (80, 45), (128, 45),         # 6–8   遠距
    (80, 54), (96, 54), (112, 54),         # 9–11  最遠
    (0, 0), (24, 0), (56, 0), (80, 0),     # 12–15 左側牆柱，近 → 遠
    (112, 0), (128, 0), (152, 0), (184, 0),  # 16–19 右側牆柱，遠 → 近
]

# sub_FC38 開頭那張七格跳表：區域類型 → 三塊表格填法之一。
# 偏移 0x0010／0x01EC／0x03F8 三個目標，所以七種區域類型只有三套素材。
AREA_TYPES = [[0, 1], [2, 5], [3, 4, 6]]

# 地板與天空：`sub_30B4` 依區域類型挑，貼圖點在 `sub_31D0`。
#
#   天空  → 緩衝區 +0x340（視圖第 0 列）
#   地板  → 緩衝區 +0x340 + 61 或 63 列
#
# 有天花板的格子（`sub_3064` 為真）不畫天空，改呼叫 thunk `0x18`
# （ROM `0x29ECC`）把上面 61 列**整片填成索引 1**。
FLOOR = [0x7898C, 0x71378, 0x9D852]
SKY_DAY = [0x70AB0, 0x70AB0, 0x9DF4E]
SKY_NIGHT = 0x6FC6E      # `-$544(a5) > 0x80` 時改用這一張，remake 還沒接
CEIL_FILL_H = 61         # thunk 0x18 填幾列
CEIL_FILL_INDEX = 1

# sub_3BE2 的程式碼順序 ＝ 由遠而近的疊圖順序。
DRAW_ORDER = [9, 10, 11, 15, 16, 6, 7, 8, 14, 17, 3, 4, 5, 13, 18, 0, 2, 1, 12, 19]

# 位置的中文說明，只給總覽圖與報表用。
LABEL = [
    "近·左", "近·正", "近·右", "中·左", "中·正", "中·右",
    "遠·左", "遠·正", "遠·右", "最遠·左", "最遠·正", "最遠·右",
    "左柱·深0", "左柱·深1", "左柱·深2", "左柱·深3",
    "右柱·深3", "右柱·深2", "右柱·深1", "右柱·深0",
]

FC38, FC38_END = mdview.FC38, mdview.FC38_END
TABLE_BASE = -0x35D2

GRAY = [(255, 0, 255)] + [(i * 17, i * 17, i * 17) for i in range(1, 16)]

# ROM 裡的整份 CRAM：`0x6FBEA` 起 **128 bytes ＝ 四條 16 色**，就接在
# 夜空 `0x6FC6E` 的影像頭前面。判準有三條同時成立：
#
#   1. `0x6FBE8`–`0x6FC6C` 這 132 bytes 是整個 `0x6F900`–`0x6FD00` 裡
#      唯一一段值全部符合 `& 0x0EEE` 的連續區，扣掉頭尾對齊剛好 128；
#   2. **程式參考的是 `0x6FBEA` 不是 `0x6FBE8`** —— 全 ROM 掃 32-bit
#      立即值，`0x0006FBEA` 命中八處，`0x0006FBE8` 零命中；
#   3. 那八處都接著 `sub_2DBA(1, …)`（目標 1 ＝ CRAM），長度是
#      `0x20`／`0x40`／`0x60`，也就是一次上傳一到三條。
#
# 視野的顏色永遠上傳到 **CRAM 第 2 條**（nametable 項值 bit14–13 ＝ 10），
# 但**來源是這四條裡的哪一條，由區域類型決定**（`0x8166` 起那一段：
# 依 `-$4C6(a5)` 與 `-$4C4(a5)` 把 32 bytes 從 `0x6FBEA + N×0x20` 抄進工作區）：
#
#     區域類型 0        → 第 1 條
#     區域類型 1        → 第 2 條
#     區域類型 2、5     → 第 0 條
#     戶外（-$4C4 == 1）→ 第 3 條
#
# 執行時的 CRAM 不會等於這一份：原版依**光照**把它調暗（分量 × 亮度 ÷ 8）。
# 實測沒有光源的地城那一幀是亮度 2，也就是分量 >> 2，十六格全中。
# remake 拿沒調暗的原值烘。
ROM_CRAM = 0x6FBEA

# 三塊素材各自該用第幾條。第 0 塊同時服務區域類型 0 與 1，**那兩型用的不是
# 同一條**（1 與 2）—— 同一批牆面素材在兩種區域類型下是不同顏色：
# 城鎮（類型 0）是藍紫色的亂石，地城（類型 1）是灰色的。這裡取城鎮那一條。
#
# 實機對照：Middlegate (8,0) 面東是**類型 0**，畫面是藍紫亂石 ＋ 紅褐地磚
# （`workplace/genesis/out/light/04-town-day-screen.png`）。
BLOCK_PAL_LINE = [1, 0, 3]


def _w(rom, p):
    return int.from_bytes(rom[p:p + 2], "big")


def _sw(rom, p):
    return int.from_bytes(rom[p:p + 2], "big", signed=True)


def _l(rom, p):
    return int.from_bytes(rom[p:p + 4], "big")


def area_tables(rom: bytes):
    """走一遍 `sub_FC38`，回傳每個區域類型的 {表格位置: 來源指標}。

    只認三種指令就夠了：push 立即值、pop 進表、表格內互抄。
    抄那一種**一定要處理** —— 位置 7、8、10、11 全靠它。
    """
    out, cur, pend = [], {}, None
    p = FC38
    while p < FC38_END:
        op = _w(rom, p)
        if op == 0x2EBC:                       # move.l #imm32,(sp)
            pend = _l(rom, p + 2)
            p += 6
            continue
        if op == 0x2B5F:                       # move.l (sp)+,d16(a5)
            slot = (_sw(rom, p + 2) - TABLE_BASE) // 4
            if slot == 0 and cur:
                out.append(cur)
                cur = {}
            cur[slot] = pend
            p += 4
            continue
        if op == 0x2B6D:                       # move.l d16(a5),d16(a5)
            src = (_sw(rom, p + 2) - TABLE_BASE) // 4
            dst = (_sw(rom, p + 4) - TABLE_BASE) // 4
            cur[dst] = cur.get(src)
            p += 6
            continue
        p += 2
    if cur:
        out.append(cur)
    return out


def variant_slots(table: dict, variant: int):
    """牆值 variant（1 起算）的二十個位置 → 來源指標，沒有的位置給 None。"""
    base = (variant - 1) * 20
    return [table.get(base + i) for i in range(20)]


def load_images(rom: bytes, srcs):
    """把一組來源指標解成 {位置: (w, h, 4bpp 資料)}。"""
    out = {}
    for i, src in enumerate(srcs):
        if not src:
            continue
        # 來源是從 `sub_FC38` 的表裡讀出來的，不是掃描猜的，所以用 relaxed ——
        # 有一筆（`0x076B6A`）的高度欄位與 `rawSize` 矛盾，以 `rawSize` 為準。
        got = mdview.decode(rom, src, relaxed=True)
        if got:
            out[i] = got
    return out


def to_indexed(w: int, h: int, data: bytes):
    """4bpp packed → (h, w) 的索引陣列。"""
    import numpy as np

    a = np.frombuffer(bytes(data).ljust(h * (w // 2), b"\0"), dtype=np.uint8)
    a = a[:h * (w // 2)].reshape(h, w // 2)
    out = np.empty((h, w), np.uint8)
    out[:, 0::2] = a >> 4
    out[:, 1::2] = a & 15
    return out


def compose(rom: bytes, table: dict, walls, palette=None):
    """照牆值陣列組出一幅視野。walls 是二十格的牆值（0 ＝ 沒有牆）。

    回傳 (h, w) 的索引陣列；索引 0 是透空／背景。
    """
    import numpy as np

    canvas = np.zeros((VIEW_H, VIEW_W), np.uint8)
    for pos in DRAW_ORDER:
        v = walls[pos]
        if not v:
            continue
        src = table.get((v - 1) * 20 + pos) or table.get(pos)
        if not src:
            continue
        got = mdview.decode(rom, src, relaxed=True)
        if not got:
            continue
        w, h, data = got
        im = to_indexed(w, h, data)
        x, y = PLACE[pos]
        ch, cw = min(h, VIEW_H - y), min(w, VIEW_W - x)
        if ch <= 0 or cw <= 0:
            continue
        canvas[y:y + ch, x:x + cw] = im[:ch, :cw]
    return canvas


def rom_palette(rom: bytes, line: int = 0):
    """讀 ROM 內建的那份 CRAM 的第 line 條（16 色）。"""
    out = []
    for i in range(16):
        v = _w(rom, ROM_CRAM + line * 32 + i * 2)
        out.append((((v & 0x00E) >> 1) * 36, ((v & 0x0E0) >> 5) * 36,
                    ((v & 0xE00) >> 9) * 36))
    return out


def read_palette(path: str):
    """讀 BlastEm 傾印的 CRAM（64 個 big-endian 16-bit）→ 四條 16 色的 RGB 表。"""
    b = open(path, "rb").read()
    if len(b) < 128:
        raise SystemExit(f"{path} 只有 {len(b)} bytes，CRAM 要 128")
    lines = []
    for ln in range(4):
        pal = []
        for i in range(16):
            v = int.from_bytes(b[ln * 32 + i * 2:ln * 32 + i * 2 + 2], "big")
            r = (v & 0x00E) >> 1
            g = (v & 0x0E0) >> 5
            bl = (v & 0xE00) >> 9
            pal.append((r * 36, g * 36, bl * 36))
        lines.append(pal)
    return lines


def save(arr, palette, path, scale=2):
    from PIL import Image

    h, w = arr.shape
    im = Image.new("P", (w, h))
    flat = []
    for c in palette:
        flat += list(c)
    flat += [0] * (768 - len(flat))
    im.putpalette(flat)
    im.putdata(arr.flatten().tolist())
    if scale > 1:
        im = im.resize((w * scale, h * scale), Image.NEAREST)
    im.save(path)


# REMAKE_SLOT[remake 槽號] = Mega Drive 的位置編號。
#
# remake 的第一人稱畫法（`internal/view/firstperson.go`）沿用 DOS 的槽位：
# `d` 是第 d 個深度的正牆、`4+d` 是左側牆、`8+d` 是右側牆，`+16` 是門的變體。
# Mega Drive 的正牆是位置 1／4／7／10，左側牆柱由近而遠是 12／13／14／15，
# 右側牆柱由近而遠是 19／18／17／16。
#
# **位置 0／2、3／5、6／8、9／11 沒有對應的 remake 槽位** —— 那是斜前方
# 那一格的正面，DOS 版的畫法裡沒有這一層，所以不烘。少畫的是斜角那一小塊，
# 不是牆不見了。
REMAKE_SLOT = [1, 4, 7, 10,        # 0–3   正牆，深度 0–3
               12, 13, 14, 15,     # 4–7   左側牆
               19, 18, 17, 16]     # 8–11  右側牆


def export(rom: bytes, tables, pal_of, outdir: str) -> None:
    """烘成 remake 吃的素材包：每個區域類型一個子目錄。

    檔名用 **remake 的槽號**（`walls/00.png`…），不是 Mega Drive 的位置編號 ——
    讀取端不必再帶一張對照表。牆值 1 出 0–11、牆值 2 出 16–27（門的變體）。
    """
    os.makedirs(outdir, exist_ok=True)
    meta = {"source": "Mega Drive (1991)", "view": [VIEW_W, VIEW_H],
            "clear": 0, "areas": []}
    import numpy as np

    for ai, t in enumerate(tables):
        pal = pal_of(ai)
        sub = os.path.join(outdir, f"area{ai}")
        os.makedirs(os.path.join(sub, "walls"), exist_ok=True)
        place, fell_back = {}, []
        base = load_images(rom, variant_slots(t, 1))
        for v in sorted({k // 20 + 1 for k in t}):
            if v > 2:
                continue          # remake 只有兩個變體（一般牆與門）
            got = load_images(rom, variant_slots(t, v))
            for rs, pos in enumerate(REMAKE_SLOT):
                img = got.get(pos)
                if img is None and v > 1:
                    # 門的變體缺一張就退回一般牆那一張 —— 少一扇門的裝飾，
                    # 好過整組素材載不進去。哪幾格退回了記在 scene.json。
                    img = base.get(pos)
                    if img is not None:
                        fell_back.append(rs + (v - 1) * 16)
                if img is None:
                    continue
                w, h, data = img
                slot = rs + (v - 1) * 16
                save(to_indexed(w, h, data), pal,
                     os.path.join(sub, "walls", f"{slot:02d}.png"), 1)
                place[slot] = list(PLACE[pos])
        # 地板與天空。天空第 0 張是白天的天空，第 1 張是有天花板的格子 ——
        # 原版那一種不貼圖，是把上面 61 列整片填成索引 1。
        os.makedirs(os.path.join(sub, "sky"), exist_ok=True)
        os.makedirs(os.path.join(sub, "floor"), exist_ok=True)
        got = mdview.decode(rom, FLOOR[ai])
        if got:
            save(to_indexed(*got), pal, os.path.join(sub, "floor", "00.png"), 1)
        got = mdview.decode(rom, SKY_DAY[ai])
        if got:
            save(to_indexed(*got), pal, os.path.join(sub, "sky", "00.png"), 1)
        save(np.full((CEIL_FILL_H, VIEW_W), CEIL_FILL_INDEX, np.uint8), pal,
             os.path.join(sub, "sky", "01.png"), 1)
        meta["areas"].append({"area": ai, "dir": f"area{ai}",
                              "types": AREA_TYPES[ai],
                              "place": place, "fellBack": sorted(fell_back)})
    with open(os.path.join(outdir, "scene.json"), "w", encoding="utf-8") as fh:
        json.dump(meta, fh, ensure_ascii=False, indent=1)
        fh.write("\n")
    n = sum(len(a["place"]) for a in meta["areas"])
    print(f"{len(meta['areas'])} 個區域、{n} 張牆 → {outdir}")


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--sheet", help="每個區域類型的素材總覽圖輸出目錄")
    ap.add_argument("--compose", help="組出整幅視野的輸出目錄")
    ap.add_argument("--export", help="烘成 remake 素材包的輸出目錄")
    ap.add_argument("--cram", help="BlastEm 傾印的 CRAM（128 bytes），沒有就用 ROM 內建那份")
    ap.add_argument("--pal-line", type=int, default=None,
                    help="強制用第幾條調色盤線；不給就依素材塊各自挑（BLOCK_PAL_LINE）")
    ap.add_argument("--gray", action="store_true", help="改用灰階（看索引分佈用）")
    ap.add_argument("--scale", type=int, default=2)
    args = ap.parse_args()

    rom = open(args.rom, "rb").read()
    tables = area_tables(rom)
    def pal_for(block: int):
        if args.gray:
            return GRAY
        line = args.pal_line if args.pal_line is not None else BLOCK_PAL_LINE[block]
        if args.cram:
            return read_palette(args.cram)[line]
        return rom_palette(rom, line)

    pal = pal_for(1)

    print(f"{len(tables)} 個區域類型")
    for ai, t in enumerate(tables):
        variants = sorted({k // 20 + 1 for k in t})
        print(f"  區域 {ai}：牆值 {variants}，{len(t)} 個表格位置")
        for v in variants:
            got = load_images(rom, variant_slots(t, v))
            miss = [i for i in range(20) if i not in got]
            print(f"    牆值 {v}：{len(got)}/20 張"
                  + (f"，缺 {miss}" if miss else ""))

    if args.sheet:
        os.makedirs(args.sheet, exist_ok=True)
        for ai, t in enumerate(tables):
            pal = pal_for(ai)
            for v in sorted({k // 20 + 1 for k in t}):
                got = load_images(rom, variant_slots(t, v))
                for pos, (w, h, data) in sorted(got.items()):
                    save(to_indexed(w, h, data), pal,
                         os.path.join(args.sheet,
                                      f"a{ai}_v{v}_{pos:02d}_{w}x{h}.png"),
                         args.scale)
        print(f"素材總覽 → {args.sheet}")

    if args.compose:
        os.makedirs(args.compose, exist_ok=True)
        # 三種代表性的牆值配置：走廊、死路、開闊。
        cases = {
            "corridor": [0] * 12 + [1, 1, 1, 1, 1, 1, 1, 1],
            "deadend": [1, 1, 1] + [0] * 9 + [1, 0, 0, 0, 0, 0, 0, 1],
            "open": [0] * 20,
            "full": [1] * 20,
        }
        for ai, t in enumerate(tables):
            pal = pal_for(ai)
            for name, walls in cases.items():
                save(compose(rom, t, walls, pal),
                     pal, os.path.join(args.compose, f"a{ai}_{name}.png"),
                     args.scale)
        print(f"組合圖 → {args.compose}")

    if args.export:
        export(rom, tables, pal_for, args.export)


if __name__ == "__main__":
    main()
