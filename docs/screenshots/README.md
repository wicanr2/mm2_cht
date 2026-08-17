# 介面截圖

由 `cmd/mm2shots` 產生，走的是 `internal/ui` 那條與視窗無關的路徑，
所以每張都是實際遊玩時會看到的畫面，不是另外畫的示意圖：

```
go run ./cmd/mm2shots -data workplace/orig/MM2 -out docs/screenshots
```

畫面是 320×200 的原版骨架整數倍放大，文字走獨立的高解析點陣路徑
疊在上面（做法見 `CLAUDE.md` §6）：中文 24×24 全形、英數字 12×24 半形，
同一個字族。**框線的座標照原版量出來，框裡放什麼
按中文重排** —— 右上那一格從原版的 `Protection` 換成隊伍（六列，各有編號、
完整名字、職業與生命），下方大框整塊給對話。原版那個兩欄各三列的名單
一欄只有 154 px，放不下完整名字，更放不下中文職業。

| 檔案 | 內容 |
|---|---|
| `00-chinese.png` | 中文疊在原版畫面上。那段神殿招呼語是 `STR.DAT` 的實際譯文，不是示意文字 |
| `01-first-person.png` | 第一人稱視角。牆與地板來自 `TOWN.16`／`TOWNF.16`，側牆上的火炬會動 |
| `01b-first-person-modern.png` | 同一個視角換成 Scale3x（遊戲中按 `F5` 切換）。顏色與幾何完全相同，差別只在色塊邊界補了斜角 |
| `01c-first-person-amiga.png` | 換成 **Amiga 版素材**（按 `F6`）。32 色，走同一套幾何 —— 兩個平台的牆面張數與排列一一對應。火炬少了燈桿底圖，那是素材本身的差異：Amiga 每格只有三張火焰 |
| `01d-first-person-amiga-modern.png` | Amiga 素材加 Scale3x。`F5` 與 `F6` 是正交的，四種組合都成立 |
| `01f-first-person-msx.png` | 換成 **MSX2 版素材**。整套場景是一張 462×128 的素材表，每一面牆是表裡的一塊矩形，落點另有一張表 —— 原版靠 VDP 的矩形搬移組畫面，這裡直接切圖來貼。視圖只有 154×64（DOS 是 208×120），所以整幅置中。正牆是三塊拼成的一條岩帶，側牆的落點 0 → 28 → 56 與 DOS 的 `sideX` 同一個形狀|
| `01g-msx-torch.png` | MSX 的火炬。**動畫影格是 remake 產生的** —— 原版每個位置只有一張貼圖，這裡把火焰左右各位移一像素做出三張，沒有新增任何像素 |
| `01i-first-person-cave.png` | **地城的場景素材**（場景碼 1）。原版每種場景一套牆——`_2play_e10` 依 `ds:039C` 推 `town*`／`cave*`／`castle*`／`out*`，remake 依 `World.Scene()` 挑，換圖不必有人記得換 |
| `01j-first-person-castle.png` | **城堡的場景素材**（場景碼 5；場景 2 也用同一套）。灰石磚、紅旗、藍地磚 |
| `01k-msx-cave.png` | MSX 也是一種場景一張表：同一個洞窟換成 `0x2021`（綠石）。四張表的地圖區間與 DOS 的場景碼逐段相同 |
| `01h-first-person-md.png` | 換成 **Mega Drive 版素材**。視圖大小與 DOS 相同（208×120），但一整根側牆柱是一張 120 高的圖，八根寬度左右對稱加起來鋪滿 208；火炬是原版直接寫進 nametable 的 53 個 tile，重切成八張 |
| `01e-first-person-pack.png` | 烘好的高解析素材包（`cmd/mm2modern`）。與 `01b` 畫的是同一件事，差別在它是檔案 —— 之後可以整批換成重畫的美術 |
| `02-cast.png` | 施法選單。法術名、等級與說明都是譯文，說明接在清單下面 |
| `03-items.png` | 物品選單。已裝備六格加背包六格，可以裝穿脫、可以使用 |
| `04-shop.png` | 商店（鐵匠鋪的貨架）。貨色與售價來自原版的商店表 |
| `05-reference.png` | 查說明書。1988 年只印在紙本上的參考資料收進遊戲裡 |
| `06-map.png` | 地圖。五座城鎮整張看得到（手冊本來就印了），其他地圖只顯示走過的格 |
| `07-combat.png` | 戰鬥。九個指令全部可用，這裡是射擊 |
| `07a-combat-anim-00.png`／`07a-combat-anim-15.png` | 同一場戰鬥的第 0 與第 15 個 tick。怪物照原版影格表的 hold 播，不是等速輪播 —— 兩張擺在一起才看得出來 |
| `07b-target.png` | 攻擊前挑目標。列的是**這一擊打得到的那幾隻** —— 近戰只列前排、射擊列全場，倒下的不列。原版是打字母 `A`–`J`，remake 沿用自己的數字選單 |
| `07c-spell-target.png` | 戰鬥中施單體法術挑目標。**列的是場上全部**（這裡四隻，前排只有兩隻）—— 法術打得到後排，近戰不行 |
| `08-protection.png` | 戰鬥中的防護效能（指令 `P`）|
| `11-lore.png` | 說明書的手札。序言與科隆的歷史，紙本才有的世界觀 |
| `10-create.png` | 建立新角色。屬性與可選職業排兩欄，不能選的職業顯示為「－」|
| `09-chest.png` | 寶箱。箱子的名字是依內容算出來的，不是隨機挑的皮 |
| `12-worldmap.png` | 世界地圖。手冊摺頁上的地名（格線 A–E × 1–4），遊戲裡本來一個字都沒有 |
| `13-puzzles.png` | 打字謎題的答案。原版的謎底靠英文文字遊戲，翻成中文就解不開了 |
| `15-md-monster.png` | 按 `F6` 把怪物換成 Mega Drive 版（場景沿用 DOS）|
| `16-amiga-monster.png` | 再按一次換成 Amiga 版 |
| `17-intro.png` | 片頭。底圖與 13 張疊圖都來自 `MASTER.16`，位置是拿實機截圖逐像素定出來的；獨角獸的頭尾一直在動，樹叢裡那幾張臉輪流探出來 |
| `14-world-grid.png` | 世界網格（`W`）。二十張野外圖由 `ATTRIB` 的鄰接欄位排成 5×4 的環面，字母數字沿用手冊那一頁 |
| `19-settings.png` | 設定（`F2`）。**原版沒有這個畫面** —— remake 刻意與原版不同的地方（目前是事件獎賞的領取方式）在這裡切回原版行為，不由我們替玩家決定死 |

`18-control-room.png`（結局的密碼題）**不是 `cmd/mm2shots` 產的**，
它走的是 `internal/ui` 的 `TestControlRoomScreenshots` —— 那條路要打贏守門的那一場、
過中止碼才到得了。替代加密的字母表雖然每次進控制室重抽，但 `Load` 的種子固定
（`0x1234`），所以這張截圖仍然是逐位元組可重現的：

```
go test ./internal/ui -run TestControlRoomScreenshots
cp workplace/gfx/ui/control-cipher.png docs/screenshots/18-control-room.png
```

那支測試順便留下控制室六頁的每一頁（`workplace/gfx/ui/control-*.png`，不入版控）。

原版的畫面另存在 `workplace/dosbox/shots/`（不入版控），
版面座標是拿素材去那些截圖上做樣板比對定出來的，
過程與數字見 [`docs/playtest/01`](../playtest/01-oracle-timeline.md)。
