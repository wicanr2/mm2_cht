# 工作紀錄

## 2026-11-11：Amiga HD、工具列與簡體中文

- 使用者確認手繪重製與新版 HD 邊框，授權開工。定稿與 READY 契約已存 [規格](docs/spec/amiga-hd-toolbar.md)，現況入口已加入 `CONTEXT.md` §1.5。
- 開工 HEAD：`9f0f4cbb246185a3fa2edd006fea9339d4134136`。保留既存的兩張 `--list/SKY_*.png` 刪除狀態與未追蹤的私人文字檔。
- 原版、mm3 與 mm45 以唯讀掛載查證。文件列出的 mm2-go／mm2-pil 映像不存在，沿用已存在的 `san1-go:1.26.7-ebiten2.9.9-r1` 與 `sc-cht-hdtools:pillow11.3.0-r2`；Go 使用非登入 shell，以保留映像的 PATH。
- 開工時尚未完成實作、測試與遊戲內驗收。

### 完成結果

- 頂端浮出工具列接到正常前端，繪製與滑鼠命中共用縮放轉換。支援繁簡、遊戲字級、素材平台、原圖／HD、設定及三種工具列字級；拖出放開與 Esc 由工具列持有。
- 簡中由鎖版 OpenCC 0.1.7 另行產生，原有繁中譯文不變。執行時只在文字出口轉換，語意判斷與存檔仍使用原值。新增 Noto 繁簡與英文字型、授權及 `translations/display-build.json`；2,244 個舊字形全部位元相同，只補 U+2190。
- HD 完整圖包在 `workplace/hd-amiga-r1/`。400 張 Amiga 場景、530 幀怪物，73 張生成拼圖，913 張獨立成品對應 930 筆。所有圖保留來源／成品雜湊及原尺寸，載入前整包驗證。生成、切分、場景遮擋、怪物播放與邊框合成已接通。
- 新邊框由已確認定稿拆出材質與開口；角色、生命、日期與訊息均取目前遊戲狀態。火炬重製為透明圖層；`sky-001` 依既有 `drawSky` 的天花板用途改成石砌材質。初版地面與第一組怪物的拼圖佔比不足，已保留舊母圖後重製。
- `hd.json` 最終 SHA-256：`81388fbf0260afa7afb1fa9ac2015be836b38d3299a2cc8cbc353726b2147875`。`asset-audit.json` 的來源／成品雜湊及尺寸檢查全部通過；從母圖 `repack` 後 930 筆成品雜湊全部相同，見 `repack-check.json`。
- `sky-00`、`town-00`、`town-01`、`townf-00` 的早期生成只有母圖與來源定位，沒有完整 prompt 檔。其餘目前母圖已保存 prompt。可由母圖重建成品，不宣稱可逐次重新生成相同母圖。

### 驗證

全程 Docker，工作樹與輸出以 UID/GID 1000:1000 寫入；原版資料唯讀。Go 使用 `--init`、Xvfb 與現有 bind 快取 `workplace/cache/gomod`／`gocache`。下列命令均在上述 Go 映像的 `/src` 執行，完整輸出位於 `workplace/hd-amiga-r1/`。

| 命令或檢查 | 結果與收據 |
|---|---|
| `xvfb-run -a go test ./...` | 通過，`go-test-final.log`；UI 套件 97.947 秒。 |
| `go vet ./...` | 通過，`go-vet-final.log` 為空。 |
| `go build -o workplace/bin/mm2 ./cmd/mm2` | 通過，可執行檔留在本機。 |
| `go test -count=1 ./internal/ui -run 'Test(HD|Presentation|AmigaMenu|Toolbar)'` | 最終怪物更新後通過，20.501 秒；實際容器命令經 Xvfb 執行，日誌 `display-final-art-test.log`。同次列出的 `internal/assets/hd` 因篩選名稱沒有執行測試；它的完整反向測試已包含在全套回歸。 |
| `bash tools/package_container.sh public linux-x64 /src/workplace/hd-amiga-r1/package-final` | 舞台目錄建置與資源 allow-list 通過；新增字型、繁簡資料及授權均已帶入。沒有產生正式 AppImage／Release。 |
| `xvfb-run -a bash tools/check_display_gui.sh hd /src/workplace/hd-amiga-r1/gui-hd-r3` | 通過，使用舞台目錄的執行檔、字型與翻譯。原圖／HD指標切換、繁簡、設定、物品、商店、步行進神殿再離開、80% 遊戲字級、150% 工具列與存讀檔全部成功。 |
| 同腳本的 `original` 分支，輸出 `gui-original-final/` | 通過相同正常操作路徑。兩分支都核對最後位於地圖 0、(7,4)、面北；重載後存檔 JSON 完全相同。 |
| 一般／快速戰鬥 | 使用既有戰鬥 fixture，種子在執行前固定 `0x1234`。原圖與 HD 的相同動畫呼叫、操作、結算狀態、寶箱及模式一致；不是 DOS oracle 比對。 |
| `bash tools/check_release.sh` | 目前私有工作 repo 情境通過，`release-check-final.log`。公開舞台資源檢查不代表本 repo 的完整歷史可直接公開。 |
| `git diff --check` | 通過。 |

GUI 腳本可透過 `MM2_BINARY`、`MM2_RESOURCE_ROOT` 指定舞台目錄；本輪指向 `package-final/.stage/mm2-cht-linux-x64-public-9f0f4cbb2461/`。各輪輸出使用全新目錄，存檔只寫入其 `run/save/`，未覆寫使用者存檔。

重跑 Go 驗證的容器設定：

```bash
timeout 300s docker run --rm --init --network none --memory 3g --cpus 2 --pids-limit 512 \
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)" \
  --mount type=bind,src=/home/anr2/cht/mm2,dst=/src \
  --mount type=bind,src=/home/anr2/cht/mm2/workplace/orig,dst=/src/workplace/orig,readonly \
  --mount type=bind,src=/home/anr2/cht/mm2/workplace/amiga,dst=/src/workplace/amiga,readonly \
  --mount type=bind,src=/home/anr2/cht/mm2/workplace/amiga-monsters,dst=/src/workplace/amiga-monsters,readonly \
  -e HOME=/tmp -e GOMODCACHE=/src/workplace/cache/gomod \
  -e GOCACHE=/src/workplace/cache/gocache -e GOMAXPROCS=2 \
  -w /src san1-go:1.26.7-ebiten2.9.9-r1 xvfb-run -a go test ./...
```

素材重建在 `sc-cht-hdtools:pillow11.3.0-r2` 容器內執行 `python3 tools/prepare_hd_amiga.py repack`。將專案唯讀掛入 `/src`，只將已存在的 `workplace/hd-amiga-r1/` 再以可寫掛入同一路徑；保留前述使用者、逾時與資源限制。這會使用已封存母圖，無須網路或重新生成。

### 環境與判準修正

