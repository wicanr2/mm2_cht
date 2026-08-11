# 《魔法門 II》繁體中文 remake：接手與收尾規範

本檔是本專案後續工作階段的操作入口。面向使用者、文件與提交訊息預設使用繁體中文；
程式識別字、檔名、指令與 API 保留原文。

## 專案目標與裁決順序

以玩家自備的合法 DOS 版《Might and Magic II》資料為 oracle，在 Go／Ebiten 中維護
可玩的繁體中文 remake。公開產物只能包含引擎、原創內容與翻譯文本，絕不包含原版
可執行檔、資料、美術、音樂、掃描、解包檔或由原版抽出的表。

判斷規則與範圍時，依下列優先序處理：

1. 使用者在目前對話明確確認的決定。
2. 本檔的收尾閘門與既有的 `docs/release.md` 發行決定。
3. 可重跑的目前程式、測試與原版 DOS 行為／截圖。
4. `CONTEXT.md` 的證據索引、`docs/formats/` 與 `docs/playtest/`。
5. `README.md`、`CLAUDE.md`、手冊與攻略。

`CONTEXT.md` 是重要索引，但其「進行中」表包含按時間追加的歷史列，部分舊列已被
同表後段、程式碼或測試推翻。**不得把單一舊列當目前待辦。** 重開任何研究前，先以
目前程式、最近提交、對應測試與原版證據稽核；推翻紀錄保留在 `CONTEXT.md`，不要改寫
歷史讓錯誤原因消失。

## 接手基準（2026-08-11）

- 工作樹在基準檢查後乾淨；每次接手仍要先以 `git status --short` 與
  `git rev-parse HEAD` 重取事實，不假設本段的 commit 仍是最新。
- `mm2-go:latest` 的完整 `go test ./...` 已通過。這證明 remake 內部回歸一致，
  **不等同**原版 parity、正常可玩路徑或封裝完成。
- UI 測試會產生 `workplace/gfx/ui/*.png` 與暫存存檔；測試原始碼掛成唯讀時必然失敗。
  這是驗證環境條件，不是產品缺陷。重跑時可寫掛載僅限本工作樹，且容器必須以目前
  UID/GID 執行；基準產物已驗證為 UID/GID `1000:1000`。
- 私有工作 repo 的 `tools/check_release.sh` 已通過。它會報告歷史中殘留的
  `data/creation.json`、`experience.json`、`pictures.json`、`terrain.json`、
  `traps.json`；此處的 5 份是腳本目前量到的事實，較舊文件寫「4 份」時不得照抄。
- **既定發行決定：** 私有 repo 不改寫歷史；公開時另建乾淨 repo，並在那份 repo
  執行 `tools/check_release.sh --public`。未取得使用者新的明確決定，不得 force-push、
  `filter-repo`、改寫歷史或改變此發行策略。

## 收尾的完成定義

把下列三種狀態明確分開，避免無限研究或過度宣稱：

| 狀態 | 定義與處置 |
|---|---|
| remake 已完成 | 目前選定的正常玩家路徑能由自備原版資料走完，資料→規則→UI→存檔垂直鏈可驗證。 |
| 原版 oracle 未知 | 未證實欄位、深度視覺差異、非玩家阻塞的原始行為，只能標為「強推論／假設／未知」，不可自動變成規則或完成阻擋項。 |
| 可選 polish | 多平台素材深化、現代美術、額外攻略／地圖呈現、非必要精細對照；除非直接影響目前玩家路徑或使用者選定發行範圍，不能搶走收尾工作。 |

在宣稱「可交付」前，至少完成且記錄下列最小充分抽樣：

1. 不用傳送、發道具、強制勝利或其他 debug hook 的正常路徑：建角或載入隨附起始
   狀態、移動、進出設施、取得／裝備／使用物品、戰鬥、休息或治療、存檔、離開、重載。
