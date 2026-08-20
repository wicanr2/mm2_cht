#!/usr/bin/env python3
"""把 docs/site/ 的 markdown 產生成靜態網站。

    docker run ... mm2-site:latest python3 tools/site/build.py \
        --src docs/site --maps workplace/site/maps --out site

**這支不自己剖析 markdown。** 表格、巢狀清單與行內標記交給 `markdown` 套件；
手寫的剖析器會在那幾處安靜地少掉一行，而少掉的那一行與「原稿就沒寫」
在畫面上一模一樣。

自訂語法只有兩個，都是「一行一個指令」的區塊：

    ::: map 11                地圖圖組：原創 SVG ＋ 遊戲內俯視圖 ＋ 圖釘清單
    ::: shot 06-map | 圖說     remake 的介面截圖

之所以不用純 markdown 的圖片語法：這兩種圖都要**配著資料一起排**
（地圖要接圖釘清單與統計、截圖要接圖說），而那些資料在 `atlas.json` 裡。
寫成指令，內容與呈現才不會各寫一份。
"""

from __future__ import annotations

import argparse
import html
import json
import re
import shutil
from pathlib import Path

import markdown

# ── 站台結構 ────────────────────────────────────────────────────────────
# 順序即側欄順序。第三欄是側欄裡的分組標題，None 表示接續上一組。
PAGES = [
    ("index.md", "首頁", None),
    ("guide/index.md", "怎麼讀這份攻略", "攻略"),
    ("guide/general.md", "開局與通用規則", None),
    ("guide/towns.md", "五座城", None),
    ("guide/outdoor.md", "二十個野外區", None),
    ("guide/interiors.md", "地下城、城堡與巫師塔", None),
    ("guide/elemental.md", "元素界與結局", None),
    ("guide/conflicts.md", "矛盾與勘誤", None),
    ("manual/index.md", "這份手冊是什麼", "說明書"),
    ("manual/story.md", "序言與科隆的歷史", None),
    ("manual/character.md", "建立角色", None),
    ("manual/play.md", "操作、畫面與野外", None),
    ("manual/combat.md", "遭遇與戰鬥", None),
    ("manual/spells.md", "法術系統", None),
    ("manual/atlas.md", "地圖冊與圖例", None),
]

SITE_TITLE = "魔法門 II 攻略與說明書"

MD_EXT = ["tables", "attr_list", "toc", "fenced_code", "md_in_html", "sane_lists"]


def load_atlas(maps_dir: Path) -> dict:
    p = maps_dir / "atlas.json"
    if not p.exists():
        raise SystemExit(f"找不到 {p}；先跑 cmd/mm2atlas 產生地圖")
    doc = json.loads(p.read_text(encoding="utf-8"))
    return (
        {m["map"]: m for m in doc["maps"]}
        | {"__legend__": doc["legend"], "__places__": doc["places"],
           "__general__": doc["general"], "__conflict__": doc["conflict"]}
    )


def hint_list(pins: list, notes: list) -> str:
    """一個地點的提示清單。有座標的排前面並標出座標，沒有的排後面。"""
    out = []
    if pins:
        rows = "".join(
            f'<li><span class="pn">{p["n"]}</span>'
            f'<span class="pc">{p["x"]},{p["y"]}</span>'
            f'<span class="pt">{html.escape(p["text"])}'
            f'<span class="src">{html.escape(p["level"])}／{html.escape(p["from"])}</span></span></li>'
            for p in pins
        )
        out.append(f'<ol class="pins">{rows}</ol>')
    if notes:
        rows = "".join(
            f'<li>{html.escape(p["text"])}'
            f'<span class="src">{html.escape(p["level"])}／{html.escape(p["from"])}</span></li>'
            for p in notes
        )
        out.append(f'<ul class="notes"><li class="hd">沒有座標的提示</li>{rows}</ul>')
    return "".join(out)


