# 任務

MM2 的任務不是一套獨立系統，而是散落在事件腳本裡的一組約定：**條件寫在角色記錄尾端的
十二個位元組**，**發派與結算寫成腳本或 overlay 函式**，**獎勵幾乎都是經驗值**。
這份文件把目前解出來的任務線收攏成一處：怎麼觸發、目標是什麼、驗收條件、獎勵多少、
狀態記在哪一個位元。

資料來自四個地方：`EVENTSI.DAT`／`EVENTSO.DAT` 的腳本（本專案的
`internal/assets/events` 解析結果）、`2CAVES.OVL` 的反組譯（[`docs/re/02`](re/02-2caves-special-events.md)）、
珍017 中文說明書轉錄（[`docs/manual/`](manual/)）、以及《軟體世界》的攻略整理（`data/hints.json`）。
前兩者是一手證據，後兩者只當佐證與交叉檢查。

## 0. 出處標記怎麼讀

| 標記 | 意思 |
|---|---|
| `EVENTSI 段 55 腳本 15` | `internal/assets/events` 解析後的段號與 `Scripts` 下標。`EVENTSI` = 室內（城鎮／城堡／地城），`EVENTSO` = 野外 |
| `indoor.55.020` | `translations/strings.json` 的字串 key。**腳本運算元 N 對應 key 的 N−1** —— 直譯器 `internal/game/world.go` 取字串時就是 `script[p+1] - 1`，[`07-event-script.md`](formats/07-event-script.md) §7 的招牌對照表也是同一個關係 |
| `+124 bit 2` | 角色記錄的**位元組偏移**（記錄長 130 bytes）|
| `選擇器 0x7a` | 腳本 opcode `0x15`／`0x18` 用的欄位選擇器。**選擇器不等於偏移**，對照表在 `data/fields.json`（由 `2PLAY.OVL` 的 `sub_1AA00` 跳表產生）|
| `sub_1CB4A`、`ds:3E48` | `2CAVES.OVL`／DGROUP 位址，見 [`docs/re/02`](re/02-2caves-special-events.md) |

野外地圖編號與遊戲內的區域碼（`A1`–`E4`）由飛行術的 5×4 表對應
（[`09-spells.md`](formats/09-spells.md)「飛行術」）：

```
A  5  9 12 15        B  6 10 13 16        C  7 11 14 38
D  8 34 36 39        E 33 35 37 40
```

座標一律寫成 `(X, Y)`，與原版顯示一致（X 是格號低 nibble，Y 是高 nibble，北是 +Y）。

## 1. 任務狀態存在哪裡

角色記錄尾端 `+118`–`+129` 那十二個位元組是全遊戲的任務進度。下表只列已經追到寫入端
與讀取端的位元；同一個位元組裡沒列出的位元是還沒對上事件的。

| 選擇器 | 偏移 | 已解出的位元 | 等級 |
|---|---|---|---|
| `0x77`／`0x78` | `+118` | 各城鎮、各區域的單次事件旗標（八個位元散在 `EVENTSI` 段 3／4／25、`EVENTSO` 段 7／8／9／14／33／38／40）| 強推論 |
| `0x74` | `+121` | bit 7 ＝ 職稱後面的 `+`（職業考驗通過）；bit 1 ＝ 法師公會會員（`EVENTSI` 段 60 腳本 4，付 20 金）；bit 6 ＝ 已見過守護天馬（`EVENTSO` 段 11 腳本 4）| 已證實 |
| `0x75` | `+122` | bit 0 ＝ 職業考驗已完成、獎勵未領；bit 1 與 bit 0 一起被清掉；bit 2–7 ＝ 巫師考驗的四組密碼與兩位巫師的釋放狀態 | 已證實 |
| `0x76` | `+123` | bit 0–4 ＝ Nordon／Nordonna 委託鏈；bit 7 ＝ 已擊敗 Spaz Twit（拿到相位槍）| 已證實 |
| `0x79` | `+120` | **領主任務的目標編號**（Hoardall 是物品、Slayer 是怪物），0 表示沒有單一目標 | 已證實 |
| `0x7a` | `+124` | 領主任務旗標，見 §2 | 已證實 |
| `0x7b` | `+125` | 泛用劇情旗標（bit 1 ＝ 馬戲團必贏、bit 5 ＝ Dawn 已死）。腳本用它 48 次，**多數位元的語意未解** | 部分已證實 |
| `0x7c` | `+126` | 競技賽獎章：綠票三場 bit 0–2、黃票三場 bit 3–5 | 已證實 |
| `0x7d` | `+127` | 競技賽獎章：紅票三場 bit 0／5／6、黑票三場 bit 1–3；bit 4 ＝ 已取走 Hoardall 城堡財寶 | 已證實 |
| `0x7e` | `+128` | bit 0／1 ＝ Lord Peabody 的委託；bit 2／3 ＝ 大德魯伊的委託；bit 4 ＝ Lord Haart 的委託；bit 5 ＝ 黑主教已認證黑券三冠 | 已證實 |
| `0x7f` | `+129` | bit 0 ＝ 天命之人（Chosen One）；bit 1 ＝ King Kalohn 已改寫歷史；bit 4 ＝ 全隊已獲女王接見；bit 5 ＝ 已接到 Square Lake 的指令（結局密碼 `WAFE` 的前提）；bit 6／7 ＝ Murray 的委託 | 已證實 |

