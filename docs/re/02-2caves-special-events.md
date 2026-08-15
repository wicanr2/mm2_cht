# 特殊互動裝置：`2CAVES`

輸入檔：`MM2.EXE` ＋ `2PLAY.OVL` ＋ `2CAVES.OVL`（SHA-256 見 [`inventory.md`](../inventory.md)）。
IDA 位址取自 `workplace/ida/out/2PLAY.asm`／`2CAVES.asm`。

`2CAVES` 的名字容易誤導 —— 它不是「洞窟繪製」，是**事件腳本 `0x0e` 分派出去的
特殊互動裝置**：傳送機、滑梯陷阱、年代之門、捐獻換經驗、馬戲團。13 個進入點
全部由 `2PLAY` 的 `sub_19716` 呼叫。

## 1. `0x0e` 的完整分派表

`sub_19716` 讀一個位元組（`sub_18DC8`），先設 `ds:0395 = 1`（要求重畫）、
`ds:043A = 2`，再依值分派。**設施與特殊裝置都是直接呼叫 overlay，其餘才轉派
腳本庫**（`sub_1956E`，範圍表見 [`07-event-script.md`](../formats/07-event-script.md) §11）。

| 代碼 | thunk | 目標 | 內容 |
|---|---|---|---|
| 1 | `0x173D2` | 1RETINN `+C130` | 旅店（另設 `ds:043A = 1`）|
| 2 | `0x1741A` | 2MISC2 `+CE30` | 訓練基地 |
| 3 | `0x1740E` | 2BRAIN `+D15A` | 酒館 |
| 4 | `0x173EA` | 2TEMPLE `+CA88` | 神殿 |
| 5 | `0x173F6` | 2TEMPLE `+CB9C` | 法師公會 |
| 6 | `0x174E6` | 2SMITH `+CCBA` | 鐵匠 |
| 7 | `0x17432` | 2BRAIN `+C7E2` | 大腦淨化 |
| 8 | `0x1744A` | 2BRAIN `+C130` | 競技賽 |
| `0x64` | `0x173C6` | 2CAVES `e09` | 馬戲團 |
| `0x7E` | `0x174AA` | 2CAVES `e00` | 座標傳送機 |
| `0x7F` | `0x174B6` | 2CAVES `e01` | 隨機遭遇 |
| `0x80` | `0x174CE` | 2CAVES `e02` | 魔法滑梯陷阱 |
| `0x81`／`0x82`／`0x83` | `0x174DA` | 2CAVES `e03(0/1/2)` | 武器架／弓架／防具架 |
| `0xC9`／`0xCA` | `0x1737E`／`0x1738A` | 2CAVES `e10`／`e11` | Lord Hoardall／Lord Slayer 的任務 |
| `0xCB` | `0x17456` | 2CAVES `e04` | 黃金換經驗 |
| `0xCC` | `0x17462` | 2CAVES `e05` | 三倍泉（全隊黃金 ×3 換經驗）|
| `0xCD` | `0x1746E` | 2CAVES `e06` | 捐寶石換經驗 |
| `0xCE` | `0x1747A` | 2CAVES `e07` | 生命上限重算 |
| `0xCF` | `0x17486` | 2CAVES `e08` | **年代之門** |
| `0xE2` | `0x17402` | 2CAVES `e12` | 每日笑話 |
| `0xFD` | `0x17396` | 2SMITH `+CEC8` | 依 `ds:0395` 再分支（2 → 1RETINN `+C1EA`、3 → thunk `0x171E6`）|
| 其他 | — | `sub_1956E` | 轉派腳本庫 |

等級：**已證實**（分派碼逐條讀出，thunk 目標由 `tools/ovl_thunks.py` 反查）。

## 2. 年代之門（`0xCF`）

八個目的地，走的是「地圖 ＋ 座標」而不是相對位移：

