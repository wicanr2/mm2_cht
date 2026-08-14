#!/usr/bin/env bash
# BlastEm 動態追蹤（docker）：下中斷點、按腳本送鍵、記錄每一次命中。
#
#   tools/md_trace.sh /out/trace.txt "--break 0xB620:選曲 --keys \"$BOOT\" --max-hits 40"
#
# 第一個參數是 log 檔（容器內路徑，`/out` 對應 workplace/genesis/out/），
# 其餘原樣傳給 md-trace。詳細旗標見 docker/blastem/md_trace.py。
#
# 走 GDB stub（`-D`）。原生除錯器 `-d` 在 headless 容器裡會靜默地不進除錯器。
#
# [HARD] 不改 ROM —— 這片有開機完整性檢查，改任一位元組（尾端 padding 除外）
# 就開不了機。要改執行時狀態用 RSP 的 `M` 封包。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROMDIR="$ROOT/workplace/genesis"
OUT="$ROMDIR/out"
IMAGE="${BLASTEM_IMAGE:-mm2-blastem:0.6.3-pre-732f5689d438}"
ROM_NAME="${BLASTEM_ROM:-Might and Magic - Gates to Another World (USA, Europe).md}"

LOG="${1:?用法: tools/md_trace.sh <log路徑> <md-trace 旗標...>}"
shift

[ -f "$ROMDIR/$ROM_NAME" ] || { echo "找不到 ROM：$ROMDIR/$ROM_NAME" >&2; exit 1; }
mkdir -p "$OUT"

docker run --rm --network none \
    --memory 2g --cpus 2 --pids-limit 512 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" \
    -e HOME=/work/home -e SDL_AUDIODRIVER=dummy \
    --entrypoint sh \
    -v "$ROMDIR:/rom:ro" \
    -v "$OUT:/out" \
    "$IMAGE" -c '
set -e
mkdir -p /work/home
# 沿用 blastem_run.sh 存下來的模擬器狀態檔與 SRAM。導航靠計時推進，在 GDB stub
# 底下會漂移（停在中斷點時模擬器時間凍結），所以到達目標畫面的路徑只走一次，
# 存成狀態檔，追蹤時用 `key:l` 載回來。
mkdir -p /work/home/.local/share/blastem/rom
cp /out/blastem-state/* /work/home/.local/share/blastem/rom/ 2>/dev/null || true
Xvfb :99 -screen 0 1024x768x24 -nolisten tcp &
sleep 1
export DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1
# BlastEm 對過長或含非 ASCII 的 ROM 檔名會建檔失敗，複製成固定短名再餵給它。
cp "/rom/'"$ROM_NAME"'" /work/rom.md
python3 -u /usr/local/bin/md-trace /work/rom.md "$@"
# 追蹤途中用 `key:grave` 存的狀態要寫回來，否則下一次載到的是舊的那一份 ——
# 症狀是「明明走的是新路線，命中的卻是上一輪的場景」。不能用 exec。
mkdir -p /out/blastem-state
cp /work/home/.local/share/blastem/rom/* /out/blastem-state/ 2>/dev/null || true
' sh --log "$LOG" "$@"