另外一組劇情狀態是**全域的 24 個位元組**（`ds:03F6` 起，選擇器 `0x00`–`0x17`，
見 [`07-event-script.md`](formats/07-event-script.md) §12）。腳本用 `0x1a` 寫、`0x17` 讀，
存檔兩邊都要存。目前確認的寫入端有四處：`EVENTSI` 段 60 腳本 2（Nordonna 的兒子獲救）寫
全域 0 與 1、段 70 腳本 4 寫全域 11 與 12、段 25 腳本 6（Sir Kill 與 Jed I 獲救）寫全域
19 與 20、`EVENTSO` 段 8 腳本 9（Red Duke 與 Dead Eye 獲救）寫全域 13 與 14。
四處都是「兩個人一組獲救」，而且都由同一組的第一個位元擋著重複觸發 ——
**形狀是「這兩個人已可在旅店雇用」**，等級 **強推論**。

**「選擇器 ≠ 偏移」這件事要小心。** `data/fields.json` 顯示 `0x74`–`0x7f` 對到的偏移是
`121, 122, 123, 118, 118, 120, 124, 125, 126, 127, 128, 129` —— 不是連續遞增。
拿「`0x74` 起連續」去換算，領主任務的目標欄位會算成 `+118`，整段結論都會歪掉。

## 2. 兩位領主：Lord Hoardall 與 Lord Slayer

兩位領主是唯一有「難度選單、隨機目標、獎勵表」的任務系統，也是唯一寫進 overlay 的。
機制全解在 [`docs/re/02`](re/02-2caves-special-events.md) §6（`sub_1D3C4`、`sub_1CC8A`、
`sub_1CD1C`、`sub_1CB4A`、`sub_1CBCA`、`sub_1CEB2`、`sub_1CD4C`），這裡補上**入口與門檻**。

### 2.1 入口與 Crusader 門檻

| 領主 | 入口 | 腳本 | 分派碼 |
|---|---|---|---|
| Lord Hoardall（找裝備）| `EVENTSI` 段 56「Hoardall's Palace」＝ Castle Woodhaven，王座廳 (9, 11) | 段 56 腳本 8 | `0e c9` → `2CAVES e10` |
| Lord Slayer（獵怪）| `EVENTSI` 段 55「Slayer's Palace」＝ Castle Hillstone，王座廳 (5, 2) | 段 55 腳本 15 | `0e ca` → `2CAVES e11` |

段號與城堡名的對應是由散落各地的線索釘死的：`M-27 Radicon rests at 2,11 in Castle
Woodhaven` 對到段 56 的 (2, 11)、`Seek the N-19 Capitor at 3,13 in Castle Hillstone`
對到段 55 的 (3, 13)（見 §8 的四件零件）。

兩支腳本的形狀相同：

```
0b 0e 00      顯示圖 14
32 04         數隊伍裡具備第二技能 4 的人數 → ds:042F
11 01         為 0 就跳過下一個 opcode
0e c9 / 0e ca 進領主的任務畫面
02 ..         印「只有 Crusader 幫得上忙」
```

**第二技能 4 是 Crusader（宗教家）**，說明書給的說明正是「使人物／隊伍可以被授與任務」
（[`docs/manual/part-2.md`](manual/part-2.md) §第二技能表，`data/reference.json` 的 `skills`）。
兩邊獨立對上：腳本用 `0x32`（數第二技能）當門檻，說明書把這項技能的用途寫在字面上。
拒絕的台詞是 `indoor.56.019`（Hoardall）與 `indoor.55.020`（Slayer）。

等級：**已證實**（腳本逐位元組讀出，技能編號有說明書佐證）。

### 2.2 難度 A／B／C：一個隨機目標

選單四項的字串在 `MM2.EXE`：`exe.3CE5` A) Page's Quest、`exe.3CF5` B) Squire's Quest、
`exe.3D07` C) Knight's Quest、`exe.3D19` D) Lord's Quest。

**Slayer** 的目標是 `rand(1, 件數) + 起點`（`sub_1CD1C`）：

| 難度 | 怪物編號 |
|---|---|
| A Page's | 32–79 |
| B Squire's | 80–143 |
| C Knight's | 144–191 |

**Hoardall** 的目標是六個裝備類別的加權挑選（`sub_1CC8A`，件數表 `ds:3E1E`、起點表 `ds:3E0C`）：

| 難度 | 六個類別的編號區間 |
|---|---|
| A | 1–24 短兵、66–78 長柄、92–97 遠程、115–117 盾、127–134 甲、155–156 盔 |
| B | 25–53、79–84、98–104、118–124、135–149、157–158 |
| C | 54–65、85–91、105–114、125–126、150–154、159 |

目標編號寫進 `+120`，`+124` bit 0 記種類（0 ＝ Hoardall、1 ＝ Slayer）。

### 2.3 A／B／C 的驗收與獎勵

| 領主 | 驗收（`sub_1CB4A`／`sub_1CBCA`）| 獎勵 |
|---|---|---|
| Slayer | `+124` bit 1（指派的那隻已擊殺，由 `2COMBAT` 的 `sub_189D2` 在怪物死亡時比對 `+120` 點亮）、目標與全隊記的同一隻（`ds:55C2`）、角色 `+38` 狀況 `< 0x80` | 依**目標編號**查兩張十項表：門檻 `ds:3E3E` = 48／64／80／96／112／128／144／160／176／192，經驗 `ds:3E48` = 2,000／4,000／5,000／7,000／10,000／15,000／25,000／50,000／100,000／250,000 |
| Hoardall | 隊伍身上有那件物品（`sub_1CB00` 找、`sub_1CB1C` 收走）、`+38 < 0x80` | **那件物品的價值**（物品記錄 `+18` 的 word）|

