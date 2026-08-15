#!/usr/bin/env python3
"""怪物素材包的共用部分：對槽號、寫檔。

Mega Drive 與 Amiga 都要做同一件事 —— 把整張怪物圖烘成索引色 PNG，
再配上 DOS `MONSTERS.16` 的槽號。**槽號不能照檔案順序推**：

  - Mega Drive 的 nametable 在 ROM 裡的順序與 DOS 槽號是一個**排列**。
  - Amiga 的 `.anm` 接近恆等，但 DOS 有空槽而 Amiga 沒有，從第 42 槽起
    整批位移一格、第 73 槽起位移兩格。

兩邊都用同一個判準：拿**剪影**（哪些像素非透空）逐張比對 DOS 的基準圖，
再做貪婪一對一指派。一對一是重點 —— 只取「每個槽的最高分」會讓兩個槽
搶到同一張圖，而那種錯誤在總覽圖上看不出來。
"""

import json
import os

TRANSPARENT = 0


def dos_masks(data_dir: str):
    """回傳 {DOS 槽號: 剪影布林陣列}，尺寸統一補到 (86, 84)。"""
    import numpy as np

    import mm216

    blob = open(os.path.join(data_dir, "MONSTERS.16"), "rb").read()
    slots = mm216.monster_index(blob)
    out = {}
    for s in range(len(slots)):
        if not slots[s]:
            continue
        fr, _ = mm216.parse_monster(blob, s)
        _, _, w, h, px = fr[0]
        m = np.frombuffer(px, dtype=np.uint8).reshape(h, w) != mm216.MON_TRANSPARENT
        pad = np.zeros((86, 84), bool)
        pad[:min(h, 86), :min(w, 84)] = m[:86, :84]
        out[s] = pad
    return out


def assign(pic_masks, dos):
    """貪婪一對一指派。pic_masks 是 {圖索引: 剪影}，回傳 {圖索引: (槽號, 分數)}。"""
    import numpy as np

    scores = []
    for s, dm in dos.items():
        for i, pm in pic_masks.items():
            pad = np.zeros((86, 84), bool)
            h, w = pm.shape
            pad[:min(h, 86), :min(w, 84)] = pm[:86, :84]
            scores.append((float((dm == pad).mean()), s, i))
    scores.sort(reverse=True)
    out, used_s, used_i = {}, set(), set()
    for sc, s, i in scores:
        if s in used_s or i in used_i:
            continue
        out[i] = (s, round(sc, 4))
        used_s.add(s)
        used_i.add(i)
    return out


def write(outdir: str, source: str, w: int, h: int, pics, slot_of) -> None:
    """把每張圖的每個影格存成索引色 PNG，外加 `set.json`。

    pics 是 [(圖索引, 調色盤, [影格的索引陣列, …]), …]。

    **一定要存索引色**（不是 RGBA）：Go 那邊 `png.Decode` 直接拿到
    `*image.Paletted`，正是 `render.Screen.BlitHiKey` 吃的型別。存 RGBA
    還要在 Go 裡反推調色盤，而反推會把剛好同色的兩個索引併掉。
    """
    from PIL import Image

    os.makedirs(outdir, exist_ok=True)
    entries = []
    for i, pal, frames in pics:
        names = []
        for f, arr in enumerate(frames):
            im = Image.new("P", (w, h))
            im.putpalette([c for rgb in pal for c in rgb])
            im.putdata(arr.flatten().tolist())
            name = "pic%02d_f%02d.png" % (i, f)
            im.save(os.path.join(outdir, name), transparency=TRANSPARENT)
            names.append(name)
        s, sc = slot_of.get(i, (-1, 0.0))
        entries.append({"pic": i, "slot": s, "match": sc, "frames": names})
    meta = {"source": source, "width": w, "height": h,
            "clear": TRANSPARENT, "pictures": entries}
    with open(os.path.join(outdir, "set.json"), "w", encoding="utf-8") as fh:
        json.dump(meta, fh, ensure_ascii=False, indent=1)
        fh.write("\n")
    n = sum(len(e["frames"]) for e in entries)
    mapped = sum(1 for e in entries if e["slot"] >= 0)
    print(f"{len(entries)} 張圖（對到槽號 {mapped}）、{n} 個影格 → {outdir}")
