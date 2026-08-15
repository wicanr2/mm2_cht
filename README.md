# Might and Magic II: Gates to Another World — 繁體中文 remake

以 New World Computing 的《Might and Magic II》DOS 版為 oracle，在 Go／Ebiten
重寫可跨平台遊玩的引擎，再接上繁體中文。定位是文化資產保存；公開儲存庫不含
原版程式、資料、美術或音樂，玩家必須自備合法原版。

## 現況

目前已有可遊玩的垂直切片：建角色、探索、事件、戰鬥、施法、商店、旅店、神殿、
訓練、一般戰鬥勝利寶箱與 remake 存檔均接到正常 UI；翻譯工作檔為
2,695／2,695。這不代表所有原版行為都已取得 parity oracle，證據限制列在下表。

**任務線也接完了**：兩位領主（Hoardall 找裝備、Slayer 獵怪，四個難度）、
八大職業考驗、競技賽、年代之門與時空旅行、馬戲團、酒館的酒餐與傳聞，
機制全部由反組譯定出來再實作。整理成
[`docs/quests.md`](docs/quests.md)（任務怎麼觸發、目標、驗收、獎勵）與
[`docs/tavern-rumors.md`](docs/tavern-rumors.md)（四十則酒館傳聞的中英對照）。

反組譯這一側，`1MENU1`／`2CAVES`／`2BRAIN` 三個 overlay 已無未解項；
目錄與各 overlay 的覆蓋狀態見 [`docs/re/README.md`](docs/re/README.md)。
還沒做的事排在 [`docs/todo.md`](docs/todo.md)。

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

開機是原版的片頭畫面，按任意鍵進遊戲：

![片頭](docs/screenshots/17-intro.png)

底圖與 13 張疊圖都在 `MASTER.16` 裡。**疊圖的透空色與落點是拿實機截圖
定出來的**：把每張圖在整個 320×196 上滑動，找「非透空像素與截圖逐格相同」
的落點 —— 判準夠硬，落點錯一格，不合的像素就從 0 跳到 391。獨角獸的頭尾
一直在動，樹叢裡藏著幾張臉會輪流探出來（上圖是左邊那張）。

更多畫面見 [`docs/screenshots/`](docs/screenshots/)。

| 領域 | 狀態 |
|---|---|
| 中文化 | **2,695／2,695（100%）** |
| overlay 與記憶體佈局 | 14 個 overlay 已建立可搜尋反組譯索引；玩家阻塞點採最小充分追查 |
| 檔案格式 | 壓縮、圖形、字型、文字、地圖、事件、物品、怪物、法術全部解開 |
| 事件腳本 | 用到的 49 個 opcode 全部有語意也全部有分支，71 個非空段全部解得開 |
| 戰鬥 | 九個指令已接上；快速與一般 UI 路徑均有抽樣測試；一般勝利留下戰利品，按 `S` 搜尋才進四選項寶箱 |
| 法術 | 96 條有 typed handler；隊員、物品、數字與飛行欄列提示已接正常 UI，未解怪物目標不猜補 |
| 水行術 | remake 於休息、換圖與傳送清除；DOS 存檔持久性尚無動態 oracle |
| 開門 | 機制已結案：開門翻的是屬性層的牆位元，同圖內保持開著、離圖再回就關回去（root `sub_13A64`）|
| Theme | DOS／Amiga／MSX／Modern 完整素材組可於啟動時選擇，遊戲中按 F6 原子切換 |
| 音樂 | Mega Drive／MSX／Amiga／DOS 本機音樂包可替換；正常 UI 依場景切曲，公開 repo 不附原版音檔 |
| 怪物動畫 | 正常戰鬥 UI 依原始影格 hold 播放第一個合法序列；用途對應暫列強推論 |
| 前端 | Ebiten 視窗；互動邏輯與視窗系統無關，headless 也跑得起來 |
| 原版 oracle | DOSBox headless，一鍵跑到第一人稱視角 |

