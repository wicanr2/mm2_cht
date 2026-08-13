#!/usr/bin/env bash
# 在容器內執行：起 Xvfb、跑 BlastEm、依 timeline 送鍵與開關 VGM 記錄、收尾。
# 由 tools/blastem_run.sh 從 host 呼叫。
#
# 用法：
#   entrypoint.sh <rom檔名> <timeline>
#
# timeline 步驟以 ';' 分隔，依序執行：
#   wait:N        等 N 秒（可小數）
#   key:KEYSYM    送一個按鍵（xdotool keysym）
#   hold:KEYSYM:N 按住 N 秒再放開（選單要按久一點時用）
#   rec:NAME      開始記錄 VGM，收尾時存成 /out/NAME.vgm
#   stop          停止記錄
#   shot:NAME     截圖存成 /out/NAME.png
#
# BlastEm 的按鍵預設對應（`/opt/blastem/default.cfg`，別憑印象）：
#   方向鍵 = 十字鍵、Return = Start、**a/s/d = A/B/C**、q/w/e = X/Y/Z、f = Mode
#   m = VGM 記錄開關、p = 截圖、esc = 選單
# ⚠ `z` 不是 A 鈕，是 `ui.sms_pause`（模擬器層的暫停）。按下去畫面照樣在跑選單，
#   看起來像「按了沒反應」，實際上是按到了完全不同的東西。
#
# [HARD] 關掉 BlastEm 之前一定要先停止記錄。直接關會讓 VGM 的 EOF offset
# 沒寫進去，而且 vgm_ptch 修不回來 —— 那份錄音就廢了。本腳本在結束路徑上
# 一律補一次停止，即使 timeline 忘了寫。

set -uo pipefail

ROM_NAME="${1:?用法: entrypoint.sh <rom檔名> <timeline>}"
TIMELINE="${2:-}"

# 音樂包播放時統一重採樣為 48 kHz 雙聲道，這裡直接錄成那個取樣率省一次重採樣。
WAV_RATE="${WAV_RATE:-48000}"
WAV_LOOPS="${WAV_LOOPS:-1}"

export DISPLAY=:99
# 容器裡沒有音效裝置。VGM 記錄取的是晶片模擬的暫存器寫入，不是輸出裝置的取樣，
# 所以 dummy 驅動不影響記錄內容；不設的話 SDL 會在初始化時直接失敗。
export SDL_AUDIODRIVER="${SDL_AUDIODRIVER:-dummy}"
# Xvfb 沒有硬體 GL，強制走軟體 rasterizer。
export LIBGL_ALWAYS_SOFTWARE=1

BUILD_ID="$(cat /opt/blastem/BUILD_ID 2>/dev/null || echo unknown)"
echo "[blastem] 版本 $BUILD_ID"

if [ ! -f "/rom/$ROM_NAME" ]; then
    echo "[blastem] 找不到 /rom/$ROM_NAME" >&2
    exit 2
fi

# BlastEm 對過長或含非 ASCII 的 ROM 檔名會建檔失敗（vgmrips wiki 已知問題），
# 而本專案的 ROM 檔名又長又有空白。複製成固定的短名再餵給它。
WORKROM=/work/rom.md
cp "/rom/$ROM_NAME" "$WORKROM"
echo "[blastem] ROM SHA-256: $(sha256sum "$WORKROM" | cut -d' ' -f1)"

# [HARD] 不要寫自己的 blastem.cfg。使用者設定檔是**整份取代**內建的 default.cfg，
# 不是合併 —— 只寫 vgm_path 兩行會把整個 bindings 區塊一起蓋掉，於是 `m`
# （`m ui.vgm_log`）就沒有綁定了。症狀是模擬器正常跑、截圖正常、就是不產生 VGM，
# 看起來像「這個版本沒有 VGM 功能」。
#
# 內建預設本來就是 `vgm_path $HOME`，所以把 HOME 指到可寫目錄就夠了。
VGMDIR=/work/home
mkdir -p "$VGMDIR"
export HOME="$VGMDIR"

