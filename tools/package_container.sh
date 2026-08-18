#!/usr/bin/env bash
# 容器內的第一步：編出 binary、把可帶的檔案排成一份「舞台目錄」。
#
# **這一步不產生最終檔案。** AppImage／zip 由第二步 `tools/pack_wrap.sh` 在
# 封裝用的 image 內接手 —— 分開的理由是 macOS 的編譯要 osxcross 那個 image，
# 而 AppImage 需要的 `mksquashfs` 只裝在 `mm2-pkg`，兩者不在同一個 image 裡。
set -euo pipefail
MODE=$1; PLATFORM=$2; OUT_ROOT=$3; INPUT=${4:-}; MUSIC_INPUT=${5:-}; ROOT=/src
case "$PLATFORM" in
  linux-x64) GOOS=linux; GOARCH=amd64; CGO=1; BIN=mm2 ;;
  windows-x64) GOOS=windows; GOARCH=amd64; CGO=0; BIN=mm2.exe ;;
  macos-universal) GOOS=darwin; GOARCH=universal; CGO=1; BIN=mm2; MACOS_TOOLCHAIN=1 ;;
  *) exit 2 ;;
esac
STAMP=$(git -C "$ROOT" rev-parse --short=12 HEAD); PKG="mm2-cht-${PLATFORM}-${MODE}-${STAMP}"
STAGE="$OUT_ROOT/.stage"; D="$STAGE/$PKG"
rm -rf "$STAGE"; mkdir -p "$D/bin" "$D/data" "$D/assets/font" "$D/assets/icon" "$D/translations"
build() { GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED="$CGO" /usr/local/go/bin/go build -trimpath -buildvcs=false -o "$1" "$2"; }
required_original=(MM2.EXE SPELLS.DAT 2PLAY.OVL MAP.DAT EVENTSI.DAT ATTRIB.DAT MM2.CH DEFAULT.DAT MONSTERS.DAT TOWN.16 TOWNF.16 TOWNT.16 SKY.16 ITEMS.DAT)
if [[ "$MODE" == local-full ]]; then
  for f in "${required_original[@]}"; do [[ -f "$INPUT/$f" ]] || { echo "local-full 缺少必要原版檔案：$f" >&2; exit 1; }; done
  GOOS=linux GOARCH=amd64 CGO_ENABLED=1 /usr/local/go/bin/go build -trimpath -buildvcs=false -o /tmp/mm2data-host ./cmd/mm2data
fi
if [[ "${MACOS_TOOLCHAIN:-0}" == 1 ]]; then
  command -v o64-clang >/dev/null || { echo '缺少 o64-clang' >&2; exit 78; }
  command -v oa64-clang >/dev/null || { echo '缺少 oa64-clang' >&2; exit 78; }
  LIPO=$(command -v lipo || command -v x86_64-apple-darwin-lipo || true)
  [[ -n "$LIPO" ]] || { echo '缺少 lipo' >&2; exit 78; }
  [[ -d /osxcross/SDK/MacOSX15.5.sdk ]] || { echo '缺少 MacOSX15.5.sdk' >&2; exit 78; }
  build_mac() { local arch=$1 cc=$2 out=$3 target=$4; GOOS=darwin GOARCH="$arch" CGO_ENABLED=1 CC="$cc" /usr/local/go/bin/go build -trimpath -buildvcs=false -o "$out" "$target"; }
  build_mac amd64 o64-clang /tmp/mm2-amd64 ./cmd/mm2
  build_mac arm64 oa64-clang /tmp/mm2-arm64 ./cmd/mm2
  "$LIPO" -create -output "$D/bin/mm2" /tmp/mm2-amd64 /tmp/mm2-arm64
  build_mac amd64 o64-clang /tmp/mm2data-amd64 ./cmd/mm2data
  build_mac arm64 oa64-clang /tmp/mm2data-arm64 ./cmd/mm2data
  "$LIPO" -create -output "$D/bin/mm2data" /tmp/mm2data-amd64 /tmp/mm2data-arm64
