# HD 三平台完整版與推廣影片

狀態：CONFORMED（本機交付範圍）。使用者於 2026-11-11 明確要求完成 HD 與工具列後，重包三平台完整版及推廣影片。既定手繪風格、繁簡輸入與原圖切換沿用 [顯示規格](amiga-hd-toolbar.md)。

- 版本唯一來源為根目錄 `VERSION`，本輪為 `v.1.1.0-20261111`，日期依使用者環境 Asia/Taipei。封包、程式版本、manifest 與交付目錄使用同一字串，不建立或變更遠端 tag／Release。
- 現行交付放 `dist-all/<版本>/full-local/`、`promo/`、`smoke/`，另有 `SHA256SUMS.json`。既有非版本化產物保留，不執行舊版會刪除整個 dist-all 的收集腳本。
- 完整版包含 DOS 原始資料、所有可用平台素材、Amiga 怪物與完整 HD 圖包，以及已驗證的 Mega Drive 音樂角色檔。來源唯讀；不帶入工作快取、生成母圖、錄影中間檔或使用者存檔。
- 三平台為 Linux x64 AppImage、Windows x64 ZIP、macOS Intel／Apple Silicon universal ZIP。完整包無須另找素材路徑；啟動器建立自己的可寫工作目錄與存檔，不改原始資料。
- 補齊仍可見的 HD 片頭與工具列收尾。動畫定位與資料若不足，先查證或製作可丟棄對照，不以未證實順序宣稱原版動畫還原。
- 使用者已選完整 HD 片頭動畫。以 Amiga 原始疊圖核對落點、重製每個動作，保留既有原圖片頭；HD 動畫屬重製編排，不宣稱 Amiga 原版時序對拍。
- 推廣片沿用約 72 秒的長度，改以新版 HD、原圖切換、繁簡、文字輸入、探索、設施與戰鬥為主。使用目前程式重拍的實機內容，音源取自本機 Mega Drive 擷取包；不採用舊版 PC 喇叭近似合成。
- `tools/record_hd_promo.sh` 錄製封包內程式的正常操作；`cmd/mm2promo` 以固定種子、原始怪物資料產生戰鬥示範，走同一 UI／規則，但明確記為受控 fixture，不充作正常玩家路徑驗收。片頭生成提示與母圖收據保存在本機 HD 圖包的 `masters/`。
- 封包驗證檔案、架構、必要素材、版本、manifest、SHA-256、啟動與存檔；Windows 以 Wine 輔助抽測，macOS 以架構與簽章檢查，不代稱真機驗收。
- `tools/verify_hd_release.py` 對最終 AppImage 解包內容與 ZIP 逐檔驗證，包含三包來源清單相同、930 筆 HD、16 個 OGG 角色、UTF-8、macOS 雙架構與 arm64 ad-hoc 簽章各頁 SHA-256。
- 封包素材由 `tools/stage_full_assets.py` 收斂為執行所需檔案；`tools/prepare_release_music.py` 以 FFmpeg 將既有實錄 WAV 轉成 OGG Vorbis quality 6，保留來源、增益與輸出雜湊。播放器仍能讀既有 WAV。
- 影片驗證解析度、幀率、時長、視音訊、音量、黑幀、凍結、逐幕字幕與素材來源，保存接觸表及抽樣幀。技術音訊檢查不冒稱人耳確認。
- `tools/make_hd_promo_frames.py` 製作字幕版面，`tools/render_hd_promo.sh` 合成 72 秒影片。遊戲視窗保留完整比例，字幕放在側欄；中文輸入使用本輪 IBus 實錄截圖，戰鬥示範的種子為 `0x1111`。
- `tools/verify_hd_promo.py` 完整解碼影片，驗證 1080p／30 fps／72 秒、立體聲 AAC、峰值與非靜音、遊戲視窗黑幀及靜止觀察；輸出逐幕抽樣、接觸表與來源雜湊。地圖、選單及輸入截圖的停留會明列，不以字幕活動掩蓋。

完整包與原版配樂影片維持本機素材的既有散布邊界。這次授權包含本機重包與影片產製，不包含公開上傳。工作紀錄集中在 [WORKLOG](../../WORKLOG.md)，目前狀態由 [CONTEXT](../../CONTEXT.md) §1.5 索引。

## 驗收結果

- 三平台完整包的 ASSET-MANIFEST 全數匹配，SOURCE-MANIFEST 三包一致，內含 RRSAL-1.0 與第三方授權。正式大小與雜湊見版本根目錄 SHA256SUMS.json。
- 完整 Go 回歸通過，UI 73.269 秒；go vet 無輸出。回歸涵蓋錯誤職業選擇後再進入 Unicode 命名，舊錯誤提示不再殘留。
- Linux 最終 AppImage 以解壓執行模式實測內附資料、HD 邊框像素、移動與新行程存讀檔。完整 GUI 路徑另覆蓋工具列、繁簡、原圖、物品、商店及步行進出神殿。Windows 最終 ZIP 唯讀掛載執行 run.bat，存檔落在使用者目錄且可重載。
- macOS x86_64／arm64 與最低 12.0 結構通過；arm64 兩個執行檔分別驗證 2,960／788 個 ad-hoc 簽章頁。不代稱 Apple 開發者簽章、公證或真機驗收。
- Linux 原生拼音 Enter、退格、Esc 及 Unicode 姓名重載通過。錄影先確認命名頁，再輸入拼音，避免在性別選單消耗輸入。
- 影片為 72.000 秒、1920×1080、固定 30 fps，共 2,160 幀；立體聲 AAC 48 kHz、約 -18.4 LUFS、真峰值約 -11.5 dBFS，完整解碼及遊戲區域黑幀檢查通過。逐幕畫面已目視；靜止觀察列在 PROVENANCE.json，未宣稱全程連續運動。
- smoke/VALIDATION.json 明列映像、實際執行檔與正式封包一致性及限制。戰鬥以受控 fixture 抽測一般／快速分支，不冒稱本輪由起點走到隨機遭遇。
- Windows／macOS 真機及原生輸入法、音訊聽感仍待確認。HD 片頭是重製編排，Amiga 原版時序 oracle 未知，不阻塞已授權的視覺重製與本機封裝。
