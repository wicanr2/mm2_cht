#!/usr/bin/env python3
"""從 Mega Drive 版抽「進設施時的場景描述」，一種設施五段、對五座城。

    tools/mdflavor.py workplace/gfx/md-all/text workplace/md-flavor/flavor.json

輸入是 `tools/mdassets.py` 從玩家自備的 ROM 解出來的文字區塊；
產物同樣不入版控（帶原版全文），remake 執行時讀它。

## 區塊的版面

每個區塊是 `uint8 長度 + 內容` 一路接下去，**沒有終止符**，字元是
Mac Roman（引號是 `\\xd2`／`\\xd3`）。九十個區塊全部照這個版面解得開；
`mdassets.py` 的區塊邊界會切掉最後一兩個位元組，所以尾巴要容錯。

## 段對到哪一座城

**index 就是城的編號**（0 中門格特、1 亞特蘭提姆、2 通達拉、3 瓦肯尼亞、
4 桑德索巴），依據是訓練所那一塊：五段描述之後**緊接著五個 DOS 的設施
名**，順序正是 `Turkov's Training`／`Island Training`／`Enhancement Center`／
`Training Academy`／`Sheik Training Arena` —— 與 DOS 那五座城的訓練所
逐項相同。另外兩個獨立佐證：鐵匠 [1] 是 `Morgan Drewnhald`（DOS 城 1 是
`Drewnhald Ironworks`）、旅店 [2] 的 `cozy, warm` 對上雪地城的
`Tundaran Arms Inn`。

**一個例外**：酒館 [2] 是 `Belinthra`，而 DOS 的 `Belinthra's Bar` 在城 3。
Mega Drive 把她挪了一座城，還是這一塊的順序另有規則，沒有查證，
所以照 index 放並在文件裡記著。

## 設施的判定

拿關鍵字在前五段裡各數三次以上才算數 —— 只中一兩次的多半是別的區塊
提到而已。五種設施各只命中一個區塊，沒有二義。
"""
import argparse
import hashlib
import json
import pathlib
import re

KIND = {
    "inn": r"\binn\b|innkeeper|registry|concierge|sign (?:you )?in",
    "blacksmith": r"blacksmith|\bsmith\b|forge|smelter",
    "tavern": r"barmaid|barkeep|tavern|tankard|proprietress|waitress",
    "training": r"\btrain|trainer|squire|gladiator",
    "temple": r"temple|priest|cleric|altar|shrine of",
}
TOWNS = 5
MIN_CHARS = 60


def segments(raw: bytes):
    out, i = [], 0
    while i < len(raw):
        n = raw[i]
        if i + 1 + n > len(raw):
            break  # 區塊邊界切掉了最後一段，容錯
        out.append(raw[i + 1:i + 1 + n].decode("mac_roman"))
        i += 1 + n
    return out


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("textdir", type=pathlib.Path)
    ap.add_argument("out", type=pathlib.Path)
    args = ap.parse_args()

    found = {}
    for p in sorted(args.textdir.glob("*.txt")):
        segs = segments(p.read_bytes())
        if len(segs) < TOWNS:
            continue
        head = segs[:TOWNS]
        if sum(1 for t in head if len(t) >= MIN_CHARS) < TOWNS:
            continue
        blob = " ".join(head).lower()
        for kind, rx in KIND.items():
            if len(re.findall(rx, blob)) < 3:
                continue
            if kind in found:
                raise SystemExit(f"{kind} 命中兩個區塊：{found[kind][0]} 與 {p.stem}")
            found[kind] = (p.stem, head)

    missing = sorted(set(KIND) - set(found))
    if missing:
        raise SystemExit(f"這幾種設施沒找到區塊：{missing}")

    rows = []
    for kind in sorted(found):
        blk, head = found[kind]
        for town, text in enumerate(head):
            text = " ".join(text.split())
            rows.append({
                "key": f"md.{kind}.{town}",
                "block": blk,
                "kind": kind,
                "town": town,
                "sha8": hashlib.sha256(text.encode()).hexdigest()[:8],
                "text": text,
            })

    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(rows, ensure_ascii=False, indent=2) + "\n",
                        encoding="utf-8")
    print(f"{len(rows)} 段（{len(found)} 種設施 × {TOWNS} 座城）寫到 {args.out}")
    for kind in sorted(found):
        print(f"  {kind:11} 區塊 {found[kind][0]}")


if __name__ == "__main__":
    main()
