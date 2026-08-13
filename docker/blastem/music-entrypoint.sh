#!/usr/bin/env bash
# 逐首擷取模式：起 Xvfb → 用 GDB stub 指定曲目錄 VGM → 轉標準 PCM WAV。
#
#   music-entrypoint.sh <rom檔名> <曲目位址hex> <輸出名> [錄製秒數]
#
# ROM 唯讀掛在 /rom，產物寫到 /out。ROM 一個位元組都不改 —— 這片有開機時的
# 完整性檢查，改了就開不了機（見 docs/research/md-music-driver.md）。
set -uo pipefail

ROM_NAME="${1:?用法: music-entrypoint.sh <rom> <曲目hex> <輸出名> [秒數]}"
SONG="${2:?缺少曲目位址}"
NAME="${3:?缺少輸出名}"
SECS="${4:-45}"

WAV_RATE="${WAV_RATE:-48000}"
WAV_LOOPS="${WAV_LOOPS:-1}"

export DISPLAY=:99
export SDL_AUDIODRIVER="${SDL_AUDIODRIVER:-dummy}"
export LIBGL_ALWAYS_SOFTWARE=1
export HOME=/work/home
mkdir -p "$HOME"

echo "[blastem] 版本 $(cat /opt/blastem/BUILD_ID)"
[ -f "/rom/$ROM_NAME" ] || { echo "[blastem] 找不到 /rom/$ROM_NAME" >&2; exit 2; }

# 檔名太長或含非 ASCII 會讓 BlastEm 建檔失敗，複製成短的 ASCII 名。
cp "/rom/$ROM_NAME" /work/rom.md
echo "[blastem] ROM SHA-256: $(sha256sum /work/rom.md | cut -d' ' -f1)"

Xvfb :99 -screen 0 640x480x24 -nolisten tcp &
XVFB_PID=$!
sleep 1
trap 'kill "$XVFB_PID" 2>/dev/null' EXIT

python3 /usr/local/bin/md-music-dump /work/rom.md "$SONG" "/out/${NAME}.vgm" \
    --record-seconds "$SECS" --vgm-dir "$HOME" || exit $?

if vgm2wav --samplerate "$WAV_RATE" --loops "$WAV_LOOPS" \
        "/out/${NAME}.vgm" "/work/${NAME}.raw.wav" >/dev/null 2>&1; then
    vgm2pcmwav "/work/${NAME}.raw.wav" "/out/${NAME}.wav"
    rm -f "/work/${NAME}.raw.wav"
else
    echo "[blastem] vgm2wav 轉檔失敗：${NAME}" >&2
    exit 4
fi
