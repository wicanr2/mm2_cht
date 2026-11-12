#!/usr/bin/env python3
"""Docker-only input glyph fallback; preserve all existing display atlases."""
import argparse
import importlib.metadata
import json
from pathlib import Path

from PIL import ImageFont
from build_display_locales import atlas, sha


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--font", type=Path, required=True)
    args = parser.parse_args()
    ranges = [(0x3000, 0xA000), (0xF900, 0xFB00), (0xFF00, 0xFFF0)]
    records = []
    for locale, index in (("tc", 3), ("sc", 2)):
        face = ImageFont.truetype(str(args.font), 23, index=index)
        unknown = bytes(face.getmask("\U0010ffff"))
        chars = [chr(cp) for start, stop in ranges for cp in range(start, stop)
                 if bytes(face.getmask(chr(cp))) != unknown]
        records.append(atlas(Path("assets/font/input-%s24.bin" % locale), chars, face))
        print(locale, len(chars), flush=True)
    record = {"Pillow": importlib.metadata.version("pillow"),
              "source_font_sha256": sha(args.font), "ranges": ranges,
              "fonts": records, "purpose": "Unicode player input fallback; existing display glyphs preserved"}
    Path("translations/input-font-build.json").write_text(json.dumps(record, indent=2)+"\n")


if __name__ == "__main__":
    main()