結算後 `+120` 清成 0。畫面上的字：完成是 `exe.3EBB` + `exe.3EE2`
（`You have done everyone a great service and you shall be rewarded. N experience points!`），
還在任務中是 `exe.3D29` + `exe.3D4D`，按 ESC 是 `Then begone, knave!`。

《軟體世界》的說法可以對上：「page、squire、Knight 三級任務多半是要你交裝備給他，
那些東西打怪就會掉」（`data/hints.json`，places 31）。

### 2.4 難度 D：領主任務

`sub_1CEB2` 不指派單一目標（`+120` 保持 0），只對每個「還沒完成過這位領主」的隊員設
`+124` bit 2（進行中）與型別位元。目標是固定的三件事。

**Hoardall：三把劍。** `sub_1CD4C` 檢查三把都在隊伍身上，是就一併收走。
三把劍不是掉落物，各由一個野外守衛把著，打贏之後用 opcode `0x2a`（待領的獎賞）發下：

| 物品 | 編號 | 地點 | 腳本 | 對手 |
|---|---|---|---|---|
| Valor Sword 英勇之劍 | 226 | A2（地圖 9）(11, 2) | `EVENTSO` 段 9 腳本 3 | Mountain Man（120）×6 |
| Honor Sword 榮譽之劍 | 227 | D4（地圖 39）(14, 11) | `EVENTSO` 段 39 腳本 10 | Guardian（125）×6 |
| Noble Sword 高貴之劍 | 228 | D1（地圖 8）(0, 8) | `EVENTSO` 段 8 腳本 8 | Priest（137）×4 |

`0x2a` 的運算元是 `3 bytes 金錢 + 2 bytes 寶石 + 3×3 bytes 三件物品`，三處各只填第一件
（`e2`／`e3`／`e4`），金錢與寶石都是 0。地點與遊戲內的線索對得上：`indoor.26.017`
（`The Sword of Nobility ... is guarded in D1 at 0,8`）與 `indoor.26.018`
（`the Sword of Valor, hidden for years in A2 at 11,2`）。

**Slayer：三隻獸。** `+124` 的 bit 5／6／7 各記一隻，湊滿 `0xE0` 才算完成。三隻各自是一個
野外事件，腳本把牠們稱作「邪惡使者（Envoy of Evil）」：

| 位元 | 台詞 | 怪物 | 地點 | 腳本 |
|---|---|---|---|---|
| `+124` bit 7 | The Winged Envoy of Evil attacks you! | 244 Dragon Lord 龍王 | D1（地圖 8）(10, 12) | `EVENTSO` 段 8 腳本 7 |
| `+124` bit 6 | The Crawling Envoy of Evil attacks! | 242 Queen Beetle 甲蟲女王 | E2（地圖 35）(11, 6) | `EVENTSO` 段 35 腳本 1 |
| `+124` bit 5 | The Slithering Envoy of Evil attacks! | 243 Serpent King 巨蛇王 | E3（地圖 37）(5, 6) | `EVENTSO` 段 37 腳本 2 |

三處的腳本形狀一樣：讀該位元，已設就整段跳過；否則印台詞、擺好固定遭遇（`0x12`）、
**寫入該位元**。《軟體世界》記的三隻與座標逐項相同（`data/hints.json`，places 32：
「Queen Beetle（E2-11,6）、Serpent King（E3-5,6）、Dragon Lord（D1-10,12）」）——
腳本、怪物名、外部攻略三者互相印證。

⚠ **位元是在遭遇被擺好的當下寫入的，不是在擊殺時。** 腳本裡 `18 00 7a ..` 緊接在
`12 ..` 後面，中間沒有任何條件。這表示逃跑或全滅之後那一格的旗標可能已經算數了。
等級：**強推論** —— 控制流讀得清楚，但「戰鬥實際在腳本的哪一步開打」還沒用 DOSBox 驗過。

完成領主任務的獎勵與難度無關，是固定的經驗值：

| 領主 | 經驗 | 完成旗標 |
|---|---|---|
| Hoardall | 100,000 | `+124` bit 3 |
| Slayer | 1,000,000 | `+124` bit 4 |

### 2.5 領主任務旗標總表

| 欄位 | 意義 |
|---|---|
| `+120` | 目標編號（Hoardall 是物品、Slayer 是怪物），0 表示沒有單一目標 |
| `+124` bit 0 | 0 ＝ Hoardall、1 ＝ Slayer |
| `+124` bit 1 | 指派的那隻已擊殺（`2COMBAT` `sub_189D2`）|
| `+124` bit 2 | 領主任務進行中（只有難度 D 會設）|
| `+124` bit 3／bit 4 | Hoardall／Slayer 的領主任務已完成 |
| `+124` bit 5／6／7 | 三隻獸各記一隻，湊滿 `0xE0` |

### 2.6 取消任務：Uncle Spudly 的藥水

兩座領主城堡的 (13, 2) 都有一個攤子（`EVENTSI` 段 55 腳本 41、段 56 腳本 24，
都是 `0e fb` → 腳本庫段 70 腳本 7）：

```
02 0b   "Castle guards peddle Uncle Spudly's New and Improved Quest Removal
         Elixir. It can be yours to drink for only 19 gold. Buy it (y/n)?"
24 13 00        全隊湊 19 金
02 0d   "This stuff works great! Your quest has been removed."
18 00 79 00 00  +120 = 0        ← 清掉目標編號
18 00 7a fb 00  +124 &= 0xFB    ← 清掉「進行中」
```

