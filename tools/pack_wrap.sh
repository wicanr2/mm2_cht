#!/usr/bin/env bash
# 容器內的第二步：把舞台目錄封成各平台真正在用的格式。
#
#   linux-x64        → AppImage（單檔、雙擊即跑）
#   windows-x64      → zip（每一筆都標 UTF-8，見 tools/pack_zip.py）
#   macos-universal  → zip，裡面是 `.app`（雙擊即跑）
#
# **啟動器要處理「包是唯讀的」這件事。** 引擎讀 `translations/`、`assets/`
# 與存檔都相對於工作目錄，而 AppImage 掛起來是唯讀的、`.app` 也不該被寫。
# 所以啟動器在使用者的資料目錄開一個可寫的工作根，把唯讀的兩個資料夾
# 連過去，`data/` 與 `save/` 放真檔，再 `cd` 進去執行。
set -euo pipefail
MODE=$1; PLATFORM=$2; STAGE=$3; OUT_ROOT=$4; RUNTIME=${5:-}
ROOT=/src
PKG=$(basename "$STAGE")
REQUIRED='MM2.EXE SPELLS.DAT 2PLAY.OVL MAP.DAT EVENTSI.DAT ATTRIB.DAT MM2.CH DEFAULT.DAT MONSTERS.DAT TOWN.16 TOWNF.16 TOWNT.16 SKY.16 ITEMS.DAT'
OUT="$OUT_ROOT/$PLATFORM"; mkdir -p "$OUT"

