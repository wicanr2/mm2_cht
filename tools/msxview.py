#!/usr/bin/env python3
"""照原版的貼圖清單重畫 MSX 版的第一人稱視圖。

    tools/msxview.py workplace/gfx/msxview

做兩件事：

1. **重建素材表。** 原版把圖形檔一張張搬進 VRAM 的固定位置，之後所有貼圖
   都用那個座標系取材。remake 不需要模擬 VRAM —— 但那個座標系是貼圖表
   唯一的參照，所以照樣把檔案貼成一張大圖，`(SX,SY)` 就直接可用。
2. **執行貼圖清單。** `tools/msxblits.py` 從反組譯抽出每個呼叫點的
   `(SX,SY,NX,NY) → (DX,DY)`，這裡逐一貼上去。

畫出來認得出是什麼，這條鏈（磁區表 → RLE → 素材位置 → 貼圖表）才算驗過。
單看數字對不對是驗不出來的：座標系錯一個偏移，畫面照樣「像有東西」。

## 素材在 VRAM 的位置

從 `f002` 裡對 `0x6854`（HL=DX、DE=DY、BC=檔案 id）的呼叫直接讀出來的。
同一塊區域會放不同的素材 —— `0x2020`–`0x2023` 那四張整套的，與
`0x2040`–`0x204A` 那組分開放的，是**兩種場景模式**，不會同時在 VRAM 裡。
"""
import collections
import json
import os
import struct
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from msxblits import blits
from msxdsk import entries, image, palette

# 模式 B：素材分開放。id → (VRAM x, VRAM y)
STAGE_B = {
    0x2045: (154, 256), 0x2046: (154, 256),      # 154×168
    0x2044: (308, 256),                           # 196×62
    0x2042: (308, 320), 0x2043: (308, 320),      # 182×102
    0x2041: (0, 320),                             # 154×64
    0x2047: (0, 356), 0x2048: (0, 356),
    0x2049: (0, 356), 0x204A: (0, 356),          # 154×28
    0x2040: (0, 424),                             # 368×8
    0x2010: (0, 432),                             # 210×24
}
# 模式 A：一張 462×128 蓋掉整個區域。
STAGE_A = {0x2020: (0, 256)}

# 視圖在 VRAM 的原點。貼圖的目的座標都從這裡算起。
VIEW = (0, 256)
VIEW_W, VIEW_H = 154, 64


def which(stage, files, sx, sy):
    """(VRAM x, y) → (檔案 id, 檔內 x, y)。落在任何素材外面就回 None。

    這一步把「VRAM 絕對座標」換成「哪個檔案的哪一塊」，remake 只要後者。
    """
    for fid, (x, y) in stage.items():
        got = image(files.get(fid, b""))
        if got is None:
            continue
        nx, ny, _ = got
        if x <= sx < x + nx and y <= sy < y + ny:
            return fid, sx - x, sy - y
    return None


def load_disk(path):
    d = open(path, "rb").read()
    files = {i: d[s * 512:s * 512 + l] for i, s, l in entries(d)}
    return files


