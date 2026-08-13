#!/usr/bin/env bash
# 逐首擷取 Mega Drive 配樂：GDB stub 指定曲目 → 錄 VGM → 轉標準 PCM WAV。
#
#   tools/md_music_dump.sh                 錄全部 18 首
#   tools/md_music_dump.sh 0B8224          只錄這一首
#   SECONDS_PER_TRACK=60 tools/md_music_dump.sh
#
# ROM 一個位元組都不改 —— 這片有開機時的完整性檢查，改了就開不了機
# （見 docs/research/md-music-driver.md）。曲目是在執行時寫進 RAM 指定的。
#
# 產物落在 workplace/genesis/music/（已 gitignore）：每首一個 .vgm、一個 .wav，
# 外加 manifest.txt 記錄可重跑所需的全部中介資料。
#
# [HARD] ROM、VGM、WAV 都是原版衍生物，不得 commit、push 或放進公開釋出包。
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROMDIR="$ROOT/workplace/genesis"
OUT="$ROMDIR/music"
IMAGE="${BLASTEM_IMAGE:-mm2-blastem:0.6.3-pre-732f5689d438}"
GOIMAGE="${GO_IMAGE:-mm2-go:latest}"
ROM_NAME="${BLASTEM_ROM:-Might and Magic - Gates to Another World (USA, Europe).md}"
SECS="${SECONDS_PER_TRACK:-40}"

# 18 首曲目的起點。來源是本專案自己的反組譯，不是外部曲庫：
# 每首前 4 bytes 是結束指標，且 0x0B620 那支跳表 switch 的 21 個 case
# 全部指到這些位址。見 docs/research/md-music-driver.md。
SONGS=(
    0AF59C 0B1370 0B2290 0B48D4 0B60CC 0B61DC
    0B8224 0B885C 0B8AE0 0B9718 0B9888 0B9A04
    0BA608 0BAE7C 0BBF68 0BC990 0BD078 0BE238
)

[ -f "$ROMDIR/$ROM_NAME" ] || { echo "找不到 ROM：$ROMDIR/$ROM_NAME" >&2; exit 1; }
mkdir -p "$OUT"

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
    echo "觸發方式        : GDB remote stub（blastem -D）在 0x06CB02 下中斷點，"
    echo "                  每幀把 RAM \$FFCB62（要播哪一首）寫成目標曲目位址，"
    echo "                  壓 40 幀確認 \$FFCB5E（正在播）已切過去才開始錄。"
    echo "                  ROM 一個位元組都沒有改。"
    echo
} > "$MANIFEST"

ok=0; fail=0
for song in "${SONGS[@]}"; do
    name="md-$song"
    echo "=== $name ==="

    timeout 300 docker run --rm --network none --memory 2g --cpus 2 --pids-limit 512 \
        --log-opt max-size=10m --log-opt max-file=3 \
        -u "$(id -u):$(id -g)" \
        --entrypoint /usr/local/bin/blastem-music-entrypoint.sh \
        -v "$ROMDIR:/rom:ro" -v "$OUT:/out" \
        "$IMAGE" "$ROM_NAME" "$song" "$name" "$SECS" \
        2>&1 | grep -vE "^Failed to set vsync|^\[blastem\] ROM SHA" || true

    if [ -f "$OUT/$name.wav" ]; then
        ok=$((ok + 1))
        read -r secs peak rms <<< "$(docker run --rm --network none \
            -u "$(id -u):$(id -g)" -e HOME=/tmp -v "$OUT:/out:ro" -w /tmp "$GOIMAGE" \
            python3 -c "
import struct, math
d=open('/out/$name.wav','rb').read()
i=12
while i < len(d)-8:
    cid=d[i:i+4]; sz=struct.unpack_from('<I',d,i+4)[0]
    if cid==b'fmt ': ch,sr=struct.unpack_from('<HI',d,i+10)[0],struct.unpack_from('<I',d,i+12)[0]
    if cid==b'data':
        s=struct.unpack_from('<%dh'%(sz//2), d, i+8)
        print('%.1f %d %.0f' % (sz/2/ch/sr, max(abs(x) for x in s), math.sqrt(sum(x*x for x in s)/len(s))))
        break
    i += 8+sz+(sz&1)
")"
        printf '%-10s %8s秒  峰值 %6s  RMS %6s  vgm=%s  wav=%s\n' \
            "$song" "$secs" "$peak" "$rms" \
            "$(sha256sum "$OUT/$name.vgm" | cut -d' ' -f1 | cut -c1-16)" \
            "$(sha256sum "$OUT/$name.wav" | cut -d' ' -f1 | cut -c1-16)" >> "$MANIFEST"
        echo "  $secs 秒，峰值 $peak，RMS $rms"
    else
        fail=$((fail + 1))
        printf '%-10s 擷取失敗\n' "$song" >> "$MANIFEST"
        echo "  失敗"
    fi
done

echo
echo "完成 $ok 首、失敗 $fail 首，見 $MANIFEST"
