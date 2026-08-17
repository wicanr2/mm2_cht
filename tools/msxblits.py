#!/usr/bin/env python3
"""從 MSX 版的程式區塊反組譯文字裡，抽出所有第一人稱貼圖的參數。

    tools/msxblits.py workplace/ida/msx_f002.asm          參數表
    tools/msxblits.py --vram workplace/ida/msx_f004.asm   來源對回檔案

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

**兩個入口都要掃。** 只掃 `0x685D`（透空）會漏掉整條正牆 —— 室內的正牆是
`sub_1E16` 用 `0x6857`（不透空）畫的一整條岩帶，只掃透空那個的話它在結果裡
**一筆都不會出現**，而剩下的透空貼圖照樣拼得出一幅「看起來有東西」的畫面。
踩過一次：remake 的 MSX 牆表因此抄成了戶外那條路徑的座標。

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
import os
import re
import sys

NUM = re.compile(r",\s*([0-9A-Fa-f]+h|\d+)\s*(;.*)?$")
# loc_XXXX → 深度。分派表是在同一遍掃描裡建的，而分派一定出現在對應的
# loc_ 之前（那是 `jp z` 的目標），所以一遍就夠。
LABELS = {}


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
    # 目前所在的函式。分組看比逐行看有用得多：一個函式就是一支繪圖常式，
    # 86 個匿名的呼叫點分完組之後只剩十來群。
    func = [""] * len(L)
    cur = "?"
    for i, x in enumerate(L):
        if x.startswith("sub_") and ":" in x:
            cur = x.split(":")[0]
        func[i] = cur

    # 深度歸屬。每支繪圖常式開頭都是同一個形狀的分派：
    #
    #     ld a,l : sub 3  : or h : jp z, loc_A     ← 深度 3
    #     ld a,l : sub 2  : or h : jp z, loc_B     ← 深度 2
    #     ld a,l : dec a  : or h : jp z, loc_C     ← 深度 1
    #     ld a,l :          or h : jp z, loc_D     ← 深度 0
    #
    # 所以深度**讀得出來，不必從貼圖高度去猜**。（高度遞減確實與深度相關，
    # 但那是相關不是證據 —— 同一個深度也會有好幾種寬度，見下面的裁切。）
    depth = [None] * len(L)
    pend, cur_d = 0, None
    for i, x in enumerate(L):
        if x == "ld      a, l":
            pend = 0
        elif x.startswith("sub     ") and x[8:].strip().isdigit():
            pend = int(x[8:].strip())
        elif x == "dec     a":
            pend = 1
        elif x.startswith("jp      z, loc_"):
            LABELS[x.split("loc_")[1]] = pend
            pend = 0
        if x.startswith("loc_") and ":" in x:
            lab = x.split(":")[0][4:]
            if lab in LABELS:
                cur_d = LABELS[lab]
            # 不在分派表裡的 loc_ 是同一段深度裡的分支（多半是左右、
            # 或有沒有被前面的牆擋住），深度不變 —— 歸成 None 的話
            # 右半邊那一批會整批掉出去，而畫面上只是「右邊沒東西」。
        elif x.startswith("sub_") and ":" in x:
            cur_d = None
        depth[i] = cur_d
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
        out.append(dict(line=i + 1, func=func[i], depth=depth[i],
                        dx=dx, dy=dy, sx=sx, sy=sy, nx=nx, ny=ny))
    return out



def loads(path: str, entry="6854h"):
    """回傳 [{line, dx, dy, id}]：`0x6854` 把某個檔案串流到 VRAM 的哪裡。

    這些呼叫決定了 VRAM 的版面，而版面決定了貼圖的 `(SX,SY)` 指到哪個檔案。
    **不要假設「素材表只有一張」** —— 戶外那一組是好幾個檔案鑲嵌在同一個
    y 帶上（`0x2044` 佔 x 308–503、`0x2042` 佔 308–489），室內那一組是
    一張 462 寬的大表蓋在同一片座標上。同一份貼圖表在兩種模式下讀到不同
    的檔案，這是原版刻意的：**程式碼共用，素材換掉**。
    """
    L = [x.strip() for x in open(path, encoding="utf-8", errors="replace").read().splitlines()]
    out = []
    for i, line in enumerate(L):
        if not line.startswith("call") or entry not in line:
            continue
        reg = {}
        for j in range(max(0, i - 25), i):
            m = re.match(r"ld\s+(hl|de|bc),\s*([0-9A-Fa-f]+h|\d+)$", L[j])
            if m:
                reg[m.group(1)] = imm("ld      x, " + m.group(2))
        out.append(dict(line=i + 1, dx=reg.get("hl"), dy=reg.get("de"), id=reg.get("bc")))
    return out


def sizes(gfx="workplace/gfx/msx"):
    """從抽出來的 PNG 檔名讀每個 id 的寬高（`d1_2044_196x62.png`）。"""
    out = {}
    if not os.path.isdir(gfx):
        return out
    for n in os.listdir(gfx):
        m = re.match(r"d\d_([0-9A-Fa-f]{4})_(\d+)x(\d+)\.png$", n)
        if m:
            out[int(m.group(1), 16)] = (int(m.group(2)), int(m.group(3)))
    return out


def cover(path: str, gfx="workplace/gfx/msx") -> None:
    """把每一筆貼圖的來源矩形對回「哪個檔案的哪一塊」。

    `--vram` 這個模式存在的理由：來源座標是 **VRAM 絕對座標**，不是某張表的
    檔內座標。拿單一一張表去減，超出那張表的就會被判成「來源不存在」——
    而那與「真的沒有素材」長得一模一樣。逐筆對回載入表才分得出來。
    """
    ld = loads(path)
    sz = sizes(gfx)
    all_blits = blits(path) + blits(path, entry="6857h")
    if not ld:
        # 這一份沒有載入呼叫（`f004` 一次都沒有），拿同目錄的 `f002` 當版面。
        alt = path.replace("f004", "f002")
        if alt != path and os.path.exists(alt):
            ld = loads(alt)
            print(f"（{path} 沒有載入呼叫，版面取自 {alt}）")
    print("VRAM 版面：")
    for b in ld:
        w, h = sz.get(b["id"], (None, None))
        if b["dx"] is None or w is None:
            print(f"   行{b['line']:<6} id={b['id'] and hex(b['id'])} → ({b['dx']},{b['dy']}) 尺寸未知")
            continue
        print(f"   id {b['id']:04x} {w:>3}×{h:<3} → x {b['dx']:>3}–{b['dx']+w-1:<3} y {b['dy']}–{b['dy']+h-1}")
    boxes = [(b["dx"], b["dy"], *sz[b["id"]], b["id"])
             for b in ld if b["dx"] is not None and b["id"] in sz]
    # 兩組素材各自能蓋住幾筆。**這一行就是判準**：室內只載 `0x202x` 那張
    # 462 寬的表，戶外載一整片鑲嵌（`0x2044` 到 x 503、`0x2042` 到 x 489）。
    # 同一筆貼圖在室內可能超出表外、在戶外剛好落在檔案裡。
    indoor = {0x2020, 0x2021, 0x2022, 0x2023, 0x2010}
    print("\n每支函式的來源能不能被兩組素材蓋住：")
    tally = {}
    for b in all_blits:
        if None in (b["sx"], b["sy"], b["nx"], b["ny"]):
            continue
        hit = {i for (x, y, w, h, i) in boxes
               if x <= b["sx"] and b["sx"] + b["nx"] <= x + w
               and y <= b["sy"] and b["sy"] + b["ny"] <= y + h}
        t = tally.setdefault(b["func"], [0, 0, 0])
        t[0] += 1
        t[1] += bool(hit & indoor)
        t[2] += bool(hit - indoor)
    for fn in sorted(tally):
        n, i, o = tally[fn]
        print(f"   {fn:10s} 共 {n:>2} 筆　室內表 {i:>2}／{n:<2}　戶外鑲嵌 {o:>2}／{n}")

def main() -> None:
    if len(sys.argv) < 2:
        raise SystemExit(__doc__)
    if sys.argv[1] == "--vram":
        for path in sys.argv[2:]:
            cover(path)
        return
    for path in sys.argv[1:]:
        r = blits(path) + blits(path, entry="6857h")
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
