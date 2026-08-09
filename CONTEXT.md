# CONTEXT — 專案脈絡與文件索引

這份是全專案的單一入口。對話被壓縮、或換一個新 session 接手時先讀這份，
再依索引跳到需要的文件。工作規範在 [`CLAUDE.md`](CLAUDE.md)。

最後更新：2026-08-09

---

## 1. 一句話現況

原版的壓縮、圖形、字型、道具表、事件字串已經解開並在 Go 上重實作；
中文化的垂直切片已經完整跑通 —— 原版素材 + 24×24 中文點陣 + 中英混排 +
`@` 換行都在畫面上驗證過。1,308 條字串已抽出，翻了 29 條。
下一步是地圖格式與第一人稱視角。

## 2. 已完成

| 領域 | 狀態 | 文件 |
|---|---|---|
| overlay 機制與記憶體佈局 | 14 個 overlay 全數反組譯，599 個函式 | [`docs/formats/01`](docs/formats/01-overlay-and-memory-layout.md) |
| LZW 壓縮 | 演算法讀自 `sub_12242`，全部資料檔可解 | [`docs/formats/03`](docs/formats/03-lzw-compression.md) |
| `.16` 圖形 | 26 個檔可解並 render，怪物圖的 RLE 除外 | [`docs/formats/04`](docs/formats/04-graphics.md) |
| `MM2.CH` 字型 | 8×8 × 128 字元，ASCII 對位驗證 | [`docs/formats/02`](docs/formats/02-data-files.md) §2 |
| `ITEMS.DAT` | stride 20 × 256 筆 | [`docs/formats/02`](docs/formats/02-data-files.md) §1 |
| `STR.DAT` | LZW + 每 byte −4，NUL 分隔單字表 | [`docs/formats/03`](docs/formats/03-lzw-compression.md) §4 |
| 事件字串 | 71 段全數抽出 1,308 條，零例外 | [`docs/formats/02`](docs/formats/02-data-files.md) §4 |
| 中文顯示 | 24×24 點陣、中英混排、`@` 換行、缺字檢查 | — |
| EGA 調色盤 | 原版標準 16 色，截圖 100% 落在表內 | [`docs/formats/04`](docs/formats/04-graphics.md) §1 |
| 原版 oracle | DOSBox headless + timeline 自動化，原版可跑 | — |
| Go 引擎骨架 | `lzw` / `gfx` / `font` / `render` / `events`，測試對照原版 | — |
| 中文化管線 | `mm2strings export/check`，版控只存譯文與原文雜湊 | — |

## 3. 進行中

| 項目 | 現況 |
|---|---|
| 事件表與腳本區 | 字串表的邊界（`FF FF` 標記）在 71 段上全部成立，字串已可抽出。標記之前是「3 bytes/筆的事件表（首位元組是格位置，遞增）+ `0xFF` 分隔的變長腳本區」，兩者的分界與欄位語意未定 —— 要讀 2PLAY/2CMDS 的反組譯，不要猜 |
| 中英字級比例 | 英文走原版 8×8 放大 3 倍、中文走 24×24 點陣，像素密度是 3:1，英文看起來明顯較粗。可用但不協調 —— 要對照原版畫面決定是否改成 2 倍 + 16×16。**冬之魔就是在這一項上走了回頭路，不要等到全部翻完才處理** |
| 翻譯進度 | 1,308 條中已翻 29 條（Middlegate 全段） |
| `MAP.DAT` 512 bytes 佈局 | 兩個 16×16 的 byte 層，高 nibble render 出可辨識的地形（草地／土路／山）。低 nibble 與第二層的語意未定 |
| `MONSTERS.16` RLE | 段內索引、動畫序列表、影像頭 x/y/w/h 已解，像素編碼未解 |
| `MASTER.16` 與地形圖 | 檔頭是第三種結構（uint16 offsets + 第二個陣列），未解。標題畫面就在裡面，有原版截圖可當 oracle 反推 |

## 4. 文件索引

```
CLAUDE.md                          工作規範、硬性原則、oracle 順序、工具鏈
docs/inventory.md                  63 個原版檔案 + 封存檔的 SHA-256
docs/formats/01-overlay-and-memory-layout.md
docs/formats/02-data-files.md      各資料檔的記錄結構
docs/formats/03-lzw-compression.md 壓縮與 STR.DAT 的位移層
docs/formats/04-graphics.md        .16 圖形
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
| 單獨把 `.OVL` 丟進 IDA 就能反組譯 | overlay 與 root 共用段基準，near call 會跨界，單獨載入時每個位移都算錯。要重建執行時佈局 | 1MENU2 的進入點第一件事就是 `call near ptr 0F052h`，目標在 overlay 自身長度之外 |

兩件事同源：**長度對得上不等於解對了**。任何「剛好整除／長度吻合」的證據，
都要再找一個獨立的驗證面（第二個實作、可見字串、render 出來的畫面）。

## 6. 工作紀律

- 動手前查 `~/.claude/rules/00-rules-index.md` 的觸發表，命中就先讀對應 rulebook。
- 每一輪：反組譯或實作 → 更新 markdown → 清掉被推翻的斷言 → commit + push → 更新本檔 §1。
- 宣告完成前重跑 `tools/go.sh test ./...`，並留下畫面證據。編譯成功不是視覺測試。
- 推論等級標籤：`已證實` / `強推論` / `假設` / `未知`。`已證實` 要附得出證據。