else
  build "$D/bin/$BIN" ./cmd/mm2; build "$D/bin/${BIN/mm2/mm2data}" ./cmd/mm2data
fi
cp "$ROOT/data/classes.json" "$ROOT/data/spells.json" "$ROOT/data/reference.json" "$D/data/"
cp "$ROOT/assets/font/lat24.bin" "$ROOT/assets/font/cjk24.bin" "$D/assets/font/"
# 翻譯：主譯文檔 ＋ Mega Drive 設施場景描述（`F2` 的第三個選項要用）。
cp "$ROOT/translations/zh-Hant.json" "$ROOT/translations/md-flavor.json" "$D/translations/"
# 圖示是我們自己畫的（`tools/make_icon.py`），不是原版美術，公開包可以帶。
case "$PLATFORM" in
  linux-x64) cp "$ROOT/assets/icon/mm2-256.png" "$D/assets/icon/" ;;
  windows-x64) cp "$ROOT/assets/icon/mm2.ico" "$D/assets/icon/" ;;
  macos-universal) cp "$ROOT/assets/icon/mm2.icns" "$D/assets/icon/" ;;
esac
cp "$ROOT/README.md" "$D/README.md"; cp "$ROOT/docs/release.md" "$D/release-policy.md"
if [[ "$MODE" == local-full ]]; then
  mkdir -p "$D/original-data"; cp -a "$INPUT/." "$D/original-data/"
  /tmp/mm2data-host -exe "$D/original-data/MM2.EXE" -spells-dat "$D/original-data/SPELLS.DAT" -play-ovl "$D/original-data/2PLAY.OVL" -out "$D/data"
  if [[ -n "$MUSIC_INPUT" ]]; then
    mkdir -p "$D/music"; cp -a "$MUSIC_INPUT/." "$D/music/"
  fi
fi
if [[ "${MACOS_TOOLCHAIN:-0}" == 1 ]]; then
  printf 'Package: %s\nCommit: %s\nMode: %s\nPlatform: %s\nBuild image: %s\nSDK: /osxcross/SDK/MacOSX15.5.sdk\nArchitectures: x86_64 arm64 (lipo universal)\n' "$PKG" "$(git -C "$ROOT" rev-parse HEAD)" "$MODE" "$PLATFORM" "${MM2_PACKAGE_IMAGE:-unknown}" > "$D/PACKAGE-MANIFEST.txt"
else
  printf 'Package: %s\nCommit: %s\nMode: %s\nPlatform: %s\nBuild image: %s\n' "$PKG" "$(git -C "$ROOT" rev-parse HEAD)" "$MODE" "$PLATFORM" "${MM2_PACKAGE_IMAGE:-unknown}" > "$D/PACKAGE-MANIFEST.txt"
fi
if [[ "$MODE" == public ]]; then
  if find "$D" -type f | grep -E '(^|/)(hints|soft-world)|\.(EXE|OVL|DAT|16|CH|DRV|zip|dsk)$'; then echo '公開包含禁止內容' >&2; exit 1; fi
  case "$PLATFORM" in
    linux-x64) ICON=assets/icon/mm2-256.png ;;
    windows-x64) ICON=assets/icon/mm2.ico ;;
    macos-universal) ICON=assets/icon/mm2.icns ;;
  esac
  expected=$(printf '%s\n' "README.md" "release-policy.md" "PACKAGE-MANIFEST.txt" "bin/$BIN" "bin/${BIN/mm2/mm2data}" \
    "data/classes.json" "data/spells.json" "data/reference.json" "assets/font/lat24.bin" "assets/font/cjk24.bin" \
    "$ICON" "translations/zh-Hant.json" "translations/md-flavor.json" | sort)
  actual=$(find "$D" -type f -printf '%P\n' | sort)
  [[ "$actual" == "$expected" ]] || { echo '公開包 allow-list 不符' >&2; diff -u <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") >&2; exit 1; }
  bash "$ROOT/tools/check_release.sh"
fi
echo "[stage] $D"
