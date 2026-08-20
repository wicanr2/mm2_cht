#!/usr/bin/env bash
# 產生攻略／說明書網站。兩步：先畫地圖，再組頁面。
#
#   bash tools/build_site.sh              # → site/
#   bash tools/build_site.sh --serve      # 另外在 8080 起一個本機伺服器
#
# 地圖要讀玩家自備的原版（`MAP.DAT`／`ATTRIB.DAT`），所以**產出目錄一律
# gitignore**：`workplace/site/` 與 `site/` 都是。網站本身不含原版素材以外
# 的東西，但地圖是從原版資料算出來的，處置比照 remake 的其他衍生產物。
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
DATA=${MM2_DATA:-workplace/orig/MM2}
SERVE=0
[[ "${1:-}" == "--serve" ]] && SERVE=1

for d in site workplace/site; do
  git check-ignore -q "$d" || { echo "輸出路徑未忽略：$d" >&2; exit 1; }
done
[[ -f "$DATA/MM2.EXE" ]] || { echo "找不到原版資料：$DATA" >&2; exit 1; }

run() {
  docker run --rm --network none --memory 4g --cpus 2 --pids-limit 512 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" -e HOME=/tmp \
    -e GOCACHE=/gocache -e GOMODCACHE=/gomod -e GOPROXY=off \
    -v "$ROOT:/src" -v mm2-gomod:/gomod -v mm2-gobuild:/gocache -w /src "$@"
}

echo "[site] 1/2 畫地圖"
run mm2-go:latest go run ./cmd/mm2atlas -data "$DATA" -out workplace/site/maps

echo "[site] 2/2 組頁面"
run mm2-site:latest python3 tools/site/build.py \
  --stamp "commit $(git rev-parse --short=12 HEAD)　$(date +%Y-%m-%d)"

if ((SERVE)); then
  echo "[site] http://localhost:8080/ （Ctrl-C 結束）"
  docker run --rm --memory 512m --cpus 1 --pids-limit 128 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" -p 8080:8080 -v "$ROOT/site:/site:ro" -w /site \
    mm2-site:latest python3 -m http.server 8080
fi
