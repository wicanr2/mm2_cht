# MM2.EXE 的 overlay 機制與執行時記憶體佈局

輸入檔：`MM2.EXE`，SHA-256 `631facb658a39e0d438c451f8a43c9f6e2aeb774fc3843c1a9bac1e14bf8c4d4`，77,824 bytes。
位址標記法：**IDA linear**（IDA 以 base 段 0x1000 載入 root，linear = 0x10000 + 執行時偏移）
與 **file offset**（`MM2.EXE` 內的位元組位置）並列，兩者關係為 `file = linear - 0xF800`。

## 1. 執行時記憶體佈局

`MM2.EXE` 的 MZ header 宣告的 image 只有 32,272 bytes，但實體檔案 77,824 bytes。
差額 43,504 bytes 不會被 DOS 的 EXE loader 載入，由程式自行讀取到 BSS 區。
所有 code 與資料共用**同一個段基準**，用 20 位元偏移定址：

```
偏移        內容                                     來源
0x00000  ┌─────────────────────────────────┐
         │ root：程式碼                     │  MM2.EXE 檔案 0x0800 起
0x06BF0  ├─────────────────────────────────┤
         │ root：overlay 描述表 + 檔名表    │  ← §2、§3
         │        + thunk 表                │  ← §4
0x077D0  ├─────────────────────────────────┤
         │ root：overlay manager            │  ← §5
0x07E10  ├─────────────────────────────────┤  MZ image 結束
         │ level-1 overlay（三選一）        │  1MENU2 / 2COMBAT / 2PLAY
0x0C130  ├─────────────────────────────────┤
         │ level-2 overlay（十一選一）      │  1MENU1 / 1RETINN / 2MISC / …
0x0D850  ├─────────────────────────────────┤
         │ 資料區 43,504 bytes              │  MM2.EXE 檔案 0x8610 起
0x18240  ├─────────────────────────────────┤
         │ 其餘 BSS                         │
0x18A20  └─────────────────────────────────┘  = image 0x7E10 + e_minalloc 0x10C1 段
```

**證據**：

- level-2 的載入段 0x0C13 = level-1 載入段 0x07E1 + 最大 level-1 overlay（2PLAY，0x432 段）的長度。
- 資料區的載入段 0x0D85 = 最大 level-2 overlay（2SMITH）的結束段。
- 資料區開頭 0x10、0x14 兩處是 far pointer `0D85:478A`、`0D85:481E`，
  段值 0x0D85 指向資料區自身，偏移 0x478A / 0x481E 落在資料區長度 0xA9F0 內。
- 佈局總長 0x18A20 與 MZ header 的 `e_minalloc` 算出的記憶體需求一致。

等級：**已證實**（除資料區的實際讀取路徑，見 §7）。

## 2. overlay 描述表

位於 IDA linear `0x16BF6`（file `0x73F6`），14 筆，每筆 16 bytes，順序與檔名表一致。
overlay 編號 1–14；編號 0 保留給 root，不進表。

| 偏移 | 型別 | 意義 |
|---|---|---|
| +0x00 | word | bit15 = 已載入旗標；低 13 位 = 上層 overlay 編號（執行時填寫，靜態全 0） |
| +0x02 | word | 重定位項數。除 1MENU1 之外全為 0 |
| +0x04 | word | **載入段** |
| +0x06 | word | 結束段（= 載入段 + 大小） |
| +0x08 | word | 檔名在描述表所在段內的偏移 |
| +0x0A | dword | 0 |
| +0x0E | word | **大小（段數）** |

表前面的 `0x16BF0`（file `0x73F0`）三個 word 屬於 root：`077D` = root 的 CS、
`8120`、`18A2`（= 總長 0x18A2 段）。

實際值：

