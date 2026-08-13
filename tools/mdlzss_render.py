#!/usr/bin/env python3
"""把 tools/mdlzss_scan.py 掃出來的 LZSS 區塊 render 成 PNG。

用途是「畫出來看」——334 個區塊裡哪一批是牆、哪一批是怪物、哪一批根本不是圖，
用眼睛一次就分得出來，比逐個追程式碼快。這是**排序候選**用的，
下結論仍然要回到程式碼（誰載入它、載到哪裡）。

調色盤：新掃到的區塊多半前面沒有調色盤（那正是舊掃描漏掉它們的原因）。
沒有調色盤時用一組固定的 16 階灰階 —— **這不是原版的顏色**，
只是為了看得出形狀。有調色盤的就用它自己的。

    mdlzss_render.py <ROM> <輸出目錄> [--min-tiles 8]
"""

from __future__ import annotations

import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mdgfx import draw_tiles, is_palette, palette  # noqa: E402
from mdlzss_scan import scan  # noqa: E402

# 沒有調色盤時用的灰階。第 0 階給洋紅，才看得出哪些是透空／未使用。
GRAY = [(255, 0, 255)] + [(i * 17, i * 17, i * 17) for i in range(1, 16)]


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("out")
    ap.add_argument("--min-tiles", type=int, default=8,
                    help="少於這個 tile 數的區塊不畫（多半是雜訊）")
    args = ap.parse_args()

    rom = open(args.rom, "rb").read()
    found = scan(rom)

    kept, end = [], -1
    for x, comp, raw, data in found:
        if x < end:
            continue
        kept.append((x, comp, raw, data))
        end = x + 8 + comp

    os.makedirs(args.out, exist_ok=True)
    drawn = skipped = 0
    for x, comp, raw, data in kept:
        tiles = int.from_bytes(data[:2], "big")
        # 只畫 tile 形狀成立的：宣告的 tile 數要與輸出長度算出來的相同。
        if (raw - 2) % 32 or (raw - 2) // 32 != tiles or tiles < args.min_tiles:
            skipped += 1
            continue
        pal = palette(rom, x - 32) if x >= 32 and is_palette(rom, x - 32) else GRAY
        im = draw_tiles(data[2:], 0, tiles, pal, cols=32, scale=2)
        im.save(os.path.join(args.out, f"md-{x:06X}-{tiles}t.png"))
        drawn += 1

    print(f"畫了 {drawn} 個區塊，跳過 {skipped} 個（tile 形狀不成立或太小）")
    print(f"輸出在 {args.out}")


if __name__ == "__main__":
    main()