def atlas(files, pal, stage, w=512, h=512):
    from PIL import Image

    im = Image.new("RGB", (w, h), (0, 0, 0))
    for fid, (x, y) in sorted(stage.items()):
        blob = files.get(fid)
        if blob is None:
            continue
        got = image(blob)
        if got is None:
            continue
        nx, ny, px = got
        tile = Image.new("RGB", (nx, ny))
        for yy in range(ny):
            for xx in range(nx):
                b = px[yy * (nx // 2) + xx // 2]
                tile.putpixel((xx, yy), pal[b >> 4 if xx % 2 == 0 else b & 15])
        im.paste(tile, (x, y))
    return im


def draw(sheet, rows, key=0, pal=None):
    """把一組貼圖畫到視圖上。透空色是色號 0（LMMM 的邏輯運算 8）。"""
    from PIL import Image

    out = Image.new("RGB", (VIEW_W, VIEW_H), (24, 24, 30))
    clear = pal[key] if pal else (0, 0, 0)
    n = 0
    for b in rows:
        if None in (b["sx"], b["sy"], b["nx"], b["ny"], b["dx"], b["dy"]):
            continue
        if b["dx"] >= VIEW_W:      # 另一塊暫存區，不在視圖裡
            continue
        piece = sheet.crop((b["sx"], b["sy"], b["sx"] + b["nx"], b["sy"] + b["ny"]))
        dx, dy = b["dx"] - VIEW[0], b["dy"] - VIEW[1]
        px = piece.load()
        for yy in range(piece.height):
            for xx in range(piece.width):
                if px[xx, yy] == clear:
                    continue
                if 0 <= dx + xx < VIEW_W and 0 <= dy + yy < VIEW_H:
                    out.putpixel((dx + xx, dy + yy), px[xx, yy])
        n += 1
    return out, n


def main() -> None:
    outdir = sys.argv[1] if len(sys.argv) > 1 else "workplace/gfx/msxview"
    os.makedirs(outdir, exist_ok=True)
    import glob

    disk = [p for p in sorted(glob.glob("workplace/msx/*.dsk")) if "[a]" not in p][0]
    files = load_disk(disk)
    pal = palette(files[0xFFF0])

    sheet = atlas(files, pal, STAGE_B)
    sheet.save(os.path.join(outdir, "atlas_B.png"))
    atlas(files, pal, STAGE_A).save(os.path.join(outdir, "atlas_A.png"))

    asm = sys.argv[2] if len(sys.argv) > 2 else "workplace/ida/msx_f002.asm"
    if len(sys.argv) > 3 and sys.argv[3] == "A":
        sheet = atlas(files, pal, STAGE_A)
    rows = blits(asm)
    g = collections.defaultdict(list)
    for b in rows:
        g[b["func"]].append(b)
    for fn, bs in sorted(g.items()):
        im, n = draw(sheet, bs, pal=pal)
        if n == 0:
            continue
        im.resize((VIEW_W * 2, VIEW_H * 2)).save(os.path.join(outdir, f"{fn}_{n}.png"))
        print(f"{fn}: {n} 塊")

    # 貼圖表輸出成 JSON：來源換算回「哪個檔案的哪一塊」，
    # remake 直接照這張表貼，不必知道 VRAM 這回事。
    stage = STAGE_A if (len(sys.argv) > 3 and sys.argv[3] == "A") else STAGE_B
    table = {"viewport": [VIEW_W, VIEW_H], "groups": {}}
    for fn, bs in sorted(g.items()):
        items = []
        for b in bs:
            if None in (b["sx"], b["sy"], b["nx"], b["ny"], b["dx"], b["dy"]):
                continue
            if b["dx"] >= VIEW_W:
                continue
            src = which(stage, files, b["sx"], b["sy"])
            if src is None:
                continue
            fid, lx, ly = src
            items.append({"file": f"{fid:04X}", "src": [lx, ly, b["nx"], b["ny"]],
                          "dst": [b["dx"] - VIEW[0], b["dy"] - VIEW[1]]})
        if items:
            table["groups"][fn] = items
    with open(os.path.join(outdir, "layout.json"), "w", encoding="utf-8") as f:
        json.dump(table, f, ensure_ascii=False, indent=1)
    print(f"貼圖表 → {outdir}/layout.json（{sum(len(v) for v in table['groups'].values())} 筆）")

    # 全部疊起來，看整幅視圖長什麼樣
    im, n = draw(sheet, rows, pal=pal)
    im.resize((VIEW_W * 3, VIEW_H * 3)).save(os.path.join(outdir, "all.png"))
    print(f"全部疊起來：{n} 塊 → {outdir}/all.png")


if __name__ == "__main__":
    main()
