# DOS《魔法門 II》施法互動提示 oracle（研究筆記）

日期：2026-08-12。這是施法 UI 垂直鏈的窄研究，不是 remake 實作規格。
以下保留原始函式名、映像偏移與證據等級；沒有把推測性的名稱寫回 IDA。

## 輸入與可追溯性

| 輸入 | SHA-256 | 位址／工具基準 |
|---|---|---|
| `workplace/orig/MM2/2CAST1.OVL` | `36a1e4e0dafe4d9532cdcf111161cc523464f6eee4c26f6f9806eac841496309` | DOS overlay；IDA Pro 9.4 匯出 |
| `workplace/orig/MM2/2CAST2.OVL` | `bfd9018f62b0c4f01d312b86265625325335605cfd069216c27f9b799a274fd4` | DOS overlay；IDA Pro 9.4 匯出 |
| `workplace/orig/MM2/MM2.EXE` | `631facb658a39e0d438c451f8a43c9f6e2aeb774fc3843c1a9bac1e14bf8c4d4` | DOS executable；root 段地址空間 |
| `workplace/orig/MM2/SPELLS.DAT` | `60b95d1c5fdf856c6c3e9b7081091839efa3717e2d6f3214e99efdcf813d61c0` | `ds:7D60` 載入區；`SPELLS.DAT` 索引 |
| `workplace/ida/out/2CAST1.asm` | `875056693136a55d8110d0081ebb57a50fef34697dd9a00e067b0f350495ef1e` | IDA Pro 9.4 匯出；IDA composite image 線性位址（Base Address `1000h`） |
| `workplace/ida/out/2CAST2.asm` | `8912396ac942273844aa89d4be501b57b82fd384a21a96d2f8d4aa43124509da` | IDA Pro 9.4 匯出；IDA composite image 線性位址（Base Address `1000h`） |
| `data/spells.json` | `05989ea63abd3374f710f14b5a82d577c9c880996b0caf9f2dc657af3f5bcec1` | remake 資料順序；非原版位址證據 |

既有匯出中的 `2CAST1.asm`／`2CAST2.asm` 沒有可安全重命名的函式；本文沿用
`sub_…`、`loc_…` 與 `byte_…` 原名。`loc_16EC2`、`sub_1D046` 等名稱只是 IDA
輸出定位，不是語意定案。

## 96 條法術的提示型態分群

下表先依 `data/spells.json` 的 `Target`／`Form` 做集合盤點，再以 overlay
呼叫點標記目前能否證明「會停下來等輸入」。同一法術可能同時有隊員或怪物目標，
因此「目標集合」不等於已證實的 UI 控制器。

### A. 隊員目標（Target：1 名／1 個隊員）

急救術、強效治療、治傷術、勇氣術、解毒術、治病術、回復陣營、狂暴術、恢復術、
回春術、解除石化、復活、復活。這是第一個應以 normal path 重播的 `Target`
群：預期需要「挑一名隊員」；目前 remake 的 `pickers` 結構可承載它，但法術選單
未接入。**證據：強推論**（手冊資料為目標描述；本次尚未取得 DOSBox 畫面／按鍵
記錄證明每一支都呼叫同一個挑人迴圈）。

### B. 物品目標（Target：施法者背包中之物品）

只有魔法偵測術。它是唯一明確的 `Item` 群，不能用隊員選單代替。**證據：已證實
目標集合，提示按鍵與取消尚未由 normal-path 記錄證實**。現有 `Session` 有物品
索引，但 `menuSpell` 選中後直接 `Cast`。

### C. 戰鬥怪物目標（Target：1／2／3／4／5／6／10／全部怪物）

幻影術、驅魔術、傷痛術、沈默術、衰弱術、冷凍光線、定身術、噴酸術、狂風陣、
致命蟲群術、麻痺術、洪水陣、后土陣、火焰枷鎖、月光術、烈火陣、扭曲重力術、
神聖之咒、能量爆破術、火箭術、催眠術、閃電箭、識別術、酸液、電擊術、冷凍射線、
衰弱心智術、火球術、防護罩、分裂術、死亡之指、砂暴術、時間扭曲、冷凍術、
超級電擊、飛劍術、奇異之光術、焚化術、高壓電擊術、隕石雨、魔法黑洞、地獄之火、
星爆術、陷敵術。`全部`／多隻數量代表法術規則的選擇範圍，不足以證明玩家要逐隻
輸入；可能是戰鬥欄位（column）或前排自動選取。**證據：目標集合已證實；提示
型態未知**。這群必須以一場至少有多個前排／後排怪物的原版戰鬥重播，不能拿
`internal/game` 的 `Reachable` 或單元測試代替 oracle。

### D. 全隊／施法者自動目標（不應新增目標提示）

喚醒術（兩筆）、祝福術、照明術（兩筆）、自然之門、拒絕傷害、製造食物、持續照明術、
水行術、四個界傳送術、傳送到地面、神聖賜與、城市傳送術、神之干涉、定位術、跳躍術、
飄浮術、魯易浮標、魔法防護術、飛行術、隱身術、巫師眼、守衛術、庇護術、傳送術、
穿透術、能量補充術、複製術、加強法力、去咒術。大多數應在法術確認後直接執行；
但「城市傳送術」及「傳送術」是另列的按鍵輸入例外（見 E）。**證據：強推論**，
因資料目標明確、但仍需 normal path 驗證取消與扣費時點。

### E. 數字／城市選擇（Choice）

