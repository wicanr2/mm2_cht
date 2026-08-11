# Might and Magic II: Gates to Another World — 繁體中文 remake

把 New World Computing 1989 年的《Might and Magic II》完整逆向，在 Go / Ebiten 上
重寫引擎，再做繁體中文化。定位是文化資產保存。

## 現況

**可以玩了。** 建角色、走路、事件、戰鬥、施法、商店、旅店、神殿、訓練、
寶箱、存檔都接上了；翻譯 100% 完成。

![中文疊在原版畫面上](docs/screenshots/00-chinese.png)

框線照原版 —— 四個紅框圍出第一人稱視圖、右上那一格、`Day／Year／Face`
那條橫列與下方大框，座標是拿原版素材去 DOSBox 實機截圖上做樣板比對量出來的。
**框裡放什麼則按中文重新分配**：原版把隊伍擠在下面兩欄各三列，一欄 154 px
放不下完整名字，更放不下中文職業；這裡把隊伍移到右邊那一格（六列，各有
編號、完整名字、職業與生命），下方整塊留給對話。戰鬥中也看得到隊伍狀態，
原版看不到。

**文字走獨立的高解析點陣層疊在原版像素上**，原版的 320×200 骨架一個像素
都沒改。中文 24×24 全形、英數字 12×24 半形，同一個字族（Noto Sans CJK ／
Noto Sans Mono）—— 英文如果留在原版那套 8×8 放大三倍，方塊像素會比中文粗一截。
兩層疊在同一張畫布再整張送出去，所以拉動視窗時等比例一起縮放，
文字不會相對原版像素跑掉。上圖那段神殿招呼語是 `STR.DAT` 的實際譯文。

更多畫面見 [`docs/screenshots/`](docs/screenshots/)。

| 領域 | 狀態 |
|---|---|
| 中文化 | **2,695／2,695（100%）** |
| overlay 與記憶體佈局 | 14 個 overlay 反組譯完成，599 個函式 |
| 檔案格式 | 壓縮、圖形、字型、文字、地圖、事件、物品、怪物、法術全部解開 |
| 事件腳本 | 用到的 49 個 opcode 全部有語意也全部有分支，71 個非空段全部解得開 |
| 戰鬥 | 九個指令全部接上；命中、傷害、行動順序、逃走、狀態、特殊攻擊照原版 |
| 法術 | 96／96 條有效果 |
| 前端 | Ebiten 視窗；互動邏輯與視窗系統無關，headless 也跑得起來 |
| 原版 oracle | DOSBox headless，一鍵跑到第一人稱視角 |

還沒做：`TOWNT.16` 深度 2 的火炬位置、`ds:74D` 與 `ds:4D84` 兩張防護等級表為什麼不一致。

## 玩

```
go run ./cmd/mm2 -data <你的 MM2 目錄>
```

視窗可以任意拉大縮小，畫面等比例縮放並置中。

↑ 前進、↓ 後退、← → 轉向、Enter 推進、Y／N 回答、R 休息受訓、C 施法、
I 物品、B 商店、K 查說明書、M／F3 地圖、G 開箱、N 建角色、D 撞門、U 開鎖、S 存檔；
戰鬥中 Enter 攻擊、T 射擊、C 施法、A 抵擋、F 溜跑、P 防護、V 檢視、X 對調。

`F5` 在兩種牆面畫法之間切換：原版像素（nearest 整數倍放大）與 Scale3x
（色塊邊界補斜角）。兩種畫的是同一批原版素材、同一組顏色，差別只在放大方式 ——
中文走 24×24 點陣，與 8×8 放大三倍的牆面像素密度差三倍，Scale3x 讓兩者接近。

`F6` 換素材來源：DOS → Amiga → 高解析素材包。三套走**同一套幾何**，
換的只是圖 —— `town.32` 與 `TOWN.16` 都是 32 張、同樣的深度與側牆順序，
所以 Amiga 版的牆可以直接貼進原本的透視裡。真正的差別只有兩個：
透空色（DOS 是色號 8、Amiga 是 0）與火炬的張數（DOS 每格四張含燈桿底圖，
Amiga 只有三張火焰）。Amiga 素材要放在 `workplace/amiga/`（自備原版磁片，
用 `tools/adf.py` 抽），沒有就只有 DOS 可選。

高解析素材包用 `cmd/mm2modern` 烘：

```
go run ./cmd/mm2modern -data <你的 MM2 目錄>          # → workplace/modern
go run ./cmd/mm2modern -amiga workplace/amiga -out workplace/modern-amiga
```

烘出來的是一疊 PNG 加一份 `set.json`。與 `F5` 的 Scale3x 畫的是同一件事，
差別在**它是檔案，可以被換掉** —— Scale3x 的上限就是原版像素的資訊量，
真正重畫的美術需要一個「檔案放這裡、尺寸與命名照這個規矩」的地方。
遊戲先找 `assets/modern`（重畫的原創美術），再找 `workplace/modern`
（自己從原版烘的，不進版控）。

## 各版本素材總覽

DOS 之外還有三個平台。**四套素材是各自逆向出來的，容器一個都不一樣**，
彼此沒有共用的格式；共通的只有美術本身 —— 同一批畫，各自重新上色與重排。