| # | 檔名 | 載入段 | 結束段 | 大小(段) | ×16 | 實際檔案 |
|---|---|---|---|---|---|---|
| 1 | 1MENU2.OVL | 07E1 | 09A8 | 01C7 | 7,280 | 7,280 |
| 2 | 2COMBAT.OVL | 07E1 | 0BC6 | 03E5 | 15,952 | 15,952 |
| 3 | 2PLAY.OVL | 07E1 | 0C13 | 0432 | 17,184 | 17,184 |
| 4 | 1MENU1.OVL | 0C13 | 0CED | 00DA | 3,488 | 3,504 |
| 5 | 1RETINN.OVL | 0C13 | 0CC1 | 00AE | 2,784 | 2,784 |
| 6 | 2MISC.OVL | 0C13 | 0D01 | 00EE | 3,808 | 3,808 |
| 7 | 2MISC2.OVL | 0C13 | 0D0B | 00F8 | 3,968 | 3,968 |
| 8 | 2CAST1.OVL | 0C13 | 0D29 | 0116 | 4,448 | 4,448 |
| 9 | 2CAST2.OVL | 0C13 | 0D38 | 0125 | 4,688 | 4,688 |
| 10 | 2CMDS.OVL | 0C13 | 0D28 | 0115 | 4,432 | 4,432 |
| 11 | 2CAVES.OVL | 0C13 | 0D6A | 0157 | 5,488 | 5,488 |
| 12 | 2BRAIN.OVL | 0C13 | 0D52 | 013F | 5,104 | 5,104 |
| 13 | 2SMITH.OVL | 0C13 | 0D85 | 0172 | 5,920 | 5,920 |
| 14 | 2TEMPLE.OVL | 0C13 | 0D02 | 00EF | 3,824 | 3,824 |

十三筆的大小欄位 ×16 與實體檔案完全相符。**1MENU1 少 16 bytes**，且它是唯一
重定位項數不為 0（值 2）的 overlay —— 尾端那 16 bytes 推測是重定位資料而非程式碼。
等級：**假設待驗**。

## 2.5 位址換算：反組譯裡的位址是哪一段

追函式時第一件事是判斷那個位址屬於 root 還是某個 overlay，否則會在
根映像的 dump 裡白找 —— overlay 佔的位址在根映像裡是未初始化區。

```
IDA linear = 檔案偏移 + 0xF800          （root 的 MZ header 是 0x800，載入段 0x1000）
root 本體           0x10000 – 0x12800
level-1 overlay 區  0x17E10 –           = 0x10000 + 0x07E1×16
level-2 overlay 區  0x1C130 –           = 0x10000 + 0x0C13×16
```

換算的依據：檔名表在 IDA linear `0x16CE8`、檔案 `0x74E8`，差 `0xF800`。

判斷一個位址屬於誰：

| 位址範圍 | 屬於 | 怎麼看 |
|---|---|---|
| `0x10000`–`0x12800` | root（`MM2.EXE`） | 直接在 `MM2.EXE.asm` 裡找 |
| `0x17E10`+ | level-1（1MENU2 / 2COMBAT / 2PLAY） | 減 `0x17E10` 得 overlay 內偏移，比對哪個 overlay 的長度容得下 |
| `0x1C130`+ | level-2（其餘十一個） | 同上，減 `0x1C130` |

例：`sub_1A606`（事件腳本直譯器）減 `0x17E10` 得 `0x27F6` = 10,230，
1MENU2 只有 7,280 bytes 裝不下，所以在 2COMBAT 或 2PLAY —— 事件腳本
自然是 2PLAY。重建映像 `2PLAY.img` 的長度 49,456 = `0xC130`，
與 level-2 載入段對得上，映像內位址加 `0x10000` 就是反組譯裡的位址。

## 2.6 DGROUP 在 BSS，查表撈不到

`ds:` 相對的位址落在 IDA 的 `seg019`，起點 linear `0x1D850`。
驗算：`ds:393h`（隊伍的 X 座標）→ `0x1DBE3`，正好落在 `byte_1DBE2` 旁。

