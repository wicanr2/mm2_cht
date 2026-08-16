# 素材烘焙用的 Python 環境：Pillow ＋ numpy。
#
#   docker build -t mm2-pil:numpy1 docker/pil
#
# `tools/amiga32.py`、`tools/mdgfx.py`、`tools/monpack.py`、`tools/sheet.py`
# 都要它。numpy 用在剪影比對（貪婪一對一指派）與位元平面展開 ——
# 純 Python 版本會慢兩個數量級，而這些是要重跑的烘焙步驟。
FROM python:3.12-slim

RUN pip install --no-cache-dir "pillow==11.*" "numpy==2.*"

WORKDIR /src
