#!/usr/bin/env python3
"""解 Mega Drive 版第一人稱視野的**版面**：哪一張貼在哪裡，以及怎麼組成一格畫面。

    tools/mdscene.py <ROM> --sheet workplace/gfx/mdscene   # 每個區域類型的素材總覽
    tools/mdscene.py <ROM> --compose workplace/gfx/mdscene # 照牆值組出整幅視野
    tools/mdscene.py <ROM> --export workplace/md-scene     # 烘成 remake 吃的素材包

貼圖本身的格式（LZSS ＋ 寬高在來源前面）在 `tools/mdview.py`，這一支只管版面。

## 版面怎麼來的

`sub_FC38(區域類型)` 把一整組貼圖指標填進 RAM `0xFFCD40` 的 52 個長字。
使用端（`sub_3DE2` 與 `sub_34E8`）的索引算式都是

    索引 = 牆值 × 0x50 + (位置 − 20) × 4

牆值就是視野格子陣列 `0xFFF3CA` 那一格的 2-bit 值（1–3，0 ＝ 沒有牆），
所以一個牆值佔 20 個位置，整張表 **60 個長字**。執行時的 RAM 傾印顯示
**第 40–59 格與第 0–19 格逐項相同** —— 也就是**牆值 3 用的是牆值 1 的圖**，
差別在另外加畫的火炬（`sub_34E8` 那一路）。`sub_3DE2` 裡二十個貼圖點的目的位址都是立即值，
組合緩衝區從 `0xFF0340` 起、列距 `0x68`（104 bytes ＝ 208 像素 4bpp），
換算出來就是 `PLACE`。

**填表有一個會讓掃描漏東西的寫法**：位置 7、8 與 10、11 不是 `move.l #imm`，
而是 `move.l d16(a5),d16(a5)` 把位置 6、9 抄過去。只掃立即值會得到
「那幾格沒填」，而那與「原版真的沒填」長得一模一樣。

## 二十個位置

四個深度的正面牆各有左／中／右三張（位置 0–11），加上八根 120 高的側牆柱
（位置 12–19）。側牆柱的 x 邊界 0/24/56/80/96 ｜ 112/128/152/184/208 正好是
同深度正牆的左右緣 —— 與 DOS 版「側牆寬度累加 24→56→80→96 等於同深度正牆
左緣」是同一套幾何，只是 Mega Drive 把一整根柱子烘成一張圖。

畫的順序是**由遠而近**（`sub_3DE2` 的程式碼順序）：最遠的正牆 → 最內側的
側牆柱 → 遠 → 次內柱 → 中 → 次外柱 → 近 → 最外柱。照這個順序疊才對，
反過來近的會被遠的蓋掉。

## 驗過了：與執行時的緩衝區逐像素相同

拿 BlastEm 傾印的視野格子陣列餵這一支，組出來的 208×120 與模擬器自己的
組合緩衝區 **24,960 個像素全部相同**（非透空的 9,866 個也全中）。
重跑方式見 `docs/research/02-other-platforms.md`。

**目的位址是線性的，記憶體不是。** `dest − 0xFF0340` 除以 `0x68` 得列、
餘數乘 2 得行 —— 這樣算出來的落點是對的（上面那個 100% 就是證據）。
但緩衝區在 RAM 裡是 **VDP 的 tile 順序**（32 bytes 一個 8×8 tile、26 個一列、
共 16 列），把它當成「每列 104 bytes 的點陣圖」畫出來是雜訊。
兩件事都成立，因為做轉換的是 blitter。
"""

from __future__ import annotations

import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import mdgfx   # noqa: E402
import mdview  # noqa: E402

# 組合緩衝區
BUF = 0xFF0340
STRIDE = 0x68          # bytes/列
VIEW_W = STRIDE * 2    # 208 像素
VIEW_H = 120

