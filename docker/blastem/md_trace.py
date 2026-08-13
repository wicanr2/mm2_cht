#!/usr/bin/env python3
"""Mega Drive 動態追蹤：下中斷點、送按鍵、記錄每一次命中。

用途是**靜態掃描到頂之後**的下一步。靜態能回答「這段碼在什麼條件下執行」，
回答不了「玩家實際走到旅店時跑的是哪一支」——後者要讓遊戲真的跑起來。

    md_trace.py <rom> --break 0xB620:選曲 --break 0x73FC:印字 \\
        --timeline "wait:8;key:Return;wait:2;key:Down" --max-hits 200

每次命中會記錄：中斷點名稱、PC、回傳位址（`(sp)` 的長字）、d0–d2、
以及可選的參數解讀（`--arg-str N` 把第 N 個堆疊參數當字串指標讀出來）。

**回傳位址是這支工具的重點**：它告訴你「這一次是誰呼叫的」，
而那正是靜態掃描給不出來的東西 —— 靜態只知道「有 39 個呼叫端」，
動態才知道「站在旅店裡的時候，跑的是第幾個」。

坑（都會靜默失敗，細節見 rsp.py）：**按鍵是停在中斷點時送的** ——
模擬器當下不處理視窗事件，但 X 會把事件排進佇列，續跑之後才處理。
反過來為了送鍵而先放行是錯的：stub 不回應 raw 0x03，放行之後就再也停不下來，
只能等下一次中斷點命中。
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from rsp import Rsp  # noqa: E402


def parse_break(s: str) -> tuple[int, str]:
    if ":" in s:
        a, name = s.split(":", 1)
    else:
        a, name = s, ""
    addr = int(a, 16 if a.lower().startswith("0x") or not a.isdigit() else 10)
    return addr, name or f"0x{addr:06X}"


def find_window() -> str:
    r = subprocess.run(
        ["xdotool", "search", "--onlyvisible", "--class", "blastem"],
        capture_output=True, text=True, check=False,
    )
    return (r.stdout.split() or [""])[0]


def send_key(window: str, name: str) -> None:
    cmd = ["xdotool", "key", "--clearmodifiers", name]
    if window:
        cmd = ["xdotool", "key", "--window", window, "--clearmodifiers", name]
    subprocess.run(cmd, check=False)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--break", dest="breaks", action="append", default=[],
                    metavar="位址[:名稱]", help="可重複")
    ap.add_argument("--timeline", default="",
                    help="每命中一次推進一格：key:KEYSYM 或 wait:秒數")
    ap.add_argument("--max-hits", type=int, default=100)
    ap.add_argument("--skip", type=int, default=0,
                    help="前 N 次命中不記錄（跳過開機期間的呼叫）")
    ap.add_argument("--arg-str", type=int, default=None,
                    help="把第 N 個堆疊參數（0 起算）當字串指標讀出來")
    ap.add_argument("--log", default="/out/trace.txt")
    args = ap.parse_args()

    breaks = dict(parse_break(b) for b in args.breaks)
    if not breaks:
        raise SystemExit("至少要給一個 --break")

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

    out(f"# ROM {os.path.basename(args.rom)}")
    out("# 中斷點：" + "、".join(f"0x{a:06X} {n}" for a, n in breaks.items()))

    for a in breaks:
        rsp.add_break(a)

    window = ""
    steps = [s for s in args.timeline.split(";") if s]
    step_i = 0
    hits = 0

    out(f"{'#':>4} {'中斷點':<10} {'PC':>8} {'呼叫者':>9}  d0/d1/d2")
    while hits < args.max_hits:
        rsp.cont()
        regs = rsp.regs()
        pc = regs[-1]
        name = breaks.get(pc, f"0x{pc:06X}?")
        sp = regs[15]
        try:
            ret = rsp.read_u32(sp)
        except Exception:
            ret = 0
        extra = ""
        if args.arg_str is not None:
            try:
                p = rsp.read_u32(sp + 4 + args.arg_str * 4)
                extra = "  " + repr(rsp.read_cstr(p, 64))
            except Exception:
                extra = "  (字串讀不到)"
        hits += 1
        if hits > args.skip:
            out(f"{hits:>4} {name:<10} {pc:08X} {ret:9X}  "
                f"{regs[0]:08X}/{regs[1]:08X}/{regs[2]:08X}{extra}")

        # 每命中一次推進一格 timeline。
        #
        # **按鍵是停著的時候送的。** X 會把事件排進佇列，模擬器續跑之後才處理，
        # 所以不需要（也不能）為了送鍵而先放行 —— 放行之後就再也停不下來
        # （stub 不收 raw 0x03），中斷點會在下一次命中時才把控制權交回來。
        if step_i < len(steps):
            st = steps[step_i]; step_i += 1
            act, _, arg = st.partition(":")
            if act == "key":
                if not window:
                    window = find_window()
                send_key(window, arg)
            elif act == "wait":
                time.sleep(float(arg))

    out(f"# 命中 {hits} 次，記錄在 {args.log}")
    log.close()
    proc.kill()


if __name__ == "__main__":
    main()
