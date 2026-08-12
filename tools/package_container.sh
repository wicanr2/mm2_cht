#!/usr/bin/env bash
set -euo pipefail
MODE=$1; PLATFORM=$2; OUT_ROOT=$3; INPUT=${4:-}; ROOT=/src; STAGE=/tmp/mm2-stage
rm -rf "$STAGE"; mkdir -p "$STAGE"
case "$PLATFORM" in
  linux-x64) GOOS=linux; GOARCH=amd64; CGO=1; BIN=mm2 ;;
  windows-x64) GOOS=windows; GOARCH=amd64; CGO=0; BIN=mm2.exe ;;
  macos-universal) GOOS=darwin; GOARCH=universal; CGO=1; BIN=mm2; MACOS_TOOLCHAIN=1 ;;
  *) exit 2 ;;
esac
STAMP=$(git -C "$ROOT" rev-parse --short=12 HEAD); PKG="mm2-cht-${PLATFORM}-${MODE}-${STAMP}"; D="$STAGE/$PKG"
mkdir -p "$D/bin" "$D/data" "$D/assets/font" "$D/translations"
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
cp "$ROOT/translations/zh-Hant.json" "$D/translations/"
cp "$ROOT/README.md" "$D/README.md"; cp "$ROOT/docs/release.md" "$D/release-policy.md"
if [[ "$MODE" == local-full ]]; then
  mkdir -p "$D/original-data"; cp -a "$INPUT/." "$D/original-data/"
  /tmp/mm2data-host -exe "$D/original-data/MM2.EXE" -spells-dat "$D/original-data/SPELLS.DAT" -play-ovl "$D/original-data/2PLAY.OVL" -out "$D/data"
