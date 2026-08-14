#!/usr/bin/env bash
# IDA Pro 9.4 headless 包裝（docker）
#
#   tools/ida.sh analyze MM2.EXE            產 workplace/ida/MM2.EXE.i64 + .asm
#   tools/ida.sh m68k    mm2                Amiga 的裸 68000 映像
#   tools/ida.sh z80     msx_boot C00       MSX 的裸 Z80 映像（段值 = 位址/16）
#   tools/ida.sh script  ida_xref.idc MM2.EXE.i64 word_1234
#   tools/ida.sh raw     idat -A -B MM2.EXE 任意 idat 命令
#
# 注意事項（詳見 ~/.claude/knowledge-base/retro/ida-pro-9.4.md）：
#   - 16-bit real mode 沒有 Hex-Rays，只能讀組語。
#   - 寫 IDC 不要寫 IDAPython；IDC 一定要 #include <idc.idc>，否則安靜 exit 1。
#   - headless 的 print 不進 stdout，腳本一律寫檔到 /work/out/。
#   - IDC 崩掉會弄壞 .i64；症狀是「Failed to initialize IDA as library」，
#     拿另一個 .i64 跑已知可用的腳本就能分辨是工具壞了還是這一份輸入壞了。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$ROOT/workplace/ida"
IMAGE="${IDA_IMAGE:-ida-pro-9.4-ver2}"

mkdir -p "$WORK/out"

# IDC 崩掉的那一次會留下未打包的資料庫散檔（.id0/.id1/.nam/.til）。
# 之後任何針對同一個目標的指令都會 exit 1 且零輸出，症狀看起來像 image 壞掉。
# 每次開跑前先清掉散檔，把「這一份輸入壞了」這個變數消掉。
clean_stale() {
  local target="$1"
  [ -n "$target" ] || return 0
  rm -f "$WORK/$target".id0 "$WORK/$target".id1 "$WORK/$target".id2 \
        "$WORK/$target".nam "$WORK/$target".til
}

run() {
  docker run --rm \
    --log-opt max-size=10m --log-opt max-file=3 \
    -v "$WORK:/work" \
    -v "$ROOT/tools:/work/tools:ro" \
    -w /work \
    "$IMAGE" "$@"
}

cmd="${1:-}"
shift || true

case "$cmd" in
  analyze)
    # $1 = workplace/ida/ 底下的目標檔名
    target="${1:?用法: tools/ida.sh analyze <檔名>}"
    [ -f "$WORK/$target" ] || { echo "找不到 $WORK/$target（先把原版檔複製進 workplace/ida/）" >&2; exit 1; }
    clean_stale "$target"
    run idat -A -B "$target"
    ;;
  z80)
    # $1 = workplace/ida/ 底下的裸 Z80 映像, $2 = 載入位址(hex，預設 C000)
    #
    # MSX 的開機磁區由 BIOS 載到 0xC000 再從 0xC01E 起跳，所以基底要給對，
    # 否則所有 `call`／`jp` 的目標都會落在別的地方，而反出來的東西
    # **看起來仍然像合理的程式碼** —— 這種錯不會有任何症狀。
    # **`-b` 的單位是 16 bytes（段）不是位元組**：載入位址 0xC000 要給 C00。
    # 給錯會載到 0xC0000，而反出來的東西照樣像合理的程式碼 —— 沒有症狀。
    target="${1:?用法: tools/ida.sh z80 <檔名> [載入位址hex，單位 16 bytes]}"; shift
    base="${1:-C00}"
    [ -f "$WORK/$target" ] || { echo "找不到 $WORK/$target" >&2; exit 1; }
    clean_stale "$target"
    rm -f "$WORK/$target.i64"
    run idat -A -B -Tbinary -pz80 "-b$base" "$target"
    ;;
  ovl)
    # $1 = .OVL 檔名, $2 = 載入段(hex, 已含 IDA base 0x1000)
    target="${1:?用法: tools/ida.sh ovl <x.OVL> <載入段hex> <腳本.idc>}"; shift
    seg="${1:?缺少載入段}"; shift
    idc="${1:?缺少 idc}"; shift
    clean_stale "$target"
    rm -f "$WORK/$target.i64"
    run idat -A -Tbinary -p8086 "-b$seg" "-S/work/idc/$idc" "$target"
    ;;
  m68k)
    # $1 = workplace/ida/ 底下的目標檔（裸 68000 映像，例如 Amiga 版的 mm2）
    #
    # Amiga 的 `mm2` **不是 hunk 檔**（開頭不是 0x000003F3），是裸機器碼 ——
    # 與 DOS 的 `.OVL` 同一類，所以載入方式也一樣：`-Tbinary` 加處理器型號。
    target="${1:?用法: tools/ida.sh m68k <檔名> [載入位址hex]}"; shift
    base="${1:-0}"
    [ -f "$WORK/$target" ] || { echo "找不到 $WORK/$target" >&2; exit 1; }
    clean_stale "$target"
    rm -f "$WORK/$target.i64"
    run idat -A -B -Tbinary -p68000 "-b$base" "$target"
    ;;
  idapy)
    # $1 = tools/ 底下的 .py, $2 = .i64, $3 = 輸出檔名（放 workplace/ida/out/），其餘為腳本參數
    #
    # [HARD] IDAPython **只在 `ida-pro-9.4-idapython:py312-v1` 上能跑**。
    # 基底 image（ver2／ver3）是**零輸出、零訊息**的靜默失敗，
    # 與「腳本寫錯」「路徑打錯」長得一模一樣，而且 exit code 不可信
    # （同一種失敗在不同 image 上分別回 0 與 1）。
    # 唯一可信的訊號是**輸出檔存在且非空**。
    py="${1:?用法: tools/ida.sh idapy <x.py> <target.i64> <輸出檔名> [args...]}"; shift
    target="${1:?缺少 .i64 目標}"; shift
    out="${1:?缺少輸出檔名}"; shift
    clean_stale "${target%.i64}"
    IMAGE="${IDA_PY_IMAGE:-ida-pro-9.4-idapython:py312-v1}" \
      run idat -A "-S/work/tools/$py /work/out/$out.json $*" "$target"
    [ -s "$WORK/out/$out.json" ] || { echo "輸出檔空的或不存在：out/$out.json（IDAPython 沒跑起來）" >&2; exit 1; }
    ;;
  script)
    # $1 = tools/ 底下的 .idc 檔名, $2 = .i64, 其餘為腳本參數
    idc="${1:?用法: tools/ida.sh script <x.idc> <target.i64> [args...]}"; shift
    target="${1:?缺少 .i64 目標}"; shift
    clean_stale "${target%.i64}"
    run idat -A "-S/work/tools/$idc $*" "$target"
    ;;
  raw)
    run "$@"
    ;;
  *)
    sed -n '2,12p' "${BASH_SOURCE[0]}"
    exit 1
    ;;
esac
