# 水域通行 oracle（`ds:16DA`／`ds:03D9`）

> 狀態：窄任務研究稿，供 remake 實作者使用。本文不授權把未證實語意直接寫成規則。
> 研究基準：2026-08-12；目前工作基準 `f6ca781ff6bb7cd63b0dd576e3cb68c8ec5fc495`。

## 證據與限制

本輪使用既有版控的 IDA 匯出組語與專案研究文件，沒有修改原始 DOS 檔或 IDA
資料庫。`workplace/ida/MM2.EXE.asm` 的 IDA 匯出標頭記錄：

* 輸入：`MM2.EXE`
* SHA-256：`631FACB658A39E0D438C451F8A43C9F6E2AEB774FC3843C1A9BAC1E14BF8C4D4`
* MD5：`6116E0ADF179D873C4D82DC75F3FAA7F`
* IDA 輸出標頭年份：2026；精確 IDA 版本未記錄，故不臆填版本。
* 位址基準：IDA 線性位址／`seg000`（`ds:` 為 DOS DGROUP 位移）；overlay 的
  `2CAST1`、`2PLAY` 位址另列為 overlay／映像位移，不與 root 位址混稱。

本輪沒有新的 DOSBox 正常路徑重播：指定容器可啟動，但目前沒有可由正常玩家
路徑直接取得、可重播水行術與水域的既有存檔；因此不把 remake 測試冒充原版
save/load oracle。mentor 在 `mm2-go:latest` 唯讀、無網路容器中重新核對版控的
`MM2.EXE.asm`；逆向輸入仍是既有 IDA 匯出，匯出標頭未記錄精確 IDA 版本，未
重建 `.i64` 或假填版本。以下「已證實」僅指 IDA 靜態控制流或已有文件交叉證據。

## 已證實

### 水域的實際擋路條件

root `MM2.EXE` 的 `sub_15E68`（IDA 線性位址 `seg000:15E68`，對應
`workplace/ida/MM2.EXE.asm` 約第 14200–14340 行）先以 `ds:59C8 & ds:59C6 & 55h`
判定有無方向障礙；野外分支（`ds:039D != 0`）再用 `sub_15F40`（`seg000:15F40`）
查 `ds:52B2` 的地形分類表。

對分類 4（水域），控制流是：

```text
terrain class == 4
    且 ds:16DA == 0x0A
    且 ds:03D9 == 0
        → 回傳訊息索引 6（"Can't swim!"）
```

因此：在這個分支中，`ds:03D9 != 0` 會解除水域阻擋；`ds:16DA != 0x0A` 時，
這條「Can't swim」條件本身不成立。不能把規則簡化為「所有水域都要水行術」。

同一函式的相鄰規則也已證實：分類 1 要 `sub_136A6(0x0B) >= 2`，分類 3 要
`sub_136A6(0x0D) >= 2`，否則回傳索引 5（`Impassable!`）。這可作控制流對照，
避免把水域條件誤當成技能人數條件。

### `ds:03D9` 是水行術旗標

既有 `docs/formats/09-spells.md` 以 `2CAST1.OVL` 的 handler 對照記錄：

* 法術引擎編號 19（水行術）→ `sub_1C8C8`（`2CAST1` overlay）→ `ds:03D9 = 1`。
* `internal/game/cast.go` 也保留同一原版位址註記，但這是 remake 實作對照，不是
  額外的原版證據。

root 的讀取端至少有三條可回查控制流：

1. `sub_15E68`（`seg000:15F10` 附近）：水域通行 gate，`cmp byte ptr ds:3D9h, 0`。
2. `sub_13700`（`seg000:13700`，`seg000:13700+0`）：把 `ds:03D9` 與 `03D8`、
   `03DA` OR 起來，參與日／時間推進後是否要處理世界事件。
3. `sub_147D8` 呼叫鏈內（`seg000:1493D` 附近）：若 `ds:03D9 != 0`，顯示目前
   的水行／效果狀態列（相鄰的 `03D8`、`03DA` 也各有自己的列）。

