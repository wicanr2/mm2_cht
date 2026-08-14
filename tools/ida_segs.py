#!/usr/bin/env python3
"""列出資料庫的段配置。IDAPython，headless。

要點：`build_ovl_image.py` 造的 overlay image 是**單一 16-bit 段**，
所以 `ds:XXXX` 會別名到 `0x10000 + XXXX`；而 `MM2.EXE.i64` 是正規 MZ，
**DGROUP 是獨立的段**，同一個別名在那裡不成立。
掃 DS 變數之前一定要先問這個，否則兩邊的座標會混在一起而看不出來。
"""
import json, sys
import ida_auto, ida_pro, ida_segment, idc

ida_auto.auto_wait()
segs = []
for i in range(ida_segment.get_segm_qty()):
    s = ida_segment.getnseg(i)
    segs.append({
        "name": ida_segment.get_segm_name(s),
        "start": "%06X" % s.start_ea,
        "end": "%06X" % s.end_ea,
        "size": s.end_ea - s.start_ea,
        "class": ida_segment.get_segm_class(s) or "",
        "base_para": "%04X" % (ida_segment.get_segm_base(s) >> 4),
    })
json.dump({"image": idc.get_root_filename(), "segs": segs},
          open(sys.argv[1], "w", encoding="utf-8"), ensure_ascii=False, indent=1)
ida_pro.qexit(0)
