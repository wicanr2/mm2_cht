#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
MODE=${1:-}; PLATFORM=${2:-}; DATA_DIR=
shift 2 2>/dev/null || true
while (($#)); do
  case "$1" in
    --data-dir) DATA_DIR=${2:?--data-dir 需要路徑}; shift 2 ;;
    *) echo "未知參數：$1" >&2; exit 2 ;;
  esac
done
[[ "$MODE" == public || "$MODE" == local-full ]] || { echo "模式錯誤" >&2; exit 2; }
[[ "$PLATFORM" == linux-x64 || "$PLATFORM" == windows-x64 || "$PLATFORM" == macos-universal ]] || { echo "平台錯誤" >&2; exit 2; }
if [[ "$MODE" == public ]]; then OUT_ROOT="$ROOT/dist/public"; else OUT_ROOT="$ROOT/.local-full"; fi
git -C "$ROOT" check-ignore -q "$OUT_ROOT" || { echo "輸出路徑未忽略" >&2; exit 1; }
if git -C "$ROOT" ls-files --error-unmatch "${OUT_ROOT#$ROOT/}" >/dev/null 2>&1; then echo "輸出路徑已納入 Git" >&2; exit 1; fi
if [[ "$MODE" == local-full ]]; then
  [[ -d "$DATA_DIR" && -f "$DATA_DIR/MM2.EXE" ]] || { echo "local-full 需要含 MM2.EXE 的資料目錄" >&2; exit 1; }
  INPUT_ARGS=(-v "$DATA_DIR:/input:ro"); INPUT=/input
else
  INPUT_ARGS=(); INPUT=
fi
CONTAINER_OUT=$([[ "$MODE" == public ]] && echo /src/dist/public || echo /src/.local-full)
if [[ "$PLATFORM" == macos-universal ]]; then
  BUILD_IMAGE=${MM2_OSXCROSS_IMAGE:-wolong-osxcross-go:20260811-event10-r4}
else
  BUILD_IMAGE=${MM2_GO_IMAGE:-mm2-go:latest}
fi
exec docker run --rm --network none --memory 4g --cpus 2 --pids-limit 512 --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp -e GOCACHE=/gocache -e GOMODCACHE=/gomod -e GOPROXY=off \
  -v "$ROOT:/src" "${INPUT_ARGS[@]}" -v mm2-gomod:/gomod -v mm2-gobuild:/gocache -w /src \
  -e MM2_PACKAGE_IMAGE="$BUILD_IMAGE" "$BUILD_IMAGE" bash /src/tools/package_container.sh "$MODE" "$PLATFORM" "$CONTAINER_OUT" "$INPUT"