def fig_place(atlas: dict, name: str) -> str:
    """畫不出格位圖的地點：只有入口座標與提示清單。

    **地圖編號沒定出來就不要編一個。** 這些地點用「區域-x,y」的地表入口定位，
    那是雜誌與遊戲內訊息都在用的寫法，讀者查得到。
    """
    for p in atlas["__places__"]:
        if p["name"] == name:
            ent = p.get("entrance")
            head = f'<p class="ent">地表入口 <code>{html.escape(ent)}</code></p>' if ent else ""
            return f'<div class="place">{head}{hint_list(p["pins"] or [], p["notes"] or [])}</div>'
    return f'<p class="miss">（hints.json 裡沒有「{html.escape(name)}」這個地點）</p>'


def fig_spells(school: int, spells: list) -> str:
    """法術表。**資料來自 `data/spells.json`，不是抄手冊抄第二次。**

    那份 JSON 由 `cmd/mm2data` 從玩家自備的原版產生，中文欄位就是手冊
    逐條轉錄的內容 —— 遊戲裡按 `C` 看到的說明與這裡是同一份。
    另外抄一份到 markdown 只會多一個會漂移的副本。
    """
    out = []
    rows_by_level: dict[int, list] = {}
    for sp in spells:
        if sp["School"] != school:
            continue
        rows_by_level.setdefault(sp["Level"], []).append(sp)
    for lv in sorted(rows_by_level):
        body = "".join(
            f'<tr><td class="n">{sp["Index"]}</td>'
            f'<td>{html.escape(sp["Name"])}<span class="src">{html.escape(sp["Origin"])}</span></td>'
            f'<td>{html.escape(sp["Cost"])}</td>'
            f'<td>{html.escape(sp["Form"])}</td>'
            f'<td>{html.escape(sp["Target"])}</td>'
            f'<td>{html.escape(sp["Desc"])}</td></tr>'
            for sp in sorted(rows_by_level[lv], key=lambda x: x["Index"])
        )
        out.append(
            f'<h3 id="lv-{school}-{lv}">第 {lv} 級</h3>'
            f'<div class="tw"><table class="sp"><thead><tr>'
            f'<th class="n">#</th><th>名稱</th><th>消耗</th><th>形式</th><th>對象</th><th>說明</th>'
            f"</tr></thead><tbody>{body}</tbody></table></div>"
        )
    return "".join(out)


def fig_general(atlas: dict) -> str:
    rows = "".join(
        f'<li>{html.escape(h["text"])}'
        f'<span class="src">{html.escape(h["level"])}／{html.escape(h["from"])}</span></li>'
        for h in atlas["__general__"]
    )
    return f'<ul class="notes">{rows}</ul>'


def fig_conflict(atlas: dict) -> str:
    out = []
    for c in atlas["__conflict__"]:
        recs = "".join(
            f'<li>{html.escape(r["claim"])}<span class="src">{html.escape(r["from"])}</span></li>'
            for r in c["records"]
        )
        out.append(
            f'<div class="conf"><h3>{html.escape(c["topic"])}</h3>'
            f'<ul class="notes">{recs}</ul>'
            f'<p class="note">{html.escape(c["note"])}</p></div>'
        )
    return "".join(out)


def fig_map(atlas: dict, idx: int, depth: int) -> str:
    """一組地圖圖：左邊原創向量圖、右邊遊戲內俯視圖，底下是圖釘清單。"""
    m = atlas.get(idx)
    if m is None:
        return f'<p class="miss">（沒有地圖 {idx} 的圖）</p>'
    up = "../" * depth
    st = m["stats"]
    if m["indoor"]:
        facts = f'牆 {st.get("wall", 0)} 面、門 {st.get("door", 0)} 道、看不見的屏障 {st.get("barrier", 0)} 面、事件格 {st.get("event", 0)} 格'
    else:
        facts = f'可通行 {st.get("open", 0)} 格、山區 {st.get("mountain", 0)}、森林 {st.get("forest", 0)}、水域 {st.get("water", 0)}、事件格 {st.get("event", 0)}'
    pins = ""
    if m.get("pins"):
        rows = "".join(
            f'<li><span class="pn">{p["n"]}</span>'
            f'<span class="pc">{p["x"]},{p["y"]}</span>'
            f'<span class="pt">{html.escape(p["text"])}'
            f'<span class="src">{html.escape(p["level"])}／{html.escape(p["from"])}</span></span></li>'
            for p in m["pins"]
        )
        pins = f'<ol class="pins">{rows}</ol>'
    notes = ""
    if m.get("notes"):
        rows = "".join(
            f'<li>{html.escape(p["text"])}'
            f'<span class="src">{html.escape(p["level"])}／{html.escape(p["from"])}</span></li>'
            for p in m["notes"]
        )
        notes = f'<ul class="notes"><li class="hd">沒有座標的提示</li>{rows}</ul>'
    return (
        f'<figure class="mapfig" id="map-{idx:02d}">'
        f'<div class="two">'
        f'<div><img src="{up}maps/{m["svg"]}" alt="{html.escape(m["name"])} 的格位圖" loading="lazy">'
        f'<figcaption>格位圖　由 <code>MAP.DAT</code> ＋ <code>ATTRIB.DAT</code> 畫出</figcaption></div>'
        f'<div><img src="{up}maps/{m["png"]}" alt="{html.escape(m["name"])} 的遊戲內俯視圖" loading="lazy">'
        f'<figcaption>遊戲內按 <kbd>M</kbd> 的畫面</figcaption></div>'
        f"</div>"
        f'<p class="facts">地圖 {idx}　{facts}</p>'
        f"{pins}"
        f"{notes}"
        f"</figure>"
    )


