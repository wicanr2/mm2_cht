#!/usr/bin/env python3
"""在 Mega Drive 的 ROM 裡找未壓縮的 4bpp tile 美術。

    tools/mdscan.py workplace/genesis/*.md            列出候選區塊
    tools/mdscan.py *.md --draw 0x06DA00 out.png      把某一段畫出來

ROM 沒有檔案系統，圖形的位置要自己找。這支用的判準是**列相似度**：

    真正的 tile 美術，同一個 tile 裡相鄰兩列的位元組常常相同
    （色塊、漸層、平坦區）。程式碼與壓縮流不會。

隨機資料的期望值約 6%（1/16）。MM2 這片 ROM 掃出來最高的區塊到 56%，
而且集中在 `0x095000`–`0x0AE000` 與 `0x06D000` 附近。

比「數 nibble 為 0 的比例」準得多 —— 後者在這片 ROM 上分不出程式碼與美術
（整片都落在 12–37%），因為 68000 的指令裡本來就有很多 0 nibble。

## 已知的落點

`0x06DA00` 起有**未壓縮的 4bpp tile 美術**，畫出來讀得到「Might」「and」
「plague」這些字。判準是 ROM 裡**沒有**這些字串（`plague` 找不到、
`Might` 只在 `0x00B577` 的標頭區），所以那是真的字形不是 ASCII 被誤畫。

`0x095000`–`0x0AE000` 列相似度高（22–56%）但畫出來認不出東西。
試過依 ROM 順序排（row-major）與 Mega Drive 精靈慣用的 column-major
（欄高 4 與 6），都還是認不出來。多半是壓縮流裡的長平坦段
（那一帶有大量 `44 44 44` ＝ 色號 4 的實心區）。
"""
import argparse
import glob
import sys


def rows_alike(rom: bytes, off: int, ntiles: int) -> float:
    """一段裡「tile 內相鄰列位元組相同」的比例。"""
    same = tot = 0
    for k in range(ntiles):
        b = off + k * 32
        if b + 32 > len(rom):
            break
        for y in range(7):
            for x in range(4):
                if rom[b + y * 4 + x] == rom[b + (y + 1) * 4 + x]:
                    same += 1
                tot += 1
    return same / tot if tot else 0.0


def draw(rom: bytes, off: int, cols: int, rows: int, path: str) -> None:
    from PIL import Image

    im = Image.new("L", (cols * 8, rows * 8), 0)
    for t in range(cols * rows):
        b = off + t * 32
        tx, ty = (t % cols) * 8, (t // cols) * 8
        for y in range(8):
            for x in range(0, 8, 2):
                k = b + y * 4 + x // 2
                if k >= len(rom):
                    im.save(path)
                    return
                v = rom[k]
                im.putpixel((tx + x, ty + y), (v >> 4) * 17)
                im.putpixel((tx + x + 1, ty + y), (v & 0xF) * 17)
    im = im.resize((im.width * 3, im.height * 3), Image.NEAREST)
    im.save(path)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("rom")
    ap.add_argument("--draw", help="把這個位址起的區塊畫成 PNG（十六進位）")
    ap.add_argument("--out", default="md.png")
    ap.add_argument("--cols", type=int, default=40)
    ap.add_argument("--rows", type=int, default=24)
    ap.add_argument("--window", type=lambda s: int(s, 0), default=0x1000)
    ap.add_argument("--top", type=int, default=20)
    a = ap.parse_args()

    paths = glob.glob(a.rom)
    if not paths:
        raise SystemExit(f"找不到 {a.rom}")
    rom = open(paths[0], "rb").read()

    if a.draw:
        draw(rom, int(a.draw, 0), a.cols, a.rows, a.out)
        print(f"畫到 {a.out}")
        return

    best = []
    for off in range(0, len(rom) - a.window, a.window):
        best.append((rows_alike(rom, off, a.window // 32), off))
    best.sort(reverse=True)
    print(f"{paths[0]}：{len(rom)} bytes，視窗 {a.window}")
    print("列相似度最高的區塊（隨機資料約 6%）：")
    for r, o in best[: a.top]:
        print(f"   0x{o:06X}  {r * 100:5.1f}%")


if __name__ == "__main__":
    sys.exit(main())