另在地圖／方向處理的 `seg000:15CBA` 附近，`ds:03D9` 與 `03D5`–`03D8`、
`03DA`、`03D6`、`03D7` 一起被檢查，決定是否呼叫 `sub_147D8`；這是效果存在性
與時間處理的交叉讀取，不是另一個通行規則。

### 休息／換圖的清除

既有 `docs/formats/08-combat.md` 的反組譯索引記錄：`2MISC` 的
`sub_1CD8A` 在休息／換圖路徑把 `ds:03D5`–`ds:03E1` 整段清成 0，故其中包括
`ds:03D9`。這表示水行術至少會在該正常生命週期邊界失效；不能假設跨圖後仍保留。

### Remake 存檔的現況（不是 DOS 格式證明）

目前 remake 的 `game.State.Globals` 會以 DGROUP 位址保存／還原全域旗標，因此
`0x03D9` 若在 remake 中施法後存檔，資料結構可以往返。但現有程式註記明確說
這是 remake 自己的 JSON 狀態格式，並未證明 DOS 原版是否把該旗標寫入名冊或
其他存檔區；這一點仍不可宣稱 parity。

## 本輪 remake 接入（2026-08-12）

已在 `internal/game` 接上靜態證據足夠的最小垂直鏈，並以 Docker 中的
`go test ./internal/game/...` 驗證：

* `EnterOutdoor` 以既有 `MapAttr.Scene()`（`ATTRIB.DAT +4` 低 nibble）作為
  `ds:16DA` 的目前強推論來源；只有場景 `0x0A` 且 `Globals[0x03D9] == 0`
  才擋水域。沒有把欄位命名為船或游泳能力。
* 水行術仍由 `sub_1C8C8` 對照的 engine effect 設定 `0x03D9 = 1`；休息與
  已定位的換圖／傳送路徑呼叫 `ClearTravelEffects`，清除原版同批
  `0x03D5–0x03E1` 暫時效果。
* remake JSON 的 `State.Globals` 會保存並還原 `0x03D9`；這只證明 remake
  自己的 round-trip，不改變 DOS 原版 save/load 仍未知的狀態。

這使「已證實控制流所涵蓋的 remake gate」不再阻塞，但原版正常玩家的休息、
換圖、存檔重載仍維持 `oracle unknown`，不得寫進 parity 或公開發行聲明。

## 強推論

### `ds:16DA` 是目前場景／貼圖組識別，不是船旗標

`internal/game/mapattr.go` 與 `docs/formats/02-data-files.md` 已將地圖 `ATTRIB.DAT`
的 `+4` 低 nibble 定義為場景（貼圖組）碼；`sub_1B1D4` 以它決定場景資料。
同時 `ds:16DA` 在水域 gate 被比較為 `0x0A`，而不是被設定為 0／1。
依型態與相鄰場景載入證據，最合理解釋是 `ds:16DA` 保存目前載入的場景／貼圖組
識別，`0x0A` 是原版只在某一野外場景套用「不能游泳」訊息的特殊值。

這個解釋能說明為何水域分類不直接無條件擋住，也能和現有文件所列的野外場景
碼 9／10／11／12 對上；但本輪沒有取得 `sub_1B1D4` 的完整 IDA 寫入交叉參照，
所以標為「強推論」，不可升格為已證實的 `ds:16DA` 寫入端。

### `ds:03D9` 的持續時間是效果／世界時間生命週期，而非每步遞減

`sub_13700` 只直接讀 `ds:03D9`，並遞減選定的 `ds:03E0` 或 `ds:03E1` 計數器；
它沒有在該段直接遞減 `ds:03D9`。配合 `sub_1CD8A` 將 `03D5`–`03E1` 一起清除，
較合理的模型是：水行術以旗標開啟，效果期間由世界／休息流程管理，休息或換圖
清除；不能把它實作成「走一步就減一」的計時器。

## 未知與不可從現有證據推出的事

