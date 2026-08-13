#!/usr/bin/env python3
"""把 vgm2wav 的輸出正規化成標準 PCM WAV。

vgm2wav 寫的是 WAVE_FORMAT_EXTENSIBLE（格式碼 65534、fmt 區塊 40 bytes）。
取樣資料本身就是 16-bit little-endian PCM，但**檔頭不是標準 PCM 的樣子**，
於是 Ebiten 的解碼器會直接拒絕：

    wav: format must be linear PCM

症狀是音樂包在啟動時整包被拒，而檔案本身聽起來完全正常 —— 播放器打得開、
波形也對，所以很容易誤判成 remake 那邊的 bug。

這支只重寫檔頭，不動任何一個取樣點：輸出是 44 bytes 的標準檔頭 ＋ 原封不動的
data 區塊。轉出來的檔案與輸入逐位元組同資料，只有容器描述改了。

    vgm2pcmwav.py <輸入.wav> <輸出.wav>
"""

import struct
import sys


def normalise(src: bytes) -> bytes:
    if src[0:4] != b"RIFF" or src[8:12] != b"WAVE":
        raise SystemExit("不是 RIFF/WAVE 檔")

    fmt = None
    data = None
    pos = 12
    while pos + 8 <= len(src):
        cid = src[pos : pos + 4]
        size = struct.unpack_from("<I", src, pos + 4)[0]
        body = src[pos + 8 : pos + 8 + size]
        if cid == b"fmt ":
            fmt = body
        elif cid == b"data":
            data = body
        pos += 8 + size + (size & 1)

    if fmt is None or data is None:
        raise SystemExit("缺少 fmt 或 data 區塊")

    tag, channels, rate, _byte_rate, _align, bits = struct.unpack_from("<HHIIHH", fmt, 0)

    # EXTENSIBLE 的真正格式藏在 SubFormat GUID 的前兩個位元組。
    # 只接受 PCM(1)；浮點(3) 要另外轉換，不能只改檔頭騙過去。
    if tag == 0xFFFE:
        if len(fmt) < 26:
            raise SystemExit("EXTENSIBLE 的 fmt 區塊過短，讀不到 SubFormat")
        tag = struct.unpack_from("<H", fmt, 24)[0]
    if tag != 1:
        raise SystemExit(f"不是 PCM（格式碼 {tag}），這支只做檔頭正規化")
    if bits not in (8, 16):
        raise SystemExit(f"位元深度 {bits} 不在音樂包允許的 8／16 之內")
    if channels not in (1, 2):
        raise SystemExit(f"聲道數 {channels} 不在音樂包允許的 1／2 之內")

    align = channels * bits // 8
    header = b"".join(
        [
            b"fmt ",
            struct.pack("<I", 16),
            struct.pack("<HHIIHH", 1, channels, rate, rate * align, align, bits),
        ]
    )
    body = header + b"data" + struct.pack("<I", len(data)) + data
    return b"RIFF" + struct.pack("<I", 4 + len(body)) + b"WAVE" + body


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("用法: vgm2pcmwav.py <輸入.wav> <輸出.wav>")
    with open(sys.argv[1], "rb") as fh:
        src = fh.read()
    out = normalise(src)
    with open(sys.argv[2], "wb") as fh:
        fh.write(out)
    print(f"正規化完成：{sys.argv[2]}（{len(out)} bytes）")


if __name__ == "__main__":
    main()
