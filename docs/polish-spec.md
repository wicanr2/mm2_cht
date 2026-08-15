# remake polish 規格

這份文件的用途是把「RE 已經解出來、但 remake 還沒照做」的事情逐條寫成
可實作、可驗收的規格。流程固定：**讀 RE 文件 → 寫這裡 → 實作**。

每一條都要有：原版位址、目前 remake 的行為、目標行為、驗收條件。
**沒有原版證據的條目不寫進來** —— 想加的玩法另開一節標明是 remake 自己的。

狀態欄：`已完成` / `實作中` / `待實作` / `擋住`（擋住的一定要寫下一步）。

---

## 1. 已完成

| # | 項目 | 原版依據 | 驗收 |
|---|---|---|---|
| P1 | 門打開改屬性層牆位元，離圖還原 | root `sub_13A64`；`2PLAY sub_1BE24` 換圖重載 | `internal/game/door_test.go` 三條 |
| P2 | 事件 `Kind` 是方向遮罩，面向不對不觸發 | `2PLAY sub_1A8C4` `0x1A939`；遮罩表 `ds:16D2` | `world_test.go` 的招牌兩條 ＋ DOSBox 四步時間線 |
| P3 | 動畫表第一段是播放腳本，bit 7 ＝隨機挑段 | root `0x15715`／`sub_15772`；`sub_11C88` 是亂數 | `monsters_test.go` 的 240 段／136 腳本項 |
| P4 | `Protection` 三行接上真來源 ＋ 飄浮術第四行 | root `sub_147D8` `0x1486B` 起逐列 | `view/status_test.go` 兩條 |
| P5 | 突襲狀態開戰時擲，盜行高易先手、守衛術擋突襲 | `2COMBAT _2combat_e03` `0x1A4E7`；`ds:549E` ＝ root `sub_13A9E` | `roster_combat_test.go` 的 `TestRollAmbush` |
| P11 | `S` 搜尋指令；戰利品不再自動彈四選單 | `2PLAY` `0x181E8` → thunk `0x17336` → root `0x13814` | `session_test.go` 的 `TestCombatVictoryNeedsSearch` ＋ `TestChestPage` |
| P12 | `B` 撞門、`Q` 查說明書照原版；商店移到 `G` | [`command-keys-oracle`](research/command-keys-oracle.md) §2 | `go test ./...`；README 與 `cmd/mm2/main.go` 一致 |
| P15 | `2CAVES` 十一支特殊裝置：座標傳送機、滑梯陷阱、隨機遭遇、三個架子、捐黃金／寶石換經驗、三倍泉、生命上限重算、馬戲團、年代之門、每日笑話 | [`docs/re/02`](re/02-2caves-special-events.md) | `internal/game/cave_test.go` 十三條 ＋ `internal/ui/cave_internal_test.go` 三條 |
| P14 | 片頭畫面：`MASTER.16` 的底圖加 13 張疊圖動畫，接 `intro` 音樂 | 60 張 DOSBox 截圖逐像素定落點；[`04-graphics`](formats/04-graphics.md) §標題畫面的疊圖動畫 | `internal/ui/intro_test.go` 兩條 ＋ `docs/screenshots/17-intro.png` |
| P13 | 世界地圖頁（`W`）：5×4 網格由 `ATTRIB` 鄰接現算 | [`world-grid-oracle`](research/world-grid-oracle.md) | `worldgrid_test.go` 三條 ＋ `TestWorldPage` ＋ `docs/screenshots/14-world-grid.png` |

---

## 2. 待實作

### ~~P6 場景碼 `ds:039C` 要由地圖編號算出來~~（已完成）

**原版依據。** `2PLAY sub_1B410(地圖編號)` 在三張各 7 筆的表上做區間查找，
`_2play_e11`（`0x1B6C7`）把結果寫進 `ds:039C`：

