#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DATA_DIR=
MD_MUSIC_DIR=$ROOT/workplace/genesis/music
while (($#)); do
  case "$1" in
    --data-dir) DATA_DIR=${2:?--data-dir 需要路徑}; shift 2 ;;
    --md-music-dir) MD_MUSIC_DIR=${2:?--md-music-dir 需要路徑}; shift 2 ;;
    --no-md-music) MD_MUSIC_DIR=; shift ;;
    *) echo "未知參數：$1" >&2; exit 2 ;;
  esac
done
[[ -d "$DATA_DIR" && -f "$DATA_DIR/MM2.EXE" ]] || {
  echo "請用 --data-dir 指定含 MM2.EXE 的合法原版資料目錄" >&2
  exit 1
}
DATA_DIR=$(cd "$DATA_DIR" && pwd)

OUT="$ROOT/workplace/promo"
git -C "$ROOT" check-ignore -q "$OUT" || {
  echo "推廣片輸出路徑沒有被 Git 忽略" >&2
  exit 1
}

docker run --rm --network none --memory 4g --cpus 2 --pids-limit 512 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp \
  -e GOCACHE=/gocache -e GOMODCACHE=/gomod -e GOPROXY=off \
  -v "$ROOT:/src" -v "$DATA_DIR:/original:ro" \
  -v mm2-gomod:/gomod -v mm2-gobuild:/gocache \
  -w /src mm2-go:latest \
  /usr/local/go/bin/go run ./cmd/mm2shots -data /original -out /src/workplace/promo/shots

mkdir -p "$OUT/music"
docker run --rm --network none --memory 1g --cpus 2 --pids-limit 256 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$ROOT:/src" -v "$DATA_DIR:/original:ro" \
  -v mm2-gomod:/gomod -v mm2-gobuild:/gocache \
  -w /src mm2-go:latest \
  /usr/local/go/bin/go run ./cmd/mm2music -exe /original/MM2.EXE \
    -out /src/workplace/promo/music/mm2-original-pc-speaker.wav -seconds 72

# Mega Drive 配樂變體（本機有音樂包才做）。兩首串成 72 秒，中間交疊兩秒。
# 這是原版 Mega Drive 的配樂，與 PC 喇叭那版一樣只留在本機，不對外散布。
PICK='import json,sys; m=json.load(open(sys.argv[1]))["tracks"]; o=[m.get("intro"), m.get("town")]; o = o if all(o) else list(m.values())[:2]; print(" ".join(o))'
if [[ -n "$MD_MUSIC_DIR" && -f "$MD_MUSIC_DIR/manifest.json" ]]; then
  read -r -a MD_TRACKS <<< "$(python3 -c "$PICK" "$MD_MUSIC_DIR/manifest.json")"
  if ((${#MD_TRACKS[@]} == 2)); then
    docker run --rm --network none --memory 2g --cpus 2 --pids-limit 256 \
      --log-opt max-size=10m --log-opt max-file=3 \
      -u "$(id -u):$(id -g)" -e HOME=/tmp \
      -v "$OUT:/out" -v "$MD_MUSIC_DIR:/md:ro" -w /out \
      --entrypoint ffmpeg game-video:latest \
      -hide_banner -loglevel error -y \
      -i "/md/${MD_TRACKS[0]}" -i "/md/${MD_TRACKS[1]}" \
      -filter_complex "[0:a][1:a]acrossfade=d=2:c1=tri:c2=tri[a]" -map "[a]" -t 72 \
      /out/music/mm2-megadrive-medley.wav
    echo "[promo] Mega Drive 配樂：${MD_TRACKS[0]} → ${MD_TRACKS[1]}"
  fi
fi

docker run --rm --network none --memory 2g --cpus 2 --pids-limit 256 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$OUT:/out" -v "$ROOT/tools:/tools:ro" -w /out \
  --entrypoint bash game-video:latest /tools/render-promo-container.sh

echo "[promo] 已產生 $OUT/mm2-remake-trailer.mp4"
if [[ -s "$OUT/mm2-remake-trailer-megadrive.mp4" ]]; then
  echo "[promo] 已產生 $OUT/mm2-remake-trailer-megadrive.mp4"
fi