目前明確未完成的 parity 項目包括：DOS 水行術存檔生命週期、尚未證實的怪物施法
目標提示、戰鬥編隊與目標命令的細節、`TOWNT.16` 深度 2 火炬位置。
未證實項目不會用猜測阻塞正常遊玩，完整清單與下一步在
[`docs/todo.md`](docs/todo.md)。

## 玩

```bash
go run ./cmd/mm2 -data <你的 MM2 目錄> -theme dos
```

視窗可以任意拉大縮小，畫面等比例縮放並置中。

若已在本機準備音樂包，可加上：

```bash
go run ./cmd/mm2 -data <你的 MM2 目錄> \
  -music-pack <音樂包目錄>/manifest.json
```

Mega Drive 是已證實最完整的 16 首主要基準；MSX、Amiga 與 DOS 可作替換音源。
音樂包只接受 PCM WAV，會在播放前整包驗證；格式、完整性與權利邊界見
[`docs/music.md`](docs/music.md)。

↑ 前進、↓ 後退、← → 轉向、Enter 推進、Y／N 回答、R 休息受訓、C 施法、
B 撞門、U 開鎖、S 搜尋、M／F3 地圖、W 世界地圖、K／Q 查說明書、I 物品、G 商店、
N 建角色、F2 存檔；戰鬥中 Enter 攻擊、T 射擊、C 施法、A 抵擋、F 溜跑、
P 防護、V 檢視、X 對調，F5／F6 一樣可按。

`B`／`C`／`M`／`Q`／`R`／`S`／`U` 與原版同鍵（見
[`docs/research/command-keys-oracle.md`](docs/research/command-keys-oracle.md)），
其餘是原版沒有的功能，放在原版沒用到的字母上。

戰鬥勝利留下的戰利品要**按 `S` 搜尋**才撿得起來，照原版
（`2PLAY` 指令分派 `0x181E8` → root `0x13814`，見
[`docs/research/command-keys-oracle.md`](docs/research/command-keys-oracle.md)）。
沒有東西可撿時會回「這裡什麼都沒有。」，對應原版的 `Nothing Here!`。

`F5` 在兩種牆面畫法之間切換：原版像素（nearest 整數倍放大）與 Scale3x
（色塊邊界補斜角）。兩種畫的是同一批原版素材、同一組顏色，差別只在放大方式 ——
中文走 24×24 點陣，與 8×8 放大三倍的牆面像素密度差三倍，Scale3x 讓兩者接近。

`F6` 換素材來源：DOS → Amiga → MSX → 高解析素材包。只有完整通過牆面、地板、
火炬與天空契約的素材組才會加入循環，切換時整組替換，不會混到兩個平台的圖。
可用啟動參數指定來源與第一套 Theme：

```bash
go run ./cmd/mm2 -data <MM2> \
  -amiga-dir <Amiga 素材目錄> \
  -msx-dir <MSX 磁片目錄> \
  -modern-dir <Modern 素材包> \
  -theme amiga
```

`-theme` 接受 `dos`、`amiga`、`msx`、`modern`；明確指定但素材不完整時會停止並
說明錯誤，不會安靜退回 DOS。各平台走**同一套幾何**，
換的只是圖 —— `town.32` 與 `TOWN.16` 都是 32 張、同樣的深度與側牆順序，
所以 Amiga 版的牆可以直接貼進原本的透視裡。真正的差別只有兩個：
透空色（DOS 是色號 8、Amiga 是 0）與火炬的張數（DOS 每格四張含燈桿底圖，
Amiga 只有三張火焰）。Amiga 素材要放在 `workplace/amiga/`（自備原版磁片，
用 `tools/adf.py` 抽）、MSX 的 `.dsk` 放 `workplace/msx/`，沒有就只有 DOS 可選。

MSX 那一套是另一種接法：**素材不是一張一張的檔案**，整套場景是一張
462×128 的素材表，每一面牆是表裡的一塊矩形，落點另有一張表
（`internal/assets/msx`）。原版靠 VDP 的矩形搬移組出畫面，remake 直接
切圖來貼 —— 不需要模擬 VRAM。視圖只有 154×64，整幅置中放進視圖區。
目前接上的是側牆與正牆各四個深度、門與火炬；更深處的門與火炬還沒定位。
**MSX 原版的火炬不會動**（每個位置只有一張貼圖，沒有 DOS 那種三張火焰的動畫組），
remake 把火焰左右各位移一像素做出三張影格 —— 那是加上去的效果，不是還原。

