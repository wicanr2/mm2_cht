# 私有研究用的繁中／英文 OCR 工具。
#
# 不 COPY 專案或掃描資料進 image；執行時以唯讀掛載提供輸入，辨識結果只寫入
# 明確指定的私有工作目錄。公開版不需要也不使用此工具。
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        imagemagick \
        tesseract-ocr \
        tesseract-ocr-chi-tra \
        tesseract-ocr-eng \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /work

COPY tools/soft_world_ocr.sh /usr/local/bin/soft-world-ocr
RUN chmod 0555 /usr/local/bin/soft-world-ocr
