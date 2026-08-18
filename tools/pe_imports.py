#!/usr/bin/env python3
"""列出一個 PE 執行檔的匯入 DLL（不靠外部套件，直接讀結構）。

用途是回答「這個 Windows 包要不要夾帶 DLL」——**要有實測清單才敢下結論**，
不能只 grep 原始碼。輸出一行一個 DLL 名。

用法：pe_imports.py <exe>
"""
import struct
import sys


def u16(b, o):
    return struct.unpack_from("<H", b, o)[0]


def u32(b, o):
    return struct.unpack_from("<I", b, o)[0]


def main() -> None:
    with open(sys.argv[1], "rb") as f:
        b = f.read()
    if b[:2] != b"MZ":
        raise SystemExit("不是 MZ")
    pe = u32(b, 0x3C)
    if b[pe:pe + 4] != b"PE\0\0":
        raise SystemExit("不是 PE")
    machine = u16(b, pe + 4)
    nsec = u16(b, pe + 6)
    opt = pe + 24
    magic = u16(b, opt)
    # PE32+ 的 data directory 起點比 PE32 多 16 bytes（幾個欄位變成 64 位元）。
    dd = opt + (112 if magic == 0x20B else 96)
    # data directory 第 0 筆是輸出表，**第 1 筆才是匯入表**，每筆 8 bytes。
    imp_rva = u32(b, dd + 8)
    sec = pe + 24 + u16(b, pe + 20)
    secs = []
    for i in range(nsec):
        o = sec + i * 40
        secs.append((u32(b, o + 12), u32(b, o + 16), u32(b, o + 20)))  # vaddr, rawsize, rawptr

    def off(rva):
        for va, size, ptr in secs:
            if va <= rva < va + max(size, 1):
                return ptr + (rva - va)
        return None

    names = []
    o = off(imp_rva)
    while o is not None:
        name_rva = u32(b, o + 12)
        if name_rva == 0 and u32(b, o) == 0:
            break
        no = off(name_rva)
        if no is None:
            break
        end = b.index(b"\0", no)
        names.append(b[no:end].decode("ascii"))
        o += 20
    arch = {0x8664: "x86_64", 0x14C: "x86", 0xAA64: "arm64"}.get(machine, hex(machine))
    print(f"# machine={arch} magic={'PE32+' if magic == 0x20B else 'PE32'}")
    for n in sorted(set(names), key=str.lower):
        print(n)


if __name__ == "__main__":
    main()
