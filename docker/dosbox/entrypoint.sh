#!/usr/bin/env bash
# 在容器內實際執行的腳本：起 Xvfb、產生 dosbox.conf、跑 DOSBox、
# 依 timeline 送鍵 / 截圖、收尾清乾淨。由 tools/dosbox_run.sh 從 host 呼叫，
# 一般不需要直接手動執行。
#
# 用法：
#   entrypoint.sh <cga|ega> <timeline> [cycles]
#
# timeline 格式：用 ';' 分隔的步驟，依序執行：
#   wait:N        等待 N 秒
#   key:KEYSYM    送一個按鍵（xdotool keysym，如 Return / space / Escape / Up / Down）
#   type:STRING   打一段文字（xdotool type，不含 Enter）
#   shot:NAME     截圖存成 /shots/NAME.png
#
# 範例：
#   "wait:2;shot:01-title;wait:3;key:Return;wait:1;shot:02-menu"
#
# 遊戲資料要求由呼叫端 bind mount 到 /game（可寫，存檔測試需要）。
# 截圖輸出目錄由呼叫端 bind mount 到 /shots。

set -uo pipefail

MODE="${1:?用法: entrypoint.sh <cga|ega> <timeline> [cycles]}"
TIMELINE="${2:-}"
CYCLES="${3:-fixed 4000}"

if [[ "$MODE" != "cga" && "$MODE" != "ega" && "$MODE" != "mcga" && "$MODE" != "hercules" ]]; then
    echo "MODE 必須是 cga 或 ega，收到：$MODE" >&2
    exit 2
fi

# MM2 靠命令列參數選顯示模式（E/T/M/C/H），沒帶參數會走偵測並可能回報
# 「Not enough memory for 16 color version」。machine 與參數要一起對上。
case "$MODE" in
    ega)      MM2ARG=E; MACHINE=ega ;;
    cga)      MM2ARG=C; MACHINE=cga ;;
    mcga)     MM2ARG=M; MACHINE=vgaonly ;;
    hercules) MM2ARG=H; MACHINE=hercules ;;
esac

export DISPLAY=:99
mkdir -p /shots/dosbox-captures

echo "[entrypoint] 啟動 Xvfb ..."
Xvfb :99 -screen 0 1024x768x24 -nolisten tcp &
XVFB_PID=$!
sleep 1

# 產生 dosbox conf。聲音全關（headless 沒有音效裝置，開了只會洗 ALSA 錯誤訊息，
# 不影響畫面，但關掉比較乾淨）。machine 依 MODE 切換 cga / ega。
CONF=/tmp/dosbox-${MODE}.conf
cat > "$CONF" << EOF
[sdl]
fullscreen=false
fulldouble=false
output=surface
autolock=false
waitonerror=false
priority=normal,normal

[dosbox]
language=
machine=${MACHINE}
captures=/shots/dosbox-captures
memsize=64

[render]
frameskip=0
aspect=false
scaler=none

[cpu]
core=auto
cputype=auto
cycles=${CYCLES}
cycleup=10
cycledown=20

[mixer]
nosound=true
rate=44100
blocksize=1024
prebuffer=20

[midi]
mpu401=intelligent
mididevice=none
midiconfig=

[sblaster]
sbtype=none
oplmode=none

[gus]
gus=false

[speaker]
pcspeaker=false
tandy=off
disney=false

[joystick]
joysticktype=none

[serial]
serial1=dummy
serial2=dummy
serial3=disabled
serial4=disabled

[dos]
xms=false
ems=false
umb=false
keyboardlayout=us

[ipx]
ipx=false

[autoexec]
mount c /game
c:
rem MM2 在可用常規記憶體「太多」時會誤判成不足（632 KB free 也照報
rem 「Not enough memory for 16 color version」）。LOADFIX 先吃掉 64 KB，
rem 把程式推到較高位址就正常了。DOSBox wiki 對這個遊戲的唯一指示就是這條。
loadfix -64
mm2 ${MM2ARG}
EOF

echo "[entrypoint] 啟動 DOSBox（machine=${MACHINE}, cycles=${CYCLES}）..."
dosbox -conf "$CONF" -userconf > /tmp/dosbox.log 2>&1 &
DOSBOX_PID=$!