**它只清 `+120` 與 `+124` bit 2**，不動 bit 1（已擊殺）、bit 0（型別）與 bit 3–7。
《軟體世界》記的「13,2 有人賣藥水，喝了可以取消正在執行的任務」逐項對上（places 31／32）。

說明書另有一句「有時需要到神殿去解除現有任務」（[`part-2.md`](manual/part-2.md)）——
`2TEMPLE.OVL` 目前解出來的四項服務（恢復狀態／恢復陣營／捐獻／離開）裡沒有這一項。
**未解**：說明書講的可能就是這瓶藥水，也可能是神殿選單裡還沒解出來的格子。

## 3. 八大職業考驗（Jurors of Mount Farview）

### 3.1 規則與獎勵

石板在 D2（地圖 34）的 (7, 0)，`EVENTSO` 段 34 腳本 12：印兩段說明
（`outdoor.34.010`／`outdoor.34.011`）之後 `0e 97` 轉派到腳本庫段 68 腳本 0。
那一段先印三段目標清單（`outdoor.68.000`–`002`），再逐一結算全隊八個位置：

```
15 0N 75 01     讀 +122 bit 0（考驗完成、獎勵未領）
11 05           沒有就換下一位
15 0N 74 80     讀 +121 bit 7（已經領過）
10 03           已經領過就換下一位
1f 0N 31 04 40 4b 4c    經驗 += 0x4C4B40 = 5,000,000
18 0N 74 7f 80          設 +121 bit 7（職稱的 "+"）
18 0N 75 fc 00          清 +122 bit 0／bit 1
```

獎勵 **5,000,000 經驗值 ＋ 職稱後面一個 `+`**，與石板上的字面（`5 million experience points`
`and recognition in the form of a '+'`）一致。石板另外要求受試者單獨前往，或只帶盜賊
（`Classes must go alone, without the rest of the party or in the company of Robbers, who must
aid at least one class to earn their mark.`）—— **這條規則就是各個考驗門口那道
`2d 0N 05`**：它問的是「隊伍裡有沒有不是這個職業、也不是盜賊的人」，有就擋。
石板的文字與閘門的語意逐字對上（見 §3.3）。等級：**已證實**。

### 3.2 八個考驗

| 職業 | 目標 | 地點 | 腳本 | 職業檢查 |
|---|---|---|---|---|
| 0 騎士 Knight | Dread Knight 恐怖武士（怪物 239）| B3（地圖 13）(5, 14) | `EVENTSO` 段 13 腳本 8 | `2d 00 05` |
| 1 聖騎士 Paladin | Frost Dragon 霜龍（怪物 148）| `EVENTSI` 段 28 (8, 8) | `EVENTSI` 段 28 腳本 3 | `2d 01 05` |
| 2 弓箭手 Archer | Baron Wilfrey 威福瑞男爵（怪物 240）| B2（地圖 10）(11, 2) | `EVENTSO` 段 10 腳本 17 | `2d 02 05` |
| 3 牧師 Cleric | 把 Corak's Soul（物品 229）帶回柯拉克的棺木 | `EVENTSI` 段 22 (8, 0) | 段 22 腳本 5 → `0e 65` → 段 66 腳本 0 | `2d 03 05` |
| 4 巫師 Sorcerer | 解救 Ybmug（惡）與 Yekop（善）兩位巫師 | `EVENTSI` 段 53／54 | 段 53／54 腳本 3–6 | `2d 04 05` |
| 5 盜賊 Robber | **沒有自己的考驗**，要陪其他職業完成 | — | — | — |
| 6 忍者 Ninja | Dawn 道恩（怪物 249）| `EVENTSI` 段 30 (8, 9) | 段 30 腳本 4 | `2d 06 05` |
| 7 野蠻人 Barbarian | Brutal Bruno 殘暴布魯諾（怪物 237）| C4（地圖 38）(0, 15) | `EVENTSO` 段 38 腳本 12 | `2d 07 05` |

職業編號的順序（騎士／聖騎士／弓箭手／牧師／巫師／盜賊／忍者／野蠻人）與生命成長表
`ds:36B4` 的八項一致。目標與座標和《軟體世界》記的逐項相同。

五個「打贏就算」的考驗都在固定遭遇後面接 `18 00 75 fe 01`（設 `+122` bit 0），
再印一句「回 Mount Farview 領獎」。牧師與巫師那兩個是多步驟的：

**牧師：** 段 22 腳本 5 先確認隊伍裡有牧師，打贏守棺的怪之後 `28 01 e5`
（拿走物品 229 Corak's Soul），成功才 `0e 65` → 段 66 腳本 0 印
`You reunite Corak's soul with his entombed body`（`indoor.66.002`）並設 `+122` bit 0。
柯拉克之魂的位置寫在 `indoor.69.006`：Lost Soul's Woods 的 (10, 15)。

**巫師：** 兩座地城各要打兩組密碼。段 53（Ybmug）的密碼是 `46` 與 `23`，段 54（Yekop）是
`64` 與 `32`（opcode `0x2f` 收字、`0x30` 比對，編碼 `明文 = 0x11A − 位元組`）。

| 條件 | 位元 |
|---|---|
| 段 53 的兩組密碼 | `+122` bit 2、bit 3 |
| 段 54 的兩組密碼 | `+122` bit 4、bit 5 |
| Ybmug 已釋放（要 bit 2 + bit 3）| `+122` bit 7 |
| Yekop 已釋放（要 bit 4 + bit 5）| `+122` bit 6 |
| 兩位都放出來 | `18 00 75 00 03` → `+122` = 0x03（bit 0 待領獎）|