# 存檔跨執行保留：SRAM（`save.sram`）與模擬器狀態檔（`*.state`）都在同一個目錄。
#
# 這片開場 45 秒才進主選單，之後還要選隊伍才進得了城 —— 而**按鍵腳本靠計時推進，
# 在 GDB stub 底下會漂移**（模擬器停在中斷點時時間是凍結的）。所以導航只做一次，
# 用 `key:grave` 存成狀態檔，後續每一次追蹤改用 `key:l` 直接載回來。
#
# BlastEm 的路徑是 $HOME/.local/share/blastem/<ROM 檔名去副檔名>/。
STATE_DIR="$VGMDIR/.local/share/blastem/rom"
mkdir -p "$STATE_DIR"
if [ -d /out/blastem-state ]; then
    cp /out/blastem-state/* "$STATE_DIR/" 2>/dev/null
    echo "[blastem] 沿用 /out/blastem-state：$(ls /out/blastem-state | tr '\n' ' ')"
fi

echo "[blastem] 啟動 Xvfb ..."
Xvfb :99 -screen 0 1024x768x24 -nolisten tcp &
XVFB_PID=$!
sleep 1

echo "[blastem] 啟動 BlastEm ..."
/opt/blastem/blastem "$WORKROM" &
BLASTEM_PID=$!
sleep 3

if ! kill -0 "$BLASTEM_PID" 2>/dev/null; then
    echo "[blastem] BlastEm 啟動後隨即結束" >&2
    kill "$XVFB_PID" 2>/dev/null
    exit 3
fi

# [HARD] 送鍵一律用 XTEST（不加 `--window`），而且要按住一小段時間。
#
# 兩個獨立的坑，症狀都是「按了沒反應」而畫面一切正常：
#
# 1. `xdotool key --window <id>` 走的是 XSendEvent 合成事件，BlastEm 的**手把輸入**
#    收不到（UI 熱鍵如 `m` 反而收得到 —— 所以「VGM 錄得到」不能拿來證明按鍵有效）。
#    不加 `--window` 才會走 XTEST，直接進 X server 的輸入佇列。
# 2. `xdotool key` 的按下與放開幾乎同時發生，遊戲每幀只 poll 一次手把就整個漏掉。
#    要 keydown → sleep ≥0.15s → keyup。
#
# 判準：送 `Down` 看選單游標有沒有移動。移動了才代表輸入這條路是通的。
KEY_HOLD="${KEY_HOLD:-0.15}"

send_key() {
    xdotool keydown --clearmodifiers "$1"
    sleep "$KEY_HOLD"
    xdotool keyup --clearmodifiers "$1"
}

RECORDING=0
REC_NAME=""

start_rec() {
    if [ "$RECORDING" = "1" ]; then
        echo "[blastem] 已經在記錄，忽略 rec:$1"
        return
    fi
    REC_NAME="$1"
    rm -f "$VGMDIR"/*.vgm
    send_key m
    RECORDING=1
    echo "[blastem] 開始記錄 → $REC_NAME"
}

stop_rec() {
    [ "$RECORDING" = "1" ] || return 0
    send_key m
    RECORDING=0
    sleep 1
    local produced
    produced="$(ls -1t "$VGMDIR"/*.vgm 2>/dev/null | head -1)"
    if [ -n "$produced" ]; then
        mv "$produced" "/out/${REC_NAME}.vgm"
        echo "[blastem] 停止記錄 → /out/${REC_NAME}.vgm ($(stat -c%s "/out/${REC_NAME}.vgm") bytes)"
        # 同一條工具鏈裡一路轉到音樂包可以直接吃的格式，中間不換機器、不換版本。
        if vgm2wav --samplerate "$WAV_RATE" --loops "$WAV_LOOPS" \
                "/out/${REC_NAME}.vgm" "/work/${REC_NAME}.raw.wav" >/dev/null 2>&1; then
            vgm2pcmwav "/work/${REC_NAME}.raw.wav" "/out/${REC_NAME}.wav"
            rm -f "/work/${REC_NAME}.raw.wav"
        else
            echo "[blastem] vgm2wav 轉檔失敗：${REC_NAME}" >&2
        fi
    else
        # 這是最重要的失敗訊息：按了 m 卻沒有檔案，代表這個版本沒有 VGM 功能，
        # 或熱鍵沒送進視窗。兩者都不能靜默通過。
        echo "[blastem] 沒有產生 VGM —— 熱鍵沒生效或版本不支援" >&2
    fi
    REC_NAME=""
}

cleanup() {
    stop_rec
    # BlastEm 收到 SIGTERM 會把 SRAM 寫回去（log 會出現 "Saved SRAM to ..."），
    # 所以要等它結束再複製，不能 kill 完馬上拿。
    kill "$BLASTEM_PID" 2>/dev/null
    wait "$BLASTEM_PID" 2>/dev/null
    if [ -n "$(ls -A "$STATE_DIR" 2>/dev/null)" ]; then
        mkdir -p /out/blastem-state
        cp "$STATE_DIR"/* /out/blastem-state/ 2>/dev/null
        echo "[blastem] 存檔寫回 /out/blastem-state：$(ls -s /out/blastem-state | tr '\n' ' ')"
    fi
    kill "$XVFB_PID" 2>/dev/null
}
trap cleanup EXIT

IFS=';' read -ra STEPS <<< "$TIMELINE"
for step in "${STEPS[@]}"; do
    [ -n "$step" ] || continue
    action="${step%%:*}"
    arg="${step#*:}"
    case "$action" in
        wait) sleep "$arg" ;;
        key)  send_key "$arg" ;;
        hold)
            hk="${arg%%:*}"; hs="${arg#*:}"
            xdotool keydown --clearmodifiers "$hk"; sleep "$hs"
            xdotool keyup --clearmodifiers "$hk" ;;
        rec)  start_rec "$arg" ;;
        stop) stop_rec ;;
        shot) xwd -root -silent | convert xwd:- "/out/${arg}.png" ;;
        *)    echo "[blastem] 未知步驟：$step" >&2 ;;
    esac
done

echo "[blastem] timeline 執行完畢"