# 等 DOSBox 視窗出現（最多等 15 秒）
WIN=""
for i in $(seq 1 30); do
    WIN=$(xdotool search --name DOSBox 2>/dev/null | head -1)
    [[ -n "$WIN" ]] && break
    sleep 0.5
done

if [[ -z "$WIN" ]]; then
    echo "[entrypoint] 錯誤：15 秒內沒等到 DOSBox 視窗，DOSBox 可能啟動失敗" >&2
    cat /tmp/dosbox.log >&2
    kill "$DOSBOX_PID" "$XVFB_PID" 2>/dev/null
    exit 1
fi
echo "[entrypoint] DOSBox 視窗 id=$WIN"

# 關鍵：沒有 window manager，xdotool windowactivate 會失敗（_NET_ACTIVE_WINDOW 不支援）。
# 必須用 windowfocus（直接 XSetInputFocus，不依賴 WM）＋全域 xdotool key（XTest），
# 不能用 xdotool key --window <id>（那是 XSendEvent，SDL 預設不理成分事件，按鍵送了等於沒送）。
xdotool windowfocus "$WIN"

run_timeline() {
    local timeline="$1"
    IFS=';' read -ra STEPS <<< "$timeline"
    for step in "${STEPS[@]}"; do
        [[ -z "$step" ]] && continue
        local kind="${step%%:*}"
        local arg="${step#*:}"
        case "$kind" in
            wait)
                echo "[entrypoint] wait ${arg}s"
                sleep "$arg"
                ;;
            key)
                echo "[entrypoint] key $arg"
                xdotool windowfocus "$WIN"
                xdotool key "$arg"
                ;;
            type)
                echo "[entrypoint] type $arg"
                xdotool windowfocus "$WIN"
                xdotool type --delay 80 "$arg"
                ;;
            shot)
                echo "[entrypoint] shot $arg"
                import -window root "/shots/${arg}.png"
                ;;
            dump)
                # 把 DOSBox 行程裡最大的一塊匿名記憶體抓出來 —— DOS 的模擬記憶體
                # 就在那裡面。遊戲有一堆表只存在於執行時的 DGROUP（BSS），
                # 檔案裡讀不到，只能這樣拿。
                echo "[entrypoint] dump $arg"
                python3 - "$DOSBOX_PID" "/shots/${arg}.bin" <<'PYDUMP'
import sys, re
pid, out = sys.argv[1], sys.argv[2]
regions = []
for line in open(f"/proc/{pid}/maps"):
    m = re.match(r"([0-9a-f]+)-([0-9a-f]+) (\S+) \S+ \S+ \S+\s*(.*)", line)
    if not m:
        continue
    lo, hi, perm, path = int(m.group(1), 16), int(m.group(2), 16), m.group(3), m.group(4).strip()
    if path or "rw" not in perm:
        continue
    size = hi - lo
    if size < 1 << 20:
        continue
    regions.append((lo, hi, size))
if not regions:
    print("[dump] 找不到合適的匿名記憶體區", file=sys.stderr)
    sys.exit(1)
# DOSBox 的 DOS 記憶體不見得是最大的那塊，全部倒出來讓分析端自己找。
total = 0
with open(f"/proc/{pid}/mem", "rb") as f, open(out, "wb") as o:
    for lo, hi, size in regions:
        try:
            f.seek(lo)
            data = f.read(size)
        except OSError:
            continue
        o.write(data)
        total += len(data)
        print(f"[dump] {lo:#x}+{size} ({size >> 20} MB)")
print(f"[dump] 合計 {total} bytes 寫到 {out}")
PYDUMP
                ;;
            *)
                echo "[entrypoint] 未知 timeline 步驟：$step" >&2
                ;;
        esac
    done
}

if [[ -n "$TIMELINE" ]]; then
    run_timeline "$TIMELINE"
else
    # 沒給 timeline：預設等 5 秒讓開場畫面穩定，截一張圖，方便單純「跑起來看看」。
    sleep 5
    import -window root "/shots/${MODE}-default.png"
fi

echo "[entrypoint] timeline 跑完，收尾 ..."
kill "$DOSBOX_PID" 2>/dev/null
sleep 1
kill -9 "$DOSBOX_PID" 2>/dev/null
kill "$XVFB_PID" 2>/dev/null

echo "[entrypoint] dosbox.log 最後 20 行："
tail -20 /tmp/dosbox.log

echo "[entrypoint] 完成"
