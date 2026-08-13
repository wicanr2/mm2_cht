#!/usr/bin/env python3
"""解 Mega Drive 版第一人稱視角的貼圖，並依區域類型分組畫出來。

這些**不是 tile**，是 blitter 用的點陣圖 —— 所以拿 `tools/mdgfx.py` 或
`tools/mdlzss_render.py` 當 8×8 tile 畫會像雜訊。格式是讀 blitter
（thunk `0x66` → ROM `0x29A22`，「LZSS 解壓 ＋ 逐列 blit」合一）得到的：

	src-4  u16  寬（每列幾個 byte，4bpp 所以像素寬是 ×2）
	src-2  u16  高
	src+0  u32  壓縮後長度
	src+4  u32  解壓後長度   ← 必須等於 寬 × 高
	src+8       LZSS 位元流

`寬 × 高 == rawSize` 是驗收條件：兩個獨立欄位互相印證，
不是「長度剛好對」那種弱證據。blitter 把解出來的資料逐列寫進組合緩衝區，
列距固定 `0x68`（104 bytes）—— 那是緩衝區的寬度，不是圖自己的寬度。

來源清單從 `sub_FC38` 抽出來：它是 7 個 case 的跳表 switch，
依區域類型（`sub_FB86(地圖編號)` 算出）把一組 ROM 指標填進 RAM `$FFCD40`。

    mdview.py <ROM> <輸出目錄>
"""

from __future__ import annotations

import argparse
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mdgfx import lzss  # noqa: E402

FC38 = 0x00FC38  # sub_FC38：依區域類型建視野貼圖表
FC38_END = 0x010226  # 跳表的 default 分支，掃到這裡為止
TABLE_DISP_LO = 0x35D2  # 表的第一格是 -$35D2(a5)
TABLE_DISP_HI = 0x3500  # 表大約到 -$3500(a5)

# 沒有調色盤時的灰階。第 0 階給洋紅，看得出哪些是透空。
GRAY = [(255, 0, 255)] + [(i * 17, i * 17, i * 17) for i in range(1, 16)]


def sources(rom: bytes) -> list[int]:
    """從 sub_FC38 抽出所有填進表裡的 ROM 指標。

    慣用句是 `move.l #imm,(sp)`（`2E BC` ＋ 32-bit）接
    `move.l (sp)+,-$35xx(a5)`（`2B 5F` ＋ 位移）。
    """
    out = []
    pat = bytes([0x2E, 0xBC])
    for m in re.finditer(re.escape(pat), rom[FC38:FC38_END]):
        p = FC38 + m.start()
        if rom[p + 6 : p + 8] != bytes([0x2B, 0x5F]):
            continue
        disp = int.from_bytes(rom[p + 8 : p + 10], "big", signed=True)
        if not (-TABLE_DISP_LO <= disp <= -TABLE_DISP_HI):
            continue
        out.append(int.from_bytes(rom[p + 2 : p + 6], "big"))
    return out


def decode(rom: bytes, src: int):
    """回傳 (寬像素, 高, 像素資料)；不符合格式就回 None。"""
    if src < 8 or src + 8 > len(rom):
        return None
    w = int.from_bytes(rom[src - 4 : src - 2], "big")
    h = int.from_bytes(rom[src - 2 : src], "big")
    raw = int.from_bytes(rom[src + 4 : src + 8], "big")
    if not w or not h or w * h != raw:
        return None
    data, _used = lzss(rom, src + 8, raw)
    if len(data) != raw:
        return None
    return w * 2, h, data


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("out")
    ap.add_argument("--scale", type=int, default=2)
    args = ap.parse_args()

    from PIL import Image

    rom = open(args.rom, "rb").read()
    srcs = sources(rom)
    os.makedirs(args.out, exist_ok=True)

    print(f"sub_FC38 填進表裡的來源：{len(srcs)} 個（相異 {len(set(srcs))} 個）")
    print(f"{'來源':>10} {'寬':>5} {'高':>5} {'解壓':>7}  結果")

    ok = bad = 0
    for src in sorted(set(srcs)):
        r = decode(rom, src)
        if not r:
            print(f"0x{src:07X}     —     —       —  不符合視野貼圖格式")
            bad += 1
            continue
        w, h, data = r
        im = Image.new("RGB", (w, h), (255, 0, 255))
        for y in range(h):
            row = y * (w // 2)
            for x in range(0, w, 2):
                v = data[row + x // 2]
                im.putpixel((x, y), GRAY[v >> 4])
                im.putpixel((x + 1, y), GRAY[v & 15])
        if args.scale > 1:
            im = im.resize((w * args.scale, h * args.scale), Image.NEAREST)
        im.save(os.path.join(args.out, f"view-{src:06X}-{w}x{h}.png"))
        print(f"0x{src:07X} {w:>5} {h:>5} {w // 2 * h:>7}  ✓")
        ok += 1

    print(f"\n畫出 {ok} 張，{bad} 個不符合格式。輸出在 {args.out}")


if __name__ == "__main__":
    main()
