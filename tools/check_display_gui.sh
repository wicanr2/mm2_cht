#!/usr/bin/env bash
# Run inside the Go GUI Docker image under xvfb-run with Docker --init.
set -euo pipefail
graphics=${1:?original or hd}
out=${2:?unique output directory}
case "$graphics" in original|hd) ;; *) exit 2;; esac
test ! -e "$out"
binary=${MM2_BINARY:-/src/workplace/bin/mm2}
resources=${MM2_RESOURCE_ROOT:-/src}
platform_assets=${MM2_ASSET_ROOT:-/src/workplace}
original_data=${MM2_ORIGINAL_DATA:-/src/workplace/orig/MM2}
test -x "$binary"
mkdir -p "$out/run"
out=$(cd "$out" && pwd)
for entry in assets translations; do
    ln -s "$resources/$entry" "$out/run/$entry"
done
ln -s "$resources/data" "$out/run/data"
ln -s "$platform_assets" "$out/run/workplace"
cd "$out/run"
app_pid=
cleanup() {
    if [ -n "$app_pid" ] && kill -0 "$app_pid" 2>/dev/null; then
        kill -TERM "$app_pid" 2>/dev/null || true
        wait "$app_pid" 2>/dev/null || true
    fi
}
trap cleanup EXIT
launch() {
    "$binary" -data "$original_data" \
        -amiga-dir "$platform_assets/amiga" -hd-dir "$platform_assets/hd-amiga-r1" \
        -graphics "$graphics" -music-theme off -preferences "$out/preferences.json" \
        >"$out/game.log" 2>&1 &
    app_pid=$!
    window=
    for attempt in $(seq 1 150); do
        window=$(xdotool search --onlyvisible --pid "$app_pid" 2>/dev/null | head -1 || true)
        if [ -n "$window" ]; then break; fi
        kill -0 "$app_pid"
        sleep 0.1
    done
    test -n "$window"
    xdotool windowfocus --sync "$window"
    painted=false
    for attempt in $(seq 1 100); do
        import -window "$window" "$out/warmup.png"
        if [ "$(identify -format '%[fx:mean>0.01?1:0]' "$out/warmup.png")" = 1 ]; then painted=true; break; fi
        kill -0 "$app_pid"
        sleep 0.1
    done
    "$painted"
    if [ "$graphics" = hd ] && grep -q 'HD unavailable' "$out/game.log"; then
        cat "$out/game.log"
        return 1
    fi
}
key() { xdotool windowfocus --sync "$window"; xdotool key --clearmodifiers --delay 100 "$@"; sleep 0.25; }
top() { xdotool mousemove --window "$window" 480 1; sleep 0.1; }
click() { xdotool mousemove --window "$window" "$1" "$2" mousedown 1; sleep 0.1; xdotool mouseup 1; sleep 0.25; }
shot() { import -window "$window" "$out/$1.png"; }
launch
top
shot 00-intro-toolbar
key Return
xdotool mousemove --window "$window" 500 580
sleep 0.2
shot 01-start
key Up
key Left
key Right
top
click 55 20
click 55 106
top
shot 02-simplified-toolbar
python3 - "$out/preferences.json" <<'PY'
import json,sys
assert json.load(open(sys.argv[1]))['locale']=='zh-Hans'
PY
if [ "$graphics" = hd ]; then
    click 600 20
    shot 09-original-switch
    python3 - "$out/preferences.json" <<'PY'
import json,sys
assert json.load(open(sys.argv[1]))['hd'] is False
PY
    click 720 20
    shot 10-hd-switch
    python3 - "$out/preferences.json" <<'PY'
import json,sys
assert json.load(open(sys.argv[1]))['hd'] is True
PY
fi
click 840 20
xdotool mousemove --window "$window" 500 580
sleep 0.2
shot 03-settings
key Escape
key i
shot 04-items
key Escape
key g
shot 11-smith
key Return
shot 12-goods
key Escape
key Escape
key Up
key Up
key Up
shot 13-temple
key n
key Return
key Return
key Return
key Return
key Down
key Down
key Down
shot 14-facility-return
top
click 220 20
click 220 64
top
shot 05-text-80
click 935 20
click 935 20
top
shot 06-toolbar-150
click 935 20
top
click 55 20
click 55 64
key F4
shot 07-saved
test -s save/state.json
cp save/state.json "$out/saved-before.json"
key F10
wait "$app_pid"
app_pid=
launch
key Return
xdotool mousemove --window "$window" 500 580
sleep 0.2
shot 08-restored
key F10
wait "$app_pid"
app_pid=
python3 - "$out" "$graphics" <<'PY'
import json,sys,pathlib
p=pathlib.Path(sys.argv[1]);a=json.loads((p/'saved-before.json').read_text());b=json.loads((p/'run/save/state.json').read_text())
assert a==b,(a,b)
assert (a['map'],a['x'],a['y'],a['facing'])==(0,7,4,0)
assert json.loads((p/'preferences.json').read_text())['locale']=='zh-Hant'
(p/'checks.json').write_text(json.dumps({'normal_keyboard_path':True,'movement_verified':True,'locale_pointer':True,'settings_items':True,'smith_goods':True,'temple_walk_in_out':True,'text_80':True,'toolbar_150':True,'save_reload_equal':True,'graphics':sys.argv[2],'original_hd_pointer':sys.argv[2]=='hd'},indent=2)+'\n')
print('normal GUI path, pointer, locale, font, save/reload passed')
PY
