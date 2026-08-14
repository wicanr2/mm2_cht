# 探索模式的指令按鍵表 oracle

日期：2026-08-15

輸入：`MM2.EXE`（SHA-256 `631facb658a39e0d438c451f8a43c9f6e2aeb774fc3843c1a9bac1e14bf8c4d4`）、
`2PLAY.OVL`（SHA-256 `7078f30f87f9f25f8c296dc9207d70957c785b0ed83ad763e20e4012a82d2202`）。
位址是 IDA composite linear（`workplace/ida/2PLAY.img.i64`、`workplace/ida/MM2.EXE.i64`）。

## 1. 主迴圈在哪

`2PLAY _2play_e00` 的主體。`0x1804F` 讀鍵，`0x18082` 過一層轉換
（`loc_16F22`），結果留在 `[bp+var_C]`，`0x180A0` 起是一串 `cmp ax, <ASCII>`
的比較鏈 —— **不是跳表**，所以在反組譯裡 grep 不到「指令表」這種東西。

比較鏈的順序是二分過的：先與 `'O'`（`0x4F`）比，大於往 `0x181F4`
（`R`/`P`/`Q`/`U`/`S` 與數字），小於等於就逐一比 `0x11`/`B`/`C`/`D`/`E`/`M`。

## 2. 按鍵 → 處理函式（已證實）

| 鍵 | 分派位址 | 呼叫 | 目標 | 是什麼 |
|---|---|---|---|---|
| `Ctrl-Q`（`0x11`）| `0x180E0` | 內嵌 | 印 `ds:14F1` 後問 `Y/N` | 離開遊戲 |
| `B` | `0x18166` | thunk `0x1722E` | `2MISC +C130`（`_2misc_e00`）| 撞門 Bash |
| `C` | `0x1816C` | thunk `0x17066` | `2MISC2 +C3F6` | 施法 Cast |
| `D` | `0x18176` | thunk `0x171FE` | `2MISC2 +C130` | 解僱 Dismiss |
| `E` | `0x1817C` | thunk `0x1723A` | `2MISC2 +C2F8` | 交換隊員 Exchange |
| `M` | `0x18186` | 近呼叫 | `_2play_e14` | 俯視地圖 |
| `O` | `0x18190` | thunk `0x17252` | root `0x1471E` | 指令視窗（要 `ds:03CE == 1`）|
| `P` | `0x181B0` | thunk `0x1725E` | root `0x147D8` | 防護視窗（要 `ds:03CE == 0`）|
| `Q` | `0x181C8` | thunk `0x1716E` | root `0x15B8A` | 快速檢視 Quick Ref |
| `R` | `0x181D4` | thunk `0x17216` | `2MISC +CF84`（`_2misc_e03`）| 休息 Rest |
| `S` | `0x181E8` | thunk `0x17336` | root `0x13814` | **搜尋 Search** |
| `U` | `0x181EE` | thunk `0x17222` | `2MISC +C242`（`_2misc_e01`）| 開鎖 Unlock |
| `1`…`N` | `0x18205` | thunk `0x1716E` | root `0x15B8A` | 檢視第 N 位（`N = ds:0426`）|

`O` 與 `P` 共用畫面右上那一格，`ds:03CE` 是兩者的切換旗標 —— 珍017 說明書
（上冊 p.38）寫的「上面所列的指令可切換防護效用視窗與指令視窗」就是這一條。

`P` 打到的 root `0x147D8` 與 [`polish-spec`](../polish-spec.md) P4 的
`sub_147D8` 是同一支，互相印證。

## 3. Search（root `0x13814`）

```asm
sub_11392(0)                      ; 開訊息視窗
sub_1410A(ds:4D9B)
sub_11676(4, 11h)                 ; 游標定位
sub_11726(ds:4D9C)                ; 印 "Search..."
si = 0
迴圈: ds:6950[si] != 0 → 跳出（[bp-2] = si）
      si++; si < 3 繼續，否則 [bp-2] = 3
if [bp-2] == 3 且 (ds:695C | ds:695E) != 0: [bp-2]--      ; 金幣
if [bp-2] == 3 且  ds:695A          != 0: [bp-2]--        ; 寶石
if [bp-2] == 3:                                           ; 五組都空
    印 ds:4DA6 = "Nothing Here!"; ds:039A = 1
else:
    call thunk 0x17672 → 2MISC `_2misc_e02`               ; 進寶箱／領獎流程
```