- Xvfb 容器未加 `--init` 時，PID 1 的訊號行為造成等待；修正後使用相同映像乾淨重跑。沒有把它列為遊戲缺陷。
- GUI 的早期指標座標、關窗按鍵與腳本更新期間的讀取失敗，均屬驗證腳本問題。最後兩個分支從固定腳本與全新目錄重跑通過。
- 既有怪物動畫會消耗遊戲亂數，因此撤回「呼叫 Draw 後狀態完全不動」的錯誤測試前提。改以執行前固定種子、相同初始狀態及等量動畫呼叫，比較原圖與 HD 結果；沒有改動原亂數規則。

### 界線與收尾

- 本輪完成工具列、簡中、第一人稱與怪物 HD、新版邊框。Amiga `book`／`intro` 等未接入既有引擎的素材只保存 HD 檔及索引；片頭仍用 DOS 動畫，世界地圖仍用已驗收掃描圖。`globe.32` 未支援，不在 930 筆分母內。
- Windows／macOS 真機與既有原版 oracle 未知沿用原狀。本輪沒有新增遊戲規則、存檔 schema、公開授權、提交、推送或正式封包，也沒有覆寫 `dist-all/`。
- 收尾 HEAD 仍為 `9f0f4cbb246185a3fa2edd006fea9339d4134136`。原有刪檔與私人文字檔狀態保留。
- 修改與新增檔案沒有 root 擁有權。全工作根自檢找到 373 筆既有 root-owned 快取／DOSBox 產物，未改動。Go 驗證容器已清空；素材映像只剩其他專案的 `san5-dosgolem-concept`，未停止。
- 下一個最小動作：從專案工作目錄啟動 `workplace/bin/mm2`，移到頂端切換繁簡與原圖／HD。實機預覽在 `workplace/hd-amiga-r1/showcase-left.png`；完整驗收索引在 [顯示規格](docs/spec/amiga-hd-toolbar.md)。

## 2026-11-11：繁簡輸入與 Unicode 名冊