* `ds:16DA` 的所有寫入端、確切寫入值表、與 `ATTRIB.DAT +4` 的逐圖對照尚未由
  IDA xref 完整匯出；需要在 `MM2.EXE.i64`／相應 overlay 資料庫追查 `mov [16DA]`
  或等價指標寫入，保留原始位址與 overlay/file offset。
* `ds:03D9` 在 `sub_1C8C8` 之外是否還有船、事件或載入器寫入端，現有匯出未足以定案。
  不應因其值是 1 就命名成 `hasBoat`。
* DOS 原版 save／load 是否保存 `ds:03D9`，以及跨圖時先清除還是載入後重建，尚未
  由可寫存檔的正常玩家路徑驗證。
* 手冊把水行術描述為「效用可維持一天」，但這只能作外部語意提示；目前沒有把
  「一天」與 `03E0`／`03E1` 的精確單位連起來的原版逐步實驗。
* `ds:16DA == 0x0A` 是否只代表某一場景、某一資產組，或在特定跨圖路徑另有暫時值，
  未完成 exact-state 對照。

## 交給實作者的最小狀態模型（暫定，不是授權改碼）

先保留兩個獨立欄位，避免合併成 `canSwim`：

```text
currentSceneGroup : byte   // 對應待驗證的 ds:16DA
walkOnWater       : byte   // 對應 ds:03D9；水行術寫 1
```

水域 gate 的最小暫定實作只有：

```text
if terrainClass == Water && currentSceneGroup == 0x0A && walkOnWater == 0:
    block("Can't swim!")
else:
    allow
```

`walkOnWater` 的生命週期先以「施法設 1；休息／換圖清 0」建模，但必須在完成下方
驗收矩陣後才可標為 remake 已完成；DOS 原版 save/load 行為在驗收前保持 `oracle unknown`。

## 最小驗收矩陣

| 狀態 | 場景組 | `ds:03D9` | 預期原版 gate | 證據狀態 |
|---|---:|---:|---|---|
| 水域，未施法 | `0x0A` | 0 | `Can't swim!`、不可進格 | 控制流已證實；remake gate 已測；原版重播待補 |
| 水域，水行術後 | `0x0A` | 1 | 可進格 | 控制流已證實；remake 施法→gate 已測；原版重播待補 |
| 水域，非 `0x0A` 場景組 | 非 `0x0A` | 0 | 不由這條 `Can't swim` 分支阻擋 | 控制流已證實；需逐圖確認 |
| 水行術後休息 | `0x0A` | 先 1、後 0 | 清除效果；再進水域應重現阻擋 | `sub_1CD8A` 索引已證實；remake 已測；原版 normal path 待補 |
| 水行術後跨圖 | 新圖 | 先 1、後 0 | 旗標是否清除／重建待定 | 靜態清除端已接入；原版 oracle unknown |
| 水行術後存檔→重載 | 同圖 | remake 先 1、後仍 1 | remake JSON 保留；DOS 是否保留待定 | remake 已測；DOS oracle unknown |
| 有船（若遊戲存在船路徑） | `0x0A` | 0 | 是否另寫旗標待定 | oracle unknown，不得猜 `hasBoat` |

## 下一個最小可重現動作

在指定 DOSBox／IDA 容器恢復後，使用同一個合法原版資料與可寫存檔副本：

1. 以正常玩家路徑進入 `0x0A` 場景的水域相鄰格，記錄未施法阻擋畫面。
2. 施放水行術後，不離圖直接走入同一水域格；記錄是否成功及 `03D9` 的記憶體值。
3. 休息一次、離圖再回圖、存檔重載各做一條獨立試驗；每次記錄 `16DA`、`03D9`、
   地圖／座標／年份日與畫面。
4. 以 `MM2.EXE.i64` 及 `2PLAY`／`2CAST1` 資料庫匯出 `16DA` 與 `03D9` 的全部
   寫入端，報告 IDA 線性位址、overlay/file offset、原始 bytes 與推論等級。

完成上述前，實作者可先保留水域阻擋，不得把未驗證船行為或 save/load 保留行為
寫成公開 release claim。