字串取自 DGROUP 初值段（**EXE 檔內偏移 = DGROUP 偏移 + 0x8630**）：
`ds:4D9C` = `Search...`、`ds:4DA6` = `Nothing Here!`。

**`_2misc_e02` 的 thunk `0x17672` 全域只有這一個呼叫端。** 掃法是對每個映像
逐位元組解 `E8`／`E9` 的 `rel16` 與 `9A` 的遠呼叫 —— 文字 grep 在這裡是零命中，
因為 IDA 把它印成 `call near ptr byte_17672`。

所以**原版所有的獎賞（戰鬥戰利品、事件腳本擺好的待領獎賞、神殿給的物品）
都只能經由搜尋領取**，沒有第二條路。珍017 說明書上冊 p.39 寫得一致：
「戰鬥後不要忘了用 `S` 找尋戰利品」。

## 4. `ds:0434` 的完整狀態機（本輪閉合）

[`chest-trigger-oracle`](chest-trigger-oracle.md) 先前把 `_2play_e00`
`0x18032` 那條「飽和遞增」列為語意未定。它其實是自動搜尋的觸發：

```asm
0x18032  cmp byte ds:0434, 0FEh
0x18037  jnz 0x18040
0x18039  inc byte ds:0434        ; 0xFE → 0xFF
0x1803D  call thunk 0x17336      ; 立刻跑一次 Search
```

於是三個狀態各自閉合：

| 值 | 誰寫的 | 搜尋時的行為 |
|---|---|---|
| `0` | 戰鬥勝利（`2COMBAT sub_19BF8`）、領完獎（`2MISC _2misc_e02`）| `_2misc_e02` 走四選一寶箱頁 |
| `0xFF` | 事件腳本 `0x2a`／`sub_19B44`、`2MISC sub_1C538` | `_2misc_e02` 直接印 `Treasure!` 發放 |
| `0xFE` | 神殿 `2TEMPLE sub_1C1EA` | 主迴圈下一輪自動 `inc` 成 `0xFF` 並自動搜尋，玩家不必按鍵 |

**四選一寶箱頁的觸發點因此是：`ds:0434 == 0`（戰鬥剛結束）且五組獎賞陣列至少
一組非空時，玩家按 `S`。** 沒有「一般寶箱產生器」，箱子的內容就是戰鬥戰利品。

## 5. 實機證據

```bash
tools/dosbox_run.sh ega "wait:3;key:Return;wait:2;key:s;wait:4;key:g;wait:5;\
key:z;wait:4;shot:30-fpv;key:m;wait:3;shot:31-map;key:Escape;wait:2;\
key:s;wait:3;shot:32-search"
```

- `31-map.png`：按 `M` 的俯視地圖。藍底格、白線牆、綠色小塊的門、隊伍位置一個
  白色方向箭頭，上緣座標 `(7,3)`、下緣 `('ESC' to go back)`。這把
  [`04-graphics`](../formats/04-graphics.md) 的 `B` 圖磚用途從強推論升級為已證實。
- `32-search.png`：剛開局什麼都沒撿的狀態下按 `S`，訊息列印
  `Search...` 與 `Nothing Here!` **同一列**、分別在第 4 欄與第 0x10 欄 ——
  與 §3 的兩次 `sub_11676(欄, 0x11)` 逐項相符。

## 6. 可重跑指令

```bash
tools/ida.sh idapy ida_dump.py 2PLAY.img.i64 cmd 17FC0+280   # 指令分派
tools/ida.sh idapy ida_dump.py MM2.EXE.i64   search 13814+94  # Search 本體
python3 tools/ovl_thunks.py 17066 1716E 171FE 17216 17222 1722E 1723A \
        17252 1725E 172B2 17336
```

找某個 thunk／函式的呼叫端要掃原始位元組，不要 grep `.asm`：

```python
# 對每個 workplace/ida/*.img：E8/E9 的 rel16 目標，與 9A 的遠呼叫偏移
if d[i] in (0xE8, 0xE9) and (i + 3 + struct.unpack_from('<h', d, i+1)[0]) & 0xFFFF == tgt: ...
```

映像位移換算：`MZ header = 0x800`，composite image 由
[`tools/build_ovl_image.py`](../../tools/build_ovl_image.py) 組出，
**IDA 位址 = 0x10000 + 映像位移**。level-2 overlay 的映像裡還疊著 parent
（`2xxx` 是 2PLAY、`1xxx` 是 1MENU2），所以十個 `2*.img` 都會掃到同一份
2PLAY 的呼叫端 —— 那不是十個呼叫端，是同一個。
