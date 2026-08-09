# CONTEXT — 專案脈絡與文件索引

這份是全專案的單一入口。對話被壓縮、或換一個新 session 接手時先讀這份，
再依索引跳到需要的文件。工作規範在 [`CLAUDE.md`](CLAUDE.md)。

最後更新：2026-08-09

---

## 1. 一句話現況

原版的壓縮、圖形、字型、道具表、事件字串已經解開並在 Go 上重實作；
引擎已經能跑：載入原版 `MAP.DAT` 與事件檔，在 16×16 的地圖上移動、
踩到事件格顯示繁中訊息（`cmd/mm2walk` 每步輸出 PNG，headless 可驗收）。
`STR.DAT` 全文與五座主要城鎮的字串已翻完。
下一步是第一人稱視角 —— 缺的是屬性層「哪一格是牆」的判定。

## 2. 已完成

| 領域 | 狀態 | 文件 |
|---|---|---|
| overlay 機制與記憶體佈局 | 14 個 overlay 全數反組譯，599 個函式 | [`docs/formats/01`](docs/formats/01-overlay-and-memory-layout.md) |
| LZW 壓縮 | 演算法讀自 `sub_12242`，全部資料檔可解 | [`docs/formats/03`](docs/formats/03-lzw-compression.md) |
| `.16` 圖形 | 兩型檔頭都可解，26 個檔全部 render，怪物圖的 RLE 除外。標題畫面與原版截圖逐像素 99.92% 相同 | [`docs/formats/04`](docs/formats/04-graphics.md) |
| `MM2.CH` 字型 | 8×8 × 128 字元，ASCII 對位驗證 | [`docs/formats/02`](docs/formats/02-data-files.md) §2 |
| `ITEMS.DAT` | stride 20 × 256 筆 | [`docs/formats/02`](docs/formats/02-data-files.md) §1 |
| `STR.DAT` | LZW + 每 byte +0x1C，400 行純文字，與原版畫面逐字相符 | [`docs/formats/05`](docs/formats/05-text-system.md) |
| 事件段佈局 | 事件表 + skip + 腳本區 + 字串區，讀自 `sub_1A85C`；59/71 段適用 | [`docs/formats/02`](docs/formats/02-data-files.md) §4 |
| `MAP.DAT` 兩層結構 | 地形層 + 屬性層各 16×16；屬性層 bit3 = 有事件（五段 100% 零例外）。MAP 段 k 對應 EVENTSI 段 k | [`docs/formats/06`](docs/formats/06-map.md) |
| 事件字串 | 71 段全數抽出 1,308 條 | [`docs/formats/02`](docs/formats/02-data-files.md) §4 |
| 中文顯示 | 24×24 點陣、中英混排、`@` 換行、缺字檢查 | — |
| EGA 調色盤 | 原版標準 16 色，截圖 100% 落在表內 | [`docs/formats/04`](docs/formats/04-graphics.md) §1 |
| 原版 oracle | DOSBox headless + timeline 自動化，可一鍵跑到第一人稱視角 | [`docs/playtest/01`](docs/playtest/01-oracle-timeline.md) |
| `DEFAULT.DAT` 角色 | 六個預設角色與原版名冊逐一對上 | [`docs/playtest/01`](docs/playtest/01-oracle-timeline.md) §4 |
| Go 解碼器 | `lzw` / `gfx` / `font` / `cjk` / `events` / `text`，測試對照原版 | — |
| **遊戲邏輯層** | `internal/game`：地圖兩層、移動、轉向、事件觸發，確定性測試 | — |
| **可跑的雛形** | `cmd/mm2walk` 走固定路線輸出每步 PNG：載入原版地圖 → 移動 → 踩到事件格 → 顯示中文 | — |
| 中文化管線 | `mm2strings export/check`，版控只存譯文與原文雜湊 | — |

## 2.5 中文化的素材分兩批

| 來源 | 條數 | 內容 |
|---|---|---|
| `EVENTSI/O.DAT` | 1,308 | 設施名、短提示、按鍵詢問 |
| `STR.DAT` | 400 行 | 劇情、對話、選單、結局、謎題 |

兩批都是明文，逐條翻即可。詳見
[`docs/formats/05-text-system.md`](docs/formats/05-text-system.md)。

## 3. 進行中

