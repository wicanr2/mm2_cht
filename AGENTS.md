# 執行規範：怎麼在這個專案裡工作

面向所有協作代理（Claude Code、Codex 及後續接手者）。
面向使用者、文件與提交訊息預設使用繁體中文；程式識別字、檔名、指令與 API 保留原文。

**這份文件只講「怎麼執行」。** 專案是什麼、已知什麼、踩過什麼坑在
[`CLAUDE.md`](CLAUDE.md)；現在做到哪在 [`CONTEXT.md`](CONTEXT.md) §1.5。
**接手時三份都要讀，順序是 `CLAUDE.md` → `CONTEXT.md` §1.5 → 本檔。**

---

## 1. 判斷優先序

規則或範圍有疑義時，依序處理：

1. 使用者在目前對話明確確認的決定。
2. 本檔的收尾閘門與 [`docs/release.md`](docs/release.md) 的發行決定。
3. 可重跑的目前程式、測試與原版 DOS 行為／截圖。
4. `CONTEXT.md` 的證據索引、`docs/formats/` 與 `docs/playtest/`。
5. `README.md`、`CLAUDE.md`、手冊與攻略。

原版素材的裁決順序（DOS 贏過其他平台）另見 `CLAUDE.md` §1。

**每次接手先自己重取事實，不要相信任何文件裡的日期快照：**
`git status --short`、`git rev-parse HEAD`、`docker ps -a`。
這份文件不記錄「上次跑到哪個 commit」——那種記錄一定會過期，
而過期的基準會讓人以為某件事還沒做。

`CONTEXT.md` §2「已完成」與 §3「進行中」是**證據索引**，按時間追加，
用來回查某個結論的出處。**目前狀態只看 §1.5**，不得把 §2／§3 的單一列當待辦。

---

## 2. Docker-only 工作規則

所有搜尋、大量資料分析、轉檔、建置、測試、DOSBox、IDA、GUI 與音訊工作
都只能在 Docker 容器中執行。主機端只做：`docker`、`git`、工作樹狀態檢查
（`ls`／`stat`／讀檔）與專案檔案編輯。不要在主機直接跑 Go、Python
或專案的工具腳本。

- 沿用既有 image：`mm2-go:latest`、`mm2-dosbox:latest`、`ida-pro-9.4-ver2`。
  先檢查 entrypoint／版本，不要因為啟動失敗就另造重複 image。
- 一次性工作用 `docker run --rm --network none`，設定相稱的 `--memory`、`--cpus`、
  `--pids-limit`，並加上 `--log-opt max-size=10m --log-opt max-file=3`
  （daemon 預設的 `json-file` 沒有 rotation）。只有明確需要網路才開網路。
- 可寫容器一律以 `-u "$(id -u):$(id -g)"` 執行。原版、壓縮檔、解包來源與
  IDA license 一律唯讀掛載；只把明確的輸出目錄或工作樹設為可寫。
- 寫入前先用容器內 `stat`／`ls -ldn` 確認目標 UID/GID，完成後抽查產物。
  遇到 root-owned 檔案只修正已確認的單一檔案或輸出目錄，禁止遞迴 `chown`。
- `tools/go.sh`、`tools/dosbox_run.sh`、`tools/ida.sh` 是容器設定的**參考來源**；
  依上述主機限制，以等價的 `docker run` 直接啟動既有 image。
- **禁止** `docker image/system/volume/builder/container/network prune` 與 `docker rmi`。
  這台機器同時放著多個客戶專案的 image。每批工作後跑
  `docker ps -a --filter ancestor=<image>`；因為一律 `--rm`，結果應為空。

### Go 回歸

**快取用 bind mount 到自己可寫的目錄，不要用具名 volume。**
docker 建立具名 volume 時擁有者是 root，容器以 `-u $(id -u)` 執行就寫不進去，
症狀是 `mkdir /gocache/00: permission denied` 或 `mkdir /gomod/cache: permission denied`
—— 看起來像 Go 壞了，其實是掛載權限。

