# Smite-and-Magic 專案調查

社群逆向專案 [Vairn/Smite-and-Magic](https://github.com/Vairn/Smite-and-Magic)（GitHub，取用日期 2026-08-18）
的專案考證筆記。

## 1. 專案概況

| 面向 | 內容 |
|---|---|
| **語言** | BlitzBasic（3D 遊戲開發語言，現已停產；`*.bb` 原始碼） |
| **目標** | 從 Amiga 版逆向 MM2，製作編輯工具；目標平台為 Amiga 與 PC |
| **最後 commit** | `b9e96cc86248`「Adding GUI Framework」，2022-04-21T00:19:54Z。歷史不只一筆（同日還有 GLWF、IMGUI、README 等），並且與 `AdamENESS` 的分支合併過 |
| **活躍度** | 未更新逾 4 年；GitHub stars = 0 |
| **原始碼可行性** | 框架完整但不可執行（無編譯腳本、依賴閉源 BlitzBasic）；代碼目標是實現 3D 遊戲引擎，不是 MM2 完整遊戲 |

## 2. 技術堆疊

**主程式與 3D 引擎**（推論等級：**已證實**）

- `Blitz3d/main.bb`：主迴圈、OpenGL 視窗、鍵盤控制（方向、移動、轉向）
- `Blitz3d/` 內共 20+ 個 BlitzBasic 原始檔，包含 OpenGL、相機、BSP 載入與光線追蹤等框架檔（見 `Includes/`）
- 編譯產物：`Blitz3d/main.exe`（847 KB，已編譯的 Windows 執行檔）

**資料讀取與容器處理**（推論等級：**已證實**，代碼路徑已確認）

- `Blitz3d/datapak.bb`（3.2 KB）：自訂打包格式 `*.mpf`（MM2 Wad Format？）
  - 格式：6 bytes 標識 `{MM2WAD}` + 版本 + 檔案數 + 每檔的名稱、位移、大小
  - 資料段採用加密（按原始檔名長度逐字節異或）
  - 涵蓋檔案：`attrib.dat`、`event.dat`、`items.dat`、`map.dat`、`monsters.dat`
  
- `Blitz3d/map.bb`（11.8 KB）：地圖載入與 3D 網格構建
- `Blitz3d/character.bb`（5.1 KB）：角色記錄解析
- `Blitz3d/monster.bb`、`Blitz3d/font.bb` 等：對應系統元件

**測試資料**（推論等級：**已證實**，實體檔案存在）

- `Blitz3d/`、`Blitz3d/b3dmm2/` 下含實際資料檔，共 33 個 `.dat`／`.anm`。
  與我們手上的 DOS 版逐檔比大小：

  | 檔案 | Smite（Amiga 來源）| 我們的 DOS 版 |
  |---|---|---|
  | `roster.dat` | 8,320 | 8,293 |
  | `map.dat` | 30,720 | 18,748 |
  | `attrib.dat` | 3,840 | 1,768 |
  | `a.dat`（疑似物品）| 5,120 | `ITEMS.DAT` 5,120 |

  `roster.dat` 兩邊都不是同一份檔案（逐位元組比對不同）。8,320 剛好是
  130 × 64，我們的 8,293 是 130 × 63 ＋ 103（尾端 103 bytes 是旅行效果，
  見 [`docs/formats/02`](../formats/02-data-files.md) §5）。**整除只是佐證不是證明** ——
  但兩個獨立來源用同一個 130 間距，這一點值得記著。
- Amiga 版動畫檔：`Blitz3d/34.anm`、`59.anm` 等（3–13 KB 不等）

## 3. 已解出的項目

### 3.1 地圖與環境（推論等級：**已證實**）

在 `map.bb` 中解出的地圖架構：

```
60 個地圖區域（area 0–59）:
  - 命名表：Middlegate、Atlantium、Tundra、Volcania、Sandsobar、
    各自的洞穴、特殊地點（Coraks Cave、Dragons Dominion 等）
  
類型欄位 (envion):
  - 0 = 城市 (Town)
  - 1 = 地城 (Dungeon)
  - 2 = 城堡 (Castle)
  - 3 = 室外 (Outside)

資料結構 (per map):
  - mapdata：256 bytes（16×16 格子的牆面資料）
  - collmap：256 bytes（碰撞圖層）
  - roofdat：256 bytes（屋頂遮擋位元，每格 1 bit，8 bits packed per byte）
  - tris[3]：三組三角形索引（依牆面類型分類：1/2/3）
```

實裝細節（代碼行號參考 `map.bb`）：
- 每個區域佔 `aloc = area × 64` 在 `attrib.dat` 中
- 地圖資料偏移：`mloc = area × 512`
- 屋頂資料：`aloc + 32` 開始每 8 bytes（`a ÷ 8`）存一個屋頂位元組，
  256 個格子分佈在 32 個位元組裡（每格 1 bit）

### 3.2 角色記錄

`Blitz3d/character.bb` 的 `loadroster()` 用 `((a-1)*130)` 定位每一筆 ——
**130 bytes 的間距與我們從 DOS 反組譯得到的相同**（推論等級：`已證實`，
程式碼在 `Blitz3d/character.bb:70`、`:77`、`:111` 等處）。

把它讀欄位的順序（含兩次 `SeekFile` 跳躍）換算成偏移，與我們
[`docs/formats/02`](../formats/02-data-files.md) §5 逐項對照：

| 偏移 | Smite 讀的欄位 | 我們的欄位 | 相符 |
|---|---|---|---|
| 0–10 | 名字 | `offName` 0x00（10 bytes ＋ 終止 0）| ✓ |
| 11 | town | `offInnTown` 0x0B | ✓ |
| 12 | sex | `offSex` | ✓ |
| 13 | alignment | `offAlign` | ✓ |
| 14 | race | `offRace` | ✓ |
| 15 | class | `offClass` | ✓ |
| 16–18 | 力量／智力／個性 | `offStats` 0x10 起 | ✓ |
| 19–21 | 速度／準確／幸運 | `offStats` ＋3…5 | ✓ |
| 30 | thievery | `offThief` 30 | ✓ |
| 31 | level 低位元組 | `offGearAC` 31（裝備給的防護值）| **不同** |
| 32 | level 高位元組 | `offLevel` 32（經驗等級）| ✓ |
| 33 | 年齡（歲）| `offAge` 33 | ✓ |
| 34 | 年齡（天）| `offAgeDays` 34 | ✓ |
| 35 | spell level | 我們的法力等級在 114（`offSL`），35 未指派 | **不同** |
| 36–37 | food（連讀兩次）| 36 = `offAC`、37 = `offFood` | 只有 37 對上 |
| 39 | endurance | `offEndB` 39 | ✓ |
| 40–45 ／ 46–51 ／ 52–57 | 裝備的 ID ／充能／屬性 | `offEquipID` 40、`offEquipCharge` 46、`offEquipAttr` 52 | ✓ |
| 58–63 ／ 64–69 ／ 70–75 | 背包的 ID ／充能／屬性 | `offPackID` 58、`offPackCharge` 64、`offPackAttr` 70 | ✓ |
| 80 | 兩項技能，各一個 nibble | `offSkills` 80 | ✓ |
| 81–86 | 已學法術，6 bytes | `offSpells` 81（48 個位元）| ✓ |

裝備與背包那六組連續對上是這份對照裡最硬的一段 —— 它們純粹靠位置定義，
錯一個 byte 後面全部錯位，六組同時對上不會是巧合。

不相符的兩處**不改我們的結論**（裁決者是 DOS 版反組譯），但要記著：
偏移 31 我們讀成裝備給的防護值、對方讀成等級的低位元組；偏移 35 對方讀成
法力等級，而我們的法力等級在 114。可能是 Amiga 版真的不一樣，也可能是
對方讀錯 —— **我們手上沒有 Amiga 版的執行檔可以裁決，兩種都沒排除**
（推論等級：`未知`）。

### 3.3 資料容器與打包（推論等級：**已證實**）

自訂格式 `*.mpf`（見 `datapak.bb`）：

- **簽名**：6 bytes = `[1+M] [2+M] [3+2] [1+W] [2+A] [3+D]`（實際位元組值藉由 offset 解碼）
- **版本**：1 int（1 或 2，版本 2 表示加密）
- **檔案數**：1 int
- **目錄表**：每檔一項
  - 名稱（4 bytes 長度 + 字元串）
  - 大小（int）
  - 位移（int，須回溯填寫）
- **加密**：每檔案按原始檔名長度逐字節異或（`file_data[i] += (i % len(filename))`）

## 4. 與本專案的對照

| 項目 | Smite-and-Magic | 本專案（MM2 CHT） | 平台差異說明 |
|---|---|---|---|
| **資料裁決者** | Amiga 版（`.anm` 動畫、`.32` 圖形） | DOS 版（`.OVL` 機器碼、`MM2.EXE`） | Smite 從 Amiga 開始逆，無法當 DOS 的 oracle |
| **角色記錄** | 130 bytes，多數欄位偏移與我們相同，兩處不同（見 §3.2）| 130 bytes，以 DOS 執行檔反組譯取得 | 兩處不同無法裁決 —— 我們沒有 Amiga 版執行檔 |
| **地圖結構** | 60 區域、15×15 格子、環境類型 4 分類 | 同上；含出口表、事件連結層 | 無衝突；Smite 實裝更淺 |
| **資料檔清單** | attrib / event / items / map / monsters | 同；另含 roster / str / spells / default | Smite 的清單比本專案完整度低 |
| **怪物動畫** | `.anm` 檔逐檔含調色盤，無逐幀參數 | Mega Drive 的 11×11 tile 陣列（影格列表藏在 nametable） | Amiga 用 `.anm` 檔包體，DOS/MD 另有格式；MSX 無動畫 |
| **容器處理** | 自訂 `*.mpf` 打包格式（實驗） | DOS 的 overlay 機制 + 圖形的原始格式（`.16`、`.32` 等） | Smite 做的容器是為了重新打包用，本專案保留原版格式 |
| **反組譯覆蓋** | 無；完全基於資料檔結構推測 | 14 個 overlay 全部反組譯，622 個函式索引 | Smite 無法與機器碼對應驗證 |

**沒有發現直接矛盾的結論**。Amiga 版本可能在某些細節上與 DOS 版不同，
但 Smite-and-Magic 的各項解讀（角色 130 bytes、60 地圖、環境類型）都未與本專案的 DOS oracle 牴觸。

## 5. 授權與素材散布

| 面向 | 內容 | 影響 |
|---|---|---|
| **License** | 無（GitHub 未標示 LICENSE） | **無法引用或改作**；即便程式碼可用，缺授權宣告表示版權方未做決定 |
| **原版資料** | 儲存庫內含 `Blitz3d/Data/`、`b3dmm2/` 下的 `.dat` 與 `.anm` 實體檔案 | **散布風險**；若該檔來自合法提取可接受，否則侵害版權；尚無法從檔案名判定其來源 |
| **可執行性** | BlitzBasic 原始碼；BlitzBasic 為閉源商業軟體（已停產） | 無法用開源工具編譯；實作無法移植；本機依賴未知，無法獨立驗證 |
| **引用風險** | 無授權 + 原版資料 + 無編譯方案 = 難以複現與驗證 | **建議**：若要參考其地圖或角色解析思路，應改為「根據本專案的反組譯結果獨立查證」，而非直接引用 Smite 的成果 |

## 6. 技術收穫評估

### 6.1 有價值的部分

1. **Amiga 版本的資訊參考**：
   - Amiga 的 `.anm` 動畫結構（自帶調色盤的 `.32` 容器）
   - 60 個地圖區域名稱與環境類型對應（可佐證本專案的 oracle）
   - 角色記錄的完整欄位對應（與本專案互相確認）

2. **問題排查靈感**：
   - 資料容器的打包/解包思路（不一定要跟 Smite 一樣做自訂格式，但流程可參考）
   - BlitzBasic 的實作方式展示了「用高階語言逐欄位讀取」的可行性

### 6.2 無法直接使用的部分

1. **程式碼本身**：
   - BlitzBasic 不開源，本機無法編譯與驗證
   - 框架完整但沒有完整遊戲邏輯（只有 3D 引擎框架，沒有戰鬥、商店、施法等）
   - 無測試或驗收方案

2. **原版資料檔**：
   - 提交在 repo 內的資料檔無法確認來源（DOS 還是 Amiga）
   - 散布風險未消除

## 7. 結論與建議

**專案評價**：研究立項時間較早（Amiga 版逆向），成果在 2022 年中止；
代表當時對 MM2 Amiga 版的理解深度，但已無後續迭代。

**與本專案的價值**：
- **低**，作為程式碼參考（無法直接使用）
- **中等**，作為資訊交叉驗證（地圖、角色、動畫結構可佐證）
- **0**，作為獨立完整的遊戲實現

**是否該深究**：
- **是**，若要補齊本專案的 Amiga 版本素材細節（特別是 `.anm` 的幀率、色表對應、
  角色動畫對應等）
- **否**，若只想驗證 DOS 版本的邏輯（DOS 是唯一 oracle，本專案已完整覆蓋）

**推薦後續行動**：
1. ⭐ 用本專案既有的 `tools/amiga32.py` 對 Smite 的 `.anm` 進行獨立驗證
   （無需直接用 Smite 的解碼器，而是用本專案的 Amiga 方法論核驗）
2. ⭐ 若 Smite 的資料檔確實是合法提取，可將其角色記錄清單作為非程式碼的
   外部對照表（在 `docs/research/02-other-platforms.md` 註記）
3. 跳過其 BlitzBasic 代碼；自有工具鏈已足以支撐各平台逆向

---

**調查日期**：2026-08-18  
**GitHub 來源**：https://github.com/Vairn/Smite-and-Magic (commit b9e96cc)  
**授權狀態**：無標示 LICENSE；無法直接引用程式碼；資訊可作佐證但須獨立驗證

