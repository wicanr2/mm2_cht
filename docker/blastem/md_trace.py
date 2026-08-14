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
import threading
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


def send_key(name: str, hold: float = 0.08) -> None:
    """送一個按鍵給 BlastEm。

    [HARD] 一律走 XTEST（不加 `--window`）而且按住的時間要落在一個窄區間裡。
    三個獨立的坑，症狀都是「按了沒反應」或「勾選狀態隨機」，畫面一切正常：

    1. `xdotool key --window <id>` 是 XSendEvent 合成事件，BlastEm 的**手把輸入**
       收不到（UI 熱鍵如 `m` 反而收得到 —— 所以「VGM 錄得到」不能拿來證明按鍵有效）。
    2. 按下與放開幾乎同時（`xdotool key`），遊戲每幀只 poll 一次手把就整個漏掉。
    3. 按太久（≥0.15 秒）會被選單當成長按重複觸發，勾選被切換兩次等於沒按。
       0.08 秒（約 5 個 frame）實測穩定。

    判準：送 `Down` 看選單游標有沒有移動、而且**只移動一格**。
    """
    subprocess.run(["xdotool", "keydown", "--clearmodifiers", name], check=False)
    time.sleep(hold)
    subprocess.run(["xdotool", "keyup", "--clearmodifiers", name], check=False)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--break", dest="breaks", action="append", default=[],
                    metavar="位址[:名稱]", help="可重複")
    ap.add_argument("--timeline", default="",
                    help="每命中一次推進一格：key:KEYSYM 或 wait:秒數")
    ap.add_argument("--keys", default="",
                    help="改用真實時間推進的按鍵腳本（背景執行緒），格式同 --timeline。"
                         "中斷點很少命中時要用這個 —— 命中驅動的 --timeline 會停在原地")
    ap.add_argument("--max-hits", type=int, default=100)
    ap.add_argument("--exit-after", type=float, default=5.0,
                    help="--keys 腳本跑完後再等幾秒才結束")
    ap.add_argument("--skip", type=int, default=0,
                    help="前 N 次命中不記錄（跳過開機期間的呼叫）")
    ap.add_argument("--ignore-d0", default="",
                    help="d0 是這些值（逗號分隔的十六進位）時只放行不記錄。"
                         "分派函式常有每幀都呼叫的查詢型 case，不濾掉會淹掉 log，"
                         "而且每次停下都要一輪 RSP 往返，會把模擬拖慢到按鍵腳本失準")
    ap.add_argument("--arg-str", type=int, default=None,
                    help="把第 N 個堆疊參數（0 起算）當字串指標讀出來")
    ap.add_argument("--poke", action="append", default=[], metavar="位址=長字",
                    help="放行之前先寫一個 32-bit 值（十六進位），可重複。"
                         "改的是**模擬器記憶體**不是 ROM 檔 —— 要做「改一個位元組會怎樣」"
                         "這類實驗時用這個，不要動 ROM（商業卡帶常有開機完整性檢查）")
    ap.add_argument("--log", default="/out/trace.txt")
    args = ap.parse_args()

    ignore_d0 = {int(v, 16) for v in args.ignore_d0.split(",") if v.strip()}
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

    # 先 poke 再下中斷點：要動的常常是開機早期就會被讀到的位址。
    # **`M` 封包寫成功回 `OK`，位址算錯照樣回 `OK`**，所以一律回讀驗證。
    for spec in args.poke:
        addr_s, _, val_s = spec.partition("=")
        addr, val = int(addr_s, 16), int(val_s, 16)
        before = rsp.read_u32(addr)
        rsp.write_u32(addr, val)
        after = rsp.read_u32(addr)
        out(f"# poke 0x{addr:06X}: {before:08X} → {after:08X}"
            + ("" if after == val else "  ⚠ 沒寫進去（這個位址可能唯讀）"))

    for a in breaks:
        rsp.add_break(a)

    steps = [s for s in args.timeline.split(";") if s]
    step_i = 0
    hits = 0

    # 真實時間推進的按鍵腳本。
    #
    # 命中驅動（`--timeline`）只在中斷點很密集時有用；下在「選曲」這種一個場景
    # 才觸發一次的函式上，腳本會停在第一格，遊戲永遠走不到目標畫面。
    #
    # 代價是**停在中斷點時模擬器時間是凍結的，而這個執行緒的時間不會**，
    # 命中越密集漂移越大。所以只在中斷點稀疏時用，並且把 wait 抓寬一點。
    # 按鍵本身不會掉：X 會排進佇列，模擬器續跑之後才處理。
    if args.keys:
        def play_keys() -> None:
            for st in (s for s in args.keys.split(";") if s):
                act, _, arg = st.partition(":")
                if act == "key":
                    send_key(arg)
                elif act == "wait":
                    time.sleep(float(arg))
                elif act == "shot":
                    # 截圖是這支工具的驗收面：中斷點零命中時，它分辨
                    # 「這一段程式沒被執行」與「按鍵腳本根本沒走到那個畫面」。
                    subprocess.run(
                        f"xwd -root -silent | convert xwd:- /out/{arg}.png",
                        shell=True, check=False)
                    out(f"# 截圖 {arg}.png")
            out("# 按鍵腳本執行完畢")
            # 主迴圈停在 `cont()` 的阻塞讀取上，最後一次命中之後不會自己醒來。
            # 腳本跑完再等一小段時間收尾命中，然後直接把行程結束掉。
            time.sleep(args.exit_after)
            out(f"# 命中 {hits} 次，記錄在 {args.log}")
            log.flush()
            os._exit(0)

        threading.Thread(target=play_keys, daemon=True).start()

    out(f"{'#':>4} {'中斷點':<10} {'PC':>8} {'呼叫者':>9}  d0/d1/d2")
    while hits < args.max_hits:
        rsp.cont()
        regs = rsp.regs()
        pc = regs[-1]
        # 查詢型 case 每幀都來。在讀回傳位址與參數之前就濾掉，省掉那幾輪 RSP 往返
        # —— 那才是把模擬拖慢、讓按鍵腳本失準的成本。
        if (regs[0] & 0xFFFFFFFF) in ignore_d0:
            continue
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
                send_key(arg)
            elif act == "wait":
                time.sleep(float(arg))

    out(f"# 命中 {hits} 次，記錄在 {args.log}")
    log.close()
    proc.kill()


if __name__ == "__main__":
    main()
