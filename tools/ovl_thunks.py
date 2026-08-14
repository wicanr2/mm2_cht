#!/usr/bin/env python3
"""解析 overlay thunk 表：thunk 位址 ⇄ (overlay 編號, 目標偏移)。

    tools/ovl_thunks.py                       列出全部 thunk
    tools/ovl_thunks.py 1765A 17666           查這幾個 thunk 指向誰
    tools/ovl_thunks.py --to 0 1C88           查誰指向 root 的 0x1C88

位址一律寫 IDA 位址（`build_ovl_image.py` 把映像放在 base 0x1000，
所以 **IDA 位址 = 0x10000 + 執行時偏移**，而檔案偏移 = IDA 位址 - 0x10000）。

**為什麼需要這支**：overlay 的程式碼從不直接 call root，一律經過 thunk，
所以在 overlay 的反組譯裡 grep root 的函式名**永遠是零命中** ——
而零命中與「真的沒人呼叫」長得一模一樣。跨 overlay 追函式只能走這張表。

thunk 的 12 bytes 版面（見 docs/formats/01 §3.5）：

    9a 44 03 7d 07     call far 077D:0344   ; overlay loader
    NN                 目標的 overlay 編號（0 = root）
    80                 標記
    ea LL LL SS SS     jmp far              ; LLLL = 目標在該 overlay 內的偏移
"""

import argparse
import pathlib
import struct
import sys

IMG_BASE = 0x10000
STUB = bytes.fromhex("9a44037d07")
ROOT = pathlib.Path(__file__).resolve().parent.parent
DEFAULT_IMG = ROOT / "workplace/ida/2MISC.img"

# overlay 編號 → 檔名。順序**照描述表**（docs/formats/01 §2 的實際值表），
# 不是檔名的字母序 —— 按字母序排會得到一份看起來完全合理、每一筆都錯的對照。
OVL_NAMES = [
    "root", "1MENU2", "2COMBAT", "2PLAY", "1MENU1", "1RETINN", "2MISC",
    "2MISC2", "2CAST1", "2CAST2", "2CMDS", "2CAVES", "2BRAIN", "2SMITH",
    "2TEMPLE",
]


def scan(img: bytes) -> list[dict]:
    """掃出全部 thunk。用樣式掃而不是讀描述表：thunk 不保證連續，
    而漏掉的那幾筆長得和「不存在」一樣。"""
    out, pos = [], 0
    while True:
        pos = img.find(STUB, pos)
        if pos < 0:
            return out
        ovl = img[pos + 5]
        mark = img[pos + 6]
        jmp = img[pos + 7]
        if mark == 0x80 and jmp == 0xEA:
            off, seg = struct.unpack_from("<HH", img, pos + 8)
            out.append({
                "thunk": IMG_BASE + pos,
                "ovl": ovl,
                "name": OVL_NAMES[ovl] if ovl < len(OVL_NAMES) else "?%d" % ovl,
                "off": off,
                "seg": seg,
            })
        pos += 1


def fmt(t: dict) -> str:
    # root 的目標偏移就是 root 內偏移，可直接加 IMG_BASE 得 IDA 位址；
    # overlay 的目標偏移要配該 overlay 的載入段才有意義，所以照原樣印。
    tgt = ("root %05X (IDA %06X)" % (t["off"], IMG_BASE + t["off"])
           if t["ovl"] == 0 else "%s +%04X" % (t["name"], t["off"]))
    return "thunk %06X (rt %05X) → ovl %2d %s" % (
        t["thunk"], t["thunk"] - IMG_BASE, t["ovl"], tgt)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("addrs", nargs="*", help="thunk 的 IDA 位址（hex）")
    ap.add_argument("--img", default=str(DEFAULT_IMG))
    ap.add_argument("--to", nargs=2, metavar=("OVL", "OFF"),
                    help="反查：指向 <overlay 編號> 的 <偏移> 的 thunk")
    args = ap.parse_args()

    thunks = scan(pathlib.Path(args.img).read_bytes())
    if not thunks:
        sys.exit("在 %s 裡掃不到 thunk（映像不對？）" % args.img)

    if args.to:
        ovl, off = int(args.to[0], 0), int(args.to[1], 16)
        hits = [t for t in thunks if t["ovl"] == ovl and t["off"] == off]
        print("\n".join(fmt(t) for t in hits) or "沒有 thunk 指向這裡")
        return

    if args.addrs:
        want = {int(a, 16) for a in args.addrs}
        by_addr = {t["thunk"]: t for t in thunks}
        for a in sorted(want):
            print(fmt(by_addr[a]) if a in by_addr else "%06X 不是 thunk" % a)
        return

    print("共 %d 筆 thunk" % len(thunks))
    for t in thunks:
        print(fmt(t))


main()
