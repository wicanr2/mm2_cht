#!/usr/bin/env python3
"""把譯文用到的中文字烘成點陣 atlas。

中文筆畫多，塞不進原版的 8×8 字位（縮到那個尺寸會糊成一團），所以中文走
獨立的高解析點陣層：原版像素層維持 320×200 再整數倍放大，中文直接畫在
放大後的畫布上（見 CLAUDE.md §6）。

預設 24×24 = 原版一個 8×8 字格放大 3 倍，兩者的行距與基準線因此對得齊。

atlas 格式（小端）：

    char[8]   "MM2CJK\\0\\0"
    uint16    glyphW
    uint16    glyphH
    uint32    count
    uint32    codepoints[count]   已排序，供二分搜尋
    bytes     count × (glyphH × ceil(glyphW/8))，每列 MSB 在左

只烘譯文實際用到的字，atlas 才不會塞進整套字庫。譯文增加時重跑即可。

同一支順便烘**英數字**（`lat24.bin`，12×24）。原版的 8×8 字型放大三倍是
3×3 的方塊像素，與 24×24 原生的中文擺在一起，英文明顯粗一截；改成同一套
字型的拉丁字母之後兩者的筆畫粗細一致。寬度取中文的一半 —— 中日韓排版
本來就是「漢字全形、拉丁半形」，不是把英文也撐成全形。

用法：tools/build_cjk_font.py translations/zh-Hant.json assets/font/cjk24.bin [--preview out.png]
      英數字一併輸出到同目錄的 lat24.bin。
"""
import json
import struct
import sys

from PIL import Image, ImageDraw, ImageFont

# 英數字用 Noto Sans Mono —— 與 Noto Sans CJK 是同一個字族，筆畫風格一致，
# 而且是**等寬**：拉丁字母的進位量固定，剛好塞得進半形格。
# 用比例字型的話 `m`／`w` 會超出格寬被切掉（畫面上變成 `Herrit`、`Terʌʌin`），
# 而切掉不會有任何錯誤。
LATIN_CANDIDATES = [
    "/usr/share/fonts/truetype/noto/NotoSansMono-Regular.ttf",
    "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
    "/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf",
]

FONT_CANDIDATES = [
    "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
    "/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc",
    "/usr/share/fonts/truetype/arphic/uming.ttc",
]


def pick_font(size):
    import os
    for path in FONT_CANDIDATES:
        if os.path.exists(path):
            # Noto CJK 的 .ttc 內含多個地區變體，TC 在索引 1 附近；
            # 取不到就退回索引 0，字形仍可用。
            for idx in (1, 0):
                try:
                    return ImageFont.truetype(path, size, index=idx), path, idx
                except Exception:
                    continue
    raise SystemExit("找不到可用的 CJK 字型，請安裝 fonts-noto-cjk")


def pick_latin(size):
    import os
    for path in LATIN_CANDIDATES:
        if os.path.exists(path):
            return ImageFont.truetype(path, size), path
    return None, None


def render_latin(font, ch, w, h, ascent, descent):
    """英數字照**基準線**對齊，不是逐字置中。

    逐字置中會讓 `o` 與 `g` 各自跑到自己的中央，一行字母上下亂跳；
    基準線對齊才是排版該有的樣子。"""
    img = Image.new("1", (w, h), 0)
    d = ImageDraw.Draw(img)
    d.fontmode = "1"
    top = (h - (ascent + descent)) // 2
    adv = d.textlength(ch, font=font)
    d.text(((w - adv) / 2, top), ch, font=font, fill=1)
    return img


def render(font, ch, w, h):
    """渲染單一字元成二值點陣。mode='1' 關掉 anti-aliasing，
    邊緣才不會出現半調子灰階 —— 點陣層只有亮/暗兩種狀態。"""
    img = Image.new("1", (w, h), 0)
    d = ImageDraw.Draw(img)
    d.fontmode = "1"
    bbox = d.textbbox((0, 0), ch, font=font)
    x = (w - (bbox[2] - bbox[0])) // 2 - bbox[0]
    y = (h - (bbox[3] - bbox[1])) // 2 - bbox[1]
    d.text((x, y), ch, font=font, fill=1)
    return img


# LAT_W 是英數字的字寬：中文的一半。
#
# 半形不是為了省空間 —— 拉丁字母塞進全形格會左右各留一大塊白，
# 讀起來像每個字母之間都插了空格。中日韓混排的慣例就是漢字全形、拉丁半形。
LAT_W = 12

# LAT_SIZE 是英數字的字級。等寬字型的進位量是字級的 0.6 倍，
# 所以 20 級剛好 12 px —— 與 LAT_W 對得上，字母不會被切掉。
LAT_SIZE = 20


