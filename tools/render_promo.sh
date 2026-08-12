#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DATA_DIR=
while (($#)); do
  case "$1" in
    --data-dir) DATA_DIR=${2:?--data-dir 需要路徑}; shift 2 ;;
    *) echo "未知參數：$1" >&2; exit 2 ;;
  esac
done
[[ -d "$DATA_DIR" && -f "$DATA_DIR/MM2.EXE" ]] || {
  echo "請用 --data-dir 指定含 MM2.EXE 的合法原版資料目錄" >&2
  exit 1
}
DATA_DIR=$(cd "$DATA_DIR" && pwd)

OUT="$ROOT/workplace/promo"
git -C "$ROOT" check-ignore -q "$OUT" || {
  echo "推廣片輸出路徑沒有被 Git 忽略" >&2
  exit 1
}

docker run --rm --network none --memory 4g --cpus 2 --pids-limit 512 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp \
  -e GOCACHE=/gocache -e GOMODCACHE=/gomod -e GOPROXY=off \
  -v "$ROOT:/src" -v "$DATA_DIR:/original:ro" \
  -v mm2-gomod:/gomod -v mm2-gobuild:/gocache \
  -w /src mm2-go:latest \
  /usr/local/go/bin/go run ./cmd/mm2shots -data /original -out /src/workplace/promo/shots

mkdir -p "$OUT/music"
docker run --rm --network none --memory 1g --cpus 2 --pids-limit 256 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$ROOT:/src" -v "$DATA_DIR:/original:ro" \
  -v mm2-gomod:/gomod -v mm2-gobuild:/gocache \
  -w /src mm2-go:latest \
  /usr/local/go/bin/go run ./cmd/mm2music -exe /original/MM2.EXE \
    -out /src/workplace/promo/music/mm2-original-pc-speaker.wav -seconds 72

docker run --rm --network none --memory 2g --cpus 2 --pids-limit 256 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$OUT:/out" -v "$ROOT/tools:/tools:ro" -w /out \
  --entrypoint bash game-video:latest /tools/render-promo-container.sh

echo "[promo] 已產生 $OUT/mm2-remake-trailer.mp4"
