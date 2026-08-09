# 事件腳本

事件不是「踩到就顯示某條字串」，而是**執行一段腳本**。

## 1. 觸發流程

```
踩到格子
  → 查事件表，取得 Index 與 Kind          （docs/formats/02 §4）
  → 從腳本區開頭跳過 Index 段（0xFF 分隔）
  → 執行該段的 opcode
```

跳段的程式碼在 `sub_1A606` 開頭：

```asm
mov al, [bp+arg_0]      ; arg_0 = 要跳過幾段
cbw
mov si, ax
loc_1A625:
call sub_18DC8          ; 讀腳本的下一個位元組
cmp al, 0FFh
jnz  short loc_1A625    ; 掃到 0xFF
dec  si
jnz  short loc_1A625    ; 重複 si 次
```

所以**事件記錄的 `Index` 是腳本段號**，不是字串序號。

## 2. 腳本直譯器

`sub_1A606` 的主迴圈：

```asm
call sub_18DC8          ; 讀 opcode
sub  ah, ah
sub  ax, 1              ; switch 50 cases
cmp  ax, 31h
jbe  short loc_1A673
...
add  ax, ax
xchg ax, bx
jmp  cs:jpt_1A676[bx]   ; 跳表
```

**50 個 opcode**，跳表在 `jpt_1A676`。

Middlegate（EVENTSI 段 0）有 44 個腳本段，實際用到的 opcode 分佈：

| opcode | 次數 | 狀態 |
|---|---|---|
| `0x04` | 16 | **已解**：`04 NN` = 顯示第 NN 條字串 |
| `0x0B` | 8 | 未解 |
| `0x0E` | 6 | 未解；參數都落在字串序號範圍內，可能是顯示字串的變體 |
| `0x02` | 5 | 未解 |
| `0x2B` | 4 | 未解 |
| `0x01` | 2 | 未解；出現在需要互動的格子（詢問／扣錢／給技能） |
| `0x15` | 1 | 未解 |

段的內容多半只有兩個位元組：

```
[ 1] 04 01     顯示字串 1（Middlegate Inn）
[ 2] 04 02     顯示字串 2（S.J. Blacksmith）
[ 4] 04 04     顯示字串 4（Gateway Temple）
[ 9] 0e 08
[18] 15 00 74 01 10 02 0b 0b 00 0e 09 14
[29] 01 15 09 11 01 0c 11 8f 0f     ← 登山術訓練，需要互動
```

段 N 的內容剛好是 `04 N`，所以「把 Index 直接當字串序號」會得到正確結果 ——
但那是碰巧對上，不是機制。remake 現在真的跑腳本，只認 `0x04`，
其餘 opcode 不顯示訊息而不是亂猜。

字串本身是**順序讀取**的：`sub_19016` 從 `word_154C0` 指向的位置逐位元組讀到
`0xFF`；`sub_18FD0` 負責「往後跳 N 條」。進入事件時 `word_154C0` 會被重設為
字串區開頭（`mov ax, word_1042C` / `mov word_154C0, ax`）。

## 3. 原版的起始座標

**Middlegate 的起點是 (7, 10)，面北。**

推導：原版從起點面北走四步會進神殿（顯示 "A slim cleric in a cowled robe…"），
而 `Gateway Temple` 這條字串的序號是 4，事件表裡 `Index=4` 的記錄在格 103 =
(7, 6)。往南四格就是 (7, 10)。

驗證：remake 從 (7,10) 面北走四步，確實停在 (7,6) 並顯示「門戶神殿」，
步數與終點都與原版一致。寫成回歸測試 `TestWalkToTempleFromStart`。

## 4. 下一步

1. 逐一解 opcode，優先處理 `0x0E`（可能是顯示字串的變體）、
   `0x01`（互動：詢問／扣錢／給技能）、`0x0B`。城鎮設施用的就是它們。
2. `Kind` 欄位（高 nibble）推測是觸發條件（走到／搜尋／開門），待驗。
3. 用原版 oracle 從 (7,10) 逐格移動對照每格的訊息。
