#!/usr/bin/env python3
"""從反組譯推出 Mega Drive 版「哪一首配哪一個場景」。

**這條鏈刻意不靠人耳。** 聽起來像城鎮不是證據 —— 換一個聽的人就換一個答案，
而且沒辦法重跑。這支把整條推導寫成可重跑的程式：呼叫端 → case → 曲目位址，
再用「編譯器把字串資料排在使用它的程式碼附近」這個性質，替每個呼叫端取得語意錨點。

輸入：
    ROM（玩家自備）
    workplace/ida/out/funcs.txt   由 tools/ida_funcs.idc 匯出的函式邊界

做法：
    1. 找所有選曲呼叫：直接 bsr/jsr 到 sub_B620，以及走 a5 thunk 的
       `jsr $22E(a5)`。**只掃直接呼叫會漏掉七成** —— 這個編譯器預設走 thunk。
    2. 呼叫端前面的 `moveq #N,d0` + `move.l d0,-(sp)` 就是 case。
    3. case → 曲目位址：讀 0x0B620 那支跳表 switch 的位移表與各分支。
    4. 呼叫端所屬函式（IDA 匯出的邊界）＋ 附近的英文句子 = 語意錨點。
       單一一筆錨點不算數，判準是整批彼此一致：戰鬥字串要聚在戰鬥 case、
       設施字串要聚在設施 case。
"""

from __future__ import annotations

import re
import sys

DISPATCH = 0x00B620  # sub_B620：選曲分派
THUNK_BASE = 0x312  # a5 的 jmp abs.l 表
CASE_TABLE = 0x0B648  # move.w (6,pc,d0.w),d0 的表基底
JMP_BASE = 0x0B646  # jmp (0,pc,d0.w) 的 PC 基準
SET_REQUESTED = bytes([0x2B, 0x5F, 0xC8, 0x50])  # move.l (sp)+,-$37B0(a5)
PUSH_IMM = bytes([0x2E, 0xBC])  # move.l #imm,(sp)


def load_funcs(path: str) -> list[tuple[int, int, str]]:
    out = []
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            parts = line.rstrip("\n").split("\t")
            if len(parts) == 3:
                out.append((int(parts[0], 16), int(parts[1], 16), parts[2]))
    return sorted(out)


def func_of(funcs, ea: int):
    for lo, hi, name in funcs:
        if lo <= ea < hi:
            return lo, hi, name
    return None


def thunk_disp(rom: bytes, target: int) -> int | None:
    """target 在 a5 跳表裡的位移；不在表裡回 None。"""
    for i in range(256):
        off = THUNK_BASE + i * 6
        if rom[off : off + 2] != b"\x4e\xf9":
            break
        if int.from_bytes(rom[off + 2 : off + 6], "big") == target:
            return i * 6
    return None


def call_sites(rom: bytes, disp: int | None) -> list[int]:
    """所有呼叫選曲分派的位址：直接呼叫 ＋ 走 thunk。"""
    sites = []
    for pc in range(0, len(rom) - 6, 2):
        op = int.from_bytes(rom[pc : pc + 2], "big")
        if op == 0x4EB9 and int.from_bytes(rom[pc + 2 : pc + 6], "big") == DISPATCH:
            sites.append(pc)
        elif op in (0x6100, 0x4EBA):
            if pc + 2 + int.from_bytes(rom[pc + 2 : pc + 4], "big", signed=True) == DISPATCH:
                sites.append(pc)
        elif (op & 0xFF00) == 0x6100 and (op & 0xFF):
            d = op & 0xFF
            if pc + 2 + (d - 256 if d & 0x80 else d) == DISPATCH:
                sites.append(pc)
        elif disp is not None and op == 0x4EAD:
            if int.from_bytes(rom[pc + 2 : pc + 4], "big") == disp:
                sites.append(pc)
    return sites


