# ScummVM 的 `mm` 引擎對 MM 系列的支援狀況

ScummVM 是老遊戲模擬與重製平台。本文調查其 `mm` 引擎（原名 `xeen`）是否涵蓋《Might and Magic II》(1988, DOS)，以及對我們的 MM2 remake 引擎有沒有可借的實作策略。

## 1. MM2 支援狀況

### 結論

**MM2 未被 ScummVM 的 `mm` 引擎支援**。

### 證據

#### 官方支援清單（已證實）
根據 ScummVM 官方相容性頁面（[scummvm.org/compatibility](https://scummvm.org/compatibility/)，取用日期 2026-08-18），`mm` 引擎支援的遊戲列表為：

- Might and Magic: Book One - Secret of the Inner Sanctum（MM1）
- Might and Magic: Clouds of Xeen（MM4）
- Might and Magic: Darkside of Xeen（MM5）
- Might and Magic: World of Xeen（MM4+MM5 合併版）
- Might and Magic: Swords of Xeen（**不是 MM6**：它是隨 World of Xeen 合輯附送的同人資料片，MM6《The Mandate of Heaven》是另一款 3D 遊戲，不在這個引擎裡）

**列表中無 Might and Magic II 或 MM2 的任何變體版本**。

#### 原始碼結構（已證實）
檢查 ScummVM 倉庫 `engines/mm/` 的結構（commit `HEAD` 日期：2026-08-18 12:34:44 UTC+2）：

```
engines/mm/
├── detection.h          定義支援的遊戲類型
├── detection.cpp
├── detection_tables.h   detection 表
├── mm1/                 MM1 專用實作
├── xeen/                Xeen 系列（MM4／MM5／Swords of Xeen）專用實作
├── shared/
│   ├── classic/         經典系列共用代碼（僅 PC speaker 驅動）
│   ├── xeen/            Xeen 系列共用代碼
│   └── utils/
```

`detection.h` 定義的 game type 列表：

```c
enum {
    GType_MightAndMagic1,      // MM1
    GType_Clouds,              // MM4: Clouds of Xeen
    GType_DarkSide,            // MM5: Darkside of Xeen
    GType_WorldOfXeen,         // World of Xeen (MM4+MM5)
    GType_Swords               // Swords of Xeen（同人資料片）
};
```

**不存在 `GType_MightAndMagic2` 或相關的類型**。

`detection_tables.h` 的 entry 列表逐一檢查，無任何 MM2 相關的簽名表（filename signatures）。

#### 版本時序
- MM1 實作：ScummVM 有全功能支援（包括原始版與增強版）
- MM4／MM5 與 Swords of Xeen：完整支援
- **MM2 與 MM3**：gap 存在，皆未實作

### 狀態分類

| 狀態 | MM1 | MM2 | MM3 | Xeen（MM4／MM5／Swords）| MM6 以後 |
|---|---|---|---|---|---|
| ScummVM 支援 | 已 | **不支援** | **未知** | 已 | 未調查 |
| 檔案格式 | 已解析 | 未解析 | 未解析 | 已解析 | 未調查 |

---

## 2. MM1 與 Xeen 的實作結構對比

### 資料層（Data Structure）

#### MM1 實作（`engines/mm/mm1/data/`）

已實作的資料結構：
- `character.cpp/h` — 單一角色記錄
- `party.cpp/h` — 隊伍（多角色）
- `roster.cpp/h` — 角色庫（存檔中的可招募角色）
- `items.cpp/h` — 物品定義與背包
- `monsters.cpp/h` — 怪物定義與實例
- `locations.cpp/h` — 戶外與室內位置
- `spells_state.h` — 法術狀態機
- `active_spells.cpp/h` — 作用中的法術跟蹤
- `game_state.cpp/h` — 全局遊戲狀態
- `trap.cpp/h` — 陷阱機制
- `treasure.cpp/h` — 寶藏與獎勵

#### Xeen 實作（`engines/mm/xeen/`）

已實作的資料結構與系統（部分列舉）：
- `character.cpp/h` — 角色系統（結構與 MM1 相異）
- `combat.cpp/h` — 戰鬥系統
- `events.cpp/h` — 事件觸發與對話
- `files.cpp/h` — 檔案格式解析（`.cc` 合併檔案）
- `dialogs/` — 對話系統
- 其他：debugger、cutscenes、interface 等

#### 結構層級對比（強推論）

| 層級 | MM1 | Xeen 系列 | 推斷適用於 MM2 |
|---|---|---|---|
| 角色記錄 | binary struct (130 bytes?) | 不同格式 | **未知** |
| 隊伍與 roster | 分離的 party/roster 結構 | 整合？ | **未知** |
| 物品與法術 | 類型編號 + 屬性 | 類型編號 + 屬性 | **可能相似** |
| 地圖與位置 | 戶外網格 + 室內 dungeon | Xeen 系列格式 | **未知** |
| 事件與腳本 | 事件表驅動 | 對話引擎驅動 | **未知** |

### 共用代碼

#### `shared/classic/` 
僅包含：
- `pc_speaker.cpp/h` — PC speaker 音效驅動（硬體輸出，非資料格式）

**結論**：無實質的資料格式共用。MM1 與 Xeen 各自獨立實作，說明兩代引擎在架構上有明顯分歧。

#### `shared/xeen/`
包含：
- `cc_archive.cpp/h` — 檔案打包格式（`.cc` 合併檔）
- `file.cpp/h` — 基礎 I/O
- `sound_driver_*.cpp/h` — 音樂驅動（AdLib、MT-32）
- `sprites.cpp/h` — 圖形精靈
- `xsurface.cpp/h` — 繪圖表面

**結論**：Xeen 專用的基礎設施。MM1 不使用這套。

### 代碼類推的限制（強推論）

1. **MM1 與 Xeen 的角色記錄格式不同** — 兩個引擎都重新定義了 character 結構，說明遊戲版本間有實質差異。
2. **檔案打包格式分化** — MM1 使用若干零散檔案（`.DAT`、`.16`）；Xeen 使用合併的 `.cc` 檔。MM2 在兩者之間，極可能各有一套。
3. **事件系統分化** — MM1 用事件表，Xeen 用對話引擎。MM2 的機制未知。

### 能否直接參考 Xeen 實作到 MM2

**不建議**。理由：
- 代碼結構專為 Xeen 設計（資料格式、引擎假設），直接移植會強行適配。
- 我們的裁決者是 DOS 版反組譯（見本專案 CLAUDE.md §2），ScummVM 的解讀不能拿來推翻。
- MM2 DOS 版應有更接近 MM1 的結構（同代），但細節未必相同。

---

## 3. 方法論可借之處

### 平台差異處理（已證實）

MM1 實作示例：
```
GF_ENHANCED = 1  // 增強版旗標
GF_GFX_PACK = 2  // 圖形包 mod 旗標
```

MM1 detection 表同時列 DOS 原版與增強版，由執行時旗標切換介面與行為。**可借的想法**：MM2 如有多平台版本（DOS、MSX、Amiga、Mega Drive），可用類似機制隔離版本差異。

### Overlay 與模組化

**未在 ScummVM `mm` 引擎中看到**。Xeen 使用 `.cc` 合併檔，MMI 使用零散檔。DOS MM2 使用 overlay（`.OVL` segment），ScummVM 沒有實裝過類似機制，這條路沒有現成參考。

### 存檔相容

ScummVM 的存檔格式（save game）與原版不相容。我們沒有調查 ScummVM 的存檔機制細節，所以**無法評估**是否有可借的設計。

---

## 4. 授權與代碼重用

### ScummVM 授權（已證實）

ScummVM 專案採用 **GPL v3** 授權（[scummvm.org/faq](https://www.scummvm.org/faq/)，取用日期 2026-08-18）。

GPL v3 的核心條款：
- 軟體及其修改版必須以相同授權（GPL v3）發佈
- 如果重新發佈（包含基於原代碼衍生的作品），必須公開原始碼

### 我們的做法

我們的 MM2 remake 引擎（Go / Ebiten）**不受 GPL 約束**（假設採用 MIT、Apache 或專有授權）。

#### 允許的做法
- **參考** ScummVM 的實裝思路（演算法、資料結構設計理念）並獨立重寫
- **引用** 不超過幾行的代碼片段並清楚標註出處（合理使用範疇）
- **閱讀與分析** ScummVM 原始碼以理解機制

#### 禁止的做法
- **直接複製** 任何實質代碼段（即使修改變數名）
- **聲稱基於 ScummVM 代碼** 而不採用 GPL v3
- **在商用發佈中嵌入** 未去除的 GPL 代碼

### 建議

**參考 MM1 的資料結構設計理念（字段順序、位元組對齊、狀態機），但根據 DOS 版反組譯重寫實作。** 不要試圖「稍微改改 ScummVM 的 MM1 引擎」來支援 MM2，投報率低且授權風險高。

---

## 5. 是否值得持續追蹤 ScummVM

### 結論

**短期不值得**。理由：

1. **MM2 支援遙遙無期** — ScummVM 專案多年未動 MM2，而 Xeen 系列已在十幾年前實裝完成。新增 MM2 的優先級顯然很低。
2. **獨立完成度更高** — 我們已有 DOS 版反組譯（14 個 overlay 全部解析完成），無需等待 ScummVM。
3. **設計自由度** — ScummVM 必須對所有平台版本相容；我們可以針對 DOS 版優化，實現更忠實的還原。

### 後續追蹤點（假設值得）

如果 ScummVM 在未來某日實裝 MM2 支援：
- 檢查 `detection.h` 新增 `GType_MightAndMagic2` 時
- 查看 `engines/mm/mm2/` 目錄的資料結構實裝
- 對比 DOS 版反組譯結果，尋找設計上的巧合或差異

**檢查方法**：
```bash
git log --oneline --all -- engines/mm/detection.h | grep -i mm2
git show <commit>:engines/mm/detection.h | grep -i mm2
```

---

## 6. 參考資源

- [ScummVM 官方相容性頁](https://scummvm.org/compatibility/) — MM 系列的完整支援清單
- [ScummVM `mm` 引擎原始碼](https://github.com/scummvm/scummvm/tree/master/engines/mm) — 目錄結構 & detection 表
- [ScummVM 授權 FAQ](https://www.scummvm.org/faq/) — GPL v3 條款與常見問題
- [Sev 的 GPL 分析文章](https://blogs.scummvm.org/sev/2009/06/23/gpl-scummvm-and-violations/) — 實際案例與風險評估

---

## 結論

ScummVM 的 `mm` 引擎涵蓋 MM1 與 Xeen 系列（MM4、MM5、Swords of Xeen），**MM2 不在支援清單內**，也沒有規劃跡象。兩套實裝各自獨立，代碼複用有限。MM1 的資料結構設計（角色記錄、隊伍、物品、怪物等）可作為設計靈感，但不應直接參考代碼。授權上，GPL v3 的約束意味著我們必須獨立實現，而非改寫 ScummVM 代碼。

根據本專案的工作紀律，**我們應專注於 DOS 版反組譯為唯一裁決者**，不依賴 ScummVM 的實現。

---

> 本筆記成文日期：2026-08-18  
> 引用源材料 commit 日期：2026-08-18 12:34:44 UTC+2  
> 取用 URL 日期：2026-08-18
