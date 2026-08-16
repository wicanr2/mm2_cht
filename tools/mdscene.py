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
所以一個牆值佔 20 個位置。`sub_3DE2` 裡二十個貼圖點的目的位址都是立即值，
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
        got = mdview.decode(rom, src)
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
        src = table.get((v - 1) * 20 + pos)
        if not src:
            continue
        got = mdview.decode(rom, src)
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


def export(rom: bytes, tables, pal, outdir: str) -> None:
    """烘成 remake 吃的素材包：每個區域類型一個子目錄。

    檔名用 **remake 的槽號**（`walls/00.png`…），不是 Mega Drive 的位置編號 ——
    讀取端不必再帶一張對照表。牆值 1 出 0–11、牆值 2 出 16–27（門的變體）。
    """
    os.makedirs(outdir, exist_ok=True)
    meta = {"source": "Mega Drive (1991)", "view": [VIEW_W, VIEW_H],
            "clear": 0, "areas": []}
    import numpy as np

    for ai, t in enumerate(tables):
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
        # 背景：組合緩衝區被 `sub_29F56` 清成 0，牆以外的區域靠背景層填。
        # 那一層還沒解，先出一張純索引 0 佔位。
        save(np.zeros((VIEW_H, VIEW_W), np.uint8), pal,
             os.path.join(sub, "bg.png"), 1)
        meta["areas"].append({"area": ai, "dir": f"area{ai}",
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
    ap.add_argument("--cram", help="BlastEm 傾印的 CRAM（128 bytes），沒有就用灰階")
    ap.add_argument("--pal-line", type=int, default=0, help="用 CRAM 的第幾條調色盤線")
    ap.add_argument("--scale", type=int, default=2)
    args = ap.parse_args()

    rom = open(args.rom, "rb").read()
    tables = area_tables(rom)
    pal = GRAY
    if args.cram:
        pal = read_palette(args.cram)[args.pal_line]

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
            for name, walls in cases.items():
                save(compose(rom, t, walls, pal),
                     pal, os.path.join(args.compose, f"a{ai}_{name}.png"),
                     args.scale)
        print(f"組合圖 → {args.compose}")

    if args.export:
        export(rom, tables, pal, args.export)


if __name__ == "__main__":
    main()
