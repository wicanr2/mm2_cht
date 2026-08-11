#!/usr/bin/env python3
"""從 Amiga 的 `.adf` 磁片映像抽檔，**含子目錄**。

    tools/adf.py amiga/Disk1.adf workplace/amiga/disk1

檔案系統是 OFS（Old File System，880K 磁片、512 bytes 一個區塊、
根區塊在 880）。先前只抽了根目錄，於是 `libs/` 底下的東西整批漏掉 ——
而 MM2 的檔案 I/O 與解碼走的是 NWC 自己的 `udos.library`，就住在那裡。
**「檔案清單看起來很完整」不等於抽完了**，子目錄不會自己出現。

## 區塊格式（只列這支用到的）

目錄／檔案標頭區塊（512 bytes）：

	+0    type          2 = T_HEADER
	+4    own key
	+8    high_seq      OFS：這個標頭掛了幾個資料區塊
	+16   first_data
	+24   資料區塊表（**倒序**）／目錄的雜湊表（72 格）
	+324  byte_size     檔案長度
	+432  name_len，之後是名字
	+496  hash_chain    同一格雜湊的下一個
	+504  extension     下一個延伸標頭
	+508  sec_type      -3 = 檔案，2 = 目錄

OFS 的資料區塊前 24 bytes 是標頭，實際內容只有 488 bytes ——
**不能整塊當資料抄**，抄了會每 512 bytes 混進 24 bytes 的垃圾，
而症狀是「檔案長度對，內容有規律的雜訊」。
"""
import os
import struct
import sys

BSIZE = 512


def be32(b, off):
    return struct.unpack_from(">I", b, off)[0]


def name_of(b):
    n = b[432]
    return b[433:433 + n].decode("latin-1")


def read_file(img, hdr_blk):
    """把一個檔案標頭（含延伸標頭）的資料串起來。"""
    out = bytearray()
    size = be32(hdr_blk, 324)
    blk = hdr_blk
    while True:
        n = be32(blk, 8)
        # 資料區塊表是倒序的：第 1 個資料區塊在表的最後一格。
        for i in range(n):
            ptr = be32(blk, 24 + 4 * (72 - 1 - i))
            db = img[ptr * BSIZE:(ptr + 1) * BSIZE]
            out += db[24:24 + be32(db, 12)]
        ext = be32(blk, 504)
        if not ext:
            break
        blk = img[ext * BSIZE:(ext + 1) * BSIZE]
    return bytes(out[:size])


def walk(img, dir_blk, out_dir, depth=0):
    os.makedirs(out_dir, exist_ok=True)
    n = 0
    for i in range(72):
        ptr = be32(dir_blk, 24 + 4 * i)
        while ptr:
            blk = img[ptr * BSIZE:(ptr + 1) * BSIZE]
            sec = struct.unpack_from(">i", blk, 508)[0]
            nm = name_of(blk)
            if sec == 2:                      # 目錄
                n += walk(img, blk, os.path.join(out_dir, nm), depth + 1)
            elif sec == -3:                   # 檔案
                data = read_file(img, blk)
                with open(os.path.join(out_dir, nm), "wb") as f:
                    f.write(data)
                print(f"  {'  ' * depth}{nm}  {len(data)}")
                n += 1
            ptr = be32(blk, 496)              # 同一格雜湊的下一個
    return n


def main() -> None:
    if len(sys.argv) < 3:
        raise SystemExit(__doc__)
    img = open(sys.argv[1], "rb").read()
    root = img[880 * BSIZE:881 * BSIZE]
    if img[:3] != b"DOS":
        print("警告：開頭不是 DOS\\0，可能不是 OFS 磁片", file=sys.stderr)
    print(f"{sys.argv[1]}：")
    n = walk(img, root, sys.argv[2])
    print(f"共 {n} 個檔")


if __name__ == "__main__":
    main()
