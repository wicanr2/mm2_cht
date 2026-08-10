#!/usr/bin/env bash
# 釋出前的 deny-list 掃描（CLAUDE.md §9）。
#
#   1. 工作區有沒有原版資產（執行檔、資料檔、美術、音樂、掃描）
#   2. 版控裡有沒有由原版產生的 JSON
#   3. git 歷史裡有沒有由原版產生的 JSON
#
# 前兩項在哪裡跑都是硬性失敗。第三項要看跑在哪個 repo：
#
#   私有工作 repo   歷史留著是**已決定接受的現狀**，只報告不擋。
#   公開 repo       歷史必須乾淨，帶 --public 跑，有東西就失敗。
#
# 公開的做法是**另開一份乾淨 repo**（fresh init 或 squash），不改寫
# 既有歷史 —— 決定與理由見 docs/release.md。
#
# 回傳 0 表示可以照該情境釋出，非 0 表示有東西不該散布。
set -uo pipefail
cd "$(dirname "$0")/.."
fail=0
public=0
[ "${1:-}" = "--public" ] && public=1

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
# 曾經被加進來、後來才 gitignore 的檔案仍留在歷史裡。
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
    if [ "$public" -eq 1 ]; then
        bad "公開 repo 的歷史裡有由原版產生的資料："
        printf '    %s\n' $hist
        say ""
        say "  公開 repo 必須是乾淨的一份 —— 不要在這裡改寫歷史，"
        say "  改用 fresh init 重新建（步驟見 docs/release.md）。"
    else
        say "· git 歷史裡留著由原版產生的資料（$(printf '%s ' $hist))"
        say "  這是**已決定接受的現狀**：私有工作 repo 保留完整開發歷史，"
        say "  公開時另開一份乾淨 repo。理由與步驟見 docs/release.md。"
        say "  要驗公開那一份，在該 repo 裡跑：tools/check_release.sh --public"
    fi
else
    say "✓ git 歷史乾淨"
fi

if [ "$fail" -eq 0 ]; then
    say ""
    if [ "$public" -eq 1 ]; then
        say "公開釋出檢查通過。"
    else
        say "工作 repo 檢查通過。公開前記得在乾淨 repo 上跑 --public。"
    fi
fi
exit "$fail"