台詞把「兩位都要放」寫在字面上：`"Equilibrium is essential. You must free my counterpart,
Yekop, and return to the Jurors."`（`indoor.53.043`）。

### 3.3 `2d 0N 05` 這個形式

八個考驗的職業閘門全部長成 `2d 0N 05`：第一運算元沒有任何高位旗標，第二運算元固定
`0x05`。**兩個運算元各帶一個值，任一個對上就算這個人符合** —— N 是該考驗要的職業，
5 是盜賊。閘門沒有 bit 5，所以「成立」代表隊伍裡有不符合的人，要擋下來。

第二個值固定 5 這件事有台詞佐證：`2d 03 05` 那一格（`EVENTSI` 段 22 腳本 4）印的是
`I cannot help anyone but Clerics and their Robber assistants.`，3 是牧師、5 是盜賊。
所以八個考驗都允許**盜賊隨行**。

機制與位址見 [`07-event-script.md`](formats/07-event-script.md) §`0x2d`。等級：**已證實**。

## 4. 競技賽、四位主教與女王三重冠

### 4.1 競技賽（`0e 08`）

對手與獎勵都用地圖編號當索引（`2BRAIN` `_2brain_e00` 讀的是同一個 `ds:0392`）：
`ds:4102` 每座城一格，獎金／獎章表只有三欄而把大於 2 的城夾成 2。兩張表形狀
不同是原版就有的，不是不一致。

三座城各一個入口，是自己的一格（不是酒館選單的分支）：

| 城 | 地圖 | 入口 | 腳本 |
|---|---|---|---|
| Middlegate 中門（Arena）| 0 | (13, 2) | `EVENTSI` 段 0 腳本 9 |
| Atlantium 亞特蘭汀（Colosseum）| 1 | (7, 9) | `EVENTSI` 段 1 腳本 52 |
| Sansobar 桑德索巴（Monster Bowl）| 4 | (13, 8) | `EVENTSI` 段 4 腳本 26 |

流程在 `2BRAIN.OVL`（`0e 08` → `2BRAIN +C130`），remake 那一端在
`internal/game/facility.go` 的 `EnterArena`／`ArenaEncounter`／`ArenaReward`：

- **要入場券**：掃全隊背包找物品 208–211（Green／Yellow／Red／Black Ticket），
  沒有就印 `Sorry, but you must have a ticket to compete in these games.`（`exe.4060`）。
  有就**收走那張券**，階層 ＝ 券號 − 208。
- **對手**：`((階層×3 + 城鎮基數) << 4) + rand(1, 16)`，每名隊員各一隻、全部同一種。
  城鎮基數 `ds:4102` 是 `[0, 2, 0, 0, 1]`（依地圖編號）—— 也就是中門 0、亞特蘭汀 2、桑德索巴 1。
- **獎金與獎章**：打贏之後依「階層 × 城鎮」查十二項表，獎金 200–50,000 金，
  並在 `+126`／`+127` 點一個位元。

十二個獎章位元（`data/combat.json` 的 `arenaBadgeOff`／`arenaBadgeBit`，
基底是記錄 `+0x79`）：

| 券 | 位元 |
|---|---|
| Green 綠 | `+126` bit 0／1／2 |
| Yellow 黃 | `+126` bit 3／4／5 |
| Red 紅 | `+127` bit 5／6／0 |
| Black 黑 | `+127` bit 1／2／3 |

這個對照不是從獎金表推的，是**四位主教的腳本各自驗證了一次**（見 §4.2）：
黃主教檢查 `+126 & 0x38`、紅主教檢查 `+127 & 0x61`、黑主教檢查 `+127 & 0x0E`、
綠主教檢查 `+126 & 0x07`，四個遮罩與上表四列逐位元相符。

### 4.2 四位主教

四座城堡各關著一位主教，要對應顏色的鑰匙才放得出來。放出來會給經驗，
**而且會把同色的三面競技獎章換成一筆大獎**：

| 主教 | 鑰匙 | 腳本 | 位置 | 釋放獎勵 | 三冠獎勵 | 檢查與清除 |
|---|---|---|---|---|---|---|
| Green 綠 | 111 Green Key | 段 70 腳本 3（`0e f7`，由 `EVENTSI` 段 56 (10, 6) 呼叫）| Castle Woodhaven (10, 6) | 3,000 | 10,000 | `+126 & 0x07`，清 bit 0–2 |
| Yellow 黃 | 112 Yellow Key | `EVENTSI` 段 57 腳本 24 | Castle Pinehurst (13, 3) | 5,000 | 50,000 | `+126 & 0x38`，清 bit 3–5 |
| Red 紅 | 113 Red Key | `EVENTSI` 段 55 腳本 17 | Castle Hillstone (11, 4) | 8,000 | 100,000 | `+127 & 0x61`，清 bit 0／5／6 |
| Black 黑 | 114 Black Key | `EVENTSI` 段 58 腳本 16 | Luxus Palace Royale (14, 14) | 10,000 | 200,000 | `+127 & 0x0E`，清 bit 1–3；**另外設 `+128` bit 5** |

釋放獎勵是全隊的（`1f 00 31 04 ..`），三冠獎勵是逐人判定的（每個人自己要有那三面獎章）。
沒有鑰匙就只印「主教被關在籠子裡」（`0x28` 拿不到物品 → `11 2c` 跳過整段獎勵）。

黑主教多做的那一步是後面的關鍵：它把黑券三冠換成 `+128` bit 5，而女王接見時
**認的是「還沒兌換的黑券三冠」或「黑主教蓋過章」兩者之一**。

