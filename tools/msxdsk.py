#!/usr/bin/env python3
"""從 MSX 版 MM2 的 `.dsk` 抽檔。

    tools/msxdsk.py workplace/msx/*.dsk workplace/msx/out

磁片**沒有可用的檔案系統**：BPB 那組參數是合法的（1440 磁區 × 512
＝ 737,280），但 FAT 與根目錄整片是零。遊戲繞過檔案系統直接讀原始磁區。

## 怎麼找到檔案表的

不是靠猜，是照著開機碼走。MSX 的 BIOS 把磁區 0 載到 `0xC000` 並從
`0xC01E` 起跳，那段 Z80 做的事是：

	ld hl,8000h : ld de,1 : ld b,1 : call 0FFA7h   ; DSKIO 讀磁區 1 到 0x8000
	ld hl,(8004h) : ld bc,1FFh : add hl,bc : srl h ; 磁區數 = (長度+511)/512
	ld de,(8002h)                                   ; 起始磁區
	ld hl,4000h : call 0FFA7h                       ; 載到 0x4000
	jp 4000h

也就是說**磁區 1 是一張表**，每筆 8 bytes：

	uint16  id        0xF000 起算；0xFFF0／0xFFF1 是開機用的兩塊
	uint16  起始磁區
	uint16  長度（bytes）
	uint16  0

開機碼讀的第一筆就是表上的第一筆（id `0xFFF1`、磁區 9、長度 51），
這是判斷「表解對了」的自我驗證條件。

反組譯的做法：

	tools/ida.sh z80 msx_boot C00     # 段值 = 位址/16，C00 → 0xC000

## 引擎與圖形

開機 stub（`id 0xFFF1`、51 bytes、載到 0x4000）把 **`id 0xFFF0`
（27,609 bytes）載到 0x6800** —— 那是常駐引擎。

VDP 的存取**不走 BIOS 也不用固定埠號**：埠放在暫存器 C，用 `out (c),a`
（`ED 79`，全檔 64 處）。所以掃 `ld c,98h` 或 `call 005Ch` 都會零命中，
**那個零是「掃錯樣式」不是「沒有」**。

  - 調色盤：`0xD2F9`，見 `palette()`
  - 圖形送 VRAM：`0xC4BC` 起，用 VDP 暫存器 17 指到 32 再 `otir` 送 15 bytes
    的命令區塊（SX/SY/DX/DY/NX/NY/CLR/ARG/CMD），命令是 `0xF0` ＝ HMMC
  - 像素解碼：`0xC51A`，見 `unrle()`

**還沒解**：每張圖在磁片上的起點與寬高。`NX`／`NY` 來自 `0xC542` 指到的
RAM 緩衝（`0x9694`），是執行時填的，要再往上追誰填它。
"""
import glob
import os
import struct
import sys


def unrle(d: bytes, pos: int, n: int):
    """MSX 版的 RLE。逐行重寫自常駐區塊 `0xC51A` 那支位元組產生器。

        c = 控制位元組
        count = c & 0x7F
        c & 0x80  →  接 count 個字面值
        否則      →  接一個位元組，重複 count 次

    原版把它寫成「每呼叫一次吐一個位元組」的產生器（用 `exx` 把計數器
    藏在替身暫存器組裡），因為資料是一個位元組一個位元組餵給 VDP 的
    HMMC 命令（`ld a,0F0h` 那一段），不先解到緩衝區。
    """
    out = bytearray()
    while len(out) < n and pos < len(d):
        c = d[pos]
        pos += 1
        cnt = c & 0x7F or 0x80
        if c & 0x80:
            out += d[pos:pos + cnt]
            pos += cnt
        else:
            out += bytes([d[pos]]) * cnt
            pos += 1
    return bytes(out), pos


def palette(resident: bytes):
    """從常駐區塊（`id 0xFFF0`，載入位址 0x6800）取 16 色調色盤。

    寫入那一段在 `0xD25F`：`ld a,90h`（VDP 暫存器 16 ＝ 調色盤指標）→
    `inc c`（埠 0x99 → 0x9A）→ `ld hl,0D2F9h : ld b,20h : otir`。
    MSX2 的格式是 `0RRR0BBB` / `00000GGG`，每個分量 3 bit。
    """
    raw = resident[0xD2F9 - 0x6800:][:32]
    return [(((raw[2 * i] >> 4) & 7) * 36, (raw[2 * i + 1] & 7) * 36,
             (raw[2 * i] & 7) * 36) for i in range(16)]


def entries(d: bytes):
    """回傳 [(id, 起始磁區, 長度)]。表在磁區 1（檔案偏移 0x200）。"""
    out = []
    for i in range(0x200, 0x400, 8):
        idv, sec, ln, _ = struct.unpack_from("<HHHH", d, i)
        if ln == 0 or sec * 512 + ln > len(d):
            continue
        # 兩片各有自己的表，磁區編號各自從頭算。
        out.append((idv, sec, ln))
    return out


def main() -> None:
    if len(sys.argv) < 3:
        raise SystemExit(__doc__)
    outdir = sys.argv[-1]
    os.makedirs(outdir, exist_ok=True)
    for pat in sys.argv[1:-1]:
        for path in sorted(glob.glob(pat)):
            d = open(path, "rb").read()
            ents = entries(d)
            tag = "d2" if "Disk 2" in path else "d1"
            if "[a]" in path:
                continue          # 兩片各有一個 [a] 版本，內容重複
            print(f"{os.path.basename(path)[:56]}: {len(ents)} 筆")
            for idv, sec, ln in ents:
                blob = d[sec * 512:sec * 512 + ln]
                name = f"{tag}_{idv:04x}_{ln}.bin"
                with open(os.path.join(outdir, name), "wb") as f:
                    f.write(blob)
            print(f"   → {outdir}")


if __name__ == "__main__":
    main()
