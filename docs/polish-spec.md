# remake polish 規格

這份文件的用途是把「RE 已經解出來、但 remake 還沒照做」的事情逐條寫成
可實作、可驗收的規格。流程固定：**讀 RE 文件 → 寫這裡 → 實作**。

每一條都要有：原版位址、目前 remake 的行為、目標行為、驗收條件。
**沒有原版證據的條目不寫進來** —— 想加的玩法另開一節標明是 remake 自己的。

狀態欄：`已完成` / `實作中` / `待實作` / `擋住`（擋住的一定要寫下一步）。

---

## 1. 已完成

| # | 項目 | 原版依據 | 驗收 |
|---|---|---|---|
| P1 | 門打開改屬性層牆位元，離圖還原 | root `sub_13A64`；`2PLAY sub_1BE24` 換圖重載 | `internal/game/door_test.go` 三條 |
| P2 | 事件 `Kind` 是方向遮罩，面向不對不觸發 | `2PLAY sub_1A8C4` `0x1A939`；遮罩表 `ds:16D2` | `world_test.go` 的招牌兩條 ＋ DOSBox 四步時間線 |
| P3 | 動畫表第一段是播放腳本，bit 7 ＝隨機挑段 | root `0x15715`／`sub_15772`；`sub_11C88` 是亂數 | `monsters_test.go` 的 240 段／136 腳本項 |
| P4 | `Protection` 三行接上真來源 ＋ 飄浮術第四行 | root `sub_147D8` `0x1486B` 起逐列 | `view/status_test.go` 兩條 |
| P5 | 突襲狀態開戰時擲，盜行高易先手、守衛術擋突襲 | `2COMBAT _2combat_e03` `0x1A4E7`；`ds:549E` ＝ root `sub_13A9E` | `roster_combat_test.go` 的 `TestRollAmbush` |
| P11 | `S` 搜尋指令；戰利品不再自動彈四選單 | `2PLAY` `0x181E8` → thunk `0x17336` → root `0x13814` | `session_test.go` 的 `TestCombatVictoryNeedsSearch` ＋ `TestChestPage` |
| P12 | `B` 撞門、`Q` 查說明書照原版；商店移到 `G` | [`command-keys-oracle`](research/command-keys-oracle.md) §2 | `go test ./...`；README 與 `cmd/mm2/main.go` 一致 |
| P15 | `2CAVES` 全部特殊裝置：座標傳送機、滑梯陷阱、隨機遭遇、三個架子、捐黃金／寶石換經驗、三倍泉、生命上限重算、馬戲團、年代之門、每日笑話、兩位領主的任務 | [`docs/re/02`](re/02-2caves-special-events.md) | `internal/game/cave_test.go` 十四條 ＋ `internal/game/quest_test.go` 五條 ＋ `internal/ui/cave_internal_test.go` 四條 |
| P14 | 片頭畫面：`MASTER.16` 的底圖加 13 張疊圖動畫，接 `intro` 音樂 | 60 張 DOSBox 截圖逐像素定落點；[`04-graphics`](formats/04-graphics.md) §標題畫面的疊圖動畫 | `internal/ui/intro_test.go` 兩條 ＋ `docs/screenshots/17-intro.png` |
| P13 | 世界地圖頁（`W`）：5×4 網格由 `ATTRIB` 鄰接現算 | [`world-grid-oracle`](research/world-grid-oracle.md) | `worldgrid_test.go` 三條 ＋ `TestWorldPage` ＋ `docs/screenshots/14-world-grid.png` |
| P17 | 全滅回到最後投宿的旅店；五座城的落點表 | `1RETINN _1retinn_e04`／`_1retinn_e03`；`ds:21E8`／`21EE`／`21F4`（[`docs/re/06`](re/06-1retinn-roster.md)）| `control_internal_test.go` 的 `TestDeadReturnsToLastInn` ＋ `TestLastInnSurvivesSave` |
| P16 | 結局控制室（`0e fd`）：守門的 Sheltem 戰、`WAFE` 中止碼、替代加密的密碼題、15 分鐘倒數、通關結算 | [`docs/re/05`](re/05-2smith-control-room.md) | `internal/game/control_test.go` 十條 ＋ `internal/ui/control_internal_test.go` 五條 ＋ `workplace/gfx/ui/control-*.png` 六張 |
| P18 | 怪物的遠程／法術攻擊三十二種：吐息、群體骰、上狀況、抽資源、自爆、拋擲隊伍 ＋ 抗性三通道 | [`docs/re/09`](re/09-2combat-map.md) §4；root `sub_138A8`／`sub_13928` | `internal/game/special_test.go` 十條 |
| P19 | 固定的功能鍵：`F1` 說明、`F2` 設定、`F4` 存檔、`F10` 離開（先自動存檔）、`Esc` 一律取消；`F8` 快速戰鬥 | remake 自己的介面決定（原版的存檔與離開在 `O` 指令視窗裡） | `internal/ui/settings_test.go` 四條 |
| P27 | **透空改走原版的 1-bit 遮罩**：影像集每張都帶一個遮罩指標，正牆與縱列牆是 0（整塊不透空）、側牆與它們的門各帶一份逐像素遮罩。`render.BlitStencil`／`BlitHiStencil` 走它，`BlitKey`／`BlitMask` 留給沒有遮罩的平台 | `EGA.DRV` 跳表第 19 項 `0xCCA`（`MM2.EXE` 的 `sub_11CDA(0x13, …)`，見 [`04-graphics`](formats/04-graphics.md) §1）| `cmd/mm2diff -oracle` 十四個座標裡十三個 0.0%（動工前 (7,4) 面西還差 3 個像素）＋ `internal/assets/gfx/mask_test.go` 三條 |
| P26 | **MSX 的戶外第一人稱**：一整排格子（每個深度列舉 ±5／±2／±1／±1 個橫向偏移），每格由碼挑三組擋路物之一 | `sub_2B0A` 列舉、`sub_2A57` 取格子位元組最高 3 bit 查每張地圖的表（`+0x201`）分派到 `sub_1103`／`sub_1A40`／`sub_1C2B`；落點由 `tools/msxout.py` 符號執行產生（[`research/02`](research/02-other-platforms.md)）| 截圖 `01r-outdoor-msx.png` ＋ `internal/assets/msx/outdoor_test.go` 三條（產生出來的每一塊都要落在視圖裡、來源要落在素材表裡，地形帶的三個變體位移也要）|
| P25 | **場景切換與野外視圖擴到全部平台**：Amiga、Mega Drive、高解析素材包都依場景碼換素材，野外也各走自己那條 | Amiga 的檔名與 DOS 同名只差大小寫（`cave.32`／`castle.32`／`outdoor1-3.32`／`outf.32` ＋ 四個地形檔），張數逐項相同；Mega Drive 的 `sub_FC38` 七格跳表把類型收斂成三套（0／1、2／5、3／4／6），`workplace/md-scene/scene.json` 的 `types` 就是它 | 截圖 `01m-cave-amiga`／`01n-outdoor-amiga`／`01o-castle-md`／`01p-outdoor-md`／`01q-outdoor-pack`；`go test ./...` |
| P24 | **室內的第一人稱補到逐像素相同**：兩側縱列的正牆（`sub_1BEBA` 的條件）、落點改查表、正牆整塊不透空、門那一組拿素牆當透空遮罩、縱列牆的火炬跟著牆種類走 | DGROUP 的六張表 `0x153E`–`0x1595`（落點 word、影格 byte）；`sub_18558`／`sub_185B4`／`sub_1867C` 與兩支的 bit7 分支（[`04-graphics`](formats/04-graphics.md)）| `cmd/mm2diff` 在米德格特十格、地圖 11 兩格共 **12 個座標都 0.0%**（動工前 (8,0) 面東 9.9%）＋ `internal/view/firstperson_geom_test.go` 四條（其中一條拿 `MM2.EXE` 逐格對表）|
| P23 | **野外的第一人稱**：三組擋路物 ＋ 地形帶 ＋ `OUTF` 地板，每一格畫什麼由地形值查表決定 | `_2play_e10` 野外分支只載 `outf` 與 `outdoor1-3`；`ds:52B2` 是地形值 → 碼；`sub_18AD0`／`18B0C`／`18BEC` 三支的影格與落點表；地形帶走 `sub_189B8`（[`04-graphics`](formats/04-graphics.md)）| `cmd/mm2diff -map 11` 逐像素比對：`(7,3)N`／`(7,3)E`／`(7,6)N` 三處**都 0.0%**（含「與前一格連續」分支；補之前是 4.2%）＋ `internal/view/outdoor_test.go` 兩條（拿真的素材尺寸驗每一張都落在視圖裡）＋ 截圖 `01l-first-person-outdoor.png` |
| P22 | **戰鬥中施單體法術要問打哪一隻**：範圍是場上全部（夾在 10），不是近戰的前排；只剩一隻就不問，Esc 取消不扣 SP 與寶石 | 2026-08-17 實機：同一場戰鬥裡近戰問 `Fight which (A - E)?`、法術問 `On which (A-J)?`；怪物名單面板兩種情形都列 `A)`–`J)`，只在打得到的那幾隻前面打勾（[`spell-interaction-oracle`](research/spell-interaction-oracle.md)）| `internal/game/spelltarget_test.go` 四條（含**讀 `cast.go` 語法樹**比對「打幾隻」那張表有沒有漂）＋ `internal/ui/spelltarget_test.go` 四條 ＋ 截圖 `07c-spell-target.png` |
| P21 | **場景素材依場景碼切換**：地城用 `cave*.16`、城堡用 `castle*.16`；MSX 也是一種場景一張表（`0x2020`／`0x2021`／`0x2022`／`0x2023`）| `2PLAY _2play_e10` 的 7 個 case：0 → `town*`、1 → `cave*`、2 與 5 → `castle*`、3／4／6 → `out*`；MSX 是 `f002 sub_31DA` 的四段地圖區間，與 DOS 的場景碼逐段相同 | 截圖 `01i-first-person-cave.png`（灰綠苔石）、`01j-first-person-castle.png`（灰石磚＋紅旗）、`01k-msx-cave.png`（MSX 換成第二張表）；`go test ./...` |
| P20 | 攻擊前挑目標：近戰列前排、射擊列全場（都夾在 10），只剩一隻就不問，Esc 取消且不消耗回合 | `2COMBAT sub_18DAA` 開頭：`ds:54A4` 決定用 `ds:9FC5` 還是 `ds:0508`、`var_C <= 1` 直接打第 0 隻、提示 `Fight`／`Shoot` ＋ `exe.0C34`、`0x1B` → `var_E--` | `internal/ui/target_test.go` 七條（含集火真的打到選中那一隻）＋ 截圖 `07b-target.png` |