| 選項 | 地圖 | X | Y | 世紀 `ds:03CA` |
|---|---|---|---|---|
| 1 | 15 | 0 | 0 | 不改 |
| 2 | 5 | 0 | 15 | 不改 |
| 3 | 33 | 15 | 15 | 不改 |
| 4 | 40 | 15 | 0 | 不改 |
| 5 | 11 | 7 | 6 | 5 |
| 6 | 37 | 5 | 5 | 6 |
| 7 | 6 | 8 | 3 | 7 |
| 8 | 38 | 14 | 4 | 8 |

三張 8 項的表分別在 `ds:36DA`（X）、`ds:36E2`（Y）、`ds:36EA`（地圖），
選項 ≥ 5 時才寫 `ds:03CA`，然後以 `(地圖, X, Y)` 呼叫 thunk `0x16ED8`。

**`ds:03CA` 就是事件腳本 `0x22`「世紀範圍檢查」讀的那個變數**，該 opcode 的
參數值域 5–9 與這裡寫入的 5–8 對得上（見 [`07-event-script.md`](../formats/07-event-script.md) §opcode 表）。
兩邊獨立解出而互相印證。

**入場條件**：掃全隊（`ds:0426` 人）取記錄指標，任一人的**記錄 `+128` 的 bit 1**
為 1 才開得了門；否則印 `sub_1C3F0(0x10)` 的第 16 號訊息。

**旗標的來源**：`EVENTSI` 的**腳本庫段 69 腳本 16**，由 `0e f3` 轉派
（代碼 `0xF3` − 基準 `0xE2` − 1 ＝ 腳本 16，見
[`07-event-script.md`](../formats/07-event-script.md) §11 的範圍表）——
「Lord Peabody needs the help of a Crusader. Will you offer your services (y/n)?」

那一段腳本寫兩次 `+128`：答應委託時 `18 00 7E FE 01` 點亮 **bit 0**，
年代之門要的 **bit 1** 由後面的 `18 00 7E FC 02` 寫，中間隔著一串逐人條件
（`15 NN 00 00 1B 28 …`）。拒絕年代之門的訊息也對得上 —— `sub_1C3F0(0x10)`
印的是 `If you wish to use the wayback machine see Lord Peabody.`

> **段號不是陣列索引。** `EVENTSI` 解出 44 段，段號是 0–4、17–32、45–63、
> 66–67、69–70 —— 陣列的第 42 個是段號 69。拿 `range` 的索引當段號寫進
> 筆記，位置就會指到不存在的地方。

等級：**已證實**（分派碼、目的地表、寫入端腳本與拒絕訊息四者互相印證）。

## 3. 魔法滑梯陷阱（`0x80`）

拿目前座標去查十項的來源表 `ds:3486`（X）／`ds:3490`（Y）；沒中就直接返回。
中了就：

1. 把目前格在**屬性層** `ds:5AD6` 的 bit 7 清掉（與開門寫的是同一層，見
   [`door-state-oracle.md`](../research/door-state-oracle.md)），並清 `ds:59C8` 的 bit 7；
2. 座標換成目的表 `ds:349A`（X）／`ds:34A4`（Y）的同索引項；
3. 印 `Magical slide trap!`，等一個按鍵；
4. **全隊每個人**把記錄的 `+106`–`+115` 逐個位元組右移一位、`+88`（目前 SP）
   當 word 右移一位。

`+107`–`+112` 是六個屬性的當前值、`+113` 等級、`+114` 法力等級、`+115` 耐力
（見 [`02-data-files.md`](../formats/02-data-files.md) §5），也就是**整組「當前值」鏡像連同 SP 全部砍半**，
沒有機率、沒有豁免判定。`+106` 的語意未解。

等級：**已證實**（控制流無分支，逐條讀出）。

## 4. 捐獻換經驗

| 代碼 | 換算 | 條件 |
|---|---|---|
| `0xCB` 黃金 | 記錄 `+102`（uint32）全數加進 `+98` 經驗值，黃金歸零 —— **1 金 ＝ 1 點經驗** | 該位置 `ds:0416[i] >= 24` → `This fountain does not recognize hirelings.`；黃金為 0 → `You have no gold` |
| `0xCD` 寶石 | 記錄 `+92`（uint16）**×10** 加進 `+98` 經驗值 | 寶石為 0 → `You have no gems.` |