| 項目 | 現況 |
|---|---|
| 事件表欄位語意 | 佈局已從 `sub_1A85C` 解出（見 [`docs/formats/02`](docs/formats/02-data-files.md) §4），59/71 段適用。`Cell` 是格位置已確定；`Index`（1 起算）與 `Kind`（高 nibble 類型）的語意未定 |
| 12 段不符合事件表佈局 | EVENTSI 8/44、EVENTSO 4/27，編號偏後。目前只抽字串，結構留未解 |
| 中英字級比例 | 英文走原版 8×8 放大 3 倍、中文走 24×24 點陣，像素密度是 3:1，英文看起來明顯較粗。可用但不協調 —— 要對照原版畫面決定是否改成 2 倍 + 16×16。**冬之魔就是在這一項上走了回頭路，不要等到全部翻完才處理** |
| 翻譯進度 | `STR.DAT` 70 條 **100%**；事件檔的**五座主要城鎮全部翻完**（Middlegate／Atlantium／Tundara／Vulcania／Sansobar）。合計 200/1,378（14.5%）。剩下是地城、野外與其他區域 |
| `STR.DAT` 訊息索引 | 未解，導致翻譯的 key 粒度粗 —— 中間沒有空行的多個獨立訊息會被併成一條（`str.274` 從商店選單一路到片尾地址）。不影響譯文品質，但 remake 的文字層要自己切 |
| 密碼謎題 | 結局前的謎題是把美國憲法序言做成密文，翻成中文後原本的解法不成立，要單獨設計 |
| **第一人稱視角** | 素材已備齊（`TOWN.16` 的 160×92 / 96×56 / 48×28 是三種深度的牆、`TOWNF.16` 地面、`TOWNT.16` 火炬），缺的是屬性層「哪一格是牆」的判定 —— 目前只解出 bit3 |
| `MAP.DAT` 其餘位元 | 地形層的 tile 對應表、屬性層另外 7 個位元（牆／門／可通行）未解。下一步用 oracle 逐格移動截圖對照 |
| 事件觸發用 `Index` 當字串序號 | 目前的實作把事件記錄的 `Index` 當字串序號（原版 `sub_18FD0` 的行為與序號一致），**假設待驗** —— 要拿原版逐格對照 |
| `MONSTERS.16` RLE | 段內索引、動畫序列表、影像頭 x/y/w/h 已解，像素編碼未解 |


## 4. 文件索引

```
CLAUDE.md                          工作規範、硬性原則、oracle 順序、工具鏈
docs/inventory.md                  63 個原版檔案 + 封存檔的 SHA-256
docs/formats/01-overlay-and-memory-layout.md
docs/formats/02-data-files.md      各資料檔的記錄結構
docs/formats/03-lzw-compression.md 壓縮與 STR.DAT 的位移層
docs/formats/04-graphics.md        .16 圖形
docs/formats/05-text-system.md     文字系統（STR.DAT 長文字）
docs/formats/06-map.md             地圖：兩層結構與屬性層 bit3
docs/playtest/01-oracle-timeline.md  原版 oracle 的按鍵流程與前置條件
tools/ida.sh                       IDA 9.4 headless（analyze / ovl / script / raw）
tools/build_ovl_image.py           重建執行時佈局供 IDA 反組譯 overlay
tools/dosbox_run.sh                原版 oracle（timeline: wait/key/type/shot）
tools/mm2lzw.py  mm216.py  probe_dat.py
internal/assets/{lzw,gfx,font,events}  Go 版解碼器
internal/render                    兩層畫布（原版像素層 + 高解析文字層）
cmd/mm2dump                        headless 輸出 PNG，供無 GPU 環境驗收
cmd/mm2strings                     匯出/檢查可翻譯字串
tools/build_cjk_font.py            從譯文烘 24×24 中文點陣 atlas
internal/assets/cjk                atlas 載入與缺字檢查
assets/font/cjk24.bin              烘好的 atlas（隨譯文重烘）
translations/zh-Hant.json          譯文 + 原文雜湊（工作檔 strings.json 不入版控）
```

## 5. 已被推翻的斷言

| 曾經寫過 | 實際 | 怎麼發現的 |
|---|---|---|
| `SPELLS.DAT` 是 LZW 段，解出 256 bytes | **不是壓縮檔**，是 192 bytes 明文。判準是段頭第二個 word 必須為 0，它是 `0x0180` | Go 版的位元流耗盡檢查比 Python 版嚴格，報「EOF 碼之前就用完」。Python 版超出範圍時補零繼續讀，湊出「長度剛好對」的假陽性 |
| `STR.DAT` 解密是每 byte −4，得到大寫單字表 | **是 +0x1C**，直接得到大小寫與標點都正確的可讀文字。−4 讓字母落在大寫 ASCII 範圍，看起來像對的，但句首字母會變成 `!`/`#`/`)` 這類符號 —— 那正是被誤讀的小數點：實際上是**小寫字母**沒被 +0x20 |
| 長文字是單字索引組出來的 | 不是。`STR.DAT` 就是完整的行文字，解密解對之後直接可讀。之所以會誤判成「單字索引」，是因為 −4 的錯誤解密把空格（`0x00`）留成分隔符，看起來像單字表 |
| 單獨把 `.OVL` 丟進 IDA 就能反組譯 | overlay 與 root 共用段基準，near call 會跨界，單獨載入時每個位移都算錯。要重建執行時佈局 | 1MENU2 的進入點第一件事就是 `call near ptr 0F052h`，目標在 overlay 自身長度之外 |

兩件事同源：**長度對得上不等於解對了**。任何「剛好整除／長度吻合」的證據，
都要再找一個獨立的驗證面（第二個實作、可見字串、render 出來的畫面）。

## 6. 工作紀律

- 動手前查 `~/.claude/rules/00-rules-index.md` 的觸發表，命中就先讀對應 rulebook。
- 每一輪：反組譯或實作 → 更新 markdown → 清掉被推翻的斷言 → commit + push → 更新本檔 §1。
- 宣告完成前重跑 `tools/go.sh test ./...`，並留下畫面證據。編譯成功不是視覺測試。
- 推論等級標籤：`已證實` / `強推論` / `假設` / `未知`。`已證實` 要附得出證據。