---

## 2. 待實作

### ~~P6 場景碼 `ds:039C` 要由地圖編號算出來~~（已完成）

**原版依據。** `2PLAY sub_1B410(地圖編號)` 在三張各 7 筆的表上做區間查找，
`_2play_e11`（`0x1B6C7`）把結果寫進 `ds:039C`：

```asm
cl = 7                              ; 預設「不在世界裡」
for si in 0..6:
    if ds:16E8[si] <= 地圖 <= ds:16F0[si]: cl = ds:16E0[si]
    if cl != 7: break
return cl
```

三張表的實際值（`ds:16E0` 場景碼、`ds:16E8` 下界、`ds:16F0` 上界）：

| 場景碼 | 地圖區間 | 張數 |
|---:|---|---:|
| 0 | 0–4 | 5 |
| 3 | 5–16 | 12 |
| 1 | 17–32 | 16 |
| 6 | 33–40 | 8 |
| 4 | 41–44 | 4 |
| 5 | 45–54 | 10 |
| 2 | 55–59 | 5 |

**60 張不重不漏**，而場景 0 正好是五座城鎮 —— 與「地圖 0–4 是城」互相印證。
`1RETINN _1retinn_e01` 另外把 `ds:039C` 設成 7（旅店名冊畫面，不在世界裡）。