寶石那個 ×10 是 `((g×4)+g)×2` 拆成移位加法算的，不是查表 —— 而完成訊息
`Was it worth your while?  A gem was worth ten experience points to me.`
把倍率寫在字面上，兩邊獨立對上。

訊息表在 `ds:3600`，每則兩行，`sub_1C3F0(n)` 印第 `n`、`n+1` 兩個指標：

| n | 內容 |
|---|---|
| 0 | Come back real soon. |
| 2 | This fountain does not recognize hirelings. |
| 4 | You have no gold |
| 6 | Your experience has multiplied three-fold. |
| 8 | Was it worth your while? A gem was worth ten experience points to me. |
| 10 | You have no gems. |
| 12 | You're maxxed out. |
| 14 | Not enough gold. |
| 16 | If you wish to use the wayback machine see Lord Peabody. |

等級：**已證實**。

## 5. 座標傳送機（`0x7E`）

依序印 `ds:346A` 那四行 —— `What is the magical location:`、空行、
`       X ( 0-15 ) ?`、`       Y ( 0-15 ) ?` —— 各收一個 0–15 的數字寫進
`ds:0393`（X）／`ds:0394`（Y），**不改地圖編號**，所以是同一張圖內的傳送。

等級：**已證實**。

## 6. 其餘裝置

### 隨機遭遇（`0x7F`）

怪物編號 ＝ `rand(1, 16) − 1 + Y × 16`，**用隊伍所在的那一列**決定是哪一段
十六隻。編號填進場上位置陣列 `ds:9680`，筆數是隊伍人數（`ds:0426`），
然後 root `0x13EB2` 開打（它把 `ds:0416` 抄進 `ds:5976`、`ds:0426` 抄進
`ds:5974`，播音效 2，再進戰鬥迴圈）。

> **`loc_16F76` 是亂數不是輸入。** root `0x11C88` 是個 LCG：`ds:4A14`
> 迭代之後 `mod (hi−lo+1) + lo`。收鍵盤的是 `sub_16EC2`（範圍 `'1'`–`'N'`，
> `0x1B` 取消）與 `loc_170DE`。這兩支長得很像，判錯會把「擲一隻怪」
> 讀成「讓玩家挑一隻怪」。

### 三個架子（`0x81`–`0x83`）

`ds:34C0` 是件數、`ds:34C4` 是起點，各三筆：

| 代碼 | 起點 | 件數 | 內容 |
|---|---|---|---|
| `0e 81` | 66 | 13 | Staff … Flamberge（長柄與鈍器）|
| `0e 82` | 92 | 6 | Blowpipe … Great Bow（遠程）|
| `0e 83` | 127 | 7 | Padded Armor … Plate Mail（護甲）|

**擲 `rand(1, 件數)`**，放進全隊第一個空的背包格（`+58` 起六格，
可用次數與屬性都清 0），印 `You have found a ` 加物品名（`ds:6960 + 20×編號`），
然後清掉這一格屬性層的 bit 7。沒有空位就什麼都不發生，那一格也不會被用掉。

三段各自成類是「起點與件數讀對了」的獨立佐證。

### 三倍泉（`0xCC`）

全隊每個人（跳過 `ds:0416[i] >= 24` 的雇傭兵）：`黃金 × 3` 加進經驗值，
黃金歸零。訊息是 `Your experience has multiplied three-fold.` ——
**字面說經驗變三倍，算的是黃金**，控制流沒有分支。

### 生命上限重算（`0xCE`）

選一名角色，算 `(耐力的屬性修正 + 職業每級生命) × 經驗等級`：

- 耐力修正走全遊戲共用的門檻表 `ds:4D84`（root `sub_1354A`，−3 到 +8）。
- 職業每級生命在 `ds:36B4`：騎士 12、聖騎士 10、弓箭手 10、牧師 8、
  巫師 6、盜賊 8、忍者 8、野蠻人 15。

