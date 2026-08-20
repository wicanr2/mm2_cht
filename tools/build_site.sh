#!/usr/bin/env bash
# 產生攻略／說明書網站。兩步：先畫地圖，再組頁面。
#
#   bash tools/build_site.sh              # → docs/
#   bash tools/build_site.sh --serve      # 另外在 8080 起一個本機伺服器
#
# **產物進版控。** GitHub Pages 直接提供 `main` 分支的 `docs/`，沒有建置步驟，
# 所以 HTML、CSS 與地圖圖檔都要 commit。地圖要讀玩家自備的原版才畫得出來
# （`MAP.DAT` ＋ `ATTRIB.DAT`），CI 上沒有那份資料 —— 這也是不做成 Actions
# 的原因。公開範圍的裁決見 docs/release.md。
#
# 產生器**不會**把 `docs/` 整個清掉再重建（那會帶走 formats/、research/、
# manual/part-*.md）。它照 `docs/.site-manifest` 記的清單刪自己上次產生的檔案。
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
DATA=${MM2_DATA:-workplace/orig/MM2}
SERVE=0
[[ "${1:-}" == "--serve" ]] && SERVE=1

[[ -f "$DATA/MM2.EXE" ]] || { echo "找不到原版資料：$DATA" >&2; exit 1; }

run() {
  docker run --rm --network none --memory 4g --cpus 2 --pids-limit 512 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" -e HOME=/tmp \
    -e GOCACHE=/gocache -e GOMODCACHE=/gomod -e GOPROXY=off \
    -v "$ROOT:/src" -v mm2-gomod:/gomod -v mm2-gobuild:/gocache -w /src "$@"
}

echo "[site] 1/2 畫地圖"
run mm2-go:latest go run ./cmd/mm2atlas -data "$DATA"

echo "[site] 2/2 組頁面"
run mm2-site:latest python3 tools/site/build.py \
  --stamp "commit $(git rev-parse --short=12 HEAD)　$(date +%Y-%m-%d)"

if ((SERVE)); then
  echo "[site] http://localhost:8080/ （Ctrl-C 結束）"
  docker run --rm --memory 512m --cpus 1 --pids-limit 128 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" -p 8080:8080 -v "$ROOT/docs:/site:ro" -w /site \
    mm2-site:latest python3 -m http.server 8080
fi