2. 一條有文件化 fixture 的晚期／高風險狀態轉換；兩條主要戰鬥或 UI 分支都要抽測。
3. 至少一組 DOS EGA 的 exact-state／相鄰-state 畫面對照。中文重排是刻意的介面設計，
   必須標明它不是 pixel-perfect 還原，不能拿相鄰場景冒充逐像素相同。
4. 完整 Go 測試、適用的建置／封裝 smoke，以及正確情境的釋出 deny-list 檢查。
5. 交接時寫下 HEAD、測試命令與結果、已證實事實、仍未知假說、下一個最小可重現動作，
   並確認沒有遺留本專案容器。

原版相關發現只有在改變上述玩家路徑或交付 gate 時才開新窄任務。資料表、孤立 helper、
反組譯覆蓋率、單一 debug 入口或綠色單元測試都不是完成證據。

## 共同決策閘門

下列情況必須先由使用者決定，不能把個人偏好、沉默或既有 prototype 當授權：

- 改變完成範圍、原版忠實度、中文排版／字型、theme、平台支援、存檔格式、資料 schema、
  授權或公開方式。
- 把未證實原版行為做成玩家可見規則，或為英文文字遊戲、中文謎題、攻略提示設計新解法。
- 破壞性 Git／檔案操作、公開／推送／force-push、建立公開 repo 或發行套件。

先查證能查證的事實；真正遇到此類前沿時，每次只問一題，說明已知證據、為何現在阻塞、
可行選項、建議及對忠實度／維護／發行的影響。使用者確認後，重述採用與排除的選項，再
實作依賴它的分支。視覺或操作取捨先做可丟棄對照截圖／prototype。

## Docker-only 工作規則

所有搜尋、大量資料分析、轉檔、建置、測試、DOSBox、IDA、GUI 與音訊工作都只能在
Docker 容器中執行。主機控制面僅能使用 `docker`、`git`、工作樹狀態檢查與 `apply_patch`
的專案檔案編輯；不要在主機直接執行 Go、Python、Bash、DOSBox 或專案工具腳本。

- 優先沿用已存在的 `mm2-go:latest`、`mm2-dosbox:latest` 與指定的 IDA Pro 9.4 映像；
  先檢查 entrypoint／版本，不能因啟動失敗另造重複 image。
- 一次性工作使用 `docker run --rm --network none`，設定相稱的 `--memory`、`--cpus`、
  `--pids-limit`，並加上 `--log-opt max-size=10m --log-opt max-file=3`。只有明確需要網路
  才能開網路。
- 可寫容器一律以 `-u "$(id -u):$(id -g)"` 執行。原版、壓縮檔、解包來源與 IDA license
  一律唯讀掛載；只把明確的輸出目錄、測試輸出或工作樹設為可寫。
- 寫入前先用容器內 `stat`／`ls -ldn` 確認目標與輸出目錄的 UID/GID；完成後抽查產物。
  遇到 root-owned 檔案只修正已確認的單一檔案或輸出目錄，禁止遞迴 `chown`。
- `tools/go.sh` 與 `tools/dosbox_run.sh` 是容器設定的參考來源；依本檔的主機限制，
  不要直接在主機執行它們。以等價的 `docker run` 直接啟動既有映像。
- 禁止 `docker image/system/volume/builder/container/network prune`、`docker rmi` 與任何全域
  清理。每批工作後執行適用的 `docker ps -a --filter ancestor=<image>`；因為容器必須
  `--rm`，結果應為空。

### 已驗證的 Go 回歸方式

先確認具名快取 volume 可由目前 UID/GID 寫入，再從專案根目錄以可寫工作樹掛載執行：

```bash
docker run --rm --network none --memory 4g --cpus 2 --pids-limit 512 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" \
  -e HOME=/tmp -e GOCACHE=/gocache -e GOMODCACHE=/gomod \
  -v "$(pwd):/src" -v mm2-gomod:/gomod -v mm2-gobuild:/gocache \
  -w /src mm2-go:latest go test ./...
```