![MSX 素材在 remake 裡](docs/screenshots/01f-first-person-msx.png)

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

## 推廣片與三平台封包

Linux、Windows、macOS 的公開引擎封包由 `tools/package_release.sh` 在 Docker 內建立；
含玩家自備原版資料的完整版只留在被忽略的 `.local-full/`，不得提交或上傳。

推廣片使用當前引擎重新截圖，並以玩家自備 `MM2.EXE` 的原始音高／時值資料離線轉譯
成 PC 喇叭（PC speaker）方波，再將曲目索引 3–8 串成僅供本機的推廣片組曲
（medley）：

```bash
bash tools/render_promo.sh --data-dir <你的 MM2 目錄>
```

輸出在被 Git 忽略的 `workplace/promo/mm2-remake-trailer.mp4`。影片含原版衍生視覺與
玩家自備音源資料，預設只留本機；這個 WAV 不是 DOSBox 原始錄音。公開前必須另行
確認素材權利或換成可再散布的原創 Theme。分鏡、重拍方式與音源證據見
[`docs/promo.md`](docs/promo.md)。

## 各版本素材總覽

DOS 之外還有三個平台。**四套素材是各自逆向出來的，容器一個都不一樣**，
彼此沒有共用的格式；共通的只有美術本身 —— 同一批畫，各自重新上色與重排。

| 平台 | 發行 | 容器 | 顏色 | 破解點 |
|---|---|---|---|---|
| DOS | 1988，New World Computing | `.16`：LZW 段內含影像目錄 | EGA 16 色 | LZW 變體，見 [`docs/formats/03`](docs/formats/03-lzw-compression.md) |
| Amiga | 1989 | `.32`：目錄 + 32 色盤 + nibble RLE 的 5 個位元平面 | 32 色（12-bit RGB） | 解碼器在 `mm2` 的 `sub_33EF2` |
| Mega Drive | 1991，Electronic Arts | ROM 內的區塊：LZSS，前面**可能**有 9-bit 調色盤 | 16 色 × 多組 | LZSS 在 ROM `0x29954` |
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

怪物在 72 個 `.anm` 檔裡，每個檔是一段動畫（檔頭 48 bytes 是每個影格的矩形表），
這裡取基準影格。

**每個 `.anm` 自帶調色盤** —— 它的本體就是一個標準的 `.32` 容器。
執行檔那邊：`sub_33A18`（動畫載入）把檔案交給 `.32` 的解析器 `sub_33322`，
後者把該檔的 64 bytes 填進送硬體的緩衝 `A4−0x4F0`，`sub_33A18` 再只把
`0x12`–`0x1F` 從畫面現有色表抄回來。所以一隻怪的顏色是**自己檔案裡的
`0x00`–`0x11`** ＋ 場景的 `0x12`–`0x1F`。推導見
[`docs/research/02-other-platforms.md`](docs/research/02-other-platforms.md)。

![Amiga 怪物](docs/gallery/amiga-monsters.png)

### Mega Drive（1991）

怪物圖是 **11×11 個 tile（88×88 px）**，由九個硬體 sprite 拼出來，
畫面內容由一張 nametable 決定（`raw % 242 == 0` 就是它，242 ＝ 11×11×2，
商是影格數）。ROM 裡 75 筆，與 DOS `MONSTERS.16` 的 75 個槽數量相同。

![Mega Drive 怪物](docs/gallery/md-monsters.png)

排序照 **DOS 的槽號**，所以這張與上面的 DOS、Amiga 兩張可以逐格對照；
末尾十三張是 MD 才有的（片頭的旋轉地球、書、城堡、酒館場景）。

