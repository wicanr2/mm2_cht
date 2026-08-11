#!/usr/bin/env python3
"""從 MSX 版的程式區塊反組譯文字裡，抽出所有第一人稱貼圖的參數。

    tools/msxblits.py workplace/ida/msx_f002.asm

## 為什麼參數讀得出來

MSX2 的 VDP 自己會做「VRAM → VRAM 搬矩形」，所以畫面不是用 CPU 疊圖，
是**一連串 VDP 命令**。常駐引擎在 `0x685D` 開了一個入口（`loc_C837`），
把 15 bytes 的命令區塊（`0xC86F`）填好再一次 `otir` 進暫存器 32–46：

    R32/33 SX ← BC          R38/39 DY ← DE
    R34/35 SY ← 堆疊 +0     R40/41 NX ← 堆疊 +2
    R36/37 DX ← HL          R42/43 NY ← 堆疊 +4
                            R46    CMD = 98h（LMMM，邏輯運算 8 ＝ 透空）

`0x685A` 是同一組但 CMD = 90h（不透空），`0x6857` 是 D0h（HMMM，位元組單位）。
呼叫端是 C 風格：引數由右往左 push（NY、NX、SY），呼叫後 `pop bc` 三次。

所以一個呼叫點就是一次「從素材表的 (SX,SY) 取 NX×NY 貼到 (DX,DY)」，
而且**這些值多半是立即值** —— 牆面的透視是編譯期就算好的常數，不是執行時算的。

## 素材怎麼進 VRAM

`0x6854`（`loc_C47C`）：HL=DX、DE=DY、BC=**檔案 id**，把檔案串流成 HMMC
命令直接寫進 VRAM。檔案前 4 bytes 就是 NX／NY（見 `tools/msxdsk.py`），
所以「載入」與「解壓」是同一件事，中間不落地。

程式區塊載入位址是 **0x0100**（判準：區塊開頭是 C 的堆疊序言、
`call`／`jp` 的目標與對常駐區變數 `0x6894` 的存取都要對得上）。
"""
import collections
import re
import sys

NUM = re.compile(r",\s*([0-9A-Fa-f]+h|\d+)\s*(;.*)?$")


def imm(s: str):
    """取出一行組語裡的立即值，沒有就回 None。"""
    m = NUM.search(s)
    if not m:
        return None
    v = m.group(1)
    return int(v[:-1], 16) if v[-1] in "hH" else int(v)


def blits(path: str, entry="685Dh"):
    """回傳 [{line, dx, dy, sx, sy, nx, ny}]，讀不到立即值的欄位是 None。

    引數是往回掃最近的 `ld hl/de/bc, imm` 與**最後三個 push**。
    往回掃會掃到不相干的指令，所以每個欄位都可能是 None（值是算出來的，
    例如深度或方向當索引查表）—— **None 是資料不是失敗**，
    照樣列出來才看得出哪些位置是動態的。
    """
    L = [x.strip() for x in open(path, encoding="utf-8", errors="replace").read().splitlines()]
    out = []
    for i, l in enumerate(L):
        if "call    " + entry not in l:
            continue
        w = L[max(0, i - 45):i]
        dx = dy = sx = None
        for x in reversed(w):
            if dx is None and x.startswith("ld      hl,"):
                dx = imm(x)
            if dy is None and x.startswith("ld      de,"):
                dy = imm(x)
            if sx is None and x.startswith("ld      bc,"):
                sx = imm(x)
            if dx is not None and dy is not None and sx is not None:
                break
        pu = []
        for k, x in enumerate(w):
            if x.startswith("push"):
                p = w[k - 1] if k else ""
                pu.append(imm(p) if p.startswith("ld      hl,") else None)
        # 補在**前面**再取後三個。補在後面的話取到的永遠是補的 None，
        # 而且看起來像「這個呼叫點的引數都是算出來的」，完全合理。
        ny, nx, sy = ([None] * 3 + pu)[-3:]
        out.append(dict(line=i + 1, dx=dx, dy=dy, sx=sx, sy=sy, nx=nx, ny=ny))
    return out


def main() -> None:
    if len(sys.argv) < 2:
        raise SystemExit(__doc__)
    for path in sys.argv[1:]:
        r = blits(path)
        ok = [b for b in r if b["nx"] and b["ny"]]
        print(f"{path}：{len(r)} 處貼圖，{len(ok)} 處尺寸是立即值")
        c = collections.Counter((b["nx"], b["ny"]) for b in ok)
        print("   尺寸分佈：" + "  ".join(f"{x}×{y}×{n}" for (x, y), n in c.most_common(8)))
        f = lambda v: "?" if v is None else str(v)
        for b in sorted(ok, key=lambda b: -(b["nx"] * b["ny"]))[:60]:
            if b["nx"] * b["ny"] < 400:
                break
            print(f"   行{b['line']:<6} ({f(b['sx']):>3},{f(b['sy']):>4}) "
                  f"{b['nx']:>3}×{b['ny']:<3} → ({f(b['dx']):>3},{f(b['dy']):>4})")


if __name__ == "__main__":
    main()
