# 反組譯產物與筆記

這個目錄放反組譯的**函式層**成果。檔案格式的規格在 [`../formats/`](../formats/)，
行為 oracle 與平台差異在 [`../research/`](../research/)，「現在做到哪」只在
[`CONTEXT.md`](../../CONTEXT.md) §1.5。

## 文件

| 檔案 | 內容 |
|---|---|
| [`00-function-index.md`](00-function-index.md) | 全部有筆記的函式符號 → 出處。**由 `tools/gen_func_index.py` 產生，不手改** |
| [`01-boot-and-display-mode.md`](01-boot-and-display-mode.md) | `1MENU1`：開機第一支 overlay，命令列的顯示模式字母、顯示卡偵測、磁片與記憶體檢查 |
| [`02-2caves-special-events.md`](02-2caves-special-events.md) | `2CAVES`：`0x0e` 分派出去的特殊裝置（傳送機、滑梯陷阱、年代之門、捐獻、馬戲團、每日笑話、兩位領主的任務）|
| [`03-character-flags.md`](03-character-flags.md) | 角色記錄 `+125`／`+129` 的每一個位元、24 個全域夥伴旗標、三隻獸旗標的寫入時機 |
| [`04-2brain-tavern.md`](04-2brain-tavern.md) | `2BRAIN` 的酒館入口：五個選單項、酒與餐點的效果表、餐點旗標、依日期輪替的傳聞 |
| [`05-2smith-control-room.md`](05-2smith-control-room.md) | `2SMITH` 的結局控制室：守門的 Sheltem 戰、`WAFE` 中止碼、替代加密的密碼題、15 分鐘計時、通關結算 |
| [`06-1retinn-roster.md`](06-1retinn-roster.md) | `1RETINN`：旅店登記、名冊與隊伍編組、五座城的落點表、全滅回旅店 |
| [`07-2smith-shop.md`](07-2smith-shop.md) | `2SMITH` 的鐵匠鋪：六個模式、四類貨底、定價、買賣、鑑定的整頁欄位 |
| [`08-2cmds-inventory.md`](08-2cmds-inventory.md) | `2CMDS`：交易、使用、裝備、卸下、丟棄；物品的六個類別區間與裝備衝突規則 |
| `dgroup-tables.json` | DGROUP 初值段裡已定位的表 |

任務線的**玩法**整理在 [`../quests.md`](../quests.md)、酒館四十則傳聞的中英對照在
[`../tavern-rumors.md`](../tavern-rumors.md)，那兩份是給讀者看的章節；
這裡的八份是給下一個接手反組譯的人看的。

## overlay 覆蓋狀況

`.asm` 產物在 `workplace/ida/out/`（gitignore）。「有筆記」＝ 該函式的符號
出現在 `docs/` 或程式碼註解裡，由 `00-function-index.md` 統計。

| overlay | 內容 | 狀態 |
|---|---|---|
| `2PLAY` | 主迴圈、移動、事件直譯器、`0x0e` 分派 | 玩家路徑上的機制已解 |
| `2COMBAT` | 戰鬥 | 九個指令與傷害鏈已解；編隊與目標細節未證實 |
| `2CAVES` | 特殊裝置 | **全解**，見 `02` |
| `1MENU1` | 開機與顯示模式 | **全解**，見 `01` |
| `1MENU2` | 建角色與名冊主選單 | 建角流程已解（`docs/formats/10`）|
| `1RETINN` | 旅店／隊伍編組／雇用／全滅 | **全解**，見 `06` |
| `2MISC` | 寶箱與雜項 | 寶箱已解（`docs/formats/02`）|
| `2MISC2` | 訓練基地與設定 | 升級表已解 |
| `2CAST1`／`2CAST2` | 法術 | 96 條 handler 已解（`docs/formats/09`）|
| `2CMDS` | 交易／使用／裝備／卸下／丟棄 | **全解**，見 `08` |
| `2SMITH` | 鐵匠 ＋ 結局控制室 | **全解**：控制室見 `05`、鐵匠見 `07` |
| `2TEMPLE` | 神殿與法師公會 | 服務已解 |
| `2BRAIN` | 競技賽／大腦淨化／酒館 | **全解**：競技賽與淨化見 `docs/formats/08`，酒館見 `04` |
| root（`MM2.EXE`）| C runtime ＋ 共用常式 | 玩家路徑上的常式已解；runtime 不列缺口 |

## 位址怎麼換算

```
root：   IDA linear = 檔案偏移 + 0xF800
overlay：IDA linear = 0x10000 + 執行時偏移（載入段 × 16 ＋ overlay 內偏移）
DGROUP： EXE 檔內偏移 = DGROUP 偏移 + 0x8630
```

細節與 overlay 描述表見 [`../formats/01-overlay-and-memory-layout.md`](../formats/01-overlay-and-memory-layout.md)。
