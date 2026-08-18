#!/usr/bin/env python3
"""從 MSX 版的引擎 `f002` 抽出「野外圖 → 戶外素材」那張表。

    tools/msxmaps.py workplace/msx/out/d1_f002_26122.bin --go \
        > internal/assets/msx/outdoor_maps.go

## 這張表在哪、憑什麼是它

`sub_4F9(地圖號)` 從位址 `0x431` 起掃 24 筆 8 bytes 的記錄，`+0` 相符就
回傳那一筆的位址（載入位址 0x100，所以檔案偏移 ＝ 位址 − 0x100）。
兩個使用端各讀一個欄位：

  - `sub_37B0`（換圖時載素材）讀 `+2` 與 `+4`：
    `+2 == 1` 載 `0x2043`、`== 0` 載 `0x2042`（擋路物 A，兩張同尺寸同落點）；
    `+4 == 1` 載 `0x2046`、`== 0` 載 `0x2045`（地平線的地形帶）。
  - `sub_1AD9` 在格子碼是 4／5（地形帶）時讀 `+6`。

`+6` 是**地形碼**：與 DOS 的貼圖組碼（`ATTRIB +4`，9 沙漠／10 海洋／
11 沼澤／12 凍原）在 24 筆裡有 23 筆相同，與 MSX 地圖檔 `+0x204` 的
高 nibble 有 22 筆相同（地圖 33 與 40 兩筆不同）。**地平線帶的變體看的是
地圖檔那個位元組，不是這一欄** —— 這一欄的使用端是 `sub_1AD9`。
地圖 40–44 這一欄是 0；那四張（41–44）的地面另有一條路
（`sub_390E` 依地圖號挑 `0x2047`–`0x204A`）。

自我驗證條件：表的結尾正好接上 `sub_4F9` 自己（`0x431 + 25×8 == 0x4F9`），
而且第 0 筆是 `0000 FFFF FFFF FFFF` 的哨兵（地圖號 0 不會被查到，
因為掃描從索引 1 起）。
"""
import argparse
import pathlib
import struct
import sys

BASE = 0x100
TABLE = 0x431
COUNT = 25  # 第 0 筆是哨兵，`sub_4F9` 掃 1..24


def read(path: pathlib.Path):
    d = path.read_bytes()
    off = TABLE - BASE
    rows = []
    for i in range(COUNT):
        rows.append(struct.unpack_from("<4H", d, off + i * 8))
    if rows[0] != (0, 0xFFFF, 0xFFFF, 0xFFFF):
        sys.exit(f"第 0 筆不是哨兵（{rows[0]}），表的位置或檔案不對")
    return rows[1:]


def emit(rows) -> str:
    out = [
        "// 這個檔是 tools/msxmaps.py 產生的，不要手改。",
        "//",
        "// 來源：MSX 版引擎 f002 的 `0x431`，24 筆 8 bytes 的記錄。",
        "// 機制見 docs/research/02-other-platforms.md「MSX 的戶外第一人稱」。",
        "",
        "package msx",
        "",
        "// OutMap 是一張野外圖要用哪幾張戶外素材。",
        "type OutMap struct {",
        "\t// FeatureA 是擋路物 A 的素材編號：0x2042 或 0x2043，兩張同尺寸同落點。",
        "\tFeatureA uint16",
        "\t// Band 是地平線地形帶的素材編號：0x2045 或 0x2046。",
        "\tBand uint16",
        "\t// Terrain 是地形碼（9 沙漠、10 海洋、11 沼澤、12 凍原）。",
        "\t// 0 表示這張圖的地面由地圖號另外挑，見 OutGroundID。",
        "\tTerrain int",
        "}",
        "",
        "// OutdoorMaps 是野外圖的素材選擇，鍵是地圖號。不在表裡的是室內圖。",
        "var OutdoorMaps = map[int]OutMap{",
    ]
    # 鍵補到等寬，產出直接就是 gofmt 過的樣子。
    w = max(len(str(m)) for m, _, _, _ in rows)
    for m, a, b, t in rows:
        fa = 0x2043 if a else 0x2042
        bd = 0x2046 if b else 0x2045
        key = f"{m}:".ljust(w + 1)
        out.append(f"\t{key} {{FeatureA: 0x{fa:04X}, Band: 0x{bd:04X}, Terrain: {t}}},")
    out.append("}")
    out.append("")
    return "\n".join(out)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("f002", type=pathlib.Path)
    ap.add_argument("--go", action="store_true", help="輸出 Go 表")
    args = ap.parse_args()
    rows = read(args.f002)
    if args.go:
        sys.stdout.write(emit(rows))
        return
    print("地圖  擋路物A  地形帶  地形碼")
    for m, a, b, t in rows:
        print(f"{m:4d}  {0x2043 if a else 0x2042:04X}   {0x2046 if b else 0x2045:04X}  {t:4d}")


if __name__ == "__main__":
    main()
