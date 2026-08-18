#!/usr/bin/env python3
"""從 MSX 版的反組譯抽出**戶外**第一人稱的貼圖幾何。

    tools/msxout.py workplace/ida/msx_f004.asm

## 與 `msxblits.py` 的分工

`msxblits.py` 掃的是「呼叫點附近最近的立即值」，對室內那條路徑夠用 ——
室內每一塊的落點都是常數。**戶外不是**：同一個深度有一整排格子，
目的 x 是 `k·v + base` 當場算出來的，`v` 是那一格相對隊伍的橫向偏移。
往回掃立即值會抓到堆疊上的 `push`（那是 SY），得到「落點 341」這種
落在視圖外的值 —— **看起來像資料壞了，其實是掃法不對**。

所以這一支不掃立即值，改**符號執行**呼叫前那一小段：把 `hl`／`de`／`bc`
當成 `(v 的係數, 常數)` 兩元組跑一遍 `add hl,hl` 那串位移加法，
`k` 與 `base` 就直接讀出來了。

## 呼叫慣例

`0x685D`（透空）與 `0x6857`（不透空）：

    BC = SX      DE = DY      HL = DX
    堆疊 +0 = SY  +2 = NX  +4 = NY      （右到左 push，所以最後 push 的是 SY）

## 每支函式的形狀

    sub_XXXX(HL = 深度, DE = v)
        depth 3 / 2 / 1 / 0 各一段
        每段：兩個「邊緣格」特例（v 等於某個值，落點是常數）
              ＋ 一般格（落點 = k·v + base）
"""
import re
import sys

CALL = re.compile(r"call\s+(685Dh|6857h|685Ah)")
IMM = re.compile(r"ld\s+(hl|de|bc),\s*([0-9A-Fa-f]+h|\d+)")


def num(s: str) -> int:
    s = s.strip()
    return int(s[:-1], 16) if s.endswith("h") else int(s)


def s16(v: int) -> int:
    return v - 0x10000 if v >= 0x8000 else v


class Sym:
    """(k, b) 代表 k·v + b。k 是 None 表示這個值與 v 無關。"""

    def __init__(self, k=0, b=0):
        self.k, self.b = k, b

    def __add__(self, o):
        return Sym(self.k + o.k, self.b + o.b)

    def __str__(self):
        if self.k == 0:
            return str(s16(self.b & 0xFFFF))
        return f"{self.k}v{self.b:+d}"


def run(lines):
    """符號執行一段指令，回傳呼叫當下的 (hl, de, bc) 與堆疊。"""
    r = {"hl": Sym(1, 0), "de": Sym(1, 0), "bc": Sym(0, 0)}
    stack = []
    for x in lines:
        x = x.strip()
        m = IMM.match(x)
        if m:
            r[m.group(1)] = Sym(0, num(m.group(2)))
        elif x == "push    hl":
            stack.append(r["hl"])
        elif x == "ex      de, hl":
            r["hl"], r["de"] = r["de"], r["hl"]
        elif x == "add     hl, hl":
            r["hl"] = r["hl"] + r["hl"]
        elif x == "add     hl, de":
            r["hl"] = r["hl"] + r["de"]
        elif x == "add     hl, bc":
            r["hl"] = r["hl"] + r["bc"]
        elif x in ("ld      e, l", "ld      d, h"):
            r["de"] = Sym(r["hl"].k, r["hl"].b)
        elif x in ("ld      c, l", "ld      b, h"):
            r["bc"] = Sym(r["hl"].k, r["hl"].b)
    return r, stack