```
DGROUP linear = 0x1D850
檔案偏移      = ds_offset + 0x1D850 - 0xF800
```

**但這段撈不出資料。** `seg019` 整段宣告成 `db N dup(?)` —— 是 BSS，
檔案裡沒有內容。照上式去讀 opcode 長度表（`ds:15E6`）得到的是一整片 0，
讀設施換算表（`ds:164C` 等五張）也一樣。

意思是那些表**執行時才填**，由初始化程式碼從別處複製或計算出來。
要拿到內容得追填表的程式碼，不能直接從檔案偏移讀。

順帶記一個踩過的坑：把 `ds` 基底當成和 `cs` 一樣（`0x10000`）去撈，
讀出來的是程式碼不是資料 —— 那個位置是 root 的程式碼段。
MZ 檔頭的 `SS=0x1822`（linear `0x28220`）也不是 DGROUP 起點，
它落在映像結尾（`0x22800`）之後的堆疊區。

## 2.7 執行時的記憶體佈局

追 `sub_160ED`（讀檔並複製到 DGROUP）與它的呼叫點，拼出這幾塊：

| DGROUP 偏移 | 內容 | 大小 |
|---|---|---|
| `0x5986` | 當前地圖的屬性（`ATTRIB.DAT` 的一筆） | 64 |
| `0x59C6` | 方向遮罩等暫存變數 | 16 |
| `0x59D6` | 地圖的**牆與事件層** | 256 |
| `0x5AD6` | 地圖的另一層 | 256 |
| `0x6052` | 事件腳本緩衝 | 0x8FC |

`0x5986 + 64 = 0x59C6` —— `ATTRIB.DAT` 那一筆的結尾正好接上牆判定用的
方向遮罩，三塊是連續的。

### 哪一層是牆

載入的呼叫順序是先 `0x5AD6` 後 `0x59D6`，看起來像「檔案前半 → `0x59D6`」。
照那樣推，牆會落在 `MAP.DAT` 段的**前** 256 bytes。

**不是。** 決定性的是事件旗標：五座城的事件格 100% 都設了
段內**後** 256 bytes 的 bit 7（42/42），前 256 bytes 只有 10/42。
而 `sub_15E68` 判牆用的 `ds:59C8h` 是從 `[si+59D6h]` 算出來的，
同一個值又被 `cmp ds:59C8h, 80h` 測試 —— 那正是事件旗標。

所以 **`0x59D6` 對應段內的後 256 bytes**，載入時的 `dx`（0 / 0x100）
不是「讀前半或後半」的意思。

兩層不是同一份資料：牆位元（`& 0x55`）只有 62.8% 相同，
整個位元組只有 31.8% 相同。

## 3. 檔名表

緊接描述表之後，IDA linear `0x16CE8`（file `0x74E8`），NUL 結尾字串連續排列，
第一筆是 `MM2.EXE`。描述表的 +0x08 欄位存的是相對本段起點（0x16BF0）的偏移。

## 3.5 thunk 的格式與怎麼用它追函式

每個 thunk 12 bytes：

```
9a 44 03 7d 07     call far 077D:0344    ; overlay loader，CS 是 MZ 檔頭的初始 CS
NN                 目標的 overlay 編號（0 = root）
80                 標記
ea LL LL SS SS     jmp far               ; LLLL = 目標在該 overlay 內的偏移
```

全檔掃 `9a 44 03 7d 07` 得到 **217 個 thunk**，其中 149 個指向 root。

**這是追跨 overlay 呼叫的鑰匙。** overlay 的程式碼不直接 call root，
一律走 thunk，所以在 overlay 的反組譯裡 grep root 的函式名永遠找不到。
要反過來做：先在 thunk 表裡找出「指向某個 root 偏移」的那一個，
再拿它的位址回 overlay 裡 grep。

