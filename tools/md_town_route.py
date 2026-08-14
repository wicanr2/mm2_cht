#!/usr/bin/env python3
"""從 DOS 版的 MAP.DAT 算出城鎮裡的走法，輸出 BlastEm 的按鍵腳本。

動態追蹤要走到某個設施門口才問得出「這個畫面跑的是哪一段程式」，
而**盲走非常貴**：每一次嘗試都要重開模擬器、載入狀態、逐格試牆。
牆的資料本來就在 MAP.DAT 裡（屬性層每格四個方向各一個位元），
直接 BFS 算最短路，一次就走到。

    python3 tools/md_town_route.py --map 0 --to 7,14
    python3 tools/md_town_route.py --map 0 --list        # 列出事件格

Mega Drive 版與 DOS 版共用同一套地圖佈局，實測四個設施座標完全吻合
（旅店 (7,3)、酒館 (4,6)、神殿 (7,7)、訓練所 (10,7)）。

座標：**第 0 列在南邊**，y 由南往北；格編號 = y*16 + x。
牆位元：北 bit6、東 bit4、南 bit2、西 bit0，1 = 有牆（低位元平面）。
"""

from __future__ import annotations

import argparse
import os
import sys
from collections import deque

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mm2lzw import unpack_segment  # noqa: E402

# 方向 → (dx, dy, 牆位元)
DIRS = {"N": (0, 1, 6), "E": (1, 0, 4), "S": (0, -1, 2), "W": (-1, 0, 0)}
ORDER = ["N", "E", "S", "W"]          # 順時針，Right 是 +1、Left 是 -1


def load_attr(path: str, seg_no: int) -> bytes:
    blob = open(path, "rb").read()
    offs = [int.from_bytes(blob[i * 2:i * 2 + 2], "little") for i in range(60)]
    return unpack_segment(blob, offs[seg_no])[1][256:512]


def load_events(path: str, seg_no: int) -> dict[int, bytes]:
    """回傳 {格編號: 腳本位元組}。腳本 `04 NN` 是招牌、`0e NN` 是設施入口。"""
    blob = open(path, "rb").read()
    offs = [int.from_bytes(blob[i * 4:i * 4 + 4], "little") for i in range(71)]
    seg = unpack_segment(blob, offs[seg_no])[1]
    i = 0
    recs = []
    while seg[i:i + 3] != b"\x00\x00\x00":
        recs.append((seg[i], seg[i + 1]))
        i += 3
    end = i + 3
    skip = int.from_bytes(seg[end:end + 2], "little")
    scripts, p = [], end + 2
    while p < end + skip:
        e = p
        while e < end + skip and seg[e] != 0xFF:
            e += 1
        scripts.append(seg[p:e])
        p = e + 1
    return {cell: scripts[idx] for cell, idx in recs if idx < len(scripts)}


def bfs(attr: bytes, start: tuple[int, int], goal: tuple[int, int],
        blocked: set[tuple[int, int]]) -> list[str] | None:
    """回傳一串方向。`blocked` 是「踩上去會觸發事件」的格子，除終點外一律繞開。"""
    q = deque([(start, [])])
    seen = {start}
    while q:
        (x, y), path = q.popleft()
        if (x, y) == goal:
            return path
        for d, (dx, dy, bit) in DIRS.items():
            if attr[y * 16 + x] >> bit & 1:
                continue
            nx, ny = x + dx, y + dy
            if not (0 <= nx < 16 and 0 <= ny < 16) or (nx, ny) in seen:
                continue
            if (nx, ny) in blocked and (nx, ny) != goal:
                continue
            seen.add((nx, ny))
            q.append(((nx, ny), path + [d]))
    return None


def to_cells(path: list[str], src: tuple[int, int], facing: str) -> str:
    """與 `to_keys` 逐鍵對齊的「這一鍵之後應該在哪一格」。

    轉向鍵不改變位置，所以會重複上一格。給 `md-walk --expect` 用：
    開迴路的路線一步錯就整條錯，而且**看起來一路順暢**，
    有了逐步的期望值才會在錯的那一步當場停下來。
    """
    cells, (x, y) = [], src
    for d in path:
        while facing != d:
            i, j = ORDER.index(facing), ORDER.index(d)
            turn = 1 if (j - i) % 4 <= 2 else -1
            facing = ORDER[(i + turn) % 4]
            cells.append(f"{x},{y}")
        dx, dy, _ = DIRS[d]
        x, y = x + dx, y + dy
        cells.append(f"{x},{y}")
    return ";".join(cells)


