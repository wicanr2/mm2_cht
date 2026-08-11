#!/usr/bin/env python3
"""把一批 PNG 排成一張總覽圖。

    tools/sheet.py 輸出.png "workplace/gfx/dos/TOWN_*.png" [...]
        --width 1100 --scale 1 --pad 6 --bg 18,18,24 --key 255,0,255

排法是**書架式**（shelf packing）：照給的順序一列一列擺，擺不下就換行，
每一列的高度取那一列最高的一張。不重新排序 —— 素材的順序本身是資訊
（`TOWN.16` 的 0-3 是正牆、4-7 是左側牆…），照面積重排會把它洗掉。

大小差很多的素材混在一起時，書架式會留下不少空白；那是可以接受的代價，
因為另一個選擇是把每張縮到同一格，而縮放會讓 16 色點陣圖糊掉。
"""
import glob
import sys

from PIL import Image, ImageDraw


def shelf(paths, width, scale, pad, key, bg):
    ims = []
    for p in paths:
        im = Image.open(p).convert("RGB")
        if key is not None:
            # 透空色在單張 PNG 裡是洋紅這類醒目的假色，整片排在一起會
            # 蓋掉素材本身。換成底色 —— 這只影響總覽圖，不動抽出來的檔案。
            #
            # **從邊界往內灌**，不是整張換色：DOS 的透空色是 EGA 8（深灰），
            # 而深灰同時是真的顏色（石頭、鐵門的陰影）。整張換掉會在牆上
            # 挖出洞來，看起來像解碼錯誤。與外界相連的那一塊才是透空。
            px = im.load()
            for x in range(im.width):
                for y in (0, im.height - 1):
                    if px[x, y] == key:
                        ImageDraw.floodfill(im, (x, y), bg)
            for y in range(im.height):
                for x in (0, im.width - 1):
                    if px[x, y] == key:
                        ImageDraw.floodfill(im, (x, y), bg)
        if scale != 1:
            im = im.resize((max(1, int(im.width * scale)), max(1, int(im.height * scale))),
                           Image.NEAREST)
        ims.append(im)
    if not ims:
        raise SystemExit("沒有符合的檔案")

    rows, cur, x, h = [], [], pad, 0
    for im in ims:
        if cur and x + im.width + pad > width:
            rows.append((cur, h))
            cur, x, h = [], pad, 0
        cur.append((x, im))
        x += im.width + pad
        h = max(h, im.height)
    if cur:
        rows.append((cur, h))

    total = pad + sum(h + pad for _, h in rows)
    out = Image.new("RGB", (width, total), bg)
    y = pad
    for row, h in rows:
        for x, im in row:
            # 同一列裡矮的靠下對齊 —— 素材本來就是站在地面上的。
            out.paste(im, (x, y + h - im.height))
        y += h + pad
    return out, len(ims)


def main():
    if len(sys.argv) < 3:
        raise SystemExit(__doc__)
    out = sys.argv[1]
    width, scale, pad, bg = 1100, 1.0, 6, (18, 18, 24)
    key = None
    pats = []
    args = sys.argv[2:]
    i = 0
    while i < len(args):
        a = args[i]
        if a == "--width":
            width = int(args[i + 1])
            i += 2
        elif a == "--scale":
            scale = float(args[i + 1])
            i += 2
        elif a == "--pad":
            pad = int(args[i + 1])
            i += 2
        elif a == "--key":
            key = tuple(int(v) for v in args[i + 1].split(","))
            i += 2
        elif a == "--bg":
            bg = tuple(int(v) for v in args[i + 1].split(","))
            i += 2
        else:
            pats.append(a)
            i += 1

    paths = []
    for p in pats:
        paths.extend(sorted(glob.glob(p)))
    im, n = shelf(paths, width, scale, pad, key, bg)
    im.save(out, optimize=True)
    print("%s：%d 張 → %d×%d" % (out, n, im.width, im.height))


if __name__ == "__main__":
    main()
