# 三平台打包骨架

目前確認的目標平台為 Windows x64、macOS universal（Intel + Apple Silicon）與
Linux x64。這份文件只描述打包閘門，不宣稱三個平台都已完成真機驗收。

## 雙軌輸出

公開包包含 remake 與 `mm2data` 執行檔、三份可公開 base data、字型、翻譯、啟動器、README、釋出政策與 manifest；玩家必須自行提供合法
原版資料。每次啟動都必須明確指定含 `MM2.EXE`、`SPELLS.DAT`、`2PLAY.OVL` 的資料目錄；啟動器會由該份 `MM2.EXE` 產生本機 runtime data，並以同一資料目錄載入圖像與遊戲資料。
啟動前還會 fail-closed 檢查 `MAP.DAT`、`EVENTSI.DAT`、`ATTRIB.DAT`、`MM2.CH`、
`DEFAULT.DAT`、`MONSTERS.DAT`、`TOWN.16`、`TOWNF.16`、`TOWNT.16`、`SKY.16` 與
`ITEMS.DAT`。
輸出在已忽略的 `dist/public/`，並且腳本完成後會執行私有工作 repo 的
`tools/check_release.sh`。正式公開前仍須另建乾淨 repo，執行
`tools/check_release.sh --public`；不得把原版資料、解包檔、衍生 JSON、私有研究稿
或原版美術放入公開包。

完整版可以帶入玩家指定的原版資料，但只允許輸出至已忽略的 `.local-full/`。runtime
data 由容器宿主目標（Linux amd64）的獨立 `mm2data-host` 產生，再建置 Linux 或
Windows 目標 binary；因此不會在 Linux 容器執行 Windows `.exe`。腳本
要求資料目錄存在且含 `MM2.EXE`，並拒絕把已納入 Git 的路徑當作輸出根目錄；此包
不得 `git add`、commit、push 或上傳。

`local-full` 可用 `--music-pack-dir` 帶入一份含 `manifest.json` 的玩家本機音樂包，
封裝後啟動器會自動傳給引擎。這個參數在 `public` 模式是硬性錯誤；音樂包、原版資料
與完整封包都維持在忽略路徑，不進公開 allow-list。

## 命令

在專案根目錄執行：

```bash
bash tools/package.sh public linux-x64
bash tools/package.sh public windows-x64
bash tools/package.sh public macos-universal

bash tools/package.sh local-full linux-x64 --data-dir /private/MM2
bash tools/package.sh local-full windows-x64 --data-dir /private/MM2
bash tools/package.sh local-full macos-universal --data-dir /private/MM2

# 只有本機完整版可選配原版音樂包
bash tools/package.sh local-full linux-x64 --data-dir /private/MM2 \
  --music-pack-dir /private/mm2-music
```

Linux／macOS 公開包啟動方式為 `run.sh /private/MM2`；Windows 使用
`run.bat C:\\MM2`。Windows 包不附上無法執行其 `.exe` 的 Unix 假啟動器。兩者都會
拒絕缺少三個必要檔案的目錄。local-full 的 `run.sh`／`run.bat` 無參數時預設使用
包內 `original-data`；為避免 Windows batch `%*` 重複傳參，Windows 啟動器目前不接受
額外遊戲參數。

Linux 與 Windows 的 Go build 在 `mm2-go:latest` Docker image 內執行，使用無網路
與 `GOPROXY=off`；依賴快取缺件就失敗，不會偷偷下載或使用宿主工具鏈。Linux 產物
是 x64 `tar.gz`，Windows 產物是含 `.exe` 的 x64 `tar.gz`。這是可重現 binary/package
骨架，尚未取代真機 smoke；本輪 Docker 驗證 Windows 只涵蓋 PE／封裝內容與 Linux
host helper，不代表 Windows 執行驗收。

macOS universal 由固定的 `wolong-osxcross-go:20260811-event10-r4` Docker image
建置，使用 `/osxcross/SDK/MacOSX15.5.sdk`、`o64-clang` 與 `oa64-clang` 分別編出
x86_64／arm64，再以 `lipo` 合併 `mm2` 與 `mm2data`。local-full 的 runtime JSON
由同容器內可執行的 Linux `mm2data` 產生，避免在 Linux 容器執行 Mach-O。套件
manifest 會記錄 image、SDK 與架構。這只證明交叉編譯與封裝；尚未宣稱 macOS 真機、
簽章或公證通過。

## 最小 smoke gate

- 共通：確認輸出路徑仍被 `.gitignore` 忽略；解壓後檢查 manifest；執行
  `tools/check_release.sh`，公開 repo 另跑 `--public`。
- Linux：Docker 內 `go test ./...`、`go build ./cmd/mm2`、Xvfb 啟動與截圖；宿主
  Linux 真機再驗視窗、GPU/音效、鍵盤、檔案大小寫與存檔重載。
- Windows：Docker 內確認 PE x64、封裝 allow-list 與 runtime JSON；真正 Windows x64
  執行（啟動、讀取原版資料、移動、存檔／重載、離開）仍須在乾淨 Windows 真機，
  另驗 Defender、路徑權限與字型顯示。本 Docker gate 不宣稱 Windows 執行 smoke。
- macOS：`lipo -info` 必須同時列 x86_64、arm64；Intel 與 Apple Silicon 真機各跑
  正常玩家路徑，驗 Metal/視窗、HiDPI、音效、大小寫行為、簽章與公證。Docker
  Linux 不能取代這些 gate。

原版資料永遠是輸入，不是公開交付物；不得把 local-full 目錄掛入 Git 或任何上傳
流程。腳本不建立 GitHub Release，也不執行 commit/push。
