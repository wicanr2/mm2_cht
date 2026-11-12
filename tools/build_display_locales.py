#!/usr/bin/env python3
"""在 Docker 中，以鎖版 OpenCC 產生簡中及獨立的抗鋸齒顯示字型。"""
import argparse
import csv
import hashlib
import importlib.metadata
import json
from pathlib import Path
import struct
import sys
import zipfile

from PIL import Image, ImageDraw, ImageFont
from build_cjk_font import go_string_chars, bake

WHEEL_SHA256 = "41b3b92943c7bed291f448e9c7fad4b577c8c2eae30fcfe5a74edf8818493aa6"
WHEEL_URL = "https://files.pythonhosted.org/packages/30/6b/055b7806f320cc8f2cdf23c5f70221c0dc1683fca9ffaf76dfc2ad4b91b6/opencc_python_reimplemented-0.1.7-py2.py3-none-any.whl"


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def strings(value):
    if isinstance(value, str):
        yield value
    elif isinstance(value, list):
        for item in value:
            yield from strings(item)
    elif isinstance(value, dict):
        for item in value.values():
            yield from strings(item)


def atlas(path, chars, face, width=24, height=24):
    """MM2CJA stores one alpha byte per pixel; legacy MM2CJK is unchanged."""
    codes = sorted({ord(c) for c in chars})
    latin = all(code < 128 for code in codes)
    baseline = None
    scale_y = 1.0
    if latin:
        # Use one baseline and one vertical scale for the entire face. Fitting
        # each wide letter in both axes made M/m shorter than narrow i/l.
        boxes = [face.getbbox(chr(code), anchor="ls") for code in codes]
        top = min(box[1] for box in boxes)
        bottom = max(box[3] for box in boxes)
        scale_y = min(1.0, (height-2)/max(1, bottom-top))
        baseline = (height-round((bottom-top)*scale_y))//2-round(top*scale_y)
    blob = bytearray(b"MM2CJA\0\0" + struct.pack("<HHI", width, height, len(codes)))
    blob += struct.pack("<%dI" % len(codes), *codes)
    for code in codes:
        ch = chr(code)
        box = face.getbbox(ch, anchor="ls") if latin else face.getbbox(ch)
        glyph = Image.new("L", (max(1, box[2]-box[0]), max(1, box[3]-box[1])), 0)
        draw = ImageDraw.Draw(glyph)
        if latin:
            draw.text((-box[0], -box[1]), ch, font=face, fill=255, anchor="ls")
            target = (min(width, max(1, round(glyph.width*scale_y))), max(1, round(glyph.height*scale_y)))
            if glyph.size != target:
                glyph = glyph.resize(target, Image.Resampling.LANCZOS)
        else:
            draw.text((-box[0], -box[1]), ch, font=face, fill=255)
        if not latin and (glyph.width > width-2 or glyph.height > height-2):
            factor = min((width-2)/glyph.width, (height-2)/glyph.height)
            glyph = glyph.resize((max(1, round(glyph.width*factor)), max(1, round(glyph.height*factor))), Image.Resampling.LANCZOS)
        im = Image.new("L", (width, height), 0)
        y = baseline+round(box[1]*scale_y) if latin else (height-glyph.height)//2
        assert y >= 0 and y+glyph.height <= height, (ch, y, glyph.height, height)
        im.paste(glyph, ((width-glyph.width)//2, y))
        blob += im.tobytes()
    path.write_bytes(blob)
    result = {"file": str(path), "sha256": sha(path), "glyphs": len(codes), "width": width, "height": height}
    if latin:
        result.update(baseline=baseline, vertical_scale=scale_y, positioning="shared baseline; width fit preserves letter height")
    return result


def latin_fonts(directory):
    fonts = []
    for size in (24, 30, 36):
        face = ImageFont.truetype(str(directory/"NotoSans-Regular.ttf"), round(21*size/24))
        fonts.append(atlas(Path("assets/font/display-sans%d.bin" % size), map(chr, range(32, 127)), face, size//2, size))
    face = ImageFont.truetype(str(directory/"NotoSerif-Regular.ttf"), 21)
    fonts.append(atlas(Path("assets/font/display-serif24.bin"), map(chr, range(32, 127)), face, 12))
    return fonts


def toolbar_fonts(directory):
    """A separate uppercase face keeps toolbar ink as tall as CJK labels."""
    fonts = []
    with Path("translations/interface.tsv").open(newline="") as stream:
        label = next(row["text"] for row in csv.DictReader(stream, delimiter="\t")
                     if row["key"] == "toolbar.language" and row["locale"] == "zh-Hant")
    for size in (24, 30, 36):
        width = size//2
        cjk = Path("assets/font/display-tc%d.bin" % size).read_bytes()
        cw, ch, count = struct.unpack_from("<HHI", cjk, 8)
        codes = struct.unpack_from("<%dI" % count, cjk, 16)
        ys = [y for c in label for y in range(ch) for x in range(cw)
              if cjk[16+count*4+codes.index(ord(c))*cw*ch+y*cw+x] > 100]
        ink_top, ink_bottom = min(ys), max(ys)+1
        ink_height = ink_bottom-ink_top
        # Bake from the same Noto face at 4x resolution, then fit the visible
        # capital body. Body text keeps its existing baseline and lowercase.
        face = ImageFont.truetype(str(directory/"NotoSans-Regular.ttf"), size*4)
        blob = bytearray(b"MM2CJA\0\0"+struct.pack("<HHI", width, size, 95))
        blob += struct.pack("<95I", *range(32, 127))
        cap = face.getbbox("H")
        cap_height = cap[3]-cap[1]
        for code in range(32, 127):
            ch = chr(code).upper()
            box = face.getbbox(ch)
            glyph = Image.new("L", (max(1, box[2]-box[0]), max(1, box[3]-box[1])))
            ImageDraw.Draw(glyph).text((-box[0], -box[1]), ch, font=face, fill=255)
            height = ink_height if ch.isalnum() or ch == "%" else max(1, min(ink_height, round(glyph.height*ink_height/cap_height)))
            glyph = glyph.resize((max(1, min(width-2, round(glyph.width*ink_height/cap_height))), height), Image.Resampling.LANCZOS)
            im = Image.new("L", (width, size))
            im.paste(glyph, ((width-glyph.width)//2, ink_top if ch.isalnum() or ch == "%" else ink_top+(ink_height-height)//2))
            blob += im.tobytes()
        path = Path("assets/font/toolbar-sans%d.bin" % size)
        path.write_bytes(blob)
        fonts.append({"file":str(path), "sha256":sha(path), "glyphs":95, "width":width, "height":size,
                      "ink_top":ink_top,"ink_bottom":ink_bottom,
                      "positioning":"toolbar only; uppercase; visible height and top measured from CJK language label; body fonts unchanged"})
    return fonts


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--wheel", type=Path)
    parser.add_argument("--fonts", type=Path)
    parser.add_argument("--latin-fonts", type=Path, required=True)
    parser.add_argument("--latin-only", action="store_true", help="rebuild Latin metrics without changing translations or CJK glyphs")
    parser.add_argument("--toolbar-only", action="store_true", help="rebuild only the uppercase toolbar atlases")
    args = parser.parse_args()
    if args.latin_only or args.toolbar_only:
        record_path = Path("translations/display-build.json")
        record = json.loads(record_path.read_text())
        for name, expected in record["latin_font_sha256"].items():
            assert sha(args.latin_fonts/name) == expected, "source font changed: "+name
        fonts = {item["file"]: item for item in (toolbar_fonts(args.latin_fonts) if args.toolbar_only else latin_fonts(args.latin_fonts)+toolbar_fonts(args.latin_fonts))}
        record["fonts"] = [fonts.get(item["file"], item) for item in record["fonts"]]
        known = {item["file"] for item in record["fonts"]}
        record["fonts"].extend(item for name,item in fonts.items() if name not in known)
        record["Pillow"] = importlib.metadata.version("pillow")
        record_path.write_text(json.dumps(record, ensure_ascii=False, indent=2)+"\n")
        print(json.dumps(list(fonts.values()), ensure_ascii=False))
        return
    if args.wheel is None or args.fonts is None:
        parser.error("--wheel and --fonts are required unless --latin-only is used")
    assert sha(args.wheel) == WHEEL_SHA256, "OpenCC wheel SHA-256 mismatch"
    vendor = Path("workplace/cache/opencc-0.1.7")
    vendor.mkdir(exist_ok=True)
    with zipfile.ZipFile(args.wheel) as archive:
        for item in archive.infolist():
            path = Path(item.filename)
            assert not path.is_absolute() and ".." not in path.parts
        archive.extractall(vendor)
    sys.path.insert(0, str(vendor))
    from opencc import OpenCC
    convert = OpenCC("t2s").convert
    corpus = []
    inputs = {}
    for name in ("zh-Hant.json", "md-flavor.json"):
        path = Path("translations") / name
        inputs[str(path)] = sha(path)
        rows = json.loads(path.read_text())
        for row in rows:
            corpus.append(row["target"])
        converted = [dict(row, target=convert(row["target"])) for row in rows]
        target = Path("translations") / ("zh-Hans.json" if name == "zh-Hant.json" else "md-flavor-zh-Hans.json")
        target.write_text(json.dumps(converted, ensure_ascii=False, indent=2)+"\n")
    for path in (Path("data/reference.json"), Path("data/hints.json")):
        inputs[str(path)] = sha(path)
        corpus.extend(strings(json.loads(path.read_text())))
    legacy_chars = set()
    for folder in ("internal", "cmd"):
        for path in sorted(Path(folder).rglob("*.go")):
            legacy_chars |= go_string_chars(path.read_text())
            if not path.name.endswith("_test.go"):
                corpus.append("".join(sorted(go_string_chars(path.read_text()))))
    with Path("translations/interface.tsv").open(newline="") as file:
        rows = list(csv.DictReader(file, delimiter="\t"))
    assert all(set(row) == {"key", "locale", "text"} for row in rows)
    assert len({(r["key"], r["locale"]) for r in rows}) == len(rows)
    corpus.extend(row["text"] for row in rows)
    # OpenCC's ordered phrase and character dictionaries are primary source data.
    # Keep only phrases whose characters can occur in this project's display text.
    characters = set("".join(corpus)) | set("▶─　…▼")
    pairs = {}
    dictionary_files = [vendor/"opencc/dictionary/TSPhrases.txt", vendor/"opencc/dictionary/TSCharacters.txt"]
    for path in reversed(dictionary_files):
        for line in path.read_text().splitlines():
            key, value = line.split("\t", 1)
            value = value.split(" ", 1)[0]
            if set(key) <= characters and key != value:
                pairs[key] = value
    # Exact complete translations take priority over general character mapping.
    for text in corpus:
        target = convert(text)
        if text != target:
            pairs[text] = target
    ordered = sorted(pairs.items(), key=lambda pair: (-len(pair[0]), pair[0]))
    Path("translations/simplified-map.json").write_text(json.dumps(ordered, ensure_ascii=False, indent=1)+"\n")
    cjk_chars = {c for c in characters | set("".join(pairs.values())) if ord(c) > 127}
    fonts = []
    source_font = args.fonts/"NotoSansCJK-Medium.ttc"
    for locale, face_index in (("tc", 3), ("sc", 2)):
        face = ImageFont.truetype(str(source_font), 23, index=face_index)
        unknown = bytes(face.getmask("\U0010ffff"))
        missing = [c for c in cjk_chars if bytes(face.getmask(c)) == unknown]
        assert not missing, "source font lacks glyphs: %r" % missing
        fonts.append(atlas(Path("assets/font/display-%s24.bin" % locale), cjk_chars, face))
    fonts.extend(latin_fonts(args.latin_fonts))
    fonts.extend(toolbar_fonts(args.latin_fonts))
    for size in (30,36):
        for locale,face_index in (("tc",3),("sc",2)):
            face=ImageFont.truetype(str(source_font),size-1,index=face_index)
            fonts.append(atlas(Path("assets/font/display-%s%d.bin"%(locale,size)),cjk_chars,face,size,size))
    license_path = next(vendor.glob("*.dist-info/LICENSE*"))
    Path("translations/LICENSE-OpenCC.txt").write_bytes(license_path.read_bytes())
    legacy_path = Path("assets/font/cjk24.bin")
    legacy = legacy_path.read_bytes()
    w,h,count = struct.unpack_from("<HHI",legacy,8)
    cps = struct.unpack_from("<%dI" % count,legacy,16)
    stride = (w+7)//8*h
    glyph_data = legacy[16+count*4:]
    keep = {cp:glyph_data[i*stride:(i+1)*stride] for i,cp in enumerate(cps)}
    added = sorted(ord(c) for c in legacy_chars if ord(c) not in keep)
    legacy_source = ImageFont.truetype(str(args.fonts/"NotoSansCJK-Regular.ttc"),24,index=1)
    for cp in added:keep[cp]=bytes(bake(legacy_source,[cp],w,h))
    if added:
        sorted_cps=sorted(keep)
        legacy_path.write_bytes(b"MM2CJK\0\0"+struct.pack("<HHI",w,h,len(keep))+struct.pack("<%dI"%len(keep),*sorted_cps)+b"".join(keep[cp] for cp in sorted_cps))
    record = {"OpenCC": {"version": "0.1.7", "url": WHEEL_URL, "wheel_sha256": WHEEL_SHA256, "license_sha256": sha(license_path)}, "Pillow": importlib.metadata.version("pillow"), "inputs": inputs, "source_font_sha256": sha(source_font), "latin_font_sha256": {name:sha(args.latin_fonts/name) for name in ("NotoSans-Regular.ttf","NotoSerif-Regular.ttf")}, "fonts": fonts, "display_pairs": len(ordered), "legacy_added_codepoints": added, "stage": "OpenCC conversion; terms retain existing TC meaning; display-only"}
    Path("translations/display-build.json").write_text(json.dumps(record, ensure_ascii=False, indent=2)+"\n")
    print(json.dumps(record, ensure_ascii=False))


if __name__ == "__main__":
    main()
