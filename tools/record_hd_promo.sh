#!/usr/bin/env bash
# Run under xvfb-run in the packaging Docker image, with --init.
set -euo pipefail
app=${1:?packaged resources}
out=${2:?new recording directory}
test ! -e "$out"
mkdir -p "$out/run" "$out/config"
out=$(cd "$out" && pwd)
for entry in assets translations data; do ln -s "$app/$entry" "$out/run/$entry"; done
export MM2_ASSET_ROOT="$app/workplace" XDG_CONFIG_HOME="$out/config" MM2_DATA_DIR="$app/data"
cd "$out/run"
game_pid=; recorder_pid=
cleanup() {
  for pid in "$recorder_pid" "$game_pid"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then kill -TERM "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi
  done
}
trap cleanup EXIT
"$app/bin/mm2" -data "$app/original-data" -music-pack "$app/music/manifest.json" -preferences "$out/preferences.json" >"$out/game.log" 2>&1 &
game_pid=$!
window=
for attempt in $(seq 1 200); do
  window=$(xdotool search --onlyvisible --pid "$game_pid" 2>/dev/null | head -1 || true)
  if [ -n "$window" ]; then break; fi
  kill -0 "$game_pid"; sleep .1
done
test -n "$window"
xdotool windowfocus --sync "$window" windowmove "$window" 0 0
for attempt in $(seq 1 100); do
  import -window "$window" "$out/warmup.png"
  if [ "$(identify -format '%[fx:mean>0.01?1:0]' "$out/warmup.png")" = 1 ]; then break; fi
  sleep .1
done
key() { xdotool key --clearmodifiers --delay 100 "$@"; sleep .35; }
top() { xdotool mousemove --window "$window" 480 1; sleep .1; }
click() { xdotool mousemove --window "$window" "$1" "$2" mousedown 1; sleep .12; xdotool mouseup 1; sleep .4; }
record() {
  ffmpeg -nostdin -v error -y -f x11grab -framerate 30 -video_size 960x640 -draw_mouse 1 -i "$DISPLAY+0,0" -t "$1" -an -c:v libx264 -crf 16 -preset veryfast -threads 2 "$out/$2.mkv" >"$out/$2-record.log" 2>&1 &
  recorder_pid=$!
}
finish() { wait "$recorder_pid"; recorder_pid=; }
top
record 10 00-intro
finish
key Return
xdotool mousemove --window "$window" 950 630
record 10 01-explore
key Up; key Left; key Right; key Down; key Left; key Right; key Up
finish
record 10 02-toolbar
top; click 55 20; click 55 106
sleep 1
top; click 600 20
sleep 2
top; click 720 20
sleep 1
click 220 20; click 220 64
finish
key Escape
xdotool mousemove --window "$window" 950 630
key g
record 10 03-facility
sleep 2
key Return
sleep 2
key Escape; key Escape
key Up; key Up; key Up
finish
key n; key Escape; key w
record 8 04-map
sleep 3
key Right
sleep 2
key Left
finish
key Escape; key F4
import -window "$window" "$out/saved.png"
key F10
wait "$game_pid"; game_pid=
printf '%s\n' 'Normal GUI capture; no teleport, item grants or forced victories.' > "$out/CAPTURE.txt"
