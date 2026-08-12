#!/bin/sh
# 《軟體世界》私有研究的逐頁 OCR 草稿工具。
#
# 必須在 mm2-ocr image 內執行；輸入為唯讀掃描目錄，輸出為工作樹外的私有目錄。
# 這只產生待人工校對的文字，不會把掃描或 OCR 結果寫進 source tree。
set -eu

if [ "$(id -u)" -eq 0 ]; then
    printf '%s\n' '拒絕以 root 執行；請用目前使用者 UID/GID 啟動容器。' >&2
    exit 2
fi

if [ "$#" -ne 2 ]; then
    printf '%s\n' '用法：soft_world_ocr.sh <唯讀掃描目錄> <私有輸出目錄>' >&2
    exit 2
fi

input=$1
output=$2

case "$output" in
    /src|/src/*)
        printf '%s\n' '拒絕寫入 /src；OCR 結果必須留在工作樹外的私有掛載目錄。' >&2
        exit 2
        ;;
esac

if [ ! -d "$input" ]; then
    printf '找不到輸入目錄：%s\n' "$input" >&2
    exit 2
fi

mkdir -p "$output"
if [ ! -w "$output" ]; then
    printf '輸出目錄不可寫入：%s\n' "$output" >&2
    exit 2
fi

found=0
for image in "$input"/*; do
    [ -f "$image" ] || continue
    case "$image" in
        *.jpeg|*.JPEG|*.jpg|*.JPG|*.png|*.PNG|*.tif|*.TIF|*.tiff|*.TIFF) ;;
        *) continue ;;
    esac
    found=1
    base=$(basename "$image")
    base=${base%.*}
    printf '辨識 %s\n' "$base" >&2
    tesseract "$image" "$output/$base" -l chi_tra+eng --psm 6
done

if [ "$found" -eq 0 ]; then
    printf '輸入目錄沒有支援的掃描影像：%s\n' "$input" >&2
    exit 2
fi
