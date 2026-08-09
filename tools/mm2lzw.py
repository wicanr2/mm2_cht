#!/usr/bin/env python3
"""MM2 的 LZW 解壓器。

演算法讀自 MM2.EXE 的 sub_12242（IDA linear 0x12242）：

    初始碼寬 9，上限 12（cmp var_1C, 0Ch）
    0x100 = CLEAR：碼寬回 9、門檻回 0x200、下一碼回 0x102
    0x101 = EOF
    0x102 起是動態字典項，每筆 3 bytes：word prefix + byte suffix
    位元流 LSB first（lodsw/lodsb 後 shr al,1 / rcr bx,1 位移 dx 次，再套遮罩）
    下一碼達到門檻且碼寬未滿 12 時：碼寬 +1、門檻 <<= 1

字典容量 4096 × 3 bytes = 12,288 = 0x300 段，與 sub_12242 配置的 0x300 相符。

段頭：載入常式先讀 4 bytes，取低 word 當解壓後長度，壓縮流從第 5 個 byte 起。
"""
import sys


class BitReader:
    """LSB-first 位元流。"""

    def __init__(self, data: bytes):
        self.data = data
        self.pos = 0          # 位元位置

    def read(self, width: int) -> int:
        byte_off, bit_off = divmod(self.pos, 8)
        if byte_off + 3 > len(self.data):
            chunk = self.data[byte_off:] + b"\x00" * 3
            v = int.from_bytes(chunk[:3], "little")
        else:
            v = int.from_bytes(self.data[byte_off:byte_off + 3], "little")
        self.pos += width
        return (v >> bit_off) & ((1 << width) - 1)


CLEAR, EOF_CODE, FIRST = 0x100, 0x101, 0x102


def decompress(data: bytes, limit: int | None = None, want_used: bool = False):
    r = BitReader(data)
    out = bytearray()
    prefix = [0] * 4096
    suffix = [0] * 4096

    width, threshold, next_code = 9, 0x200, FIRST
    prev = -1
    last_char = 0

    while True:
        code = r.read(width)
        if code == EOF_CODE:
            break
        if code == CLEAR:
            width, threshold, next_code = 9, 0x200, FIRST
            code = r.read(width)
            if code in (CLEAR, EOF_CODE):
                break
            out.append(code & 0xFF)
            prev = code
            last_char = code & 0xFF
            if limit and len(out) >= limit:
                break
            continue

        cur = code
        stack = []
        if code >= next_code:            # KwKwK
            stack.append(last_char)
            code = prev
        while code > 0xFF:
            stack.append(suffix[code])
            code = prefix[code]
        stack.append(code)
        last_char = code

        out += bytes(reversed(stack))

        if prev >= 0 and next_code < 4096:
            prefix[next_code] = prev
            suffix[next_code] = last_char
            next_code += 1
            if next_code >= threshold and width < 12:
                width += 1
                threshold <<= 1
        prev = cur

        if limit and len(out) >= limit:
            break

    if want_used:
        return bytes(out), (r.pos + 7) // 8
    return bytes(out)


def unpack_segment(blob: bytes, off: int) -> tuple[int, bytes]:
    """解一段：4 bytes 段頭（低 word = 解壓後長度）+ LZW 流。"""
    size = int.from_bytes(blob[off:off + 2], "little")
    return size, decompress(blob[off + 4:], size)


def unpack_all(blob: bytes, off: int = 0):
    """把串接在一起的多個段依序解開，回傳 [(段起點, 宣告長度, 資料), ...]。"""
    segs = []
    while off + 4 < len(blob):
        size = int.from_bytes(blob[off:off + 2], "little")
        if size == 0:
            break
        out, used = decompress(blob[off + 4:], size, want_used=True)
        segs.append((off, size, out))
        if len(out) < size:
            break
        off += 4 + used
    return segs


if __name__ == "__main__":
    path, off = sys.argv[1], int(sys.argv[2], 0)
    blob = open(path, "rb").read()
    size, out = unpack_segment(blob, off)
    sys.stderr.write("段頭宣告 %d bytes，解出 %d bytes\n" % (size, len(out)))
    sys.stdout.buffer.write(out)