- 使用者要求繼續，並補上簡體輸入。查證後，使用者明確選擇完整支援姓名與繁簡答案，授權相容舊檔的 Unicode 名冊資料。契約及驗收已更新於 [輸入規格](docs/spec/amiga-hd-toolbar.md#簡體輸入擴充)。
- 本輪路由命中 `local/localization-display-semantic-isolation.md` 與既有 spec 閘門。沿用 `reverse-engineer-retro-game-remake`、`grilling`；別名只在輸入邊界轉成原版識別值，未讓顯示譯文參與遊戲比對。
- 初步認為名字會被十個 bytes 截斷；可丟棄探針與實際程式顯示更早的根因：`Character.Empty` 直接把所有非 ASCII 姓名判為空槽。另確認原 JSON 只保存世界狀態，沒有角色名冊，`Restore` 也未讀回相容記錄。前一判斷不完整，已訂正規格與現況。

### 實作

- `exp/textinput.Field` 接到前端，分開已提交文字與組字預覽。新增繁簡輸入字型各 29,246 字，只補現有字型缺少的字；字型範圍、來源與 SHA-256 在 `translations/input-font-build.json`。
- `translations/input-aliases.tsv` 保存 KEYS、DRUIDS、MEENU 與四道數字題的繁簡答案，沿用已驗收譯名。支援全形英數，錯誤與跨題別名不會被當成正解。動態控制室密碼仍使用畫面上的字母代碼。
- 第 3 版 JSON 保存完整隊伍、名冊、UTF-8 姓名與原始 `Raw`，仍讀取第 1／2 版。原版記錄保持 130 bytes；Unicode 姓名輸出相容欄位時代以 `PLAYER`。新角色初始化原版記錄，原版欄位、學法術、物品增減重新解析後均保留 Unicode 姓名。存檔改為暫存檔寫完後原子替換，寫入失敗會回報。
- 實機發現 N 鍵先被「拒絕」綁定吞掉，修正為由目前模式接受的動作處理，新增前端回歸測試。
- 實機另發現 Linux GLFW 會將 XIM 已消耗的 Enter、Esc、退格仍送進遊戲。`go.mod`／`go.sum` 版本不變，來源固定於 `vendor/` 並保留上游授權。`tools/patch_ebiten_ime.py` 綁定原始 SHA-256，只修補已消耗的編輯鍵，保留未消耗事件的後續轉送與去重。
- 修補來源 `x11_window_linbsd.c` 原始 SHA-256：`eaba4f2e10664d997a32386f1dfdf79adbce5b2c51e57b2f466ef2d334c00270`；最終 SHA-256：`149f76078c8b284afab13c8706d59b2d2243d5e888172f80ac6e375a9228d623`。共享模組快取未改動，重跑工具會驗證完整雜湊。

### 驗證與交付範圍

| 檢查 | 結果與本機收據 |
|---|---|
| 完整回歸 | `xvfb-run -a go test -timeout 360s ./...` 通過，UI 95.659 秒，`input-go-test-final.log`。在容器 `/tmp/mm2-input-regression` 的完整來源副本執行，包含 vendor；測試存檔不觸碰使用者工作根。 |
| 靜態檢查 | `go vet ./...` 通過，`input-go-vet.log` 為空。 |
| 中文姓名 | UI 建角、繁簡、十字上限、退格、編組、Raw 欄位／法術／物品更新、存讀檔與破損快照反向測試通過。沒有以原版 parser 放寬 ASCII 槽位來接受垃圾資料。 |
| 答案 | 繁簡、全形、格式、UTF-8、錯誤與跨題比對，以及既有 KEYS 事件續跑通過。遊戲層仍收到原版英數識別值。 |
| 真實拼音 | IBus/libpinyin 正常建角輸入 `测试long`，第一個 Enter 只確認組字，第二個 Enter 才建立角色。組字退格不刪已提交文字，Esc 不退出命名。新行程重載後 JSON 相同，收據 `gui-input-edit-final/checks.json`，畫面 `04-ime-confirm.png`。 |
| 建置 | Linux `workplace/bin/mm2`、Windows x64 `workplace/bin/mm2-input-smoke.exe` 編譯成功；Windows 產物只作建置抽測，未真機啟動。 |
| 封裝資源 | `tools/package_container.sh public linux-x64 /src/workplace/hd-amiga-r1/package-input-final` 通過，新輸入字型、別名、收據及授權均存在。只建立舞台目錄，未新增正式封包。 |
| 正常操作 | 舞台執行檔及資源實跑探索、繁簡、原圖／HD、設定、物品、商店、步行進出神殿、字級、存檔與新行程重載，`gui-input-normal-r2/checks.json` 全通過。 |
| 釋出檢查 | 目前私有工作 repo 情境通過，`input-release-check.log`；沒有新公開、推送或發行聲明。 |

Go 驗證使用上一節同一容器設定，記憶體 4 GiB、外層逾時 600 秒，專案來源先複製到容器暫存目錄，`workplace` 的資料與既有輸出以連結沿用。原始資料仍唯讀，快取仍是 UID/GID 1000:1000 的 bind mount。

真實輸入法另使用 `mm2-input-test:20261111`，來源為 `tools/Dockerfile.input-test`，從既有 GUI 映像增加非 root D-Bus 與鎖版輸入法。版本依 [Debian ibus](https://packages.debian.org/bookworm/ibus) 1.5.27-5、[Debian ibus-libpinyin](https://packages.debian.org/bookworm/ibus-libpinyin) 1.15.1-1；套件清單留在映像 `/usr/local/share/mm2-input-test-packages.tsv`。容器內命令為 `dbus-run-session -- xvfb-run -a bash tools/check_input_gui.sh <全新輸出目錄>`，保持 `--init`、非 root、無網路、逾時及程序 trap。

### 已分類的失敗與限制

- 初輪字型檢查把簡體測試 fixture 當成舊繁中介面字形，報 15 字缺字。改為只掃正式介面來源，排除 fixture 與 vendor；輸入文字由獨立繁簡字型測試負責，沒有重烘或修改既有字形來迎合測試。
- 第一個遊玩狀態測例未建立正常初始 globals／explored，讀檔會補初值，造成比較差異。修正 fixture 後通過。首輪全套命令逾時後，先縮小測試確認活性，再以隔離副本重跑完整回歸。
- 測試映像最初缺少 UID 1000 帳號，D-Bus 無法啟動；已修正可重現 Dockerfile。IBus CLI 曾回傳失敗但實際已切換引擎，現在以引擎回讀驗證。GUI 啟動以首張非黑畫面作條件，滑鼠明確按下／放開；重載腳本追錯子程序 PID 的問題已修正。這些環境與腳本失敗未列為產品缺陷。
- 一段修補腳本因 shell 串接誤在主機執行，違反 Docker 限定。已向使用者說明，隨後在容器重新核對；最終修補內容與上述完整雜湊一致，後續建置、執行及驗證全部在 Docker。
- Windows／macOS 原生輸入法仍未真機驗證。Linux 字型有明列範圍，不能宣稱涵蓋所有 Unicode。這是 remake 輸入擴充，不新增原版逐字元或逐時序 parity 聲明。
- HEAD 仍為 `9f0f4cbb246185a3fa2edd006fea9339d4134136`。前輪 HD 與工作樹變更保留；既存兩張刪圖及私人文字檔未動。沒有提交、推送或覆寫正式 `dist-all/`。
- 文件完成後從乾淨輸入重建舞台目錄，只有 README 更新；執行檔與已驗證資源的雜湊全部相同。字型雜湊、Python 語法、文件入口與相對連結檢查通過，`git diff --check` 通過。
- 收尾沒有本輪 Go 或輸入法容器。素材映像只剩其他專案的 `san5-dosgolem-concept`，未動它。新增與修改檔案均非 root 擁有；原有 373 筆衛生項目保持原狀。保留目前可重現工具映像，未執行 prune／rmi。

## 2026-11-11：HD 片頭、三平台完整版與推廣影片

使用者要求完成 HD／工具列後，重包三平台完整版及推廣片，並選擇完整重製片頭動畫。本輪只產生本機交付，沒有提交、推送、建立 tag／Release 或公開上傳。HEAD 保持 `9f0f4cbb246185a3fa2edd006fea9339d4134136`，前輪修改與既存私人檔案保留。

### 實作與來源

- 以內建 image_gen.imagegen 精修 Amiga 片頭與十一個來源姿勢。母圖及完整提示在 `workplace/hd-amiga-r1/masters/intro-000-detail-r1.png`、同名 prompt.json、`introclips-00-r2.png` 與同名 prompt.json。重建 930 筆後 manifest SHA-256 不變：`24b919e51639d58bd516cbcad829def12e4dfa5240ccd0bcc41bb380fada18d3`。
- 原始 intro.32 SHA-256 為 `8bde0a61fcb8b3736328e37797590dba159e4c7b1ec29d548f1d658a9da9b25d`；introclips.32 為 `7797e1f089659546e2069c93bea7ad0abfc53db35fcc30d5c5488620a75214ed`。以 Pillow 11.3.0 比對共同像素推定落點，屬強推論。`internal/assets/hd/intro.go` 以局部形變避免嘴尾接縫，其餘姿勢使用疊圖；不宣稱 Amiga 原版時序。DOS 原圖片頭保留。
- 新增 MM2_ASSET_ROOT，分開唯讀素材與可寫存檔。Windows 改用使用者 LOCALAPPDATA；Linux／macOS 沿用自己的使用者資料目錄。VERSION 為程式、包名、manifest 及交付目錄的唯一版號來源。
- 以 Ebiten 官方 audio/vorbis 增加 OGG 解碼，鎖定 oggvorbis v1.0.5／vorbis v1.0.2，保留 WAV 相容性。重新 vendor 後套回 XIM 修補，完整修補 SHA-256 仍為 `149f76078c8b284afab13c8706d59b2d2243d5e888172f80ac6e375a9228d623`。
- Mega Drive 實錄轉為 Vorbis quality 6、-6 dB、20 ms 邊緣淡化。十六首驗時長、聲道、取樣率、非靜音、PCM 峰值、循環邊界及完整解碼／回捲，命令與雜湊見 `smoke/music-encoding.json`。來源錄音已有削波，不宣稱波形修復或樂句接點通過人耳確認。
- 逐幕檢查發現有效職業選擇仍保留上一次錯誤訊息，修正 CreatePickClass，回歸走真實錯誤選擇再進入 Unicode 命名。職業資格與遊戲規則未改。

### 建置與驗證

沿用 san1-go:1.26.7-ebiten2.9.9-r1、san1-macos:1.26.7-sdk15.5-r1、kol-package:1.26.7-ebiten2.9.9-20261011-r1、mm2-input-test:20261111 與 yuan-wine-verify:noble-r2。容器非 root、離線、有資源／PID 上限與外層逾時，GUI 使用 --init 與 trap；快取在 workplace/cache。

| 檢查 | 結果與收據 |
|---|---|
| 回歸 | 隔離副本執行 `xvfb-run -a go test -timeout 360s ./...` 全通過，UI 73.269 秒；go vet 通過且日誌為空。見 smoke/release-go-test-final.log 與 release-go-vet-final.log。 |
| GUI | 最終 AppImage 實際解壓執行、預設 HD 像素、移動及使用者目錄存讀檔通過。完整 GUI 另驗繁簡、工具列、字級、原圖／HD、商店、神殿及存讀檔。 |
| Windows | 最終 ZIP 唯讀掛載，Wine 執行 run.bat，HD、移動、使用者存檔及新行程重載通過。執行檔、啟動器及音樂 manifest 與正式包同雜湊。 |
| macOS | 兩個執行檔均含 x86_64／arm64、最低 12.0；arm64 簽章逐頁通過。未真機執行，不稱公證或開發者簽章。 |
| 原生輸入法 | IBus/libpinyin 驗 Enter、退格、Esc 不穿透，姓名「测试long」保存與新行程重載通過。錄影腳本與截圖附在 smoke/linux-ime。 |
| 影片 | 新版 GUI 實錄、原生輸入截圖與種子 0x1111 的戰鬥示範，72 秒／1080p／30 fps。配樂為原版 Mega Drive 實錄，非舊版近似 PC 喇叭合成。接觸表、逐幕 PNG、影音檢查及來源雜湊在 promo。 |
| 三包一致性 | verify_hd_release.py 逐檔核對資源、共同來源、930 筆 HD、16 個 OGG 角色、版本、UTF-8、權限及簽章。見 smoke/package-verification.json。 |
| 釋出 | 工作 repo 情境的 check_release.sh 通過，未在舊私有 repo 套用 --public。 |

重建入口為 `tools/package.sh local-full <平台> --data-dir <絕對路徑> --music-pack-dir <正式 OGG 包>`，階段為 package_container.sh 與 pack_wrap.sh。平台舞台目錄各自獨立；舊 collect_dist_all.sh 未執行，既有交付保留。

### 失敗分類與限制

- 預設 GOPROXY=off，下載兩個鎖版 OGG 相依時臨時啟用官方代理，之後離線建置。初次 GUI 測試缺 DISPLAY，改由既有 Xvfb 重跑。
- macOS 工具鏈缺 Python，將素材收集移至既有封裝映像。舞台複本有唯讀權限，重包前只移除已確認的舞台目標，不改來源權限。曾提前掃描未寫完的 ZIP，待封裝結束後重驗，正式 CRC 與雜湊通過。
- Wine 改用容器內專屬 HOME。單行巢狀批次迴圈提早退出，改明確區塊及無尾端反斜線的目標參數後，正式啟動器通過。
- X11 來源錄影為可變幀率，不能用標示的 30 fps 作證。經 fps=30／固定輸出後，正式片為 2,160 幀、72.000 秒。選單、地圖及輸入截圖的停留與靜止觀察明列。
- AppImage 驗收曾誤要求未更動的偏好檔已存在；實作僅在偏好改動後寫檔。改以 HD 邊框像素及實際存讀檔驗證，未為驗收變更產品。
- 錄影快速數字鍵曾仍停在性別選單就開始拼音，第一個字被選單消耗。改以實際命名頁標題為條件後輸入，原生拼音與存檔通過。原測試的空姓名提示保留為正當驗證情境，正式影片改走正常選單。
- 戰鬥使用受控 fixture 抽測一般／快速分支，不稱本輪已由正常起點走到隨機戰鬥。Windows／macOS 真機及原生輸入法、音訊人耳確認仍待補。HD 片頭不宣稱 Amiga 原版時序 parity。
- 兩張既有刪圖與私人文字檔未動；完整包及影片只留 ignored 版本目錄。最終擁有權、SHA-256 及 Docker 清理結果附在交付收據。

### 存檔隔離勘誤

收尾稽核確認 TestUnicodeNameThroughCreateSaveAndReload 原先會在 repository 根目錄寫入 save/state.json，並於 cleanup 移除 save。本輪修正職業提示時曾直接執行一次該測試；當時未先記錄根目錄存檔是否存在，目前無法確認是否曾覆蓋既有玩家存檔，也未找到原存檔備份。沒有把測試收據冒充原存檔還原。已改為測試專屬暫存根，資料與素材唯讀連結沿用，另以預存哨兵檔驗證針對性測試不再碰原存檔。完整 Go 回歸仍在獨立副本執行。兩張既存刪圖與私人文字檔未動的敘述仍成立；不能把它擴張為本輪完全未碰根目錄 save。

使用者後續表示沒有要保留的進度，應已手動刪除，不需還原。隔離修正的針對性測試通過，預存玩家存檔哨兵 SHA-256 前後一致，收據為 smoke/save-isolation-test.log。使用者另要求節省空間，收尾只清理本輪可重建快取與重複解包，保留正式產物、原始資料、生成母圖及影片來源。

空間清理完成：移除本輪平台舞台目錄、八份重複解包、未採用錄影、剪輯中間檔及本專案 Go 建置快取，合計約 2.73 GiB。正式三包與影片 SHA-256 清理前後一致；原始資料、HD 母圖與提示、最終影片來源及模組快取保留。未刪 Docker image，未執行 prune／rmi。明細見 smoke/space-cleanup.json。完整 GUI 最後以跨影格滑鼠按壓通過，避免軟體繪圖環境漏接短點擊。重合成影片時使用 render_hd_promo.sh 的第三個參數 capture-r3。

Docker 收尾確認：本輪建置、GUI、Wine、錄影及封裝容器均已移除，其他專案容器未動。衛生掃描仍為原有 373 筆，本輪交付沒有新增 root-owned 檔案。git diff --check 通過。

## 2026-11-11：工具列英文字修正與 README HD 總覽

使用者指出「素材 Amiga」的英文字高低錯落，並要求 HD 素材總覽放在 README。本輪路由命中復古遊戲介面驗證與 CJK 高解析畫布契約，載入既有端到端入口及 `81-retro-cjk-hires-canvas.md`。沿用目前 Noto Sans／Serif，未更換字型設計。

### 原因與實作

- 原字模產生器為了塞進半形欄位，對每個字母分別縮小寬高，又將不同字母貼齊頂端。24 像素字模的 A 高 11、m 高 6、i 高 16、M 高 8 像素，造成錯位。
- `tools/build_display_locales.py` 改用整個 Latin 字集共同基線與垂直比例，過寬字形只壓縮寬度。重烘 Sans 24／30／36 與 Serif 24；其他 CJK 字模、繁簡譯文及轉換表雜湊保持相同。來源與成品雜湊在 `translations/display-build.json`。
- `tools/render_hd_overview.py` 從實際 HD 成品與本輪遊玩截圖產生 WebP。展示圖為 2400×2040，完整圖集為 3104×7860，共約 3.3 MiB。逐項核對來源與成品雜湊，完整收錄 930 個唯一對應 ID、913 幅獨立畫稿、72 組怪物，另列兩種邊框。原素材的空白影格有明確標示。
- README 直接顯示總覽，點圖開啟完整圖集。檔案位於 `docs/gallery/amiga-hd-overview.webp` 與 `amiga-hd-complete.webp`；重建入口已加入 [顯示規格](docs/spec/amiga-hd-toolbar.md)。

### 驗證與本機交付

| 檢查 | 結果與收據 |
|---|---|
| 字模與渲染 | `go test ./internal/i18n ./internal/render -count=1` 通過。新測試檢查大寫與數字字高、共同基線、小寫高度與下伸部分，以及 Amiga 相對中文字高。 |
| 工具列操作 | `go test ./internal/ui -run TestToolbarPointerOwnershipAndSelections -count=1` 通過。 |
| 實際畫面 | Xvfb 啟動現有執行檔與新字模，確認 HD 探索及 100%／125%／150% 工具列並存檔離開。工作目錄與存檔位於專屬 `workplace/toolbar-font/run/`。 |
| 三平台封包 | 從現有未發布本機包解開至容器暫存目錄，先核對正式程式來源與執行檔，再更新四個字模、字型收據、README 及兩張圖集。執行檔 bytes 完全相同。逐包 manifest、930 筆 HD、音樂、共同來源、ZIP CRC／UTF-8／權限及 Mac 雙架構與 arm64 簽章重驗通過，最後原子替換本機包。 |

本輪完整收據位於 `dist-all/v.1.1.0-20261111/smoke/font-fix/`；逐包檢查為 `smoke/package-verification.json`。推廣影片沿用前次已驗證版本，未重錄。沿用現有未發布本機版號，沒有移動 tag／Release、提交或上傳。Go 建置快取與解包位於容器暫存空間，容器退出即清除；不留下第二組完整包。

收尾 SHA-256 索引的 67 個檔案逐一重驗通過；交付檔擁有權為 1000:1000，沒有新增 root-owned 檔案。全工作根衛生掃描仍為既有 373 筆，封裝與驗證容器已移除。HEAD 未變，`git diff --check` 通過。

## 2026-11-11：Android 手機操作定稿

使用者要求規劃方便操作、按鍵裝飾配合 MM2 的手機版。先依 `grill-me`／`grilling` 查證現有輸入與平台能力，製作橫向、直向的可丟棄草稿。使用者回覆「ok 定稿」，接受建議的 Android、A 款橫向雙手操作；隨後明確限定「android 版本就好囉」，因此 iPhone／iOS 與 B 款直向版不列入範圍。

- 路由命中復古遊戲生命週期、規格閘門與文件分工，載入端到端入口、`retro-remake-spec-gated-workflow.md` 及 `project-document-responsibilities.md`。沿用既有 `docs/spec/assets/`，未新增根目錄工作區。
- 定稿保存於 [Android 規格](docs/spec/android-mobile.md)。原始對照圖、情境圖、HTML、檢查程式與收據從 `/tmp/mm2-mobile-plan-r1/` 原樣複製至 `docs/spec/assets/android-mobile-r1/`，共約 6.1 MiB。保存完整原圖，右側 B 只作歷史比較，不是開發分支。來源與 SHA-256 見 `approval.json`。
- 草稿使用現有遊戲截圖與 MM2 HD 邊框，沒有新增圖像生成。Chromium 1194 在既有 `mcr.microsoft.com/playwright:v1.56.1-noble` 容器檢查 A／B 各 16 個主要點擊區、48 邏輯單位下限、移動回饋、隊員選取、返回、戰鬥按鍵及 Unicode 輸入預覽通過。這些是瀏覽器草稿收據，未外推為 Android 遊戲驗收。
- 查證目前前端仍是鍵盤及工具列滑鼠入口；鎖定的 Ebiten 2.9.9 對 Android 使用無 IME 的字元輸入替代實作。原生輸入法、背景恢復與支援 API／架構列入下一階段查證，沒有宣稱已可直接打包。
- README 新增穩定入口，CONTEXT §1.5 記錄唯一目前狀態。只完成範圍、視覺與開發順序定稿；Android 正式實作契約仍為 DRAFT，下一步是工具鏈盤點與 A 款原生操作切片。未更改遊戲程式、版號、存檔或既有三平台包。

保存後逐一重驗草稿與來源雜湊，11 個相關檔案的擁有權均為 1000:1000。新規格的相對連結全部有效；索引檢查先命中既有 Amiga 規格作正對照，再確認 README 與 CONTEXT 都可找到 Android 規格。全工作根衛生掃描維持既有 373 筆，本輪容器已移除。HEAD 仍為 `9f0f4cbb246185a3fa2edd006fea9339d4134136`，`git diff --check` 通過；本輪只有文件與定稿保存，未重跑 Go 測試。沒有提交或推送。

## 2026-11-11：Android 原生實作與驗證進行中

使用者明確要求開工。路由命中規格閘門、平台實作、跨平台驗收與交付契約，套用 `reverse-engineer-retro-game-remake`，沿用既有共同決策結果，不重問已確認的 Android／橫向範圍。

- `internal/mobileui/` 依 A 款配置實作實際遊戲畫面、裝飾按鍵、六人狀態、選單直接點選與分頁。`mobile/` 透過 Ebitengine 行動入口接既有 Session；Android Java 殼層負責私有資產、系統生命週期及原生輸入欄。
- 觸控以第一指持有，支援拖出取消、400ms 起步／180ms 重複、跨模式停止；原生事件先排入獨立佇列，避免慢速渲染漏掉短按。輸入經既有字元驗證，過長、無效字元及過期提交都不改既存文字。
- 原 Save 的寫入流程抽成回傳 error 的 helper，原玩家訊息保留。手機持久檢查點只在沒有 pending、戰鬥及未領寶箱的探索狀態寫入；背景保留互動記憶體，程序終止回到最後成功檢查點，未新增存檔 schema。
- 沿用 `rich2-go-android:20261102-r1`，Go 1.26.7／Ebiten 2.9.9、SDK35／NDK27.2、Java17；以官方 `ebitenmobile` 綁定後用現有 SDK 工具組裝 APK。arm64、x86_64 與 API23 映像所需 x86 已編譯；本機 debug 簽章與 16KB ZIP 對齊通過。中間 APK 在 `workplace/android/`，不是正式交付完成。
- 手機互動、字庫與渲染抽測通過。完整 `xvfb-run -a go test -timeout 360s ./...` 在容器暫存來源副本執行，全套通過，UI 102.043 秒；`go vet ./...` 通過且日誌為空。收據在 `workplace/android/go-regression.log`、`go-vet.log`。測試存檔隔離於暫存根。

### 已查明的環境與封裝問題

- 登入 shell 會覆蓋 Android 工具 PATH；改用相同映像的非登入 shell，未新增工具映像。桌面 AppImage 的固定時間戳可能早於 ZIP 下限，封裝器已明確處理。原生組字 API 使用 Spannable，已修正第一次 Java 編譯的型別錯誤。
- 首次 Android 6 正常啟動抓到缺少 MAP.DAT。原因是桌面包將原版放在 `original-data/`，Android 收集器卻假定為研究目錄 `workplace/orig/MM2/`。已依實際舞台路徑映射並加入必要原版檔案檢查，修正版正從正常入口重驗。
- 本機沒有 KVM。Android 15 軟體模擬器日誌出現 system_server 被 watchdog 殺掉，未進入遊戲，屬驗證環境。依 [AOSP Build](https://android.googlesource.com/platform/frameworks/base/+/main/core/java/android/os/Build.java) 與 [Watchdog](https://android.googlesource.com/platform/frameworks/base/+/master/services/core/java/com/android/server/Watchdog.java) 的軟體模擬器契約，以 `ro.hw_timeout_multiplier=5` 重跑；不改遊戲時序或把這個環境旗標放進 APK。
- 曾在 Bash 驗證腳本執行中更新同檔，造成後續解析位置錯誤及暖機行程提早退出，已向使用者說明。後續長工作先複製固定腳本到容器暫存再執行，保留有界父行程及 trap。
- Android 6 系統映像是 x86，初次誤用 x86_64 路徑已修正。已暖機的驗證容器仍在使用；收尾須明確關閉並清理。本輪尚未下 Android 完成聲明，也未改正式版號或重包桌面交付。

## 2026-11-11：Android 本機測試版交付

延續已授權的 A 款 Android 實作，正式版號改為 `v.1.2.0-20261111`。前節「未改版號／仍在驗證」是初期紀錄；目前結果以本節與 CONTEXT §1.5 為準。HEAD 保持 `9f0f4cbb246185a3fa2edd006fea9339d4134136`，未提交、推送、建立 tag 或公開上傳。既有桌面三包與推廣片保留在 v.1.1.0-20261111。

### 實作與修正

- `internal/ui/mobile.go` 提供既有選單確認、隊員選取、完整資料、原生輸入限制及安全檢查點介面。Save 寫入抽成回傳 error 的共用流程，未改存檔 schema 或遊戲規則。
- `internal/mobileui` 依定稿接左側方向、右側情境指令、六人狀態、分頁選單、背包選取後裝備／使用，以及完整訊息閱讀。裝飾使用現有 HD 邊框與程式圖示。
- `mobile` 接 Ebitengine，`android/MainActivity.java` 提供私有素材匯入、原生文字欄、輸入法與生命週期。原生提交與生命週期工作離開 UI 主執行緒；輸入請求及診斷以原子快照讀取，修正初期渲染鎖造成的輸入逾時。
- 實際觸控收據抓到另一個產品問題：重繪較慢時，佇列中的短按 MOVE 被處理時間算成長按，只選取而未確認。改保留 `MotionEvent.getEventTime()`，事件與持續按壓共用 `CLOCK_MONOTONIC`。依據為 AOSP SystemClock.cpp 與 Timers.cpp，連結在規格。修正後重跑原本失敗的連續建角選單通過，沒有放寬 400ms／180ms 契約。
- 姓名、答案與控制碼經現有字元驗證及帶序號提交；無效、過長及過期提交不改既存文字。新增玩家文字走 `translations/interface.tsv`，沿用現有繁簡字庫與授權。

### 驗證與交付

| 檢查 | 結果與正式收據 |
|---|---|
| 完整回歸 | 隔離來源副本執行 `xvfb-run -a go test -timeout 360s ./...` 及 `go vet ./...` 通過，UI 71.801 秒；後續手機完整測試與繁簡測試通過。見 `smoke/go-regression-final.log`、`go-vet-final.log`、`mobile-final.log`。 |
| 戰鬥分支 | 固定種子 0x1234，比對手機與鍵盤的一般／快速戰鬥、戰利品及存檔狀態，通過。見 `smoke/mobile-combat.log`。只證明 remake 控制層結果一致。 |
| 正常觸控 | 相同最終 APK 由啟動器進入，實走轉向、實際座標移動、旅店進出、隊員資料、裝備、購買、使用、休息及兩種地圖。金幣 200→185，取得短劍；使用抽測為非魔法武器拒絕提示。見 `smoke/android/initial-smoke.json`、`items-smoke.json` 與實際 PNG。 |
| 原生輸入與恢復 | 原生 InputConnection 驗組字 Enter 不提前送出、退格、確定文字、提交及取消。姓名「测试long」寫入並由新行程完整載回，簡體偏好保留；背景恢復通過。見 `smoke/android/player-smoke.json`、11／12／13／14 開頭截圖及取消前後 JSON。 |
| 封裝 | 三架構 arm64／x86_64／x86、本機 debug 簽章、ZIP／ELF 16KB 對齊、版本及 1,612 個內嵌檔案雜湊通過。正式 APK 149,779,290 bytes，SHA-256 `09501c08be852c0c806de319aea105a19c6b7b677858712b1feaa33e30dfb926`。見 `smoke/package-check.json`、`apk-check.json`、來源指紋與 `build-environment.json`。 |
| 釋出邊界 | 工作 repo 情境 `check_release.sh` 通過。完整 APK 含原版資料與衍生素材，只留本機。正式入口為 `dist-all/v.1.2.0-20261111/full-local/`，全部正式檔案由版本根目錄 `SHA256SUMS.json` 索引。 |

重建使用 `rich2-go-android:20261102-r1`，Go 1.26.7／Ebiten 2.9.9、SDK35／NDK27.2.12479018、Java17。`tools/build_android.py` 在唯讀 /src、可寫 /out、唯讀 /module-seed 中離線建置；`tools/verify_android.py --deliver` 逐項核對目前來源及三組相同 APK 的遊玩收據後才建立新版本目錄。正式來源與素材來源 AppImage 的雜湊、映像 ID 均有收據。所有可寫容器以 1000:1000 執行。

### 驗證腳本勘誤與限制

- 早期腳本以重繪次數推定動作已完成，改等 `action_serial` 及具體狀態；前進改要求實際座標變動，撞牆訊息不算移動成功。
- 取消建角的測試曾誤要求整份存檔完全相同。現有 State 本來就保存亂數種子，建角擲點會推進它。重驗保留前後 JSON，唯一差異為 seed，名冊、隊伍及其他進度相同；沒有修改產品來迎合測試。
- Android 6 的 UiAutomator 不列出測試輸入法視窗，以該獨立 fixture 的四欄佈局點擊。原生取消按鈕的零時長注入曾只收起鍵盤，改用 160ms 點擊並等原生編輯視窗消失；相同 APK 重驗通過。測試輸入法及私鑰未加入完整包。
- 最終模擬器為 API23 x86、無 KVM，客體 2 核／2GiB，容器最後使用 4 CPU／4GiB 及有界 90 分鐘父行程。Android 15、實體手機、第三方拼音鍵盤、其他螢幕尺寸及人耳音訊仍待驗。地圖拖曳放大屬可選 polish；不擴張為原版 parity 或 Android 通關聲明。

### 收尾

正式 APK 與所有索引檔逐一重驗 SHA-256、1000:1000 擁有權及文件相對連結通過；索引正對照先命中既有 Amiga 規格，再確認 Android 入口。衛生掃描維持原有 373 筆，沒有新增 root-owned 檔案。已清理本輪 Android Go 建置快取、AAR、runtime.zip 與重複 APK，共 859,009,696 bytes，約 819 MiB；保留正式 APK、來源／測試收據與供後續升級使用的本機簽章。清理前後原桌面來源包與正式 APK 雜湊一致，明細見 `smoke/space-cleanup.json` 與 `final-check.json`。

模擬器由結束旗標走 trap 正常退出，容器已移除；本輪建置、測試與封裝容器也已移除，其他專案未動，未刪映像或執行 prune。最後 `git diff --check` 通過。既存兩張刪圖與私人文字檔保留原狀。下一個外部驗收動作是將正式 APK 安裝到 Android 真機，抽測觸控、該機拼音鍵盤、背景恢復及音訊。

## 2026-11-12：含 Android 的推廣片與版本化交付

使用者要求推廣片包含 Android，完整版全部放在 `dist-all/` 的版本目錄。本輪只處理錄影、合成、歸檔與文件，保留桌面 v.1.1.0-20261111 及 Android v.1.2.0-20261111 的既有程式與完整包。

- 路由命中復古推廣片、發行輸出、版號與文件分工，載入 `game-promo-video-ffmpeg.md`、原版音樂規則及 `retro-remake-dist-all-output.md`、`retro-remake-release-versioning.md`、`project-document-responsibilities.md`。沒有新增公開發行、授權或平台決定。
- 桌面從已交付 AppImage 唯讀解包，在 Xvfb 重錄片頭、探索、工具列與設施。戰鬥沿用受控種子 0x1111 的既有示範，Linux 輸入法段沿用實測截圖序列。沒有把受控戰鬥稱為正常起點遊玩證據。
- Android 使用 SHA-256 `09501c08be852c0c806de319aea105a19c6b7b677858712b1feaa33e30dfb926` 的相同 APK，在 API23 x86 軟體模擬器實際觸控，保存操作前後 framebuffer、時間戳、雜湊及狀態。探索 13 張、選單 22 張；原生文字欄使用同版 APK 已驗證的組字及提交截圖，裁去獨立測試鍵盤，字幕明列為實測截圖。
- 影片共 72 秒、1920×1080、30 fps、2,160 幀。Android 操作段共 24 秒，姓名欄截圖段 8 秒；未冒稱連續錄影、手機真機或效能測試。配樂複用前片的原版 Mega Drive 擷取音樂，完整 AAC 封包 SHA-256 與來源相同。
- 正式影片、九幕總覽、逐幕 PNG、來源、分鏡、格式、音訊與視覺檢視收據在 `dist-all/v.1.2.0-20261111/promo/`。影片 6,476,502 bytes，SHA-256 `d29bda2f5c68df16f50fefcbfce84b6892c2a3320735449a94fbf956112929d7`。來源工作區為 `workplace/promo-android-v.1.2.0-20261111/`；重建入口與限制見 [推廣片文件](docs/promo.md)。

### 拍攝與封裝問題

- 模擬器 `screenrecord` 的軟體 H.264 編碼器在 libstagefright_omx 崩潰，屬錄影環境問題。改用實際 framebuffer 擷取。早期連續擷取增加負載，且系統全螢幕首次教學擋住操作；最後改在啟動前確認教學，逐項操作後擷取。
- 最後一次 Android 選單操作均已完成，只有額外收尾截圖碰到前景驅動的 240 秒界限。依已存在 PNG 的寫入時間與已保存遊戲診斷補齊來源收據，保留此限制，沒有再啟動模擬器或假造執行腳本雜湊。
- 拼接的不同時間基準最初只有 2,140 幀，已在最終視訊編碼明確鎖定 30 fps／2,160 幀。隨後發現視訊幀數限制在編碼器延遲輸出前提早結束音軌，片尾少 0.64 秒。修正為先完成視訊，再複製完整音軌封裝；驗證器增加音軌 72 秒與封包雜湊比對，最終重驗通過。

### 歸檔、驗證與清理

- 舊的未分版本 `local-full/`、`public/`、`promo/` 共八檔，以包內原始來源 commit `ffa373aa81e7ecdc2275c8ef80616092958cb035` 歸到 `dist-all/ffa373aa81e7/`。這批屬正式版號制度前的歷史包，未補造新正式版號。原檔名、內容、原索引及原校驗清單保留，搬移前後大小與 SHA-256 相同；對照在 `MIGRATION.json`。
- `tools/archive_legacy_dist.py` 保存遷移與重驗入口。停用原本會刪除整個 dist-all 的舊 `tools/collect_dist_all.sh`，改成明確指向目前封裝文件並以狀態 2 結束。已檢查語法及預期結束狀態。README、CONTEXT 與版本索引改指向目前目錄。
- 新片全片視音訊解碼、72 秒與幀數、立體聲 AAC、字幕邊界通過。音量為 −18.4 LUFS、峰值 −11.5 dBFS，沒有黑畫面或超過兩秒的靜音；選單、地圖、輸入欄與片尾的凍結觀察保留，未宣稱全片無靜止。九幕抽樣檢視確認字幕與遊戲畫面，音訊技術結果不代替人耳驗收。
- 加入影片前重驗 v.1.1 的 67 筆與 v.1.2 的 40 筆既有索引，全部一致。正式影片及收據加入 v.1.2 的 `SHA256SUMS.json`；沒有重新建置已驗證的完整包，這輪也沒有重跑無關的 Go 回歸。
- 既有 Docker 映像 `kol-package:1.26.7-ebiten2.9.9-20261011-r1`、`rich2-android-emulator:api23-20261102-r1`、`sc-cht-hdtools:pillow11.3.0-r2` 執行全部工作。來源唯讀掛載，可寫目錄使用 1000:1000、網路關閉、資源上限及有界前景程序；模擬器由 trap 關閉，未刪任何映像。
- 正式交付雜湊核對後，清除本輪失敗拍攝、測試執行目錄、可重建中間檔及正式檔副本，共 46,150,882 bytes，約 44.01 MiB。原始桌面錄影、最後 Android 擷取、時間戳、來源與日誌保留。明細見 `promo/space-cleanup.json`。

最後狀態與雜湊收據見該版 `promo/final-check.json`。HEAD 保持 `9f0f4cbb246185a3fa2edd006fea9339d4134136`，未提交、推送或公開上傳。原有兩張刪圖及私人文字檔保留。手機真機、第三方拼音鍵盤與人耳確認仍沿用前輪限制。

收尾重驗：v.1.1 共 67 筆、v.1.2 加入最終收據後共 59 筆，歷史搬移 8 檔均核對。20 個交付及影片入口有效。首次入口檢查誤把容器 `/deliver/../README.md` 當專案 README，屬雙掛載路徑問題；改以 `/src/dist-all/` 解析後，同容器重跑通過。腳本語法、`git diff --check` 通過，衛生掃描維持原有 373 筆，正式交付檔擁有權為 1000:1000。本輪容器均已移除，其他專案未動。

## 2026-11-12：工具列英文等高修訂版

使用者從推廣片指出英文仍有大小差。本次修正採工具列專用的等高大寫字模，並以實際成片與新完整包驗證。桌面修訂版為 `v.1.2.1-20261112`，保存在 `dist-all/v.1.2.1-20261112/`；舊桌面、Android APK 與影片保留原版號及雜湊。

### 原因與修正

- 前次共同基線修正保留一般大小寫字高。24 px 字格中，大寫 A 約 16 px、小寫 m 約 12 px，中文字約 22 px，仍不符合使用者要求的等高工具列。
- `tools/build_display_locales.py --toolbar-only` 以同一份已釘雜湊 Noto Sans 烘製 `toolbar-sans24.bin`、`toolbar-sans30.bin`、`toolbar-sans36.bin`。英文顯示為大寫，字高依現有中文標籤的實際墨跡量測，分別為 22／27／32 px。來源、Pillow 版本及成品雜湊在 `translations/display-build.json`。正文的 Sans／Serif 與繁簡中文字模逐位元維持原值。
- `Display.ToolbarFonts` 只在工具列使用這三份字模。平台識別值、文字內容與儲存格式不變。長標籤只縮窄寬度，不連帶縮小高度；文字另避開下拉箭頭欄，避免 150% 的 MEGA DRIVE 與箭頭重疊。
- 封裝資源清單加入三份字模。macOS 工具鏈沒有 Git，封裝器改接已驗證的 `MM2_SOURCE_COMMIT`，檢查完整 40 位十六進位識別碼；主機封裝入口自動傳入。正式三包的來源 commit 與共同來源清單已核對。

### 驗證與影片

| 檢查 | 結果與收據 |
|---|---|
| 字型與工具列 | 繁簡、100%／125%／150%、AMIGA／MEGA DRIVE、字高、邊界、箭頭避讓及命中測試通過。原生工具列截圖及前後對照在 `smoke/toolbar/`。首次 150% 檢查找到中文字實際高 32 px，已改由字模量測取代以字格高度推定。 |
| 最終來源回歸 | 隔離副本執行 `xvfb-run -a go test -timeout 360s ./...` 全套通過，UI 86.803 秒；`go vet ./...` 通過且日誌為空。見 `smoke/go-regression-final.log`、`go-vet-final.log`。 |
| Linux 正常操作 | 最終 AppImage 唯讀解包後實走繁簡、原圖／HD、工具列、移動、商店與神殿進出、物品、存檔、離開及新行程重載。見 `smoke/linux/checks.json`。 |
| Linux 輸入法 | IBus 拼音組字、Enter／退格／Esc、中文姓名建立與新行程重載通過。乾淨錄影流程先確認姓名表單，再開始輸入；影片未使用測試刻意觸發的空姓名存檔提示。輸入錄影執行檔 SHA-256 與正式 Linux 包相同，見 `promo/PROVENANCE.json`、`smoke/linux-input/`。 |
| Windows 與 macOS | 最終 Windows ZIP 經 Wine 啟動 run.bat、移動、存檔及新行程重載通過，收據綁定最終 ZIP 雜湊。macOS universal 的兩架構、最低版本及 arm64 簽章結構通過，未宣稱真機驗收。 |
| 三包 | 逐檔核對資源、三份新字模、共同來源、930 筆 HD、16 個 OGG 音樂角色、版本、UTF-8、權限及 Mac 簽章。見 `smoke/package-verification.json`。 |
| 新版影片 | 所有桌面段落由最終來源重錄，Android 沿用已驗證的 v.1.2.0 畫面擷取與原生編輯截圖。72 秒、1080p、30 fps、2,160 幀，全片解碼、字幕、黑畫面、音量及音軌來源比對通過，九幕已目視檢查。影片 SHA-256 為 `852cf28f6817b1b6438fafa867f053dbd59c54dbbc786eac0093e84b5b9e287f`。 |

首次 Linux 舞台呼叫的工作目錄在 `/tmp`，Go 找不到 go.mod；改回 `/src` 後同工具鏈重跑。Wine 首次拒絕在 root 擁有的 `/tmp` 下自行建立設定，明確建立由 1000 擁有的 HOME 與 WINEPREFIX 後重跑通過。這兩項是驗證環境問題，沒有修改遊戲規則。初始 GUI 截圖尚未浮出工具列，不用它作字高對照；正式對照圖採最終原生工具列渲染，成片另逐幕檢視。

沿用 `san1-go:1.26.7-ebiten2.9.9-r1`、`san1-macos:1.26.7-sdk15.5-r1`、`kol-package:1.26.7-ebiten2.9.9-20261011-r1`、`mm2-input-test:20261111`、`yuan-wine-verify:noble-r2` 及 `sc-cht-hdtools:pillow11.3.0-r2`。工作均在非 root、離線、有資源上限與外層逾時的 Docker 容器執行。原始資料與舊交付唯讀，存檔與設定隔離於本輪工作目錄，未啟動 Android 模擬器。

正式交付共 81 筆檔案已列入 SHA256SUMS.json；舊 v.1.1 的 67 筆及 v.1.2 的 59 筆雜湊維持原值。核對正式副本後，清理本輪快取、三平台舞台、重複成片、舊拍攝及測試執行目錄，共 1,235,582,839 bytes，約 1.15 GiB。保留原始資料、最終桌面錄影、戰鬥與輸入片段、姓名實測截圖及原 Android 擷取。明細與最終收據為 `smoke/space-cleanup.json`、`smoke/final-check.json`。

26 個相關入口有效，正式檔擁有權為 1000:1000，衛生掃描維持既有 373 筆。HEAD 保持 `9f0f4cbb246185a3fa2edd006fea9339d4134136`；未提交、推送或公開上傳。原有兩張刪圖與私人文字檔保留。本輪容器已移除，其他專案未動。Windows／macOS 真機、Android 真機、第三方拼音鍵盤與人耳確認沿用既有限制。

## 2026-11-12：README 開場介紹濃縮

依使用者提供的文章，將科隆世界觀、柯拉克失蹤及繁體中文重製成果濃縮放到 README 最前面，補入 Amiga 手繪 HD、重製片頭、930 筆素材、裝飾邊框、工具列與繁簡輸入。開場保留說明書引句並連到原轉錄，詳細故事與技術章節保留在後文；既有下載、HD 總覽及權利邊界沿用。

路由命中 `retro-remake-readme-standard.md` 與文件分工，套用 humanizer。實際核對繁簡翻譯檔各 2,703 條及 HD 規格，沒有在開場沿用缺乏目前證據的通關比例或整款逐像素還原聲明。這輪僅修改 README 與本工作紀錄，不變更程式、版號、封包或影片；文件連結及差異檢查完成後交付，無需重跑遊戲測試。

## 2026-11-12：提交與推送

使用者授權 commit 與 push。本次提交納入已完成的 Amiga HD 載入、工具列、繁簡顯示與 Unicode 輸入、Android 程式、可重現封裝／錄影工具、字型與授權、文件與定稿圖，以及鎖版相依來源，共 1,141 個檔案。推送目標為既有 `origin/main`；提交前 fetch 確認本地與遠端同為 `9f0f4cbb246185a3fa2edd006fea9339d4134136`。作者與提交者均使用 `wicanr2@gmail.com`，不改寫歷史。

精確清單與內容掃描排除原版資料、完整包、影片、簽章私鑰、私人文字檔及原有兩張 SKY 圖片刪除。Ebitengine 的兩個音訊測試檔與鎖版模組來源逐位元相同，Apache 授權保留。暫存後的工作 repo 釋出檢查通過。空白警告來自 44 份上游來源與原始授權；除既有 XIM 修補外均與模組快取相同，XIM 修補以原始／修補雜湊及精確替換契約驗證，修補差異沒有空白警告。自有檔案的暫存差異檢查通過，未改動第三方檔的原始排版。

沿用最終來源已通過的完整 Go 回歸、vet、正常 GUI、Linux 輸入與 Windows Wine 收據；本次沒有新增程式修改，不重跑相同測試。提交範圍、釋出與上游來源核對收據保存在本機 `workplace/toolbar-font/height-v.1.2.1/publish-*.json`／`.log`。Git 操作以本輪提交與 `origin/main` 的實際指標為準；本機交付與未納入的既存變更維持原狀。
