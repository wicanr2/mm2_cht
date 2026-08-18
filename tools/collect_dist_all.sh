#!/usr/bin/env bash
# 把三平台的封裝與推廣片集中到 dist-all/。
#
# **這個目錄不是上傳目錄。** local-full 的包內含原版資料，推廣片的畫面與配樂
# 也是原版衍生內容 —— 兩者都只能留在本機。可以對外的只有 public/ 那三個檔，
# 而且要照 docs/release.md 從乾淨 repo 走。同一句話也寫在產出的 README 裡，
# 因為半年後看到這個目錄的人不會回頭讀這支腳本。
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT="$ROOT/dist-all"
git -C "$ROOT" check-ignore -q "$OUT/" || { echo "dist-all 未被 Git 忽略" >&2; exit 1; }
STAMP=$(git -C "$ROOT" rev-parse --short=12 HEAD)

rm -rf "$OUT"; mkdir -p "$OUT/public" "$OUT/local-full" "$OUT/promo"
copy_mode() {
  local src=$1 dst=$2 n=0
  for p in linux-x64 windows-x64 macos-universal; do
    for f in "$src/$p"/*"$STAMP"*; do
      [[ -e "$f" ]] || continue
      cp -p "$f" "$dst/"; n=$((n + 1))
    done
  done
  echo "$n"
}
PUB=$(copy_mode "$ROOT/dist/public" "$OUT/public")
LOC=$(copy_mode "$ROOT/.local-full" "$OUT/local-full")
PROMO=0
for f in "$ROOT/workplace/promo"/*.mp4; do
  [[ -e "$f" ]] || continue
  cp -p "$f" "$OUT/promo/"; PROMO=$((PROMO + 1))
done

cd "$OUT"
find . -type f ! -name SHA256SUMS.txt ! -name README.md -print0 \
  | sort -z | xargs -0 sha256sum > SHA256SUMS.txt

{
  cat <<EOF
# dist-all — 本機集中區

commit \`$STAMP\`。三平台各一份封裝、外加推廣片。校驗值在 \`SHA256SUMS.txt\`。

## 哪些能公開，哪些不能

| 目錄 | 內容 | 能不能對外 |
|---|---|---|
| \`public/\` | 引擎、譯文、字型、圖示 | **可以**，但要照 [\`docs/release.md\`](../docs/release.md) 從乾淨 repo 走 |
| \`local-full/\` | 上面那些 ＋ 原版資料 ＋ 本機音樂包 | **不可以**。整包含原版檔案 |
| \`promo/\` | 72 秒推廣片 | **不可以**。畫面是原版素材，配樂是原版衍生 |

公開那三個檔玩家要自備合法原版資料；\`local-full\` 解開就能玩，但只給自己。

## 怎麼跑

| 平台 | 檔案 | 啟動 |
|---|---|---|
| Linux | \`*.AppImage\` | \`chmod +x\` 之後 \`./檔名.AppImage /path/to/MM2\`（完整版不必給路徑）|
| Windows | \`*-windows-x64-*.zip\` | 解開後 \`run.bat C:\\MM2\`（完整版直接雙擊 \`run.bat\`）|
| macOS | \`*-macos-universal-*.zip\` | 解開後雙擊 \`MM2-CHT.app\`，第一次會問原版資料在哪 |

AppImage 需要系統有 FUSE；沒有的話用 \`./檔名.AppImage --appimage-extract-and-run /path/to/MM2\`。
macOS 沒有簽章與公證，第一次開要在「隱私權與安全性」按「仍要打開」，
或 \`xattr -dr com.apple.quarantine MM2-CHT.app\`。

## 這次收了什麼

EOF
  printf -- '- public：%s 個檔\n- local-full：%s 個檔\n- 推廣片：%s 個檔\n\n' "$PUB" "$LOC" "$PROMO"
  echo '```'
  find . -type f ! -name SHA256SUMS.txt ! -name README.md -printf '%12s  %P\n' | sort -k2
  echo '```'
} > README.md

echo "[dist-all] public=$PUB local-full=$LOC promo=$PROMO → $OUT"
du -sh "$OUT"