### 4.3 女王三重冠與天命之人

Luxus Palace Royale 的入口在 D2（地圖 34）的 (14, 14)（`EVENTSO` 段 34 腳本 6，
要有城堡鑰匙 `16 00 d5`），進去是 `EVENTSI` 段 58。王座廳在 (7, 13)／(8, 13)
（段 58 腳本 18），它依 `+129` 的兩個位元決定轉派到腳本庫段 69 的哪一支：

| 條件 | 轉派 | 內容 |
|---|---|---|
| `+129` bit 4 未設 | `0e e3` → 段 69 腳本 0 | 覲見拉曼達女王 |
| bit 4 已設、bit 1 未設 | `0e e4` → 段 69 腳本 1 | 「Remember to rescue my father!」|
| 兩個都設了 | `0e e5` → 段 69 腳本 2 | King Kalohn 回到王座上 |

腳本 0 逐一檢查全隊八個位置，每個人都要：

1. `+121` bit 7 —— 職稱有 `+`（八大職業考驗通過）；
2. `+127 & 0x0E == 0x0E`（黑券三冠）**或** `+128` bit 5（黑主教已認證）。

全部通過就印 `indoor.69.000`：

> Weary Queen Lamanda perks up, "At last, the true and brave triple crown winners!
> From your party, I bequeath a Chosen One to save my father and our beloved Cron.
> See Lord Peabody to change history."

然後 `18 01 7f fe 01`（**第一位隊員** `+129` bit 0 ＝ 天命之人）與
`18 00 7f ef 10`（全隊 `+129` bit 4）。條件不足就印 `indoor.69.001`
（`If only you gained your Plus and won the Triple Crown.`）。

**天命之人這個位元有兩處讀取端**：C2（地圖 11）(10, 7) 的「Chosen Ones only」洞窟
（`EVENTSO` 段 11 腳本 2，沒有就 `An invisible force repels you.`），
以及 `EVENTSI` 段 23 腳本 16（`66 Devil Kings bow down to the Chosen One.`）。

三重冠的走法在遊戲內也有明講（`indoor.57.019`，黃主教那一格旁邊）：
`To win the Queen's Triple Crown, take a Black Ticket to the Arena, Monster Bowl,
and Colosseum.` 《軟體世界》補的順序（2 號城買三張黑票，依序打 1 號城 Arena、
5 號城 Monster Bowl、2 號城 Coloseum）與獎章表的三個城相符。

## 5. Lord Peabody 的委託與年代之門

年代之門（`0e cf`）在 Castle Pinehurst 的 (2, 5)（`EVENTSI` 段 57 腳本 19），
王座廳在 (4, 3)（段 57 腳本 27 → `0e f3` → 腳本庫段 69 腳本 16）。八個目的地與
入場條件見 [`docs/re/02`](re/02-2caves-special-events.md) §2；這裡是**旗標的來源**。

段 69 腳本 16 的流程：

| 狀態 | 動作 |
|---|---|
| `+128` bit 1 已設（已完成）| 印 `indoor.69.007`「Hi, how have you been?」|
| `+128` bit 0 已設（進行中）| 直接跳到 Sherman 檢查 |
| 隊伍沒有 Crusader（`32 04`）| 印 `indoor.69.009`「Lord Peabody needs the help of a Crusader.」|
| 尚未接受 | 印 `indoor.69.011`「Lord Peabody needs some help. Will you offer your services (y/n)?」；答應則 `18 00 7e fe 01` 設 **bit 0**，並印 `indoor.69.010`「Please return to me with my boy, Sherman, and I will reward you.」|
| 已接受、Sherman 不在隊上 | 印 `indoor.69.012`「Don't come back without Sherman!」|
| Sherman 在隊上 | 印 `indoor.69.013`「Thank you very, very much. ... you may use the time machine in the corner.」；`18 00 7e fc 02` 清 bit 0、設 **bit 1** |

**驗收條件是「名冊索引 40 的人在隊伍裡」**：八個隊伍位置各做一次
`15 0N 00 00`（選擇器 `0x00` 走 `sub_1B0B2`，回傳「這個記錄是名冊的第幾筆」）
再配 `1b 28`／`1b 29` 兩道門檻夾出 `== 40`。也就是 Sherman 是**雇來的**，不是任務道具。
線索在 `indoor.32.010`：`Mighty Nakazawa and Lord Peabody's servant Sherman were last seen
having some problems with Amazons near Native's Cove at 10,1.`

**`+128` bit 1 才是年代之門的鑰匙**（`2CAVES` 的入場檢查讀的就是記錄 `+128` bit 1），
bit 0 只表示「接了但還沒帶人回來」。拒絕進門時印的
`If you wish to use the wayback machine see Lord Peabody.`（`exe.35EE` 一帶）與這條線對得上。

等級：**已證實**（分派碼、腳本、拒絕訊息、名冊檢查四者互相印證）。
**修正**：`docs/re/02` §2 把這段寫成「地圖 42 的事件（`EVENTSI` 段 42 腳本 16）」，
並把 `18 00 7E FC 02` 說成接受任務時的寫入。實際上 `EVENTSI` 沒有段 42（段號只有
0–4、17–32、45–59、60–70），這段腳本在**腳本庫段 69 腳本 16**；接受時寫的是
`18 00 7e fe 01`（bit 0），`fc 02`（bit 1）是帶回 Sherman 之後才寫。

### 世紀的內部值比文本少 1

