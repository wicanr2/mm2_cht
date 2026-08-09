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

**50 個 opcode**，跳表在 `jpt_1A676`。已認出的：

| opcode | 行為 |
|---|---|
| 1 | `call sub_1905E` → `sub_19016`（顯示字串） |
| 2 | `push 13h` / `call sub_19074` → 帶參數的字串顯示 |
| 3–50 | 未解 |

字串本身是**順序讀取**的：`sub_19016` 從 `word_154C0` 指向的位置逐位元組讀到
`0xFF`；`sub_18FD0` 負責「往後跳 N 條」。進入事件時 `word_154C0` 會被重設為
字串區開頭（`mov ax, word_1042C` / `mov word_154C0, ax`）。

## 3. 對 remake 的影響

完整的事件行為要實作這 50 個 opcode。在那之前，`internal/game` 的
`World.trigger` 用一個**近似**：把 `Index` 當字串序號直接取字串。

這個近似在 Middlegate 的抽樣格子上給出看起來合理的訊息，
但**沒有經過原版逐格對照，可能是巧合**。等級：**假設待驗**。

要驗證得靠原版 oracle：從已知位置逐格移動、記錄每格顯示的訊息，
與 remake 的輸出比對（流程見 [`docs/playtest/01`](../playtest/01-oracle-timeline.md)）。
在那之前不要把它當成已解。

## 4. 下一步

1. **確定原版的起始座標。** 目前 remake 的起點是猜的。原版走四步會進神殿，
   可以用這個特徵在地圖資料裡回推。
2. 逐一解 opcode，優先處理顯示字串、y/n 詢問、扣錢、給道具這幾類 ——
   城鎮設施用的就是它們。
3. `Kind` 欄位（高 nibble）推測是觸發條件（走到／搜尋／開門），待驗。
