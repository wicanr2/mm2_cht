# 一般寶箱觸發 oracle 稽核

日期：2026-08-12

## 結論

目前已證實原版有三條不同的「獎賞」路徑，不能把它們合併：

1. 事件腳本 `0x2a` 寫入待領獎賞，將 `ds:0434` 設為非零。之後
   `2MISC.OVL` 的 `_2misc_e02` 直接顯示 `Treasure!` 並把內容發給隊伍，**不開四選一
   寶箱頁**。
2. 一般寶箱頁（開箱／找陷阱／偵測魔法／離開）只在 `ds:0434 == 0` 的分支出現。
   戰鬥以外是否另有探索搜尋來源、其事件座標與重訪狀態仍是**未知**。
3. 一般戰鬥勝利由 `2COMBAT.img` `sub_1A0D4` 呼叫 `sub_19BF8`，清除 `ds:0434`，
   `sub_19B88` 生成 0–3 件物品；每件 `sub_19A3C` 依 `ds:10EA/10F6` 遭遇 band
   與玩家自備 `ITEMS.DAT` 寫入 `ds:6950`、`ds:6953`、`ds:6956`。同一張 `ds:10F6`
   四列四 bytes 表也供遭遇怪物與物品抽樣，`ds:10F2` 四 bytes 供物品充能；每隻
   怪死亡／逃走的 `sub_188FC` 另累加金幣／寶石。

remake 的 `ClaimReward` 保留第一條「待領獎賞 → 自動領取」路徑；一般戰鬥勝利則由
`Encounter.VictoryChestFromItems` 建立 `Chest`，UI 在 `fightRound` 結束後自動開四選單。
競技賽仍由 `ArenaReward` 處理；`World.Reward` 不會冒充一般寶箱。

## 已證實的靜態證據

### 原版 overlay

`2MISC.OVL` 的 `_2misc_e02` 先讀 `ds:0434`：

- 非零：顯示 `Treasure!`（`ds:2A2B`），把 `ds:6950`、`ds:6953`、`ds:6956`、
  `ds:695A`、`ds:695C` 的物品／金幣／寶石發給隊伍；`sub_1C64A` 的消費端確認
  `ds:695A`（word）是 Gems、`ds:695C/E`（dword）是 Gold。
- 零：進入 `ds:2A36` 的四選單，即一般寶箱頁。

事件腳本 `0x2a`（`2PLAY.OVL` `sub_1A1A0`，長度 15）寫入 3 bytes 金幣、2 bytes
寶石、三件各 3 bytes 的物品，最後設定 `ds:0434 = 0xff`。這證實它是待領獎賞，
不是普通箱子的產生器。既有格式文件也記錄：腳本事件只會把獎賞「擺好」，真正的
發放由另一處看到 `ds:0434` 的程式完成。

目前沒有足夠證據把任何 `EVENTSI.DAT`／`EVENTSO.DAT` 格子指派為一般寶箱。事件格的
正常觸發模型是：隊伍移入 `MAP.DAT` 屬性層的 `AttrHasEvent` 格，依段號與腳本索引
執行事件；但尚未找到一段事件在執行後設定「寶箱五組陣列且 `ds:0434 = 0`」的來源。

### 檔案與證據識別

本次查閱使用 `mm2-go:latest`（Debian 12 容器，容器內沒有可用的 `go` 命令；以
`grep`／既有版控文件和原始檔作唯讀查閱）。原始檔雜湊如下：

| 輸入 | SHA-256 |
|---|---|
| `MM2.EXE` | `631facb658a39e0d438c451f8a43c9f6e2aeb774fc3843c1a9bac1e14bf8c4d4` |
| `2PLAY.OVL` | `7078f30f87f9f25f8c296dc9207d70957c785b0ed83ad763e20e4012a82d2202` |
| `2MISC.OVL` | `c8291896a6db9b34564f44ea904f647c03ef2d5d18d09ee13ea52152a32e8b9f` |
| `EVENTSI.DAT` | `bcdef5461a55ee5c2232291f067368e4ce984510dfea9fd2bc4ba4df3ac53ca3` |
| `MAP.DAT` | `78bb61a3940e46f879664068b407a8f366f136a33199d931028805f1adac34d9` |

既有反組譯文件以 IDA Pro 9.4 的 `2PLAY.img` 位址空間記錄事件函式；本次另以
版控的 `workplace/ida/2COMBAT.img.i64` 與 `workplace/ida/out/2COMBAT.asm` 核對。
輸入 `2COMBAT.OVL` SHA-256 為 `3832363354105accabaa18052c18d022662a222e414eece20a2e3a410823e173`，
`MM2.EXE` SHA-256 為 `631facb658a39e0d438c451f8a43c9f6e2aeb774fc3843c1a9bac1e14bf8c4d4`。
位址是 IDA composite linear `2COMBAT.img`；`sub_1A0D4` 呼叫 `sub_19BF8` 約在
linear `0x1A28B`。分支、掉落欄位與勝利呼叫鏈屬於**已證實**；remake 尚未宣稱
逐 seed 的完整數值 parity。

## Remake 目前接線

已接上的待領獎賞路徑：

```text
正常移動 → World.RunScript → opcode 0x2a → World.Reward.Pending
  → game.Session.finishEvent → ClaimReward → Treasure!／直接發放
```

已接上的一般戰鬥路徑：

```text
正常移動 → 戰鬥 → `Encounter.Fight` 勝利 → `VictoryChestFromItems`
  → `ui.Session.Chest` → 自動 `chestMenu` → `chestDo`
```

`internal/ui/session_test.go:1467` 的 `TestChestPage` 直接設定 `s.Chest` 後按
`KeyChest`，只證明 UI 與規則可用，不能證明玩家能從正常地圖事件抵達。

## 動態重播與下一步

探索中「搜尋（S）」建立寶箱的完整座標／重訪語意仍未知；本輪不以直接注入冒充該
探索來源。戰鬥勝利箱子只在玩家勝利、非競技賽、非事件 `0x2a` 分支建立：

- 哪一張地圖、哪一個 `(x,y)` 事件格會建立一般寶箱；
- 觸發是踩格、搜尋物件，或另一個 overlay 的按鍵分支；
- 原版玩家在觸發後要按哪一個鍵進入四選單，以及離開／重訪／存檔後的狀態；
- 寶箱內容五組陣列的完整寫入來源與是否會消耗事件格。

戰鬥無戰利品時回到訊息模式；開箱／離開後清除持久 UI 狀態。`ClaimReward` 不改成
一般箱子事件的替代品。
