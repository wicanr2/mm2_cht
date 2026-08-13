#!/usr/bin/env python3
"""在 Mega Drive ROM 裡掃出**所有** LZSS 區塊，不靠調色盤當入口。

先前 `tools/mdgfx.py` 用「調色盤 → 後面接 16 bytes 區塊頭」找圖形，
掃出 62 個。但 `sub_42FC` 載入的兩組貼圖（`0x06E9A2`、`0x06F520`）
**前面沒有調色盤**，於是整批漏掉 —— `tools/mdgfx.py` 的 `blocks()` 已經不要求調色盤，但它要求**16 bytes 的區塊頭
＋ `F0FF` magic**；那兩組貼圖連區塊頭都沒有，只有裸的 compSize/rawSize/位元流，
所以照樣掃不到。

改用解壓常式自己的呼叫慣例當結構條件，不需要任何前綴：

	u32 [X+0]  compSize   壓縮後長度
	u32 [X+4]  rawSize    解壓後長度
	    [X+8]  位元流

驗收條件是**解出來的長度精確等於宣告的 rawSize** —— LZSS 這種解碼器餵什麼
都吐得出東西，所以「輸出看起來合理」不能當證據，要有宣告長度這種獨立條件。

    mdlzss_scan.py <ROM> [--dump 目錄]
"""

from __future__ import annotations

import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mdgfx import lzss  # noqa: E402  解壓常式逐行抄自 ROM 0x29954


def scan(rom: bytes, min_raw: int = 0x100, max_raw: int = 0x8000):
    out = []
    n = len(rom)
    for x in range(0, n - 8, 2):
        comp = int.from_bytes(rom[x : x + 4], "big")
        raw = int.from_bytes(rom[x + 4 : x + 8], "big")
        if not (0x20 < comp < 0x20000):
            continue
        if not (min_raw <= raw <= max_raw):
            continue
        if x + 8 + comp > n:
            continue
        # 壓縮率的健全性：LZSS 不會把資料放大太多，也不會小到離譜
        if not (raw // 8 <= comp <= raw * 2):
            continue
        try:
            # lzss() 回傳 (輸出, 吃掉的位元組數)，不是單純的 bytes。
            data, used = lzss(rom, x + 8, raw)
        except Exception:
            continue
        if len(data) != raw:
            continue
        # 吃掉的位元組數要與宣告的 compSize 相符（容一點尾端對齊）。
        # 這是第二個獨立條件，光看輸出長度會有假陽性。
        if not (comp - 4 <= used <= comp + 4):
            continue
        out.append((x, comp, raw, data))
    return out


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--dump", help="把解出來的 tile 存成 .bin 的目錄")
    args = ap.parse_args()

    rom = open(args.rom, "rb").read()
    found = scan(rom)

    # 重疊的候選只留第一個：同一段位元流從不同起點也可能湊出合法解。
    kept, end = [], -1
    for x, comp, raw, data in found:
        if x < end:
            continue
        kept.append((x, comp, raw, data))
        end = x + 8 + comp

    print(f"合法 LZSS 區塊：{len(kept)} 個（候選 {len(found)} 個，去重疊後）")
    print(f"{'位址':>9} {'壓縮':>8} {'解壓':>8} {'tile 數':>8}  開頭")
    for x, comp, raw, data in kept:
        tiles = int.from_bytes(data[:2], "big")
        ok = "✓" if (raw - 2) // 32 == tiles else " "
        print(f"0x{x:07X} {comp:>8} {raw:>8} {tiles:>8}{ok}  "
              + " ".join(f"{b:02X}" for b in data[:8]))
        if args.dump:
            os.makedirs(args.dump, exist_ok=True)
            with open(os.path.join(args.dump, f"lzss-{x:06X}.bin"), "wb") as fh:
                fh.write(data)

    print()
    print("tile 數欄的 ✓ 代表 (rawSize−2)/32 等於解出來的第一個字 —— "
          "兩個獨立欄位互相印證，才算是 tile 資料而不是別的東西。")


if __name__ == "__main__":
    main()
