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
"""
import glob
import os
import struct
import sys


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
