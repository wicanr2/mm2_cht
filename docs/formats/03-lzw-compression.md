# LZW 壓縮與 STR.DAT 的位移層

MM2 的資料檔幾乎全部經過同一套 LZW 壓縮。演算法讀自 `MM2.EXE` 的 `sub_12242`
（IDA linear `0x12242`），不是從檔頭位元組猜出來的。

實作：[`tools/mm2lzw.py`](../../tools/mm2lzw.py)。

## 1. 段格式

```
+0  uint16  解壓後長度
+2  uint16  0
+4  LZW 位元流
```

載入常式（`seg000:6052` 一帶）先 `LSEEK` 到段起點、讀 4 bytes、取**低 word**
當解壓後長度，據此 `int 21h AH=48h` 配置記憶體，再呼叫 `sub_12242`。

## 2. LZW 參數

| 項目 | 值 | 出處 |
|---|---|---|
| 初始碼寬 | 9 bits | `mov [bp+var_1C], 9` |
| 碼寬上限 | 12 bits | `cmp [bp+var_1C], 0Ch` |
| CLEAR | `0x100` | `cmp ax, 100h` → 重設碼寬/門檻/下一碼 |
| EOF | `0x101` | `cmp ax, 101h` → 釋放記憶體、關檔、返回 |
| 第一個動態碼 | `0x102` | `mov [bp+var_16], 102h` |
| 初始門檻 | `0x200` | `mov [bp+var_1A], 200h` |
| 位元順序 | LSB first | `lodsw` / `lodsb` 後 `shr al,1` + `rcr bx,1` 位移 dx 次，再套遮罩 |
| 字典項 | 3 bytes | `bx = code*3`（`shl bx,1` + `add bx,ax`），`[bx]` = prefix word，`[bx+2]` = suffix byte |
| 字典容量 | 4096 | 配置 `0x300` 段 = 12,288 bytes = 4096 × 3 |

碼寬增長發生在**新增字典項之後**：下一碼達到門檻且碼寬未滿 12 時，碼寬 +1、門檻左移一位。
`KwKwK`（碼 >= 下一個可用碼）走 `cmp ax, [bp+var_16]; jl` 那條分支，補上前一次的首字元。

輸入緩衝 1,024 bytes，位元位置超過 `0x3FD` 時把剩餘位元組搬到緩衝開頭再補讀
（`rep movsb` + `int 21h AH=3Fh`）—— 這是原版的串流細節，一次解整段時不需要複製。

## 3. 驗證

段頭宣告的長度與解出的長度逐段比對，全部相符：

| 檔案 | 段數 | 宣告 = 解出 | 內容佐證 |
|---|---|---|---|
| `EVENTSI.DAT` | 71（索引 uint32×71） | 1454 / 1469 / 1465 … | `Middlegate Inn`、`Gateway Temple`、`Thundrax Weaponry` |
| `EVENTSO.DAT` | 71（前 4 筆空） | ✓ | — |
| `MAP.DAT` | 60（索引 uint16×60） | 全部 512 | 512 = 16×16 格 × 2 bytes |
| `MONSTERS.DAT` | 1 | 6,656 | — |
| `ATTRIB.DAT` | 1 | 3,840 | — |
| `STR.DAT` | 1 | 7,707 | 見 §4 |
| `SPELLS.DAT` | 1 | 256 | — |

`EVENTSI.DAT` 前三段解出的地點名依序是 Middlegate、Atlantium、Tundara，
與 `MM2.EXE` 尾部的城鎮列表同序 —— 段索引即城鎮編號。等級：**強推論**。

## 4. STR.DAT 的位移層

LZW 解開之後再把每個位元組 **−4**，得到 NUL 分隔的單字表：

```
$ID YOU HEAR ABOUT THE ORC WHO THOUGHT·KITES WERE MADE FROM FLYPAPER …
```

位移量由多個英文單字同時對上決定（`XLSYKLX`→`THOUGHT`、`JP]TETIV`→`FLYPAPER`、
`[IVI`→`WERE`），不是單點猜測。解出後可見字串覆蓋 54%，共 729 段。

`$ID` 這類記號與出現在單字之間的 `0xFD`（−4 前是 `0x01`）不是普通分隔符，
語意未定。等級：**假設待驗**。

這張表是文字壓縮用的單字字典：對話與訊息以索引引用它，而不是逐字存放。
中文化時要一併處理索引層，不能只換單字表。

## 5. 另一條解密路徑：XOR

`sub_12616` 是另一種資料保護，與 §4 的位移不同：

```asm
si = word_223DF          ; 目前的金鑰位置，初值取自 word_1DD64
loop cx 次:
    lodsb                ; al = 金鑰位元組
    xor al, es:[di]
    stosb
    cmp byte ptr [si], 0 ; 金鑰走到 NUL 就繞回開頭
    jnz  next
    si = word_1DD64
```

也就是**用一段 NUL 結尾的金鑰字串反覆 XOR**，且金鑰位置跨呼叫延續
（`word_223DF` 在函式結束時被寫回）。

`sub_12242` 只在傳入的 `di != 0` 時對讀入的每個緩衝呼叫它。事件檔的載入點傳
`xor di, di`，所以 `EVENTSI/O.DAT` 不走這條路 —— 這與純 LZW 就能解開的實測一致。
哪些檔案走 XOR 路徑、金鑰字串的實際內容，尚未確認。

## 6. 可重跑指令

```bash
python3 tools/mm2lzw.py workplace/orig/MM2/EVENTSI.DAT 0x11C | xxd | head
python3 -c "
import sys; sys.path.insert(0,'tools')
from mm2lzw import unpack_segment
d=open('workplace/orig/MM2/STR.DAT','rb').read()
size,out=unpack_segment(d,0)
print(bytes((b-4)&0xFF for b in out)[:120])
"
```