**格子的順序是 sprite 順序不是 row-major** —— 照 row-major 攤會得到
每一格都合法、看起來卻像雜訊的東西。那張 sprite 版面是從實機的
sprite 屬性表影子（work RAM `0xFFD2A8`）讀出來的。

**怪物素材已經接進遊戲**：烘好素材包之後按 `F6`，戰鬥中的怪物會在
DOS → Mega Drive → Amiga 之間換（場景沿用 DOS —— 那兩個平台的牆面
還沒抽進引擎）。`F5`／`F6` 在戰鬥中也能按，看得到怪物的時候正是想換的時候。

| Mega Drive | Amiga |
|---|---|
| ![](docs/screenshots/15-md-monster.png) | ![](docs/screenshots/16-amiga-monster.png) |

```bash
python3 tools/mdgfx.py "workplace/genesis/*.md" --export workplace/md-monsters \
    --data workplace/orig/MM2
python3 tools/amiga32.py --export-monsters workplace/amiga-monsters \
    workplace/orig/MM2 workplace/amiga/*.anm
```

槽號兩邊都是用剪影比對再做貪婪一對一指派對出來的（見
[`tools/monpack.py`](tools/monpack.py)）——**照檔案順序推會錯**：
Mega Drive 的順序與 DOS 槽號是一個排列，Amiga 接近恆等但從第 42 槽起
整批位移。Amiga 目前只出基準圖，動畫零件的存法還沒解。

既然 sprite 版面定案，就直接烘成檔案，不必在執行時重建：

```bash
python3 tools/mdgfx.py "workplace/genesis/*.md" --export workplace/md-monsters \
    --data workplace/orig/MM2
```

72 張圖、531 個影格的 PNG（88×88 RGBA，索引 0 透空）加一份 `set.json`。
槽號用剪影比對再做一對一指派對出來（ROM 的順序與 DOS 的槽號是一個排列），
59 個 DOS 非空槽全部對到，分數中位數 0.984；另外 13 張是 MD 才有的場景圖。
產出落在 `workplace/`，不進版控。

下面這張是 72 個 tile 池本身（去重過的圖案，順序與畫面無關）：

![Mega Drive tile](docs/gallery/md-tiles.png)

### MSX2（1989，日版）

兩片磁片共 56 張。磁片**沒有可用的檔案系統**（BPB 合法但 FAT 與根目錄
整片是零），遊戲繞過檔案系統直接讀磁區，索引是**兩張各 192 筆的表**
（磁區 1–3 與 4–6）。第一人稱的四套場景各是一張 462×128 的大圖，
天空、地板、側牆、正牆、門、火炬全在裡面 —— 畫面由 VDP 的矩形搬移
一塊一塊組出來。

![MSX 素材](docs/gallery/msx-tiles.png)

第一人稱視圖是 VDP 的矩形搬移一塊一塊組出來的。把原版的貼圖清單
（從反組譯抽出來的 `(SX,SY,NX,NY) → (DX,DY)`）跑一遍重畫出來 ——
左起：室內全部疊起來、戶外全部疊起來、側牆、正牆、門、樹。

![MSX 第一人稱](docs/gallery/msx-firstperson.png)

總覽圖由 `tools/sheet.py` 從各平台的抽取結果排出來：

```
tools/sheet.py docs/gallery/dos-monsters.png "workplace/gfx/mon/mon*_00.png" \
    --scale 0.5 --key 255,0,255
```

## 驗證方式

每一項結論都要有獨立於「長度剛好對」的證據：

- 標題畫面連同 13 張疊圖合成出來，與原版 DOSBox 截圖**逐像素相同**；
  透空色與每張疊圖的落點都是這樣定出來的（落點錯一格，不合的像素從 0 跳到 391）；
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
- [`docs/todo.md`](docs/todo.md) — 還沒做的事，按「會不會擋住玩家或發行」分層
- [`docs/quests.md`](docs/quests.md) — 任務線：觸發、目標、驗收、獎勵
- [`docs/tavern-rumors.md`](docs/tavern-rumors.md) — 四十則酒館傳聞的中英對照
- [`docs/re/`](docs/re/) — 反組譯筆記與函式索引
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