def case_of(rom: bytes, site: int) -> int | None:
    """呼叫前的 moveq #N,d0 + move.l d0,-(sp)。"""
    if rom[site - 2 : site] == bytes([0x2F, 0x00]) and rom[site - 4] == 0x70:
        return rom[site - 3]
    if rom[site - 4 : site - 2] == bytes([0x2F, 0x00]) and rom[site - 6] == 0x70:
        return rom[site - 5]
    return None


def case_songs(rom: bytes) -> dict[int, list[int]]:
    """case → 它會設定的曲目位址（可能多首，依另一個變數再分支）。"""
    targets = [
        JMP_BASE + int.from_bytes(rom[CASE_TABLE + i * 2 : CASE_TABLE + i * 2 + 2], "big")
        for i in range(0x15)
    ]
    bounds = sorted(set(targets)) + [0x0B840]
    out: dict[int, list[int]] = {}
    for i, t in enumerate(targets):
        end = next((b for b in bounds if b > t), t + 16)
        songs, p = [], t
        while p < end - 9:
            if rom[p : p + 2] == PUSH_IMM and rom[p + 6 : p + 10] == SET_REQUESTED:
                songs.append(int.from_bytes(rom[p + 2 : p + 6], "big"))
                p += 10
            else:
                p += 2
        out[i] = songs
    return out


def prose_strings(rom: bytes) -> list[tuple[int, str]]:
    """看起來像英文句子的字串。純資料與程式碼碰巧可列印的片段要濾掉。"""
    out = []
    for m in re.finditer(rb"[\x20-\x7E]{6,}\x00", rom):
        s = m.group()[:-1].decode("latin1")
        alpha = sum(c.isalpha() or c == " " for c in s)
        if alpha > len(s) * 0.75 and " " in s.strip():
            out.append((m.start(), s))
    return out


def anchors_near(strings, site: int, span: int = 0x500, n: int = 2):
    """呼叫端附近的字串。

    這是**強推論**不是已證實：依據是這個編譯器把字串資料排在使用它的程式碼
    附近。MM2 的文字不用絕對指標也不用 PC 相對引用（與 DOS 版一樣走索引表），
    所以拿不到精確引用，只能用位置。判準是「整批呼叫端的錨點彼此一致」——
    戰鬥字串聚在戰鬥 case、設施字串聚在設施 case，單一一筆不算數。
    """
    near = sorted((abs(a - site), a, s) for a, s in strings if abs(a - site) < span)
    return [(d, s) for d, _a, s in near[:n]]


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("用法: md_music_scenes.py <ROM> <funcs.txt>")
    rom = open(sys.argv[1], "rb").read()
    funcs = load_funcs(sys.argv[2])

    disp = thunk_disp(rom, DISPATCH)
    sites = call_sites(rom, disp)
    songs = case_songs(rom)

    print(f"選曲分派 0x{DISPATCH:06X}，a5 thunk 位移 = {disp}")
    print(f"呼叫端 {len(sites)} 處（含走 thunk 的）\n")

    by_case: dict[int, list[int]] = {}
    for s in sites:
        c = case_of(rom, s)
        if c is not None:
            by_case.setdefault(c, []).append(s)

    strings = prose_strings(rom)
    print(f"{'case':>4} {'曲目':<28} 呼叫端、所屬函式、附近的字串錨點")
    print("-" * 100)
    for c in sorted(by_case):
        sl = songs.get(c, [])
        songtxt = ", ".join(f"0x{a:06X}" for a in sl) if sl else "（不換曲）"
        print(f"{c:>4} {songtxt:<28}")
        for s in by_case[c]:
            f = func_of(funcs, s)
            if not f:
                hint = " ／ ".join(f"{t[:44]!r}(±{d})" for d, t in anchors_near(strings, s))
                print(f"       0x{s:06X}  {'(IDA 未分析)':<12} {hint or '（附近無字串）'}")
                continue
            _lo, _hi, name = f
            hint = " ／ ".join(f"{t[:44]!r}(±{d})" for d, t in anchors_near(strings, s))
            print(f"       0x{s:06X}  {name:<12} {hint or '（附近無字串）'}")
        print()


if __name__ == "__main__":
    main()