```asm
cl = 7                              ; 預設「不在世界裡」
for si in 0..6:
    if ds:16E8[si] <= 地圖 <= ds:16F0[si]: cl = ds:16E0[si]
    if cl != 7: break
return cl
```

三張表的實際值（`ds:16E0` 場景碼、`ds:16E8` 下界、`ds:16F0` 上界）：

| 場景碼 | 地圖區間 | 張數 |
|---:|---|---:|
| 0 | 0–4 | 5 |
| 3 | 5–16 | 12 |
| 1 | 17–32 | 16 |
| 6 | 33–40 | 8 |
| 4 | 41–44 | 4 |
| 5 | 45–54 | 10 |
| 2 | 55–59 | 5 |

**60 張不重不漏**，而場景 0 正好是五座城鎮 —— 與「地圖 0–4 是城」互相印證。
`1RETINN _1retinn_e01` 另外把 `ds:039C` 設成 7（旅店名冊畫面，不在世界裡）。

**目前 remake。** `World.Scene` 是欄位，預設 0，沒有人算它；
`gamedata.Pictures.Scene` 已經有 7 張表，opcode `0x0b` 拿 `w.Scene` 去查。
所以現在**所有地圖都當場景 0**，地城與野外的事件圖號會查到城鎮那張表。

**做法。** 沒有「換圖時記得更新」這回事 —— `World.Scene` 由欄位改成
**從 `MapIndex` 導出的方法**，那條漏就結構上不存在了。原版是每次換圖重算再
寫進 `ds:039C`，行為相同。

**驗收（`internal/game/scene_test.go`）。** 七個區間不重不漏蓋滿 60 張、
逐張比對場景碼、五座城鎮都是場景 0、出界回 `SceneOutside`（7）。

### ~~P9 天空影格由天花板位元圖決定~~（已完成）

**原版依據。** `2PLAY _2play_e02`（`0x18517`）查 `ATTRIB.DAT` `+32`…`+63`
那張 16×16 位元圖（每列 2 bytes、遮罩 `1 << (x & 7)`），回傳 0／1，
`_2play_e03`（`0x18773`）直接拿它當 `SKY.16` 的影格編號。

