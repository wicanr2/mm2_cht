#!/usr/bin/env python3
"""對原版資料檔做一次結構探測：找記錄長度候選、熵、可見字串比例。

檔案大小能整除只證明某種排版可能成立，不能當結論——這裡只產生候選，
每個候選都要另外用實際內容驗證（render 出來、對照遊戲內數值）。

用法：tools/probe_dat.py <原版 MM2 目錄> [檔名...]
"""
import math
import os
import re
import sys
from collections import Counter


def entropy(b: bytes) -> float:
    if not b:
        return 0.0
    c = Counter(b)
    n = len(b)
    return -sum(v / n * math.log2(v / n) for v in c.values())


def divisors(n, lo=2, hi=512):
    return [d for d in range(lo, min(hi, n) + 1) if n % d == 0]


def stride_score(b: bytes, stride: int) -> float:
    """同一欄位在各記錄間應該比隨機更像：量測「同 offset 的位元組」的平均熵。

    真正的記錄陣列，某些欄位會反覆出現同樣的少數值（型別、旗標、0 填充），
    使得逐欄熵明顯低於整體熵。
    """
    rows = len(b) // stride
    if rows < 4:
        return 99.0
    total = 0.0
    for col in range(stride):
        col_bytes = bytes(b[r * stride + col] for r in range(rows))
        total += entropy(col_bytes)
    return total / stride


def probe(path):
    b = open(path, "rb").read()
    name = os.path.basename(path)
    ent = entropy(b)
    runs = [m.group() for m in re.finditer(rb"[ -~]{4,}", b)]
    ascii_cov = sum(len(r) for r in runs) / len(b) if b else 0
    zeros = b.count(0) / len(b)

    print("=" * 78)
    print("%s  %d bytes   熵 %.2f bit/byte   ASCII %.0f%%   零位元組 %.0f%%"
          % (name, len(b), ent, ascii_cov * 100, zeros * 100))

    if runs:
        sample = [r.decode()[:40] for r in runs[:6]]
        print("  字串樣本: %s" % " | ".join(sample))

    cands = []
    for d in divisors(len(b)):
        rows = len(b) // d
        if not 2 <= rows <= 4096:
            continue
        s = stride_score(b, d)
        if s < ent - 0.3:
            cands.append((ent - s, d, rows, s))
    cands.sort(reverse=True)
    if cands:
        print("  記錄長度候選（逐欄熵明顯低於整體）:")
        for gain, d, rows, s in cands[:6]:
            print("    stride %4d × %5d 筆   逐欄熵 %.2f（低 %.2f）" % (d, rows, s, gain))
    else:
        print("  無明顯固定長度記錄結構")

    print("  前 48 bytes: %s" % b[:48].hex(" ", 16))


def main():
    srcdir = sys.argv[1]
    names = sys.argv[2:] or sorted(f for f in os.listdir(srcdir)
                                   if f.endswith((".DAT", ".CH")))
    for n in names:
        probe(os.path.join(srcdir, n))


if __name__ == "__main__":
    main()
