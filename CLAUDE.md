# Might and Magic II: Gates to Another World — 繁體中文化與 remake

把 New World Computing《Might and Magic II》(1988, DOS) 完整逆向、在 Go / Ebiten 上重寫引擎，
再做繁體中文化。定位是**文化資產保存**，替華人遊戲圈留下這款經典。

方法論沿用姊妹專案《Demon's Winter》冬之魔（`~/cht/daemon_winter`，
[demon_winter_cht](https://github.com/wicanr2/demon_winter_cht)）——包含它走過的回頭路。

---

## 1. 硬性原則

1. **完整性 > 投報。** 不得以「成本高、效益低」為由跳過任何素材、任何格式、任何平台版本。
   卡關就換方法，記錄「卡在哪、試過什麼」，不寫「暫緩／低投報」當結論。
2. **SDD：spec 齊了才實作。** 反組譯 → 收攏成規格（`docs/spec/`）→ 才動手寫程式。
   只有標 `READY` 的規格可以開工。
3. **推論等級要誠實。** 每個斷言標 `已證實` / `強推論` / `假設` / `未知`。
   `已證實` 必須附位址、位元組範圍、資料 diff、原版截圖或可重跑的實驗。
4. **未知位元組必須原樣往返（round-trip）。** 不准為了讓工作清單看起來完整而發明玩法。
5. **不散布原版執行檔、資料檔、美術或音樂。** 公開產出只有引擎程式碼與翻譯文本，
   玩家自備合法原版。原版資料一律 gitignore。

### 裁決順序（oracle）

| 順位 | 來源 | 用途 |
|---|---|---|
| 1 | `MM2.EXE` + 14 個 `.OVL` 的實際行為 | **唯一裁決者**。規則爭議一律以此為準 |
| 2 | DOSBox 實機截圖／錄影 | 靜態反組譯撞牆時的第三條路（見 §5） |
| 3 | MSX 版（Starcraft 1989，四張 `.dsk`） | **交叉參考，不承諾支援**。用來對照素材差異與日版譯名 |
| 4 | 官方手冊、珍017 中文說明書掃描、社群攻略 | 佐證與譯名來源，可能描述其他平台 |

平台差異出現衝突時，DOS 版贏；MSX 版的差異寫進 `docs/research/` 存證，不改 DOS 行為。

---

## 2. 素材盤點

| 素材 | 位置 | 內容 |
|---|---|---|
| DOS 版本體 | `Might and Magic 2 - Gates to Another World (1988).zip` | 64 檔 655,404 bytes，TorrentZip |
| MSX 版 | `msx/*.zip` | Starcraft 1989 日版，Disk 1/2 各含 `[a]` 版本，737,280 bytes/片 |
| 中文說明書掃描 | `珍017-魔法門II.rar` | 91 張 jpg（`SCAN1047`/`1048`/`1051` 三本），來源標註「骨灰集散地」 |

DOS 版檔案分類：

- **程式**：`MM2.EXE` (77,824)、`1MENU1/1MENU2/1RETINN/2BRAIN/2CAST1/2CAST2/2CAVES/2CMDS/2COMBAT/2MISC/2MISC2/2PLAY/2SMITH/2TEMPLE` 共 14 個 `.OVL`
- **顯示驅動**：`CGA.DRV`、`EGA.DRV`、`HGA.DRV`、`MCGA.DRV`、`TGA.DRV`、`TIMER.DRV`
- **資料**：`EVENTSI.DAT` (49,609)、`EVENTSO.DAT` (25,797)、`MAP.DAT` (18,748)、`ROSTER.DAT` (8,293)、
  `MONSTERS.DAT` (5,702)、`ITEMS.DAT` (5,120)、`STR.DAT` (4,700)、`ATTRIB.DAT`、`DEFAULT.DAT`、`SPELLS.DAT` (192)
- **圖形**：`.16` 系列共 26 個，最大 `MONSTERS.16` (168,257)；含地形（`DESERT`/`SWAMP`/`TUNDRA`/`OCEAN`/`SKY`/`OUTDOOR1-3`）、
  場景（`TOWN`/`CAVE`/`CASTLE` 各含 `B`/`F`/`T` 三個附屬檔）、介面（`MASTER`/`BOOK`/`GLOBE`/`DISK`/`NWCP`/`XFER`/`THROW`/`ENDGAME`）
- **字型**：`MM2.CH` (1,024)

**開工第一件事：對全部素材算 SHA-256 寫進 `docs/inventory.md`。** 後續每份筆記都標「輸入檔 + SHA-256 + IDA 位址」。

---

## 3. 已確認的起點事實

以下是寫這份文件時實際驗過的，不是從別的專案抄來的假設。

| 事實 | 等級 | 證據 |
|---|---|---|
| `MM2.EXE` 未經 PKLITE/LZEXE 打包 | 已證實 | MZ header 正常，`strings` 直接看得到 `Middlegate`、`Atlantium`、`Knight`、`Version 1.01` 等遊戲文本 |
| 執行檔內建 overlay 檔名表 | 已證實 | `strings` 命中 14 個 `.OVL`（大寫）與 26 個資料/圖檔名（**小寫**，如 `monsters.16`、`eventsi.dat`） |
| `.OVL` 不是 MZ，是裸機器碼段 | 已證實 | `file` 回 `data`；`2COMBAT.OVL` 首三位元組 `55 8B EC` = `push bp; mov bp,sp`，C 函式序言 |
| 用 Phoenix 的 overlay runtime（推測是 PLINK86） | 強推論 | EXE 內 `Copyright (C) 1984, 1985, 1986 by Phoenix Software Associates Ltd.`。**呼叫慣例因此推定為 C（右至左壓棧、呼叫者清棧），不是 Pascal** |
| `.DRV` 是 COM 格式的顯示驅動 | 已證實 | `file` 認出 `DOS executable (COM)`，起始指令是 jump table |
| `STR.DAT` 是 LZW 段，解開後每個位元組再 **+0x1C** 就是完整可讀的行文字 | 已證實 | 見 [`docs/formats/03`](docs/formats/03-lzw-compression.md)。不是單字索引 —— 那個誤判來自錯的解密量（−4）把空格留成分隔符 |
| DOS 之外還有 Amiga（1989）、Mega Drive（1991）、MSX2（1989）三個版本，素材全部萃取完成 | 已證實 | 容器三個都不一樣，見 [`docs/research/02`](docs/research/02-other-platforms.md)。Amiga 與 MSX 的素材已接進 remake，遊戲中按 `F6` 切換 |
| overlay 機制、描述表、thunk 表、記憶體佈局 | 已證實 | 見 [`docs/formats/01-overlay-and-memory-layout.md`](docs/formats/01-overlay-and-memory-layout.md)。14 個 overlay 全部反組譯完成，599 個函式 |
| `MM2.EXE` 尾部 43,504 bytes 不在 MZ image 內 | 已證實 | MZ header 宣告 34,320 bytes，實體檔案 77,824。尾部（file `0x8610` 起）是遊戲主字串表：城鎮、職業、種族、陣營、技能名，950 段可見字串佔 34% |
| 尾部資料區執行時載入到偏移 `0x0D850` | 強推論 | 區內開頭兩個 far pointer `0D85:478A`、`0D85:481E` 指向自身；`0x0D85` 正是最大 overlay 的結束段；總長與 `e_minalloc` 算出的 `0x18A20` 吻合 |

### 檔名大小寫這件事

EXE 裡的資料檔名是小寫、overlay 名是大寫，而 zip 裡的實體檔名全是大寫。DOS 不分大小寫所以原版沒事，
**Go remake 跑在 Linux/macOS 上會炸**。檔案存取層一開始就要做大小寫無關解析，不要等到踩到才補。

---

## 4. 未解清單放哪裡

**這份文件不列未解項目。** 開工順序看 [`CONTEXT.md`](CONTEXT.md) §3「進行中」，
它每一輪跟著更新；各檔案格式自己的缺口列在 `docs/formats/*.md` 的「未解／待解」小節。

理由是寫過的教訓：kick-off 時列的「第一批未知」在解掉之後不會自動消失，
而這份文件每個 session 都會載入 —— **已經解掉卻還掛在清單上的項目，
比沒有清單更糟**，它會讓下一輪重做已經做完的事，或以為某個答案還不存在。
斷言被推翻或問題被解決時，改的是**現況那一段**，推翻紀錄集中到
`CONTEXT.md` 的「已被推翻的斷言」表。

### 外部資料的盤點狀態

已經讀完並用上的只有一份：1990 年代《軟體世界》雜誌那幾篇
（[`docs/research/soft-world/`](docs/research/)），它給了 `ROSTER.DAT` 與
`ITEMS.DAT` 的獨立對照表，角色記錄 130 bytes 由它與 taskboy 的工具互相印證。

**下面這三個還沒讀，碰到相關領域之前先去看**：

- [`Vairn/Smite-and-Magic`](https://github.com/Vairn/Smite-and-Magic) — MM2 逆向專案，從 Amiga 版入手
- ScummVM `mm` 引擎（原 `xeen`）涵蓋 MM1 與 Xeen 系列（MM4/5），**未見 MM2**
- [blurglecruncheon 的 MM2 地圖與線索站](https://www.geocities.ws/blurglecruncheon/mm2/main.htm)

**碰新領域前先查「有沒有人已經做過同一件事」**，不是做到一半才發現。

---

## 5. 工具鏈

### IDA Pro 9.4（反組譯主力）

```
image 來源：/home/anr2/ida_94_official/dist
image 名稱：ida-pro-9.4-ver2
```

包成 `tools/ida.sh`，不用 docker compose。完整踩坑清單見
`~/.claude/knowledge-base/retro/ida-pro-9.4.md`，這裡列這個專案一定會撞到的：

- **16-bit real mode 沒有 Hex-Rays**，只能讀組語。
- **寫 IDC，不要寫 IDAPython**（實測 `-S` 給 `.py` 無輸出）。IDC 少了 `#include <idc.idc>` 會**安靜地 exit 1**。
- **headless 的 `print` 不進 stdout**，腳本一律 `fopen` 寫檔。不寫檔＝沒跑，exit code 還是 0。
- **不要 grep `.asm` 找線性位址。** 16-bit 的反組譯文字顯示 `segment:offset`，
  五位十六進位常數在整份 `.asm` 裡是零筆——零命中與「真的沒人碰」長得一模一樣。
  要查誰碰某塊記憶體，用 `XrefType()` 逐 byte 查資料庫的 xref 圖。
- **判讀讀／寫不要自己解析指令文字**，`XrefType()` 已經標好了。
- **IDC 崩掉會弄壞 `.i64`**，症狀是「Failed to initialize IDA as library」，看起來像 image 壞掉。
  判斷方法：拿另一個 `.i64` 跑同一支已知可用的腳本。壞的那個刪掉重跑 `analyze`。
- **函式索引先建再讀。** 超過二三十份筆記之後，光靠記憶一定會重讀已經解過的函式。
  掃 `docs/**/*.md` 抽 `sub_[0-9A-F]+` 產 `docs/re/00-function-index.md`。
- **「唯一」「只有一處」沒有全檔掃描佐證就不要寫。**

`.i64`、`.asm`、解包後的 binary 全部 gitignore。

### 其他平台的工具

`tools/ida.sh` 除了 16-bit 的 `analyze`／`ovl`，另有 `m68k`（Amiga／Mega Drive
的裸機器碼）與 `z80`（MSX）。**`-b` 的單位是 16 bytes 不是位元組**：
MSX 載到 `0x0100` 要給 `10`。給錯會反出**看起來完全合理**的組語，沒有症狀。

| 工具 | 做什麼 |
|---|---|
| `tools/adf.py` | Amiga `.adf` 抽檔（OFS，要剝掉每個資料區塊 24 bytes 的標頭）|
| `tools/amiga32.py` | Amiga `.32`／`.anm`：目錄、12-bit 調色盤、nibble RLE、5 個位元平面 |
| `tools/mdgfx.py` | Mega Drive 的區塊：LZSS（ROM `0x29954`）＋ 9-bit 調色盤 |
| `tools/msxdsk.py` | MSX `.dsk`：**兩張** 192 筆的磁區表、常駐引擎、調色盤、RLE |
| `tools/msxblits.py` | 從 MSX 的反組譯抽第一人稱貼圖參數 |
| `tools/msxview.py` | 照那張表重畫 MSX 的視圖（驗收用）|
| `tools/sheet.py` | 把一批 PNG 排成總覽圖 |

**基底暫存器的值要先定再讀碼。** 三個平台都撞過同一件事：Amiga 的 `A4`、
Mega Drive 的 `a5`、MSX 的載入位址。定錯不會有症狀 —— 反出來的東西照樣
像合理的程式碼，只是全部指向錯的地方。定的方法是**結構條件**不是內容：
「所有 `jsr (d16,a4)` 的目標必須落在 6-byte 對齊的 thunk 表裡」這種。

### 其他

- **Go 1.25 + Ebiten v2**，模組名 `github.com/wicanr2/mm2_cht`。分層沿用冬之魔：
  `internal/{assets,game,ui,i18n,rng,audio}`，UI/平台 → 遊戲規則 → 解析後資料，單向依賴。
- **編譯一律走 docker**（`docker/go`）。手打 `docker run` 一律帶 `--rm --log-opt max-size=10m --log-opt max-file=3`。
- **DOSBox docker 化**，可自動送鍵與截圖，當行為 oracle 與畫面對照基準。
- Python 一律 docker uv.venv，不污染系統環境。

### Docker 邊界（違反過一次，代價是別的專案的 image）

**禁止** `docker image prune`（含 `-a`）、`system prune`、`volume prune`、`builder prune`、`rmi`、
`container prune`、`network prune`。這台機器同時放著多個客戶專案的 image。
只清理自己這批建立的 container，或一開始就 `--rm`。要空間就列候選清單請使用者決定。

派 subagent 時這些邊界要**寫進 prompt**，不能只靠 agent 自律；沒寫的等於允許。

---

## 6. 畫面與中文顯示策略

**先像素還原原版，中文走高解析疊加層。**

- 原版 320×200 EGA 骨架與素材**不動**，用 nearest-neighbor 整數倍放大保持銳利。
- 中文字用獨立的高解析點陣路徑（16×16 或 24×24）直接畫進放大後的緩衝，
  **不縮小中文去塞原本的小字位**——筆畫會糊成一團。ASCII 仍走原版字型路徑。
- 座標映射、滑鼠命中區都要一起換算；minimap 這類直接畫在舊座標空間的 widget 單獨處理。
- 做法細節見 `~/.claude/rulebook/81-retro-cjk-hires-canvas.md`。

冬之魔一開始直接把畫布拉到 640×400 重排版面，後來被指出「中英混排風格迥異、版面不像原版」而回頭改。
**這個專案從第一版就走疊加層路線。**

翻譯文本一律進 `translations/` 的 JSON 與譯名表，不進 Go 原始碼。
Go 只能保留穩定的 key、動作、格式參數與版面行為。每批加 UI 的功能都要過硬編碼中文掃描。

---

## 7. 文件契約

第一輪逆向產出之後建立 `CONTEXT.md` 當全專案單一入口（對話被壓縮或換 session 接手時先讀它），
內含：現況一覽、文件索引、術語表、oracle 優先序、**已被推翻的斷言清單**、工作紀律。

```
docs/
  inventory.md      素材清單 + SHA-256
  re/               反組譯筆記（每份標輸入檔 + SHA-256 + IDA 位址）
  formats/          檔案格式規格
  spec/             實作規格（只有 READY 的可以動手）
  research/         平台差異、社群成果、外部資料調查
  playtest/         DOSBox 實機對照與截圖證據
  manual-cht/       珍017 中文說明書轉錄
  gallery/          各平台的素材總覽圖（README 引用）
translations/
  glossary.md       統一譯名表
```

### 寫入紀律

**斷言被推翻時，把正文改寫成正確答案。** 不要在正文敘述自己怎麼推錯的——
單獨讀到那一節的人只會看到那一節，正文摻雜錯誤版本加檢討，他拿到的就是錯的答案。
推翻紀錄集中到 `CONTEXT.md` 的「已被推翻的斷言」表，正文最多留一個指標。

教訓寫成**規則**，不要寫成事件敘述：現在式的教訓（「X 還是不存在」）會在修好的那一刻變成假斷言。

**不要把猜測 commit 成結論式筆記。** 掛著「已查證」名義的猜測比沒有筆記更糟。

---

## 8. 每一輪要做的事

1. 動手前查 `~/.claude/rules/00-rules-index.md` 的觸發表，命中就先讀對應 rulebook。
   ⚠ 「任務開始」不只是使用者剛開口那一刻——長跑實驗的每一輪、連續兩輪同類失敗、
   要下「這個修法有效／無效」的結論前，都要回來查表。
2. 反組譯或實作 → 更新 markdown → **清掉被推翻的斷言** → commit + push → 更新 `CONTEXT.md` 現況。
3. 宣告完成前重跑對應測試，並在 markdown 留下畫面或實機證據。**編譯成功不是視覺測試。**
4. 慢回饋的驗證（DOSBox 逐項對照）動手前先把證據看齊，不要「改一行再跑一遍」。
5. 高成本或佔用共用資源的動作，事前報數字：「N 個單位 × 每單位耗時 ≈ 總時長，期間佔 M 核」。

### 反覆出現的錯誤結論

- 檔案大小能整除只證明某種排版**可能**成立。要 render 出來比對。
- 欄位名不是證據。寫入來源與至少一個使用端都要追。
- remake 自己的單元測試通過，證明的是內部一致性，不是與原版一致。
- 用偵錯捷徑走完的流程，不能證明遊戲正常可玩。
- 陳舊的工作清單會與已經解決的研究互相矛盾。開新的逆向之前先用程式與證據稽核文件。
- **掃不到就先懷疑樣式，不要先下「不存在」。** MSX 的 VDP 存取掃 `ld c,98h`
  零命中，因為埠放在暫存器裡（`out (c),a`）。
- **拿某個前綴當掃描入口會漏掉沒有那個前綴的資料。** Mega Drive 的圖形區塊
  原本以調色盤當入口又框了範圍，漏掉十塊（其中一塊是字型）；MSX 的檔案表
  只讀了第一張，圖形檔一個都看不到。**漏掉的長相與「不存在」一模一樣。**
- **驗收要問「存不存在會動的案例」，不是「這一個案例會不會動」。** 挑到沒有
  該現象的樣本，會得出「功能壞了」並往錯的方向查一輪。

---

## 9. GitHub

```
https://github.com/wicanr2/mm2_cht   （private，第一個可玩切片後再轉 public）
```

- 原版 zip / rar / 解包後的資料一律 gitignore。
- 釋出包要過 deny-list 掃描，確認沒夾帶原版資產。
- push、開 PR、合併分支前先回報。

## 10. 邊界

- 只做靜態分析、格式保存與互通性研究。不協助破解 DRM、繞過授權、修改付費驗證。
- IDA license 只唯讀掛載，不出現在 log、截圖或報告裡。
- 不在 container 內執行老遊戲；IDA 只讀檔案。
- 不把 `/home`、SSH agent 或整個 host filesystem 掛進 container。
- 原版素材、解包目錄、使用者提供的字型視為唯讀參考。測試存檔一律寫 `/tmp` 或明確的測試輸出目錄。

## 工作目錄

- `workplace/orig/` — 解包後的原版檔案（唯讀對待，gitignore）
- `workplace/ida/` — `.i64` / `.asm` 反組譯產物（gitignore）
</content>
