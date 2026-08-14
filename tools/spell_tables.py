#!/usr/bin/env python3
"""解 2CAST1／2CAST2 的法術跳表，輸出「法術 → overlay → handler」對照。

    tools/spell_tables.py            列出 96 條
    tools/spell_tables.py --md       輸出 markdown 表格（貼進 09-spells.md）
    tools/spell_tables.py --check    只印統計，異常時 exit 1

**表的幾何是結構條件定出來的，不是猜的**（見 docs/formats/09-spells.md §4）：

- 每一筆不是預設出口，就是指向一個 6-byte trampoline；trampoline 連續排列。
- 表尾正好接上預設出口的位址。
- 兩張表合起來必須把 96 條法術**不重不漏**地分完。

三個條件同時成立的解只有一組，而且第三個條件會把「差兩筆」的錯位抓出來 ——
`2CAST2` 的表**以「法術編號 − 2」索引**，光看結構看不出這件事。
"""

import argparse
import json
import pathlib
import struct
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
IMG = ROOT / "workplace/ida"

# (映像, 表的映像內偏移, 筆數, 預設出口, 索引基底)
TABLES = [
    ("2CAST1", 0xD1CE, 96, 0xD28E, 0),
    ("2CAST2", 0xD084, 92, 0xD13C, 2),
]


def handlers(img: str, off: int, count: int, default: int, base: int) -> dict:
    """回傳 {法術編號: handler 的 IDA 位址}。"""
    d = (IMG / f"{img}.img").read_bytes()
    out = {}
    for i, v in enumerate(struct.unpack_from("<%dH" % count, d, off)):
        if v == default:
            continue
        # trampoline：`E8 rel16`（call handler）＋ `E9 rel16`（jmp 共同出口）
        rel = struct.unpack_from("<h", d, v + 1)[0]
        out[i + base] = 0x10000 + v + 3 + rel
    return out


def names() -> list:
    """SPELLS.DAT 的順序：0–47 巫師、48–95 牧師。

    `data/spells.json` 是反過來排的（前 48 牧師），換算與 `cmd/mm2data` 同一條。
    """
    src = json.loads((ROOT / "data/spells.json").read_text(encoding="utf-8"))
    n = len(src)
    return [src[(i + 48) % n] for i in range(n)]


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--md", action="store_true")
    ap.add_argument("--check", action="store_true")
    args = ap.parse_args()

    maps = {img: handlers(img, off, cnt, dflt, base)
            for img, off, cnt, dflt, base in TABLES}
    a, b = maps["2CAST1"], maps["2CAST2"]
    overlap = sorted(set(a) & set(b))
    missing = [i for i in range(96) if i not in a and i not in b]

    print("2CAST1 %d 條、2CAST2 %d 條、重疊 %d、沒有實作 %d"
          % (len(a), len(b), len(overlap), len(missing)), file=sys.stderr)
    if overlap or missing:
        print("  重疊：%s\n  沒有實作：%s" % (overlap, missing), file=sys.stderr)
        if args.check:
            sys.exit(1)
    if args.check:
        return

    ns = names()
    if args.md:
        print("| 編號 | 法術 | overlay | handler |")
        print("|---:|---|---|---|")
    for i in range(96):
        img = "2CAST1" if i in a else "2CAST2" if i in b else "—"
        ea = a.get(i, b.get(i))
        h = "`sub_%05X`" % ea if ea else "—"
        nm = ns[i]["Name"] if i < len(ns) else "?"
        if args.md:
            print("| %d | %s | %s | %s |" % (i, nm, img, h))
        else:
            print("%2d  %-10s %-7s %s" % (i, nm, img, h))


main()