| 平台 | 發行 | 容器 | 顏色 | 破解點 |
|---|---|---|---|---|
| DOS | 1988，New World Computing | `.16`：LZW 段內含影像目錄 | EGA 16 色 | LZW 變體，見 [`docs/formats/03`](docs/formats/03-lzw-compression.md) |
| Amiga | 1989 | `.32`：目錄 + 32 色盤 + nibble RLE 的 5 個位元平面 | 32 色（12-bit RGB） | 解碼器在 `mm2` 的 `sub_33EF2` |
| Mega Drive | 1991，Electronic Arts | ROM 內的區塊：9-bit 調色盤 + LZSS | 16 色 × 多組 | LZSS 在 ROM `0x29954` |
| MSX2 | 1989，Starcraft（日版） | 兩張 192 筆的磁區表；每張圖 `NX/NY` 檔頭 + RLE | MSX2 16 色 | RLE 在常駐區塊 `0xC51A` |

三個非 DOS 平台都走過同一條冤枉路：**先猜編碼參數猜了幾百組，全錯；
改成讀機器碼，第一次就對**。詳細的過程、走不通的路與不要重複走的路
記在 [`docs/research/02-other-platforms.md`](docs/research/02-other-platforms.md)。

### DOS（1988）

牆面、地形、火炬、天空 —— 239 張。第一人稱的四個深度、正牆與左右側牆
都在這裡；透空的部分在原版是色號 8。

![DOS 素材](docs/gallery/dos-tiles.png)

59 個怪物，各取第一張影格。

![DOS 怪物](docs/gallery/dos-monsters.png)

### Amiga（1989）

同一批畫重新上色成 32 色，尺寸比 DOS 少 1–3 個像素（螢幕比例不同）。
排列與 DOS 一一對應，所以 remake 按 `F6` 直接換得過去。

![Amiga 素材](docs/gallery/amiga-tiles.png)

怪物在 72 個 `.anm` 檔裡，每個檔是一段動畫，這裡取基準影格。
調色盤不在檔案裡 —— 場景檔的 32 格只有前 16 格有色，後 16 格留給怪物
在執行時另外設，所以拿場景的盤畫出來會是一片黑。

![Amiga 怪物](docs/gallery/amiga-monsters.png)

### Mega Drive（1991）

62 個區塊、10,120 個 8×8 tile。**看起來是散的，因為它本來就是散的**：
tile 按 ROM 順序排，把它們拼成畫面的 tilemap 還沒解 —— 這張是 tile 表，
不是遊戲畫面。

![Mega Drive tile](docs/gallery/md-tiles.png)

### MSX2（1989，日版）

兩片磁片共 56 張。磁片**沒有可用的檔案系統**（BPB 合法但 FAT 與根目錄
整片是零），遊戲繞過檔案系統直接讀磁區，索引是**兩張各 192 筆的表**
（磁區 1–3 與 4–6）。第一人稱的四套場景各是一張 462×128 的大圖，
天空、地板、側牆、正牆、門、火炬全在裡面 —— 畫面由 VDP 的矩形搬移
一塊一塊組出來。

![MSX 素材](docs/gallery/msx-tiles.png)

總覽圖由 `tools/sheet.py` 從各平台的抽取結果排出來：

```
tools/sheet.py docs/gallery/dos-monsters.png "workplace/gfx/mon/mon*_00.png" \
    --scale 0.5 --key 255,0,255
```

## 驗證方式

每一項結論都要有獨立於「長度剛好對」的證據：

- 標題畫面 render 出來與原版 DOSBox 截圖**逐像素比對 99.92% 相同**；
- EGA 調色盤由原版截圖裁決 —— 65,600 個像素 **100% 落在標準 16 色內**；
- `STR.DAT` 解出的神殿對話與原版畫面**逐字相符**，含引號與斷行位置；
- `DEFAULT.DAT` 的六個預設角色與原版名冊逐一對上；
- Go 與 Python 兩套獨立實作互相對照，其中一次成功抓出另一套的假陽性；
- 版面座標拿素材去原版截圖上做樣板比對定出來 —— 側牆**逐像素 100% 相符**，
  因此抓到 remake 的視圖整體偏了 8 px。

被推翻過的斷言集中記在 [`CONTEXT.md`](CONTEXT.md) §5。

## 文件

- [`CONTEXT.md`](CONTEXT.md) — 專案脈絡與文件索引，接手先讀這份
- [`CLAUDE.md`](CLAUDE.md) — 工作規範、硬性原則、oracle 順序
- `docs/formats/` — 各檔案格式的規格與證據
- `docs/playtest/` — 原版 oracle 的操作流程

## 做法

- 原版 DOS 執行檔是唯一的規則裁決者。手冊與攻略是佐證，可能描述其他平台。
- 反組譯 → 收攏成規格 → 才實作。
- 每個斷言標推論等級（已證實／強推論／假設／未知），`已證實` 要附得出證據。
- 原版 320×200 EGA 骨架與素材保持不動，中文走獨立的高解析點陣疊加層。

## 授權與素材

不散布原版執行檔、資料檔、美術或音樂。公開產出只有引擎程式碼與翻譯文本，
玩家自備合法原版。原版資料一律 gitignore；翻譯檔只存譯文與原文雜湊，不存原文。
`cmd/mm2modern` 烘出來的高解析素材包預設落在 `workplace/`，也不進版控 ——
放大過的原版美術仍然是原版美術。

上面那幾張總覽圖是例外，收在 `docs/gallery/`：它們是四個平台的原版美術，
留在這裡是為了記錄逆向的結果。**這是有意識的取捨，不是漏掉的** —— 對外釋出前
用 `tools/check_release.sh` 過一遍，並決定這批圖跟不跟著走。

## 姊妹專案

[demon_winter_cht](https://github.com/wicanr2/demon_winter_cht) — SSI《Demon's Winter》(1988)
繁中 remake，本專案的方法論、工具鏈與分層架構沿用自它。