年代之門選項 5–8 會把世紀寫成 5–8（`ds:03CA`），而腳本用 `0x22` 檢查同一個變數。
兩處事件把對應關係釘死了：

| 事件 | 檢查 | 遊戲文本 |
|---|---|---|
| Spaz Twit（A1 地圖 5）| `22 06 06` | `travel to the 7th century A1 11,3`（`indoor.19.016`）|
| The Long One（E2 地圖 35 腳本 10）| `22 07 07` | `In the eighth century, a man known simply as The Long One`（`outdoor.64.003`）|
| King Kalohn（C4 地圖 38 腳本 13）| `22 08 08` | `battled in the 9th century in C4 at 14,5`（`indoor.26.016`）|

也就是**內部值 N 對到文本的第 N+1 世紀**。年代之門的選項 8（地圖 38、(14, 4)、世紀 8）
正好落在 King Kalohn 與 Mega Dragon 決戰的那一格旁邊。等級：**強推論**
（三組獨立事件一致，尚未用 DOSBox 逐格對過畫面）。

## 6. Lord Haart 的家族遺物

Haart Hold 在 B1（地圖 6）的 (5, 5)，`EVENTSO` 段 6 腳本 11。接見要答應
`Request an audience with Lord Haart (y/n)?`（`outdoor.06.014`）。

| 階段 | 動作 |
|---|---|
| 尚未接受 | `0e 56` → 腳本庫段 64 腳本 0：印 `outdoor.64.001` 問「do his house a favor (y/n)?」，答應則印兩段遺物說明（`outdoor.64.003`／`004`）並 `18 00 7e ef 10` 設 `+128` bit 4 |
| 進行中、東西沒齊 | 印 `outdoor.06.015`「I implore you, retrieve the phaser and loincloth from the 7th and 8th centuries!」|
| 兩件都在 | 印 `outdoor.06.016`：250,000 經驗，另外 `0x2a` 擺下 50,000 金 ＋ 500 寶石待領；清 `+123` bit 7 與 `+128` bit 4，收走腰布 |

兩件遺物都要靠時間旅行：

| 遺物 | 編號 | 取得 | 位置 |
|---|---|---|---|
| Phaser 相位槍 | 200 | 打敗 Spaz Twit（怪物 234，`EVENTSO` 段 5 腳本 13，世紀值 6）；獎賞用 `0x2a` 發，並設 `+123` bit 7 | A1（地圖 5）(11, 3) |
| +7 Loincloth +7 腰布 | 225 | 打敗 The Long One（怪物 233，`EVENTSO` 段 35 腳本 10，世紀值 7）後 `19 01 e1 00 00` 直接給 | E2（地圖 35）(5, 4) |

驗收方式兩件不同：相位槍用 `28 01 c8`（拿走物品 200），腰布用 `16 01 e1`（只檢查有沒有）
再在最後 `28 01 e1` 收走。等級：**已證實**。

## 7. 其他委託

### Nordon 與 Nordonna（`EVENTSI` 段 60，中門一帶）

兩段連續的委託，狀態全在 `+123`：

| 步驟 | 腳本 | 內容 | 旗標 |
|---|---|---|---|
| 接受 | 段 60 腳本 1 | `The humble wizard Nordon asks, "Will you do me a service (y/n)?"` → 要找回被地精偷走的金酒杯 | 設 bit 0 |
| 交金杯 | 段 60 腳本 1 | `28 01 e0` 收走物品 224 Gold Goblet → 2,000 經驗、教一條法術（`2e 6e 80`）、`0x2a` 擺下 1,000 金待領 | 設 bit 1 |
| 找 Nordonna | 段 60 腳本 2 | 姊姊要你救出兩個兒子 Drog 與 Sir Hyron | 設 bit 2 |
| 救回兒子 | 段 60 腳本 2 | 兒子可在中門旅店雇用；另外寫兩個全域位元（`1a 00 01`／`1a 01 01`）| 設 bit 4 |

台詞裡的獎勵（`2,000 exp, the spell Eagle Eye, and if you search, 1,000 gold`）與腳本的
三個動作逐項相符 —— 字面與位元組互相印證。

### 大德魯伊（腳本庫段 66 腳本 2）

`The Elder Druid ... wonders if you would perform a service for him in exchange for a great
reward (y/n)?`（`indoor.66.011`）。要在同一個洞窟裡除掉他的叛徒弟子。

| 狀態 | 位元 |
|---|---|
| 委託進行中 | `+128` bit 3 |
| 目標已除掉 | `+128` bit 2 |
| 領獎（教法術 `2e f3 10` ＝ Divine Intervention）| 清 bit 2／3 |

### Murray 與 Dawn（腳本庫段 66 腳本 1）

Murray 的度假島被 Dawn 搞壞了，委託是「殺掉 Dawn 再回來」（`indoor.66.005`）。

| 狀態 | 位元 |
|---|---|
| 已接受 | `+129` bit 6 |
| 已領獎 | `+129` bit 7 |
| Dawn 已死 | `+125` bit 5（由忍者考驗那一格 `EVENTSI` 段 30 腳本 4 設定）|

領獎是 `0x2a` 擺下 100,000 金。**忍者考驗與 Murray 的委託共用同一個「Dawn 已死」位元**，
但兩邊的完成旗標各自獨立：段 30 腳本 4 先設 `+125` bit 5，再用 `2d 06 05` 確認隊伍裡
有忍者，才設 `+122` bit 0。

### 救人類的一次性事件

這幾條沒有「接受／驗收」兩段結構，走過去、打贏、人就自由了，形式上比較接近事件而非任務，
但都會影響後面能雇到誰：

