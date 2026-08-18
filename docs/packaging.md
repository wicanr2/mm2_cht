# 三平台封裝

目標平台是 Linux x64、Windows x64 與 macOS universal（Intel ＋ Apple Silicon）。
每個平台各出兩種模式：**public**（可散布，玩家自備原版資料）與
**local-full**（含原版資料，只留本機）。

| 平台 | 產物 | 玩家怎麼開 |
|---|---|---|
| Linux x64 | `*.AppImage`（單檔）| `./檔名.AppImage /path/to/MM2` |
| Windows x64 | `*.zip` | 解開後 `run.bat C:\MM2` |
| macOS universal | `*.zip`，裡面是 `MM2-CHT.app` | 雙擊，第一次問原版資料在哪 |

## 兩步封裝

`tools/package.sh` 先在編譯用的 image 裡排出「舞台目錄」
（`tools/package_container.sh`：編 binary、挑檔案、跑公開包的 allow-list），
再在封裝用的 `mm2-pkg` image 裡封成最終格式（`tools/pack_wrap.sh`）。

分成兩步的理由是工具鏈不在同一個 image 裡：macOS 的交叉編譯要 osxcross 那個
image，而 AppImage 需要的 `mksquashfs` 裝在 `mm2-pkg`（由 `tools/Dockerfile.pkg`
從 `mm2-go` 疊上去）。兩步都是 `--network none`、`GOPROXY=off`。

```bash
docker build -f tools/Dockerfile.pkg -t mm2-pkg:latest .   # 只要建一次

bash tools/package.sh public linux-x64
bash tools/package.sh public windows-x64
bash tools/package.sh public macos-universal

bash tools/package.sh local-full linux-x64 --data-dir /private/MM2
bash tools/package.sh local-full windows-x64 --data-dir /private/MM2
bash tools/package.sh local-full macos-universal --data-dir /private/MM2

# 只有本機完整版可選配原版音樂包
bash tools/package.sh local-full linux-x64 --data-dir /private/MM2 \
  --music-pack-dir /private/mm2-music

bash tools/collect_dist_all.sh    # 六個包 ＋ 推廣片集中到 dist-all/
```

公開包輸出到被忽略的 `dist/public/`，完整包到 `.local-full/`，兩者都在腳本開頭
用 `git check-ignore` 擋一次。完整包不得 `git add`、commit、push 或上傳。

## 包裡有什麼

公開包：`bin/`（`mm2`、`mm2data`）、三份可公開的 base data、字型、
`translations/zh-Hant.json` 與 `translations/md-flavor.json`、圖示、README、
釋出政策、`PACKAGE-MANIFEST.txt`。allow-list 逐檔比對，多一個少一個都失敗。

完整包多了 `original-data/`（玩家指定那份）與可選的 `music/`；runtime data 在
封裝時就由容器宿主目標（Linux amd64）的 `mm2data-host` 產生好，所以不會在
Linux 容器裡執行 Windows 或 Mach-O 執行檔。

圖示由 `tools/make_icon.py` 畫出來（幾何圖形，EGA 色票），**沒有一個像素來自
原版素材**，所以公開包可以帶著它。

## 唯讀的包怎麼存檔

引擎讀 `translations/`、`assets/` 與寫 `save/` 都相對於工作目錄，而 AppImage
掛起來是唯讀的，`.app` 也不該被寫。啟動器因此在使用者的資料目錄開一個可寫的
工作根，把唯讀的兩個資料夾連過去，`data/` 與 `save/` 放真檔，再 `cd` 進去執行：

| 平台 | 工作根 |
|---|---|
| Linux | `${XDG_DATA_HOME:-$HOME/.local/share}/mm2-cht` |
| macOS | `$HOME/Library/Application Support/mm2-cht` |
| Windows | 解壓出來的包目錄本身（本來就可寫，不另外開）|

AppImage 的掛載點每次都不同，所以那兩個連結每次啟動都重建。

## Windows 的 zip 與 DLL

