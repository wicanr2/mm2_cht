#!/usr/bin/env python3
"""在幾個已知的畫面狀態之間比對整塊 work RAM，找出代表那個狀態的旗標。

用途是把按鍵腳本從「盲打」變成「狀態驅動」。盲打走不完長路徑：
確認鍵在有對話框時是「確認」、沒有對話框時是「開啟選單」，
沒有一個按鍵在兩種狀態下都安全，腳本一定發散。

    ram_diff.py <rom> --phase 名稱:按鍵:幀數 [--phase …] [--out /out]

每個 phase 送一個按鍵（`-` 代表不送）、跑指定幀數，然後把
`0xFF0000`–`0xFFFFFF` 整塊存成 `/out/ram-<名稱>.bin`。
最後兩兩比對，列出「在某些狀態相同、在另一些狀態不同」的位址。

怎麼停下來讀記憶體：**stub 不回應非同步中斷**，放行之後就再也停不下來，
所以在每一次 vblank 下中斷點，用「命中 → 讀 → 放行」當成單步。
代價是模擬速度掉到約 1/6，幀數要照這個算 wall clock。

按鍵在停著的時候送：X 會排進佇列，模擬器續跑之後才處理。
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from rsp import Rsp  # noqa: E402

RAM_BASE = 0xFF0000
RAM_SIZE = 0x10000
CHUNK = 512

# 每一幀都會經過的位置（vblank 的音樂管理），拿來當「單步」的節拍器。
VBLANK_BREAK = 0x06CB02

# 手把按住幾個**模擬幀**。太短遊戲 poll 不到，太長會被選單當成長按重複觸發。
HOLD_FRAMES = 6


def key_down(name: str) -> None:
    subprocess.run(["xdotool", "keydown", "--clearmodifiers", name], check=False)


def key_up(name: str) -> None:
    subprocess.run(["xdotool", "keyup", "--clearmodifiers", name], check=False)


def snapshot(rsp: Rsp) -> bytes:
    out = bytearray()
    for off in range(0, RAM_SIZE, CHUNK):
        out += rsp.read_mem(RAM_BASE + off, CHUNK)
    return bytes(out)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--phase", action="append", default=[], metavar="名稱:按鍵:幀數",
                    help="可重複。按鍵給 `-` 表示不送鍵")
    ap.add_argument("--out", default="/out")
    ap.add_argument("--shot", action="store_true", help="每個 phase 也截一張圖")
    ap.add_argument("--no-dump", action="store_true",
                    help="不存 RAM 快照，只報 SP（快很多）")
    ap.add_argument("--sp-frames", type=int, default=8,
                    help="每個 phase 結尾連續取樣幾幀的 SP")
    args = ap.parse_args()

    phases = []
    for spec in args.phase:
        name, key, frames = spec.split(":")
        phases.append((name, key, int(frames)))
    if not phases:
        raise SystemExit("至少要給一個 --phase")

    proc = subprocess.Popen(
        ["/opt/blastem/blastem", args.rom, "-D"],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, bufsize=0,
    )
    rsp = Rsp(proc)
    if not rsp.status().startswith(b"S"):
        raise SystemExit("stub 沒有停在進入點")
    rsp.add_break(VBLANK_BREAK)
    rsp.cont()  # 第一次要等整個開機流程，可能十秒以上

    snaps = {}
    for name, key, frames in phases:
        # [HARD] 按住的時間要用**模擬幀數**算，不是真實秒數。
        #
        # 這支工具靠「命中 vblank 中斷點 → 讀 → 放行」當單步，所以模擬器
        # 絕大多數時間是凍住的：真實時間按住 0.08 秒等於**零個模擬幀**，
        # 遊戲每幀 poll 一次手把，看到的永遠是「沒按」。
        # 症狀是按鍵完全無效，而畫面一切正常 —— 與 md_trace 那邊
        # 「按太短」的坑同源，但在這裡即使按住幾秒也一樣沒用。
        if key != "-":
            key_down(key)
            for _ in range(min(HOLD_FRAMES, frames)):
                rsp.cont()
            key_up(key)
            frames -= min(HOLD_FRAMES, frames)
        for _ in range(frames):
            rsp.cont()

        # 堆疊指標本身就是「現在是不是卡在巢狀的 modal 迴圈裡」的訊號：
        # 對話框與選單都是在原本的主迴圈裡再進一層，SP 會明顯更低。
        # 這比在 RAM 裡找旗標可靠 —— 不必猜哪個位元組有意義，
        # 而且 `g` 一次就拿得到，不必掃 64 KB。
        sps = []
        for _ in range(args.sp_frames):
            sps.append(rsp.regs()[15])
            rsp.cont()
        sp_txt = f"SP {min(sps):06X}–{max(sps):06X}"

        if args.no_dump:
            print(f"[ram_diff] {name}: 送鍵 {key}、跑 {frames} 幀  {sp_txt}", flush=True)
        else:
            snaps[name] = snapshot(rsp)
            path = os.path.join(args.out, f"ram-{name}.bin")
            with open(path, "wb") as fh:
                fh.write(snaps[name])
            print(f"[ram_diff] {name}: 送鍵 {key}、跑 {frames} 幀  {sp_txt} → {path}",
                  flush=True)
        if args.shot:
            subprocess.run(
                f"xwd -root -silent | convert xwd:- {args.out}/ram-{name}.png",
                shell=True, check=False)

    if not args.no_dump:
        names = [p[0] for p in phases]
        print(f"\n[ram_diff] 兩兩差異的位元組數（{len(names)} 個狀態）", flush=True)
        for i, a in enumerate(names):
            for b in names[i + 1:]:
                n = sum(1 for x, y in zip(snaps[a], snaps[b]) if x != y)
                print(f"  {a} vs {b}: {n}")

    proc.kill()


if __name__ == "__main__":
    main()
