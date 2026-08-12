#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
MODE=${1:?模式}; INPUT=${2:-}
[[ "$MODE" == public || "$MODE" == local-full ]] || { echo '模式錯誤' >&2; exit 2; }
OUT=$([[ "$MODE" == public ]] && echo /src/dist/public || echo /src/.local-full)
ARGS=()
[[ -n "$INPUT" ]] && ARGS=(-v "$INPUT:/input:ro")
exec docker run --rm --network none --memory 4g --cpus 2 --pids-limit 512 --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp -e GOCACHE=/gocache -e GOMODCACHE=/gomod -e GOPROXY=off \
  -v "$ROOT:/src" "${ARGS[@]}" -v mm2-gomod:/gomod -v mm2-gobuild:/gocache -w /src \
  -e MM2_PACKAGE_IMAGE="${MM2_OSXCROSS_IMAGE:-wolong-osxcross-go:20260811-event10-r4}" \
  "${MM2_OSXCROSS_IMAGE:-wolong-osxcross-go:20260811-event10-r4}" bash /src/tools/package_container.sh "$MODE" macos-universal "$OUT" /input
