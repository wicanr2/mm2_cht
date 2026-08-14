#!/usr/bin/env python3
"""掃全段指令，找出運算元位移落在指定 DGROUP 範圍的每一條。IDAPython，headless。

    tools/ida.sh idapy ida_dsuse.py 2PLAY.img.i64 use-2PLAY 59C0-59D5 16DA

**為什麼不能用 xref 圖**：IDA 只替能解析成單一位址的參考建 data xref。
`mov al, [bx+59CAh]` 這種 **`[reg+disp]` 形式基底暫存器未知，IDA 不建 xref** ——
`DataRefsFrom` 回空。於是「查 xref 找誰寫這個變數」在 16-bit C 程式上是
**結構性的盲區**：陣列存取幾乎全是這個形式，掃出來會是零，
而零命中與「真的沒人碰」長得一模一樣。

正確的問法是掃**運算元的位移值**（`get_operand_value`），
IDA 反組譯時已經算好了，不必解析文字（助憶碼後面補的是多個空格，
而 `push` 的第 0 個運算元是來源不是目的 —— 這兩種自己算的寫法都判錯過）。

讀寫也不自己猜：用 IDA 自己的指令特徵 `CF_CHG<n>`／`CF_USE<n>`，
它標的是「這個運算元會不會被改寫」。
"""

import json
import sys

import ida_auto
import ida_bytes
import ida_funcs
import ida_idp
import ida_pro
import ida_segment
import ida_ua
import idc

CHG = [ida_idp.CF_CHG1, ida_idp.CF_CHG2, ida_idp.CF_CHG3,
       ida_idp.CF_CHG4, ida_idp.CF_CHG5, ida_idp.CF_CHG6]
USE = [ida_idp.CF_USE1, ida_idp.CF_USE2, ida_idp.CF_USE3,
       ida_idp.CF_USE4, ida_idp.CF_USE5, ida_idp.CF_USE6]

# 只看會帶位移的記憶體運算元型別。o_imm 要另外算，因為 `mov ax, 59CAh`
# 是「取位址」而不是存取，兩者的意義不同，混在一起會把常數誤判成存取。
MEM_TYPES = {ida_ua.o_mem, ida_ua.o_displ, ida_ua.o_phrase}


def main() -> None:
    ida_auto.auto_wait()
    out_path = sys.argv[1]
    wanted = []
    for spec in sys.argv[2:]:
        lo, _, hi = spec.partition("-")
        wanted.append((int(lo, 16), int(hi or lo, 16)))

    def hit(v: int) -> bool:
        return any(lo <= v <= hi for lo, hi in wanted)

    rows = []
    for i in range(ida_segment.get_segm_qty()):
        seg = ida_segment.getnseg(i)
        ea = seg.start_ea
        while ea < seg.end_ea:
            if ida_bytes.is_code(ida_bytes.get_flags(ea)):
                insn = ida_ua.insn_t()
                if ida_ua.decode_insn(insn, ea):
                    feat = insn.get_canon_feature()
                    for n in range(6):
                        op = insn.ops[n]
                        if op.type == ida_ua.o_void:
                            break
                        val = None
                        if op.type in MEM_TYPES:
                            val = op.addr
                        elif op.type == ida_ua.o_imm:
                            val = op.value
                        if val is None or not hit(val):
                            continue
                        rows.append({
                            "ea": "%06X" % ea,
                            "rt": "%05X" % (ea - 0x10000),
                            "ds": "%04X" % val,
                            "op": n,
                            "optype": int(op.type),
                            "imm": op.type == ida_ua.o_imm,
                            "chg": bool(feat & CHG[n]),
                            "use": bool(feat & USE[n]),
                            "func": ida_funcs.get_func_name(ea) or "",
                            "insn": idc.GetDisasm(ea),
                        })
            nxt = idc.next_head(ea, seg.end_ea)
            ea = nxt if nxt != idc.BADADDR and nxt > ea else ea + 1

    json.dump({"image": idc.get_root_filename(), "rows": rows},
              open(out_path, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    ida_pro.qexit(0)


main()
