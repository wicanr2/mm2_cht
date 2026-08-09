#!/usr/bin/env python3
"""從 DOSBox 的記憶體 dump 讀出遊戲執行時才存在的查表。

那些表都在 DGROUP，而 DGROUP 整段是 BSS —— MM2.EXE 檔案裡沒有初值，
只能在遊戲跑起來之後拿。取得 dump 的方式：

    tools/dosbox_run.sh ega "wait:3;key:Return;wait:2;key:s;wait:4;key:g;wait:5;key:z;wait:5;dump:ingame"

一定要先走到第一人稱視角再 dump；標題畫面時那些表還沒填。

用法：
    tools/dump_tables.py workplace/dosbox/shots/ingame.bin
"""
import json
import struct
import sys

# 已知的表：DGROUP 偏移 → (元素數, 元素寬度, 說明)
TABLES = {
    0x1006: (8, 1, "職業表 A，語意未定"),
    0x1012: (8, 1, "每回合攻擊次數的除數，sub_18DAA 用"),
    0x101A: (8, 1, "同段程式碼的第二個除數"),
    0x1022: (8, 1, "職業的位元遮罩（2,4,8,16…）"),
    0x1032: (8, 1, "職業表 D，語意未定"),
    0x10EA: (7, 1, "遭遇的難度門檻，rand(1,100) 落在哪一段"),
    0x10F6: (16, 1, "每類怪物的基礎編號與範圍，四個一組"),
    0x13F6: (32, 1, "32 項表 1，語意未定"),
    0x1416: (32, 1, "32 項表 2，語意未定"),
    0x1436: (32, 1, "32 項表 3，語意未定"),
    0x1456: (32, 1, "32 項表 4，語意未定"),
    0x15E8: (51, 2, "事件腳本的 opcode 長度表"),
    0x164C: (22, 1, "城鎮 0 的設施畫面換算表"),
    0x1662: (22, 1, "城鎮 3 的設施畫面換算表"),
    0x167C: (22, 1, "城鎮 1 的設施畫面換算表"),
    0x1694: (22, 1, "城鎮 4、6 的設施畫面換算表"),
    0x16AC: (22, 1, "城鎮 2、5 的設施畫面換算表"),
}

# 定位 DGROUP 用的 pattern：opcode 長度表的前十二個值。
#
# 為什麼不用別的錨點：`MM2.EXE` 這個字串在 dump 裡不只一處（算出來的
# DGROUP 讀到的是字型點陣），地圖資料在讀檔緩衝裡也有一份。
# 拿表自己當 pattern 才唯一。
ANCHOR_OFFSET = 0x15E8  # 由字串指標表 ds:10AA 校準：30/30 落在 NUL 之後
ANCHOR = struct.pack("<12H", 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 3, 3)


def find_dgroup(mem: bytes) -> int:
    hits = []
    i = 0
    while True:
        i = mem.find(ANCHOR, i)
        if i < 0:
            break
        hits.append(i)
        i += 1
    if not hits:
        raise SystemExit("找不到定位用的 pattern —— dump 時可能還沒進到遊戲中")
    if len(hits) > 1:
        print(f"警告：pattern 命中 {len(hits)} 處，取第一個", file=sys.stderr)
    return hits[0] - ANCHOR_OFFSET


def main() -> None:
    if len(sys.argv) < 2:
        raise SystemExit(__doc__)
    mem = open(sys.argv[1], "rb").read()
    dg = find_dgroup(mem)
    print(f"DGROUP = {dg:#x}（dump 內偏移）\n")
    out = {}
    for off, (count, width, note) in sorted(TABLES.items()):
        base = dg + off
        if width == 1:
            vals = list(mem[base:base + count])
        else:
            vals = [int.from_bytes(mem[base + i * 2:base + i * 2 + 2], "little")
                    for i in range(count)]
        out[f"{off:04X}"] = {"note": note, "values": vals}
        head = vals if len(vals) <= 16 else vals[:16] + ["…"]
        print(f"  ds:{off:04X}  {note}")
        print(f"           {head}")
    path = "docs/re/dgroup-tables.json"
    json.dump(out, open(path, "w"), ensure_ascii=False, indent=1)
    print(f"\n已寫入 {path}")


if __name__ == "__main__":
    main()