所以**影格 1 是天花板不是天空**。資料側：24 張全 0 的圖精確等於獨立已知的
「二十四張野外圖」，26 張全 1 的是地城與城堡，10 張混合的是五座城鎮與
另外五張。這同時把 `+32`…+63`「位元圖，用途未定」解掉了。

**先前為什麼對不起來**：拿「正牆在第幾格」去對 —— 它跟牆無關，只看腳下那一格。

**目前 remake。** `drawSky` 寫死影格 0。

**做法。** `MapAttr.Ceiling(x,y)`，`UseAttrs` 時攤進 `Map.Ceiling`，
`drawSky` 依隊伍所在格挑影格。

**驗收（`internal/game/ceiling_test.go`）。** 全 0 的那批必須精確等於
`Indoor()` 為假的那批（一張不差）、五座城鎮必須是混合、要有整張全 1 的圖。

### ~~P10 `ds:03C8` 不必實作~~（已完成：確認它沒有讀取端）

`ds:03C8` 全十五個資料庫**只有一處寫**（`2PLAY sub_1A202`，事件 opcode
`0x2c` 的累加）、**零處讀**。與 `ds:9E34`（怪物 `+0x16` bit7）、屬性層
bit 5 同一類：原版寫了但不用。remake 原樣往返即可，不必替它發明單位。

（限制：掃描比對的是運算元的位移值，範圍 `03C8`–`03CB`。若有程式以
更低的基底加索引讀它，這個掃法看不到 —— 但同一塊的 `ds:03CA` 讀取端
到處都是且全是直接定址，沒有索引式存取的跡象。）

### ~~P8 定位術要打開地圖畫面~~（已完成）

**原版依據。** `2CAST1 sub_1C1D2`（法術編號 5、引擎編號 53）在共用確認提示
過關之後呼叫 `_2play_e00` 與 `_2play_e14`（thunk `0x172B2`）—— 它用 `.16` 的
`B` 圖磚以 16 px 步長把整張俯視圖畫出來。`0x172B2` 這個 thunk 只有 2CAST1
用，但 `_2play_e14` 本身另有一個近呼叫端：2PLAY 指令分派 `0x18189` 的 `M` 鍵
（見 [`command-keys-oracle`](research/command-keys-oracle.md)）。
**定位術與 `M` 打開的是同一張圖。**

珍017 中文說明書逐字相符：定位術「給予隊伍所在之精確位置，並**顯示目前
16×16 區域之地圖，標示你所在位置及方向**」—— 那組圖磚裡的
↑ ↓ → ← 四個箭頭就是「標示方向」。

**這同時把 04-graphics.md 的 B3 解掉了**：俯視地圖的觸發指令就是定位術。

**目前 remake。** 有完整的地圖畫面（`M` 鍵），但定位術只回一句話。

**做法。** `CastResult.ShowMap` 由定位術設起來，UI 收到就切 `ModeMap`；
同時把整張圖標成看過（`Explored.MarkMap`）—— 原版是把 16×16 直接畫出來，
而 remake 的地圖畫面是持久的，「畫出整張」對應成「整張標成看過」。

**驗收（`internal/game/locate_test.go`）。** 施法後 `ShowMap` 為真、
當前圖的四角與中間都標成看過、別張圖不受影響。

### ~~P7 戰利品不給 `0xF0` 的物品附魔~~（已完成）

**原版依據。** `2COMBAT sub_19A3C` `0x19ADF`：`+0x0E == 0xF0` 時跳過隨機
附魔與充能那一段，物品編號照樣寫入。58 件是 `0xF0`（鑰匙、票券、藥水等
非裝備品）。

**目前 remake（寫這條規格時的認知是錯的）。** 以為 remake 沒有這一條，
實際上它有 —— 但做成 `continue`，**整件跳過**，於是那 58 件永遠不會掉出來。

**原版的順序才是關鍵**：`0x19AC2` 先把編號寫進 `ds:6950`、`0x19AC9` 依
`+0x0F` 取充能、`0x19ADF` 才檢查 `0xF0`；命中就跳到函式尾端，而尾端照樣把
充能寫回去。所以是「照樣掉、充能照給、只有附魔那一擲沒跑到」。

**驗收（`TestVictoryLootKeepsNonEquipItems`）。** 把整張物品表設成 `0xF0`
跑 200 場戰鬥：一定要掉得出東西，而且沒有任何一件帶附魔或被標成魔法物品。

### ~~P11 `S` 搜尋指令：戰利品要玩家自己撿~~（已完成）

**原版依據。** `2PLAY` 主迴圈 `0x18239` 比到 `'S'` 就跳 `0x181E8`，經 thunk
`0x17336` 打到 root `0x13814`。那支印 `Search...`，掃三個物品槽（`ds:6950`）、
金幣（`ds:695C/E`）、寶石（`ds:695A`），五組全空就印 `Nothing Here!`，
否則進 `2MISC _2misc_e02` —— 而 `_2misc_e02` 的 thunk **全域只有這一個呼叫端**。
所以戰利品、事件獎賞、神殿物品**全部只能經由搜尋領取**。完整按鍵表與
`ds:0434` 三狀態見 [`command-keys-oracle`](research/command-keys-oracle.md)。
珍017 說明書上冊 p.39 相符：「戰鬥後不要忘了用 `S` 找尋戰利品」。

**目前 remake。** 沒有搜尋指令。戰鬥一結束 `fightRound` 就把 `Chest` 塞進 UI
並自動彈四選單；四選單另外掛在自創的 `G` 鍵上，而 `S` 被存檔佔著。

**做法。**

- `KeyChest` 改名 `KeySearch`，綁 `S`；存檔移到 `F2`（原版的存檔在 `O` 指令
  視窗裡，本來就不是字母鍵），拿掉自創的 `G`。
- 沒有待領的東西時回「這裡什麼都沒有。」，對應 `Nothing Here!`。
- 戰鬥勝利改成把箱子擺著不自動開，訊息提示按 `S` 取。
- 事件 `0x2a` 的待領獎賞維持 `ClaimReward` 自動發放 —— 原版那條也要按 `S`，
  但 remake 的事件流程沒有「按鍵前先把訊息掛著」的中斷點，硬改會讓獎賞在
  事件文字之後憑空出現。**這是刻意的差異，記在這裡而不是假裝一致。**

**驗收（`internal/ui/session_test.go`）。** 戰鬥勝利後不進選單、`Chest` 仍在；
按 `S` 才進四選單；沒有箱子時按 `S` 得到「這裡什麼都沒有。」且不進選單。

### ~~P12 鍵位別跟原版打架~~（已完成）

**原版依據。** [`command-keys-oracle`](research/command-keys-oracle.md) §2 的
十三個按鍵。遊戲內按 `K` 查得到的說明書轉錄（`data/reference.json` 的
`fieldCommands`）列的就是這一組原文指令名，玩家照著按會按到別的東西。

**目前 remake 的衝突。** `B` 是商店（原版是撞門）、`D` 是撞門（原版是解僱）。
其餘差異都是原版沒有的功能佔了原版沒用的字母，不算打架。

**做法。** `B` 還給撞門，商店移到原版沒用的 `G`；`Q` 加成「查說明書」的別名
（原版 `Q` 是 Quick Ref，remake 的 `K` 是它的超集）；`D` 空著不亂給
—— remake 沒有解僱功能，把它綁到別的指令只會製造第二次衝突。

**驗收。** `go test ./...`；README 的按鍵表與 `cmd/mm2/main.go` 一致。

### ~~P13 世界地圖：說明書最後一項沒收進遊戲的東西~~（已完成）

**原版依據。** 二十張野外圖由 `ATTRIB +5`…`+8` 連成 5×4 的環面，
`C2` ＝ 地圖 11（`EVENTSO` 第 11 段 `(7,3)` 的 `0c 00 f5` 換到米德格特，
與說明書「區域 C2，X=7 Y=3」相符）。推導與交叉驗證見
[`world-grid-oracle`](research/world-grid-oracle.md)。

**目前 remake。** `data/reference.json` 的 `worldMap` 有 51 條「區域碼 → 地名」，
按 `K` 查得到；但**那張圖本身沒有**，玩家看得到 `C2 方形湖` 卻不知道 C2 在哪。
掃描頁不能散布，所以只能自己畫 —— 而畫它需要的資料玩家本來就有。

**做法。**

- `internal/game/worldgrid.go`：由 `MapAttr` 的鄰接欄位現算 5×4 網格，
  **整份只寫死一個常數**「A1 ＝ 地圖 5」。提供 `WorldGrid()`、
  `RegionOf(地圖)`、`WorldTileset(地圖)`。
- 新的 `W` 鍵與 `ModeWorld`（原版沒用到 `W`）：5×4 的區域格，每格標區域碼、
  依貼圖組上色，沒踏進去過的壓網點；隊伍所在的那一格加亮框，右欄印區域碼、
  該區域的地名（取自 `reference.json`）與四色圖例。
- 城鎮／地城四面自指，不在網格上，右欄改印地名加「不在世界網格上」。

**驗收（`internal/game/worldgrid_test.go`）。** 二十張不重不漏、每一格的
東鄰與南鄰都與 `ATTRIB` 相符、`C2` 是地圖 11、A1／B1 是凍原、D2／E2 是沙漠、
D4 是沼澤、四個元素領域與五座城鎮不在網格上；UI 端 `TestWorldPage`
按 `W` 進得去畫得出來，畫面證據 `docs/screenshots/14-world-grid.png`。

---

## 3. 擋住

（空）
