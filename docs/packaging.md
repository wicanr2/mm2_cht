# 平台封裝

目前 HD 完整版使用 [版本化交付規格](spec/hd-full-release.md)，完整包集中於
`dist-all/<VERSION>/full-local/`。`tools/package.sh` 已改用 VERSION、離線既有工具鏈與
工作樹內快取；它只以 Docker 執行建置與封裝。舊 `collect_dist_all.sh` 不再用於現行交付。
正式建置也可直接在受限容器呼叫 `tools/package_container.sh` 與 `tools/pack_wrap.sh`，
使用獨立平台舞台目錄；後者以 `MM2_FLAT_OUTPUT=1` 輸出至版本目錄。
直接呼叫編譯步驟時，以 `MM2_SOURCE_COMMIT` 傳入主機已核對的完整 commit；腳本檢查 40 位十六進位識別碼。`tools/package.sh` 會自動傳入，macOS 交叉工具鏈不需要另裝 Git。
素材由 `tools/stage_full_assets.py` 擷取執行所需檔案，配樂由 `tools/prepare_release_music.py`
轉為 OGG。來源與資源雜湊隨包保存，母圖、快取、舊存檔不入包。

Android 本機完整版另由 `tools/build_android.py` 建置，交付與驗證入口見
[Android 規格](spec/android-mobile.md#建置與核對入口)。只提供本機 debug 簽章 APK，
含原版資料與衍生素材，不作商店或公開發行。`tools/verify_android.py --deliver` 必須核對
同一 APK 的啟動、正常操作／原生輸入及購買／使用收據，才建立 `dist-all/<VERSION>/`；
不覆寫既存版本目錄，也不把測試輸入法、簽章私鑰或開發快取加入完整包。

目標平台是 Linux x64、Windows x64 與 macOS universal（Intel ＋ Apple Silicon）。
每個平台各出兩種模式：**public**（可散布，玩家自備原版資料）與
**local-full**（含原版資料，只留本機）。

| 平台 | 產物 | 玩家怎麼開 |
|---|---|---|
| Linux x64 | `*.AppImage`（單檔）| `./檔名.AppImage /path/to/MM2` |
| Windows x64 | `*.zip` | 解開後 `run.bat C:\MM2` |
| macOS universal | `*.zip`，裡面是 `MM2-CHT.app` | 雙擊，第一次問原版資料在哪 |

## 兩步封裝

`tools/package.sh` 先呼叫 `tools/package_container.sh` 編譯，再由
`tools/pack_wrap.sh` 收集素材並封裝。沿用既有的
`san1-go:1.26.7-ebiten2.9.9-r1`、`san1-macos:1.26.7-sdk15.5-r1` 及
`kol-package:1.26.7-ebiten2.9.9-20261011-r1`；映像須已存在，不自動改用其他版本。
macOS SDK 位於 `/osxcross/SDK/MacOSX15.5.sdk`，使用 `o64-clang`／`oa64-clang` 與 `lipo`。
macOS 最低版本為 12.0，與 [Go 1.26 的平台契約](https://go.dev/doc/go1.26) 一致。
建置與封裝均離線、非 root、有資源限制及外層逾時，快取保存在 `workplace/cache/`。

```bash
# 配樂先以 tools/prepare_release_music.py 在封裝容器產生正式 OGG 包。
bash tools/package.sh local-full linux-x64 --data-dir /private/MM2 \
  --music-pack-dir /private/mm2-ogg
# 另兩平台沿用同一資料與 OGG 包，只換第二個參數。
bash tools/package.sh local-full windows-x64 --data-dir /private/MM2 --music-pack-dir /private/mm2-ogg
bash tools/package.sh local-full macos-universal --data-dir /private/MM2 --music-pack-dir /private/mm2-ogg
```

公開 patch 與完整包分別進入被忽略的 `dist-all/<VERSION>/patch/` 與 `full-local/`，
並在腳本開頭檢查 Git 排除規則。完整包不得提交、推送或上傳。

## 包裡有什麼

公開包：`bin/`（`mm2`、`mm2data`）、三份可公開的 base data、字型、
繁簡中文譯文、`translations/interface.tsv`、`translations/simplified-map.json`、
顯示字型與 Noto／OpenCC 授權、圖示、README、
釋出政策、`PACKAGE-MANIFEST.txt`。allow-list 逐檔比對，多一個少一個都失敗。

顯示資源清單由 `tools/package_container.sh` 的 `display_fonts` 與
`display_translations` 維護，包含 `input-tc24.bin`／`input-sc24.bin` 輸入字型、
`toolbar-sans24.bin`／`toolbar-sans30.bin`／`toolbar-sans36.bin` 工具列等高大寫字模、
`input-aliases.tsv` 及字型生成收據。Go 相依來源固定於 `vendor/`，Linux XIM
修補由 `tools/patch_ebiten_ime.py` 驗證並套用；重新執行 `go mod vendor` 後須在
容器內重跑修補工具。Amiga 原始資料、怪物包與手繪 HD 包留在本機，
不加入公開封裝；HD 圖包契約見 [HD 規格](spec/amiga-hd-toolbar.md)。

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
| Windows | `%LOCALAPPDATA%\mm2-cht`，字型與譯文複製至工作根，封包可唯讀。 |

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

現行完整包以 [dist-all 版本索引](../dist-all/README.md) 為準：桌面三平台為
`v.1.1.0-20261111/full-local/`，Android 為 `v.1.2.0-20261111/full-local/`。
各正式版本有自己的 `SHA256SUMS.json`。正式版號制度前的 commit 版保留在
`dist-all/ffa373aa81e7/`，舊包程式與檔名不變，沒有重新編造正式版號；
搬移核對入口為 `tools/archive_legacy_dist.py`，須在非 root Docker 中把 dist-all 掛到 `/deliver` 執行。

舊 `tools/collect_dist_all.sh` 已停用，避免整個刪除 dist-all 或重建未版本化輸出。
以下表格與抽測是 2026-08-20 的歷史紀錄，不作目前檔案清單。

| 平台 | 完整版 | 大小 | 公開版 | 大小 |
|---|---|---|---|---|
| Linux x64 | `…-local-full-1d6b46209d82.AppImage` | 113.6 MB | `…-public-1d6b46209d82.AppImage` | 9.1 MB |
| Windows x64 | `…-local-full-1d6b46209d82.zip` | 111.7 MB | `…-public-1d6b46209d82.zip` | 7.9 MB |
| macOS universal | `…-local-full-1d6b46209d82.zip` | 119.6 MB | `…-public-1d6b46209d82.zip` | 15.8 MB |

舊 `.local-full/` 舞台曾於當時清理。現行完整包按上述版本索引取用；
重建入口為 `tools/package.sh local-full`，輸入與驗證方式見前文。

那一輪驗過的：Linux 在 Xvfb 下實跑並截三張圖（探索、`F2` 設定、`F4` 存檔），
中文全部畫得出來，存檔寫進 `~/.local/share/mm2-cht/save/`（`ROSTER.DAT` ＋
`state.json`）；Windows 是 PE32+ x86-64 且**只匯入 `kernel32.dll`**（不必夾帶
DLL），`run.bat` 是 CRLF ASCII；macOS 是雙架構 fat 檔，**arm64 那一片帶
`LC_CODE_SIGNATURE`、x86_64 沒有**（Apple Silicon 對未簽章的 arm64 會直接
`Killed: 9`，Intel 那一片不需要）。兩個 zip 各 115／121 筆、全部標了 UTF-8 旗標，
原版資料 63 檔、音樂 17 檔都在包內。集中之後又拿 `dist-all/` 裡的那一份
AppImage 再跑一次（走路、`F4`），畫面與存檔都正常 —— **刪掉 `.local-full`
之前先驗集中區那一份**，因為刪完它就是唯一的副本。

已驗（Docker 內，可重跑）：

- Linux：AppImage `--appimage-extract` 解得開；解出來的 `AppRun` 在 Xvfb 下
  跑滿 12 秒沒有崩，工作根長出 15 個 data 檔與 `save/`，譯文與字型從
  相對路徑讀得到；給錯目錄與不給目錄都被擋下並印出中文訊息。完整包不給
  參數也跑得起來（原版資料與 16 首音樂都在包內）。
- **畫面要截圖看過，不能只看「沒有崩」。** `mm2-go` 裡就有 `Xvfb`、`xdotool`
  與 ImageMagick 的 `import`：起 `Xvfb :99`、`DISPLAY=:99` 跑 `AppRun`、
  送幾個 `Return`，再 `import -window root`（沒有視窗管理員，遊戲窗就落在
  root 上）。要看的是隊伍面板、`'O' 選項`／`Day=`／`Year=`／`Face=` 那一列
  與訊息框**有沒有字**。
  這一條是 2026-08-20 補的：`cmd/mm2` 把畫布送進 Ebiten 之前多 `Flush()`
  一次，把整層中文洗掉，三個平台的包畫面都只剩空框 —— 而**全部 Go 測試是
  綠的、封裝檢查也過、程式跑滿 12 秒沒崩**。離屏測試直接讀 `Session.Draw()`
  的結果，結構上就看不到「送進視窗那一段」出的錯。
- 沒有音效裝置的機器上，遊戲照樣跑，只是沒有音樂。這是完整包的 smoke 抓到的：
  容器裡沒有 ALSA，而完整包一定帶音樂包，於是 Ebiten 的音訊 context 一建立
  就開不起裝置，錯誤從 `RunGame` 冒出來把整個遊戲帶走。那個錯誤事後攔不住
  （沒有公開 API 問得到），所以改成**事前**看裝置在不在（Linux 看 `/dev/snd`
  或 PulseAudio 的 socket），沒有就根本不建 context。

未驗（要真機，Docker 取代不了）：

- Windows 真機執行：啟動、讀原版資料、移動、存檔／重載、離開，另驗 Defender、
  路徑權限與字型顯示。（Linux 那條的截圖驗收在 Windows／macOS 上沒有對應做法，
  兩邊的執行檔只能靠「同一個 commit、同一份 `internal/*`」推。）
- macOS 真機執行：Intel 與 Apple Silicon 各跑一次，驗 Metal／視窗、HiDPI、
  音效、大小寫行為，以及 Gatekeeper 放行流程。
- Linux 宿主真機：視窗、GPU／音效、鍵盤、存檔重載；以及沒有 FUSE 的系統上
  `--appimage-extract-and-run` 的實際行為。

原版資料永遠是輸入，不是公開交付物。腳本不建立 GitHub Release，也不執行
commit／push。