例：隨機數產生器在 root linear `0x11C88`，root 內偏移 `0x1C88`。
掃出指向它的 thunk 在 `0x16F76` —— 而 `sub_16F76` 正是 `2COMBAT.OVL`
裡被呼叫最多的函式（58 次）。戰鬥的每一次擲骰都經過那裡。

## 4. overlay thunk 表

IDA linear `0x16D8A`（file `0x758A`）起，**219 筆，每筆 12 bytes**，
一路排到 file `0x7FCE`。呼叫 overlay 內的函式一律經過這裡：

```
9A <off> <seg>    call far  overlay manager     5 bytes
db  <overlay 編號>                              1
db  0x80                                        1   ← 全表固定
db  0xEA                                        1   ← 全表固定，far jmp 的 opcode
dw  <目標偏移>                                  2   ← 執行時的段內偏移
dw  0x0000                                      2   ← 待填的段值
```

`0x80` 與 `0xEA` 在 219 筆裡沒有例外。後 5 bytes 排成 `EA <offset> <segment>`
的形狀，是一條待填段值的 far jmp —— overlay 載入完成後可以就地改寫成直接跳轉。
等級：**強推論**（形狀與 manager 讀 inline 參數的行為一致，尚未追到改寫的那條指令）。

manager 的兩個進入點各自對應一種呼叫語意：

| 目標 | thunk 數 | 行為 |
|---|---|---|
| `077D:02E4`（0x17AB4） | 2 | 直接走共同路徑 |
| `077D:0344`（0x17B14） | 217 | 先設 `byte_17DC1 = 0xFF`、清 `off_16BF2` 的 bit15，再跳進共同路徑 |

`0x80` 這個位元組會與 overlay 編號一起被讀成一個 word（見 §5），
即 manager 收到的是 `0x8000 | 編號`。

### 目標偏移的座標系

thunk 的目標偏移是**執行時的段內偏移**，等於 `載入段×16 + overlay 內偏移`。
把 219 筆全部換算回各 overlay 的內部偏移，68 個 overlay 進入點**全部落在
各自 overlay 的長度範圍內，零越界** —— 這同時驗證了描述表的載入段解讀。

| overlay | 進入點數 | overlay | 進入點數 |
|---|---|---|---|
| root（編號 0，非 overlay 呼叫） | 151 | 2MISC2 | 5 |
| 1MENU2 | 1 | 2CAST1 | 2 |
| 2COMBAT | 7 | 2CAST2 | 1 |
| 2PLAY | 16 | 2CMDS | 5 |
| 1MENU1 | 2 | 2CAVES | 13 |
| 1RETINN | 5 | 2BRAIN | 3 |
| 2MISC | 4 | 2SMITH | 2 |
| | | 2TEMPLE | 2 |

## 5. overlay manager

`sub_17AB4`（seg003:02E4）保存全部暫存器後，從堆疊取出回傳位址，
讀取回傳位址處的一個 word 當作 inline 參數，再把回傳位址 +2 跳過它：

```asm
mov     ax, [bp+16h]        ; 回傳位址的 segment
mov     ds, ax
mov     si, [bp+14h]        ; 回傳位址的 offset
add     word ptr [bp+14h], 2 ; 跳過 inline 參數
mov     cx, [si]            ; CX = 0x8000 | overlay 編號
```

`sub_17A57` 負責載入，CX = overlay 編號：

```asm
mov     ax, 10h
dec     cx
mul     cx
mov     di, ax                          ; DI = (編號-1) × 16，描述表索引
test    word ptr es:[di+6], 8000h       ; 已載入？
...
mov     cx, es:[di+6]
and     cx, 1FFFh                       ; 低 13 位 = 上層 overlay，往上遞迴載入
```

`sub_17981` 做實際的檔案讀取，用到描述表的 +0x08（重定位項數）、+0x0A（載入段）、
+0x0C（結束段）、+0x0E（檔名偏移）、+0x14（大小）—— 這些偏移是相對 `[di+6]` 的
基準記的，換算回筆首即 §2 的欄位表。

