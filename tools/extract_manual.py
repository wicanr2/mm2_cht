#!/usr/bin/env python3
"""從說明書轉錄抽出遊戲內要查的參考資料。

原版把第二技能、指令一覽這類參考資料只印在紙本上，遊戲裡查不到。
這支把 `docs/manual/part-*.md` 的表格抽成 `data/reference.json`，
讓前端可以在遊戲內顯示。

**來源是中文說明書的轉錄，不是原版資產** —— 與翻譯文本同樣入版控。

    tools/extract_manual.py docs/manual data/reference.json
"""
import json
import re
import sys
from pathlib import Path


def tables(text):
    """把 markdown 的表格抽成 (標題, [列]) 的序列。"""
    out, title, rows = [], None, []
    for line in text.splitlines():
        h = re.match(r"^#{2,4}\s+(.*)$", line)
        if h:
            if rows:
                out.append((title, rows))
            title, rows = h.group(1).strip(), []
            continue
        if line.startswith("|") and not re.match(r"^\|[\s:|-]+\|$", line):
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            # 表頭列要丟掉。判準是「這一列每一格都是欄位名」——
            # 只比對前兩格會誤殺（有些資料列的第一格剛好叫「原文」）。
            header = {"原文", "中文", "#", "效果", "說明出處", "畫面英文"}
            if cells and not all(c in header for c in cells):
                rows.append(cells)
    if rows:
        out.append((title, rows))
    return out


def main():
    src, out = Path(sys.argv[1]), Path(sys.argv[2])
    want = {
        "第二技能": "skills",
        "城鎮／主選單指令": "townCommands",
        "冒險（三度空間）畫面指令": "fieldCommands",
    }
    data = {"source": "珍017 繁體中文說明書轉錄（docs/manual/part-*.md）"}
    for md in sorted(src.glob("part-*.md")):
        for title, rows in tables(md.read_text(encoding="utf-8")):
            if not title:
                continue
            for key, name in want.items():
                if key in title and name not in data:
                    data[name] = [{"cols": r} for r in rows]
    missing = [n for n in want.values() if n not in data]
    if missing:
        raise SystemExit(f"抽不到：{missing} —— 標題可能改過了")
    out.write_text(json.dumps(data, ensure_ascii=False, indent=1), encoding="utf-8")
    for k, v in data.items():
        if isinstance(v, list):
            print(f"  {k}: {len(v)} 列")
    print(f"寫出 {out}")


if __name__ == "__main__":
    main()
