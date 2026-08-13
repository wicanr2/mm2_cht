#!/usr/bin/env bash
# 逐首擷取 Mega Drive 配樂：改寫選曲立即值 → 開機錄 VGM → 轉標準 PCM WAV。
#
#   tools/md_music_dump.sh                 錄全部 18 首
#   tools/md_music_dump.sh 0B8224          只錄這一首
#   SECONDS_PER_TRACK=60 tools/md_music_dump.sh
#
# 產物落在 workplace/genesis/music/（已 gitignore）：每首一個 .vgm、一個 .wav，
# 外加 manifest.txt 記錄可重跑所需的全部中介資料。
#
# [HARD] ROM、VGM、WAV 都是原版衍生物，不得 commit、push 或放進公開釋出包。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROMDIR="$ROOT/workplace/genesis"
OUT="$ROMDIR/music"
PATCHED="$ROMDIR/patched"
IMAGE="${BLASTEM_IMAGE:-mm2-blastem:0.6.3-pre-732f5689d438}"
GOIMAGE="${GO_IMAGE:-mm2-go:latest}"
ROM_NAME="${BLASTEM_ROM:-Might and Magic - Gates to Another World (USA, Europe).md}"
SECS="${SECONDS_PER_TRACK:-45}"

# 18 首曲目的起點。來源是本專案自己的反組譯，不是外部曲庫：
# 每首前 4 bytes 是結束指標，且 18 個 move.l (sp)+,-$37B0(a5) 前面全是
# move.l #<這些位址>,(sp)。見 docs/research/md-music-driver.md。
SONGS=(
    0AF59C 0B1370 0B2290 0B48D4 0B60CC 0B61DC
    0B8224 0B885C 0B8AE0 0B9718 0B9888 0B9A04
    0BA608 0BAE7C 0BBF68 0BC990 0BD078 0BE238
)

[ -f "$ROMDIR/$ROM_NAME" ] || { echo "找不到 ROM：$ROMDIR/$ROM_NAME" >&2; exit 1; }
mkdir -p "$OUT" "$PATCHED"

if [ $# -gt 0 ]; then
    SONGS=("$@")
fi

ROM_SHA="$(sha256sum "$ROMDIR/$ROM_NAME" | cut -d' ' -f1)"
BUILD_ID="$(docker run --rm --network none -u "$(id -u):$(id -g)" \
    --entrypoint cat "$IMAGE" /opt/blastem/BUILD_ID)"
LIBVGM="$(docker run --rm --network none -u "$(id -u):$(id -g)" \
    --entrypoint cat "$IMAGE" /opt/blastem/LIBVGM_COMMIT)"

MANIFEST="$OUT/manifest.txt"
{
    echo "# Mega Drive 配樂擷取"
    echo "原始 ROM        : $ROM_NAME"
    echo "原始 ROM SHA-256: $ROM_SHA"
    echo "模擬器          : $BUILD_ID"
    echo "libvgm commit   : $LIBVGM"
    echo "每首錄製秒數    : $SECS"
    echo "觸發方式        : tools/md_patch_song.py 把 18 處選曲立即值與 vblank"
    echo "                  閒置路徑（0x06CB10）全部改寫成目標曲目位址，開機即播"
    echo
    printf '%-10s %-10s %8s  %-64s %s\n' 曲目 大小 WAV秒數 VGM-SHA256 WAV-SHA256
} > "$MANIFEST"

for song in "${SONGS[@]}"; do
    name="md-$song"
    echo "=== $name ==="

    docker run --rm --network none --memory 512m --cpus 1 --pids-limit 128 \
        --log-opt max-size=10m --log-opt max-file=3 \
        -u "$(id -u):$(id -g)" -e HOME=/tmp \
        -v "$ROMDIR:/rom:ro" -v "$PATCHED:/patched" -v "$ROOT/tools:/tools:ro" \
        -w /tmp "$GOIMAGE" \
        python3 /tools/md_patch_song.py "/rom/$ROM_NAME" "/patched/$name.md" "$song"

    # 開機到音樂穩定播放要幾秒；rec 之後再等 SECS 秒。
    docker run --rm --network none --memory 2g --cpus 2 --pids-limit 512 \
        --log-opt max-size=10m --log-opt max-file=3 \
        -u "$(id -u):$(id -g)" \
        -v "$PATCHED:/rom:ro" -v "$OUT:/out" \
        "$IMAGE" "$name.md" "wait:8;rec:$name;wait:$SECS;stop" \
        2>&1 | grep -E "停止記錄|沒有產生|正規化|轉檔失敗" || true

    rm -f "$PATCHED/$name.md"

    if [ -f "$OUT/$name.wav" ]; then
        secs="$(docker run --rm --network none -u "$(id -u):$(id -g)" -e HOME=/tmp \
            -v "$OUT:/out:ro" -w /tmp "$GOIMAGE" python3 -c "
import wave,sys
w=wave.open('/out/$name.wav','rb'); print('%.1f' % (w.getnframes()/w.getframerate()))")"
        printf '%-10s %-10s %8s  %-64s %s\n' \
            "$song" "$(stat -c%s "$OUT/$name.wav")" "$secs" \
            "$(sha256sum "$OUT/$name.vgm" | cut -d' ' -f1)" \
            "$(sha256sum "$OUT/$name.wav" | cut -d' ' -f1)" >> "$MANIFEST"
    else
        printf '%-10s %s\n' "$song" "擷取失敗" >> "$MANIFEST"
    fi
done

echo
echo "完成，見 $MANIFEST"