def fig_shot(name: str, caption: str, depth: int) -> str:
    up = "../" * depth
    cap = f"<figcaption>{html.escape(caption)}</figcaption>" if caption else ""
    return (
        f'<figure class="shot"><img src="{up}shots/{name}.png" '
        f'alt="{html.escape(caption or name)}" loading="lazy">{cap}</figure>'
    )


def fig_legend(atlas: dict) -> str:
    rows = "".join(
        f'<li><span class="sw {it["shape"]}" style="--c:{it["color"]}"></span>{html.escape(it["label"])}</li>'
        for it in atlas["__legend__"]
    )
    return f'<ul class="legend">{rows}</ul>'


# 表格要能水平捲動，不能撐破版面。python-markdown 不包容器，這裡補上。
# 順便把「整欄都是數字」的欄位標成 `n`（右對齊、等寬、按位數對齊）——
# **只標最後一欄以外的欄位也要標**，但相鄰的右對齊欄不貼齊右緣，
# 由 CSS 的 padding 處理，否則兩欄的數字會連在一起讀成一個。
TABLE = re.compile(r"<table>.*?</table>", re.S)
CELL = re.compile(r"<(td|th)([^>]*)>(.*?)</\1>", re.S)
NUMISH = re.compile(r"^[\d.,%+\-–—/:×~ ]+$")


def mark_tables(body: str) -> str:
    def one(mo: re.Match) -> str:
        t = mo.group(0)
        rows = re.findall(r"<tr>(.*?)</tr>", t, re.S)
        if not rows:
            return f'<div class="tw">{t}</div>'
        cols = max(len(CELL.findall(r)) for r in rows)
        numeric = []
        for c in range(cols):
            vals = []
            for r in rows[1:]:
                cells = CELL.findall(r)
                if c < len(cells):
                    v = re.sub(r"<[^>]+>", "", cells[c][2]).strip()
                    if v:
                        vals.append(v)
            numeric.append(bool(vals) and all(NUMISH.match(v) for v in vals))

        def fixrow(r: str) -> str:
            i = -1

            def fixcell(m: re.Match) -> str:
                nonlocal i
                i += 1
                if i < len(numeric) and numeric[i]:
                    return f'<{m.group(1)}{m.group(2)} class="n">{m.group(3)}</{m.group(1)}>'
                return m.group(0)

            return CELL.sub(fixcell, r)

        for r in rows:
            t = t.replace(f"<tr>{r}</tr>", f"<tr>{fixrow(r)}</tr>", 1)
        return f'<div class="tw">{t}</div>'

    return TABLE.sub(one, body)


BLOCK = re.compile(r"^::: *(map|shot|legend|place|general|conflict|spells) *(.*)$", re.M)


