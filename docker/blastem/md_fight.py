#!/usr/bin/env python3
"""把一場戰鬥自動打完，途中記錄指定中斷點的命中。

    md_fight.py <rom> --load --route Up,Up,Up,Right,Up \\
        --break 0xB620:選曲 --invincible --shot fight

為什麼要有這一支：走路那一套（`md_walk.py`）在戰鬥裡會發散。戰鬥選單有
八個指令，盲送確認鍵會掉進 `Fight`／`Shoot`／`Cast`／`Use` 的「選哪一隻怪」
子選單，一輪都推不完 —— 實測連按 60 次確認鍵，回合數還停在 1。

原廠說明書（Mega Drive 版）把可自動化的那一條講得很清楚：

    Attack: Character attacks the first monster, using whatever weapon they
            have equipped. If the first monster dies as a result of this
            attack, all the monsters behind it move up one position.

**八個指令裡只有 `Attack` 沒有後續提示**，而且它就是選單第一項。

**還沒解決的是「怎麼把選擇標記移到 Attack」**，三條都試過：

- 完全不按方向鍵（`--climb 0`）：標記是**跨回合保留**的，停在 `Cast`
  就每次都進法術面板。
- 方向鍵按住讓自動重複撞到頂（`--climb 45`）：選單是**環狀**的，會繞過頭，
  實測繞進 `Controls` 子選單。
- 找出「標記停在第幾項」那個變數：按兩次 `Down` 做等差搜尋，
  8 個候選全是計時器。標記大概不是存成單純的索引位元組
  （可能是指標，或只存在 sprite 屬性表裡 —— 那在 VRAM，`m` 讀不到）。

所以目前這支能可靠做到的是：**把仗打起來、讓隊伍不會死、把回合推進**，
還不能保證每一次行動都選到 `Attack`。

不會被打死：每隔幾幀把「目前 HP」寫回「最大 HP」。**改的是模擬器記憶體，
不是 ROM**（這片有開機完整性檢查，改 ROM 就開不了機）。
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from rsp import Rsp  # noqa: E402

VBLANK_BREAK = 0x06CB02
HOLD_FRAMES = 6          # 一般按鍵按住幾個模擬幀
CLIMB_FRAMES = 45        # 方向鍵按住多久，讓自動重複把標記推到最上面
SETTLE_FRAMES = 20
HP_EVERY = 8             # 每幾幀補一次血


def parse_break(s: str) -> tuple[int, str]:
    a, _, name = s.partition(":")
    addr = int(a, 16)
    return addr, name or f"0x{addr:06X}"


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--route", default="", help="先走這條路線把仗打起來")
    ap.add_argument("--break", dest="breaks", action="append", default=[],
                    metavar="位址[:名稱]")
    ap.add_argument("--ignore-d0", default="")
    ap.add_argument("--load", action="store_true")
    ap.add_argument("--field-sp", default="FFCA00")
    ap.add_argument("--actions", type=int, default=60,
                    help="最多送幾次「選最上面那一項 ＋ 確認」")
    ap.add_argument("--hp", default="FFCE9F,2,134,6",
                    help="目前HP位址,到最大HP的位移,角色記錄長度,人數")
    ap.add_argument("--invincible", action="store_true",
                    help="每隔幾幀把目前 HP 寫回最大 HP（只改模擬器記憶體）")
    ap.add_argument("--watch-hp", action="store_true", help="每次行動前印一次全隊 HP")
    ap.add_argument("--wait-combat", type=int, default=120,
                    help="路線走完後最多等幾幀讓遭遇畫面起來")
    ap.add_argument("--find-cursor", action="store_true",
                    help="進入戰鬥後按兩次 Down，找出「選擇標記停在第幾項」那個位元組")
    ap.add_argument("--climb", type=int, default=0,
                    help="按確認前先把方向鍵按住幾幀往上推。0 = 不按（選單是環狀的，按住會繞回去）")
    ap.add_argument("--shot", default="")
    ap.add_argument("--log", default="/out/fight.txt")
    args = ap.parse_args()

    field_sp = int(args.field_sp, 16) | 0xFF000000
    hp_base, hp_max_off, rec_len, n_chars = (int(v, 16) if i == 0 else int(v)
                                             for i, v in enumerate(args.hp.split(",")))
    ignore_d0 = {int(v, 16) for v in args.ignore_d0.split(",") if v.strip()}
    breaks = dict(parse_break(b) for b in args.breaks)

    proc = subprocess.Popen(["/opt/blastem/blastem", args.rom, "-D"],
                            stdin=subprocess.PIPE, stdout=subprocess.PIPE, bufsize=0)
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
    rsp.cont()

    state = {"sp": 0, "frame": 0}

    def hp_pairs() -> list[tuple[int, int]]:
        return [(rsp.read_mem(hp_base + k * rec_len, 1)[0],
                 rsp.read_mem(hp_base + hp_max_off + k * rec_len, 1)[0])
                for k in range(n_chars)]

    def hp_probe() -> str:
        """把角色記錄裡三個「滿血時等於 HP」的候選欄位一起印出來。

        滿血時目前值與上限相等，所以用「等於 HP 的位元組序列」搜出來的候選
        一定不只一個 —— 要挨打之後才分得出哪個是目前值。
        """
        cols = []
        for off in (0x00, 0x02, 0x16):
            cols.append("+%02X:" % off + ",".join(
                str(rsp.read_mem(hp_base + off + k * rec_len, 1)[0])
                for k in range(n_chars)))
        return "  ".join(cols)

    def top_up() -> None:
        """把目前 HP 寫回最大 HP。max 從記憶體讀，不寫死數字。"""
        for k in range(n_chars):
            cur = hp_base + k * rec_len
            mx = rsp.read_mem(cur + hp_max_off, 1)[0]
            if rsp.read_mem(cur, 1)[0] != mx:
                rsp.cmd(f"M{cur:x},1:{mx:02x}".encode())

    def run_frames(n: int) -> int:
        done = 0
        while done < n:
            rsp.cont()
            regs = rsp.regs()
            pc = regs[-1]
            if pc == VBLANK_BREAK:
                done += 1
                state["sp"] = regs[15]
                state["frame"] += 1
                if args.invincible and state["frame"] % HP_EVERY == 0:
                    top_up()
                continue
            if (regs[0] & 0xFFFFFFFF) in ignore_d0:
                continue
            try:
                ret = rsp.read_u32(regs[15])
            except Exception:
                ret = 0
            out(f"  ★ {breaks.get(pc, f'0x{pc:06X}')}  呼叫者 {ret:06X}  "
                f"d0={regs[0]:08X} d1={regs[1]:08X} d2={regs[2]:08X}")
        return state["sp"]

    def press(key: str, frames: int = HOLD_FRAMES) -> None:
        subprocess.run(["xdotool", "keydown", "--clearmodifiers", key], check=False)
        run_frames(frames)
        subprocess.run(["xdotool", "keyup", "--clearmodifiers", key], check=False)

    if args.load:
        press("l")
        run_frames(90)

    for i, key in enumerate(k for k in args.route.split(",") if k):
        sp = run_frames(2)
        if sp < field_sp:
            out(f"# 路線第 {i + 1} 步：已經不在可移動狀態（SP {sp:06X}），開打")
            break
        press(key)
        sp = run_frames(SETTLE_FRAMES)
        out(f"{i + 1:3d} 送 {key:<5} SP {sp:06X}")

    def sample_sp(frames: int = 8) -> int:
        """取這幾幀裡**最低**的 SP。

        單點取樣不可靠：一幀的 SP 可能剛好落在某個深層繪圖常式裡，
        也可能剛好在 modal 的空檔。判「現在是不是在戰鬥」要看最低點。
        """
        lo = 0xFFFFFFFF
        for _ in range(frames):
            run_frames(1)
            lo = min(lo, state["sp"])
        return lo

    # 遭遇是隨機的，而且畫面要幾幀才起來。先等它出現再開打。
    for _ in range(args.wait_combat // 8):
        if sample_sp() < field_sp:
            break
    else:
        out(f"# 等了 {args.wait_combat} 幀都沒進入戰鬥（SP 一直在可移動區間）")

    if args.find_cursor:
        # 戰鬥選單是**環狀**的：按住方向鍵讓它「撞到頂」會繞回去，
        # 實測繞進了 Controls 子選單。要精準選 Attack 就得知道
        # 選擇標記現在停在第幾項 —— 用「按一下 Down、值 +1」把它找出來。
        def snap() -> bytes:
            return b"".join(rsp.read_mem(0xFF0000 + o, 256)
                            for o in range(0, 0x10000, 256))
        a = snap()
        press("Down"); run_frames(SETTLE_FRAMES)
        b = snap()
        press("Down"); run_frames(SETTLE_FRAMES)
        c = snap()
        # 標記不一定存成「第幾項」—— 也可能是螢幕列位置（每項 +8）
        # 或指向選單項目的指標。所以找的是**等差**，不是 +1。
        hits = [i for i in range(0x10000)
                if a[i] != b[i] and (b[i] - a[i]) & 0xFF == (c[i] - b[i]) & 0xFF]
        out(f"# 按 Down 兩次時呈等差變化的位元組：{len(hits)} 個")
        for i in hits:
            out(f"    0xFF{i:04X}  {a[i]} → {b[i]} → {c[i]}"
                f"   （每次 {(b[i] - a[i]) & 0xFF:+d}）")
        log.close()
        proc.kill()
        return

    out(f"# 開始自動戰鬥（最多 {args.actions} 次行動）"
        + ("，無敵開啟" if args.invincible else ""))
    for a in range(1, args.actions + 1):
        sp = sample_sp()
        if sp >= field_sp:
            out(f"# 第 {a} 次行動前已回到可移動狀態 —— 戰鬥結束")
            break
        if args.watch_hp:
            out("    " + hp_probe())
        # 選擇標記推到最上面（Attack）再確認。
        #
        # ⚠ 戰鬥選單是**環狀**的，按住方向鍵「撞到頂」會繞回去 ——
        # 實測繞進了 Controls 子選單（畫面跑出 Protection 面板）。
        # `--climb 0` 表示完全不按方向鍵，賭「每個角色輪到時標記會回到 Attack」。
        if args.climb:
            press("Up", args.climb)
            run_frames(4)
        press("d")
        run_frames(SETTLE_FRAMES)
        out(f"{a:3d} Attack  SP {state['sp']:06X}")

    if args.shot:
        subprocess.run(f"xwd -root -silent | convert xwd:- /out/{args.shot}.png",
                       shell=True, check=False)
        out(f"# 截圖 {args.shot}.png")

    log.close()
    proc.kill()


if __name__ == "__main__":
    main()
