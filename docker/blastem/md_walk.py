#!/usr/bin/env python3
"""狀態驅動地把隊伍走完一條路線，途中記錄指定中斷點的命中。

    md_walk.py <rom> --route Up,Up,Left,Up --break 0xB620:選曲 --load

為什麼不能盲打：確認鍵在**有對話框**時是「確認」、**沒有對話框**時是
「開啟指令選單」，沒有一個按鍵在兩種狀態下都安全。盲打的腳本一走進
會跳訊息的地形就發散 —— 實測 30 次 `Up;確認` 的結果是隊伍一步沒走，
最後停在角色改名對話框裡。

判斷狀態用的是**堆疊指標**，不是 RAM 裡的旗標。對話框與選單都是在主迴圈裡
再進一層巢狀迴圈，SP 會明顯更低，而且三種狀態的區間完全不重疊（本片實測）：

    野外可移動   SP FFCA3A–FFCACA
    訊息對話框   SP FFC94C–FFC9D8
    指令選單     SP FFC784–FFC84E

比在 RAM 裡找旗標可靠得多 —— 不必猜哪個位元組有意義（那一帶其實是堆疊，
逐位元組比對出來的「一致」只是呼叫深度剛好相同），而且 `g` 一次就拿得到。

節拍器是 vblank 中斷點：stub 不回應非同步中斷，放行之後就再也停不下來，
所以用「命中 → 讀 → 放行」當單步。代價是模擬速度掉到約 1/6。
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from rsp import Rsp  # noqa: E402

VBLANK_BREAK = 0x06CB02
HOLD_FRAMES = 6          # 手把按住幾個模擬幀（真實秒數在這裡等於零幀）
SETTLE_FRAMES = 24       # 按完等畫面穩定
MAX_ATTEMPTS = 6         # 一步最多試幾次（每次清掉一層 modal）


def parse_break(s: str) -> tuple[int, str]:
    a, _, name = s.partition(":")
    addr = int(a, 16)
    return addr, name or f"0x{addr:06X}"


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--route", default="", help="逗號分隔的按鍵，例如 Up,Up,Left,Up")
    ap.add_argument("--break", dest="breaks", action="append", default=[],
                    metavar="位址[:名稱]")
    ap.add_argument("--ignore-d0", default="",
                    help="d0 是這些值（十六進位、逗號分隔）時不記錄")
    ap.add_argument("--load", action="store_true", help="開頭先送 `l` 載入狀態檔")
    ap.add_argument("--confirm", default="d", help="確認鍵（BlastEm 預設 d = C 鈕）")
    ap.add_argument("--field-sp", default="FFCA00",
                    help="SP 大於等於這個值就當作「可以移動」")
    ap.add_argument("--shot", default="", help="結尾截圖的檔名（存到 /out）")
    ap.add_argument("--log", default="/out/walk.txt")
    args = ap.parse_args()

    field_sp = int(args.field_sp, 16) | 0xFF000000
    ignore_d0 = {int(v, 16) for v in args.ignore_d0.split(",") if v.strip()}
    breaks = dict(parse_break(b) for b in args.breaks)

    proc = subprocess.Popen(
        ["/opt/blastem/blastem", args.rom, "-D"],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, bufsize=0,
    )
    rsp = Rsp(proc)
    if not rsp.status().startswith(b"S"):
        raise SystemExit("stub 沒有停在進入點")

    log = open(args.log, "w", encoding="utf-8")

    def out(s: str) -> None:
        print(s, flush=True)
        log.write(s + "\n")
        log.flush()

    for a in breaks:
        rsp.add_break(a)
    rsp.add_break(VBLANK_BREAK)
    rsp.cont()  # 開機到第一次 vblank，可能十秒以上

    state = {"sp": 0}

    def run_frames(n: int) -> int:
        """推進 n 個 vblank，途中命中其他中斷點就記錄。回傳最後一次的 SP。"""
        done = 0
        while done < n:
            rsp.cont()
            regs = rsp.regs()
            pc = regs[-1]
            if pc == VBLANK_BREAK:
                done += 1
                state["sp"] = regs[15]
                continue
            if (regs[0] & 0xFFFFFFFF) in ignore_d0:
                continue
            sp = regs[15]
            try:
                ret = rsp.read_u32(sp)
            except Exception:
                ret = 0
            out(f"  ★ {breaks.get(pc, f'0x{pc:06X}')}  呼叫者 {ret:06X}  "
                f"d0={regs[0]:08X} d1={regs[1]:08X} d2={regs[2]:08X}")
        return state["sp"]

    def press(key: str) -> None:
        subprocess.run(["xdotool", "keydown", "--clearmodifiers", key], check=False)
        run_frames(HOLD_FRAMES)
        subprocess.run(["xdotool", "keyup", "--clearmodifiers", key], check=False)

    if args.load:
        press("l")
        run_frames(90)

    route = [k for k in args.route.split(",") if k]
    out(f"# 路線 {len(route)} 步，可移動判準 SP >= {field_sp:06X}")

    for i, key in enumerate(route, 1):
        for attempt in range(MAX_ATTEMPTS):
            sp = run_frames(2)
            if sp >= field_sp:
                press(key)
                sp = run_frames(SETTLE_FRAMES)
                out(f"{i:3d} 送 {key:<5} SP {sp:06X}")
                break
            # 卡在對話框或選單裡：送確認鍵剝掉一層，再回來看 SP。
            press(args.confirm)
            run_frames(SETTLE_FRAMES)
            out(f"{i:3d} （SP {sp:06X} 表示有 modal，送 {args.confirm} 清一層）")
        else:
            out(f"{i:3d} 放棄：試了 {MAX_ATTEMPTS} 次仍然離不開 modal")
            break

    if args.shot:
        subprocess.run(f"xwd -root -silent | convert xwd:- /out/{args.shot}.png",
                       shell=True, check=False)
        out(f"# 截圖 {args.shot}.png")

    log.close()
    proc.kill()


if __name__ == "__main__":
    main()