def blocks(path, funcs):
    """把每支函式切成「分派條件 → 一段程式」。"""
    L = [x.rstrip() for x in open(path, encoding="utf-8", errors="replace")]
    # 標籤位置
    at = {}
    for i, x in enumerate(L):
        m = re.match(r"(sub_[0-9A-F]+|loc_[0-9A-F]+):", x.strip())
        if m:
            at.setdefault(m.group(1), i)

    out = {}
    for fn in funcs:
        i = at.get(fn)
        if i is None:
            continue
        # 深度分派：`ld a,l / sub N / or h / jp z, loc_X`
        depth, pend = {}, 0
        j = i
        while j < len(L):
            x = L[j].strip()
            if x == "ld      a, l":
                pend = 0
            elif x.startswith("sub     ") and x[8:].strip().isdigit():
                pend = int(x[8:].strip())
            elif x == "dec     a":
                pend = 1
            elif x.startswith("jp      z, loc_"):
                depth[x.split("loc_")[1]] = pend
                pend = 0
            elif x == "ret":
                break
            j += 1
        # 函式的結尾：下一個 `sub_` 標籤。不夾住的話最後一個深度會
        # 一路吃進下一支函式，而多出來的那些看起來像「這個深度有很多塊」。
        stop = len(L)
        for name, k in at.items():
            if name.startswith("sub_") and k > i and k < stop:
                stop = k
        out[fn] = [(stop, None)]
        for lab, d in depth.items():
            k = at.get("loc_" + lab)
            if k is None or k >= stop:
                continue
            out[fn].append((d, k))
    return L, at, out


# 分派條件：`ld a,e / <調整> / and d 或 or d / inc a / jp nz` ——
# 成立的那個 v 由中間那道調整決定。四種形狀對應 v = ±imm 與 ±1。
def condValue(seg):
    """從一段前置比較裡讀出「這個特例對應哪個 v」。讀不出來回 None。"""
    adj, mode = None, None
    for x in seg:
        if x == "ld      a, e":
            adj, mode = 0, None
        elif re.match(r"sub\s+([0-9A-Fa-f]+h|\d+)$", x):
            adj = num(x.split()[1])
        elif x == "inc     a":
            if mode is None and adj is not None:
                adj = -1 if adj == 0 else adj
        elif x == "dec     a":
            adj = 1
        elif x.startswith("and     d"):
            mode = "neg"   # 高位元組要是 0xFF → v 是負的
        elif x.startswith("or      d"):
            mode = "pos"   # 高位元組要是 0 → v 是正的
    if adj is None or mode is None:
        return None
    # `and d` 那一支：a = e - adj，要 a & d == 0xFF，也就是 e = adj - 1、d = 0xFF
    #   → v = (adj - 1) - 256 ＝ adj - 257 …… 化簡後就是 -(257 - adj) 的低位
    # 直接用「e 的值」回推比較不會錯：
    if mode == "neg":
        e = (adj - 1) & 0xFF
        return s16(0xFF00 | e)
    return adj


def cases(L, start, end):
    """切出一個深度裡的每一段：特例（帶條件）與一般格（落點是 v 的一次式）。"""
    got, cond, seg = [], None, []
    for i in range(start, min(end, len(L))):
        x = L[i].strip()
        if x.startswith("jp      nz, loc_"):
            cond = condValue(seg)
            seg = []
        elif CALL.search(x):
            r, st = run(seg)
            ny, nx, sy = ([None] * 3 + st)[-3:]
            got.append(dict(cond=cond, sx=r["bc"], dy=r["de"], dx=r["hl"],
                            sy=sy, nx=nx, ny=ny, opaque="6857h" in x))
            cond, seg = None, []
        elif x == "ret":
            seg = []
        else:
            seg.append(x)
    return got


def main():
    path = sys.argv[1] if len(sys.argv) > 1 else "workplace/ida/msx_f004.asm"
    funcs = sys.argv[2:] or ["sub_1103", "sub_1A40", "sub_1C2B"]
    L, at, disp = blocks(path, funcs)
    for fn in funcs:
        print(f"=== {fn} ===")
        raw = disp.get(fn, [])
        stop = raw[0][0] if raw else 0
        ent = sorted([e for e in raw[1:]], key=lambda t: t[1])
        for n, (d, k) in enumerate(ent):
            end = ent[n + 1][1] if n + 1 < len(ent) else stop
            print(f"  深度 {d}")
            for c in cases(L, k, end):
                who = "一般" if c["cond"] is None else f"v={c['cond']}"
                print(f"    {who:6s} src({c['sx']},{c['sy']}) "
                      f"{c['nx']}×{c['ny']} → dst({c['dx']},{c['dy']})"
                      + ("  不透空" if c["opaque"] else ""))


if __name__ == "__main__":
    main()
