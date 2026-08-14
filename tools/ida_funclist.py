#!/usr/bin/env python3
"""列出資料庫裡的全部函式：位址、名稱、大小。IDAPython，headless。

    tools/ida.sh idapy ida_funclist.py 2CMDS.img.i64 funcs-2CMDS

**不要從 `.asm` 找函式位址**：IDA 的匯出在函式標頭那幾行不帶位址欄，
grep 到的行號不是位址，而「行號看起來像位址」是會一路錯下去的那種錯。
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
    rows = []
    for ea in idautils.Functions():
        f = ida_funcs.get_func(ea)
        rows.append({
            "ea": "%06X" % ea,
            "rt": "%05X" % (ea - 0x10000),
            "name": idc.get_func_name(ea),
            "size": f.end_ea - f.start_ea if f else 0,
        })
    json.dump({"image": idc.get_root_filename(), "funcs": rows},
              open(sys.argv[1], "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    ida_pro.qexit(0)


main()