| 法術 | 原始函式／位址基準 | 靜態組語可證實的提示順序與取消 |
|---|---|---|
| 傳送術（1–9 格） | `sub_1C590`；IDA composite image 線性位址 `0x1C590`；level-2 overlay 檔案 offset `0xCD90`（`0x1C590 - 0xF800`）。共用提示 `sub_1D046` 同一位址基準。 | 先呼叫 `sub_1D046`；之後由 `loc_16EC2` 收 `1`–`9`。`0x1B` 走取消分支；每一步才更新座標。**已證實（靜態組語；非 DOSBox replay）** |
| 城市傳送術（1–5 城） | `sub_1CA20`；IDA composite image 線性位址 `0x1CA20`；level-2 overlay 檔案 offset `0xD220`（`0x1CA20 - 0xF800`）。 | 先呼叫 `sub_1D046`；之後由 `loc_16EC2` 收 `1`–`5`，`0x1B` 取消；確認後才寫 `ds:0392` 並把座標設 `0xFF` 交 root 取 `ATTRIB +14`。**已證實（靜態組語；非 DOSBox replay）** |

`sub_1D046`（IDA composite image 線性位址 `0x1D046`；level-2 overlay 檔案 offset
`0xD846`）本身顯示 `ds:315E` 的提示，呼叫 `loc_16EC2`，以
Enter（`0x0D`）結束提示；若當前格 `AttrNoMagic`，會回到 `sub_1CEFA`。這證明
共同提示的順序是「法術 handler → `sub_1D046` 的提示／Enter → 法術專用輸入」；
這是 IDA 靜態控制流程證據，尚未證明玩家畫面上的中文逐字內容，也不是一次
DOSBox normal-path 重播。

`loc_16EC2` 必須另行看待：IDA 在 `2CAST1.asm` 與 `2CAST2.asm` 都各輸出
`loc_16EC2`（各自是 composite image 線性位址 `0x16EC2`），但目前沒有足夠的
segment／呼叫端證據把它命名成 root routine、共用 thunk 或某一個 overlay 的檔案
偏移。因此本文只以「各匯出檔內的原始 IDA 線性定位」引用它，不把它寫成
`2CAST1` 的 overlay offset，也不把兩份 routine 合併成同一個位址。其輸入範圍
與 `0x1B` 取消是由各自 overlay 的靜態 call site／區間檢查推得，證據等級為
**強推論**；要升為已證實，仍需 IDA database 的 segment 關係或 DOSBox trace。

### F. 步數／欄位仍不可定案

鷹眼術的資料寫「施法者等級×5 步／等級」，但本次沒有證據顯示玩家輸入步數；
多目標戰鬥法術的「欄位（column）」也沒有從現有匯出安全推出。這兩者先標為
**未知**，不能直接把 `Target` 數字轉成 UI 欄位或逐隻選擇。

## 目前可交給實作者的最小 oracle gate

本輪能以原始位址確定的共同骨架是（全部為靜態組語，不宣稱 normal-path replay）：

1. `2CAST1`／`2CAST2` 的 handler 先呼叫 `sub_1D046`（IDA 線性 `0x1D046`；
   level-2 overlay offset `0xD846`）顯示提示並等
   Enter；取消或禁魔格會在正式效果前返回。
2. 需要數字的兩條已明確呼叫 `loc_16EC2`：傳送術 `1`–`9`、城市傳送術 `1`–`5`；
   `0x1B` 是取消，不應扣除後才取消。
3. **扣 SP／寶石時點：本次組語摘錄尚未完成從 root 施法分派到 handler 的資料流
   追蹤，故不得宣稱是「進入選單即扣」或「確認後才扣」。** 這是本 mentor 任務
   下一個最小 DOSBox／IDA gate；目前只可將 remake 的直接 `Cast` 視為缺少證據，
   不能以它反推原版。

## 後續最小重播矩陣（尚未完成，非產品決策）

| 群 | 一條必要重播 | 必記錄 |
|---|---|---|
| 隊員目標 | 急救術或治傷術 | 進入提示、隊員選單、Esc、確認後 SP／寶石前後值 |
| 物品目標 | 魔法偵測術 | 物品清單、Esc、確認後 SP／寶石前後值 |
| 怪物目標 | 單目標與多目標各一條 | 前後排／欄位顯示、Esc、確認後 SP／寶石前後值 |
| Choice | 傳送術與城市傳送術各一條 | `1`／邊界值／Esc、扣費時點與落點 |

本文件目前是證據索引，不授權自行改變中文提示排版、save schema 或多目標選擇
規則。

## Remake 已接入的最小 UI gate（2026-08-12）

`internal/game.SpellPromptFor` 只把目前 `spellEffects` 確實讀取的輸入接到正常
施法選單：隊員索引、施法者背包槽位、傳送／城市／魯易浮標的數字，以及飛行術的
欄位與列。`internal/ui` 在提示子選單確認前不呼叫 `Cast`；`Esc` 只回到法術清單，
因此不扣 SP／寶石。提示欄位是暫存狀態，不寫入存檔；選單中的存檔鍵也不會改變
遊戲狀態。

這不是把 96 條法術的 `Target` 文字自動轉成輸入規則：尚未證實的怪物目標、欄位
選擇與其他原版提示仍維持自動效果或 fail-closed，等待對應的 DOSBox normal-path
oracle。`TestTargetedSpellPromptCanCancelBeforeCost` 與
`TestSpellItemAndChoicePromptsCanCancel` 是目前 UI 鍵路徑證據。
