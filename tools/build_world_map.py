#!/usr/bin/env python3
"""把珍017 的 Cron 世界地圖平面掃描烘成遊戲用索引色 PNG。

來源掃描不進版控；輸出是使用者在 2026-08-26 明確同意公開的地圖呈現。
這支工具只做非生成式處理，不重畫文字、地形或地名。
"""

import argparse
from pathlib import Path

from PIL import Image, ImageEnhance, ImageFilter, ImageOps


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("source", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()

    with Image.open(args.source) as opened:
        image = ImageOps.exif_transpose(opened).convert("RGB")

    # 原掃描已展平；只校正掃描造成的灰霧、略提色彩，再用 Lanczos 放大。
    # 不做幾何重建，避免文字與格線被生成式工具改寫。
    image = ImageOps.autocontrast(image, cutoff=0.5)
    image = ImageEnhance.Color(image).enhance(1.04)
    image = ImageEnhance.Contrast(image).enhance(1.03)
    image = image.resize((1536, 1122), Image.Resampling.LANCZOS)
    image = image.filter(ImageFilter.UnsharpMask(radius=1.2, percent=80, threshold=3))

    # render.Screen 的現有高解析貼圖契約是索引色；256 色保留彩色地圖，
    # 同時避免為單一畫面另開一條 RGBA 素材管線。
    image = image.quantize(colors=256, method=Image.Quantize.MEDIANCUT,
                           dither=Image.Dither.FLOYDSTEINBERG)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    image.save(args.output, optimize=True)


if __name__ == "__main__":
    main()
