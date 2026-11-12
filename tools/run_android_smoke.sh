#!/usr/bin/env bash
# 只在既有 Android 模擬器容器內執行。AVD 在 /tmp，容器退出即清除。
set -euo pipefail
[[ -f /.dockerenv && -d /out && -f /src/tools/android_smoke.py ]] || exit 2
export HOME=/tmp/mm2-android-home ANDROID_AVD_HOME=/tmp/mm2-avd
ANDROID_SMOKE_API="${ANDROID_SMOKE_API:-35}"
[[ "$ANDROID_SMOKE_API" == 23 || "$ANDROID_SMOKE_API" == 35 ]] || exit 2
export ANDROID_SMOKE_API
export ANDROID_USER_HOME="$HOME/.android" ANDROID_EMULATOR_HOME="$HOME/.android"
export ANDROID_SMOKE_OUT="/out/emulator-api$ANDROID_SMOKE_API"
mkdir -p "$HOME/.android" "$ANDROID_AVD_HOME" "$ANDROID_SMOKE_OUT"
image_abi=x86_64
[[ "$ANDROID_SMOKE_API" != 23 ]] || image_abi=x86
printf 'no\n' | avdmanager create avd --force -n mm2-phone -k "system-images;android-$ANDROID_SMOKE_API;default;$image_abi" --device pixel >"$ANDROID_SMOKE_OUT/avd.log" 2>&1
python3 - <<'PY'
from pathlib import Path
import os
p=Path(os.environ['ANDROID_AVD_HOME'])/'mm2-phone.avd/config.ini'
values={'hw.lcd.width':'780','hw.lcd.height':'1688','hw.lcd.density':'320','hw.ramSize':'2048','hw.gpu.enabled':'yes','hw.gpu.mode':'swiftshader_indirect','disk.dataPartition.size':'1536M','showDeviceFrame':'no','skin.name':'780x1688','skin.dynamic':'yes'}
lines=[line for line in p.read_text().splitlines() if line.split('=')[0].strip() not in values]
p.write_text('\n'.join(lines+[k+'='+v for k,v in values.items()])+'\n')
PY
emulator -avd mm2-phone -no-window -no-audio -no-boot-anim -no-snapshot -accel off -gpu swiftshader_indirect -feature -Vulkan -cores 2 -memory 2048 >"$ANDROID_SMOKE_OUT/boot.log" 2>&1 &
emulator_pid=$!
trap 'adb emu kill >/dev/null 2>&1 || true; kill "$emulator_pid" 2>/dev/null || true; wait "$emulator_pid" 2>/dev/null || true' EXIT
set +e
smoke_status=0
if [[ "${ANDROID_SMOKE_BOOT_ONLY:-0}" != 1 ]]; then
    python3 /src/tools/android_smoke.py
    smoke_status=$?
fi
printf '%s\n' "$smoke_status" >"$ANDROID_SMOKE_OUT/status.txt"
if [[ "${ANDROID_SMOKE_KEEPALIVE:-0}" == 1 ]]; then
    while [[ ! -f /tmp/mm2-smoke-finish ]]; do sleep 2; done
fi
exit "$smoke_status"