兩道關卡：目前的基礎上限已經到了 → `You're maxxed out.`；
黃金的高位 word 小於 15（＝ 983,040）→ `Not enough gold.`。
過了就把 `+96` 與 `+116` 都設成算出來的值。**原版只驗不扣錢。**

### 馬戲團（`0x64`）

敘述四行（`ds:3A04`）＋ `Play (y/n)?`，答應之後列出七個攤位（`ds:3A0C`）。
勝負**不是擲的**：隊上只要有人的記錄 `+125` bit 1 為 1 就必贏，否則必輸。

| 攤位 | 記錄偏移 | 屬性 | 贏 |
|---|---|---|---|
| 1 Test of Strength | `+16` | 力量 | You rang the bell! |
| 2 Kissing Booth | `+18` | 人格 | You scored! |
| 3 Horseshoes | `+20` | 準確度 | You got a ringer! |
| 4 Head Dunk | `+39` | 耐力 | You survived! |
| 5 Sack Race | `+19` | 速度 | You won! |
| 6 Lucky Dice | `+21` | 運氣 | You rolled a 7! |
| 7 Shell Game | `+17` | 智慧 | Incredible memory! |

贏：每個帶旗標的人各 **+10**（`> 90` 就直接設成 100），**旗標用掉**。
輸：印該攤的失敗台詞，另擲 `rand(1, 254) <= 127`（一半）給安慰獎
`Cupie Doll`（物品 218）。

七個攤位對到七個屬性、七句贏、七句輸，是同一份對照的三重印證。

### 每日笑話（`0xE2`）

腳本那一格的文字是 `Here's the joke of the day:`，接著這支印一個笑話。

先把 **22 個笑話**（每個四行）從 `str.dat` 依序讀進 `ds:55C6`
（`sub_17732` 開檔、`loc_1773E` 逐筆讀，檔名指標在 `ds:0502`），
那正是 `STR.DAT` 的第 0–87 筆 —— 與 remake 既有的 `str.000`…`str.084`
逐格對得上，第 88 筆起換成酒館女侍的台詞。

挑哪一個：`今天 mod 22`。「今天」＝ `ds:03A2[ds:03CA × 2]`，也就是
opcode `0x23` 判日期時讀的同一格計數器（見
[`07-event-script.md`](../formats/07-event-script.md) §`0x23`）。

等級：**已證實**（腳本文字、字串範圍與日期公式三者互相印證）。

### Lord Hoardall 與 Lord Slayer 的任務（`0xC9`／`0xCA`）

`sub_1D3C4(0)` 是 Hoardall（找裝備）、`(1)` 是 Slayer（獵怪），同一支帶參數，
種類記在 `ds:55C4`。流程是**先結算、再看有沒有在任務中、都沒有才發新任務**：

	sub_1D094()   結算完成的任務 → 有就印獎勵，結束
	sub_1D252()   還在任務中 → 印「已經派給你了」，結束
	否則          問難度 A–D，指派

#### 任務狀態存在角色記錄裡

| 欄位 | 意義 |
|---|---|
| `+120` | 目標編號：Hoardall 是物品、Slayer 是怪物。0 表示沒有單一目標 |
| `+124` bit 0 | 0 ＝ Hoardall、1 ＝ Slayer |
| `+124` bit 1 | 指派的那隻**已擊殺**。由 `2COMBAT` 的 `sub_189D2` 在怪物死掉時點亮：逐一比對每個隊員的 `+120` 與死者編號，型別也要是 Slayer |
| `+124` bit 2 | **領主任務進行中**（只有難度 D 會設）|
| `+124` bit 3／bit 4 | Hoardall／Slayer 的領主任務已完成 |
| `+124` bit 5–7 | 領主任務的三隻獸各記一隻，湊滿 `0xE0` 才算完成 |

#### A／B／C：一個隨機目標

`sub_1CD1C(難度)`（Slayer）：`目標 = rand(1, 件數) + 起點`。

| 難度 | 怪物編號 |
|---|---|
| A Page's | 32–79 |
| B Squire's | 80–143 |
| C Knight's | 144–191 |