版權字串 `Copyright (C) 1984, 1985, 1986 by Phoenix Software Associates Ltd.`
指向 Phoenix 的 overlay linker（PLINK86 系列）。呼叫慣例因此推定為 C
（右至左壓棧、呼叫者清棧），與實際觀察到的 `add sp, N` 收尾一致。等級：**強推論**。

## 6. 反組譯方法

把單一 `.OVL` 丟給 IDA 會得到錯誤結果，原因是 overlay 與 root 共用段基準，
overlay 內的 near call 會直接跨進 root——例如 1MENU2 的進入點第一件事就是
`call near ptr 0F052h`，目標在 overlay 自身長度之外。單獨載入時這個位移算出來是垃圾。

正確做法是重建執行時佈局（`tools/build_ovl_image.py`）：

```
0x0000        root image（MM2.EXE 去掉 2,048 bytes 的 MZ header）
load_seg×16   目標 overlay
（level-2 另外填入推定的 parent level-1）
```

段基準統一用 IDA base 0x1000，於是 IDA 位址 = 0x10000 + 執行時偏移，
thunk 表的目標偏移可以直接當進入點餵給 IDA。

三個必要步驟，缺一個就分析不出東西：

1. `set_segm_addressing(seg, 0)` 把段設成 16-bit。IDA 的 binary loader 預設 64-bit，
   不設會把 `55` 讀成 `push rbp`。
2. `del_items(ea, DELIT_EXPAND, 16)`。binary loader 把整段鋪成 8-byte 資料項，
   不是 8 的倍數的進入點會讓 `create_insn` 回 0。
3. `add_entry(ea, ea, name, 1)` 而非 `add_func` —— 後者在未分析區域回 0。

成效（`workplace/ida/out/*.asm`）：

| overlay | 指令 | 函式 | overlay | 指令 | 函式 |
|---|---|---|---|---|---|
| 1MENU2 | 2,615 | 25 | 2CAST1 | 1,415 | 49 |
| 2COMBAT | 4,997 | 117 | 2CAST2 | 1,697 | 64 |
| 2PLAY | 5,295 | 128 | 2CMDS | 1,500 | 37 |
| 1MENU1 | 28 | 2 | 2CAVES | 1,875 | 37 |
| 1RETINN | 1,124 | 12 | 2BRAIN | 1,860 | 24 |
| 2MISC | 1,426 | 32 | 2SMITH | 2,142 | 32 |
| 2MISC2 | 1,501 | 19 | 2TEMPLE | 1,347 | 21 |

1MENU1 只解出 28 條指令，與它的 3,488 bytes 不成比例；它同時是唯一帶重定位項的
overlay，兩件事可能同源。列為待解。

level-2 的 parent 依檔名前綴推定（`1xxx` → 1MENU2，`2xxx` → 2PLAY），
因為描述表的 parent 欄位是執行時才填的。推錯只會讓 parent 那一段解析成雜訊，
不影響 overlay 自身。等級：**假設待驗**。

## 7. 待解

1. **資料區（file `0x8610` 起 43,504 bytes）的讀取路徑。** 佈局位置已由 far pointer
   佐證，但「誰在什麼時候把它讀進 0x0D850」還沒追到。root 的 LSEEK 呼叫是起點。
2. **1MENU1 的 16 bytes 與重定位項數 2。**
3. **thunk 的自我改寫。** `0xEA` 預留的 far jmp 是否真的被回填。

## 8. 可重跑指令

```bash
python3 tools/build_ovl_image.py workplace/orig/MM2 workplace/ida/img
cp workplace/ida/img/*.img workplace/ida/ && cp workplace/ida/img/idc/*.idc workplace/ida/idc/
tools/ida.sh ovl 2PLAY.img 1000 img_2PLAY.idc      # 產出 workplace/ida/out/2PLAY.asm
```
