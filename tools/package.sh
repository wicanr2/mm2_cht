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
VERSION=$(cat "$ROOT/VERSION")
if [[ "$MODE" == public ]]; then OUT_ROOT="$ROOT/dist-all/$VERSION/patch"; else OUT_ROOT="$ROOT/dist-all/$VERSION/full-local"; fi
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
CONTAINER_WORK="/src/workplace/release-$VERSION"
CONTAINER_OUT="/src/${OUT_ROOT#$ROOT/}"
if [[ "$PLATFORM" == macos-universal ]]; then
  BUILD_IMAGE=${MM2_OSXCROSS_IMAGE:-san1-macos:1.26.7-sdk15.5-r1}
else
  BUILD_IMAGE=${MM2_GO_IMAGE:-san1-go:1.26.7-ebiten2.9.9-r1}
fi
PACK_IMAGE=${MM2_PKG_IMAGE:-kol-package:1.26.7-ebiten2.9.9-20261011-r1}
SOURCE_ARGS=()
for source_dir in orig amiga amiga-monsters md-monsters md-scene msx modern md-flavor genesis; do
  if [[ -d "$ROOT/workplace/$source_dir" ]]; then
    SOURCE_ARGS+=(-v "$ROOT/workplace/$source_dir:/src/workplace/$source_dir:ro")
  fi
done

# AppImage 的 runtime 是上游那顆固定的靜態 ELF。放在被忽略的 workplace/，
# 雜湊寫死在這裡 —— 換了就要有人明確改這一行，不會靜悄悄換掉。
RUNTIME="$ROOT/workplace/appimage/runtime-x86_64"
RUNTIME_SHA=1cc49bcf1e2ccd593c379adb17c9f85a36d619088296504de95b1d06215aebbf
if [[ "$PLATFORM" == linux-x64 ]]; then
  [[ -f "$RUNTIME" ]] || { echo '缺少已釘選的 workplace/appimage/runtime-x86_64；不自動下載浮動來源。' >&2; exit 1; }
fi

run() {
  local image=$1; shift
  timeout 900s docker run --rm --init --network none --memory 4g --cpus 2 --pids-limit 512 --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" -e HOME=/tmp -e GOCACHE=/src/workplace/cache/gocache -e GOMODCACHE=/src/workplace/cache/gomod -e GOPROXY=off -e GOMAXPROCS=2 \
    -v "$ROOT:/src" "${INPUT_ARGS[@]}" "${SOURCE_ARGS[@]}" -w /src \
    -e MM2_KEEP_STAGE=1 -e MM2_FLAT_OUTPUT=1 -e MM2_SOURCE_COMMIT="$(git -C "$ROOT" rev-parse HEAD)" -e MM2_PACKAGE_IMAGE="$BUILD_IMAGE" "$image" "$@"
}

run "$BUILD_IMAGE" mkdir -p /src/workplace/cache/gomod /src/workplace/cache/gocache
if [[ "$PLATFORM" == linux-x64 ]]; then
  run "$PACK_IMAGE" bash -c "echo '$RUNTIME_SHA  /src/workplace/appimage/runtime-x86_64' | sha256sum -c -"
fi
run "$BUILD_IMAGE" bash /src/tools/package_container.sh "$MODE" "$PLATFORM" "$CONTAINER_WORK" "$INPUT" "$MUSIC_INPUT"
PKG="mm2-cht-${PLATFORM}-${MODE}-${VERSION}"
run "$PACK_IMAGE" bash /src/tools/pack_wrap.sh "$MODE" "$PLATFORM" "$CONTAINER_WORK/.stage/$PLATFORM/$PKG" "$CONTAINER_OUT" \
  "$([[ "$PLATFORM" == linux-x64 ]] && echo /src/workplace/appimage/runtime-x86_64 || echo '')"
