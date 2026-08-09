#!/usr/bin/env bash
# 一行指令啟動 Might and Magic II 的 DOSBox 參考環境（headless，全程 docker）。
#
# 用法：
#   tools/dosbox_run.sh <cga|ega> [timeline] [cycles]
#
# 參數：
#   mode      必填，cga 或 ega（切換模擬顯示卡，遊戲會依此載入對應素材）
#   timeline  選填，';' 分隔的自動化步驟，見 docker/dosbox/entrypoint.sh 開頭註解：
#               wait:N / key:KEYSYM / type:STRING / shot:NAME
#             不給就只等 5 秒後截一張圖（存成 <mode>-default.png）
#   cycles    選填，DOSBox [cpu] cycles 設定，預設 "fixed 4000"
#
# 範例：
#   tools/dosbox_run.sh ega
#   tools/dosbox_run.sh ega "wait:2;shot:01-title;wait:3;key:Return;wait:1;shot:02-menu"
#   tools/dosbox_run.sh cga "wait:5;shot:01-cga-boot"
#
# 第一次跑、或改了 docker/dosbox/Dockerfile / entrypoint.sh 之後，會自動重 build image
# （用檔案的 mtime 跟 image 建立時間比對，不必每次手動 docker build）。
#
# 遊戲資料：第一次跑會自動從 workplace/orig/MM2/ 複製一份可寫副本到
# workplace/dosbox/game/（workplace/orig/ 唯讀，不可動）。之後重跑不會覆蓋，
# 如果要還原成乾淨初始狀態（例如存檔 diff 實驗做完要重來），手動刪除
# workplace/dosbox/game/ 再重跑即可。
#
# 截圖輸出：workplace/dosbox/shots/*.png（不入版控）。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

MODE="${1:-}"
TIMELINE="${2:-}"
CYCLES="${3:-fixed 4000}"

if [[ "$MODE" != "cga" && "$MODE" != "ega" ]]; then
    echo "用法: $0 <cga|ega> [timeline] [cycles]" >&2
    echo "例:   $0 ega \"wait:2;shot:01-title;wait:3;key:Return;shot:02-menu\"" >&2
    exit 1
fi

IMAGE=mm2-dosbox:latest
DOCKER_DIR="$REPO_ROOT/docker/dosbox"
GAME_DIR="$REPO_ROOT/workplace/dosbox/game"
SHOTS_DIR="$REPO_ROOT/workplace/dosbox/shots"
ORIG_DIR="$REPO_ROOT/workplace/orig/MM2"

# --- 確保 image 存在且是新的 ---
# 判斷方式：image 不存在，或 Dockerfile / entrypoint.sh 比 image 建立時間新，就重 build。
NEED_BUILD=0
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
    NEED_BUILD=1
else
    IMAGE_CREATED=$(docker image inspect "$IMAGE" --format '{{.Created}}' 2>/dev/null)
    IMAGE_EPOCH=$(date -d "$IMAGE_CREATED" +%s 2>/dev/null || echo 9999999999)
    SRC_EPOCH=$(stat -c %Y "$DOCKER_DIR/Dockerfile" "$DOCKER_DIR/entrypoint.sh" | sort -n | tail -1)
    if [[ "$SRC_EPOCH" -gt "$IMAGE_EPOCH" ]]; then
        NEED_BUILD=1
    fi
fi

if [[ "$NEED_BUILD" -eq 1 ]]; then
    echo "[dosbox_run] build image $IMAGE ..."
    docker build -t "$IMAGE" -f "$DOCKER_DIR/Dockerfile" "$DOCKER_DIR"
fi

# --- 確保可寫遊戲副本存在（workplace/orig/ 唯讀，不能直接掛） ---
if [[ ! -f "$GAME_DIR/MM2.EXE" ]]; then
    echo "[dosbox_run] 複製遊戲檔案到可寫副本 $GAME_DIR ..."
    mkdir -p "$GAME_DIR"
    cp -r "$ORIG_DIR"/. "$GAME_DIR"/
    chmod -R u+w "$GAME_DIR"
fi

mkdir -p "$SHOTS_DIR"

echo "[dosbox_run] mode=$MODE cycles=\"$CYCLES\""
[[ -n "$TIMELINE" ]] && echo "[dosbox_run] timeline=$TIMELINE"

docker run --rm \
    -v "$GAME_DIR:/game" \
    -v "$SHOTS_DIR:/shots" \
    "$IMAGE" \
    "$MODE" "$TIMELINE" "$CYCLES"
