#!/usr/bin/env bash
# BlastEm headless（docker）：跑 Mega Drive ROM，依 timeline 送鍵、錄 VGM、截圖。
#
#   tools/blastem_run.sh "wait:6;shot:01-title;rec:intro;wait:30;stop"
#
# timeline 的步驟格式見 docker/blastem/entrypoint.sh。輸出落在
# workplace/genesis/out/（已被 .gitignore 忽略）。
#
# ROM 由玩家自備，唯讀掛載；image 與本 repo 都不含任何原版資產。
#
# [HARD] VGM、WAV 與 ROM 都是原版衍生物，不得 commit、push 或放進公開釋出包。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROMDIR="$ROOT/workplace/genesis"
OUT="$ROMDIR/out"
IMAGE="${BLASTEM_IMAGE:-mm2-blastem:0.6.3-pre-732f5689d438}"
ROM_NAME="${BLASTEM_ROM:-Might and Magic - Gates to Another World (USA, Europe).md}"

TIMELINE="${1:?用法: tools/blastem_run.sh \"<timeline>\"}"

[ -f "$ROMDIR/$ROM_NAME" ] || { echo "找不到 ROM：$ROMDIR/$ROM_NAME" >&2; exit 1; }
mkdir -p "$OUT"

docker run --rm --network none \
    --memory 2g --cpus 2 --pids-limit 512 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" \
    -e KEY_HOLD="${KEY_HOLD:-0.15}" \
    -v "$ROMDIR:/rom:ro" \
    -v "$OUT:/out" \
    "$IMAGE" "$ROM_NAME" "$TIMELINE"
