#!/usr/bin/env python3
"""把 Mega Drive ROM 抽出來的英文區塊整理成 TSV，並與 DOS 的文字對照。

    tools/mdtext.py workplace/gfx/md-all/text workplace/md-text.tsv

## 為什麼要對照

Mega Drive 版是**另一份稿**：同一款遊戲、同一批場景，但 UI 訊息與部分
台詞被改寫過。要判斷「能不能拿它擴充 DOS 版的同一個場景」，得先知道
哪些是同一句、哪些是它自己新寫的 —— 逐句比對才答得出來，憑印象會高估。

## 產物不入版控

TSV 帶原版英文全文，與 `translations/strings.json` 同一類（見
`cmd/mm2strings` 的說明）：譯文入版控、原文不入。所以預設寫到
`workplace/` 底下。

## 比對方式

三層，由嚴到寬：

  exact   整句在 DOS 的原文集合裡逐字找得到
  norm    去掉大小寫、標點與連續空白之後找得到（換行位置不同的同一句）
  none    找不到

DOS 那一側的原文有兩個來源，缺一不可：

  translations/strings.json   事件、`STR.DAT`、EXE 內嵌、物品、怪物名
  data/*.json                 從原版**表格**萃出來的名字（法術的 `Origin`、
                              職業、種族、狀況、標籤…）

只取第一個會把「法術名」整批誤判成 MD 專有 —— 那 96 條在 DOS 也有，
只是它們住在資料表不住在字串區。**比對的分母錯了，結論就整個歪掉。**
"""
import argparse
import json
import pathlib
import re
import sys

# 區塊裡的字串以 0x00 分隔；抽可列印的段落，短於 4 字元的丟掉
# （多半是分隔用的單一符號，不是台詞）。
MIN_LEN = 4


def strings(raw: bytes):
    out = []
    for chunk in re.split(rb"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\xff]+", raw):
        t = chunk.decode("latin-1").strip()
        if len(t) >= MIN_LEN and re.search(r"[A-Za-z]", t):
            out.append(t)
    return out


def norm(s: str) -> str:
    return re.sub(r"[^a-z0-9]+", "", s.lower())


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("textdir", type=pathlib.Path)
    ap.add_argument("out", type=pathlib.Path)
    ap.add_argument("--dos", type=pathlib.Path,
                    default=pathlib.Path("translations/strings.json"))
    args = ap.parse_args()

    dos_exact, dos_norm = {}, {}

    def add(src: str, key: str) -> None:
        src = (src or "").strip()
        if len(src) < MIN_LEN or not re.search(r"[A-Za-z]", src):
            return
        dos_exact.setdefault(src, key)
        dos_norm.setdefault(norm(src), key)

    if args.dos.exists():
        for e in json.loads(args.dos.read_text(encoding="utf-8")):
            add(e.get("source"), e["key"])
    # 資料表裡的英文名（法術的 Origin、職業、種族、狀況…）。逐層走，
    # 不猜欄位名 —— 只要是英文字串就收，寧可分母寬一點也不要把
    # 「DOS 也有」誤判成「MD 專有」。
    for p in sorted(pathlib.Path("data").glob("*.json")) if pathlib.Path("data").is_dir() else []:
        try:
            blob = json.loads(p.read_text(encoding="utf-8"))
        except Exception:
            continue

        def walk(v):
            if isinstance(v, str):
                add(v, "data." + p.stem)
            elif isinstance(v, list):
                for x in v:
                    walk(x)
            elif isinstance(v, dict):
                for x in v.values():
                    walk(x)

        walk(blob)

    rows = []
    for p in sorted(args.textdir.glob("*.txt")):
        blk = p.stem
        for i, t in enumerate(strings(p.read_bytes())):
            key, how = "", "none"
            if t in dos_exact:
                key, how = dos_exact[t], "exact"
            elif norm(t) in dos_norm:
                key, how = dos_norm[norm(t)], "norm"
            rows.append((blk, i, len(t), how, key, t))

    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w", encoding="utf-8") as f:
        f.write("block\tidx\tchars\tmatch\tdos_key\ttext\n")
        for blk, i, n, how, key, t in rows:
            f.write(f"{blk}\t{i}\t{n}\t{how}\t{key}\t" +
                    t.replace("\t", " ").replace("\n", "\\n") + "\n")

    tot = len(rows)
    ex = sum(1 for r in rows if r[3] == "exact")
    nm = sum(1 for r in rows if r[3] == "norm")
    print(f"{tot} 條寫到 {args.out}")
    print(f"  逐字相同 {ex}（{100*ex/tot:.0f}%）")
    print(f"  正規化後相同 {nm}（{100*nm/tot:.0f}%）")
    print(f"  DOS 那側找不到 {tot-ex-nm}（{100*(tot-ex-nm)/tot:.0f}%）")
    if not dos_exact:
        print("  （沒有 DOS 原文檔，比對整欄是 none）", file=sys.stderr)


if __name__ == "__main__":
    main()
