#!/usr/bin/env bash
# Retired: the old collector deleted the entire dist-all tree.
set -euo pipefail
printf '%s\n' '舊版非版本化收集器已停用，不更動 dist-all。' \
  '桌面建置請用 tools/package.sh，Android 請在鎖版容器執行 tools/build_android.py。' \
  '交付規則與驗證入口見 docs/packaging.md。' >&2
exit 2
