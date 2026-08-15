# 後續工作

「現在做到哪」在 [`CONTEXT.md`](../CONTEXT.md) §1.5，那張表是唯一狀態來源。
這一份只排**還沒做的事**，按「會不會擋住玩家或發行」分層。每一項都寫清楚
卡在哪、下一步是什麼，避免下一輪重新盤點。

## 1. 擋住公開發行

| # | 項目 | 卡在哪 | 下一步 |
|---|---|---|---|
| R1 | git 歷史裡還留著五份由原版產生的 JSON | `data/creation.json`／`experience.json`／`pictures.json`／`terrain.json`／`traps.json` 都已 `git rm --cached` 並進 `.gitignore`，但歷史裡還在。`tools/check_release.sh` 的第五道檢查因此不過 | 公開時**另建乾淨 repo**（見 [`release.md`](release.md)），不改私有 repo 的歷史。要在原 repo 清就得 `git filter-repo` ＋ force push —— **那一步要先取得同意** |
| R2 | 公開 repo 尚未建立 | — | 建好之後在該 repo 跑 `tools/check_release.sh --public`，份數以當次執行結果為準 |
| R3 | Windows／macOS 真機驗收 | 三平台封裝的 Docker smoke 都過了，但沒有真機開過 | 真機各跑一次正常玩家路徑；macOS 另需簽章與公證 |
| R4 | 推廣片人耳驗收 | 現有 72 秒 1080p 版用的是玩家自備 `MM2.EXE` 的 DOS PC Speaker 轉譯 | 人耳過一遍；Mega Drive 配樂版要等本機 16 首可重現音樂包完成才重拍 |

## 2. 原版 oracle 未知（不擋玩家路徑）

這些是「remake 內部自洽，但沒有原版對照」的項目。**測試綠不算數**，
要對照原版才升級成已證實。

| # | 項目 | 現況 |
|---|---|---|
| O1 | 施法輸入的逐提示對照 | 只依目前 handler 的 typed consumer 接線；未證實的怪物目標不猜補。見 [`spell-interaction-oracle.md`](research/spell-interaction-oracle.md) |
| O2 | 水行術旗標的存檔持久性 | 靜態控制流已證實，DOS save/load replay 還沒做。見 [`water-traversal-oracle.md`](research/water-traversal-oracle.md) |
| O3 | 戰鬥編隊與目標命令的細節 | 九個指令已接，編隊規則未證實 |
| O4 | 門狀態 DOSBox 複驗 | 機制已結案（`sub_13A64`），只剩事後複驗 —— 是複驗不是取答案 |
| O5 | 事件 `0x2a` 的獎賞領取 | remake 自動發放，原版要按 `S`。差異理由記在 [`polish-spec.md`](polish-spec.md) P11 |
| O6 | Mega Drive 戰鬥結算那五首的角色 | 強推論，現在玩家聽得到了，值得補一次動態驗證。見 [`music.md`](music.md) |

## 3. 反組譯筆記的缺口

835 個函式裡 388 個有筆記。**覆蓋率低不等於有缺口** —— root 的多數是
C runtime，不列。真正缺筆記的是這幾個：

| # | overlay | 有筆記 | 說明 |
|---|---|---|---|
| N1 | `2SMITH` | 6/32 | 買賣鑑定已解，**結局控制室只解出獎勵那一段**（`+129` bit 3 給 50,000,000 經驗）。`WAFE` 之後的完整流程沒讀 |
| N2 | `2CMDS` | 8/37 | 交易／使用／裝備／卸下／丟棄，**功能 remake 都已實作**，缺的是把依據寫成筆記 |
| N3 | `1RETINN` | 3/12 | 旅店與隊伍編組同上；雇用名單讀 `ds:03F6` 已解（[`re/03`](re/03-character-flags.md)）|
| N4 | `2COMBAT` | 44/117 | 傷害鏈與九個指令已解；編隊、目標選擇、逃跑判定的細節還沒逐支寫 |

寫筆記的收益是「下一輪不必重開 IDA」，不是修 bug —— 這四項都沒有已知的
行為缺陷。

## 4. 素材與平台

| # | 項目 | 卡在哪 |
|---|---|---|
| A1 | Amiga `.anm` 的動畫零件 | 容器的 `count` 與影像數對不上，目前只出基準圖。見 [`research/02`](research/02-other-platforms.md) |
| A2 | MSX／Mega Drive 的場景素材 | 怪物已接進 `F6` 循環，**牆面與地板還沒**（DOS／Amiga／MSX 三套有，MD 沒有）|
| A3 | 片頭疊圖的換格週期與觸發規則 | 取樣間隔 1 秒只能定出「比 1 秒快」；remake 用固定輪替，不宣稱同步。見 [`formats/04`](formats/04-graphics.md) |

## 5. 已經沒有未解項的區塊

不要再回頭盤點這些：

- **任務線**：兩位領主、八大職業考驗、競技賽、四位主教、Lord Peabody
  與年代之門、主線收尾 —— 全部解出並整理成 [`quests.md`](quests.md)
- **`2CAVES`／`2BRAIN`／`1MENU1`** 三個 overlay 全解，見 [`re/README.md`](re/README.md)
- **角色記錄的旗標**：`+120`／`+124`／`+125`／`+129` 每一個位元、
  24 個全域夥伴旗標，見 [`re/03`](re/03-character-flags.md)
- **酒館**：五個選單項、酒與餐點的效果、四十則傳聞、`str.dat` 索引對應，
  見 [`re/04`](re/04-2brain-tavern.md) 與 [`tavern-rumors.md`](tavern-rumors.md)
- **檔案格式**：壓縮、圖形、字型、文字、地圖、事件、物品、怪物、法術