zip 由 `tools/pack_zip.py` 產生，**每一筆都明確打開 UTF-8 旗標**
（general purpose bit 11）。`zip -r` 只在檔名超出 CP437 時才標，全 ASCII 的
不標 —— 沒標的話解壓端只能用系統預設編碼猜，在繁中 Windows 上就是 CP950。
旗標不能在寫入時設：`ZipFile._open_to_write` 會把 `flag_bits` 歸零再自己填，
所以寫完要回頭改位元組，中央目錄與區域檔頭兩處都改。腳本自己會複驗一次。

DLL 不夾帶，理由寫在包內的 `WINDOWS-DLL.txt`，清單由 `tools/pe_imports.py`
從**包裡那支執行檔的 PE 匯入表**實際讀出來：只有 `kernel32.dll`。執行檔是
`CGO_ENABLED=0` 的純 Go，沒有 C 執行期，也就沒有 MSVC redistributable 的相依；
其餘 DLL（`d3d11`、`dxgi`、`opengl32`、`xinput*`、`winmm` …）都是 Ebiten 在
執行時視情況 `LoadLibrary` 的 Windows 系統元件。唯一可能缺席的
`d3dcompiler_47.dll` 屬微軟，本專案不代為散布；缺了它 Ebiten 會退回 OpenGL。

## macOS 的 .app

`MM2-CHT.app` 含 `Info.plist`（`CFBundleDisplayName` 是「魔法門 II 繁體中文版」）、
`mm2.icns` 與一支 shell 啟動器。雙擊時沒有終端機，所以錯誤用 `osascript` 的
對話框講，公開包的資料目錄用 `choose folder` 問，問過一次記在
`Application Support/mm2-cht/data-dir`。

執行檔由 `o64-clang`／`oa64-clang` 分別編出 x86_64 與 arm64，再 `lipo` 合併；
`PACKAGE-MANIFEST.txt` 記下 image、SDK 與架構。zip 保留 Unix 權限位元
（`Contents/MacOS/` 底下那支解開後仍是可執行檔）。

**沒有簽章也沒有公證**，第一次開會被 Gatekeeper 擋，包內 `README-macOS.txt`
寫了兩種放行方式。

## 已驗與未驗

已驗（Docker 內，可重跑）：

- Linux：AppImage `--appimage-extract` 解得開；解出來的 `AppRun` 在 Xvfb 下
  跑滿 12 秒沒有崩，工作根長出 15 個 data 檔與 `save/`，譯文與字型從
  相對路徑讀得到；給錯目錄與不給目錄都被擋下並印出中文訊息。完整包不給
  參數也跑得起來（原版資料與 16 首音樂都在包內）。
- 沒有音效裝置的機器上，音樂失敗只關音樂不關遊戲。這是完整包的 smoke
  抓到的：容器裡沒有 ALSA，而完整包一定帶音樂包，於是 `Update` 回一個
  裝置層的錯誤、`RunGame` 收攤、`log.Fatal` —— 遊戲當場退出。音樂包在
  載入時就逐首解碼驗過，跑到那裡的錯誤只會是裝置層的，所以現在記一行
  就把音樂關掉繼續玩。
- Windows：PE 是 x86_64 PE32+，匯入表只有 `kernel32.dll`；zip 22 筆全部帶
  UTF-8 旗標；`run.bat` 全 CRLF。
- macOS：`Mach-O universal binary with 2 architectures`（x86_64 ＋ arm64），
  zip 解開後 `Contents/MacOS/mm2-cht` 仍是 `0755`。

未驗（要真機，Docker 取代不了）：

- Windows 真機執行：啟動、讀原版資料、移動、存檔／重載、離開，另驗 Defender、
  路徑權限與字型顯示。
- macOS 真機執行：Intel 與 Apple Silicon 各跑一次，驗 Metal／視窗、HiDPI、
  音效、大小寫行為，以及 Gatekeeper 放行流程。
- Linux 宿主真機：視窗、GPU／音效、鍵盤、存檔重載；以及沒有 FUSE 的系統上
  `--appimage-extract-and-run` 的實際行為。

原版資料永遠是輸入，不是公開交付物。腳本不建立 GitHub Release，也不執行
commit／push。
