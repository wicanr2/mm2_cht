# 介面截圖

由 `cmd/mm2shots` 產生，走的是 `internal/ui` 那條與視窗無關的路徑，
所以每張都是實際遊玩時會看到的畫面，不是另外畫的示意圖：

```
go run ./cmd/mm2shots -data workplace/orig/MM2 -out docs/screenshots
```

畫面是 320×200 的原版骨架整數倍放大，中文走獨立的高解析點陣路徑
疊在上面（做法見 `CLAUDE.md` §6）。

| 檔案 | 內容 |
|---|---|
| `01-first-person.png` | 第一人稱視角。牆與地板來自 `TOWN.16`／`TOWNF.16`，側牆上的火炬會動 |
| `02-cast.png` | 施法選單。法術名、等級與說明都是譯文，說明接在清單下面 |
| `03-items.png` | 物品選單。已裝備六格加背包六格，可以裝穿脫、可以使用 |
| `04-shop.png` | 商店。貨色與售價來自原版的商店表 |
| `05-reference.png` | 查說明書。1988 年只印在紙本上的參考資料收進遊戲裡 |
| `06-map.png` | 地圖。五座城鎮整張看得到（手冊本來就印了），其他地圖只顯示走過的格 |
| `07-combat.png` | 戰鬥。九個指令全部可用，這裡是射擊 |
| `08-protection.png` | 戰鬥中的防護效能（指令 `P`）|

原版的畫面另存在 `workplace/dosbox/shots/`（不入版控），
版面座標是拿素材去那些截圖上做樣板比對定出來的，
過程與數字見 [`docs/playtest/01`](../playtest/01-oracle-timeline.md)。
