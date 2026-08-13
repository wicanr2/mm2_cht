#!/usr/bin/env python3
"""逐首擷取 Mega Drive 配樂：用 GDB remote stub 在執行時指定曲目，不改 ROM。

為什麼走 GDB stub：
  - 改 ROM 不行 —— 這片有開機時的完整性檢查，改動任何一個位元組（尾端 padding
    除外）就開不了機，畫面全黑。見 docs/research/md-music-driver.md。
  - BlastEm 的原生除錯器也不行 —— 它靠 termhelper 另開終端機視窗，headless
    容器裡 `-d` 會靜默地不進除錯器，stdin 餵命令零回應。
  - `blastem ROM -D` 直接在 stdio 上講 GDB remote protocol，不需要終端機。

做法：在 vblank 的音樂管理下中斷點，每次命中就把「要播哪一首」寫成目標曲目，
連續壓幾十幀確保換曲完成，然後移除中斷點讓模擬器全速跑，這時才用熱鍵錄 VGM
（模擬器停在中斷點時不處理視窗事件，熱鍵送不進去）。

驗證「換曲確實完成」要趁還停在中斷點時讀回「正在播哪一首」——放行之後就問不到了，
BlastEm 的 stub 不回應 raw 0x03 非同步中斷，送了會永遠等不到回覆而卡死。

VGM 記錄的是模擬時間不是真實時間，所以中斷點造成的減速不會讓錄音變調。
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from rsp import Rsp  # noqa: E402

# vblank 的音樂管理入口（level_6_interrupt 裡比較「要播」與「正在播」的那一段）。
VBLANK_MUSIC = 0x06CB02

# a5 = 0x312，所以 -$37B0(a5) 與 -$37B4(a5) 的實體位址是：
REQUESTED_SONG = 0xFFCB62  # 要播哪一首
CURRENT_SONG = 0xFFCB5E  # 正在播哪一首


def key(window: str, name: str) -> None:
    cmd = ["xdotool", "key", "--clearmodifiers", name]
    if window:
        cmd = ["xdotool", "key", "--window", window, "--clearmodifiers", name]
    subprocess.run(cmd, check=False)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("rom")
    ap.add_argument("song", help="曲目起始位址（hex）")
    ap.add_argument("out", help="輸出的 .vgm 路徑")
    ap.add_argument("--hold-frames", type=int, default=40)
    ap.add_argument("--record-seconds", type=float, default=45.0)
    ap.add_argument("--vgm-dir", default=os.environ.get("HOME", "/tmp"))
    args = ap.parse_args()

    song = int(args.song, 16)

    proc = subprocess.Popen(
        ["/opt/blastem/blastem", args.rom, "-D"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        bufsize=0,
    )
    rsp = Rsp(proc)

    # 進入點就停著等我們，先確認狀態再動作。
    stop = rsp.cmd(b"?")
    if not stop.startswith(b"S"):
        raise SystemExit(f"預期停止回覆，收到 {stop!r}")

    rsp.add_break(VBLANK_MUSIC)

    # 每一幀都把「要播哪一首」壓成目標曲目。壓幾十幀是為了讓淡出換曲那段
    # 走完（原版每幀把音量減 2，從 $B0 降到 $58 要 44 幀）。
    for _ in range(args.hold_frames):
        rsp.cont()
        rsp.write_mem(REQUESTED_SONG, song)

    # 驗證要趁還停著的時候做：放行之後就沒有辦法再問了。
    # BlastEm 的 stub 不回應 raw 0x03 非同步中斷，送了會永遠等不到回覆。
    current = rsp.read_mem(CURRENT_SONG)
    if current != song:
        raise SystemExit(
            f"壓了 {args.hold_frames} 幀之後正在播的仍是 0x{current:06X}，"
            f"不是 0x{song:06X}；換曲沒有完成"
        )

    rsp.del_break(VBLANK_MUSIC)

    # 移除中斷點之後才送熱鍵：停在中斷點時模擬器不處理視窗事件，鍵送不進去。
    # 這裡只送不等回覆（要讓它一直跑），但**必須是合法封包** ——
    # 寫一個裸的 b"c" 進 stdin 會被 stub 忽略，模擬器就一直停著，
    # 症狀是熱鍵沒反應而且後面的 interrupt 永遠等不到回覆。
    rsp.send(b"c")

    time.sleep(2.0)
    window = ""
    try:
        window = (
            subprocess.run(
                ["xdotool", "search", "--onlyvisible", "--class", "blastem"],
                capture_output=True,
                text=True,
                check=False,
            ).stdout.split()
            or [""]
        )[0]
    except Exception:
        pass

    before = set(os.listdir(args.vgm_dir))
    key(window, "m")
    time.sleep(args.record_seconds)
    key(window, "m")
    time.sleep(1.5)

    produced = sorted(set(os.listdir(args.vgm_dir)) - before)
    vgm = [f for f in produced if f.endswith(".vgm")]

    proc.kill()

    if not vgm:
        raise SystemExit("沒有產生 VGM —— 熱鍵沒送進視窗")

    # /work 與 /out 是不同的掛載點，os.rename 會 EXDEV；而 image 裝的是
    # python3-minimal，沒有 shutil。自己複製再刪。
    src = os.path.join(args.vgm_dir, vgm[0])
    with open(src, "rb") as fin, open(args.out, "wb") as fout:
        while True:
            chunk = fin.read(1 << 20)
            if not chunk:
                break
            fout.write(chunk)
    os.unlink(src)
    size = os.path.getsize(args.out)
    # 錄製期間模擬器是自由執行的，沒有辦法再問它現在播什麼（stub 不收 raw 0x03），
    # 所以這裡只能保證「開始錄之前確實已經換到目標曲目」。
    print(f"0x{song:06X} → {args.out}（{size} bytes）")


if __name__ == "__main__":
    main()
