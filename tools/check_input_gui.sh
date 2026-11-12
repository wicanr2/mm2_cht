#!/usr/bin/env bash
# Docker-only, under dbus-run-session and xvfb-run in mm2-input-test:20261111.
set -euo pipefail
out=${1:?unique output directory}
test ! -e "$out"
test -x /src/workplace/bin/mm2
mkdir -p "$out/run" "$out/runtime" "$out/config" "$out/cache"
chmod 700 "$out/runtime"
out=$(cd "$out" && pwd)
export XDG_RUNTIME_DIR="$out/runtime" XDG_CONFIG_HOME="$out/config" XDG_CACHE_HOME="$out/cache"
export LANG=C.UTF-8 LC_ALL=C.UTF-8 XMODIFIERS=@im=ibus GTK_IM_MODULE=ibus QT_IM_MODULE=ibus
export IBUS_ADDRESS="unix:path=$out/runtime/ibus.sock"
for entry in assets translations data workplace; do ln -s "/src/$entry" "$out/run/$entry"; done
app_pid=; ibus_pid=
cleanup() {
    for pid in "$app_pid" "$ibus_pid"; do
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then kill -TERM "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi
    done
}
trap cleanup EXIT
ibus-daemon --address="$IBUS_ADDRESS" --xim --replace --panel=disable --config=default >"$out/ibus.log" 2>&1 &
ibus_pid=$!
ready=false
for attempt in $(seq 1 150); do
    if ibus list-engine 2>/dev/null | grep -q libpinyin; then ready=true; break; fi
    kill -0 "$ibus_pid"
    sleep 0.1
done
"$ready"
/usr/bin/python3 - <<'PY'
from gi.repository import Gio
Gio.Settings.new('org.freedesktop.ibus.general').set_boolean('use-system-keyboard-layout',True)
Gio.Settings.sync()
PY
set_engine() {
    # IBus 1.5.27 can report failure after actually changing the engine.
    # Verify the actual engine instead of treating that CLI status as proof.
    ibus engine "$1" >"$out/set-engine.log" 2>&1 || true
    test "$(ibus engine)" = "$1"
}
set_engine xkb:us::eng
cd "$out/run"
/src/workplace/bin/mm2 -data /src/workplace/orig/MM2 -amiga-dir /src/workplace/amiga \
    -hd-dir /src/workplace/hd-amiga-r1 -graphics hd -locale zh-Hans -music-theme off \
    -preferences "$out/preferences.json" >"$out/game.log" 2>&1 &
app_pid=$!
window=
for attempt in $(seq 1 200); do
    window=$(xdotool search --onlyvisible --pid "$app_pid" 2>/dev/null | head -1 || true)
    if [ -n "$window" ]; then break; fi
    kill -0 "$app_pid"
    sleep 0.1
done
test -n "$window"
xdotool windowfocus --sync "$window"
key() { xdotool key --clearmodifiers --delay 100 "$@"; sleep 0.4; }
shot() { import -window "$window" "$out/$1.png"; }
painted=false
for attempt in $(seq 1 100); do
    shot warmup
    if [ "$(identify -format '%[fx:mean>0.01?1:0]' "$out/warmup.png")" = 1 ]; then painted=true; break; fi
    kill -0 "$app_pid"
    sleep 0.1
done
"$painted"
key Return
key n
shot 00-create
for digit in 1 2 3 4 5 6 7 8; do key "$digit"; done
key Return
key Return
key Return
shot 01-name-empty
set_engine libpinyin
sleep 0.5
ibus engine >"$out/engine.txt"
xdotool type --delay 100 ceshi
sleep 0.4
shot 02-preedit
key space
xdotool type --delay 100 xin
key Escape
shot 02b-ime-cancel
xdotool type --delay 100 long
key BackSpace
xdotool type --delay 100 g
sleep 0.4
shot 03-preedit-second
key Return
shot 04-ime-confirm
# The first Enter must only commit IME text, leaving the name field open.
key Return
key F4
shot 05-created-saved
test -s save/state.json
cp save/state.json "$out/saved-before.json"
python3 - "$out" <<'PY'
import json,pathlib,sys
p=pathlib.Path(sys.argv[1]);d=json.loads((p/'saved-before.json').read_text())
assert d['version']==3
names=[c['Name'] for c in d['characters']['roster']]
assert names==['测试long'],names
print('Native XIM Pinyin, IME Enter ownership, Unicode character save passed')
PY
key F10
wait "$app_pid"
app_pid=
/src/workplace/bin/mm2 -data /src/workplace/orig/MM2 -amiga-dir /src/workplace/amiga \
    -hd-dir /src/workplace/hd-amiga-r1 -graphics hd -locale zh-Hans -music-theme off \
    -preferences "$out/preferences.json" >"$out/reload.log" 2>&1 &
app_pid=$!
for attempt in $(seq 1 200); do
    window=$(xdotool search --onlyvisible --pid "$app_pid" 2>/dev/null | head -1 || true)
    if [ -n "$window" ]; then break; fi
    kill -0 "$app_pid"
    sleep 0.1
done
test -n "$window"
xdotool windowfocus --sync "$window"
key Return
key F4
shot 06-reloaded
key F10
wait "$app_pid"
app_pid=
python3 - "$out" <<'PY'
import json,pathlib,sys
p=pathlib.Path(sys.argv[1]);a=json.loads((p/'saved-before.json').read_text());b=json.loads((p/'run/save/state.json').read_text())
assert a==b
(p/'checks.json').write_text(json.dumps({'native_xim_pinyin':True,'engine':'libpinyin','ime_enter_ownership':True,'ime_backspace_ownership':True,'ime_escape_ownership':True,'unicode_roster':True,'save_reload_equal':True},indent=2)+'\n')
print('Fresh-process Unicode roster reload passed')
PY
