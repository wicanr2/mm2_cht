#!/usr/bin/env python3
"""把 Mega Drive ROM 的所有選曲立即值改寫成指定的一首，做出「開機就播這首」的暫存 ROM。

為什麼要這樣做而不是用模擬器除錯器：BlastEm 的除錯器在管道與 pty 下都不回應，
兩種都試過。而選曲的結構已經解得夠清楚，靜態改寫反而更確定 ——
不依賴按鍵時序、不依賴玩到哪個場景，開機就是目標曲目。

改的是三種位置，全部只動 4 bytes 的立即值，不動任何曲目資料或驅動：

  1. 18 處 `move.l #<曲目位址>,(sp)` ＋ `move.l (sp)+,-$37B0(a5)`
     —— 遊戲不論要求哪一個場景的音樂，拿到的都是目標曲目。
  2. 不動 vblank 閒置路徑（`0x06CB0E` 的 `move.l #<空曲>,d0`）。
     **實測改了會讓音樂完全不出聲**：驅動閒置時那條路徑每一幀都會重觸發，
     原本重送的是只有聲部設定與休止的空曲（重送無害），換成真曲子就變成
     每幀從頭開始，錄到的 VGM 只有 711 bytes。差異測試：未改寫 126,817 bytes、
     只改這一處 711 bytes、其餘條件相同。
  3. 不動任何 `cmp` 用的空曲常數；它們只是「現在是不是靜音」的判斷，
     改了反而會讓淡出邏輯誤判。

遊戲自己會在片頭要求音樂，而 18 處都改過，所以不論它要求哪一個場景，
拿到的都是目標曲目。

產出的 ROM 是暫存驗證素材，與原版一樣不得入版控或散布。

    md_patch_song.py <原始ROM> <輸出ROM> <曲目位址hex>
"""

import re
import sys

# move.l (sp)+,-$37B0(a5) —— 寫進「要播哪一首」的那條指令。
# 18 個寫入端全部是這個形式；前面 6 bytes 一定是 move.l #imm,(sp)（2E BC ＋ 4 bytes）。
SET_REQUESTED = bytes([0x2B, 0x5F, 0xC8, 0x50])
PUSH_IMM = bytes([0x2E, 0xBC])

# vblank 閒置路徑的位址，只拿來做完整性檢查，**不改它**（理由見檔頭）。
IDLE_MOVE = 0x06CB0E


def patch(rom: bytearray, song: int) -> list[int]:
    """把所有選曲立即值改成 song，回傳被改的位移。"""
    patched = []
    target = song.to_bytes(4, "big")

    for m in re.finditer(re.escape(SET_REQUESTED), bytes(rom)):
        h = m.start()
        if rom[h - 6 : h - 4] != PUSH_IMM:
            # 樣式不合就跳過並回報。靜默略過會讓「改了幾處」這個數字失去意義。
            print(f"  跳過 0x{h:06X}：前面不是 move.l #imm,(sp)", file=sys.stderr)
            continue
        rom[h - 4 : h] = target
        patched.append(h - 4)

    # 完整性檢查：這條指令還在，代表 ROM 佈局與解出來的一致。它本身不改。
    if rom[IDLE_MOVE : IDLE_MOVE + 2] != bytes([0x20, 0x3C]):
        raise SystemExit(
            f"0x{IDLE_MOVE:06X} 不是 move.l #imm,d0，ROM 與預期不符，不改"
        )

    if len(patched) != 18:
        raise SystemExit(f"預期改寫 18 處選曲立即值，實際 {len(patched)} 處")

    return patched


def main() -> None:
    if len(sys.argv) != 4:
        raise SystemExit("用法: md_patch_song.py <原始ROM> <輸出ROM> <曲目位址hex>")

    src, dst, song_hex = sys.argv[1], sys.argv[2], sys.argv[3]
    song = int(song_hex, 16)

    with open(src, "rb") as fh:
        rom = bytearray(fh.read())

    # 曲目位址前 4 bytes 必須是合法的結束指標，這是曲目格式的自我描述條件。
    # 不驗的話打錯一個位元就會做出「開機播垃圾」而且看起來像模擬器壞了的 ROM。
    end = int.from_bytes(rom[song - 4 : song], "big")
    if not (song < end <= len(rom)):
        raise SystemExit(
            f"0x{song:06X} 不像曲目起點：前 4 bytes 讀出的結束指標是 0x{end:08X}"
        )
    size = end - song

    patched = patch(rom, song)

    with open(dst, "wb") as fh:
        fh.write(rom)

    print(f"曲目 0x{song:06X}..0x{end:06X}（{size} bytes），改寫 {len(patched)} 處立即值")


if __name__ == "__main__":
    main()
