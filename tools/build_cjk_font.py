#!/usr/bin/env python3
"""把譯文用到的中文字烘成點陣 atlas。

中文筆畫多，塞不進原版的 8×8 字位（縮到那個尺寸會糊成一團），所以中文走
獨立的高解析點陣層：原版像素層維持 320×200 再整數倍放大，中文直接畫在
放大後的畫布上（見 CLAUDE.md §6）。

預設 24×24 = 原版一個 8×8 字格放大 3 倍，兩者的行距與基準線因此對得齊。

atlas 格式（小端）：

    char[8]   "MM2CJK\\0\\0"
    uint16    glyphW
    uint16    glyphH
    uint32    count
    uint32    codepoints[count]   已排序，供二分搜尋
    bytes     count × (glyphH × ceil(glyphW/8))，每列 MSB 在左

只烘譯文實際用到的字，atlas 才不會塞進整套字庫。譯文增加時重跑即可。

用法：tools/build_cjk_font.py translations/zh-Hant.json assets/font/cjk24.bin [--preview out.png]
"""
import json
import struct
import sys

from PIL import Image, ImageDraw, ImageFont

FONT_CANDIDATES = [
    "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
    "/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc",
    "/usr/share/fonts/truetype/arphic/uming.ttc",
]


def pick_font(size):
    import os
    for path in FONT_CANDIDATES:
        if os.path.exists(path):
            # Noto CJK 的 .ttc 內含多個地區變體，TC 在索引 1 附近；
            # 取不到就退回索引 0，字形仍可用。
            for idx in (1, 0):
                try:
                    return ImageFont.truetype(path, size, index=idx), path, idx
                except Exception:
                    continue
    raise SystemExit("找不到可用的 CJK 字型，請安裝 fonts-noto-cjk")


def render(font, ch, w, h):
    """渲染單一字元成二值點陣。mode='1' 關掉 anti-aliasing，
    邊緣才不會出現半調子灰階 —— 點陣層只有亮/暗兩種狀態。"""
    img = Image.new("1", (w, h), 0)
    d = ImageDraw.Draw(img)
    d.fontmode = "1"
    bbox = d.textbbox((0, 0), ch, font=font)
    x = (w - (bbox[2] - bbox[0])) // 2 - bbox[0]
    y = (h - (bbox[3] - bbox[1])) // 2 - bbox[1]
    d.text((x, y), ch, font=font, fill=1)
    return img


def main():
    src, out = sys.argv[1], sys.argv[2]
    preview = None
    if "--preview" in sys.argv:
        preview = sys.argv[sys.argv.index("--preview") + 1]
    size = 24

    entries = json.load(open(src))
    chars = set()
    for e in entries:
        for ch in e.get("target", ""):
            if ord(ch) > 0x7F:
                chars.add(ch)
    codepoints = sorted(ord(c) for c in chars)
    if not codepoints:
        raise SystemExit("譯文裡沒有非 ASCII 字元，還沒有東西可以烘")

    font, path, idx = pick_font(size)
    rowBytes = (size + 7) // 8
    blob = bytearray()
    for cp in codepoints:
        img = render(font, chr(cp), size, size)
        px = img.load()
        for y in range(size):
            for bx in range(rowBytes):
                b = 0
                for bit in range(8):
                    x = bx * 8 + bit
                    if x < size and px[x, y]:
                        b |= 0x80 >> bit
                blob.append(b)

    header = b"MM2CJK\0\0" + struct.pack("<HHI", size, size, len(codepoints))
    header += struct.pack("<%dI" % len(codepoints), *codepoints)
    import os
    os.makedirs(os.path.dirname(out), exist_ok=True)
    open(out, "wb").write(header + bytes(blob))
    print("字型 %s (index %d)" % (path, idx))
    print("烘出 %d 個字，%d×%d，%d bytes -> %s"
          % (len(codepoints), size, size, len(header) + len(blob), out))

    if preview:
        cols = 24
        rows = (len(codepoints) + cols - 1) // cols
        sheet = Image.new("1", (cols * size, rows * size), 0)
        for i, cp in enumerate(codepoints):
            sheet.paste(render(font, chr(cp), size, size),
                        ((i % cols) * size, (i // cols) * size))
        sheet.convert("L").resize((cols * size * 2, rows * size * 2), Image.NEAREST).save(preview)
        print("預覽 ->", preview)


if __name__ == "__main__":
    main()
