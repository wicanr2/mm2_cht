#!/usr/bin/env python3
"""從 MSX 版 MM2 的 `.dsk` 抽檔。

    tools/msxdsk.py workplace/msx/*.dsk workplace/msx/out     抽出檔案表裡的檔
    tools/msxdsk.py --gfx workplace/gfx/msx workplace/msx/*.dsk  抽圖形

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
import collections
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


def image(blob: bytes):
    """解一個圖形檔。回傳 (寬, 高, 像素) 或 None。

    檔頭就是 VDP 要的東西：`uint16 NX; uint16 NY;` 接 RLE 的 4bpp 像素。
    載圖那一段（`0xC4B8`）把檔案串流到 VDP 的 HMMC 命令，前 4 bytes
    原地 `otir` 進暫存器 40–43，所以檔頭不是「格式」是「命令參數」。

    **不必掃描**：`entries()` 補上第二張表之後，圖形檔各自有 id
    （`0x20xx`／`0x21xx`／`0x30xx`），直接照 id 取。
    """
    if len(blob) < 8:
        return None
    nx, ny = struct.unpack_from("<HH", blob, 0)
    if not (8 <= nx <= 512 and 4 <= ny <= 512 and nx % 2 == 0):
        return None
    need = (nx // 2) * ny
    px, _ = unrle(blob, 4, need)
    if len(px) != need:
        return None
    return nx, ny, px


def images(d: bytes, lo=0x2000, hi=None):
    """掃出磁片上的圖形。回傳 [(偏移, 寬, 高, 像素)]。

    **`image()` 出來之後這支只剩考古價值**：當年還不知道圖形檔有自己的 id，
    只能整片掃。留著是因為它的三個驗收條件本身是可重用的判準。

    每張圖的檔頭就是 4 bytes：

        uint16  NX   寬（像素）
        uint16  NY   高
        接著是 RLE 的 4bpp 像素，解出來剛好 (NX/2) × NY bytes

    為什麼是「檔頭就是寬高」：繪圖那一段（`0xC4BC`）把 `0xC542` 指到的
    4 bytes 直接 `otir` 進 VDP 的暫存器 40–43（NX／NY），而 `0xC495`
    把那個指標設成 `0x9694` —— 圖形緩衝的開頭。所以圖檔載進去之後，
    前 4 bytes 原地就是 VDP 要的 NX／NY。

    驗收條件是**兩個獨立數字同時成立**：宣告的寬高算出來的長度，
    要與 RLE 實際解出來的長度完全相同；再加上壓縮率落在合理範圍
    （0.15–1.0）與「不是單一顏色佔九成」。這比「畫出來看起來像圖」強得多。
    """
    out = []
    hi = hi or len(d)
    off = lo
    while off < hi - 8:
        nx, ny = struct.unpack_from("<HH", d, off)
        if not (8 <= nx <= 256 and 8 <= ny <= 212 and nx % 2 == 0):
            off += 2
            continue
        need = (nx // 2) * ny
        if not 200 <= need <= 40000:
            off += 2
            continue
        px, end = unrle(d, off + 4, need)
        used = end - off - 4
        if len(px) != need or not need * 0.15 <= used <= need:
            off += 2
            continue
        top = max(collections.Counter(px).values()) / need
        if top > 0.9:
            off += 2
            continue
        # 真的美術，相鄰兩列會有相當比例的位元組相同；長度對得上但畫出來
        # 是雜訊的假命中在這一關會被擋掉（實測真圖 25–60%，雜訊 < 12%）。
        bpr = nx // 2
        n = min(4000, need - bpr)
        same = sum(1 for i in range(n) if px[i] == px[i + bpr])
        if n < 100 or same / n < 0.15:
            off += 2
            continue
        out.append((off, end, nx, ny, px))
        off += 2
    # 命中會重疊（同一張圖從不同起點都可能「解得出來」），
    # 取覆蓋範圍最大的優先、彼此不重疊。**命中後直接跳過會漏掉很多**：
    # 前面一個假命中會把後面真正的起點吃掉。
    out.sort(key=lambda r: r[1] - r[0], reverse=True)
    keep = []
    for r in out:
        if all(r[1] <= k[0] or r[0] >= k[1] for k in keep):
            keep.append(r)
    keep.sort()
    return [(a, nx, ny, px) for a, _, nx, ny, px in keep]


def to_png(nx: int, ny: int, px: bytes, pal, path: str, scale: int = 2):
    from PIL import Image

    im = Image.new("RGB", (nx, ny), (255, 0, 255))
    for y in range(ny):
        for x in range(nx):
            k = y * (nx // 2) + x // 2
            if k < len(px):
                im.putpixel((x, y), pal[px[k] >> 4 if x % 2 == 0 else px[k] & 15])
    im.resize((nx * scale, ny * scale), Image.NEAREST).save(path)


def entries(d: bytes):
    """回傳 [(id, 起始磁區, 長度)]。

    **表有兩張**，各 192 筆、各佔三個磁區：磁區 1–3 與磁區 4–6。
    查表那一段（`sub_C5F5`）先讀磁區 1 起的三個（`ld b,3 : ld de,1`）搜一遍，
    沒中再讀磁區 4 起的三個（`ld de,4`）；`sub_C698` 的迴圈計數是 `0C0h`
    ＝ 192，正好是三個磁區除以 8。

    只讀第一張表的話，圖形檔（id `0x20xx`／`0x21xx`）一個都看不到 ——
    而第一張表本身完全合法、解得出一批檔案，所以不會有任何症狀。

    兩片各有自己的表，磁區編號各自從頭算；查不到的 id 會換片再找。
    """
    out = []
    for base in (0x200, 0x800):
        for i in range(base, base + 192 * 8, 8):
            idv, sec, ln, _ = struct.unpack_from("<HHHH", d, i)
            if ln == 0 or sec * 512 + ln > len(d):
                continue
            out.append((idv, sec, ln))
    return out


def main() -> None:
    if len(sys.argv) < 3:
        raise SystemExit(__doc__)
    if sys.argv[1] == "--id":
        # tools/msxdsk.py --id 輸出目錄 磁片... —— 照檔案表逐檔抽圖
        outdir = sys.argv[2]
        os.makedirs(outdir, exist_ok=True)
        res = open("workplace/msx/out/d1_fff0_27609.bin", "rb").read()
        pal = palette(res)
        n = 0
        for pat in sys.argv[3:]:
            for path in sorted(glob.glob(pat)):
                if "[a]" in path:
                    continue
                d = open(path, "rb").read()
                tag = "d2" if "Disk 2" in path else "d1"
                for idv, sec, ln in entries(d):
                    got = image(d[sec * 512:sec * 512 + ln])
                    if got is None:
                        continue
                    nx, ny, px = got
                    to_png(nx, ny, px, pal,
                           os.path.join(outdir, f"{tag}_{idv:04X}_{nx}x{ny}.png"))
                    n += 1
        print(f"{n} 張")
        return
    if sys.argv[1] == "--gfx":
        outdir = sys.argv[2]
        os.makedirs(outdir, exist_ok=True)
        res = open("workplace/msx/out/d1_fff0_27609.bin", "rb").read()
        pal = palette(res)
        for pat in sys.argv[3:]:
            for path in sorted(glob.glob(pat)):
                if "[a]" in path:
                    continue
                d = open(path, "rb").read()
                tag = "d2" if "Disk 2" in path else "d1"
                ims = images(d)
                for off, nx, ny, px in ims:
                    to_png(nx, ny, px, pal,
                           os.path.join(outdir, f"{tag}_{off:06X}_{nx}x{ny}.png"))
                print(f"{os.path.basename(path)[:50]}: {len(ims)} 張")
        return
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
