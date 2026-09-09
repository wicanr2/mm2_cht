#!/usr/bin/env bash
# 共用：確保 docker image 存在，且不比它的建置來源舊。由 tools/*.sh 以 source 引入。
#
# 為什麼需要這個檔：image 一旦被清掉，不會有任何錯誤訊息——下一次執行才失敗，
# 而且失敗的樣子通常看起來像別的問題。原本只有 dosbox_run.sh 會自己重建，
# 其餘腳本直接 docker run，清掉就再也回不來。
#
# 用法：
#   source "$(dirname "${BASH_SOURCE[0]}")/ensure_image.sh"
#   ensure_image "$IMAGE" "$ROOT/docker/blastem"

ensure_image() {
    local tag="$1" dir="$2"
    local need=0
    if ! docker image inspect "$tag" >/dev/null 2>&1; then
        need=1
    else
        local created epoch src
        created=$(docker image inspect "$tag" --format '{{.Created}}' 2>/dev/null)
        epoch=$(date -d "$created" +%s 2>/dev/null || echo 9999999999)
        # 目錄下任何來源檔比 image 新就重建；vendor 的存檔壓縮檔不算來源。
        src=$(find "$dir" -maxdepth 1 -type f -not -name '*.tar.gz' -printf '%T@\n' 2>/dev/null \
              | sort -n | tail -1 | cut -d. -f1)
        if [ -n "$src" ] && [ "$src" -gt "$epoch" ]; then need=1; fi
    fi
    if [ "$need" -eq 1 ]; then
        echo "[ensure_image] build $tag <- $dir" >&2
        docker build -t "$tag" "$dir" >&2
    fi
}