# 位置 → (x, y)，由 sub_3DE2 的目的位址立即值換算。
PLACE = [
    (0, 14), (24, 14), (184, 14),          # 0–2   近距 左／正／右
    (0, 32), (56, 32), (152, 32),          # 3–5   中距
    (32, 45), (80, 45), (128, 45),         # 6–8   遠距
    (80, 54), (96, 54), (112, 54),         # 9–11  最遠
    (0, 0), (24, 0), (56, 0), (80, 0),     # 12–15 左側牆柱，近 → 遠
    (112, 0), (128, 0), (152, 0), (184, 0),  # 16–19 右側牆柱，遠 → 近
]

# sub_FC38 開頭那張七格跳表：區域類型 → 三塊表格填法之一。
# 偏移 0x0010／0x01EC／0x03F8 三個目標，所以七種區域類型只有三套素材。
AREA_TYPES = [[0, 1], [2, 5], [3, 4, 6]]

# 地板與天空：`sub_30B4` 依區域類型挑，貼圖點在 `sub_31D0`。
#
#   天空  → 緩衝區 +0x340（視圖第 0 列）
#   地板  → 緩衝區 +0x340 + 61 或 63 列
#
# 有天花板的格子（`sub_3064` 為真）不畫天空，改呼叫 thunk `0x18`
# （ROM `0x29ECC`）把上面 61 列**整片填成索引 1**。
FLOOR = [0x7898C, 0x71378, 0x9D852]
SKY_DAY = [0x70AB0, 0x70AB0, 0x9DF4E]
SKY_NIGHT = 0x6FC6E      # `-$544(a5) > 0x80` 時改用這一張，remake 還沒接
CEIL_FILL_H = 61         # thunk 0x18 填幾列
CEIL_FILL_INDEX = 1

# sub_3BE2 的程式碼順序 ＝ 由遠而近的疊圖順序。
DRAW_ORDER = [9, 10, 11, 15, 16, 6, 7, 8, 14, 17, 3, 4, 5, 13, 18, 0, 2, 1, 12, 19]

# 位置的中文說明，只給總覽圖與報表用。
LABEL = [
    "近·左", "近·正", "近·右", "中·左", "中·正", "中·右",
    "遠·左", "遠·正", "遠·右", "最遠·左", "最遠·正", "最遠·右",
    "左柱·深0", "左柱·深1", "左柱·深2", "左柱·深3",
    "右柱·深3", "右柱·深2", "右柱·深1", "右柱·深0",
]

FC38, FC38_END = mdview.FC38, mdview.FC38_END
TABLE_BASE = -0x35D2

GRAY = [(255, 0, 255)] + [(i * 17, i * 17, i * 17) for i in range(1, 16)]

# ROM 裡的整份 CRAM：`0x6FBEA` 起 **128 bytes ＝ 四條 16 色**，就接在
# 夜空 `0x6FC6E` 的影像頭前面。判準有三條同時成立：
#
#   1. `0x6FBE8`–`0x6FC6C` 這 132 bytes 是整個 `0x6F900`–`0x6FD00` 裡
#      唯一一段值全部符合 `& 0x0EEE` 的連續區，扣掉頭尾對齊剛好 128；
#   2. **程式參考的是 `0x6FBEA` 不是 `0x6FBE8`** —— 全 ROM 掃 32-bit
#      立即值，`0x0006FBEA` 命中八處，`0x0006FBE8` 零命中；
#   3. 那八處都接著 `sub_2DBA(1, …)`（目標 1 ＝ CRAM），長度是
#      `0x20`／`0x40`／`0x60`，也就是一次上傳一到三條。
#
# 視野的顏色永遠上傳到 **CRAM 第 2 條**（nametable 項值 bit14–13 ＝ 10），
# 但**來源是這四條裡的哪一條，由區域類型決定**（`0x8166` 起那一段：
# 依 `-$4C6(a5)` 與 `-$4C4(a5)` 把 32 bytes 從 `0x6FBEA + N×0x20` 抄進工作區）：
#
#     區域類型 0        → 第 1 條
#     區域類型 1        → 第 2 條
#     區域類型 2、5     → 第 0 條
#     戶外（-$4C4 == 1）→ 第 3 條
#
# 執行時的 CRAM 不會等於這一份：原版依**光照**把它調暗（分量 × 亮度 ÷ 8）。
# 實測沒有光源的地城那一幀是亮度 2，也就是分量 >> 2，十六格全中。
# remake 拿沒調暗的原值烘。
ROM_CRAM = 0x6FBEA