# launcher_body 產生 Unix 啟動器的共通段。
#   $1 = 取得包內容的變數名（APP）已由呼叫端 export
#   $2 = 沒給參數時的預設資料目錄（空字串代表「一定要給」）
launcher_body() {
  local default=$1
  cat <<EOF
if [ \$# -ge 1 ]; then ORIG=\$1; shift; else ORIG="$default"; fi
if [ -z "\$ORIG" ]; then
  fail "用法：\$(basename "\$0") <原版資料目錄>（該目錄要含 MM2.EXE）"
fi
for f in $REQUIRED; do
  if [ ! -f "\$ORIG/\$f" ]; then fail "原版資料缺少 \$f：\$ORIG"; fi
done
mkdir -p "\$STATE/data" "\$STATE/save"
ln -sfn "\$APP/assets" "\$STATE/assets"
ln -sfn "\$APP/translations" "\$STATE/translations"
cp -f "\$APP"/data/*.json "\$STATE/data/"
chmod -R u+w "\$STATE/data"
"\$APP/bin/mm2data" -exe "\$ORIG/MM2.EXE" -spells-dat "\$ORIG/SPELLS.DAT" -play-ovl "\$ORIG/2PLAY.OVL" -out "\$STATE/data"
MUSIC=
if [ -f "\$APP/music/manifest.json" ]; then MUSIC="\$APP/music/manifest.json"; fi
export MM2_DATA_DIR="\$STATE/data"
cd "\$STATE"
if [ -n "\$MUSIC" ]; then
  exec "\$APP/bin/mm2" -data "\$ORIG" -music-pack "\$MUSIC" "\$@"
fi
exec "\$APP/bin/mm2" -data "\$ORIG" "\$@"
EOF
}

case "$PLATFORM" in
linux-x64)
  [[ -f "$RUNTIME" ]] || { echo "缺少 AppImage runtime：$RUNTIME" >&2; exit 1; }
  APPDIR=/tmp/AppDir; rm -rf "$APPDIR"; mkdir -p "$APPDIR/usr"
  cp -a "$STAGE/." "$APPDIR/usr/"
  cp "$APPDIR/usr/assets/icon/mm2-256.png" "$APPDIR/mm2-cht.png"
  mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"
  cp "$APPDIR/mm2-cht.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/mm2-cht.png"
  ln -sf mm2-cht.png "$APPDIR/.DirIcon"
  cat > "$APPDIR/mm2-cht.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=Might and Magic II 繁體中文版
Name[zh_TW]=魔法門 II：另一個世界之門 繁體中文版
Comment=New World Computing 1988 年作品的繁體中文 remake
Exec=AppRun
Icon=mm2-cht
Categories=Game;RolePlaying;
Terminal=false
EOF
  DEFAULT=""; [[ "$MODE" == local-full ]] && DEFAULT='$APP/original-data'
  {
    cat <<'EOF'
#!/bin/sh
# AppImage 的進入點。掛載點每次都不一樣，所以路徑一律由這支自己算。
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
APP="$HERE/usr"
STATE="${XDG_DATA_HOME:-$HOME/.local/share}/mm2-cht"
fail() { echo "$*" >&2; exit 1; }
EOF
    launcher_body "$DEFAULT"
  } > "$APPDIR/AppRun"
  chmod +x "$APPDIR/AppRun"
  SFS=/tmp/mm2.squashfs; rm -f "$SFS"
  mksquashfs "$APPDIR" "$SFS" -root-owned -noappend -no-progress -quiet \
    -comp gzip -b 128K -mkfs-time 0 -all-time 0
  cat "$RUNTIME" "$SFS" > "$OUT/$PKG.AppImage"
  chmod +x "$OUT/$PKG.AppImage"
  rm -rf "$APPDIR" "$SFS"
  echo "[package] $OUT/$PKG.AppImage"
  ;;
windows-x64)
  if [[ "$MODE" == public ]]; then WIN_ORIG='set "ORIG=%~1"'; else WIN_ORIG='if "%~1"=="" (set "ORIG=%ROOT%original-data") else (set "ORIG=%~1")'; fi
  cat > "$STAGE/run.bat" <<'EOF'
@echo off
setlocal
set "ROOT=%~dp0"
EOF
  printf '%s\r\n' "$WIN_ORIG" >> "$STAGE/run.bat"
  cat >> "$STAGE/run.bat" <<EOF
if "%ORIG%"=="" (echo Usage: run.bat ^<original data folder^>& exit /b 1)
for %%F in ($REQUIRED) do if not exist "%ORIG%\\%%F" (echo Missing %%F.& exit /b 1)
if not "%~2"=="" (echo Extra arguments are not supported by this launcher.& exit /b 1)
"%ROOT%bin\\mm2data.exe" -exe "%ORIG%\\MM2.EXE" -spells-dat "%ORIG%\\SPELLS.DAT" -play-ovl "%ORIG%\\2PLAY.OVL" -out "%ROOT%data"
set "MM2_DATA_DIR=%ROOT%data"
cd /d "%ROOT%"
if exist "%ROOT%music\\manifest.json" ("%ROOT%bin\\mm2.exe" -data "%ORIG%" -music-pack "%ROOT%music\\manifest.json") else ("%ROOT%bin\\mm2.exe" -data "%ORIG%")
EOF
  # batch 檔在 Windows 上要 CRLF，不然 `for` 的續行會被吃掉。
  python3 - "$STAGE/run.bat" <<'PY'
import sys
p = sys.argv[1]
b = open(p, 'rb').read().replace(b'\r\n', b'\n').replace(b'\n', b'\r\n')
open(p, 'wb').write(b)
PY
  # DLL 清單由**實際的 PE 匯入表**產生，不是抄來的。
  {
    cat <<'EOF'
這個包不夾帶任何 DLL —— 下面說明為什麼，以及缺了東西會怎樣。

bin/mm2.exe 的 PE 匯入表（由 tools/pe_imports.py 從這個包裡的執行檔實際讀出）：

EOF
    python3 "$ROOT/tools/pe_imports.py" "$STAGE/bin/mm2.exe" | sed 's/^/  /'
    cat <<'EOF'

只有 kernel32.dll。執行檔是純 Go 編的（CGO_ENABLED=0），沒有 C 執行期
（runtime），所以不需要 MSVC redistributable，也沒有第三方 DLL 可以夾帶。

其餘的 DLL 由 Ebiten 在執行時視情況用 LoadLibrary 載入，全部是 Windows
自己的系統元件，隨作業系統安裝：

  d3d11.dll、dxgi.dll、d3dcompiler_47.dll        DirectX 11 繪圖路徑
  opengl32.dll                                   OpenGL 繪圖路徑（DirectX 不成時的退路）
  user32.dll、gdi32.dll、imm32.dll、shcore.dll   視窗、輸入法與 DPI
  winmm.dll、ole32.dll                           音訊與 COM
  xinput1_4.dll、xinput9_1_0.dll、dinput8.dll    搖桿

其中只有 d3dcompiler_47.dll 有機會缺席（它屬微軟，隨 Windows 10／11 內建，
本專案不代為散布）。真的缺了，Ebiten 會自動退回 OpenGL，遊戲照樣跑。
EOF
  } > "$STAGE/WINDOWS-DLL.txt"
  python3 "$ROOT/tools/pack_zip.py" "$STAGE" "$OUT/$PKG.zip"
  ;;
macos-universal)
  APPROOT=/tmp/macpkg; rm -rf "$APPROOT"
  BUNDLE="$APPROOT/$PKG/MM2-CHT.app"
  mkdir -p "$BUNDLE/Contents/MacOS" "$BUNDLE/Contents/Resources"
  cp -a "$STAGE/." "$BUNDLE/Contents/Resources/"
  cp "$STAGE/assets/icon/mm2.icns" "$BUNDLE/Contents/Resources/mm2.icns"
  VER=$(git -C "$ROOT" rev-list --count HEAD)
  cat > "$BUNDLE/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>MM2-CHT</string>
  <key>CFBundleDisplayName</key><string>魔法門 II 繁體中文版</string>
  <key>CFBundleIdentifier</key><string>io.github.wicanr2.mm2cht</string>
  <key>CFBundleExecutable</key><string>mm2-cht</string>
  <key>CFBundleIconFile</key><string>mm2.icns</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>0.$VER</string>
  <key>CFBundleVersion</key><string>$VER</string>
  <key>LSMinimumSystemVersion</key><string>10.15</string>
  <key>LSApplicationCategoryType</key><string>public.app-category.role-playing-games</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
EOF
  DEFAULT=""; [[ "$MODE" == local-full ]] && DEFAULT='$APP/original-data'
  {
    cat <<'EOF'
#!/bin/sh
# `.app` 的進入點。雙擊時沒有終端機，所以錯誤要用對話框講，
# 資料目錄也要用選擇器問 —— 問過一次就記在設定檔裡，之後不再問。
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
APP="$HERE/../Resources"
STATE="$HOME/Library/Application Support/mm2-cht"
CONF="$STATE/data-dir"
fail() {
  if [ -t 2 ]; then echo "$*" >&2; else
    osascript -e "display dialog \"$*\" buttons {\"好\"} with icon stop" >/dev/null 2>&1 || true
  fi
  exit 1
}
mkdir -p "$STATE"
EOF
    if [[ "$MODE" == public ]]; then
      cat <<'EOF'
if [ $# -eq 0 ] && [ -f "$CONF" ]; then set -- "$(cat "$CONF")"; fi
if [ $# -eq 0 ]; then
  set -- "$(osascript -e 'POSIX path of (choose folder with prompt "選擇《Might and Magic II》原版資料目錄（要含 MM2.EXE）")' 2>/dev/null || true)"
fi
EOF
    fi
    launcher_body "$DEFAULT" | sed 's|^export MM2_DATA_DIR|printf "%s" "$ORIG" > "$CONF"\nexport MM2_DATA_DIR|'
  } > "$BUNDLE/Contents/MacOS/mm2-cht"
  chmod +x "$BUNDLE/Contents/MacOS/mm2-cht"
  cat > "$APPROOT/$PKG/README-macOS.txt" <<'EOF'
《魔法門 II》繁體中文版 — macOS

雙擊 MM2-CHT.app 即可。第一次會跳出資料夾選擇器，指到你自己那份原版
Might and Magic II 的資料目錄（裡面要有 MM2.EXE）；選過一次就記住了。
（完整版的包已經內含資料，不會問。）

這個 app 沒有 Apple 開發者簽章，也沒有經過公證（notarization），所以第一次
打開時系統會擋。兩種放行方式擇一：

  在「系統設定 → 隱私權與安全性」按「仍要打開」，或在終端機執行
  xattr -dr com.apple.quarantine /Applications/MM2-CHT.app

想從終端機啟動並自己指定資料目錄：

  ./MM2-CHT.app/Contents/MacOS/mm2-cht /path/to/MM2

執行檔是 universal（x86_64 + arm64），Intel 與 Apple Silicon 都能跑。
可以用 lipo -info MM2-CHT.app/Contents/Resources/bin/mm2 自己確認。
EOF
  python3 "$ROOT/tools/pack_zip.py" "$APPROOT/$PKG" "$OUT/$PKG.zip"
  rm -rf "$APPROOT"
  ;;
*) exit 2 ;;
esac
rm -rf "$(dirname "$STAGE")"
