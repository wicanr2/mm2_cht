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
    offsets = read_offsets(raw, count)
    images = []
    for i, base in enumerate(offsets):
        if base + 4 > len(raw):
            break
        w, h = struct.unpack_from("<HH", raw, base)
        end = offsets[i + 1] if i + 1 < len(offsets) else len(raw)
        px = raw[base + 4:end]
        # 影像後面固定空 4 bytes；資料不夠畫滿宣告的寬高就是把非影像的
        # 偏移當成影像了，跳過而不是產生垃圾。
        if w == 0 or h == 0 or (w * h + 1) // 2 > len(px):
            continue
        images.append((w, h, px))
    return raw, count, offsets[0], offsets, images


def read_offsets(raw: bytes, count: int):
    """檔頭有兩種形狀，長度都是 2 + count*4，所以不能靠長度分辨。

    A 型：uint32 offsets[count]         —— TOWNT、DISK、NWCP…
    B 型：uint16 offsetsA[count] + uint16 offsetsB[count] —— MASTER、地形圖

    判準是「當 uint32 讀出來的偏移是否遞增且落在緩衝內」。A 型的檔案在
    uint32 的高位 word 剛好都是 0，B 型讀出來會是幾百萬的巨值。
    """
    u32 = list(struct.unpack_from("<%dI" % count, raw, 2))
    ok = all(0 < u32[i] <= len(raw) for i in range(count)) and \
         all(u32[i] < u32[i + 1] for i in range(count - 1))
    if ok:
        return u32
    a = list(struct.unpack_from("<%dH" % count, raw, 2))
    b = list(struct.unpack_from("<%dH" % count, raw, 2 + count * 2))
    # 兩組偏移在緩衝裡是交錯的，必須合併排序後才能用相鄰值界定影像邊界。
    # 尾端的 0 是空槽。
    return sorted({x for x in a + b if 0 < x <= len(raw) - 4})


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


# ── MONSTERS.16 ────────────────────────────────────────────────────────────
# 唯一一個帶自己索引表的 .16。權威實作在 internal/assets/gfx/monsters.go，
# 那裡有完整的證據鏈；這裡只是拿來一眼看圖的工具。

MON_SLOTS = 75          # 索引表項數，原版一次讀 0x12C bytes
MON_TRANSPARENT = 5     # 背景色


def monster_index(blob: bytes):
    """75 個 uint32；0 是空槽。"""
    return [struct.unpack_from("<I", blob, i * 4)[0] for i in range(MON_SLOTS)]


def monster_slot(index, pic: int) -> int:
    """圖號 → 槽號。原版 sub_6818 遇到空槽會往後掃，到底回繞。"""
    i = (pic - 1) % len(index)
    for _ in range(len(index)):
        if index[i]:
            return i
        i = (i + 1) % len(index)
    return -1


def decode_rle(src: bytes, w: int, h: int) -> bytes:
    """高 nibble + 1 = 長度，低 nibble = 顏色，列優先鋪滿 w×h。"""
    need = w * h
    out = bytearray()
    for b in src:
        out += bytes([b & 0x0F]) * ((b >> 4) + 1)
        if len(out) >= need:
            break
    out = out[:need]
    out += bytes([MON_TRANSPARENT]) * (need - len(out))
    return bytes(out)


def parse_monster(blob: bytes, slot: int):
    """回傳 (影格清單, 動畫表頭原始位元組)。影格是 (x, y, w, h, 索引像素)。"""
    off = monster_index(blob)[slot]
    _, raw = unpack_segment(blob, off)
    n = struct.unpack_from("<H", raw, 0)[0]
    offs = list(struct.unpack_from("<%dH" % n, raw, 2))
    frames = []
    for i, o in enumerate(offs):
        end = offs[i + 1] if i + 1 < len(offs) else len(raw)
        x, y, w, h = raw[o], raw[o + 1], raw[o + 2], raw[o + 3]
        frames.append((x, y, w, h, decode_rle(raw[o + 4:end], w, h)))
    tbl = raw[2 + n * 2:offs[0]]
    return frames, tbl


def monster_sheet(blob: bytes, cols: int = 10) -> bytes:
    """每個非空槽的基準圖排成一張，供肉眼一次檢查全部 59 張。"""
    index = monster_index(blob)
    tiles = [parse_monster(blob, s)[0][0] for s, v in enumerate(index) if v]
    cw, ch = 84, 86
    rows = (len(tiles) + cols - 1) // cols
    W, H = cols * cw, rows * ch
    buf = [[MON_TRANSPARENT] * W for _ in range(H)]
    for n, (_, _, w, h, px) in enumerate(tiles):
        ox, oy = (n % cols) * cw, (n // cols) * ch
        for yy in range(min(h, ch)):
            for xx in range(min(w, cw)):
                buf[oy + yy][ox + xx] = px[yy * w + xx]
    data = b"".join(b"\x00" + bytes(v for c in row for v in EGA[c]) for row in buf)

    def chunk(tag, d):
        return struct.pack(">I", len(d)) + tag + d + struct.pack(">I", zlib.crc32(tag + d))

    return (b"\x89PNG\r\n\x1a\n"
            + chunk(b"IHDR", struct.pack(">IIBBBBB", W, H, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(data, 9))
            + chunk(b"IEND", b""))


def main():
    path, outdir = sys.argv[1], sys.argv[2]
    verbose = "-v" in sys.argv
    os.makedirs(outdir, exist_ok=True)
    stem = os.path.basename(path).split(".")[0]
    blob = open(path, "rb").read()

    if stem.upper() == "MONSTERS":
        index = monster_index(blob)
        live = [s for s, v in enumerate(index) if v]
        print("MONSTERS.16   槽 %d 個（空 %d）" % (len(live), MON_SLOTS - len(live)))
        for s in live:
            frames, tbl = parse_monster(blob, s)
            if verbose:
                print("   槽 %-2d 影格 %-2d 表頭 %s" % (s, len(frames), tbl[:tbl.find(b"\xff")].hex()))
            for i, (x, y, w, h, px) in enumerate(frames):
                packed = bytes((px[k] << 4) | (px[k + 1] if k + 1 < len(px) else 0)
                               for k in range(0, len(px), 2))
                open(os.path.join(outdir, "mon%02d_%02d.png" % (s, i)), "wb").write(
                    to_png(w, h, packed))
        open(os.path.join(outdir, "sheet_MONSTERS.png"), "wb").write(monster_sheet(blob))
        return

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