# Mega Drive 的 3 位元分量 → 8 位元。**不是線性的 ×36**：拿 BlastEm 的原生
# 截圖逐格反查，實際輸出的八個階是這一組（與一般 Genesis 模擬器用的
# DAC 曲線相同）。用 ×36 畫出來每一格都差幾個階，看起來對、逐像素比就全錯。
MD_RAMP = (0, 49, 87, 119, 146, 174, 206, 255)


def md_rgb(v: int):
    """9-bit BGR（`0000 BBB0 GGG0 RRR0`）→ RGB 0–255。"""
    return (MD_RAMP[(v >> 1) & 7], MD_RAMP[(v >> 5) & 7], MD_RAMP[(v >> 9) & 7])

# 三塊素材各自該用第幾條。第 0 塊同時服務區域類型 0 與 1，**那兩型用的不是
# 同一條**（1 與 2）—— 同一批牆面素材在兩種區域類型下是不同顏色：
# 城鎮（類型 0）是藍紫色的亂石，地城（類型 1）是灰色的。這裡取城鎮那一條。
#
# 實機對照：Middlegate (8,0) 面東是**類型 0**，畫面是藍紫亂石 ＋ 紅褐地磚
# （`workplace/genesis/out/light/04-town-day-screen.png`）。
BLOCK_PAL_LINE = [1, 0, 3]


def _w(rom, p):
    return int.from_bytes(rom[p:p + 2], "big")


def _sw(rom, p):
    return int.from_bytes(rom[p:p + 2], "big", signed=True)


def _l(rom, p):
    return int.from_bytes(rom[p:p + 4], "big")


def area_tables(rom: bytes):
    """走一遍 `sub_FC38`，回傳每個區域類型的 {表格位置: 來源指標}。

    只認三種指令就夠了：push 立即值、pop 進表、表格內互抄。
    抄那一種**一定要處理** —— 位置 7、8、10、11 全靠它。
    """
    out, cur, pend = [], {}, None
    p = FC38
    while p < FC38_END:
        op = _w(rom, p)
        if op == 0x2EBC:                       # move.l #imm32,(sp)
            pend = _l(rom, p + 2)
            p += 6
            continue
        if op == 0x2B5F:                       # move.l (sp)+,d16(a5)
            slot = (_sw(rom, p + 2) - TABLE_BASE) // 4
            if slot == 0 and cur:
                out.append(cur)
                cur = {}
            cur[slot] = pend
            p += 4
            continue
        if op == 0x2B6D:                       # move.l d16(a5),d16(a5)
            src = (_sw(rom, p + 2) - TABLE_BASE) // 4
            dst = (_sw(rom, p + 4) - TABLE_BASE) // 4
            cur[dst] = cur.get(src)
            p += 6
            continue
        p += 2
    if cur:
        out.append(cur)
    return out


def variant_slots(table: dict, variant: int):
    """牆值 variant（1 起算）的二十個位置 → 來源指標，沒有的位置給 None。"""
    base = (variant - 1) * 20
    return [table.get(base + i) for i in range(20)]


def load_images(rom: bytes, srcs):
    """把一組來源指標解成 {位置: (w, h, 4bpp 資料)}。"""
    out = {}
    for i, src in enumerate(srcs):
        if not src:
            continue
        # 來源是從 `sub_FC38` 的表裡讀出來的，不是掃描猜的，所以用 relaxed ——
        # 有一筆（`0x076B6A`）的高度欄位與 `rawSize` 矛盾，以 `rawSize` 為準。
        got = mdview.decode(rom, src, relaxed=True)
        if got:
            out[i] = got
    return out


