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

835 個函式裡多數有筆記。**覆蓋率低不等於有缺口** —— root 的多數是
C runtime，不列。真正缺筆記的是這幾個：

| # | overlay | 有筆記 | 說明 |
|---|---|---|---|
| ~~N1~~ | `2SMITH` | **31/31** | **全解**：控制室見 [`re/05`](re/05-2smith-control-room.md)（已接進 remake，[`polish-spec`](polish-spec.md) P16），鐵匠見 [`re/07`](re/07-2smith-shop.md) |
| ~~N2~~ | `2CMDS` | **37/37** | **全解**，見 [`re/08`](re/08-2cmds-inventory.md) |
| ~~N3~~ | `1RETINN` | **12/12** | **全解**，見 [`re/06`](re/06-1retinn-roster.md)。順帶修掉一個會卡住玩家的洞：全滅之後所有按鍵都被吃掉，原版是回到最後投宿的旅店 |
| ~~N4~~ | `2COMBAT` | **116/116** | **全解**，見 [`re/09`](re/09-2combat-map.md)。順帶解出八個狀況位元的名字，以及 remake 還沒有的整套怪物遠程／法術攻擊（三十二種）|

寫筆記的收益本來只是「下一輪不必重開 IDA」，但**三次例外都在讀完之後才出現**：
控制室（N1）、全滅回旅店（N3）與怪物的遠程／法術攻擊（N4）都是讀完才知道
remake 少了整段機制。所以這幾項不能當成純文件工作跳過。

| # | 項目 | 卡在哪 | 下一步 |
|---|---|---|---|
| N1b | `ds:58B8` 印在載入之前 | 靜態面四項證據都指向「第一次進控制室時守門旁白印的是 `ds:0000`」，缺實機 | DOSBox 走到 `0e fd` 的格子，比對第一次與第二次進入時第 19–22 列 |
| N4b | remake 沒有怪物的遠程／法術攻擊 | 三十二種效果、傷害分配、抗性三通道都已解（[`re/09`](re/09-2combat-map.md) §4），但 `internal/game` 只實作了近身攻擊 | 先接三十二種裡沒有未解項的那些（吐息、群體骰、上狀況、自爆）；`ds:1436` 減 `0x11` 那一步在實機驗證前不照抄 |

## 4. 素材與平台

| # | 項目 | 卡在哪 |
|---|---|---|
| ~~A1~~ | Amiga `.anm` 的動畫零件 | **解完**：72 個檔 530 個影格（72 基準圖 ＋ 458 零件）與 `0x31` 的動畫表都解出來並接進 remake。見 [`research/02`](research/02-other-platforms.md) |
| ~~A2~~ | Mega Drive 的場景素材 | **解完並接進 `F6`**，與執行時的組合緩衝區**逐像素相同**（24,960 個全中）。調色盤依區域類型挑（類型 2／5 用第 0 條）、光照公式 `分量 × 亮度 ÷ 8` 也量出來了。見 [`research/02`](research/02-other-platforms.md) |
| A4 | Mega Drive 還沒歸類的 LZSS 區塊 | `tools/mdassets.py` 掃出 150 個只通過結構條件的區塊，其中**只有 `0x0345D0` 一個在 ROM 裡有人指到**（1210 bytes，內容像是一張遞增的 16 位元索引表），其餘多半是假陽性。要往下追就從那一個開始 |
| A6 | Mega Drive 的 90 個英文文字區塊還沒接進對照 | 已經解出並存成 `workplace/gfx/md-all/text/*.txt`（合計 81 KB）。字串之間沒有分隔符，取用靠索引表 —— 索引表在哪還沒找。找到之後可以與 DOS 的 `STR.DAT` 逐句對照，當譯文的第二份佐證 |
| A5 | Mega Drive 的光照沒接進 remake | 公式與級數都有了（`research/02`「光照」），remake 目前一律用未調暗的原值。要接的話是在 `TownSet` 上加一個亮度係數 —— 但 DOS 版自己的暗處規則還沒查，兩邊要一起定 |
| ~~A3~~ | 片頭疊圖的換格週期與觸發規則 | **解完**：`ds:18D8` 是 47 步的固定清單、`ds:1936`／`ds:1954` 是落點，一步 700 個 tick（PIT 除數 `0x0400` ⇒ 0.601 秒）。remake 照它播。見 [`formats/04`](formats/04-graphics.md) |

## 5. 已經沒有未解項的區塊

不要再回頭盤點這些：

- **任務線**：兩位領主、八大職業考驗、競技賽、四位主教、Lord Peabody
  與年代之門、主線收尾 —— 全部解出並整理成 [`quests.md`](quests.md)
- **`2CAVES`／`2BRAIN`／`1MENU1`** 三個 overlay 全解，見 [`re/README.md`](re/README.md)
- **結局控制室**：守門的 Sheltem 戰、`WAFE` 中止碼、替代加密的密碼題、
  15 分鐘計時、通關結算與最終分數 —— 解完也接進 remake 了，
  見 [`re/05`](re/05-2smith-control-room.md) 與 [`polish-spec`](polish-spec.md) P16
- **角色記錄的旗標**：`+120`／`+124`／`+125`／`+129` 每一個位元、
  24 個全域夥伴旗標，見 [`re/03`](re/03-character-flags.md)
- **酒館**：五個選單項、酒與餐點的效果、四十則傳聞、`str.dat` 索引對應，
  見 [`re/04`](re/04-2brain-tavern.md) 與 [`tavern-rumors.md`](tavern-rumors.md)
- **檔案格式**：壓縮、圖形、字型、文字、地圖、事件、物品、怪物、法術
