# 一般寶箱觸發 oracle 稽核

日期：2026-08-12

## 結論

目前只能證實原版有兩條不同的「獎賞」路徑，不能把它們合併成一般寶箱：

1. 事件腳本 `0x2a` 寫入待領獎賞，將 `ds:0434` 設為非零。之後
   `2MISC.OVL` 的 `_2misc_e02` 直接顯示 `Treasure!` 並把內容發給隊伍，**不開四選一
   寶箱頁**。
2. 一般寶箱頁（開箱／找陷阱／偵測魔法／離開）只在 `ds:0434 == 0` 的分支出現。填入
   寶箱五組內容陣列、設定這條分支的程式碼尚未定位，因此其正常事件、座標及按鍵仍是
   **未知**。

所以目前 remake 的 `ClaimReward` 已接好第一條「待領獎賞 → 自動領取」路徑，但不能算作
「事件 → 一般寶箱 UI」完成。`ui.Session.Chest` 只有直接注入／測試入口；`KeyChest` 在
`Chest == nil` 時不做任何事。`game.Session.Step` 回傳的是移動與遭遇，沒有寶箱訊號；
`World.Reward` 是另一種 typed 狀態，不能直接冒充一般寶箱。

## 已證實的靜態證據

### 原版 overlay

`2MISC.OVL` 的 `_2misc_e02` 先讀 `ds:0434`：

- 非零：顯示 `Treasure!`（`ds:2A2B`），把 `ds:6950`、`ds:6953`、`ds:6956`、
  `ds:695A`、`ds:695C` 的物品／金幣／寶石發給隊伍。
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

既有反組譯文件以 IDA Pro 9.4 的 `2PLAY.img` 位址空間記錄相關函式（例如
`sub_1A1A0`、`sub_1A8C4`）；本次沒有可用的 IDA image／`.i64`，沒有新增或改名
任何反組譯符號，也沒有把本次文字查閱冒充新的 IDA 交叉參照。上述分支判定屬於
**已證實（沿用既有 IDA 證據）**；一般寶箱來源與觸發點屬於**未知**。

## Remake 目前接線

已接上的待領獎賞路徑：

```text
正常移動 → World.RunScript → opcode 0x2a → World.Reward.Pending
  → game.Session.finishEvent → ClaimReward → Treasure!／直接發放
```

尚未接上的一般寶箱路徑：

```text
正常移動 → 事件或其他原版來源（未知） → game.Session 的 Chest typed signal（不存在）
  → ui.Session.Chest → KeyChest → chestMenu → chestDo
```

`internal/ui/session_test.go:1467` 的 `TestChestPage` 直接設定 `s.Chest` 後按
`KeyChest`，只證明 UI 與規則可用，不能證明玩家能從正常地圖事件抵達。

## 動態重播與下一步

便宜模型的隔離工作階段沒有可用的 `mm2-dosbox` 權限；mentor 隨後確認主機已有
`mm2-dosbox:latest`，但本輪未完成新的正常玩家重播。因此以下仍未知：

- 哪一張地圖、哪一個 `(x,y)` 事件格會建立一般寶箱；
- 觸發是踩格、戰鬥勝利後、搜尋物件，或另一個 overlay 的按鍵分支；
- 原版玩家在觸發後要按哪一個鍵進入四選單，以及離開／重訪／存檔後的狀態；
- 寶箱內容五組陣列的完整寫入來源與是否會消耗事件格。

最小後續工作是取得指定 DOSBox／IDA 工具鏈後，從正常新局建立一條可重播路徑：
先掃描「踩事件格後進入 `_2misc_e02` 四選單」的畫面，再記錄地圖、座標、面向、按鍵、
內容與存檔前後狀態；若找不到，才以 `2MISC.OVL` 對 `ds:0434` 的所有寫入端做 IDA
交叉參照追查。未取得這項證據前，不應把 `ChestFromReward` 或 `ClaimReward` 改成
一般箱子事件的替代品。
