#!/usr/bin/env python3
"""產生 remake 的圖示（原創美術，不取自原版素材）。

主題取書名「Gates to Another World」：一道石門，門裡是另一個世界。
色票沿用推廣片那套 EGA 色（`docs/promo.md`），所以圖示與影片是同一個調子。

**這是我們自己畫的幾何圖形**，沒有一個像素來自 `.16` 影像集 ——
公開包可以帶著它走，原版美術不行。

    docker run --rm -v "$PWD:/w" -w /w mm2-pil:pillow11 python3 tools/make_icon.py

輸出 `assets/icon/`：`mm2-{16,32,64,128,256,512}.png`、`mm2.ico`、`mm2.icns`。
"""
import math
import os
import struct

from PIL import Image, ImageDraw

OUT = "assets/icon"
S = 512  # 主稿邊長，其餘尺寸由它縮出來

BLACK = (0x09, 0x06, 0x06)
DARK = (0x2A, 0x0A, 0x0A)
RED = (0x97, 0x04, 0x04)
DEEPRED = (0x5B, 0x04, 0x03)
BLUE = (0x2F, 0x46, 0xBF)
LIGHT = (0x50, 0x5A, 0xF3)
PALE = (0xAF, 0xDD, 0xEF)
AMBER = (0xA7, 0x55, 0x05)


def lerp(a, b, t):
    return tuple(int(round(x + (y - x) * t)) for x, y in zip(a, b))


def draw() -> Image.Image:
    im = Image.new("RGBA", (S, S), BLACK + (255,))
    d = ImageDraw.Draw(im)

    # 背景：由下往上稍微亮一點，讓門有站在地面上的感覺。
    for y in range(S):
        d.line([(0, y), (S, y)], fill=lerp(BLACK, DARK, y / S))

    # 門洞：上半是半圓，下半是矩形。先畫門裡的另一個世界。
    m = S * 0.18          # 左右留白
    top = S * 0.20        # 拱頂
    base = S * 0.86       # 地面
    w = S - 2 * m
    r = w / 2
    cx = S / 2
    arch_c = top + r

    portal = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    pd = ImageDraw.Draw(portal)
    pd.pieslice([m, top, m + w, top + w], 180, 360, fill=BLUE + (255,))
    pd.rectangle([m, arch_c, m + w, base], fill=BLUE + (255,))
    # 門裡由下往上亮起來：地平線的光。
    grad = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    gd = ImageDraw.Draw(grad)
    for y in range(int(top), int(base) + 1):
        t = (y - top) / (base - top)
        gd.line([(0, y), (S, y)], fill=lerp(BLUE, LIGHT, t ** 2) + (255,))
    im.paste(grad, (0, 0), portal)

    # 門裡的星子：固定座標，不用亂數，重跑要一模一樣。
    stars = [(0.42, 0.34), (0.55, 0.29), (0.63, 0.40), (0.37, 0.47),
             (0.50, 0.44), (0.59, 0.53), (0.44, 0.58)]
    for i, (sx, sy) in enumerate(stars):
        px, py = sx * S, sy * S
        rad = 2.5 + (i % 3)
        d.ellipse([px - rad, py - rad, px + rad, py + rad], fill=PALE + (255,))

    # 門框：外圈深紅、內圈亮紅，寬度用同心的兩層畫出來。
    def frame(pad, colour, width):
        pd_ = [m - pad, top - pad, m + w + pad, top + w + pad]
        d.arc(pd_, 180, 360, fill=colour, width=width)
        d.line([(m - pad, arch_c), (m - pad, base)], fill=colour, width=width)
        d.line([(m + w + pad, arch_c), (m + w + pad, base)], fill=colour, width=width)

    frame(S * 0.055, DEEPRED, int(S * 0.075))
    frame(S * 0.012, RED, int(S * 0.030))

    # 門楣上的拱心石。
    kw, kh = S * 0.10, S * 0.075
    d.polygon([(cx - kw / 2, top - S * 0.055), (cx + kw / 2, top - S * 0.055),
               (cx + kw * 0.34, top - S * 0.055 - kh), (cx - kw * 0.34, top - S * 0.055 - kh)],
              fill=RED)

    # 地面：門站在一條土色的線上，順便把門洞的下緣收掉。
    d.rectangle([0, base, S, S], fill=AMBER)
    d.rectangle([0, base + S * 0.045, S, S], fill=lerp(AMBER, BLACK, 0.55))

    # 門的兩側各切一段陰影，讓石材看起來有厚度。
    d.line([(m - S * 0.09, arch_c - S * 0.02), (m - S * 0.09, base)],
           fill=lerp(DEEPRED, BLACK, 0.45), width=int(S * 0.02))
    d.line([(m + w + S * 0.09, arch_c - S * 0.02), (m + w + S * 0.09, base)],
           fill=lerp(DEEPRED, BLACK, 0.45), width=int(S * 0.02))
    return im


def icns(sizes: dict) -> bytes:
    """最小可用的 ICNS：每個尺寸塞一張 PNG。

    容器是 `icns` + 總長度，接著一連串 `type + 長度(含表頭) + 資料`。
    """
    types = {32: b"ic11", 64: b"ic12", 128: b"ic07", 256: b"ic08", 512: b"ic09"}
    body = b""
    for px, blob in sorted(sizes.items()):
        if px not in types:
            continue
        body += types[px] + struct.pack(">I", len(blob) + 8) + blob
    return b"icns" + struct.pack(">I", len(body) + 8) + body


def main() -> None:
    os.makedirs(OUT, exist_ok=True)
    master = draw()
    blobs = {}
    for px in (16, 32, 64, 128, 256, 512):
        im = master.resize((px, px), Image.LANCZOS)
        path = f"{OUT}/mm2-{px}.png"
        im.save(path)
        with open(path, "rb") as f:
            blobs[px] = f.read()
    master.resize((256, 256), Image.LANCZOS).save(
        f"{OUT}/mm2.ico", sizes=[(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)])
    with open(f"{OUT}/mm2.icns", "wb") as f:
        f.write(icns(blobs))
    print("[icon] 已寫出", ", ".join(sorted(os.listdir(OUT))))


if __name__ == "__main__":
    main()
