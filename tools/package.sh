#!/usr/bin/env bash
# 三平台封裝。兩步：先在編譯用的 image 裡排出舞台目錄，再在封裝用的
# image 裡封成 AppImage／zip。分兩步的理由寫在 tools/pack_wrap.sh 開頭。
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
MODE=${1:-}; PLATFORM=${2:-}; DATA_DIR=; MUSIC_PACK_DIR=
shift 2 2>/dev/null || true
while (($#)); do
  case "$1" in
    --data-dir) DATA_DIR=${2:?--data-dir 需要路徑}; shift 2 ;;
    --music-pack-dir) MUSIC_PACK_DIR=${2:?--music-pack-dir 需要路徑}; shift 2 ;;
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
  if [[ -n "$MUSIC_PACK_DIR" ]]; then
    [[ -f "$MUSIC_PACK_DIR/manifest.json" ]] || { echo "音樂包目錄缺少 manifest.json" >&2; exit 1; }
    INPUT_ARGS+=(-v "$MUSIC_PACK_DIR:/music-input:ro"); MUSIC_INPUT=/music-input
  else
    MUSIC_INPUT=
  fi
else
  [[ -z "$MUSIC_PACK_DIR" ]] || { echo "公開包不得帶入原版音樂包" >&2; exit 1; }
  INPUT_ARGS=(); INPUT=
  MUSIC_INPUT=
fi
CONTAINER_OUT=$([[ "$MODE" == public ]] && echo /src/dist/public || echo /src/.local-full)
if [[ "$PLATFORM" == macos-universal ]]; then
  BUILD_IMAGE=${MM2_OSXCROSS_IMAGE:-wolong-osxcross-go:20260811-event10-r4}
else
  BUILD_IMAGE=${MM2_GO_IMAGE:-mm2-go:latest}
fi
PACK_IMAGE=${MM2_PKG_IMAGE:-mm2-pkg:latest}

# AppImage 的 runtime 是上游那顆固定的靜態 ELF。放在被忽略的 workplace/，
# 雜湊寫死在這裡 —— 換了就要有人明確改這一行，不會靜悄悄換掉。
RUNTIME="$ROOT/workplace/appimage/runtime-x86_64"
RUNTIME_SHA=1cc49bcf1e2ccd593c379adb17c9f85a36d619088296504de95b1d06215aebbf
RUNTIME_URL=https://github.com/AppImage/type2-runtime/releases/download/continuous/runtime-x86_64
if [[ "$PLATFORM" == linux-x64 ]]; then
  if [[ ! -f "$RUNTIME" ]]; then
    mkdir -p "$(dirname "$RUNTIME")"
    echo "[package] 取 AppImage runtime：$RUNTIME_URL"
    curl -sfL -o "$RUNTIME" "$RUNTIME_URL"
  fi
  echo "$RUNTIME_SHA  $RUNTIME" | sha256sum -c - >/dev/null || { echo "AppImage runtime 雜湊不符" >&2; exit 1; }
fi

run() {
  local image=$1; shift
  docker run --rm --network none --memory 4g --cpus 2 --pids-limit 512 --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" -e HOME=/tmp -e GOCACHE=/gocache -e GOMODCACHE=/gomod -e GOPROXY=off \
    -v "$ROOT:/src" "${INPUT_ARGS[@]}" -v mm2-gomod:/gomod -v mm2-gobuild:/gocache -w /src \
    -e MM2_PACKAGE_IMAGE="$BUILD_IMAGE" "$image" "$@"
}

run "$BUILD_IMAGE" bash /src/tools/package_container.sh "$MODE" "$PLATFORM" "$CONTAINER_OUT" "$INPUT" "$MUSIC_INPUT"
STAMP=$(git -C "$ROOT" rev-parse --short=12 HEAD)
PKG="mm2-cht-${PLATFORM}-${MODE}-${STAMP}"
run "$PACK_IMAGE" bash /src/tools/pack_wrap.sh "$MODE" "$PLATFORM" "$CONTAINER_OUT/.stage/$PKG" "$CONTAINER_OUT" \
  "$([[ "$PLATFORM" == linux-x64 ]] && echo /src/workplace/appimage/runtime-x86_64 || echo '')"