| 事件 | 腳本 | 結果 |
|---|---|---|
| Mr. Wizard 被 Lich Lord 抓走 | `EVENTSO` 段 36 腳本 1 | 救出後可在 Hourglass Inn 雇用（`outdoor.36.004`）|
| Sir Kill 與 Jed I 被塌方困住 | `EVENTSI` 段 25 腳本 6 | 救出後可在 Tundaran Arms Inn 雇用（`indoor.25.005`）；由全域 19 擋，完成寫全域 19、20 |
| 兩名囚犯（Lord Slayer 關的）| 腳本庫段 70 腳本 2（`0e f6`）| 可在 Hourglass Inn 雇用（`indoor.70.005`）|
| Red Duke 與 Dead Eye | `EVENTSO` 段 8 腳本 9 | 可在 Hotel Four 雇用，另寫兩個全域位元 |

## 8. 主線的收尾

主線本身不用旗標串起來，是靠**物品**與 `+129` 的幾個位元。

### 8.1 四件零件換元素寶珠

`Fluxer, Radicon, Todilor and Capitor are essential to gain the Element Orb.`
（`indoor.67.020`）—— 四座城堡各藏一件，湊齊才拿得走台座上的寶珠：

| 零件 | 編號 | 位置 | 線索 |
|---|---|---|---|
| J-26 Fluxer | 251 | Castle Pinehurst（段 57）(7, 6) | `indoor.17.015` |
| M-27 Radicon | 252 | Castle Woodhaven（段 56）(2, 11) | `indoor.19.019` |
| A-1 Todilor | 253 | Luxus Palace Royale（段 58）(0, 6) | `indoor.20.014` |
| N-19 Capitor | 254 | Castle Hillstone（段 55）(3, 13) | `indoor.21.015` |

寶珠的台座在 `EVENTSI` 段 30 的 (10, 15)（腳本 5）：先問 `Do you have what it takes to
get it (y/n)?`，答應之後 `0e 73` → 腳本庫段 67 腳本 9。那一段依序檢查 251／252／253
（`0x16`）、拿走 254（`0x28`），四件都在才印 `You wrest the Orb from its pedestal`，
收走前三件並給出物品 223 Element Orb。

### 8.2 交給 King Kalohn 之後

| 步驟 | 腳本 | 條件 | 結果 |
|---|---|---|---|
| 交給 King Kalohn | `EVENTSO` 段 38 腳本 13 | 世紀值 8（第 9 世紀）＋ 隊伍持有物品 219–223（四個元素之爪 ＋ 元素寶珠），五件一次收走 | 歷史改寫、Kalohn 打贏 Mega Dragon；設 `+129` bit 1；`Return to the tenth century ... at Luxus Palace Royale` |
| 交不出來 | 同上 | — | Mega Dragon（怪物 250）當場攻擊，`If only you'd had the Elemental Orb and Talons...` |
| 回十世紀覲見 | 腳本庫段 69 腳本 2 | — | `Go to Square Lake with the password WAFE.`；設 `+129` bit 5 |
| 最後地城 | `EVENTSI` 段 23 腳本 18 | `+129` bit 5 | 沒有就印 `You shouldn't be here until you save the King in the 9th century.` 並被送走 |
| 控制室 | `2SMITH.OVL` | 打字 `WAFE` ＋ 隊伍裡有人 `+129` bit 5 | 見 [`07-event-script.md`](formats/07-event-script.md)「謎題」段 |

## 9. 委託類事件的清單怎麼再擴充

上面的清單是這樣掃出來的，重跑一次就能補：

1. 掃兩個事件檔全部 1,363 段腳本裡的 `0x15`／`0x18`，過濾**選擇器 `0x74`–`0x7f`**
   （角色記錄尾端十二個位元組），對照 `data/fields.json` 換成偏移；
2. 每個位元找出寫入端與讀取端各至少一處 —— 只有一邊的先當未解，不要替它編語意；
3. 用 `translations/strings.json` 的原文交叉確認上下文。

**不要拿「字串裡有沒有 quest／reward」當入口。** 三隻獸（Envoy of Evil）、
八大考驗的目標、三把劍的守衛，字面上都沒有這兩個字。

## 10. remake 的接入狀態

各機制在 remake 這一端接到哪裡，以 [`CONTEXT.md`](../CONTEXT.md) §1.5 為準，這份文件不複製一份。
現成的程式碼入口：`internal/game/quest.go`（兩位領主的指派、驗收與獎勵）、
`internal/game/cave.go`（年代之門、馬戲團與其餘 `2CAVES` 裝置）、
`internal/game/facility.go`（競技賽的入場、對手、獎金與獎章）、
`internal/game/world.go` 的腳本直譯器（所有靠腳本驅動的任務線）。

## 11. 未解

| 項目 | 卡在哪 |
|---|---|
| 三隻獸的旗標到底在遭遇觸發時寫入，還是在擊殺後 | 腳本上是觸發時寫；「戰鬥在腳本的哪一步開打」需要 DOSBox 對照 |
| `+125` 大部分位元的語意 | 腳本用它 48 次，目前只確認 bit 1（馬戲團）與 bit 5（Dawn 已死）|
| `+129` bit 2／bit 3 的寫入端 | 腳本裡只有讀取端；可能在 `2SMITH` 的結局流程 |
| 全域 24 個位元組（`ds:03F6` 起）的語意 | 只知道哪些腳本寫了哪一格，語意未解 |
| 說明書說的「到神殿解除任務」 | `2TEMPLE` 解出來的服務裡沒有這一項 |