`sub_1CC8A(難度)`（Hoardall）：**六個裝備類別的加權挑選**。`ds:3E1E` 每難度
八個位元組，第 0 個是總數、第 1–6 個是各類別的件數；`ds:3E0C` 每難度六個
起點。擲 `rand(1, 總數)` 之後逐段扣，落在哪一段就從那一段的起點往後數。

| 難度 | 類別（起點–結束）|
|---|---|
| A | 1–24 短兵、66–78 長柄、92–97 遠程、115–117 盾、127–134 甲、155–156 盔 |
| B | 25–53、79–84、98–104、118–124、135–149、157–158 |
| C | 54–65、85–91、105–114、125–126、150–154、159 |

六段全部落在乾淨的類別邊界上（`Small Club`…`Katana`、`Staff`…`Flamberge`、
`Blowpipe`…`Great Bow`…），是「總數與起點讀對了」的獨立佐證。

#### A／B／C 的驗收

`sub_1CB4A`（Slayer）：要 `+124` bit 1（打死了）、目標與全隊記的同一隻
（`ds:55C2`）、角色的 `+38` 狀況 `< 0x80`。獎勵依**目標的編號**查兩張十項表：

	門檻 ds:3E3E = 48 64 80 96 112 128 144 160 176 192
	經驗 ds:3E48 = 2,000 4,000 5,000 7,000 10,000 15,000 25,000 50,000 100,000 250,000

`sub_1CBCA`（Hoardall）：隊伍身上要有那件物品（`sub_1CB00` 找、`sub_1CB1C`
收走），同樣要 `+38 < 0x80`。獎勵是**那件物品的價值**（物品記錄 `+18` 的 word）。

兩者結算後都把 `+120` 清成 0。

#### D：領主任務

`sub_1CEB2` 不指派單一目標 —— 它對每個「還沒完成過這位領主」的隊員設
`+124` bit 2 與型別位元，`+120` 保持 0。目標是固定的：

- **Hoardall**：三把劍 **Valor Sword（226）、Honor Sword（227）、Noble Sword（228）**。
  `sub_1CD4C` 檢查三把都在隊伍身上，是就一併收走。
- **Slayer**：三隻獸，`+124` 的 bit 5–7 各記一隻，湊滿 `0xE0` 才算。

完成的獎勵是**固定的經驗值**，與難度無關：

| 領主 | 經驗 | 完成旗標 |
|---|---|---|
| Hoardall | 100,000 | `+124` bit 3 |
| Slayer | 1,000,000 | `+124` bit 4 |

畫面上的字：`You have done everyone a great service and you shall be rewarded.
N experience points!`；還在任務中是 `Your party has already been quested to
seek out the …`（單一目標印名字，領主任務印 `three swords.`／`three beasts.`）；
按 ESC 是 `Then begone, knave!`。

`ds:55C2`／`ds:55C3` 是「全隊共用的目標」暫存，由 `sub_1D094` 在進入
逐人驗收那一段之前清成 0（`sub al, al` 之後連寫兩格）。

等級：**已證實**（狀態位元的寫入端與讀取端都追到了，兩張獎勵表與六段裝備
類別各自成類）。

## 7. remake 現況

**`2CAVES` 的每一支都接上了**（`internal/game/cave.go`、`internal/game/quest.go`、
`internal/ui/cave.go`）。不需要玩家輸入的五支（隨機遭遇、三個架子、三倍泉、
每日笑話）由 `Session.Step` 當場做完，其餘由 `World.Device` 交給 UI 開畫面，
與設施同一條路。

任務是唯一跨系統的一支：狀態存在角色記錄的 `+120`／`+124`，而「打死了指派
的那隻」由戰鬥回報 —— remake 接在 `Encounter.recordDefeat`，對應原版
`2COMBAT` 的 `sub_189D2`。

呈現上兩處與原版不同，機制不變：座標傳送機把原版的兩次提問合成一行
（`X Y`），年代之門把「這個選項會改世紀」標在選單上 —— 原版畫面只有
`What era do you desire (1-8)?` 一行，看不出 1–4 與 5–8 的差別。