```bash
CACHE=/tmp/mm2-cache; mkdir -p "$CACHE/gomod" "$CACHE/gocache"

docker run --rm --network none --memory 4g --cpus 2 --pids-limit 512 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" \
  -e HOME=/tmp -e GOCACHE=/gocache -e GOMODCACHE=/gomod \
  -v "$(pwd):/src" -v "$CACHE/gomod:/gomod" -v "$CACHE/gocache:/gocache" \
  -w /src mm2-go:latest go test ./...
```

模組快取是空的時候，`--network none` 會讓 `cmd/mm2`（唯一需要抓 ebiten 的套件）
**setup failed**，其餘套件照樣全綠。那是環境不是產品：先拿掉 `--network none`
跑一次 `go mod download`（**不要用 `go mod download all`，它會改寫 `go.sum`**），
再離線重跑測試。

UI 測試會寫 `workplace/gfx/ui/*.png` 與暫存存檔，**工作樹唯讀掛載時必然失敗**。
同樣是驗證環境條件不是產品缺陷。

### 釋出檢查

```bash
docker run --rm --network none --memory 768m --cpus 1 --pids-limit 128 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$(id -u):$(id -g)" -e HOME=/tmp -v "$(pwd):/src:ro" -w /src \
  mm2-go:latest bash tools/check_release.sh
```

在公開用的新 repo 才把最後一行改成 `bash tools/check_release.sh --public`。
腳本回報的份數以**當次執行結果**為準，不要照抄任何文件裡寫死的數字。

---

## 3. 完成的定義

把三種狀態明確分開，避免無限研究或過度宣稱：

| 狀態 | 定義與處置 |
|---|---|
| remake 已完成 | 目前選定的正常玩家路徑能由自備原版資料走完，資料→規則→UI→存檔垂直鏈可驗證。 |
| 原版 oracle 未知 | 未證實欄位、深度視覺差異、非玩家阻塞的原始行為，只能標為「強推論／假設／未知」，不可自動變成規則或完成阻擋項。 |
| 可選 polish | 多平台素材深化、現代美術、額外攻略／地圖呈現、非必要精細對照；除非直接影響目前玩家路徑或使用者選定的發行範圍，不能搶走收尾工作。 |

宣稱「可交付」前，至少完成並記錄下列最小充分抽樣：

1. 不用傳送、發道具、強制勝利或其他 debug hook 的正常路徑：建角或載入隨附起始
   狀態、移動、進出設施、取得／裝備／使用物品、戰鬥、休息或治療、存檔、離開、重載。
2. 一條有文件化 fixture 的晚期／高風險狀態轉換；兩條主要戰鬥或 UI 分支都要抽測。
3. 至少一組 DOS EGA 的 exact-state／相鄰-state 畫面對照。中文重排是刻意的介面設計，
   必須標明它不是 pixel-perfect 還原，不能拿相鄰場景冒充逐像素相同。
4. 完整 Go 測試、適用的建置／封裝 smoke，以及正確情境的釋出 deny-list 檢查。
5. 交接時寫下 HEAD、測試命令與結果、已證實事實、仍未知假說、下一個最小可重現動作，
   並確認沒有遺留本專案的容器。

**資料表、孤立 helper、反組譯覆蓋率、單一 debug 入口或綠色單元測試都不是完成證據。**
原版相關發現只有在改變上述玩家路徑或交付 gate 時，才開新的窄任務。

---

## 4. 需要使用者決定的閘門

下列情況必須先由使用者決定，不能把個人偏好、沉默或既有 prototype 當授權：

- 改變完成範圍、原版忠實度、中文排版／字型、theme、平台支援、存檔格式、
  資料 schema、授權或公開方式。
- 把未證實的原版行為做成玩家可見規則，或為英文文字遊戲、中文謎題、
  攻略提示設計新解法。