**目前 remake。** `World.Scene` 是欄位，預設 0，沒有人算它；
`gamedata.Pictures.Scene` 已經有 7 張表，opcode `0x0b` 拿 `w.Scene` 去查。
所以現在**所有地圖都當場景 0**，地城與野外的事件圖號會查到城鎮那張表。

**做法。** 沒有「換圖時記得更新」這回事 —— `World.Scene` 由欄位改成
**從 `MapIndex` 導出的方法**，那條漏就結構上不存在了。原版是每次換圖重算再
寫進 `ds:039C`，行為相同。

**驗收（`internal/game/scene_test.go`）。** 七個區間不重不漏蓋滿 60 張、
逐張比對場景碼、五座城鎮都是場景 0、出界回 `SceneOutside`（7）。

### ~~P9 天空影格由天花板位元圖決定~~（已完成）

**原版依據。** `2PLAY _2play_e02`（`0x18517`）查 `ATTRIB.DAT` `+32`…`+63`
那張 16×16 位元圖（每列 2 bytes、遮罩 `1 << (x & 7)`），回傳 0／1，
`_2play_e03`（`0x18773`）直接拿它當 `SKY.16` 的影格編號。

所以**影格 1 是天花板不是天空**。資料側：24 張全 0 的圖精確等於獨立已知的
「二十四張野外圖」，26 張全 1 的是地城與城堡，10 張混合的是五座城鎮與
另外五張。這同時把 `+32`…+63`「位元圖，用途未定」解掉了。

**先前為什麼對不起來**：拿「正牆在第幾格」去對 —— 它跟牆無關，只看腳下那一格。

**目前 remake。** `drawSky` 寫死影格 0。

**做法。** `MapAttr.Ceiling(x,y)`，`UseAttrs` 時攤進 `Map.Ceiling`，
`drawSky` 依隊伍所在格挑影格。

**驗收（`internal/game/ceiling_test.go`）。** 全 0 的那批必須精確等於
`Indoor()` 為假的那批（一張不差）、五座城鎮必須是混合、要有整張全 1 的圖。

### ~~P10 `ds:03C8` 不必實作~~（已完成：確認它沒有讀取端）

`ds:03C8` 全十五個資料庫**只有一處寫**（`2PLAY sub_1A202`，事件 opcode
`0x2c` 的累加）、**零處讀**。與 `ds:9E34`（怪物 `+0x16` bit7）、屬性層
bit 5 同一類：原版寫了但不用。remake 原樣往返即可，不必替它發明單位。

（限制：掃描比對的是運算元的位移值，範圍 `03C8`–`03CB`。若有程式以
更低的基底加索引讀它，這個掃法看不到 —— 但同一塊的 `ds:03CA` 讀取端
到處都是且全是直接定址，沒有索引式存取的跡象。）

### ~~P8 定位術要打開地圖畫面~~（已完成）

**原版依據。** `2CAST1 sub_1C1D2`（法術編號 5、引擎編號 53）在共用確認提示
過關之後呼叫 `_2play_e00` 與 `_2play_e14`（thunk `0x172B2`）—— 它用 `.16` 的
`B` 圖磚以 16 px 步長把整張俯視圖畫出來。`0x172B2` 這個 thunk 只有 2CAST1
用，但 `_2play_e14` 本身另有一個近呼叫端：2PLAY 指令分派 `0x18189` 的 `M` 鍵
（見 [`command-keys-oracle`](research/command-keys-oracle.md)）。
**定位術與 `M` 打開的是同一張圖。**

珍017 中文說明書逐字相符：定位術「給予隊伍所在之精確位置，並**顯示目前
16×16 區域之地圖，標示你所在位置及方向**」—— 那組圖磚裡的
↑ ↓ → ← 四個箭頭就是「標示方向」。

**這同時把 04-graphics.md 的 B3 解掉了**：俯視地圖的觸發指令就是定位術。

**目前 remake。** 有完整的地圖畫面（`M` 鍵），但定位術只回一句話。

**做法。** `CastResult.ShowMap` 由定位術設起來，UI 收到就切 `ModeMap`；
同時把整張圖標成看過（`Explored.MarkMap`）—— 原版是把 16×16 直接畫出來，
而 remake 的地圖畫面是持久的，「畫出整張」對應成「整張標成看過」。

**驗收（`internal/game/locate_test.go`）。** 施法後 `ShowMap` 為真、
當前圖的四角與中間都標成看過、別張圖不受影響。

### ~~P7 戰利品不給 `0xF0` 的物品附魔~~（已完成）

**原版依據。** `2COMBAT sub_19A3C` `0x19ADF`：`+0x0E == 0xF0` 時跳過隨機
附魔與充能那一段，物品編號照樣寫入。58 件是 `0xF0`（鑰匙、票券、藥水等
非裝備品）。

**目前 remake（寫這條規格時的認知是錯的）。** 以為 remake 沒有這一條，
實際上它有 —— 但做成 `continue`，**整件跳過**，於是那 58 件永遠不會掉出來。

**原版的順序才是關鍵**：`0x19AC2` 先把編號寫進 `ds:6950`、`0x19AC9` 依
`+0x0F` 取充能、`0x19ADF` 才檢查 `0xF0`；命中就跳到函式尾端，而尾端照樣把
充能寫回去。所以是「照樣掉、充能照給、只有附魔那一擲沒跑到」。

**驗收（`TestVictoryLootKeepsNonEquipItems`）。** 把整張物品表設成 `0xF0`
跑 200 場戰鬥：一定要掉得出東西，而且沒有任何一件帶附魔或被標成魔法物品。

### ~~P11 `S` 搜尋指令：戰利品要玩家自己撿~~（已完成）

**原版依據。** `2PLAY` 主迴圈 `0x18239` 比到 `'S'` 就跳 `0x181E8`，經 thunk
`0x17336` 打到 root `0x13814`。那支印 `Search...`，掃三個物品槽（`ds:6950`）、
金幣（`ds:695C/E`）、寶石（`ds:695A`），五組全空就印 `Nothing Here!`，
否則進 `2MISC _2misc_e02` —— 而 `_2misc_e02` 的 thunk **全域只有這一個呼叫端**。
所以戰利品、事件獎賞、神殿物品**全部只能經由搜尋領取**。完整按鍵表與
`ds:0434` 三狀態見 [`command-keys-oracle`](research/command-keys-oracle.md)。
珍017 說明書上冊 p.39 相符：「戰鬥後不要忘了用 `S` 找尋戰利品」。

**目前 remake。** 沒有搜尋指令。戰鬥一結束 `fightRound` 就把 `Chest` 塞進 UI
並自動彈四選單；四選單另外掛在自創的 `G` 鍵上，而 `S` 被存檔佔著。

**做法。**

- `KeyChest` 改名 `KeySearch`，綁 `S`；存檔移到 `F2`（原版的存檔在 `O` 指令
  視窗裡，本來就不是字母鍵），拿掉自創的 `G`。
- 沒有待領的東西時回「這裡什麼都沒有。」，對應 `Nothing Here!`。
- 戰鬥勝利改成把箱子擺著不自動開，訊息提示按 `S` 取。
- 事件 `0x2a` 的待領獎賞**預設**走 `ClaimReward` 自動發放 —— 原版那條也要按
  `S`，但 remake 的事件流程沒有「按鍵前先把訊息掛著」的中斷點，自動發放
  比較順。**想照原版的在 `F2` 設定裡切**（`Session.AutoClaimReward`）：
  切成「按 S 領取」之後獎賞留在 `World.Reward`，按 `S` 搜尋才入袋。
  這是預設值的差異，不是做不到。

**驗收（`internal/ui/session_test.go`）。** 戰鬥勝利後不進選單、`Chest` 仍在；
按 `S` 才進四選單；沒有箱子時按 `S` 得到「這裡什麼都沒有。」且不進選單。

### ~~P12 鍵位別跟原版打架~~（已完成）

**原版依據。** [`command-keys-oracle`](research/command-keys-oracle.md) §2 的
十三個按鍵。遊戲內按 `K` 查得到的說明書轉錄（`data/reference.json` 的
`fieldCommands`）列的就是這一組原文指令名，玩家照著按會按到別的東西。

**目前 remake 的衝突。** `B` 是商店（原版是撞門）、`D` 是撞門（原版是解僱）。
其餘差異都是原版沒有的功能佔了原版沒用的字母，不算打架。

**做法。** `B` 還給撞門，商店移到原版沒用的 `G`；`Q` 加成「查說明書」的別名
（原版 `Q` 是 Quick Ref，remake 的 `K` 是它的超集）；`D` 空著不亂給
—— remake 沒有解僱功能，把它綁到別的指令只會製造第二次衝突。

**驗收。** `go test ./...`；README 的按鍵表與 `cmd/mm2/main.go` 一致。

### ~~P13 世界地圖：說明書最後一項沒收進遊戲的東西~~（已完成）

**原版依據。** 二十張野外圖由 `ATTRIB +5`…`+8` 連成 5×4 的環面，
`C2` ＝ 地圖 11（`EVENTSO` 第 11 段 `(7,3)` 的 `0c 00 f5` 換到米德格特，
與說明書「區域 C2，X=7 Y=3」相符）。推導與交叉驗證見
[`world-grid-oracle`](research/world-grid-oracle.md)。

**目前 remake。** `data/reference.json` 的 `worldMap` 有 51 條「區域碼 → 地名」，
按 `K` 查得到；但**那張圖本身沒有**，玩家看得到 `C2 方形湖` 卻不知道 C2 在哪。
掃描頁不能散布，所以只能自己畫 —— 而畫它需要的資料玩家本來就有。

**做法。**

- `internal/game/worldgrid.go`：由 `MapAttr` 的鄰接欄位現算 5×4 網格，
  **整份只寫死一個常數**「A1 ＝ 地圖 5」。提供 `WorldGrid()`、
  `RegionOf(地圖)`、`WorldTileset(地圖)`。
- 新的 `W` 鍵與 `ModeWorld`（原版沒用到 `W`）：5×4 的區域格，每格標區域碼、
  依貼圖組上色，沒踏進去過的壓網點；隊伍所在的那一格加亮框，右欄印區域碼、
  該區域的地名（取自 `reference.json`）與四色圖例。
- 城鎮／地城四面自指，不在網格上，右欄改印地名加「不在世界網格上」。

**驗收（`internal/game/worldgrid_test.go`）。** 二十張不重不漏、每一格的
東鄰與南鄰都與 `ATTRIB` 相符、`C2` 是地圖 11、A1／B1 是凍原、D2／E2 是沙漠、
D4 是沼澤、四個元素領域與五座城鎮不在網格上；UI 端 `TestWorldPage`
按 `W` 進得去畫得出來，畫面證據 `docs/screenshots/14-world-grid.png`。

### ~~P16 結局控制室~~（已完成）

**原版依據。** `2PLAY sub_19716` 的 `0xFD` → 2SMITH `_2smith_e01`（`+CEC8`）
→ `sub_1D2A4`。五隻守門的怪、`WAFE` 中止碼、`+129` bit 5、替代加密的密碼題、
900,000 的倒數、五千萬經驗與最終分數，逐段的位址見
[`docs/re/05`](re/05-2smith-control-room.md)。

**目前 remake。** 已接完整條鏈：`0e fd` → 守門那一場（`ds:9680` 的五個編號
＝ Sheltem 與四隻元素生物，走與競技賽同一套「開打／打完之後」拆法）→
控制室三段畫面 → 通關結算。勝敗場數（`ds:0410`／`ds:0412`）補進 `Session`
與存檔，結局那一行才有數字可填。

**刻意的差異（兩項，都要在對外說明時講清楚）。**

1. **密碼題中英並陳，答案直接附上。** 原版只印密文，玩家要認出那是美國
   憲法序言的改寫、推出字母對應、再把 `Preamble` 編碼打進去。翻成中文之後
   那條線索斷了 —— 中文沒有字母可以代換 —— 與 `KEYS`／`DRUIDS`／`MEENU`
   同一類（見 [`07-event-script`](formats/07-event-script.md) §謎題）。remake
   把密文、英文原文、中文譯文三份一起攤開，密文與原文對照就看得出字母怎麼換；
   輸入欄旁邊直接附上編碼後那八個字元，照打即可。**比對邏輯完全不動**：
   收到的字串仍與 `Encode("Preamble")` 逐字元比，長度仍是 8。
2. **倒數掛在動畫節拍上。** 原版每輪輪詢扣 75，中間夾一個延遲呼叫，
   而那支常式的單位還沒證實（`docs/re/05` §6）。remake 改掛火炬動畫的
   `Tick`，畫面上顯示的仍是原版那個從 `15:00:00` 起算的鐘。

**版面。** 探索畫面下方的訊息框只有三列，塞不下這幾頁，所以控制室是整頁
（`ModeControl` ＋ `view.DrawControlRoom`），與世界地圖、片頭同一類的
remake 版面決定，不是 pixel-perfect 還原。內容超出可用列數時**留一行標記**，
不安靜截掉。

**驗收。** `internal/game/control_test.go` 十條（`0e fd` 真的在 `EVENTSI` 裡、
字母表是排列且大小寫分開、每次重抽、中止碼七種輸入、只認編碼後的答案、
鐘走完進逾時、獎勵只發一次、勝敗場數往返、密文取自 `STR.DAT` 原文、
守門五隻的編號）；`internal/ui/control_internal_test.go` 五條（完整玩家路徑、
中止碼打錯、逾時、兩個欄位的長度、六張畫面）。畫面證據
`workplace/gfx/ui/control-{guard,abort,brief,cipher,win,score}.png`。

### ~~P17 全滅不是死路~~（已完成）

**原版依據。** `_1retinn_e04`（`0x1CB40`）印十行 `Death Strikes!`，按 Enter 之後
`ds:0392 = ds:03D4` 再跳 `_1retinn_e01` —— 回到最後投宿的旅店重新編組。
`ds:03D4` 由登記入住時寫（`_1retinn_e00`），落點查 `ds:21E8`／`21EE`／`21F4`
三張六格的表。逐段見 [`docs/re/06`](re/06-1retinn-roster.md) §2、§3、§6。

**目前 remake。** 全滅之後 `Key` 對 `ModeDead` 一律 `return false` ——
**所有按鍵都被吃掉，玩家卡死**。這個洞是讀 `1RETINN` 的筆記時才發現的，
不是既有清單上的項目。

**做法。**

- `internal/game/inn.go`：`TownStart` 五座城的落點（X、Y、朝向）照那三張表；
  `Session.LastInn`（原版 `ds:03D4`）進存檔；`CheckInAtInn` 在踩進旅店時
  記下城號並把整隊記錄 `+11` 寫成城號 ＋ 1；`ReviveAtInn` 送回去。
- `StartMiddlegate` 改成引用 `TownStart[0]` —— 同一個 (7, 3) 面北原本
  只有 DOSBox 的動態證據，現在靜態表也對得上，不必再寫死第二份。
- UI：`ModeDead` 改畫整頁的 `Death Strikes!`（十行譯文已在 `exe.*` 裡），
  按 Enter 回旅店。`view.DrawControlRoom` 一併改名成 `DrawTextPage` ——
  它本來就是通用的整頁文字，控制室與全滅共用。

**刻意的差異。** 倒下的人**狀況不清除**。原版全滅之後隊伍散回名冊、
要去神殿救；remake 沒有名冊分頁，所以只把隊伍送回旅店，
治療仍然要走神殿那條路 —— 這條路不代替神殿。

**驗收。** `TestDeadReturnsToLastInn`（在 Sansobar 登記、走到別的圖、
全滅、按 Enter 要回到 (3, 10) 面西）與 `TestLastInnSurvivesSave`
（含整隊 `+11` 的寫入與存檔往返）。

---

## 3. 擋住

（空）

---

## 4. 刻意與原版不同的地方

原版做得到、remake 故意不照做的，列在這裡連理由一起寫。不在這張表上的
差異都算缺陷。

### D1 上狀況的抗性讀對的欄位

**原版行為。** 抗性表 `ds:1436` 存的是角色記錄的欄位偏移。`sub_1B70C`
把它減 `0x11` 存進 `ds:154AD`；傷害那條路的 root `sub_13928` 取偏移時
再加回 `0x11`，讀到的是對的抗性欄位。**上狀況那條路的 `sub_1B2DE`
沒有加回來**，於是催眠與凝視拿名字的第五個字元當抗性百分比、麻痺拿
寄放城鎮的編號，表值為 0 的六種（死亡之指、舞動之劍、內爆、劇痛、沈默、
蜂擁）更是讀到記錄外面。

**remake 行為。** 兩條路都讀 `記錄[表值]`，也就是 `sub_13928` 算出來的
那一格。

**理由。** 讀出界的那六種在原版是讀到相鄰記錄的內容，**本來就不可重現**；
而「名字第五個字元決定催眠抗性」照抄之後，玩家改個名字就會改變抗性，
在中文化版更是名字換了編碼整條規則跟著變。原版的意圖看表值就很清楚。

### D3 施法直接按 `C`，不必先開角色卡

**原版行為。** 移動畫面的 `C` 是設定視窗（`2MISC2 +C3F6`：音效、腳步聲、
`Disposition`、`Delay`）。施法在角色卡上 —— 按人物編號開卡，卡片下方才有
`'C' Cast`，接著是 `Cast Spell Level:` → `Number:` 兩段數字，旁邊顯示
9×7 的 Spell Book 打勾表。（2026-08-17 實機重播，
[`spell-interaction-oracle`](research/spell-interaction-oracle.md)。）

**remake 行為。** 探索畫面直接按 `C` 開施法選單（先選施法者再選法術，
法術名與說明都是譯文）；設定收進 `F2`。

**理由。** 兩段數字選法術要求玩家記得「牧師第 1 級第 4 條是急救術」——
那是紙本說明書時代的介面，而 remake 已經把法術名與說明譯出來了，
列出來選比背編號合理。**提示的語意沒有變**：需要目標的仍然先問目標、
不需要輸入的仍然是確認一次，而且**目標提示取消不扣 SP／寶石**（與原版相同，
同一輪量到的）。需要數字的那幾條原版會扣，見 D4。

### D2 蒸發財物一律清金錢

**原版行為。** `sub_1B5C8` 逐一看 `ds:0416[i]`：名冊索引 24 以上的雇傭兵
清寶石（`+92`），其餘清金錢（`+102`）。

**remake 行為。** 一律清金錢。

**理由。** remake 沒有記錄「隊伍這個位置是不是雇傭兵」——
`ds:0416` 那一排在 remake 裡沒有對應物。等雇傭兵接進來再補這一條。

### D4 問落點的法術，取消不扣費

**原版行為。** 傳送術與城市傳送術這一類要玩家給數字的法術，數字提示排在
`'Return' to cast` **之後** —— 也就是先付款再問落點。那時候按 `Esc` 只是
放棄落點，SP 與寶石照扣、隊伍不動，一次白施。實機量到：城市傳送術
200 SP／99 寶石 → `Town (1-5)?` 按 `Esc` → 192／91；傳送術 200 → `Squares: (1-9)?`
按 `Esc` → 195。（[`spell-interaction-oracle`](research/spell-interaction-oracle.md)
「扣費的時點分兩種」。）

**remake 行為。** 數字提示排在扣費之前，`Esc` 回到法術選單，一分不扣。

**理由。** 原版那個順序是「先確認再問參數」的介面必然結果，不是設計意圖 ——
玩家看到 `Town (1-5)?` 才想起自己記錯城市編號，代價是整發法術。remake 的
選單直接列城市名，取消是玩家改變主意而不是誤按，沒有理由罰他。目標提示
（`On whom (1-6)?`／`On which (A-J)?`）原版取消本來就不扣，remake 照做。