def expand(text: str, atlas: dict, depth: int) -> str:
    def sub(mo: re.Match) -> str:
        kind, arg = mo.group(1), mo.group(2).strip()
        if kind == "map":
            return fig_map(atlas, int(arg), depth)
        if kind == "legend":
            return fig_legend(atlas)
        if kind == "place":
            return fig_place(atlas, arg)
        if kind == "general":
            return fig_general(atlas)
        if kind == "conflict":
            return fig_conflict(atlas)
        if kind == "spells":
            return fig_spells(0 if arg.strip() == "cleric" else 1, atlas["__spells__"])
        name, _, cap = arg.partition("|")
        return fig_shot(name.strip(), cap.strip(), depth)

    return BLOCK.sub(sub, text)


def nav(cur: str, depth: int) -> str:
    up = "../" * depth
    out = ['<nav class="side"><a class="brand" href="%sindex.html">%s</a>' % (up, SITE_TITLE)]
    for path, label, group in PAGES:
        if group:
            out.append(f'<div class="grp">{group}</div>')
        href = up + path.replace(".md", ".html")
        cls = ' class="on"' if path == cur else ""
        out.append(f"<a{cls} href=\"{href}\">{html.escape(label)}</a>")
    out.append("</nav>")
    return "".join(out)


TEMPLATE = """<!doctype html>
<html lang="zh-Hant">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>{title}</title>
<link rel="stylesheet" href="{up}assets/site.css">
</head>
<body>
<a class="skip" href="#main">跳到內容</a>
{nav}
<main id="main">
{body}
<footer>
<p>本站的內容來自 1990 年前後《軟體世界》雜誌的《魔法門 II》連載，以及軟體世界研究開發中心發行的
珍017 中文說明書。兩者都是二手資料，人工轉抄且已知有誤植；與原版程式的行為衝突時以原版為準。
地圖與截圖由 <a href="https://github.com/wicanr2/mm2_cht">mm2_cht</a> 這個 remake 產生。</p>
<p class="meta">{stamp}</p>
</footer>
</main>
</body>
</html>
"""


def build(src: Path, maps: Path, shots: Path, out: Path, stamp: str, spells: Path) -> None:
    atlas = load_atlas(maps)
    atlas["__spells__"] = json.loads(spells.read_text(encoding="utf-8")) if spells.exists() else []
    if out.exists():
        shutil.rmtree(out)
    (out / "assets").mkdir(parents=True)
    # 地圖與截圖照抄，但**不要把建置的輸入一起發出去**：`atlas.json` 是
    # 產生器讀的，頁面不 fetch 它；`README.md` 是 repo 內的說明。
    shutil.copytree(maps, out / "maps", ignore=shutil.ignore_patterns("atlas.json"))
    if shots.exists():
        shutil.copytree(shots, out / "shots", ignore=shutil.ignore_patterns("README.md"))
    shutil.copy(Path(__file__).with_name("site.css"), out / "assets" / "site.css")
    # `.nojekyll`：GitHub Pages 預設會拿 Jekyll 再處理一次，而 Jekyll 會
    # 吃掉底線開頭的目錄，也會對已經是 HTML 的檔案再做一次樣板替換。
    (out / ".nojekyll").write_text("", encoding="utf-8")

    made = 0
    for path, label, _ in PAGES:
        f = src / path
        if not f.exists():
            print(f"缺頁：{path}")
            continue
        depth = len(Path(path).parts) - 1
        md = markdown.Markdown(extensions=MD_EXT, output_format="html5")
        body = mark_tables(md.convert(expand(f.read_text(encoding="utf-8"), atlas, depth)))
        title = label if path == "index.md" else f"{label}｜{SITE_TITLE}"
        dst = out / path.replace(".md", ".html")
        dst.parent.mkdir(parents=True, exist_ok=True)
        dst.write_text(
            TEMPLATE.format(
                title=html.escape(title), nav=nav(path, depth),
                body=body, up="../" * depth, stamp=html.escape(stamp),
            ),
            encoding="utf-8",
        )
        made += 1
    print(f"{made} 頁 → {out}")


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--src", default="docs/site")
    ap.add_argument("--maps", default="workplace/site/maps")
    ap.add_argument("--shots", default="docs/screenshots")
    ap.add_argument("--spells", default="data/spells.json")
    ap.add_argument("--out", default="site")
    ap.add_argument("--stamp", default="")
    a = ap.parse_args()
    build(Path(a.src), Path(a.maps), Path(a.shots), Path(a.out), a.stamp, Path(a.spells))


if __name__ == "__main__":
    main()