- 破壞性 Git／檔案操作、公開／推送／force-push、建立公開 repo 或發行套件。
- **改寫 git 歷史**（`filter-repo`、force push）。既定決定是私有 repo 不改寫、
  公開時另建乾淨 repo；要變更這個策略需要新的明確授權。

先查證能查證的事實；真正遇到這類前沿時，每次只問一題，說明已知證據、
為何現在阻塞、可行選項、建議及對忠實度／維護／發行的影響。
使用者確認後，重述採用與排除的選項，再實作依賴它的分支。
視覺或操作取捨先做可丟棄的對照截圖／prototype。

**授權是逐項的**：「可以拉映像」不含「可以清映像」，
「可以用 IDA」不含「可以改發行策略」。沒有明確涵蓋的就是沒授權。

---

## 5. 逆向與證據的記錄方式

- DOS `MM2.EXE` 與 14 個 overlay 的實際行為是規則 oracle；MSX、Amiga、Mega Drive、
  手冊與攻略只作交叉證據。**不要把非 DOS 的差異默默改成 DOS 規則。**
- 需要反組譯時優先用指定的 IDA Pro 9.4 image。Ghidra、objdump 等只能交叉驗證，
  不能取代 IDA 的 `.i64` xref、函式邊界與資料流。
- **動手前先查 [`docs/re/00-function-index.md`](docs/re/00-function-index.md)。**
  解完一支函式後重跑 `tools/gen_func_index.py`（容器內）更新索引。
- 對函式、全域、位址與相對運算元採**非破壞性註記**：保留原始名稱、線性位址、
  段:偏移、bytes／偏移與位址空間；另附語意、`已證實／強推論／假設／未知` 與證據來源。
  不以推測性的重新命名覆蓋原始定位資訊。
- 每份證據記錄輸入檔名與雜湊、工具版本、位址基準。直接 xref 沒有寫入端時，
  要追查指標／暫存器／相對基址的間接寫入，**不能宣稱「沒有寫入」**。
- 資料格式能整除、單張 render、單一字串命中或單元測試通過，都只是**一項**證據；
  至少再找一個獨立的驗證面。未知欄位原樣往返，不為了填表發明玩法。
- milestone 真的改變現況時才更新 `CONTEXT.md` §1.5、對應的 format／playtest 文件
  與推翻表；不要把暫時的想法寫成已定案規格。小型檢查不必製造文件噪音。

---

## 6. 翻譯與外部工具

- 既有譯文與字形流程以 `translations/zh-Hant.json`、`cmd/mm2strings`、
  `tools/build_cjk_font.py` 為準。原文資料與由原版抽出的內容不得進版控。
- 所有新的玩家可見中文都要走既有 key 與字型缺字檢查，**不要把中文硬編碼進 Go**。
  新字沒烘進 atlas 時畫面會缺字而**不報錯**，所以加字後要重烘並檢查。
- **引入任何自動翻譯工具前**，先證明它的實際來源、輸入／輸出 schema、授權、
  可重現性，以及與既有 key／雜湊／術語表的相容性。不得自動覆寫已驗收的譯文；
  最多先做不入版控、可丟棄的樣本比較，由使用者決定是否擴大範圍。
  來源不明的工具不要為它另開實作分支。

---

## 7. 每輪最小流程

1. 讀 `CLAUDE.md` → `CONTEXT.md` §1.5 → 本檔；確認工作樹、HEAD、容器狀態。
2. 查 `~/.claude/rules/00-rules-index.md` 的觸發表，命中就先讀對應 rulebook。
3. 只選一條能改善目前交付 gate 的垂直鏈，列出需要的原版證據與可重跑的驗證方式。
4. 在隔離容器中實作／測試；同時抽測一般與替代分支，確認存檔與正常 UI 路徑。
5. 分清產品失敗、環境／腳本失敗與 oracle 未知；修正環境後用同一容器命令重跑。
6. 真正完成且工作樹／容器狀態已確認時，才更新文件、提交，或請使用者授權推送。
   **完成的同時就更新 `CONTEXT.md` §1.5**，不要留給下一輪整理。
