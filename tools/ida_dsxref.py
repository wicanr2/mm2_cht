#!/usr/bin/env python3
"""逐位元組查一段 DGROUP 位址的交叉參考，輸出 JSON。IDAPython，headless。

    tools/ida.sh idapy ida_dsxref.py MM2.EXE.i64 dsxref-MM2 59C0-59DF 16D0-16E0

為什麼要逐位元組：**結構化的記憶體通常只有起點有名字**，中間的
`base + i*stride` 被 IDA 標成對某個無名位址的參考，查基址什麼都查不到。
`get_name_ea_simple` 那種寫法只能查具名符號。

為什麼不 grep `.asm`：16-bit 的反組譯文字顯示 `segment:offset`，
**線性位址在整份 `.asm` 裡是零筆**，零命中與「真的沒人碰」長得一模一樣。

位址換算：`build_ovl_image.py` 把每個 overlay 的 image 統一放在 IDA base
`0x1000`，所以 **IDA 位址 = 0x10000 + 執行時偏移**。`ds:59CA` 這種參考
在單段模型下會落在 `0x159CA` —— 每個映像都用同一套別名，
所以同一支腳本可以掃過全部 15 個資料庫再合併。

讀寫判定一律問 `xref.type`（IDA 建庫時就標好了）。**不要自己解析指令文字**：
IDA 的助憶碼後面補的是多個空格，而 `push` 的第 0 個運算元是來源不是目的 ——
這兩種自己算的寫法都實際判錯過。
"""

import json
import sys

import ida_auto
import ida_bytes
import ida_funcs
import ida_name
import ida_pro
import ida_segment
import ida_xref
import idautils
import idc

IMG_BASE = 0x10000

KIND = {
    ida_xref.dr_O: "取址",
    ida_xref.dr_W: "寫",
    ida_xref.dr_R: "讀",
    ida_xref.dr_T: "文字",
    ida_xref.fl_CF: "call far",
    ida_xref.fl_CN: "call near",
    ida_xref.fl_JF: "jmp far",
    ida_xref.fl_JN: "jmp near",
}


def main() -> None:
    ida_auto.auto_wait()
    out_path = sys.argv[1]
    ranges = []
    for spec in sys.argv[2:]:
        lo, _, hi = spec.partition("-")
        ranges.append((int(lo, 16), int(hi or lo, 16)))

    seg = ida_segment.get_first_seg()
    result = {
        "image": idc.get_root_filename(),
        "seg_start": seg.start_ea if seg else None,
        "seg_end": seg.end_ea if seg else None,
        "hits": [],
    }

    for lo, hi in ranges:
        for off in range(lo, hi + 1):
            ea = IMG_BASE + off
            for xref in idautils.XrefsTo(ea, 0):
                frm = xref.frm
                result["hits"].append({
                    "ds": "%04X" % off,
                    "from_ida": "%06X" % frm,
                    # 執行時偏移才是文件裡引用的座標
                    "from_rt": "%05X" % (frm - IMG_BASE),
                    "kind": KIND.get(xref.type, str(xref.type)),
                    "func": ida_funcs.get_func_name(frm) or "",
                    "insn": idc.GetDisasm(frm),
                })

    with open(out_path, "w", encoding="utf-8") as fh:
        json.dump(result, fh, ensure_ascii=False, indent=1)
    ida_pro.qexit(0)


main()