def to_keys(path: list[str], facing: str, wait: float) -> str:
    """方向串 → 按鍵腳本。轉向用最少的 Left／Right，不用 180 度一次轉完。"""
    steps = []
    for d in path:
        while facing != d:
            i, j = ORDER.index(facing), ORDER.index(d)
            turn = "Right" if (j - i) % 4 <= 2 else "Left"
            facing = ORDER[(i + (1 if turn == "Right" else -1)) % 4]
            steps.append(f"key:{turn};wait:{wait}")
        steps.append(f"key:Up;wait:{wait}")
    return ";".join(steps)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--orig", default="workplace/orig/MM2")
    ap.add_argument("--map", type=int, default=0, help="地圖段編號（0 = Middlegate）")
    ap.add_argument("--outdoor", action="store_true",
                    help="野外地圖（事件在 EVENTSO.DAT，不是 EVENTSI.DAT）")
    # 剛「離開旅店」進到城裡時是 (7,3)（DOS 版記憶體 dump 的直接證據，
    # 見 docs/playtest/01）。走一步北之後觸發 Corak 事件，那之後才是 (7,4)。
    # **起點差一格，整條路線就會走進別的設施**，而且看起來一路順暢。
    ap.add_argument("--from", dest="src", default="7,3")
    ap.add_argument("--to", dest="dst", default=None)
    ap.add_argument("--facing", default="N")
    ap.add_argument("--wait", type=float, default=1.1)
    ap.add_argument("--list", action="store_true", help="列出事件格就結束")
    ap.add_argument("--cells", action="store_true",
                    help="改印逐鍵對齊的期望座標（給 md-walk --expect）")
    args = ap.parse_args()

    attr = load_attr(os.path.join(args.orig, "MAP.DAT"), args.map)
    events = load_events(
        os.path.join(args.orig, "EVENTSO.DAT" if args.outdoor else "EVENTSI.DAT"),
        args.map)

    if args.list or not args.dst:
        for cell in sorted(events):
            sc = events[cell]
            kind = ""
            if sc[:1] == b"\x04":
                kind = "招牌"
            elif b"\x0e" in sc:
                kind = f"入口 0e {sc[sc.index(b'\x0e') + 1]:02x}"
            print(f"({cell % 16:2d},{cell // 16:2d}) {sc.hex(' '):<26} {kind}")
        return

    src = tuple(int(v) for v in args.src.split(","))
    dst = tuple(int(v) for v in args.dst.split(","))
    # 只有設施入口要繞開 —— 踩上去會跳出 Yes／No 對話框，後面的按鍵全部被吃掉。
    # 招牌（`04 NN`）只是在文字區顯示店名，不擋路（實測走過旅店與神殿的招牌格
    # 都沒有停下來）。
    #
    # 判斷要用**結構**：`0e` 開頭，或城鎮那個固定樣式 `0b XX 00 0e NN`。
    # 用「腳本裡出現過 0e 這個位元組」會誤判 —— 野外 (7,4) 的
    # `15 00 74 40 10 04 0b 0e 00 …` 只是某個運算元剛好是 0x0E，
    # 而它被擋掉之後 BFS 會繞一大圈，路線看起來合理但方向整個是錯的。
    blocked = {
        (c % 16, c // 16) for c, sc in events.items()
        if sc[:1] == b"\x0e"
        or (len(sc) >= 5 and sc[0] == 0x0B and sc[2] == 0x00 and sc[3] == 0x0E)
    }
    path = bfs(attr, src, dst, blocked)
    if path is None:
        # 繞不開事件格時退一步：允許經過，讓呼叫端自己處理跳出來的對話框。
        path = bfs(attr, src, dst, set())
        if path is None:
            raise SystemExit(f"{src} 走不到 {dst}")
        print("# 注意：路徑經過其他事件格，中途會跳對話框", file=sys.stderr)
    if args.cells:
        print(to_cells(path, src, args.facing))
    else:
        print(to_keys(path, args.facing, args.wait))


if __name__ == "__main__":
    main()
