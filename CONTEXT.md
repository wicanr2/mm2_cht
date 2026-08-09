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
**牆的判定已解出，第一人稱視角已經畫得出來**：走廊、側牆、透視地板、
遠處的正牆都到位，中文訊息疊在上面。事件腳本改成逐 opcode 執行，
六成五的事件格能顯示訊息。角色記錄已解，隊伍面板接上畫面。
全部 1,378 條文字翻譯完成。

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
| **可跑的雛形** | `cmd/mm2walk` 走固定路線輸出每步 PNG：載入原版地圖 → 移動 → 執行事件腳本 → 顯示中文 | — |
| 原版起始座標 | Middlegate (7,10) 面北，從神殿位置回推並驗證 | [`docs/formats/07`](docs/formats/07-event-script.md) §3 |
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
| **牆的判定** | 屬性層每格四個位元：bit 6/4/2/0 = 南/東/北/西，1 = 有牆。遮罩與位移量出自 `sub_1423E` + `sub_15E68` 的 `and 55h`；位元與方位的對應由「牆有兩面必須一致」定出（60 張地圖自洽率 93.8%，次高 86.7%，隨機 50%）。三重旁證：外圈 64 面全封閉、牆線圖是一座城、第一人稱畫得出來。見 [`docs/formats/06`](docs/formats/06-map.md) §4 |
| **第一人稱視角** | `internal/view/firstperson.go`。四個深度、正牆與左右側牆、透視地板。素材配置自證：側牆寬度累加 24→56→80→96 剛好等於同深度正牆的左緣。`cmd/mm2walk` 走一趟每步輸出 PNG |
| **事件腳本直譯器** | 位在 `2PLAY.OVL`。50 個 handler 的跳表（`jpt_1A676`）、腳本指標（`word_1042A`）、取字串程序（`sub_18FD0` + `sub_19016`）都已解出。`01`/`02`/`04` 是顯示字串（靠左／視窗／置中）。**EVENTSI 的 1,865 個事件格有 1,215 個（65.1%）能顯示訊息** |
| **overlay 位址換算** | `IDA linear = 檔案偏移 + 0xF800`；root `0x10000`–`0x12800`、level-1 `0x17E10`+、level-2 `0x1C130`+。追函式前先判斷屬於誰，否則會在根映像裡白找。見 [`docs/formats/01`](docs/formats/01-overlay-and-memory-layout.md) §2.5 |
| **角色記錄** | 130 bytes/筆。名稱、職業（0–5）、六個屬性、年齡、HP／上限、屬性的當前值都已定位。職業與屬性順序互相定錨：六個預設角色的屬性峰值全部落在自己職業該高的那一項。見 [`docs/formats/02`](docs/formats/02-data-files.md) §5 |
| **隊伍面板** | `internal/view/party.go`。右上角六人列表，名字走原版 8×8、職業走中文高解析層、HP 依比例變色 |
| **手冊掃描** | 93 頁整理成 [`docs/manual/part-1..5.md`](docs/manual/)。官方譯名（地名、人名、職業、屬性、法術、設施）、七項屬性表、八種職業、五種族、三陣營、城鎮地圖與圖例、96 條法術數值。手冊自身不一致處已逐項標出 |
| **座標系** | 第 0 列在**南**邊。手冊城鎮地圖的 y 軸由下往上，圖上設施位置與事件表的格編號在 y 上十處抽驗有八處完全相同。連帶把牆的位元改成 bit6=北，與 `sub_1423E` 的 `'N'`→`0xC0` 一致 |
| 事件表欄位語意 | 佈局已從 `sub_1A85C` 解出（見 [`docs/formats/02`](docs/formats/02-data-files.md) §4），59/71 段適用。`Cell` 是格位置已確定；`Index`（1 起算）與 `Kind`（高 nibble 類型）的語意未定 |
| 12 段不符合事件表佈局 | EVENTSI 8/44、EVENTSO 4/27，編號偏後。目前只抽字串，結構留未解 |
| 中英字級比例 | 英文走原版 8×8 放大 3 倍、中文走 24×24 點陣，像素密度是 3:1，英文看起來明顯較粗。可用但不協調 —— 要對照原版畫面決定是否改成 2 倍 + 16×16。**冬之魔就是在這一項上走了回頭路，不要等到全部翻完才處理** |
| **翻譯** | **1,378/1,378（100%）**。70 條刻意不翻（交織密碼的密文、座標表、切分碎片），政策寫在 [`translations/glossary.md`](translations/glossary.md)。中文字型烘到 1,916 字 |
| **訊息在地化** | 2,683 個事件格裡有 1,530 個會顯示訊息，這些訊息 100% 能換成中文。逐段查表 —— 一段腳本可能連續顯示好幾條字串，整段拿去查會有四分之一查不到 |
| `STR.DAT` 訊息索引 | 未解，導致翻譯的 key 粒度粗 —— 中間沒有空行的多個獨立訊息會被併成一條（`str.274` 從商店選單一路到片尾地址）。不影響譯文品質，但 remake 的文字層要自己切 |
| 密碼謎題 | 結局前的謎題是把美國憲法序言做成密文，翻成中文後原本的解法不成立，要單獨設計 |
| `MAP.DAT` 其餘位元 | 地形層的 tile 對應表未解；屬性層的高位元平面（bit 7/5/3/1）語意未定，門與觸發類型應該在那裡 |
| 第一人稱視角的細節 | 牆與地板已就位。火炬動畫（`TOWNT.16` 36 張）、天花板、右側角色面板還沒接。視圖區座標與原版截圖的逐像素對照也還沒做 |
| **事件腳本的其餘 opcode** | 50 個 handler 位址與長度已全部列出，語意解出三個顯示字串的（`01`/`02`/`04`）。長度表讓腳本能跳過未解的 opcode 繼續走，1,363 段裡 83.7% 剛好走到段尾。剩下要解的是 `0x2b`（232 段以它開頭，是條件分支）與長度仍有誤的 16%。見 [`docs/formats/07`](docs/formats/07-event-script.md) |
| **怪物表** | `MONSTERS.DAT` 已解：26 × 256。名稱每個位元組加了 `0x80`（所以 grep 怪物名找不到東西），減掉之後 256 筆全是乾淨 ASCII。末五筆是四元素領主與 Sheltem，排序與劇情一致。數值 12 bytes 未解。見 [`docs/formats/02`](docs/formats/02-data-files.md) §6 |
| `MONSTERS.16` RLE | 段內索引、動畫序列表、影像頭 x/y/w/h 已解，像素編碼未解 |
| **隨機數產生器** | `sub_11C88` 逐行重寫成 `internal/game/rng.go`：`+0x9248` → `ror 3` → `xor 0x9248` → `+0x11`，取 15 bit 後對 `hi-lo+1` 取模。播種用 DOS 時鐘。同種子同數列，所以隨機結果可以與原版一致、可重播 |
| **戰鬥流程骨架** | `internal/game/combat.go`。行動順序（速度高者先）、身體狀況五態、生命歸零→無意識→死亡的轉移、九個戰鬥指令。指令按鍵與 `2COMBAT.OVL` 的 `jpt_19573` 對得上（`sub ax, 46h` 之後 17 個 case），其中 `F`/`S`/`R`/`U`/`P` 五個已找到 handler |
| **戰鬥的數值層** | **未解**。命中與傷害公式沒從 `2COMBAT.OVL` 讀出來，怪物記錄那 12 個位元組的語意也未定 —— `+0x15` 值域 1–183 但不單調，不是單純的影像索引。骨架把數值收在 `Combatant` 介面後面，公式解出來換實作即可，流程不必動 |
| 角色記錄的 SP | `+88`/`+90`（uint16 目前／上限）。只有牧師與巫師非零，與手冊「遊俠與弓箭手要高經驗等級才有法力」一致 |
| 角色記錄的其餘欄位 | 手冊列出人物資料畫面 21 個欄位，記錄裡只定位了 7 個。運氣（第七項屬性）的位置未定 —— 屬性區與其第二份都只有六項 |
| 城鎮設施的互動 | 旅店、商店、神殿、訓練所的功能手冊都有說明，但對應的 opcode 語意未解，remake 目前只顯示訊息 |


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
docs/formats/07-event-script.md    事件腳本：50 個 opcode 的直譯器
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
| 牆是 2-bit 欄位（值 0 無牆 / 1 牆 / 2 另一種牆 / 3 門） | **是單一位元**。`and 55h` 只取每個欄位的低位元，高位元屬於另一個平面。當成 2-bit 欄位時自洽率 81%，改成單位元後 93.8% | 「值 3 = 城門」是看了牆線圖底列的四個就推廣的。寫成測試（門應該全在外圈）當場打掉：21 面裡只有 5 面在外圈，而且 56 面值 2/3 全部集中在南向 —— 因為 bit 7 同時是南向的高位元 |
| 用原版按鍵量到的 11 個通行樣本 | **不可信，已棄用**。任何位元／欄位／層的組合都與它零相關（值 0 五可三不可） | 判讀方式是比對 3D 視圖區的畫面差異，但踩到事件格會彈訊息吃掉後續按鍵 —— 畫面不動被記成「走不動」，而且一旦錯一次，後面整串都是錯的 |
| Middlegate 起點是 (7,10) | **待驗**。牆規則解出後 (7,9) 的北面是實牆，「面北走四步進神殿」那條路徑走不通，回推的前提有問題。神殿在 (7,6) 仍然確定（事件表 Index=4 的格 103） | 牆的自洽性檢驗與起點推導互相矛盾，而自洽性不靠 oracle，證據等級較高 |
| 單獨把 `.OVL` 丟進 IDA 就能反組譯 | overlay 與 root 共用段基準，near call 會跨界，單獨載入時每個位移都算錯。要重建執行時佈局 | 1MENU2 的進入點第一件事就是 `call near ptr 0F052h`，目標在 overlay 自身長度之外 |

前兩條同源：**長度對得上不等於解對了**。任何「剛好整除／長度吻合」的證據，
都要再找一個獨立的驗證面（第二個實作、可見字串、render 出來的畫面）。

牆那三條同源：**資料自己能檢驗的事，不要用 oracle 去猜**。牆有兩面、
城鎮要封閉、素材寬度要對得上 —— 這些約束不花成本、不會被操作失誤污染，
而且一次就把 192 種排列篩到只剩一種。用按鍵樣本回歸則耗了十幾輪，
最高只到 12/14，還推出三個錯結論。**先問「這批資料自己有沒有內部約束」，
再考慮去外面量。**

## 6. 工作紀律

- 動手前查 `~/.claude/rules/00-rules-index.md` 的觸發表，命中就先讀對應 rulebook。
- 每一輪：反組譯或實作 → 更新 markdown → 清掉被推翻的斷言 → commit + push → 更新本檔 §1。
- 宣告完成前重跑 `tools/go.sh test ./...`，並留下畫面證據。編譯成功不是視覺測試。
- 推論等級標籤：`已證實` / `強推論` / `假設` / `未知`。`已證實` 要附得出證據。
