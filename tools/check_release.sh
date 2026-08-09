#!/usr/bin/env bash
# 釋出前的 deny-list 掃描（CLAUDE.md §9）。
#
# 檢查兩件事：
#   1. 工作區有沒有原版資產（執行檔、資料檔、美術、音樂、掃描）
#   2. git 歷史裡有沒有由原版產生的 JSON
#
# 回傳 0 表示乾淨，非 0 表示有東西不該散布。
set -uo pipefail
cd "$(dirname "$0")/.."
fail=0

say() { printf '%s\n' "$*"; }
bad() { fail=1; say "✗ $*"; }

# --- 1. 工作區：原版副檔名 -------------------------------------------------
# 這些副檔名只會出現在原版資料裡。workplace/ 與被 gitignore 的路徑不算。
tracked_bad=$(git ls-files -- '*.EXE' '*.exe' '*.OVL' '*.ovl' '*.DAT' '*.dat' \
    '*.16' '*.CH' '*.DRV' '*.drv' '*.rar' '*.zip' '*.dsk' 2>/dev/null)
if [ -n "$tracked_bad" ]; then
    bad "版控裡有原版資產副檔名的檔案："
    printf '    %s\n' $tracked_bad
else
    say "✓ 版控裡沒有原版資產副檔名"
fi

# --- 2. 工作區：由原版產生的 JSON 不該被追蹤 --------------------------------
# 判準是檔案自己說的：cmd/mm2data 產出的每一份都帶 "source" 欄位指向原版檔名。
generated=$(git ls-files 'data/*.json' 2>/dev/null | while read -r f; do
    if grep -qE '"source"[[:space:]]*:[[:space:]]*"[^"]*(MM2\.EXE|\.DAT|\.OVL)' "$f" 2>/dev/null; then
        printf '%s\n' "$f"
    fi
done)
if [ -n "$generated" ]; then
    bad "版控裡有由原版產生的資料："
    printf '    %s\n' $generated
else
    say "✓ 版控裡沒有由原版產生的資料"
fi

# --- 3. git 歷史 -----------------------------------------------------------
# 曾經被加進來、後來才 gitignore 的檔案仍留在歷史裡，公開之前要清掉。
hist=$(git log --all --diff-filter=A --name-only --format= -- 'data/*.json' 2>/dev/null \
    | sort -u | while read -r f; do
        [ -n "$f" ] || continue
        # 目前仍追蹤且不是原版產生的（手抄自手冊）就不算
        if git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
            grep -qE '"source"[[:space:]]*:[[:space:]]*"[^"]*(MM2\.EXE|\.DAT|\.OVL)' "$f" 2>/dev/null || continue
        fi
        printf '%s\n' "$f"
    done)
if [ -n "$hist" ]; then
    bad "git 歷史裡還留著由原版產生的資料（公開前必須清）："
    printf '    %s\n' $hist
    say ""
    say "  清理方式（會改寫歷史，需要 force push，動手前先取得同意）："
    say "    git filter-repo --invert-paths \\"
    for f in $hist; do say "      --path $f \\"; done
    say "      --force"
else
    say "✓ git 歷史乾淨"
fi

[ "$fail" -eq 0 ] && say "" && say "釋出檢查通過。"
exit "$fail"
