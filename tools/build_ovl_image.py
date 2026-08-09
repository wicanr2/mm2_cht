#!/usr/bin/env python3
"""重建 MM2 執行時的記憶體佈局，讓 IDA 能正確反組譯 overlay。

MM2 的 root image 與全部 overlay 共用同一個段基準，near call 會直接跨越
root 與 overlay 的界線（例如 1MENU2 的進入點第一件事就是 call 進 root）。
把 .OVL 單獨丟給 IDA 會算錯每一個 near call 的位移，且 thunk 給的進入點
座標（= 載入段*16 + 偏移）也對不上。

所以替每個 overlay 組一份 image：

    0x0000            root image（MM2.EXE 去掉 MZ header）
    load_seg*16       該 overlay 的內容
    其餘              填 0

段基準統一用 IDA base 0x1000，於是 IDA 位址 = 0x10000 + 執行時偏移，
thunk 表的 target 可以直接當進入點用。

用法：tools/build_ovl_image.py <原版 MM2 目錄> <輸出目錄>
"""
import os
import sys

SEG2 = 0x73F0
DESC = SEG2 + 6
THUNK = 0x758A


def parse(exe: bytes):
    def w(off):
        return int.from_bytes(exe[off:off + 2], "little")

    def name(off):
        return exe[SEG2 + off:exe.index(b"\x00", SEG2 + off)].decode()

    ovls = {}
    for i in range(14):
        b = DESC + i * 16
        ovls[i + 1] = dict(name=name(w(b + 8)), load_seg=w(b + 4),
                           end_seg=w(b + 6), para=w(b + 14))

    entries = {}
    p = THUNK
    while p + 12 <= len(exe) and exe[p] == 0x9A:
        entries.setdefault(exe[p + 5], set()).add(int.from_bytes(exe[p + 8:p + 10], "little"))
        p += 12
    return ovls, entries


def main():
    srcdir, outdir = sys.argv[1], sys.argv[2]
    exe = open(os.path.join(srcdir, "MM2.EXE"), "rb").read()
    hdr = int.from_bytes(exe[8:10], "little") * 16
    cp = int.from_bytes(exe[4:6], "little")
    cblp = int.from_bytes(exe[2:4], "little")
    root_len = (cp - 1) * 512 + (cblp or 512) - hdr
    root = exe[hdr:hdr + root_len]

    ovls, entries = parse(exe)
    os.makedirs(outdir, exist_ok=True)
    os.makedirs(os.path.join(outdir, "idc"), exist_ok=True)

    # level-2 overlay（載入段 0x0C13）疊在 level-1 之後，被呼叫時記憶體裡通常還有
    # 一個 level-1 overlay。描述表的 parent 欄位是執行時才填的，靜態讀不到，
    # 所以依檔名前綴推定：1xxx 屬選單階段（parent 1MENU2），2xxx 屬遊戲中（parent 2PLAY）。
    # 【假設待驗】推錯只會讓 level-1 那段解析成雜訊，不影響 overlay 自身。
    by_name = {o["name"].split(".")[0]: o for o in ovls.values()}
    lvl1 = min(o["load_seg"] for o in ovls.values())

    print("root image %d bytes (0x0000-0x%04X)" % (len(root), len(root)))
    for num, o in ovls.items():
        base = o["load_seg"] * 16
        size = o["para"] * 16
        data = open(os.path.join(srcdir, o["name"]), "rb").read()
        stem0 = o["name"].split(".")[0]
        parent = None
        if o["load_seg"] != lvl1:
            parent = by_name["1MENU2" if stem0.startswith("1") else "2PLAY"]
        img = bytearray(max(o["end_seg"], parent["end_seg"] if parent else 0) * 16)
        img[0:len(root)] = root
        if parent:
            pd = open(os.path.join(srcdir, parent["name"]), "rb").read()
            img[parent["load_seg"] * 16:parent["load_seg"] * 16 + len(pd)] = pd
        img[base:base + len(data)] = data

        stem = o["name"].split(".")[0]
        img_path = os.path.join(outdir, "%s.img" % stem)
        open(img_path, "wb").write(img)

        offs = sorted(entries.get(num, ()))
        lines = [
            "#include <idc.idc>",
            "",
            "// %s: root 0x0000-0x%04X + overlay 0x%04X-0x%04X, %d 個進入點"
            % (o["name"], len(root), base, base + size, len(offs)),
            "static main() {",
            "    auto fh, s;",
            "    auto_wait();",
            "    s = get_first_seg();",
            "    set_segm_addressing(s, 0);          // 16-bit",
            "    auto_wait();",
        ]
        for i, off in enumerate(offs):
            ea = 0x10000 + off
            lines.append("    del_items(0x%X, DELIT_EXPAND, 16);" % ea)
            lines.append('    add_entry(0x%X, 0x%X, "%s_e%02d", 1);' % (ea, ea, stem.lower(), i))
        lines += [
            "    auto_wait();",
            '    fh = fopen("/work/out/%s.asm", "w");' % stem,
            "    gen_file(OFILE_ASM, fh, 0, BADADDR, 0);",
            "    fclose(fh);",
            "    qexit(0);",
            "}",
        ]
        open(os.path.join(outdir, "idc", "img_%s.idc" % stem), "w").write("\n".join(lines) + "\n")
        print("%-12s 載入 0x%05X 長 %5d  image %6d bytes  %2d 進入點"
              % (o["name"], base, size, len(img), len(offs)))


if __name__ == "__main__":
    main()
