#!/usr/bin/env bash
# Existing API23 image; one bounded owner, no persistent AVD or sentinel.
set -euo pipefail
[[ -f /.dockerenv && -d /out && -f /src/tools/record_android_promo.py ]]
export HOME=/tmp/mm2-promo-home ANDROID_AVD_HOME=/tmp/mm2-promo-avd
export ANDROID_USER_HOME="$HOME/.android" ANDROID_EMULATOR_HOME="$HOME/.android"
mkdir -p "$ANDROID_USER_HOME" "$ANDROID_AVD_HOME" /out/android
stat -c '%u:%g %n' /out /out/android
printf 'no\n' | avdmanager create avd --force -n mm2-promo -k 'system-images;android-23;default;x86' --device pixel >/out/android/avd.log 2>&1
python3 - <<'PY'
from pathlib import Path
import os
p=Path(os.environ['ANDROID_AVD_HOME'])/'mm2-promo.avd/config.ini'
values={'hw.lcd.width':'780','hw.lcd.height':'1688','hw.lcd.density':'320','hw.ramSize':'2048','hw.gpu.enabled':'yes','hw.gpu.mode':'swiftshader_indirect','disk.dataPartition.size':'1536M','showDeviceFrame':'no','skin.name':'780x1688','skin.dynamic':'yes'}
lines=[s for s in p.read_text().splitlines() if s.split('=')[0].strip() not in values]
p.write_text('\n'.join(lines+[k+'='+v for k,v in values.items()])+'\n')
PY
emulator -avd mm2-promo -no-window -no-audio -no-boot-anim -no-snapshot -accel off -gpu swiftshader_indirect -feature -Vulkan -cores 2 -memory 2048 >/out/android/boot.log 2>&1 &
emulator_pid=$!
trap 'adb emu kill >/dev/null 2>&1 || true; kill "$emulator_pid" 2>/dev/null || true; wait "$emulator_pid" 2>/dev/null || true' EXIT
exec_status=0
python3 /tmp/record_android_promo.py || exec_status=$?
exit "$exec_status"