def bake(font, codepoints, w, h):
    """把一批字烘成點陣，每列 MSB 在左。"""
    rowBytes = (w + 7) // 8
    blob = bytearray()
    for cp in codepoints:
        img = render(font, chr(cp), w, h)
        px = img.load()
        for y in range(h):
            for bx in range(rowBytes):
                b = 0
                for bit in range(8):
                    x = bx * 8 + bit
                    if x < w and px[x, y]:
                        b |= 0x80 >> bit
                blob.append(b)
    return blob


def write_atlas(path, codepoints, blob, w, h):
    header = b"MM2CJK\0\0" + struct.pack("<HHI", w, h, len(codepoints))
    header += struct.pack("<%dI" % len(codepoints), *codepoints)
    open(path, "wb").write(header + bytes(blob))


def main():
    src, out = sys.argv[1], sys.argv[2]
    preview = None
    if "--preview" in sys.argv:
        preview = sys.argv[sys.argv.index("--preview") + 1]
    size = 24

    # 介面本身用到的字不在譯文檔裡，但同樣要烘進來，否則面板會缺字。
    # **選單的游標 ▶ 也算**：它不是譯文的一部分，漏掉就整個看不見游標，
    # 而畫面不會報錯 —— 只會安靜地少一個字。
    UI_CHARS = (
        "騎士聖弓箭手牧師巫盜賊力智性格耐速準確年齡等級經驗金幣狀態隊伍生命法無"
        "由誰施什麼術的物品裝備背包商店查說明書第二技能城鎮指令冒險畫面"
        "空滿了卸下買不起花金了個一都還會提問將回答是否下"
        "▶─　"
    )

    entries = json.load(open(src))
    chars = set(UI_CHARS)
    for e in entries:
        for ch in e.get("target", ""):
            if ord(ch) > 0x7F:
                chars.add(ch)

    # 遊戲內的說明書與攻略提示（`data/reference.json`、`data/hints.json`）也要烘
    # —— 那些字不在譯文檔裡。
    # 漏掉不會報錯，只會在畫面上**安靜地少一個字**：手札裡的「昆登」
    # 就這樣變成「　登」，讀的人不會知道少了什麼。
    for extra in ("data/reference.json", "data/hints.json"):
        try:
            ref = json.load(open(extra, encoding="utf-8"))
        except OSError:
            continue

        def walk(v):
            if isinstance(v, str):
                for ch in v:
                    if ord(ch) > 0x7F:
                        chars.add(ch)
            elif isinstance(v, list):
                for x in v:
                    walk(x)
            elif isinstance(v, dict):
                for x in v.values():
                    walk(x)

        walk(ref)
    codepoints = sorted(ord(c) for c in chars)
    if not codepoints:
        raise SystemExit("譯文裡沒有非 ASCII 字元，還沒有東西可以烘")

    font, path, idx = pick_font(size)
    blob = bake(font, codepoints, size, size)

    import os
    os.makedirs(os.path.dirname(out), exist_ok=True)
    write_atlas(out, codepoints, blob, size, size)
    print("字型 %s (index %d)" % (path, idx))
    print("烘出 %d 個字，%d×%d，%d bytes -> %s"
          % (len(codepoints), size, size, 16 + len(codepoints) * 4 + len(blob), out))

    # 英數字：半形（漢字全形、拉丁半形），等寬字型，照基準線對齊。
    lat = os.path.join(os.path.dirname(out), "lat%d.bin" % size)
    lfont, lpath = pick_latin(LAT_SIZE)
    if lfont is None:
        raise SystemExit("找不到等寬拉丁字型，請安裝 fonts-noto-mono")
    ascent, descent = lfont.getmetrics()
    ascii_cps = list(range(0x20, 0x7F))
    rowBytes = (LAT_W + 7) // 8
    lat_blob = bytearray()
    for cp in ascii_cps:
        img = render_latin(lfont, chr(cp), LAT_W, size, ascent, descent)
        px = img.load()
        for y in range(size):
            for bx in range(rowBytes):
                b = 0
                for bit in range(8):
                    x = bx * 8 + bit
                    if x < LAT_W and px[x, y]:
                        b |= 0x80 >> bit
                lat_blob.append(b)
    write_atlas(lat, ascii_cps, lat_blob, LAT_W, size)
    print("英數字型 %s" % lpath)
    print("烘出 %d 個英數字，%d×%d -> %s"
          % (len(ascii_cps), LAT_W, size, lat))

    if preview:
        cols = 24
        rows = (len(codepoints) + cols - 1) // cols
        sheet = Image.new("1", (cols * size, rows * size), 0)
        for i, cp in enumerate(codepoints):
            sheet.paste(render(font, chr(cp), size, size),
                        ((i % cols) * size, (i // cols) * size))
        sheet.convert("L").resize((cols * size * 2, rows * size * 2), Image.NEAREST).save(preview)
        print("預覽 ->", preview)


if __name__ == "__main__":
    main()