若測試先因唯讀工作樹失敗，標為驗證環境問題，確認輸出目錄權限後用同一映像與命令
乾淨重跑；不要把它寫成產品缺陷。釋出檢查可將 `/src` 唯讀掛載：

```bash
docker run --rm --network none --memory 768m --cpus 1 --pids-limit 128 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp -v "$(pwd):/src:ro" -w /src \
  mm2-go:latest bash tools/check_release.sh
```

在公開用的新 repo 才將最後一行改為 `bash tools/check_release.sh --public`。

## 逆向、資料與文件契約

- DOS `MM2.EXE` 與 14 個 overlay 的實際行為是規則 oracle；MSX、Amiga、Mega Drive、
  手冊與攻略只作交叉證據。不要把非 DOS 差異默默改成 DOS 規則。
- 若需要反組譯，優先使用指定 IDA Pro 9.4 image。Ghidra、objdump 等只能交叉驗證，
  不能取代 IDA 的 `.i64` xref、函式邊界與資料流。
- 對函式、全域、地址與相對運算元採非破壞性註記：保留原始名稱、線性地址、段：偏移、
  bytes／偏移與位址空間；另附語意、`已證實／強推論／假設／未知` 與證據來源。
  不以推測性重新命名覆蓋原始定位資訊。
- 每份證據記錄輸入檔名與雜湊、工具版本、地址基準。直接 xref 沒有寫入端時，要追查
  指標／暫存器／相對基址的間接寫入，不能宣稱「沒有寫入」。
- 資料格式能整除、單張 render、單一字串命中或單元測試通過，都只是一項證據；至少再找
  一個獨立驗證面。未知欄位原樣往返，不為了填表發明玩法。
- 一個 milestone 真的改變現況時才更新 `CONTEXT.md`、相應 format／playtest 文件與推翻表；
  不要把暫時想法寫成已定案規格。小型檢查不必製造文件噪音。

## 翻譯與外部工具

- 既有譯文與字形流程以 `translations/zh-Hant.json`、`cmd/mm2strings`、
  `tools/build_cjk_font.py` 為準。原文資料與由原版抽出的內容不得進版控。
- 2026-08-11 的唯讀稽核確認工作樹與 Git 歷史都沒有 `translate.js`（也沒有其他
  JavaScript、`package.json` 或 Node 整合點）。`mm2strings check` 對目前原版資料的結果為
  2,695 條、未收錄 0、原文不符 0、未翻譯 0；因此目前**不採用**未提供來源的
  `translate.js`，不要為不存在的工具另開實作分支。
- 引入 `translate.js` 或任何自動翻譯工具前，先證明它的實際來源、輸入／輸出 schema、
  授權、可重現性與與既有 key／雜湊／術語表的相容性。不得自動覆寫已驗收譯文；最多先做
  不入版控、可丟棄的樣本比較，並由使用者決定是否擴大範圍。
- 所有新玩家可見中文需走既有 key 與字型缺字檢查；不要把中文硬編碼到 Go 原始碼。

## 每輪最小流程

1. 讀本檔，再讀 `CONTEXT.md` 的相關證據與最新提交；先確認工作樹、HEAD、容器狀態。
2. 只選一條能改善目前交付 gate 的垂直鏈，列出需要的原版證據與可重跑驗證。
3. 在隔離容器中實作／測試；同時抽測一般與替代分支，確認存檔與正常 UI 路徑。
4. 分清產品失敗、環境／腳本失敗與 oracle 未知；修正環境後用同一容器命令重跑。
5. 僅在真正完成且工作樹／容器狀態已確認時，更新相應文件、提交或請使用者授權推送。

下一個不依賴產品決策的優先工作是：以正常玩家路徑完成最小的「建角／移動／設施／
物品或戰鬥／存檔重載」證據鏈，並補一條已文件化的晚期 fixture；先確認玩家阻塞問題，
再討論多平台素材、視覺 polish 或公開發行。
