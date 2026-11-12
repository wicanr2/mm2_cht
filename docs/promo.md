# 《魔法門 II》HD 與 Android 推廣片

已完成 [72 秒、1920×1080、30 fps 的桌面與 Android 推廣片](../dist-all/v.1.2.1-20261112/promo/mm2-cht-hd-android-promo-v.1.2.1-20261112.mp4)。
[九幕總覽](../dist-all/v.1.2.1-20261112/promo/contact-sheet.jpg)、逐幕 PNG 與驗證收據保存在同一版本的 `promo/`；實際完成狀態以
[CONTEXT](../CONTEXT.md#current-status) 為準。各平台完整包由
[dist-all 版本索引](../dist-all/README.md) 取用，保留各自程式的實際版號。

## 分鏡

| 時間 | 畫面 | 呈現方式 |
|---|---|---|
| 0–6 秒 | Amiga 手繪片頭 | 桌面完整包新錄影 |
| 6–14 秒 | HD 探索與石雕邊框 | 桌面正常操作 |
| 14–22 秒 | 繁簡、原圖／HD與工具列 | 桌面正常操作 |
| 22–30 秒 | 手繪怪物與攻擊目標 | 修訂版 UI 的受控戰鬥示範 |
| 30–42 秒 | Android 橫向雙手操作 | 正式 APK 的實際畫面擷取 |
| 42–54 秒 | Android 隊員、繁簡與地圖 | 正式 APK 的實際操作 |
| 54–62 秒 | Android 原生姓名欄 | 同版 APK 的原生文字欄實測截圖 |
| 62–68 秒 | 中文姓名、組字與保存 | 修訂版 Linux 輸入法實測截圖序列 |
| 68–72 秒 | 四平台收束 | 桌面完整版與 Android 本機測試版 |

桌面沿用深色側欄，Android 改用寬畫面，字幕置於遊戲範圍之外。
配色取自目前 MM2 的深藍底、古銀框、金線與紅寶石，標題使用襯線字。
桌面段落使用工具列等高大寫修訂版；片頭、探索、工具列、戰鬥及 Linux 輸入法畫面均已重錄。

## 真實性與來源

- 桌面來源為 `v.1.2.1-20261112` 的 Linux 完整 AppImage，唯讀解包後在 Xvfb 正常操作重錄。存檔及偏好寫入專屬錄影目錄。
- Android 來源為已交付的 `v.1.2.0-20261111` APK，SHA-256 為 `09501c08be852c0c806de319aea105a19c6b7b677858712b1feaa33e30dfb926`。在 AOSP API23 x86 軟體模擬器由正常啟動器開啟，依觸控動作擷取實際 framebuffer，保留逐張時間戳、雜湊與操作狀態。
- Android 專段標明實際畫面擷取與加速剪輯，不冒稱手機真機、連續 30 fps 擷取或效能測試。系統首次全螢幕教學在拍攝前處理。
- Android 姓名欄採同一 APK 已通過驗收的組字與確定文字截圖，裁去測試鍵盤後放大應用程式原生文字欄。字幕標示實測截圖，不冒稱手機拼音實錄；完整 InputConnection 驗收收據仍在該版 smoke。Linux 中文姓名與組字段另使用已驗證的 Linux 輸入法畫面，字幕標示平台。
- 戰鬥段沿用 `cmd/mm2promo` 受控狀態、種子 `0x1111` 與正常 UI／規則；不作從正常起點走到該場戰鬥的驗收。
- 配樂直接複用前版已驗證成片中的 AAC 音軌，不重新合成音色。來源為 BlastEm／libvgm 擷取的 Mega Drive 原版曲目，原始 ROM、擷取工具及音源雜湊可回查 `workplace/genesis/music/manifest.txt` 與前版 `PROVENANCE.json`。

影片、錄影與原版衍生音樂只留本機。未新增公開上傳或素材散布授權。

## 重建與驗證

所有工作都在既有 Docker 工具鏈中，以目前使用者 UID／GID、資源上限及外層逾時執行。
來源掛到唯讀 `/src`，專用工作目錄 `workplace/toolbar-font/height-v.1.2.1/` 掛到可寫 `/out`。
Android 使用前片保留的原始擷取 `workplace/promo-android-v.1.2.0-20261111/android/`，不重複拍攝或宣稱為本輪新錄。

| 入口 | 工作 |
|---|---|
| `tools/record_hd_promo.sh` | 在 `kol-package:1.26.7-ebiten2.9.9-20261011-r1` 以 Xvfb 錄製已解包桌面程式。 |
| `tools/record_android_promo.sh` | 在 `rich2-android-emulator:api23-20261102-r1` 建立暫存 AVD，前景有界執行並由 trap 關閉。啟動前把此腳本、`record_android_promo.py` 與 `android_smoke.py` 複製到容器 `/tmp`，避免執行中修改來源；所需操作完成後若只在收尾超時，以實際 PNG 寫入時間與保存的遊戲診斷補齊來源收據。 |
| `tools/build_android_promo.py --work /out --version v.1.2.1-20261112 --android-source /src/workplace/promo-android-v.1.2.0-20261111/android --workspace workplace/toolbar-font/height-v.1.2.1` | 在上述 kol-package 映像合成及驗證；Noto 字型唯讀掛到 `/fonts`。 |

合成腳本驗證來源雜湊、畫面變化、黑畫面、視音訊完整解碼、72 秒／2,160 幀、立體聲 AAC、音量、靜音與字幕邊界。
視訊完成編碼後再封裝音訊，另比對 72 秒 AAC 封包雜湊與來源相同，避免視訊編碼延遲截短片尾。
原始 Android 擷取保留變動時間戳，最終輸出才鎖定 30 fps，避免拼接時間基準造成缺幀。
凍結觀察會保留；回合制選單、地圖、輸入欄、Linux 輸入法截圖序列及片尾的靜止會明列。
逐幕 PNG、接觸表、分鏡及 `PROVENANCE.json` 隨正式影片保存。音訊技術檢查不代替人耳驗收。

歷史 commit 版的兩支舊片保留在 `dist-all/ffa373aa81e7/promo/`，三平台 HD 前片保留在 `dist-all/v.1.1.0-20261111/promo/`；不覆寫歷史影片。