def to_indexed(w: int, h: int, data: bytes):
    """4bpp packed → (h, w) 的索引陣列。"""
    import numpy as np

    a = np.frombuffer(bytes(data).ljust(h * (w // 2), b"\0"), dtype=np.uint8)
    a = a[:h * (w // 2)].reshape(h, w // 2)
    out = np.empty((h, w), np.uint8)
    out[:, 0::2] = a >> 4
    out[:, 1::2] = a & 15
    return out


def compose(rom: bytes, table: dict, walls, palette=None):
    """照牆值陣列組出一幅視野。walls 是二十格的牆值（0 ＝ 沒有牆）。

    回傳 (h, w) 的索引陣列；索引 0 是透空／背景。
    """
    import numpy as np

    canvas = np.zeros((VIEW_H, VIEW_W), np.uint8)
    for pos in DRAW_ORDER:
        v = walls[pos]
        if not v:
            continue
        src = table.get((v - 1) * 20 + pos) or table.get(pos)
        if not src:
            continue
        got = mdview.decode(rom, src, relaxed=True)
        if not got:
            continue
        w, h, data = got
        im = to_indexed(w, h, data)
        x, y = PLACE[pos]
        ch, cw = min(h, VIEW_H - y), min(w, VIEW_W - x)
        if ch <= 0 or cw <= 0:
            continue
        canvas[y:y + ch, x:x + cw] = im[:ch, :cw]
    return canvas


def rom_palette(rom: bytes, line: int = 0):
    """讀 ROM 內建的那份 CRAM 的第 line 條（16 色）。"""
    return [md_rgb(_w(rom, ROM_CRAM + line * 32 + i * 2)) for i in range(16)]


def read_palette(path: str):
    """讀 BlastEm 傾印的 CRAM（64 個 big-endian 16-bit）→ 四條 16 色的 RGB 表。"""
    b = open(path, "rb").read()
    if len(b) < 128:
        raise SystemExit(f"{path} 只有 {len(b)} bytes，CRAM 要 128")
    return [[md_rgb(int.from_bytes(b[ln * 32 + i * 2:ln * 32 + i * 2 + 2], "big"))
             for i in range(16)] for ln in range(4)]


def save(arr, palette, path, scale=2):
    from PIL import Image

    h, w = arr.shape
    im = Image.new("P", (w, h))
    flat = []
    for c in palette:
        flat += list(c)
    flat += [0] * (768 - len(flat))
    im.putpalette(flat)
    im.putdata(arr.flatten().tolist())
    if scale > 1:
        im = im.resize((w * scale, h * scale), Image.NEAREST)
    im.save(path)


# ── 火炬 ─────────────────────────────────────────────────────────────
#
# 火炬**不進組合緩衝區**。`sub_3836` 掃視野格子陣列，值是 3 的格子就透過
# thunk `0x54`／`0x5A`／`0x4E` 直接改 nametable，所以逐像素比對組合緩衝區
# 與實機截圖時，剩下的那 1.7% 差額正好是火炬。
#
# `sub_3836` 是十八段展平的程式碼（沒有迴圈、沒有表），每一段長這樣：
#
#     格子[N] == 3 且 擋住它的那幾格都是 0
#         → 0x5A(舊畫面, 新畫面, x, y, w, h)          ; 備份這一塊
#           0x54(tile 起點, 0x6000, x, y, w, h, 畫面)  ; 貼上
#
# 那四個數字是 **nametable 的行、列與寬高（單位是 tile）**。定它們的方法是
# 拿三段只寫一個 nametable 字的程式碼當量尺：`0x498`／`0x49A`／`0x49C`／
# `0x4A0` 除以 2 再除以 64 得列 9、行 12／13／14／16 —— 與其他段的 x 對得上。
# 左右鏡射的常數是 **30**（`x' = 30 − w − x`），五對鏡射格子逐項成立。
#
# 視圖佔 nametable 的行 2–27、列 2–16，所以 **視圖座標 = (行 − 2) × 8**。
# 這個換算用實機截圖驗過：格子 0x0D 這一塊算出來是 (32,40) 16×32，
# 而截圖減組合緩衝區量到的可見像素是 (37,45) 8×24 —— 落在那一塊裡面，
# 鏡射的 0x12 也對稱（(160,40) 對 (163,45)）。
TORCH_BLOCK = 0x6D05A     # 區塊表頭；53 個 tile，開機時載到 VRAM tile 0x780
TORCH_PALETTE = 0x6D03C   # 火炬那一條調色盤（16 色），nametable 屬性寫死第 3 條
TORCH_TABLE = 0x0BF4DC    # 火焰的色表，9 個字
TORCH_FLAME = (5, 6, 7)   # 火焰佔的三格
TORCH_FRAMES = 9          # 相位數 ＝ 色表長度

# 火焰動畫是**一條 9 色的表配三個相位差固定的取樣點**，不是三個值輪替
# （`sub_7F78` 室內分支，ROM `0x7F78`）：
#
#     n = (計數器 + 1) mod 9
#     第 5 格 ← T[n]        第 7 格 ← T[(n+1) mod 9]      第 6 格 ← T[(n+3) mod 9]
#
# 值的流向是 **第 6 格 →（兩步）→ 第 7 格 →（一步）→ 第 5 格**。九個相位
# 只用到五種顏色（暗紅、正紅、紅橘、橘、亮黃）。原版一個相位停 5 幀，
# 整圈 45 幀。
#
# `0x6D03C` 這一條的第 5–7 格在 ROM 裡是動畫開始前的初值；其餘 13 格與
# 實機 CRAM 第 3 條逐格相同（BlastEm 存檔挖出來比對過），**所以火炬的
# 顏色整條都讀得出來，不必靠傾印**。這三格也**不吃光照** —— 沒有光源的
# 地城裡其餘全暗，火把照樣是滿亮的表值。
TORCH_PHASE_TAPS = (0, 3, 1)   # 第 5／6／7 格各取 n + 這個位移

# 八張火炬圖：(tile 起點, 寬, 高)，起點是 VRAM tile 減 `0x780`。
# **53 個 tile 剛好被這八張分完**（10+8+4+1+18+8+3+1 = 53）——
# 任何一張的寬高猜錯，總和就對不上，所以這是版面的獨立驗證。
# tile 在每一張裡是 **row-major**（照 col-major 排出來是雜訊）。
TORCH_ART = {
    "front0": (0x00, 2, 5),   # 正牆深度 0
    "front1": (0x0A, 2, 4),   # 正牆深度 1，兼斜前方那兩格
    "front2": (0x12, 2, 2),   # 正牆深度 2，兼更遠的斜前方
    "front3": (0x16, 1, 1),   # 正牆深度 3：一個 tile
    "side0":  (0x17, 3, 6),   # 側牆深度 0
    "side1":  (0x29, 2, 4),   # 側牆深度 1
    "side2":  (0x31, 1, 3),   # 側牆深度 2
    "side3":  (0x34, 1, 1),   # 側牆深度 3：一個 tile
}

# remake 的火炬槽 → (圖, 視野格子, nametable 行列, 要不要左右鏡射)。
#
# 槽號沿用 DOS 的分組（`internal/view/firstperson.go` 的 `base / 4`）：
# 0–2 左側牆由近而遠、3–5 右側牆、6 正牆深度 0、7 補牆、8 正牆深度 2、
# 9 是**這個素材包自己加的**正牆深度 1（DOS 版那一階沒解出來，
# Mega Drive 有）。
#
# 槽 7（補牆的那一對）留空：那一對在 Mega Drive 是格子 3 與 5 的斜前方牆，
# 而 remake 的牆面本來就沒畫斜前方那一層（見 REMAKE_SLOT 的說明）。
# 只點火炬不畫牆會浮在空中。深度 3 的那兩張（一個 tile）同理沒有槽位。
TORCH_SLOT = [
    ("side0",  0x0C, 2, 6, False),   # 0 左側牆深度 0
    ("side1",  0x0D, 6, 7, False),   # 1 左側牆深度 1
    ("side2",  0x0E, 10, 8, False),  # 2 左側牆深度 2
    ("side0",  0x13, 25, 6, True),   # 3 右側牆深度 0
    ("side1",  0x12, 22, 7, True),   # 4 右側牆深度 1
    ("side2",  0x11, 19, 8, True),   # 5 右側牆深度 2
    ("front0", 0x01, 14, 6, False),  # 6 正牆深度 0
    None,                            # 7 補牆：Mega Drive 畫在斜前方，不接
    ("front2", 0x07, 14, 8, False),  # 8 正牆深度 2
    ("front1", 0x04, 14, 7, False),  # 9 正牆深度 1（DOS 沒有這一階）
]

# 視圖在 nametable 裡的原點（行、列）。
TORCH_ORIGIN = (2, 2)


def torch_tiles(rom: bytes):
    """回傳火炬那 53 個 tile 的像素（已解壓、已去掉開頭的張數）。"""
    blk = [b for b in mdgfx.blocks(rom) if b["hdr"] == TORCH_BLOCK]
    if not blk:
        raise SystemExit(f"ROM 裡找不到火炬區塊 {TORCH_BLOCK:#x}")
    return mdgfx.decode(rom, blk[0])


def torch_image(pix: bytes, start: int, w: int, h: int, mirror: bool):
    """把 w×h 個 tile 拼成 (h*8, w*8) 的索引陣列。"""
    import numpy as np

    out = np.zeros((h * 8, w * 8), np.uint8)
    k = 0
    for r in range(h):
        for c in range(w):
            t = pix[(start + k) * 32:(start + k) * 32 + 32]
            k += 1
            a = np.frombuffer(t, np.uint8).reshape(8, 4)
            cell = np.empty((8, 8), np.uint8)
            cell[:, 0::2] = a >> 4
            cell[:, 1::2] = a & 15
            out[r * 8:r * 8 + 8, c * 8:c * 8 + 8] = cell
    return out[:, ::-1] if mirror else out


def torch_palettes(rom: bytes):
    """回傳九個相位的調色盤。圖不換、顏色換。

    底是 `TORCH_PALETTE` 那一條，第 5／6／7 格逐相位改成色表上的三個點。
    """
    base = [md_rgb(_w(rom, TORCH_PALETTE + i * 2)) for i in range(16)]
    table = [md_rgb(_w(rom, TORCH_TABLE + i * 2)) for i in range(TORCH_FRAMES)]
    out = []
    for n in range(TORCH_FRAMES):
        p = list(base)
        for slot, tap in zip(TORCH_FLAME, TORCH_PHASE_TAPS):
            p[slot] = table[(n + tap) % TORCH_FRAMES]
        out.append(p)
    return out


def export_torches(rom: bytes, subdir: str):
    """烘出火炬素材，回傳 {槽位索引: [x, y]}（視圖座標）。

    輸出的檔名是 **`火炬槽 × 相位數 + 相位`**，與 remake 的
    `TownSet.torchFrames`（`base / 4 * stride`）對得上。

    火炬自己一條調色盤（nametable 屬性寫死第 3 條），與牆面那一條無關，
    所以這裡不吃 `pal_of` 給的區域調色盤。
    """
    os.makedirs(os.path.join(subdir, "torch"), exist_ok=True)
    pix = torch_tiles(rom)
    pals = torch_palettes(rom)
    place = {}
    r0, c0 = TORCH_ORIGIN
    for g, ent in enumerate(TORCH_SLOT):
        if ent is None:
            continue
        art, _cell, col, row, mirror = ent
        start, w, h = TORCH_ART[art]
        img = torch_image(pix, start, w, h, mirror)
        for f in range(TORCH_FRAMES):
            save(img, pals[f],
                 os.path.join(subdir, "torch", f"{g * TORCH_FRAMES + f:02d}.png"), 1)
        place[g * TORCH_FRAMES] = [(col - c0) * 8, (row - r0) * 8]
    return place


# REMAKE_SLOT[remake 槽號] = Mega Drive 的位置編號。
#
#
# remake 的第一人稱畫法（`internal/view/firstperson.go`）沿用 DOS 的槽位：
# `d` 是第 d 個深度的正牆、`4+d` 是左側牆、`8+d` 是右側牆，`+16` 是門的變體。
# Mega Drive 的正牆是位置 1／4／7／10，左側牆柱由近而遠是 12／13／14／15，
# 右側牆柱由近而遠是 19／18／17／16。
#
# **位置 0／2、3／5、6／8、9／11 沒有對應的 remake 槽位** —— 那是斜前方
# 那一格的正面，DOS 版的畫法裡沒有這一層，所以不烘。少畫的是斜角那一小塊，
# 不是牆不見了。
REMAKE_SLOT = [1, 4, 7, 10,        # 0–3   正牆，深度 0–3
               12, 13, 14, 15,     # 4–7   左側牆
               19, 18, 17, 16]     # 8–11  右側牆


def export(rom: bytes, tables, pal_of, outdir: str) -> None:
    """烘成 remake 吃的素材包：每個區域類型一個子目錄。

    檔名用 **remake 的槽號**（`walls/00.png`…），不是 Mega Drive 的位置編號 ——
    讀取端不必再帶一張對照表。牆值 1 出 0–11、牆值 2 出 16–27（門的變體）。
    """
    os.makedirs(outdir, exist_ok=True)
    meta = {"source": "Mega Drive (1991)", "view": [VIEW_W, VIEW_H],
            "clear": 0, "areas": []}
    import numpy as np

    for ai, t in enumerate(tables):
        pal = pal_of(ai)
        sub = os.path.join(outdir, f"area{ai}")
        os.makedirs(os.path.join(sub, "walls"), exist_ok=True)
        place, fell_back = {}, []
        base = load_images(rom, variant_slots(t, 1))
        for v in sorted({k // 20 + 1 for k in t}):
            if v > 2:
                continue          # remake 只有兩個變體（一般牆與門）
            got = load_images(rom, variant_slots(t, v))
            for rs, pos in enumerate(REMAKE_SLOT):
                img = got.get(pos)
                if img is None and v > 1:
                    # 門的變體缺一張就退回一般牆那一張 —— 少一扇門的裝飾，
                    # 好過整組素材載不進去。哪幾格退回了記在 scene.json。
                    img = base.get(pos)
                    if img is not None:
                        fell_back.append(rs + (v - 1) * 16)
                if img is None:
                    continue
                w, h, data = img
                slot = rs + (v - 1) * 16
                save(to_indexed(w, h, data), pal,
                     os.path.join(sub, "walls", f"{slot:02d}.png"), 1)
                place[slot] = list(PLACE[pos])
        # 地板與天空。天空第 0 張是白天的天空，第 1 張是有天花板的格子 ——
        # 原版那一種不貼圖，是把上面 61 列整片填成索引 1。
        os.makedirs(os.path.join(sub, "sky"), exist_ok=True)
        os.makedirs(os.path.join(sub, "floor"), exist_ok=True)
        got = mdview.decode(rom, FLOOR[ai])
        if got:
            save(to_indexed(*got), pal, os.path.join(sub, "floor", "00.png"), 1)
        got = mdview.decode(rom, SKY_DAY[ai])
        if got:
            save(to_indexed(*got), pal, os.path.join(sub, "sky", "00.png"), 1)
        save(np.full((CEIL_FILL_H, VIEW_W), CEIL_FILL_INDEX, np.uint8), pal,
             os.path.join(sub, "sky", "01.png"), 1)
        # 火炬的素材、落點與顏色三個區域類型完全共用 —— `sub_3836` 不看
        # 區域類型，調色盤也是自己那一條。
        torch = export_torches(rom, sub)
        meta["areas"].append({"area": ai, "dir": f"area{ai}",
                              "types": AREA_TYPES[ai],
                              "place": place, "torchPlace": torch,
                              "torchFrames": TORCH_FRAMES,
                              "fellBack": sorted(fell_back)})
    with open(os.path.join(outdir, "scene.json"), "w", encoding="utf-8") as fh:
        json.dump(meta, fh, ensure_ascii=False, indent=1)
        fh.write("\n")
    n = sum(len(a["place"]) for a in meta["areas"])
    tn = sum(len(a["torchPlace"]) for a in meta["areas"])
    print(f"{len(meta['areas'])} 個區域、{n} 張牆、{tn} 盞火炬 → {outdir}")


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("rom")
    ap.add_argument("--sheet", help="每個區域類型的素材總覽圖輸出目錄")
    ap.add_argument("--compose", help="組出整幅視野的輸出目錄")
    ap.add_argument("--export", help="烘成 remake 素材包的輸出目錄")
    ap.add_argument("--cram", help="BlastEm 傾印的 CRAM（128 bytes），沒有就用 ROM 內建那份")
    ap.add_argument("--pal-line", type=int, default=None,
                    help="強制用第幾條調色盤線；不給就依素材塊各自挑（BLOCK_PAL_LINE）")
    ap.add_argument("--gray", action="store_true", help="改用灰階（看索引分佈用）")
    ap.add_argument("--scale", type=int, default=2)
    args = ap.parse_args()

    rom = open(args.rom, "rb").read()
    tables = area_tables(rom)
    def pal_for(block: int, force_line: int | None = None):
        if args.gray:
            return GRAY
        line = force_line
        if line is None:
            line = args.pal_line if args.pal_line is not None else BLOCK_PAL_LINE[block]
        if args.cram:
            return read_palette(args.cram)[line]
        return rom_palette(rom, line)

    pal = pal_for(1)

    print(f"{len(tables)} 個區域類型")
    for ai, t in enumerate(tables):
        variants = sorted({k // 20 + 1 for k in t})
        print(f"  區域 {ai}：牆值 {variants}，{len(t)} 個表格位置")
        for v in variants:
            got = load_images(rom, variant_slots(t, v))
            miss = [i for i in range(20) if i not in got]
            print(f"    牆值 {v}：{len(got)}/20 張"
                  + (f"，缺 {miss}" if miss else ""))

    if args.sheet:
        os.makedirs(args.sheet, exist_ok=True)
        for ai, t in enumerate(tables):
            pal = pal_for(ai)
            for v in sorted({k // 20 + 1 for k in t}):
                got = load_images(rom, variant_slots(t, v))
                for pos, (w, h, data) in sorted(got.items()):
                    save(to_indexed(w, h, data), pal,
                         os.path.join(args.sheet,
                                      f"a{ai}_v{v}_{pos:02d}_{w}x{h}.png"),
                         args.scale)
        print(f"素材總覽 → {args.sheet}")

    if args.compose:
        os.makedirs(args.compose, exist_ok=True)
        # 三種代表性的牆值配置：走廊、死路、開闊。
        cases = {
            "corridor": [0] * 12 + [1, 1, 1, 1, 1, 1, 1, 1],
            "deadend": [1, 1, 1] + [0] * 9 + [1, 0, 0, 0, 0, 0, 0, 1],
            "open": [0] * 20,
            "full": [1] * 20,
        }
        for ai, t in enumerate(tables):
            pal = pal_for(ai)
            for name, walls in cases.items():
                save(compose(rom, t, walls, pal),
                     pal, os.path.join(args.compose, f"a{ai}_{name}.png"),
                     args.scale)
        print(f"組合圖 → {args.compose}")

    if args.export:
        export(rom, tables, pal_for, args.export)


if __name__ == "__main__":
    main()
