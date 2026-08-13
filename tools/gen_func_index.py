#!/usr/bin/env python3
"""產生 docs/re/00-function-index.md。

掃 docs/**/*.md 與 internal/**/*.go，抽出所有 sub_XXXX 形式的原版函式符號，
列出每一支「在哪份文件的哪一行被寫過」與該行的摘要。

用途是 CLAUDE.md 工具鏈那一節的要求：追一支函式之前先查索引，
避免把已經解過的函式重讀一遍。索引是掃出來的，不手工維護 ——
手工名單一定會與文件漂移，而漂移正是它要防的事。

重新產生（在容器內執行）：

    docker run --rm --network none -u "$(id -u):$(id -g)" \
      -v "$(pwd):/src" -w /src mm2-go:latest python3 tools/gen_func_index.py
"""

from __future__ import annotations

import re
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "docs" / "re" / "00-function-index.md"

# 原版符號：IDA 的 sub_XXXX（16-bit 線性位址 4–5 碼、68000 可到 6 碼）。
SYMBOL = re.compile(r"\bsub_([0-9A-Fa-f]{3,6})\b")

# 掃描範圍。docs 是筆記本體，internal 的註解常帶「這一段抄自 sub_XXXX」。
SOURCES = [
    (ROOT / "docs", "*.md"),
    (ROOT / "internal", "*.go"),
    (ROOT / "cmd", "*.go"),
]

# 索引自己不算來源，否則每次重跑都會把上一版的內容再收一遍。
SKIP = {OUT}


def summarise(line: str) -> str:
    """把一行原文壓成適合塞進表格的摘要。"""
    text = line.strip()
    # markdown 表格列：取含符號的那一格，通常就是該函式的說明。
    if text.startswith("|"):
        cells = [c.strip() for c in text.strip("|").split("|")]
        text = max(cells, key=lambda c: len(c) if SYMBOL.search(c) else 0)
    text = re.sub(r"^[#>*\-\s]+", "", text)
    text = re.sub(r"^//\s*", "", text)
    text = re.sub(r"\s+", " ", text)
    text = text.replace("|", "／")
    if len(text) > 110:
        text = text[:109] + "…"
    return text


def collect() -> dict[str, list[tuple[str, int, str]]]:
    hits: dict[str, list[tuple[str, int, str]]] = defaultdict(list)
    for base, pattern in SOURCES:
        if not base.is_dir():
            continue
        for path in sorted(base.rglob(pattern)):
            if path in SKIP:
                continue
            rel = path.relative_to(ROOT).as_posix()
            try:
                lines = path.read_text(encoding="utf-8").splitlines()
            except UnicodeDecodeError:
                continue
            for no, line in enumerate(lines, 1):
                seen_in_line = set()
                for match in SYMBOL.finditer(line):
                    # 大小寫統一成 IDA 的印法（大寫十六進位）。
                    name = "sub_" + match.group(1).upper()
                    if name in seen_in_line:
                        continue
                    seen_in_line.add(name)
                    hits[name].append((rel, no, summarise(line)))
    return hits


def sort_key(name: str) -> tuple[int, str]:
    return (int(name[4:], 16), name)


def main() -> None:
    hits = collect()
    if not hits:
        raise SystemExit("掃不到任何 sub_ 符號，先確認執行目錄是專案根目錄")

    documented = {n: v for n, v in hits.items() if any(f.startswith("docs/") for f, _, _ in v)}
    code_only = {n: v for n, v in hits.items() if n not in documented}

    out: list[str] = []
    out.append("# 反組譯函式索引")
    out.append("")
    out.append("由 `tools/gen_func_index.py` 掃 `docs/**/*.md`、`internal/**/*.go` 與")
    out.append("`cmd/**/*.go` 產生，**不要手改** —— 手工維護的名單會與文件漂移，")
    out.append("而漂移正是這份索引要防的事。追一支函式之前先查這裡，")
    out.append("查得到就先讀既有筆記，不要重開反組譯。")
    out.append("")
    out.append("重新產生：")
    out.append("")
    out.append("```")
    out.append('docker run --rm --network none -u "$(id -u):$(id -g)" \\')
    out.append('  -v "$(pwd):/src" -w /src mm2-go:latest python3 tools/gen_func_index.py')
    out.append("```")
    out.append("")
    out.append(
        f"共 {len(hits)} 個符號：{len(documented)} 個在 `docs/` 有筆記，"
        f"{len(code_only)} 個只出現在程式碼註解裡。"
    )
    out.append("")
    out.append("位址是 IDA 的線性位址。16-bit overlay 的換算是")
    out.append("`IDA linear = 檔案偏移 + 0xF800`，見 `docs/formats/01`；")
    out.append("六碼的是 Amiga／Mega Drive 的 68000 映像。")
    out.append("")
    out.append("## 已有筆記的函式")
    out.append("")
    out.append("| 函式 | 摘要 | 出處 |")
    out.append("|---|---|---|")
    for name in sorted(documented, key=sort_key):
        occurrences = documented[name]
        docs_hits = [o for o in occurrences if o[0].startswith("docs/")]
        summary = max((o[2] for o in docs_hits), key=len)
        where = ", ".join(f"`{f}:{n}`" for f, n, _ in docs_hits[:3])
        if len(docs_hits) > 3:
            where += f" 等 {len(docs_hits)} 處"
        out.append(f"| `{name}` | {summary} | {where} |")
    out.append("")

    if code_only:
        out.append("## 只出現在程式碼註解")
        out.append("")
        out.append("這些是 remake 的註解引用了原版函式，但 `docs/` 裡沒有對應筆記。")
        out.append("要動它們之前，先把該函式的證據補進 `docs/`。")
        out.append("")
        out.append("| 函式 | 摘要 | 出處 |")
        out.append("|---|---|---|")
        for name in sorted(code_only, key=sort_key):
            occurrences = code_only[name]
            summary = max((o[2] for o in occurrences), key=len)
            where = ", ".join(f"`{f}:{n}`" for f, n, _ in occurrences[:2])
            if len(occurrences) > 2:
                where += f" 等 {len(occurrences)} 處"
            out.append(f"| `{name}` | {summary} | {where} |")
        out.append("")

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text("\n".join(out) + "\n", encoding="utf-8")
    print(f"{OUT.relative_to(ROOT)}：{len(hits)} 個符號")


if __name__ == "__main__":
    main()
