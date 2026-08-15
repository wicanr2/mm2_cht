# 開機流程與顯示模式：`1MENU1`

輸入檔：`MM2.EXE`（SHA-256 見 [`inventory.md`](../inventory.md)）＋ `1MENU1.OVL`。
IDA 位址取自 `workplace/ida/out/1MENU1.asm`（image 由 `tools/build_ovl_image.py` 重建）。

`1MENU1` 是**開機第一支跑的 overlay**：解析命令列的顯示模式字母、檢查磁片與
記憶體、載入顯示驅動，然後把控制權交給名冊選單。

## 1. 進入點

root 的 `main`（`sub_10002`，IDA `0x10002`）取 `argv[1]` 的第一個字元
（沒有參數就用空白），然後依序呼叫四個 thunk：

```asm
call    near ptr byte_16CDE+0ACh   ; thunk 0x16D8A → _1menu1_e00(字元)
call    near ptr byte_16D96        ; thunk        → _1menu1_e01
call    near ptr byte_16DA2        ; thunk        → _1menu2_e00   名冊／建角色
call    near ptr byte_16DAE        ; thunk        → _2play_e00    進遊戲
```

等級：**已證實**。

## 2. 命令列的顯示模式

`_1menu1_e00(字元)` 先轉大寫再分派，寫進 `ds:496A`：

| 字母 | `ds:496A` | 驅動 |
|---|---|---|
| `C` | 0 | `CGA.DRV` |
| `E` | 1 | `EGA.DRV` |
| `T` | 2 | `TGA.DRV`（Tandy 1000 16 色）|
| `H` | 3 | `HGA.DRV`（Hercules 單色）|
| `M` | 4 | `MCGA.DRV`（MCGA／VGA）|
| 空白 | 不寫（保持初值 `0xFFFF`）| 由偵測決定 |
| 其他 | — | 印用法訊息後結束 |

編號對得上檔名表：`ds:49BA` 起是五個 8-byte 的檔名槽，順序就是
`CGA`／`EGA`／`TGA`／`HGA`／`MCGA`，後面 `ds:49E4` 是五個指向它們的指標
（`49BA 49C2 49CA 49D2 49DA`）。`ds:496A` 的初值 `0xFFFF` ＝ 尚未選定。

參數不合法時 `sub_1C130` 印出這段再 `exit(1)`（字串取自 DGROUP 初值段，
拼字錯誤是原版就有的）：

```
Might and Magic Book Two - Copyright (C) 1989 New World Computing, Inc.
IBM Version by Inside Out Software, Inc.
Valid arguements:
  E - EGA graphics
  T - Tandy 1000 16 color graphics
  M - MCGA/VGA graphics
  C - CGA graphics
  H - Hercules mono graphics
NOTE: MCGA/VGA requires 448K
```

等級：**已證實**（分派碼、檔名表與訊息三者互相印證）。

## 3. 顯示卡偵測

root `sub_11BA6` 用 `int 10h` `AX=1A00`（Display Combination Code）取得代碼，
經 `ds:4A04` 的 16 項轉換表換成內部碼，再由 `sub_11C20` 設旗標：

| DCC | 轉換後 | 旗標 | 意義 |
|---|---|---|---|
| 0–3、6、9、D–F | 0 | `ds:4972` | 無／MDA／CGA／PGA |
| 4、5 | 1 | `ds:4976` | EGA |
| 7、8 | 4 | `ds:4978` | VGA |
| A–C | 5 | `ds:497A` | MCGA |

`int 10h` 回不出 DCC 時退回 `AH=12h BL=10h`（取 EGA 資訊），再不行就直接
戳 `3D4h` 判斷有沒有 6845。

`_1menu1_e00` 載入驅動之後有一條規則：

```asm
cmp     word_14978, 0       ; 偵測到 VGA？
jz      short loc_1C69E
cmp     word_1496A, 4       ; 目前是 MCGA？
jnz     short loc_1C69E
cmp     [bp+arg_0], 4Dh     ; 玩家明打了 'M'？
jz      short loc_1C69E
mov     word_1496A, 1       ; 否則退回 EGA
```

也就是**VGA 機器預設走 EGA，除非命令列明寫 `M`**。

等級：偵測表與旗標**已證實**；「沒明打 `M` 時 `ds:496A` 怎麼變成 4」的那條
自動選擇路徑尚未定位，所以這條規則的觸發條件標**強推論**。

## 4. 開機檢查

| 函式 | 做什麼 |
|---|---|
| `sub_1C432` | 掃 A:／B: 找資料磁片，找不到印 `Insert <遊戲名> in a floppy drive` |
| `sub_1C376` | 把 `ds:04C2` 起的檔名指標表複製到 `ds:0520` 起的工作副本 |
| `sub_1C322` | `ds:496A` 為 0（CGA）或 3（Hercules）時，先填 `ds:033E`–`ds:0346` ＝ `2,2,1,1,0,3,0,3,3` 再呼叫；兩色系統的色彩對應 |
| `sub_1C56C` | 記憶體檢查：`ds:0346` ＝ 3 且 `ds:495C` < 98，或 `ds:0346` ＝ 15 且 `ds:495C` < 200 時，印 `MM2: Not enough memory for the 4/16 color version. Release TSRs and retry.` 並 `exit(1)` |

顯示驅動本身由 root `sub_11E64(檔名, …)` 載入，回傳的 far 指標存在
`ds:4A24`／`ds:4A26`；root `0x11D5B` 就是「呼叫驅動的 function 0」。
同一支載入器也負責 `TIMER.DRV`（`ds:4B54`）。

等級：**已證實**（每項都有字串或表格佐證），`sub_1C322` 那九個位元組的
逐格語意**未解**。

## 5. 這個 overlay 為什麼晚了半年才解出來

`1MENU1` 是十四個 overlay 裡唯一**重定位項數不為 0** 的一個，檔案最前面多
一段 16 bytes 的重定位表，程式碼從 `0x10` 才開始。`tools/build_ovl_image.py`
原本整份貼進 image，於是兩個進入點都落在指令中間 —— IDA 認出 28 條指令就
停了，`.asm` 看起來像「這個 overlay 幾乎沒有程式碼」。

修掉之後同一份輸入解出 27 個函式、1,405 條指令。格式細節見
[`01-overlay-and-memory-layout.md`](../formats/01-overlay-and-memory-layout.md) §2。

**規則：反組譯結果「異常地少」時先驗進入點落在不落在指令邊界**，
不要當成「這個模組本來就小」。判準很硬 —— C 編出來的函式開頭是
`55 8B EC`（`push bp; mov bp,sp`），對不上就是起點錯了。