fi
if [[ "$PLATFORM" != windows-x64 && "$MODE" == public ]]; then
cat > "$D/run.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
set -- "$@"
ROOT=$(cd "$(dirname "$0")" && pwd); ORIG=${1:-}
for f in MM2.EXE SPELLS.DAT 2PLAY.OVL MAP.DAT EVENTSI.DAT ATTRIB.DAT MM2.CH DEFAULT.DAT MONSTERS.DAT TOWN.16 TOWNF.16 TOWNT.16 SKY.16 ITEMS.DAT; do [[ -f "$ORIG/$f" ]] || { echo "原版資料缺少 $f" >&2; exit 1; }; done
shift
"$ROOT/bin/mm2data" -exe "$ORIG/MM2.EXE" -spells-dat "$ORIG/SPELLS.DAT" -play-ovl "$ORIG/2PLAY.OVL" -out "$ROOT/data"
export MM2_DATA_DIR="$ROOT/data"; cd "$ROOT"; exec "$ROOT/bin/mm2" -data "$ORIG" "$@"
EOF
elif [[ "$PLATFORM" != windows-x64 ]]; then
cat > "$D/run.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")" && pwd); ORIG=${1:-$ROOT/original-data}
for f in MM2.EXE SPELLS.DAT 2PLAY.OVL MAP.DAT EVENTSI.DAT ATTRIB.DAT MM2.CH DEFAULT.DAT MONSTERS.DAT TOWN.16 TOWNF.16 TOWNT.16 SKY.16 ITEMS.DAT; do [[ -f "$ORIG/$f" ]] || { echo "包內 original-data 缺少 $f" >&2; exit 1; }; done
[[ $# -eq 0 || $# -eq 1 ]] || { echo 'local-full 啟動器只接受一個資料目錄參數' >&2; exit 1; }
"$ROOT/bin/mm2data" -exe "$ORIG/MM2.EXE" -spells-dat "$ORIG/SPELLS.DAT" -play-ovl "$ORIG/2PLAY.OVL" -out "$ROOT/data"
export MM2_DATA_DIR="$ROOT/data"; cd "$ROOT"; exec "$ROOT/bin/mm2" -data "$ORIG"
EOF
fi
[[ "$PLATFORM" == windows-x64 ]] || chmod +x "$D/run.sh"
if [[ "$PLATFORM" == windows-x64 ]]; then
  if [[ "$MODE" == public ]]; then WIN_ORIG='set "ORIG=%~1"'; else WIN_ORIG='if "%~1"=="" (set "ORIG=%ROOT%original-data") else (set "ORIG=%~1")'; fi
cat > "$D/run.bat" <<'EOF'
@echo off
setlocal
set "ROOT=%~dp0"
EOF
  printf '%s\n' "$WIN_ORIG" >> "$D/run.bat"
  cat >> "$D/run.bat" <<'EOF'
if not exist "%ORIG%\MM2.EXE" (echo Missing MM2.EXE.& exit /b 1)
if not exist "%ORIG%\SPELLS.DAT" (echo Missing SPELLS.DAT.& exit /b 1)
for %%F in (MM2.EXE SPELLS.DAT 2PLAY.OVL MAP.DAT EVENTSI.DAT ATTRIB.DAT MM2.CH DEFAULT.DAT MONSTERS.DAT TOWN.16 TOWNF.16 TOWNT.16 SKY.16 ITEMS.DAT) do if not exist "%ORIG%\%%F" (echo Missing %%F.& exit /b 1)
if not "%~2"=="" (echo Extra arguments are not supported by this launcher.& exit /b 1)
"%ROOT%bin\mm2data.exe" -exe "%ORIG%\MM2.EXE" -spells-dat "%ORIG%\SPELLS.DAT" -play-ovl "%ORIG%\2PLAY.OVL" -out "%ROOT%data"
set "MM2_DATA_DIR=%ROOT%data"
"%ROOT%bin\mm2.exe" -data "%ORIG%"
EOF
fi
if [[ "${MACOS_TOOLCHAIN:-0}" == 1 ]]; then
  printf 'Package: %s\nCommit: %s\nMode: %s\nPlatform: %s\nBuild image: %s\nSDK: /osxcross/SDK/MacOSX15.5.sdk\nArchitectures: x86_64 arm64 (lipo universal)\n' "$PKG" "$(git -C "$ROOT" rev-parse HEAD)" "$MODE" "$PLATFORM" "${MM2_PACKAGE_IMAGE:-unknown}" > "$D/PACKAGE-MANIFEST.txt"
else
  printf 'Package: %s\nCommit: %s\nMode: %s\nPlatform: %s\n' "$PKG" "$(git -C "$ROOT" rev-parse HEAD)" "$MODE" "$PLATFORM" > "$D/PACKAGE-MANIFEST.txt"
fi
if [[ "$MODE" == public ]]; then
  if find "$D" -type f | grep -E '(^|/)(hints|soft-world)|\.(EXE|OVL|DAT|16|CH|DRV|zip|dsk)$'; then echo '公開包含禁止內容' >&2; exit 1; fi
  expected=$(printf '%s\n' "README.md" "release-policy.md" "PACKAGE-MANIFEST.txt" "bin/$BIN" "bin/${BIN/mm2/mm2data}" "data/classes.json" "data/spells.json" "data/reference.json" "assets/font/lat24.bin" "assets/font/cjk24.bin" "translations/zh-Hant.json")
  if [[ "$PLATFORM" == windows-x64 ]]; then
    expected=$(printf '%s\n%s' "$expected" "run.bat")
  else
    expected=$(printf '%s\n%s' "$expected" "run.sh")
  fi
  actual=$(find "$D" -type f -printf '%P\n' | sort); expected=$(printf '%s\n' "$expected" | sort)
  [[ "$actual" == "$expected" ]] || { echo '公開包 allow-list 不符' >&2; diff -u <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") >&2; exit 1; }
  bash "$ROOT/tools/check_release.sh"
fi
mkdir -p "$OUT_ROOT/$PLATFORM"
tar --sort=name --mtime='UTC 1970-01-01' --owner=0 --group=0 --numeric-owner -czf "$OUT_ROOT/$PLATFORM/$PKG.tar.gz" -C "$STAGE" "$PKG"
rm -rf "$STAGE"; echo "[package] 已產生 $OUT_ROOT/$PLATFORM/$PKG.tar.gz"
