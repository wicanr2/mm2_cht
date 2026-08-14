#!/usr/bin/env python3
"""把一段位址的反組譯連同每一條指令的資料參考傾印成 JSON。IDAPython，headless。

    tools/ida.sh idapy ida_dump.py MM2.EXE.i64 dump 15EC0+40 15F80+20

範圍寫成 `起點+長度`（都是十六進位的 IDA 位址）。

**每條指令附上它的 data ref 目標位址**，這是這支腳本存在的理由：
16-bit 的反組譯文字寫的是 `ds:59CA` 這種 `segment:offset`，
光看文字不知道 IDA 把它解析到哪一個線性位址 —— 而要跨映像比對就得知道。
與其猜 DGROUP 的基底，不如拿一條**已知會碰它**的指令問 IDA。
"""

import json
import sys

import ida_auto
import ida_funcs
import ida_pro
import idautils
import idc

def main() -> None:
    ida_auto.auto_wait()
    out_path = sys.argv[1]
    rows = []
    for spec in sys.argv[2:]:
        start, _, length = spec.partition("+")
        ea = int(start, 16)
        end = ea + int(length or "40", 16)
        while ea < end:
            rows.append({
                "ea": "%06X" % ea,
                "func": ida_funcs.get_func_name(ea) or "",
                "insn": idc.GetDisasm(ea),
                "drefs": ["%06X" % d for d in idautils.DataRefsFrom(ea)],
                "crefs": ["%06X" % c for c in idautils.CodeRefsFrom(ea, False)],
            })
            nxt = idc.next_head(ea, end)
            ea = nxt if nxt != idc.BADADDR and nxt > ea else ea + 1
    json.dump({"image": idc.get_root_filename(), "rows": rows},
              open(out_path, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    ida_pro.qexit(0)


main()
