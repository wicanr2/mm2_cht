#!/usr/bin/env python3
"""解 MM2 的 .16 圖形檔並輸出 PNG。

檔案本身是一個 LZW 段（見 docs/formats/03-lzw-compression.md）。解開之後：

    uint16  count           影像數
    uint32  offsets[count]  相對解壓緩衝開頭的偏移；offsets[0] 恰等於 2+count*4

    每張影像：uint16 width, uint16 height, 然後 4bpp packed 像素
              （每 byte 兩個像素，高 nibble 在左）

影像資料的結尾與下一個 offset 之間固定空 4 bytes，用途未定；
所以影像邊界一律以 offsets 為準，不用寬高回推。

用法：tools/mm216.py <x.16> <輸出目錄>
"""
import struct
import sys
import zlib
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mm2lzw import unpack_segment

# EGA 預設 16 色。原版開機是否整組換掉調色盤尚未確認，這裡先用標準值。
EGA = [
    (0x00, 0x00, 0x00), (0x00, 0x00, 0xAA), (0x00, 0xAA, 0x00), (0x00, 0xAA, 0xAA),
    (0xAA, 0x00, 0x00), (0xAA, 0x00, 0xAA), (0xAA, 0x55, 0x00), (0xAA, 0xAA, 0xAA),
    (0x55, 0x55, 0x55), (0x55, 0x55, 0xFF), (0x55, 0xFF, 0x55), (0x55, 0xFF, 0xFF),
    (0xFF, 0x55, 0x55), (0xFF, 0x55, 0xFF), (0xFF, 0xFF, 0x55), (0xFF, 0xFF, 0xFF),
]


def parse(blob: bytes):
    _, raw = unpack_segment(blob, 0)
    count = struct.unpack_from("<H", raw, 0)[0]
    offsets = struct.unpack_from("<%dI" % count, raw, 2)
    images = []
    for i, base in enumerate(offsets):
        if base + 4 > len(raw):
            break
        w, h = struct.unpack_from("<HH", raw, base)
        end = offsets[i + 1] if i + 1 < count else len(raw)
        px = raw[base + 4:end]
        images.append((w, h, px))
    return raw, count, offsets[0], offsets, images


def to_png(w: int, h: int, px: bytes, scale: int = 2) -> bytes:
    rows = []
    per_row = (w + 1) // 2
    for y in range(h):
        line = bytearray()
        for x in range(w):
            b = px[y * per_row + x // 2] if y * per_row + x // 2 < len(px) else 0
            idx = (b >> 4) if x % 2 == 0 else (b & 0xF)
            line += bytes(EGA[idx]) * scale
        rows.append(bytes(line))

    raw = b"".join(b"\x00" + r for r in rows for _ in range(scale))

    def chunk(tag, data):
        c = tag + data
        return struct.pack(">I", len(data)) + c + struct.pack(">I", zlib.crc32(c))

    return (b"\x89PNG\r\n\x1a\n"
            + chunk(b"IHDR", struct.pack(">IIBBBBB", w * scale, h * scale, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(raw, 9))
            + chunk(b"IEND", b""))


def main():
    path, outdir = sys.argv[1], sys.argv[2]
    verbose = "-v" in sys.argv
    os.makedirs(outdir, exist_ok=True)
    stem = os.path.basename(path).split(".")[0]
    blob = open(path, "rb").read()
    raw, count, hdrlen, offsets, images = parse(blob)

    bad = [i for i, (w, h, px) in enumerate(images) if (w * h + 1) // 2 > len(px)]
    print("%-13s 解出 %6d bytes  count=%-3d hdrlen=%-3d  影像 %d 張  %s"
          % (os.path.basename(path), len(raw), count, hdrlen, len(images),
             "資料不足: %s" % bad if bad else "全部完整"))
    for i, (w, h, px) in enumerate(images):
        need = (w * h + 1) // 2
        if verbose:
            print("   #%-2d %3d × %-3d  需 %6d  給 %6d  多 %d"
                  % (i, w, h, need, len(px), len(px) - need))
        open(os.path.join(outdir, "%s_%02d.png" % (stem, i)), "wb").write(to_png(w, h, px))


if __name__ == "__main__":
    main()


def contact_sheet(images, cols=9, pad=2, scale=2) -> bytes:
    """把一個 .16 的全部影像排成一張圖，供肉眼一次檢查。"""
    rows = (len(images) + cols - 1) // cols
    cw = max(w for w, _, _ in images) + pad
    ch = max(h for _, h, _ in images) + pad
    W, H = cw * cols, ch * rows
    canvas = [[0] * W for _ in range(H)]
    for i, (w, h, px) in enumerate(images):
        ox, oy = (i % cols) * cw, (i // cols) * ch
        per_row = (w + 1) // 2
        for y in range(h):
            for x in range(w):
                k = y * per_row + x // 2
                if k >= len(px):
                    continue
                b = px[k]
                canvas[oy + y][ox + x] = (b >> 4) if x % 2 == 0 else (b & 0xF)
    raw = b""
    for row in canvas:
        line = b"".join(bytes(EGA[v]) * scale for v in row)
        raw += (b"\x00" + line) * scale

    def chunk(tag, data):
        c = tag + data
        return struct.pack(">I", len(data)) + c + struct.pack(">I", zlib.crc32(c))

    return (b"\x89PNG\r\n\x1a\n"
            + chunk(b"IHDR", struct.pack(">IIBBBBB", W * scale, H * scale, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(raw, 9))
            + chunk(b"IEND", b""))
