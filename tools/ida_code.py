#!/usr/bin/env python3
"""先在指定位址強制建碼，再把要看的範圍 dump 成 JSON。IDAPython，headless。

    tools/ida.sh idapy ida_code.py EGA.DRV.i64 out entries=442,89C,1333 dump=1333+80

`-Tbinary` 載進來的 `.COM`／裸機器碼**預設全是資料** —— IDA 沒有進入點可跟，
不種進入點就 dump，拿到的是一堆 `dq`，而那與「這段真的不是程式碼」
長得一模一樣。進入點從跳表算：每筆 `E9 rel16`，目標 = 該筆位址 + 3 + rel16。

段的定址寬度也要自己設成 16（見下），否則反出來的是 32-bit 指令。

**建碼與 dump 一定要同一次執行。** `ida_pro.qexit()` 不寫回資料庫，
分兩次跑會得到「第一次說建碼成功、第二次仍是資料」這種自相矛盾的結果。
"""

import json
import sys

import ida_auto
import ida_bytes
import ida_funcs
import ida_pro
import ida_segment
import idautils
import idc


def main() -> None:
    out_path = sys.argv[1]
    entries, dumps = [], []
    for a in sys.argv[2:]:
        if a.startswith("entries="):
            entries = [int(x, 16) for x in a[8:].split(",") if x]
        elif a.startswith("dump="):
            dumps.append(a[5:])

    # **段的定址寬度要自己設成 16。** `-p8086` 只選處理器模組，
    # IDA 9.4 把 `-Tbinary` 建出來的段預設成 32-bit —— 於是
    # `b8 0d 00 cd 10`（`mov ax,0Dh; int 10h`）會反成一條
    # `mov eax, 10CD000Dh`。**反出來的東西看起來完全合理，沒有症狀**，
    # 只是每一條指令的長度都錯，後面整段跟著錯位。
    for i in range(ida_segment.get_segm_qty()):
        ida_segment.set_segm_addressing(ida_segment.getnseg(i), 0)

    ida_auto.auto_wait()
    made = 0
    for ea in entries:
        ida_bytes.del_items(ea, ida_bytes.DELIT_EXPAND, 1)
        if idc.create_insn(ea):
            made += 1
        ida_funcs.add_func(ea)
    ida_auto.auto_wait()

    rows = []
    for spec in dumps:
        start, _, length = spec.partition("+")
        ea, end = int(start, 16), int(start, 16) + int(length or "40", 16)
        while ea < end:
            rows.append({
                "ea": "%06X" % ea,
                "func": ida_funcs.get_func_name(ea) or "",
                "insn": idc.GetDisasm(ea),
                "drefs": ["%06X" % d for d in idautils.DataRefsFrom(ea)],
            })
            nxt = idc.next_head(ea, end)
            ea = nxt if nxt != idc.BADADDR and nxt > ea else ea + 1

    json.dump({"entries": len(entries), "made": made, "rows": rows},
              open(out_path, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    ida_pro.qexit(0)


main()
