# 門成功後狀態 oracle（2026-08-12）

## 結論

本輪**沒有取得一扇門成功後的 oracle**。正常玩家路徑可穩定重播到中門第一人稱走廊，但目前自動化在起始隊伍位置按 `B`／`D` 時仍停留在選單或隊伍解散流程，未能證明撞門成功。因此「成功後立即前進、轉身、離圖再進、存檔重載」仍是 **unknown**，沒有把 remake 的門狀態寫成已證實。

已證實的既有資料仍可作前置證據：`2MISC.img` 的 `0xC130` 附近是門判定，撞門 handler 的成功跳轉在 `0xC19C`；`2MISC.img` 的 `0xC2B2` 是開鎖流程。這些定位與公式來自 [`docs/formats/06-map.md`](../formats/06-map.md)，本輪未重新開啟 IDA 資料庫。

## 輸入與工具雜湊

| 項目 | SHA-256／版本 |
|---|---|
| `workplace/orig/MM2/MM2.EXE` | `631facb658a39e0d438c451f8a43c9f6e2aeb774fc3843c1a9bac1e14bf8c4d4` |
| `workplace/orig/MM2/2MISC.OVL` | `c8291896a6db9b34564f44ea904f647c03ef2d5d18d09ee13ea52152a32e8b9f` |
| `workplace/orig/MM2/2PLAY.OVL` | `7078f30f87f9f25f8c296dc9207d70957c785b0ed83ad763e20e4012a82d2202` |
| `workplace/ida/2MISC.img.i64` | `2991a7470528c7ba6b4d69ed464385dbe7f47c9143c7ffc21bf146fb166756b6` |
| `workplace/ida/2PLAY.img.i64` | `41b5d84cf52e4d7aed989029217d7bc23c8642b4d63257008ec01700fb3232f17` |
| DOSBox image | `mm2-dosbox:latest`；本輪以既有映像執行，映像 digest 未另取 |
| IDA | 指定 `ida-pro-9.4-ver3:latest`；headless `idat` 報 license 無效，故本輪沒有新增 IDA 交叉參照 |
| 位址空間 | `2MISC.img` 的 image offset；既有文件亦以 `2PLAY.img` image offset 標示 |

## 正常玩家路線與結果

原版從乾淨自備資料啟動，未使用 debug hook：

```text
Return → S → G → Z
```

這條路線穩定得到中門第一人稱畫面（隊伍：YOOHOO、SQUATH、GOLDBUM、BUDDY、HOLTMAN、GAM）。起始畫面截圖 `door-north.png` 顯示朝北走廊；每個按鍵間隔至少一秒，避免既有文件記錄的 DOSBox 掉鍵問題。

之後嘗試：

1. 在起始走廊按 `U`，畫面沒有進入開鎖選人流程；這表示該位置不是可操作門，不能把結果當開鎖失敗。
2. 在起始走廊按 `B`，畫面沒有產生門成功／失敗訊息；這不是有效門狀態證據。
3. 另以 `D`（remake 的 README 所列撞門鍵）測試，原版畫面進入
   `Dismiss whom (1-6)?`。這證明目前 remake 為避開 `B` 商店鍵而採用的 `D`
   並不是原版撞門鍵；原版指令表仍是 `B` 撞門、`U` 開鎖。
4. 走兩步北、轉身後按 `B`，仍只得到一般第一人稱畫面；未看到 `Success!` 或門狀態轉換。

本輪產物（皆在 ignored 的 `workplace/dosbox/shots/`，不是公開素材）：

| 截圖 | 雜湊 | 觀察 | 等級 |
|---|---|---|---|
| `door-north.png` | `d657f226659f26bf1c294674616468b40a8715866bc074b8656608f514c17a01` | 正常路徑抵達中門走廊 | 已證實 |
| `door-reverse-bash.png` | `6250d8c048f47c580b78b0c185f1c6ca64f717eb21dc943094af74fe20545120` | 轉身後仍是第一人稱，未見成功訊息 | 已證實（負結果） |
| `door-reverse-n.png` | `e4eea6f06a28a44fd4711a6a311ed974f4b4cd8487715cd6333fc2946b5d78ff` | `N` 被後續流程當作隊伍選擇／輸入，不能解讀為門回答 | 已證實 |
| `door-bash-now.png` | `a600797a18e24f52a9afe3a2c4f5e1ce52760ed069afe4515b6f7ceb1b6f6ae4` | `B` 後仍停在原畫面 | 已證實（負結果） |
| `door-bash-n.png` | `a0677b06b697e306e88aefca66224c5b139630ba3db84a63efb614bf65007dff` | `N` 後進入解雇隊員提示 | 已證實 |

## 既有 IDA 與格式證據

- `docs/formats/06-map.md` 將 `2MISC.img:0xC130` 標為門入口判定，門種類為地形層每方向兩位元值 `2`。
- 同文件將撞門成功條件定位到 `0xC19C`，並記錄隊伍力量、`rand(10,109)/10` 與地圖屬性 `ds:5998`。
- 開鎖 handler 定位為 `2MISC.img:0xC2B2`，失敗後依 `ds:5999` 觸發陷阱。
- 以上是既有研究的**已證實／強推論分級結論**；本輪 IDA 9.4 headless 因 license 失敗，沒有把它們升級或改寫。

## 未解項與下一個最小動作

- **unknown：** 哪一面中門門牆可由乾淨起始隊伍正常抵達，以及原版按鍵如何讓撞門／開鎖 handler 被觸發。
- **unknown：** 成功後門在地圖資料、當前格、視圖與相鄰格如何變化。
- **unknown：** 成功後離圖再進、存檔重載是否保留開門狀態；本輪沒有成功門，故沒有進行存檔測試，也不猜 save schema。
- 下一輪應先取得一個**可控但仍是正常玩家路徑**的門：從地圖 0 已解析門座標（`(4,1) N?`、`(5,1)`、`(6,1)`、`(10,3)`、`(10,4)`、`(13,5)`、`(14,6)`）挑一面，使用原版實際 action menu／方向路線逐步截圖；不可把 `D` 當撞門鍵，也不可用座標注入、強制勝利或 debug 旗標代替正常路徑。

本輪沒有修改正式程式、沒有 commit／push；只新增本研究紀錄。
