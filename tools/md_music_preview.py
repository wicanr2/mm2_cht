#!/usr/bin/env python3
"""把擷取到的曲目串成一段試聽帶，方便一次聽完指派音樂包角色。

為什麼需要人來聽：曲目位址是解出來的（18 首，證據見
docs/research/md-music-driver.md），但「哪一首是城鎮、哪一首是戰鬥」要追
0x0B620 那支分派函式的呼叫端所屬函式才知道，目前未解。呼叫端附近沒有
字串引用可以反推，所以在解出來之前，角色指派只能由人聽了決定 ——
不從曲長或檔案順序去猜，那是發明事實。

輸出一個 WAV：每首取前 N 秒，中間插一段靜音，並印出對照表。

    md_music_preview.py <音樂目錄> <輸出.wav> [每首秒數]
"""

from __future__ import annotations

import os
import struct
import sys


def read_wav(path: str) -> tuple[int, int, int, bytes]:
    with open(path, "rb") as fh:
        d = fh.read()
    if d[0:4] != b"RIFF" or d[8:12] != b"WAVE":
        raise SystemExit(f"{path} 不是 RIFF/WAVE")
    pos, fmt, data = 12, None, None
    while pos + 8 <= len(d):
        cid = d[pos : pos + 4]
        size = struct.unpack_from("<I", d, pos + 4)[0]
        if cid == b"fmt ":
            fmt = d[pos + 8 : pos + 8 + size]
        elif cid == b"data":
            data = d[pos + 8 : pos + 8 + size]
        pos += 8 + size + (size & 1)
    if fmt is None or data is None:
        raise SystemExit(f"{path} 缺 fmt 或 data")
    _tag, channels, rate = struct.unpack_from("<HHI", fmt, 0)
    bits = struct.unpack_from("<H", fmt, 14)[0]
    return channels, rate, bits, data


def main() -> None:
    if len(sys.argv) < 3:
        raise SystemExit("用法: md_music_preview.py <音樂目錄> <輸出.wav> [每首秒數]")
    src_dir, out_path = sys.argv[1], sys.argv[2]
    per = float(sys.argv[3]) if len(sys.argv) > 3 else 8.0

    names = sorted(f for f in os.listdir(src_dir) if f.endswith(".wav") and f != os.path.basename(out_path))
    if not names:
        raise SystemExit(f"{src_dir} 裡沒有 .wav")

    pieces: list[bytes] = []
    ref: tuple[int, int, int] | None = None
    print(f"{'序':>2}  {'曲目':<10} {'起點':>9}  {'長度':>7}")
    for i, name in enumerate(names, 1):
        ch, rate, bits, data = read_wav(os.path.join(src_dir, name))
        if ref is None:
            ref = (ch, rate, bits)
        elif (ch, rate, bits) != ref:
            raise SystemExit(f"{name} 的格式與前面不同，不串接")
        frame = ch * bits // 8
        total = len(data) // frame
        take = min(total, int(per * rate))
        pieces.append(data[: take * frame])
        # 一秒靜音當分隔，聽的時候才分得出換曲
        pieces.append(b"\x00" * (rate * frame))
        song = name.removeprefix("md-").removesuffix(".wav")
        print(f"{i:>2}  {name:<10} 0x{song:>7}  {total / rate:>6.1f}秒")

    ch, rate, bits = ref  # type: ignore[misc]
    body = b"".join(pieces)
    align = ch * bits // 8
    header = b"".join(
        [
            b"fmt ",
            struct.pack("<I", 16),
            struct.pack("<HHIIHH", 1, ch, rate, rate * align, align, bits),
        ]
    )
    chunk = header + b"data" + struct.pack("<I", len(body)) + body
    with open(out_path, "wb") as fh:
        fh.write(b"RIFF" + struct.pack("<I", 4 + len(chunk)) + b"WAVE" + chunk)

    secs = len(body) // align / rate
    print(f"\n試聽帶：{out_path}（{len(names)} 首 × {per:.0f} 秒，共 {secs:.0f} 秒）")
    print("每首之間有一秒靜音；順序與上表相同。")


if __name__ == "__main__":
    main()
