# 反組譯函式索引

由 `tools/gen_func_index.py` 掃 `docs/**/*.md`、`internal/**/*.go` 與
`cmd/**/*.go` 產生，**不要手改** —— 手工維護的名單會與文件漂移，
而漂移正是這份索引要防的事。追一支函式之前先查這裡，
查得到就先讀既有筆記，不要重開反組譯。

重新產生：

```
docker run --rm --network none -u "$(id -u):$(id -g)" \
  -v "$(pwd):/src" -w /src mm2-go:latest python3 tools/gen_func_index.py
```

共 421 個符號：409 個在 `docs/` 有筆記，12 個只出現在程式碼註解裡。

位址是 IDA 的線性位址。16-bit overlay 的換算是
`IDA linear = 檔案偏移 + 0xF800`，見 `docs/formats/01`；
六碼的是 Amiga／Mega Drive 的 68000 映像。

## 已有筆記的函式

| 函式 | 摘要 | 出處 |
|---|---|---|
| `sub_706` | VDP 暫存器在 `sub_706` 設定（`move.w #$8xxx,(a0)` 那一串）： | `docs/research/02-other-platforms.md:775`, `docs/research/02-other-platforms.md:788` |
| `sub_C48` | 門（`sub_C48`／`sub_D10`／`sub_DA7`）走 DOS 的槽位 +16，正面深度 0 | `docs/research/02-other-platforms.md:691`, `docs/research/02-other-platforms.md:716` |
| `sub_D10` | 門（`sub_C48`／`sub_D10`／`sub_DA7`）走 DOS 的槽位 +16，正面深度 0 | `docs/research/02-other-platforms.md:691`, `docs/research/02-other-platforms.md:716`, `docs/research/02-other-platforms.md:739` 等 4 處 |
| `sub_DA7` | 門（`sub_C48`／`sub_D10`／`sub_DA7`）走 DOS 的槽位 +16，正面深度 0 | `docs/research/02-other-platforms.md:691`, `docs/research/02-other-platforms.md:716` |
| `sub_1103` | `sub_1103(HL = 深度, DE = 橫向索引)`：`HL` 先與 3、2、1、0 依序比對分出四段， | `docs/research/02-other-platforms.md:690`, `docs/research/02-other-platforms.md:721`, `docs/research/02-other-platforms.md:722` 等 4 處 |
| `sub_12EF` | `sub_12EF`／`sub_14DA` | `docs/research/02-other-platforms.md:693` |
| `sub_14DA` | `sub_12EF`／`sub_14DA` | `docs/research/02-other-platforms.md:693` |
| `sub_17FC` | `sub_17FC`／`sub_18A8` | `docs/research/02-other-platforms.md:694` |
| `sub_18A8` | `sub_17FC`／`sub_18A8` | `docs/research/02-other-platforms.md:694` |
| `sub_18AD` | 與左右側牆深度 0–2 都接上了。火炬（`sub_1956`／`sub_19CB`／`sub_18AD`） | `docs/research/02-other-platforms.md:717` |
| `sub_1956` | 與左右側牆深度 0–2 都接上了。火炬（`sub_1956`／`sub_19CB`／`sub_18AD`） | `docs/research/02-other-platforms.md:717`, `docs/research/02-other-platforms.md:739`, `docs/research/02-other-platforms.md:742` |
| `sub_19CB` | 與左右側牆深度 0–2 都接上了。火炬（`sub_1956`／`sub_19CB`／`sub_18AD`） | `docs/research/02-other-platforms.md:717` |
| `sub_1A40` | `sub_1A40`／`sub_1C2B` | `docs/research/02-other-platforms.md:689` |
| `sub_1C2B` | `sub_1A40`／`sub_1C2B` | `docs/research/02-other-platforms.md:689` |
| `sub_1F4D` | 證據在呼叫端 `sub_4752` —— 它讀完該格的牆值之後 `1` 只呼叫 `sub_1F4D`、 | `docs/research/02-other-platforms.md:692`, `docs/research/02-other-platforms.md:737`, `docs/research/02-other-platforms.md:738` |
| `sub_1FF9` | 先前把它們找到 `sub_1F4D`／`sub_1FF9` 是錯的**：那兩支是左右側牆的繪製函式。 | `docs/research/02-other-platforms.md:692`, `docs/research/02-other-platforms.md:737` |
| `sub_24FC` | 視圖是 **154×64**（`sub_24FC` 把組好的視圖整塊搬到畫面的 (16,40)）， | `docs/research/02-other-platforms.md:701` |
| `sub_29DE` | 呼叫一次 `sub_29DE(欄位序號, 值)`，把序號與偏移一次全列了出來： | `docs/formats/02-data-files.md:537` |
| `sub_2D12` | VRAM 0x4000／0xC000／0x104000），`sub_2D12` 是 VDP 初始化。 | `docs/research/02-other-platforms.md:496` |
| `sub_2DBA` | `sub_2DBA(region, vramAddr, byteLen, srcPtr)` 是 VRAM 傳輸（`region` 0/1/2 → | `docs/research/02-other-platforms.md:487`, `docs/research/02-other-platforms.md:495`, `docs/research/02-other-platforms.md:768` 等 5 處 |
| `sub_36A6` | 數第二技能**：`sub_36A6(N)` 回傳隊伍裡具備技能 N 的人數，寫進 `ds:042F`。與野外通行判定用的是同一支 | `docs/formats/06-map.md:471`, `docs/formats/06-map.md:472`, `docs/formats/06-map.md:476` 等 4 處 |
| `sub_3B80` | 解包規則抄自 `2COMBAT.img` 的 `sub_3B80`（倍率表在 DGROUP `ds:4DB8`， | `docs/formats/02-data-files.md:704` |
| `sub_3DE2` | `sub_FC38(區域類型)` 建表 → `sub_3DE2` 逐格 blit 進 `0xFF0000` → DMA | `docs/research/02-other-platforms.md:868`, `docs/research/02-other-platforms.md:882`, `docs/research/02-other-platforms.md:943` 等 5 處 |
| `sub_428C` | 順帶：同一段的 `sub_428C` 把朝向（存成 ASCII 字母 `N`/`S`/`E`/`W`）換成 | `docs/formats/06-map.md:352` |
| `sub_42FC` | `sub_42FC` 兩者都用：先把 `0xFF0000` 開頭 8 bytes DMA 到 VRAM `0xF000`， | `docs/research/02-other-platforms.md:747`, `docs/research/02-other-platforms.md:755`, `docs/research/02-other-platforms.md:858` 等 5 處 |
| `sub_4486` | `sub_4486`（開機）把三個指標存進 a5 變數： | `docs/research/02-other-platforms.md:852` |
| `sub_4752` | 證據在呼叫端 `sub_4752` —— 它讀完該格的牆值之後 `1` 只呼叫 `sub_1F4D`、 | `docs/research/02-other-platforms.md:738` |
| `sub_5188` | 黃金另有直接證據：`2PLAY.img` 的 `sub_5188` 把全隊的 `+102` 加總、夠付就 | `docs/formats/02-data-files.md:584` |
| `sub_57E0` | 播放曲子 N**（root `sub_57E0`）：十首，指標表 `ds:5214`、音高表 `ds:5144`、時值表 `ds:51F4`，曲子以 `0xFF` 收尾 | `docs/formats/07-event-script.md:118` |
| `sub_5E68` | `2PLAY.img` 的 `sub_5E68` 是前進前的通行檢查，回傳訊息序號或 `0xFFFF`（可走）： | `docs/formats/02-data-files.md:915`, `docs/formats/06-map.md:238`, `docs/formats/06-map.md:442` 等 4 處 |
| `sub_5F40` | 類別 = sub_5F40(地形碼) ; 碼 & 0x1F 之後查 ds:52B2 的 32 項表 | `docs/formats/06-map.md:470` |
| `sub_6171` | 同時釘死的是 `0x5986` 那一塊：`sub_6171` 用 `rep movsw` 複製 **`cx = 0x20` | `docs/formats/06-map.md:517` |
| `sub_6818` | call sub_6818(索引) | `docs/formats/04-graphics.md:92` |
| `sub_6EFA` | `sub_A4EE` 由主迴圈頂端（`0x07120`）呼叫，在遊戲開始之前；失敗處理 `sub_6EFA` 結尾也請求它（回到主選單） | `docs/research/md-music-scenes.md:55` |
| `sub_7036` | 2COMBAT 0xA3A1 call sub_7036(ds:9E2B) ; 圖號 | `docs/formats/04-graphics.md:90` |
| `sub_72FA` | `0x32`（測試，交給 root 的 `sub_72FA`）與付款那組函式；讀它的是 | `docs/formats/07-event-script.md:262` |
| `sub_73FC` | `sub_73FC(字串, x, y, 旗標)` 是印字函式，326 個呼叫端（133 直接、193 走 thunk | `docs/research/md-re-status.md:144`, `docs/research/md-re-status.md:152` |
| `sub_8290` | `ds:9FCF` 是目標不是攻擊者。** 前置的 `sub_8290` 隨機挑一個目標寫進 | `docs/formats/08-combat.md:31` |
| `sub_8302` | `ds:9FCF`（跳過狀況位元組 `[rec+0x26] >= 0x80` 的），`sub_8302` 則是 | `docs/formats/08-combat.md:32` |
| `sub_8398` | 出自 `2COMBAT.img` 的 `sub_8398`，完整流程見 [`docs/formats/08`](08-combat.md)。 | `docs/formats/02-data-files.md:472`, `docs/formats/02-data-files.md:804`, `docs/formats/08-combat.md:9` 等 4 處 |
| `sub_8BAE` | 命中訊息在 `sub_8BAE`：`<n> time(s) and ` 之後，`ds:9FD4 == 0` 印 | `docs/formats/08-combat.md:63` |
| `sub_8C78` | `missed!`，否則印 `hit `。隊伍攻擊的訊息在 `sub_8C78`（`back stabs`／ | `docs/formats/08-combat.md:64` |
| `sub_8E81` | `sub_8E81`。形狀與怪物那條完全不同 —— 不是百分比命中，是「擲一個上限 | `docs/formats/08-combat.md:69` |
| `sub_A4EE` | `sub_A4EE` 由主迴圈頂端（`0x07120`）呼叫，在遊戲開始之前；失敗處理 `sub_6EFA` 結尾也請求它（回到主選單） | `docs/research/md-music-scenes.md:55` |
| `sub_B620` | `sub_B620` 是 `link a6,#0` 的 C 風格函式，參數在 `$8(a6)`（`move.w $A(a6),d0` | `docs/research/md-music-driver.md:129`, `docs/research/md-music-scenes.md:30`, `docs/research/md-music-scenes.md:115` 等 6 處 |
| `sub_C5F5` | （`sub_C5F5`）先讀磁區 1 起的**三個**磁區（`ld b,3 : ld de,1`）搜一遍， | `docs/research/02-other-platforms.md:529` |
| `sub_C698` | 沒中再讀磁區 4 起的三個（`ld de,4`）；`sub_C698` 的迴圈計數 `0C0h` ＝ 192， | `docs/research/02-other-platforms.md:530` |
| `sub_CC8C` | `2MISC2.img` 的 `sub_CC8C`。等級 2–10 直接查表，11 級以上改成分段的等差累加。 | `docs/formats/07-event-script.md:500`, `docs/formats/08-combat.md:147`, `docs/release.md:19` |
| `sub_CE12` | `2CMDS.img` 的 `sub_CE12` 以 `記錄 + 基底` 取 `+0x28`（編號）與 `+0x34`（屬性）， | `docs/formats/02-data-files.md:172`, `docs/formats/02-data-files.md:184`, `docs/formats/02-data-files.md:229` 等 6 處 |
| `sub_D00E` | `sub_CE12` 呼叫四個判斷式（`sub_D00E`／`D026`／`D03E`／`D056`）決定要往哪 | `docs/formats/02-data-files.md:184` |
| `sub_F5CE` | X `0xFFF3A4`、Y `0xFFF3A6`、朝向 ASCII `0xFFF3B4`；`0xFFF3A8`–`B2` 是 `sub_F5CE` 那四組牆遮罩／位移 | `docs/research/02-other-platforms.md:907`, `docs/research/md-re-status.md:34`, `docs/research/md-re-status.md:75` 等 4 處 |
| `sub_FB86` | `World.MapIndex` 與 `ATTRIB` 記錄（DOS 資料），不是 MD 的 `sub_FB86`。 | `docs/music.md:157`, `docs/research/02-other-platforms.md:873`, `docs/research/md-music-driver.md:139` 等 10 處 |
| `sub_FC38` | `sub_FC38` 是 7 個 case 的跳表 switch（表在 `0x0FC56`），區域類型 0（五座城鎮） | `docs/research/02-other-platforms.md:866`, `docs/research/02-other-platforms.md:873`, `docs/research/02-other-platforms.md:876` 等 6 處 |
| `sub_10002` | root 的 `main`（`sub_10002`，IDA `0x10002`）取 `argv[1]` 的第一個字元 | `docs/re/01-boot-and-display-mode.md:11` |
| `sub_10E56` | `0x010F14` 與 case 19 的呼叫端 `0x010FDA` 在同一支函式（`sub_10E56`，進入地圖）裡： | `docs/research/02-other-platforms.md:872`, `docs/research/02-other-platforms.md:907`, `docs/research/md-music-driver.md:147` 等 5 處 |
| `sub_11392` | sub_11392(0) ; 開訊息視窗 | `docs/research/command-keys-oracle.md:45` |
| `sub_11622` | `sub_1173C` 對每個場景載四個檔，走 `sub_11622` → `sub_31F9E` → | `docs/research/02-other-platforms.md:291` |
| `sub_11676` | `sub_11676(欄, 0x11)` 逐項相符。按鍵表與函式內容見 | `docs/playtest/01-oracle-timeline.md:374`, `docs/research/command-keys-oracle.md:47`, `docs/research/command-keys-oracle.md:107` |
| `sub_11726` | 指標會被當引數傳給別的函式（印名字的 `sub_11726`、`sub_1C8AA`、 | `docs/formats/02-data-files.md:60`, `docs/research/command-keys-oracle.md:48` |
| `sub_1173C` | `sub_1173C` 對每個場景載四個檔，走 `sub_11622` → `sub_31F9E` → | `docs/research/02-other-platforms.md:291` |
| `sub_11A30` | do al = sub_11A30() ; 取一個鍵（thunk 0x16DBA） | `docs/research/spell-interaction-oracle.md:85` |
| `sub_11BA6` | root `sub_11BA6` 用 `int 10h` `AX=1A00`（Display Combination Code）取得代碼， | `docs/re/01-boot-and-display-mode.md:60` |
| `sub_11C20` | 經 `ds:4A04` 的 16 項轉換表換成內部碼，再由 `sub_11C20` 設旗標： | `docs/re/01-boot-and-display-mode.md:61` |
| `sub_11C88` | 擲骰用的是 root 的 `sub_11C88`，回傳 **`[lo, hi]` 兩端都含**的均勻值 | `docs/formats/02-data-files.md:808`, `docs/formats/02-data-files.md:830`, `docs/formats/04-graphics.md:161` 等 5 處 |
| `sub_11CCC` | 播種在 `sub_11CCC`：`int 21h` AH=2Ch 取 DOS 時間，把 DX（秒與百分秒） | `docs/formats/02-data-files.md:829` |
| `sub_11CDA` | 的程式碼走訪那塊緩衝區** —— 整段交給顯示驅動（`sub_11CDA`，功能碼 `0x16`）， | `docs/formats/04-graphics.md:219` |
| `sub_11E36` | sub_11E36(檔名) ; 檔案在不在 | `docs/formats/03-lzw-compression.md:106` |
| `sub_11E64` | `sub_11E64` 的四個呼叫端都只推兩個參數（`add sp, 4`），**沒有人能從外面 | `docs/formats/03-lzw-compression.md:102`, `docs/formats/03-lzw-compression.md:105`, `docs/formats/03-lzw-compression.md:121` 等 4 處 |
| `sub_11ED2` | var_6 = sub_11ED2(0516h, 檔名) ; ← 在「要解密」清單裡嗎 → 解密旗標 | `docs/formats/03-lzw-compression.md:107`, `docs/formats/03-lzw-compression.md:108` |
| `sub_121B6` | sub_121B6(檔名, 長度輸出, var_6) | `docs/formats/03-lzw-compression.md:109` |
| `sub_12242` | MM2 的資料檔幾乎全部經過同一套 LZW 壓縮。演算法讀自 `MM2.EXE` 的 `sub_12242` | `docs/formats/03-lzw-compression.md:3`, `docs/formats/03-lzw-compression.md:21`, `docs/formats/03-lzw-compression.md:94` 等 4 處 |
| `sub_124B0` | sub_124B0(檔名, 長度輸出, var_6) | `docs/formats/03-lzw-compression.md:111` |
| `sub_12616` | `sub_12616` 是另一種資料保護，與 §4 的位移不同： | `docs/formats/03-lzw-compression.md:78` |
| `sub_12734` | 寫入、推進驅動呼叫、以及 `sub_12734` 清掉它。**沒有任何 EXE 或 overlay | `docs/formats/04-graphics.md:218` |
| `sub_13268` | sub_13268(lo, hi): | `docs/research/spell-interaction-oracle.md:84` |
| `sub_1354A` | `sub_1354A` 從 −3 起，門檻表 `ds:4D84` 裡每有一項小於該屬性值就加一： | `docs/formats/08-combat.md:638`, `docs/formats/08-combat.md:660` |
| `sub_136A6` | `sub_136A6(0x0D) >= 2`，否則回傳索引 5（`Impassable!`）。這可作控制流對照， | `docs/research/water-traversal-oracle.md:45`, `docs/research/water-traversal-oracle.md:46` |
| `sub_13700` | 2. `sub_13700`（`seg000:13700`，`seg000:13700+0`）：把 `ds:03D9` 與 `03D8`、 | `docs/research/water-traversal-oracle.md:60`, `docs/research/water-traversal-oracle.md:132`, `docs/research/water-traversal-oracle.md:157` 等 4 處 |
| `sub_13766` | 拿走背包裡的物品**：逐人掃 `+58` 的六格，找到就用 root `sub_13766` 移除並往前補，一次只拿一件 | `docs/formats/07-event-script.md:144`, `docs/formats/07-event-script.md:581` |
| `sub_13928` | 對選定的每個人呼叫 sub_13928(記錄, 值, …) | `docs/formats/07-event-script.md:728`, `docs/formats/07-event-script.md:736` |
| `sub_13A64` | 擲 < 0x60 且 技能 >= 擲 → sub_13A64 開門 + 訊息 4（Success!） | `docs/formats/06-map.md:80`, `docs/formats/06-map.md:395`, `docs/polish-spec.md:17` 等 7 處 |
| `sub_13A9E` | `2COMBAT _2combat_e03` `0x1A4E7`；`ds:549E` ＝ root `sub_13A9E` | `docs/polish-spec.md:21` |
| `sub_13B68` | `sub_13B68(記錄, n)` 是共用的加齡程序：`+33` 加 n，**上限 200**。 | `docs/formats/09-spells.md:362`, `docs/formats/09-spells.md:367`, `docs/formats/09-spells.md:368` 等 4 處 |
| `sub_13B80` | `ds:9680` 是場上每個位置的**怪物編號**（`sub_13B80` 拿它當 | `docs/formats/09-spells.md:573` |
| `sub_1410A` | sub_1410A(ds:4D9B) | `docs/research/command-keys-oracle.md:46` |
| `sub_1423E` | ds:59C6 方向遮罩 北 0xC0、東 0x30、南 0x0C、西 0x03 （`sub_1423E` 依朝向設定） | `docs/formats/02-data-files.md:361`, `docs/formats/06-map.md:66`, `docs/formats/06-map.md:197` 等 4 處 |
| `sub_1428C` | 不是猜的。`sub_142DE`（前進一步）把 `sub_1428C` 給的兩個增量分別加到 | `docs/formats/07-event-script.md:413`, `docs/formats/07-event-script.md:414` |
| `sub_142DE` | 不是猜的。`sub_142DE`（前進一步）把 `sub_1428C` 給的兩個增量分別加到 | `docs/formats/06-map.md:113`, `docs/formats/07-event-script.md:413` |
| `sub_14478` | call sub_14478 ; 非 -1 → 撞到東西 | `docs/formats/06-map.md:120` |
| `sub_147D8` | 3. `sub_147D8` 呼叫鏈內（`seg000:1493D` 附近）：若 `ds:03D9 != 0`，顯示目前 | `docs/polish-spec.md:20`, `docs/research/command-keys-oracle.md:40`, `docs/research/water-traversal-oracle.md:62` 等 4 處 |
| `sub_14C4A` | `root sub_14C4A` | `docs/research/chest-trigger-oracle.md:60` |
| `sub_14EFE` | 停頓** `(7N+1) × 2` 個單位（root `sub_14EFE`），可被按鍵中斷 | `docs/formats/07-event-script.md:134` |
| `sub_14F3A` | root 的 `sub_14F3A` 每次刷新隊伍面板時對每個成員跑一次： | `docs/formats/08-combat.md:402`, `docs/formats/08-combat.md:634` |
| `sub_15644` | 分派寫在 root 的 `sub_15644`： | `docs/formats/02-data-files.md:1070`, `docs/formats/09-spells.md:13` |
| `sub_15772` | `sub_15772` 讀到 `FF` 就把段指標設回 `0FFFFh` 並回傳 0，外層迴圈於是 | `docs/formats/04-graphics.md:163`, `docs/formats/04-graphics.md:166`, `docs/formats/04-graphics.md:179` 等 5 處 |
| `sub_15E68` | 1. `sub_15E68`（`seg000:15F10` 附近）：水域通行 gate，`cmp byte ptr ds:3D9h, 0`。 | `docs/formats/01-overlay-and-memory-layout.md:247`, `docs/formats/06-map.md:39`, `docs/formats/06-map.md:75` 等 9 處 |
| `sub_15F40` | 判定有無方向障礙；野外分支（`ds:039D != 0`）再用 `sub_15F40`（`seg000:15F40`） | `docs/research/water-traversal-oracle.md:30` |
| `sub_160B4` | `sub_160B4` 與 `sub_160D0` 是同一支 `sub_160ED` 的兩個入口，差別只在第三個 | `docs/formats/06-map.md:366`, `docs/formats/06-map.md:367`, `docs/formats/06-map.md:368` 等 6 處 |
| `sub_160D0` | `sub_160B4` 與 `sub_160D0` 是同一支 `sub_160ED` 的兩個入口，差別只在第三個 | `docs/formats/06-map.md:365`, `docs/formats/06-map.md:376` |
| `sub_160ED` | 載入器 `sub_160ED` 只用 `int 21h` 的 `3Dh`／`42h`／`3Fh`／`48h`／`49h`，沒有 `40h`；其他常式是否會寫同一個檔沒有掃過 | `docs/formats/01-overlay-and-memory-layout.md:227`, `docs/formats/03-lzw-compression.md:128`, `docs/formats/06-map.md:301` 等 6 處 |
| `sub_16818` | `sub_16818`（`monsters.16`）與 `sub_160ED`（`MAP.DAT` 的地圖記錄）另有自己的 | `docs/formats/03-lzw-compression.md:128`, `docs/formats/04-graphics.md:216` |
| `sub_16DD2` | `sub_16DD2` 的四個參數在另一處是 `(1, 0Eh, 0Dh, …)`，形狀一致 —— | `docs/formats/04-graphics.md:469`, `docs/formats/04-graphics.md:471` |
| `sub_16E62` | `sub_17066` 之後 `sub_16E62(0)` | `docs/formats/08-combat.md:453` |
| `sub_16EC2` | 原版用 `sub_16EC2(下限, 上限)` 讀一個按鍵，`0x1B` 表示取消。 | `docs/formats/09-spells.md:269` |
| `sub_16EE6` | 要求輸入文字**：`sub_16EE6(54C4h, 10)` 讀十個字進 `ds:54C4`，讀到空的就重來 | `docs/formats/07-event-script.md:151` |
| `sub_16EF2` | 由 `_2play_e13`（含一個 5×5 的迴圈）與 `_2play_e14` 透過 `sub_16EF2` | `docs/formats/04-graphics.md:353` |
| `sub_16F76` | 掃出指向它的 thunk 在 `0x16F76` —— 而 `sub_16F76` 正是 `2COMBAT.OVL` | `docs/formats/01-overlay-and-memory-layout.md:280` |
| `sub_16FA6` | call sub_16FA6 | `docs/formats/02-data-files.md:1065` |
| `sub_16FFA` | 取記錄 → `sub_16FFA(記錄)`，回 `0FFFFh` 表示取消 | `docs/formats/08-combat.md:452` |
| `sub_17036` | `0x0b` 讀兩個位元組，第一個經 `sub_18EE6` 換算後交給 `sub_17036`。 | `docs/formats/04-graphics.md:466`, `docs/formats/07-event-script.md:188` |
| `sub_1704E` | 繪圖入口 `sub_1704E(種類, x, y)` | `docs/formats/04-graphics.md:465`, `docs/formats/04-graphics.md:484` |
| `sub_1705A` | `E` 呼叫的 `sub_1705A` 是 thunk。照 thunk 格式（[`01`](01-overlay-and-memory-layout.md) §3.5） | `docs/formats/08-combat.md:454`, `docs/formats/08-combat.md:464` |
| `sub_17066` | `sub_17066` 之後 `sub_16E62(0)` | `docs/formats/08-combat.md:453` |
| `sub_17096` | 重畫名單、發一次音效 sub_17096(8) | `docs/formats/08-combat.md:314` |
| `sub_1710E` | 與傷害加成），再加上記錄 `+107` 經 `sub_1710E` 換算的值；`ds:54A3` 之後 | `docs/formats/08-combat.md:591` |
| `sub_1714A` | `sub_1714A` 的第一個參數與手冊的「目標」欄逐條相符（1／2／3／4／5／6／10）， | `docs/formats/09-spells.md:435`, `docs/formats/09-spells.md:437`, `docs/formats/09-spells.md:451` 等 4 處 |
| `sub_17162` | mov si, sub_17162(第 N 人) ; 角色記錄 | `docs/formats/07-event-script.md:369` |
| `sub_1732A` | sub_1732A() 非 0 → ds:0395 = 1（要求重畫） | `docs/formats/07-event-script.md:729` |
| `sub_1743E` | 之類 → 呼叫 `sub_1743E` → 換回來）。城鎮與野外那些「付 N 金幣送你 | `docs/formats/02-data-files.md:409` |
| `sub_174C2` | 等按鍵**：`sub_174C2` 之後進取鍵迴圈，0／`0D`／`F0`／`F2` 不算。出現 226 次，是訊息的分頁點 | `docs/formats/07-event-script.md:112` |
| `sub_17726` | 15 路跳表（技能編號 1–15 減一當索引），呼叫 `sub_17726(欄位位址, 量)` 減值： | `docs/formats/08-combat.md:949` |
| `sub_17981` | `sub_17981` 做實際的檔案讀取，用到描述表的 +0x08（重定位項數）、+0x0A（載入段）、 | `docs/formats/01-overlay-and-memory-layout.md:354` |
| `sub_17A57` | `sub_17A57` 負責載入，CX = overlay 編號： | `docs/formats/01-overlay-and-memory-layout.md:341` |
| `sub_17AB4` | `sub_17AB4`（seg003:02E4）保存全部暫存器後，從堆疊取出回傳位址， | `docs/formats/01-overlay-and-memory-layout.md:330` |
| `sub_17E10` | `sub_17E10` 第二段（先測 `ds:54A4 == 0`） | `docs/formats/08-combat.md:489`, `docs/formats/08-combat.md:490` |
| `sub_17EB9` | `sub_17EB9` 每走一步做一次： | `docs/formats/02-data-files.md:953` |
| `sub_17FB2` | 怪物攻擊的播報動詞是**隨機八選一**。`sub_17FB2` 擲 `rand(1, 8)`， | `docs/formats/08-combat.md:596` |
| `sub_18056` | sub_18056() 印 " casts"，正常施展 | `docs/formats/08-combat.md:252` |
| `sub_1814A` | 相鄰的 bit6 進 `ds:9E35`，170 隻有）。`ds:9E35` 在 `2COMBAT sub_1814A` | `docs/formats/02-data-files.md:765` |
| `sub_18368` | sub_18368() 印 "*** Spell Failed ***" | `docs/formats/08-combat.md:250` |
| `sub_18398` | remake 的動詞擲點在傷害結算之後，原版在 `sub_18398` 尾巴的播報裡； | `docs/formats/08-combat.md:614` |
| `sub_1845A` | → sub_1845A 後排也打得到 | `docs/formats/08-combat.md:198` |
| `sub_1846C` | → sub_1846C 普通近戰 | `docs/formats/08-combat.md:196` |
| `sub_1847E` | `sub_1847E` 回傳 0 不是「不行動」，是「這次不用特殊攻擊」。** | `docs/formats/08-combat.md:193`, `docs/formats/08-combat.md:203`, `docs/formats/08-combat.md:206` |
| `sub_184D4` | sub_184D4(這隻怪) ; 前置 | `docs/formats/08-combat.md:192` |
| `sub_184FE` | `sub_184FE`（怪物攻擊）尾巴會把它歸零。等級：已證實。 | `docs/formats/08-combat.md:536` |
| `sub_18624` | 1. **建角色的寫入處**（`1MENU2.img` 的 `sub_18624`）。畫面上的七格依序 | `docs/formats/02-data-files.md:623`, `docs/formats/08-combat.md:415`, `docs/formats/08-combat.md:807` 等 5 處 |
| `sub_18674` | 七格抗性攤在 `ds:9E36` 起的**連續七個位元組**，`sub_18674(屬性-1)` 直接 | `docs/formats/02-data-files.md:726`, `docs/formats/02-data-files.md:770`, `docs/formats/02-data-files.md:775` 等 4 處 |
| `sub_18696` | sub_18696(1, 這隻怪, 屬性) | `docs/formats/08-combat.md:627` |
| `sub_188B2` | 擲完接著呼叫 `sub_18952`（算職業資格）與 `sub_188B2`（畫上去）。 | `docs/formats/10-character-creation.md:22`, `docs/formats/10-character-creation.md:49` |
| `sub_188FC` | （`sub_1A1A0`／`sub_19B44`）與戰鬥戰利品（`sub_19A3C`／`sub_188FC`）。 | `docs/formats/08-combat.md:302`, `docs/formats/08-combat.md:757`, `docs/research/chest-trigger-oracle.md:20` 等 5 處 |
| `sub_18952` | 峰值在 `+0x14`（準確度），而職業門檻（`sub_18952`）要的正是智慧與準確度； | `docs/formats/02-data-files.md:647`, `docs/formats/08-combat.md:783`, `docs/formats/10-character-creation.md:22` 等 4 處 |
| `sub_189D2` | sub_189D2() ; 通知隊伍：纏住你的那隻沒了 | `docs/formats/08-combat.md:301` |
| `sub_189EE` | `sub_189EE` 的七格暫存正好解釋了這個安排怎麼來的。 | `docs/formats/10-character-creation.md:84` |
| `sub_189F8` | 屬性擲骰（`sub_189F8`） | `docs/formats/10-character-creation.md:6` |
| `sub_18A22` | 逃走：ds:54A6 = 1 → sub_18AB8() 印訊息 → sub_18A22() 移除 → ds:54A6 = 0 | `docs/formats/02-data-files.md:1053`, `docs/formats/08-combat.md:305`, `docs/formats/08-combat.md:308` 等 4 處 |
| `sub_18A60` | A–G sub_18A60：挑第二項，兩項對調，重算可選職業 | `docs/formats/08-combat.md:772` |
| `sub_18AB8` | 逃走：ds:54A6 = 1 → sub_18AB8() 印訊息 → sub_18A22() 移除 → ds:54A6 = 0 | `docs/formats/08-combat.md:303`, `docs/formats/08-combat.md:334`, `docs/formats/08-combat.md:338` |
| `sub_18AF4` | 死亡與逃走走**同一條路**，`sub_18AF4`： | `docs/formats/08-combat.md:298`, `docs/formats/09-spells.md:521` |
| `sub_18B1A` | `sub_18B1A` 依序問種族（`ds:08B7`，1–5）、陣營（`ds:08D0`，1–3）、 | `docs/formats/08-combat.md:773`, `docs/formats/08-combat.md:801` |
| `sub_18C78` | 隊伍那一側不擲：`sub_18C78` 固定用 `ds:11D5`（` attacks `）或 | `docs/formats/08-combat.md:610` |
| `sub_18DAA` | 對照組是同一批的 `ds:03E3`（祝福術）：同一支掃描在 `sub_18DAA` 找到它加進 | `docs/formats/08-combat.md:487`, `docs/formats/08-combat.md:506`, `docs/formats/08-combat.md:535` 等 5 處 |
| `sub_18DC8` | `sub_19716` 讀一個位元組（`sub_18DC8`），先設 `ds:0395 = 1`（要求重畫）、 | `docs/formats/07-event-script.md:16`, `docs/formats/07-event-script.md:25`, `docs/formats/07-event-script.md:38` 等 5 處 |
| `sub_18DD8` | （`(hi << 8) + lo`），`0x24` 走 `sub_18DD8` 一次讀完 —— 兩種寫法， | `docs/formats/07-event-script.md:448`, `docs/formats/07-event-script.md:671` |
| `sub_18DF8` | `sub_18DF8` 先呼叫 `sub_18DD8` 讀兩個湊成 word，再讀第三個當高位。 | `docs/formats/07-event-script.md:448` |
| `sub_18E22` | 讀全域變數**：選擇器經 `sub_18E22` 換成位址，值進 `ds:042F`。見 §12 | `docs/formats/07-event-script.md:128`, `docs/formats/07-event-script.md:549` |
| `sub_18EE6` | 顯示一張 `monsters.16` 的圖**：參數經 `sub_18EE6` 換成圖號。見 §14 | `docs/formats/07-event-script.md:116`, `docs/formats/07-event-script.md:188`, `docs/formats/07-event-script.md:189` 等 5 處 |
| `sub_18F48` | 主迴圈在 `sub_18F48`： | `docs/formats/08-combat.md:767` |
| `sub_18F64` | 長度表本身在原版是一張查表，`sub_18F64`（跳過 N 個 opcode）用它前進： | `docs/formats/07-event-script.md:65` |
| `sub_18FD0` | `sub_18FD0` 讀一個位元組 N，把字串指標 `word_154C0` 重設到區塊開頭 | `docs/formats/02-data-files.md:307`, `docs/formats/07-event-script.md:54`, `docs/formats/07-event-script.md:78` |
| `sub_19016` | `2PLAY.OVL` 有兩支常式操作它。**`sub_19016` 逐位元組讀字串**： | `docs/formats/02-data-files.md:295`, `docs/formats/07-event-script.md:56` |
| `sub_1905E` | 01 NN sub_1905E → loc_16E79 靠左 | `docs/formats/07-event-script.md:47`, `docs/formats/07-event-script.md:106` |
| `sub_19074` | 02 NN sub_19074 → loc_16EFD 開一個 0x26×0x16 的視窗 | `docs/formats/07-event-script.md:48`, `docs/formats/07-event-script.md:107` |
| `sub_190C0` | `sub_190D6` 把 `ds:54A4` 設 0（近戰）、`sub_190C0` 設 1（射擊）， | `docs/formats/08-combat.md:449`, `docs/formats/08-combat.md:534` |
| `sub_190D6` | `sub_190D6` 把 `ds:54A4` 設 0（近戰）、`sub_190C0` 設 1（射擊）， | `docs/formats/08-combat.md:450`, `docs/formats/08-combat.md:456`, `docs/formats/08-combat.md:534` |
| `sub_190F2` | `sub_190F2` | `docs/formats/07-event-script.md:108` |
| `sub_19110` | 04 NN sub_19110 → loc_16F52 算出長度後置中 | `docs/formats/07-event-script.md:49`, `docs/formats/07-event-script.md:109` |
| `sub_1914A` | 讀取端在 `2COMBAT.img` 的 `0x19162`，而那支函式（`sub_1914A`）是 | `docs/formats/02-data-files.md:1027`, `docs/formats/08-combat.md:420` |
| `sub_19160` | `sub_19160` | `docs/formats/07-event-script.md:110` |
| `sub_191EC` | `sub_191EC` | `docs/formats/07-event-script.md:111` |
| `sub_193B8` | 切換腳本讀取模式**：`ds:50FF = 0FDh` 之後呼叫 `sub_193B8(1)`。模式 `0FDh` 下，終止碼 `0FFh` 變成「位置歸零重讀」 | `docs/formats/07-event-script.md:112`, `docs/formats/07-event-script.md:113`, `docs/formats/07-event-script.md:782` |
| `sub_1940E` | 切換腳本讀取模式**：`ds:50FF = 0FDh` 之後呼叫 `sub_193B8(1)`。模式 `0FDh` 下，終止碼 `0FFh` 變成「位置歸零重讀」 | `docs/formats/07-event-script.md:113`, `docs/formats/07-event-script.md:766` |
| `sub_1941E` | 先設 `ds:50FF = 0xFD` 再呼叫 `sub_1941E(1)`，也是 Y／N 詢問，取鍵的路徑不同 | `docs/formats/07-event-script.md:114`, `docs/formats/07-event-script.md:115` |
| `sub_1946E` | 先設 `ds:50FF = 0xFD` 再呼叫 `sub_1941E(1)`，也是 Y／N 詢問，取鍵的路徑不同 | `docs/formats/07-event-script.md:115` |
| `sub_1947E` | 顯示一張 `monsters.16` 的圖**：參數經 `sub_18EE6` 換成圖號。見 §14 | `docs/formats/07-event-script.md:116` |
| `sub_194D4` | `sub_194D4` | `docs/formats/07-event-script.md:117` |
| `sub_19560` | 播放曲子 N**（root `sub_57E0`）：十首，指標表 `ds:5214`、音高表 `ds:5144`、時值表 `ds:51F4`，曲子以 `0xFF` 收尾 | `docs/formats/07-event-script.md:118` |
| `sub_1956E` | 腳本庫**（`sub_1956E`，範圍表見 [`07-event-script.md`](../formats/07-event-script.md) §11）。 | `docs/formats/02-data-files.md:415`, `docs/formats/02-data-files.md:419`, `docs/formats/07-event-script.md:503` 等 9 處 |
| `sub_19640` | 場上可以有上百隻，一次打得到的只有前排幾隻。`sub_19640` 決定它： | `docs/formats/08-combat.md:285` |
| `sub_19716` | 重建影像的 IDA 位址空間中：`sub_19716 @ 0x19716`、`sub_1956E @ 0x1956E`、 | `docs/formats/02-data-files.md:415`, `docs/formats/02-data-files.md:419`, `docs/formats/07-event-script.md:119` 等 10 處 |
| `sub_1974C` | `ds:0FC2` 由 `sub_1974C` 算出 —— 它掃一遍隊伍記錄，取最高的 | `docs/formats/08-combat.md:341` |
| `sub_198C8` | 要求重畫**（`ds:0395 = 1` 後呼叫 `sub_1A580`），出現 278 次 | `docs/formats/07-event-script.md:120` |
| `sub_198D2` | `sub_198D2` | `docs/formats/07-event-script.md:121` |
| `sub_198F2` | `sub_198F2` | `docs/formats/07-event-script.md:122` |
| `sub_198FE` | `sub_199B8` 反覆加怪直到 `sub_198FE` 判定湊夠了： | `docs/formats/08-combat.md:344` |
| `sub_19912` | 靜態數呼叫點會漏掉**迴圈與條件分支**：`sub_19912`（opcode `0x12`）裡的 | `docs/formats/07-event-script.md:80`, `docs/formats/07-event-script.md:98`, `docs/formats/07-event-script.md:123` |
| `sub_19984` | `sub_19984` | `docs/formats/07-event-script.md:124` |
| `sub_19990` | `sub_19990` | `docs/formats/07-event-script.md:125` |
| `sub_199B8` | `sub_199B8` 反覆加怪直到 `sub_198FE` 判定湊夠了： | `docs/formats/07-event-script.md:307`, `docs/formats/08-combat.md:344` |
| `sub_19A02` | 三條 opcode（`0x26` 填、`0x31` 讀並寫出完整退路、`sub_19A02` 給約定） | `docs/formats/07-event-script.md:126`, `docs/formats/07-event-script.md:129`, `docs/formats/07-event-script.md:271` 等 7 處 |
| `sub_19A30` | 容器不同（`sub_19A30` 開檔後讀 48 bytes 標頭、1 byte 計數、再讀 `計數−1` bytes | `docs/research/02-other-platforms.md:212`, `docs/research/02-other-platforms.md:223` |
| `sub_19A3C` | `sub_19B88` 生成 0–3 件物品；每件 `sub_19A3C` 依 `ds:10EA/10F6` 遭遇 band | `docs/formats/02-data-files.md:53`, `docs/formats/08-combat.md:753`, `docs/polish-spec.md:126` 等 6 處 |
| `sub_19ABC` | `sub_19ABC` | `docs/formats/07-event-script.md:127` |
| `sub_19B20` | 讀全域變數**：選擇器經 `sub_18E22` 換成位址，值進 `ds:042F`。見 §12 | `docs/formats/07-event-script.md:128` |
| `sub_19B38` | `sub_19B38` → `sub_19A02(1)` | `docs/formats/07-event-script.md:129` |
| `sub_19B44` | （`sub_1A1A0`／`sub_19B44`）與戰鬥戰利品（`sub_19A3C`／`sub_188FC`）。 | `docs/formats/07-event-script.md:130`, `docs/formats/08-combat.md:747`, `docs/research/chest-trigger-oracle.md:53` 等 5 處 |
| `sub_19B88` | linear `0x1A28B` 呼叫 `sub_19BF8`。它先把 `ds:0434` 清零，再由 `sub_19B88` | `docs/formats/08-combat.md:752`, `docs/research/chest-trigger-oracle.md:17` |
| `sub_19BF8` | 位址是 IDA composite linear `2COMBAT.img`；`sub_1A0D4` 呼叫 `sub_19BF8` 約在 | `docs/formats/08-combat.md:752`, `docs/research/chest-trigger-oracle.md:16`, `docs/research/chest-trigger-oracle.md:56` 等 5 處 |
| `sub_19C1A` | `sub_19C1A` | `docs/formats/07-event-script.md:131` |
| `sub_19C40` | `sub_19C40` | `docs/formats/07-event-script.md:132` |
| `sub_19C5E` | `sub_19C5E` | `docs/formats/07-event-script.md:133` |
| `sub_19C72` | 停頓** `(7N+1) × 2` 個單位（root `sub_14EFE`），可被按鍵中斷 | `docs/formats/07-event-script.md:134` |
| `sub_19C8A` | `sub_19C8A` | `docs/formats/07-event-script.md:135` |
| `sub_19CB8` | `sub_19CB8`，每次用**同一個**數值。 | `docs/formats/07-event-script.md:476` |
| `sub_19E40` | `sub_19F38` → `sub_19E40(1)` | `docs/formats/07-event-script.md:136`, `docs/formats/07-event-script.md:137` |
| `sub_19F38` | `sub_19F38` → `sub_19E40(1)` | `docs/formats/07-event-script.md:137` |
| `sub_19F44` | if ds:0426 != 0: sub_19F44() | `docs/formats/02-data-files.md:1038` |
| `sub_19F90` | `sub_19F90` | `docs/formats/07-event-script.md:139` |
| `sub_1A01E` | `sub_1A01E` | `docs/formats/07-event-script.md:141` |
| `sub_1A04C` | `sub_1A04C` | `docs/formats/07-event-script.md:142` |
| `sub_1A082` | `sub_1A082`（opcode `0x26`） | `docs/formats/07-event-script.md:143`, `docs/formats/07-event-script.md:306`, `docs/formats/07-event-script.md:821` |
| `sub_1A0D4` | 位址是 IDA composite linear `2COMBAT.img`；`sub_1A0D4` 呼叫 `sub_19BF8` 約在 | `docs/formats/08-combat.md:751`, `docs/research/chest-trigger-oracle.md:16`, `docs/research/chest-trigger-oracle.md:90` |
| `sub_1A126` | 拿走背包裡的物品**：逐人掃 `+58` 的六格，找到就用 root `sub_13766` 移除並往前補，一次只拿一件 | `docs/formats/07-event-script.md:144`, `docs/formats/07-event-script.md:580` |
| `sub_1A19A` | `sub_1A19A` | `docs/formats/07-event-script.md:145` |
| `sub_1A1A0` | 事件腳本 `0x2a`（`2PLAY.OVL` `sub_1A1A0`，長度 15）寫入 3 bytes 金幣、2 bytes | `docs/formats/07-event-script.md:146`, `docs/research/chest-trigger-oracle.md:37`, `docs/research/chest-trigger-oracle.md:52` 等 4 處 |
| `sub_1A1E2` | `sub_1A1E2` | `docs/formats/07-event-script.md:147` |
| `sub_1A202` | `ds:03C8` 全十五個資料庫**只有一處寫**（`2PLAY sub_1A202`，事件 opcode | `docs/formats/07-event-script.md:148`, `docs/polish-spec.md:92` |
| `sub_1A21E` | `sub_1A21E` | `docs/formats/07-event-script.md:149` |
| `sub_1A386` | `sub_1A386` | `docs/formats/07-event-script.md:150` |
| `sub_1A404` | 要求輸入文字**：`sub_16EE6(54C4h, 10)` 讀十個字進 `ds:54C4`，讀到空的就重來 | `docs/formats/07-event-script.md:151` |
| `sub_1A45A` | `sub_1A45A` | `docs/formats/07-event-script.md:152` |
| `sub_1A4BC` | opcode `0x31`（`sub_1A4BC`）另外把對象值 **8** 也對到 `ds:54BE`， | `docs/formats/07-event-script.md:153`, `docs/formats/07-event-script.md:312` |
| `sub_1A570` | 數第二技能**：`sub_36A6(N)` 回傳隊伍裡具備技能 N 的人數，寫進 `ds:042F`。與野外通行判定用的是同一支 | `docs/formats/07-event-script.md:154` |
| `sub_1A580` | 要求重畫**（`ds:0395 = 1` 後呼叫 `sub_1A580`），出現 278 次 | `docs/formats/07-event-script.md:120` |
| `sub_1A606` | 已證實是 `sub_1A606` 的腳本段號**，不是字串序號。一般事件段的腳本區以 `0xFF` 起頭，所以解析後直接取 `Scripts[Index]`；不要再減一。腳本庫由特殊設施送入 1 起算 selector… | `docs/formats/01-overlay-and-memory-layout.md:121`, `docs/formats/02-data-files.md:331`, `docs/formats/02-data-files.md:349` 等 7 處 |
| `sub_1A82C` | 另外 12 條走 `sub_1A82C(sides, bonus)`（在 `2COMBAT` 裡）： | `docs/formats/09-spells.md:256`, `docs/formats/09-spells.md:476` |
| `sub_1A85C` | `2PLAY.OVL` 的 `sub_1A85C` 給出完整佈局：從偏移 0 開始每次讀 3 個位元組， | `docs/formats/02-data-files.md:315` |
| `sub_1A882` | 原版的 `P` 指令（`sub_1A882`）畫的就是這五條，旗標指標在 `ds:136C`、 | `docs/formats/08-combat.md:482`, `docs/formats/08-combat.md:507` |
| `sub_1A8C4` | 格 80 的事件 `Index=21` 直接傳給 `sub_1A606`，一般段對應 `Scripts[21] = 0e 11`。 | `docs/formats/02-data-files.md:336`, `docs/formats/02-data-files.md:413`, `docs/formats/02-data-files.md:418` 等 7 處 |
| `sub_1AA00` | `sub_1AA00(記錄基底, 選擇器)` 是一張 **128 項的跳表**，每一項把 | `docs/formats/07-event-script.md:320` |
| `sub_1AFBC` | （`>= 0x80` 就跳過），與先前由 `sub_1AFBC` 定出的狀況欄位位置一致 —— | `docs/formats/08-combat.md:58`, `docs/formats/09-spells.md:212` |
| `sub_1B0B2` | 128 項裡 126 項是這個形狀，另外兩項（`0x00`／`0x01`）指向 `sub_1B0B2`， | `docs/formats/07-event-script.md:329`, `docs/formats/07-event-script.md:846` |
| `sub_1B1D4` | 場景碼 `ds:039C` 由 `sub_1B1D4` 從 `ATTRIB.DAT` 的 `+4` 算出來， | `docs/formats/07-event-script.md:622` |
| `sub_1B410` | 原版依據。** `2PLAY sub_1B410(地圖編號)` 在三張各 7 筆的表上做區間查找， | `docs/formats/06-map.md:527`, `docs/polish-spec.md:33` |
| `sub_1B4E0` | 讀它們的地方不是平面，是當前格的快取。** `2PLAY` 的 `sub_1B4E0` 每步做 | `docs/formats/06-map.md:411`, `docs/formats/06-map.md:595` |
| `sub_1B5EA` | `sub_1B5EA` 的 X 參數是 `0xFF` 時改用 `ATTRIB.DAT` 的 `+14` | `docs/formats/02-data-files.md:913`, `docs/formats/07-event-script.md:408`, `docs/formats/07-event-script.md:433` |
| `sub_1B70C` | 怪物特殊攻擊 case 2 2COMBAT sub_1B70C cmp 0FFh / inc ← 施加，上限 255 | `docs/formats/08-combat.md:1076`, `docs/research/water-traversal-oracle.md:155` |
| `sub_1B75E` | 方位由 `2PLAY sub_1B75E`（走出邊界時換圖）定死，不是靠地圖回推 —— | `docs/formats/02-data-files.md:933`, `docs/research/world-grid-oracle.md:11`, `docs/research/world-grid-oracle.md:74` |
| `sub_1B89E` | `sub_1B89E` 等八支），那些也全部讀過，沒有第三個讀取端。 | `docs/formats/02-data-files.md:61` |
| `sub_1B92E` | `sub_1B92E`（背包，A–F）與 `sub_1B9A4`（已裝備，1–6）： | `docs/formats/02-data-files.md:153` |
| `sub_1B9A4` | `sub_1B92E`（背包，A–F）與 `sub_1B9A4`（已裝備，1–6）： | `docs/formats/02-data-files.md:153` |
| `sub_1BA18` | 判讀點是 `2COMBAT.img` 的 `sub_1BA18`（戰鬥中的 `U`）與 `2CMDS.img` 的 | `docs/formats/02-data-files.md:72` |
| `sub_1BBAE` | 其餘（1..0x7F） → 走 sub_1BBAE／sub_1CF34 的八路 byte 效果 | `docs/formats/02-data-files.md:78` |
| `sub_1BE24` | `2PLAY sub_1BE24` 是唯一的地圖載入端，由 `byte_10399`（上次載入的地圖編號） | `docs/formats/06-map.md:357`, `docs/formats/06-map.md:423`, `docs/formats/06-map.md:620` 等 5 處 |
| `sub_1BE92` | `sub_1BE92` 是唯一持久的那一份： | `docs/formats/06-map.md:430` |
| `sub_1C130` | 參數不合法時 `sub_1C130` 印出這段再 `exit(1)`（字串取自 DGROUP 初值段， | `docs/formats/09-spells.md:86`, `docs/formats/09-spells.md:87`, `docs/formats/09-spells.md:487` 等 4 處 |
| `sub_1C16A` | `sub_1C16A` | `docs/formats/09-spells.md:88`, `docs/formats/09-spells.md:465` |
| `sub_1C178` | 恢復狀態**（`sub_1C178`）：付錢之後呼叫 `sub_1C698`。它以基礎生命上限 | `docs/formats/08-combat.md:818` |
| `sub_1C1AC` | `sub_1C1AC` | `docs/formats/09-spells.md:91` |
| `sub_1C1B2` | 恢復陣營**（`sub_1C1B2`）：付錢之後只做一件事 —— | `docs/formats/08-combat.md:823` |
| `sub_1C1B4` | `2CAST1 sub_1C1B4`（inc，上限 `0xFE`）、`sub_1C8A0`（+`0x14`，上限 `0xEB`） | `docs/formats/09-spells.md:89`, `docs/formats/09-spells.md:137`, `docs/formats/09-spells.md:235` 等 4 處 |
| `sub_1C1D2` | `_2play_e14`）與巫師系定位術（`2CAST1 sub_1C1D2` 經 thunk `0x172B2`）。 | `docs/formats/04-graphics.md:368`, `docs/formats/09-spells.md:90`, `docs/polish-spec.md:102` |
| `sub_1C1EA` | `2CAST1 sub_1C8C8`（水行術設值）、`2TEMPLE sub_1C1EA`（神殿祝福一次設滿整段）、 | `docs/formats/08-combat.md:1079`, `docs/formats/09-spells.md:92`, `docs/formats/09-spells.md:93` 等 8 處 |
| `sub_1C22C` | `sub_1C22C` | `docs/formats/09-spells.md:94` |
| `sub_1C23E` | `sub_1C23E` 只在室內有效（`cmp ds:039D, 1 / je 失敗`，而它接下來查的 | `docs/formats/09-spells.md:95`, `docs/formats/09-spells.md:350` |
| `sub_1C2B4` | 價錢 0 就是「這一項不必做」** —— 原版拿 0 當旗標（`sub_1C2B4` 開頭 | `docs/formats/08-combat.md:857` |
| `sub_1C320` | `2CAST1 sub_1C320`／`sub_1C550`（inc，上限 `0xFF`） | `docs/formats/09-spells.md:96`, `docs/formats/09-spells.md:236`, `docs/research/water-traversal-oracle.md:153` |
| `sub_1C322` | 等級：**已證實**（每項都有字串或表格佐證），`sub_1C322` 那九個位元組的 | `docs/re/01-boot-and-display-mode.md:96`, `docs/re/01-boot-and-display-mode.md:103` |
| `sub_1C326` | 付錢**（`sub_1C326`）：拿記錄 `+102` 的 32 位元金幣與價格比大小，夠就減掉。 | `docs/formats/08-combat.md:834` |
| `sub_1C338` | `sub_1C390` 對職業 5（賊）與 6（忍者）多呼叫一次 `sub_1C338`， | `docs/formats/02-data-files.md:992`, `docs/formats/08-combat.md:729` |
| `sub_1C340` | `sub_1C340` 讀 `1`–`2`：`1` 把現在的位置記進 `ds:03E8`（地圖）與 | `docs/formats/09-spells.md:97`, `docs/formats/09-spells.md:336`, `docs/research/spell-interaction-oracle.md:100` |
| `sub_1C376` | `sub_1C376` | `docs/re/01-boot-and-display-mode.md:95` |
| `sub_1C390` | < 51 → sub_1C41E 抽傷害、sub_1C390 施加到隊伍、ds:0430 = 3、重繪 | `docs/formats/02-data-files.md:992`, `docs/formats/08-combat.md:729`, `docs/formats/08-combat.md:736` 等 5 處 |
| `sub_1C3A0` | `sub_1C4A2` 與法師公會的 `sub_1C3A0` **逐行對稱**，差別只有兩處：讀哪一組表， | `docs/formats/08-combat.md:862`, `docs/formats/08-combat.md:891` |
| `sub_1C3D6` | `sub_1C3D6` | `docs/formats/09-spells.md:99`, `docs/formats/09-spells.md:488` |
| `sub_1C3EE` | `sub_1C3EE` 讀一個 `A`–`E` 的字母與一個 `1`–`4` 的數字， | `docs/formats/09-spells.md:100`, `docs/formats/09-spells.md:322`, `docs/research/spell-interaction-oracle.md:101` |
| `sub_1C3F0` | 為 1 才開得了門；否則印 `sub_1C3F0(0x10)` 的第 16 號訊息。 | `docs/re/02-2caves-special-events.md:66` |
| `sub_1C412` | 程式裡沒有這件事。整支法術 `sub_1C412`（`2CAST2`）只有兩條指令碰 | `docs/formats/08-combat.md:501`, `docs/formats/09-spells.md:101` |
| `sub_1C41E` | < 51 → sub_1C41E 抽傷害、sub_1C390 施加到隊伍、ds:0430 = 3、重繪 | `docs/research/door-state-oracle.md:48`, `docs/research/door-state-oracle.md:61` |
| `sub_1C42E` | `sub_1C42E` | `docs/formats/09-spells.md:102`, `docs/formats/09-spells.md:489` |
| `sub_1C432` | `sub_1C432` | `docs/re/01-boot-and-display-mode.md:94` |
| `sub_1C46A` | `sub_1C46A` | `docs/formats/09-spells.md:103` |
| `sub_1C4A2` | `sub_1C4A2` 與法師公會的 `sub_1C3A0` **逐行對稱**，差別只有兩處：讀哪一組表， | `docs/formats/08-combat.md:839`, `docs/formats/08-combat.md:862` |
| `sub_1C4A6` | 傷害與地圖陷阱共用同一條公式**：`sub_1C4A6` 只做畫面閃爍與播報， | `docs/formats/08-combat.md:728` |
| `sub_1C4B4` | `sub_1C4B4` | `docs/formats/09-spells.md:105`, `docs/formats/09-spells.md:490` |
| `sub_1C4EE` | `sub_1C4EE` | `docs/formats/09-spells.md:106` |
| `sub_1C4FC` | `2CAST1 sub_1C1EA`／`sub_1C4FC`（設 `0xFF` 或累加，上限 `0xFA`） | `docs/formats/09-spells.md:104`, `docs/research/water-traversal-oracle.md:157` |
| `sub_1C524` | `sub_1C524` | `docs/formats/09-spells.md:107`, `docs/formats/09-spells.md:491` |
| `sub_1C538` | 事件腳本 `0x2a`／`sub_19B44`、`2MISC sub_1C538` | `docs/research/chest-trigger-oracle.md:57`, `docs/research/command-keys-oracle.md:88` |
| `sub_1C550` | `2CAST1 sub_1C320`／`sub_1C550`（inc，上限 `0xFF`） | `docs/formats/09-spells.md:108`, `docs/formats/09-spells.md:238`, `docs/research/water-traversal-oracle.md:153` |
| `sub_1C56C` | `sub_1C56C` | `docs/formats/09-spells.md:109`, `docs/re/01-boot-and-display-mode.md:97` |
| `sub_1C570` | `sub_1C570` | `docs/formats/09-spells.md:114`, `docs/formats/09-spells.md:239` |
| `sub_1C588` | `sub_1C588` | `docs/formats/09-spells.md:110` |
| `sub_1C590` | `sub_1C590`；IDA composite image 線性位址 `0x1C590`；level-2 overlay 檔案 offset `0xCD90`（`0x1C590 - 0xF800`）。共用提示 `s… | `docs/formats/09-spells.md:115`, `docs/formats/09-spells.md:271`, `docs/research/spell-interaction-oracle.md:67` 等 4 處 |
| `sub_1C5A6` | i=2 捐獻 sub_1C5A6 | `docs/formats/08-combat.md:841` |
| `sub_1C5A8` | `sub_1C5A8` | `docs/formats/09-spells.md:111`, `docs/formats/09-spells.md:508` |
| `sub_1C5B8` | i=1 恢復陣營 sub_1C5B8 | `docs/formats/08-combat.md:840` |
| `sub_1C5CA` | → 對 `+80` 的兩個 nibble 各呼叫一次 `sub_1C5CA` → `+80` 清成 0， | `docs/formats/02-data-files.md:636`, `docs/formats/08-combat.md:942`, `docs/formats/08-combat.md:947` |
| `sub_1C5E8` | `sub_1C5E8` | `docs/formats/09-spells.md:112` |
| `sub_1C616` | i=0 恢復狀態 sub_1C616 i=3,4,5 三條法術 sub_1C4A2(0..2) | `docs/formats/08-combat.md:839` |
| `sub_1C620` | `sub_1C620` | `docs/formats/09-spells.md:113`, `docs/formats/09-spells.md:492` |
| `sub_1C648` | 能量補充術**（`sub_1C648`）：`+64 + 槽位`（充能）加 `rand(1,6)`。 | `docs/formats/09-spells.md:119`, `docs/formats/09-spells.md:394` |
| `sub_1C64A` | `sub_1C64A` 加到角色 `+0x5C`（Gems）。bits 3–4 控制以怪物 ID（必要時右移 4 | `docs/formats/08-combat.md:759`, `docs/research/chest-trigger-oracle.md:33` |
| `sub_1C65A` | `sub_1C65A` | `docs/formats/09-spells.md:116` |
| `sub_1C68C` | 複製術**（`sub_1C68C`）：把選中的那件抄到第一個空槽， | `docs/formats/09-spells.md:122`, `docs/formats/09-spells.md:403` |
| `sub_1C690` | `sub_1C690` | `docs/formats/09-spells.md:117` |
| `sub_1C698` | 恢復狀態**（`sub_1C178`）：付錢之後呼叫 `sub_1C698`。它以基礎生命上限 | `docs/formats/08-combat.md:818` |
| `sub_1C6B0` | `sub_1C6B0` | `docs/formats/09-spells.md:118`, `docs/formats/09-spells.md:493` |
| `sub_1C6CC` | A–F。`sub_1C6CC` 進來時一次算好六格： | `docs/formats/08-combat.md:837` |
| `sub_1C6F8` | `sub_1C6F8` | `docs/formats/09-spells.md:120`, `docs/formats/09-spells.md:494` |
| `sub_1C722` | `sub_1C722` 往面向的方向走一格，**完全不查牆** —— 這是全遊戲唯一 | `docs/formats/09-spells.md:123`, `docs/formats/09-spells.md:345` |
| `sub_1C732` | `sub_1C732` | `docs/formats/09-spells.md:121`, `docs/formats/09-spells.md:495` |
| `sub_1C75C` | `sub_1C75C` | `docs/formats/09-spells.md:124`, `docs/formats/09-spells.md:467` |
| `sub_1C774` | 加強法力**（`sub_1C774`）：`+70 + 槽位`（屬性）的低六位加一， | `docs/formats/09-spells.md:132`, `docs/formats/09-spells.md:398` |
| `sub_1C7AA` | sub_1C7AA 選人；回 0x1B → 走 loc_1728E 返回 | `docs/research/door-state-oracle.md:56` |
| `sub_1C7B2` | `sub_1C7B2` | `docs/formats/09-spells.md:125`, `docs/formats/09-spells.md:496` |
| `sub_1C7DA` | `ds:03B4` —— 自然之門（`sub_1C7DA`）把 `ds:03B4` 寫死在程式碼裡， | `docs/formats/07-event-script.md:800`, `docs/formats/09-spells.md:142`, `docs/formats/09-spells.md:300` |
| `sub_1C7EE` | `sub_1C7EE` | `docs/formats/09-spells.md:126`, `docs/formats/09-spells.md:497` |
| `sub_1C81A` | `sub_1C81A` | `docs/formats/09-spells.md:127`, `docs/formats/09-spells.md:468` |
| `sub_1C824` | 找陷阱做完一定接著開箱** —— `sub_1C8AE` 最後呼叫 `sub_1C824(0FFh)`， | `docs/formats/08-combat.md:704`, `docs/formats/08-combat.md:714`, `docs/formats/08-combat.md:715` |
| `sub_1C850` | `sub_1C850` | `docs/formats/09-spells.md:128` |
| `sub_1C86C` | `sub_1C86C` | `docs/formats/09-spells.md:129`, `docs/formats/09-spells.md:509` |
| `sub_1C87C` | `sub_1C87C` | `docs/formats/09-spells.md:148`, `docs/formats/09-spells.md:229` |
| `sub_1C89E` | `sub_1C89E` | `docs/formats/09-spells.md:130`, `docs/formats/09-spells.md:498` |
| `sub_1C8A0` | `2CAST1 sub_1C1B4`（inc，上限 `0xFE`）、`sub_1C8A0`（+`0x14`，上限 `0xEB`） | `docs/formats/09-spells.md:151`, `docs/research/water-traversal-oracle.md:151` |
| `sub_1C8AA` | 指標會被當引數傳給別的函式（印名字的 `sub_11726`、`sub_1C8AA`、 | `docs/formats/02-data-files.md:60` |
| `sub_1C8AE` | 找陷阱做完一定接著開箱** —— `sub_1C8AE` 最後呼叫 `sub_1C824(0FFh)`， | `docs/formats/08-combat.md:704`, `docs/formats/08-combat.md:714` |
| `sub_1C8C8` | 法術引擎編號 19（水行術）→ `sub_1C8C8`（`2CAST1` overlay）→ `ds:03D9 = 1`。 | `docs/formats/09-spells.md:152`, `docs/formats/09-spells.md:230`, `docs/research/water-traversal-oracle.md:53` 等 6 處 |
| `sub_1C8CA` | `sub_1C8CA` | `docs/formats/09-spells.md:131`, `docs/formats/09-spells.md:469` |
| `sub_1C8E0` | `2CAST1 sub_1CA00`／`sub_1CA10`／`sub_1C8E0`／`sub_1C984`，一律設 1 | `docs/formats/09-spells.md:154`, `docs/formats/09-spells.md:231`, `docs/research/water-traversal-oracle.md:156` |
| `sub_1C8F0` | 回復陣營**（`sub_1C8F0`）把 `+13` 抄到 `+106`。所以 | `docs/formats/09-spells.md:156`, `docs/formats/09-spells.md:421` |
| `sub_1C900` | `sub_1C900` | `docs/formats/09-spells.md:133` |
| `sub_1C91A` | 偵測魔法（`sub_1C91A`）不擲骰、沒有風險，只數 `ds:6953`／`ds:6956` | `docs/formats/08-combat.md:718` |
| `sub_1C928` | `sub_1C928` | `docs/formats/09-spells.md:135` |
| `sub_1C92A` | 野外圖。傳送到地面（`sub_1C92A`）就是讀這兩格，`+22` 為 0 時整條 | `docs/formats/02-data-files.md:1015`, `docs/formats/09-spells.md:157`, `docs/formats/09-spells.md:677` |
| `sub_1C944` | `sub_1C944` | `docs/formats/09-spells.md:139` |
| `sub_1C984` | `2CAST1 sub_1CA00`／`sub_1CA10`／`sub_1C8E0`／`sub_1C984`，一律設 1 | `docs/formats/09-spells.md:164`, `docs/formats/09-spells.md:232`, `docs/research/water-traversal-oracle.md:156` |
| `sub_1C994` | 回春術（`sub_1C994`）也走加齡程序，但方向由擲骰決定： | `docs/formats/09-spells.md:165`, `docs/formats/09-spells.md:376` |
| `sub_1CA00` | `2CAST1 sub_1CA00`／`sub_1CA10`／`sub_1C8E0`／`sub_1C984`，一律設 1 | `docs/formats/02-data-files.md:987`, `docs/formats/08-combat.md:689`, `docs/formats/09-spells.md:168` 等 5 處 |
| `sub_1CA10` | `2CAST1 sub_1CA00`／`sub_1CA10`／`sub_1C8E0`／`sub_1C984`，一律設 1 | `docs/formats/09-spells.md:174`, `docs/formats/09-spells.md:234`, `docs/research/water-traversal-oracle.md:156` |
| `sub_1CA20` | 先呼叫 `sub_1D046`；之後由 `loc_16EC2` 收 `1`–`5`，`0x1B` 取消；確認後才寫 `ds:0392` 並把座標設 `0xFF` 交 root 取 `ATTRIB +14`。**已證實（… | `docs/formats/09-spells.md:176`, `docs/formats/09-spells.md:275`, `docs/research/spell-interaction-oracle.md:68` 等 4 處 |
| `sub_1CA40` | 勇氣術（`2CAST2` 的 `sub_1CA40`）整支只做一件事：選一名隊員， | `docs/formats/08-combat.md:527`, `docs/formats/09-spells.md:141` |
| `sub_1CA6E` | `sub_1CA6E` | `docs/formats/09-spells.md:143`, `docs/formats/09-spells.md:460` |
| `sub_1CAA4` | 復活（`sub_1CAA4`）只對 `+38 >= 80h`（石化與死亡那一類）有效， | `docs/formats/09-spells.md:179`, `docs/formats/09-spells.md:364` |
| `sub_1CAAE` | `sub_1CAAE` | `docs/formats/09-spells.md:145` |
| `sub_1CAEC` | `sub_1CAEC` | `docs/formats/09-spells.md:146` |
| `sub_1CB10` | 去咒術**（`sub_1CB10`）：只在 `+64 + 槽位` 等於 `0xFF` 時動手， | `docs/formats/09-spells.md:180`, `docs/formats/09-spells.md:407` |
| `sub_1CB12` | `sub_1CB12` | `docs/formats/09-spells.md:147`, `docs/formats/09-spells.md:507` |
| `sub_1CB48` | 選物品走 `sub_1CB48`（回傳 `0x1B` 表示取消）。 | `docs/formats/09-spells.md:392` |
| `sub_1CB52` | `sub_1CB52` | `docs/formats/09-spells.md:150` |
| `sub_1CB8A` | `sub_1CB8A` | `docs/formats/09-spells.md:153`, `docs/formats/09-spells.md:461` |
| `sub_1CBD8` | `sub_1CBD8` | `docs/formats/09-spells.md:158` |
| `sub_1CBEC` | （`sub_1CBEC`）。第 0 項是巫師系第 1 條、第 49 項是牧師系第 2 條 —— | `docs/formats/09-spells.md:26`, `docs/formats/09-spells.md:85`, `docs/formats/09-spells.md:134` 等 4 處 |
| `sub_1CBF8` | `sub_1CBF8` | `docs/formats/09-spells.md:159` |
| `sub_1CC34` | `sub_1CC34` | `docs/formats/09-spells.md:160`, `docs/formats/09-spells.md:462` |
| `sub_1CC3A` | `2CAST1 sub_1CC3A`／`sub_1CCB4` | `docs/formats/09-spells.md:98`, `docs/formats/09-spells.md:237`, `docs/research/water-traversal-oracle.md:152` |
| `sub_1CC5C` | `sub_1CC5C` → `sub_1CE46(8)` | `docs/formats/09-spells.md:136`, `docs/formats/09-spells.md:194` |
| `sub_1CC64` | `sub_1CC64` 的順序有個小瑕疵，引擎照抄： | `docs/formats/09-spells.md:161`, `docs/formats/09-spells.md:282` |
| `sub_1CC68` | `sub_1CC68` | `docs/formats/09-spells.md:138` |
| `sub_1CCA8` | `sub_1CCA8` → `sub_1CE46(0Fh)` | `docs/formats/09-spells.md:140`, `docs/formats/09-spells.md:195` |
| `sub_1CCB4` | `2CAST1 sub_1CC3A`／`sub_1CCB4` | `docs/formats/09-spells.md:144`, `docs/formats/09-spells.md:228`, `docs/research/water-traversal-oracle.md:152` |
| `sub_1CCD6` | `sub_1CCD6` | `docs/formats/09-spells.md:149`, `docs/formats/09-spells.md:196` |
| `sub_1CCDC` | `sub_1CCDC` | `docs/formats/09-spells.md:162` |
| `sub_1CD04` | `sub_1CD04` | `docs/formats/09-spells.md:167` |
| `sub_1CD16` | `sub_1CD16` | `docs/formats/09-spells.md:155`, `docs/formats/09-spells.md:197` |
| `sub_1CD40` | `sub_1CD40` | `docs/formats/09-spells.md:169` |
| `sub_1CD56` | `sub_1CD56` | `docs/formats/09-spells.md:163`, `docs/formats/09-spells.md:198` |
| `sub_1CD7C` | `sub_1CD7C` | `docs/formats/09-spells.md:170`, `docs/formats/09-spells.md:463` |
| `sub_1CD8A` | 它沒有在該段直接遞減 `ds:03D9`。配合 `sub_1CD8A` 將 `03D5`–`03E1` 一起清除， | `docs/formats/08-combat.md:1080`, `docs/research/water-traversal-oracle.md:72`, `docs/research/water-traversal-oracle.md:133` 等 5 處 |
| `sub_1CD96` | `sub_1CD96` | `docs/formats/09-spells.md:166`, `docs/formats/09-spells.md:199` |
| `sub_1CDB0` | `sub_1CDB0`／`sub_1D094`／`sub_1D252` | `docs/re/02-2caves-special-events.md:119` |
| `sub_1CDBA` | `sub_1CDBA` | `docs/formats/09-spells.md:171`, `docs/formats/09-spells.md:464` |
| `sub_1CDCA` | `sub_1CDCA` | `docs/formats/09-spells.md:172`, `docs/formats/09-spells.md:200` |
| `sub_1CE40` | `sub_1CE40` | `docs/formats/09-spells.md:173` |
| `sub_1CE46` | `sub_1CCA8` → `sub_1CE46(0Fh)` | `docs/formats/09-spells.md:194`, `docs/formats/09-spells.md:195` |
| `sub_1CE7C` | `sub_1CE7C` | `docs/formats/09-spells.md:175`, `docs/formats/09-spells.md:510` |
| `sub_1CEB6` | `sub_1CEB6` | `docs/formats/09-spells.md:177` |
| `sub_1CED8` | `sub_1CED8`（戰鬥外），兩支形狀一樣： | `docs/formats/02-data-files.md:73` |
| `sub_1CEFA` | Enter（`0x0D`）結束提示；若當前格 `AttrNoMagic`，會回到 `sub_1CEFA`。這證明 | `docs/research/spell-interaction-oracle.md:72` |
| `sub_1CF1C` | `sub_1CF1C` | `docs/formats/09-spells.md:178` |
| `sub_1CF34` | 其餘（1..0x7F） → 走 sub_1BBAE／sub_1CF34 的八路 byte 效果 | `docs/formats/02-data-files.md:78` |
| `sub_1CF5C` | `sub_1CF5C` | `docs/formats/09-spells.md:188` |
| `sub_1CF8C` | 「選一名隊員」那批法術在原版走 `sub_1CF8C` 選單（回傳 `0x1B` | `docs/formats/09-spells.md:386`, `docs/research/spell-interaction-oracle.md:104` |
| `sub_1D046` | `sub_1C590`；IDA composite image 線性位址 `0x1C590`；level-2 overlay 檔案 offset `0xCD90`（`0x1C590 - 0xF800`）。共用提示 `s… | `docs/formats/09-spells.md:187`, `docs/research/spell-interaction-oracle.md:19`, `docs/research/spell-interaction-oracle.md:67` 等 9 處 |
| `sub_1D094` | `sub_1CDB0`／`sub_1D094`／`sub_1D252` | `docs/re/02-2caves-special-events.md:119` |
| `sub_1D13E` | `sub_1D13E` 與 `sub_1D170` 只是印字串（`ds:31CA`／`ds:31D3`）再暫停， | `docs/formats/09-spells.md:619` |
| `sub_1D170` | `sub_1D13E` 與 `sub_1D170` 只是印字串（`ds:31CA`／`ds:31D3`）再暫停， | `docs/formats/09-spells.md:619` |
| `sub_1D19C` | 進貨的洗牌（`sub_1D19C`）產生 0–25 的排列，與這 6 件的貨架 | `docs/formats/02-data-files.md:1253` |
| `sub_1D1A6` | `2CAST2 sub_1D1A6` | `docs/research/spell-interaction-oracle.md:105` |
| `sub_1D23A` | 表示沒選到就整支跳過），或 `sub_1D23A` 確認場上有怪；擲傷害寫進 | `docs/formats/09-spells.md:434`, `docs/research/spell-interaction-oracle.md:106` |
| `sub_1D252` | `sub_1CDB0`／`sub_1D094`／`sub_1D252` | `docs/re/02-2caves-special-events.md:119` |
| `sub_1D2AE` | 攻擊 handler 的形狀一致：先呼叫 `sub_1D2AE` 選目標（回傳 `0x1B` | `docs/formats/09-spells.md:433` |
| `sub_1D3C4` | `sub_1D3C4` 有 `monsters.dat`、`Hoardall (A-D)?`、`Slayer (A-D)?`、`Then begone, knave!` | `docs/re/02-2caves-special-events.md:31`, `docs/re/02-2caves-special-events.md:117` |
| `sub_29F7E` | thunk 6** —— `sub_29F7E`，引數 `(src, dest, len)`，內容是設 VDP 的 | `docs/research/02-other-platforms.md:829` |
| `sub_3117E` | → … → `sub_3117E(檔名, 緩衝, 偏移, 長度)`。**A4 = 0x3D93E**，定法是 | `docs/research/02-other-platforms.md:284`, `docs/research/02-other-platforms.md:306`, `docs/research/02-other-platforms.md:314` |
| `sub_312A8` | → sub_312A8 → sub_34314(C open) → sub_354BE → dos.Open | `docs/research/02-other-platforms.md:307` |
| `sub_31302` | → sub_31302 → … → sub_346FE → sub_354D4 → dos.Read | `docs/research/02-other-platforms.md:308` |
| `sub_31F9E` | `sub_1173C` 對每個場景載四個檔，走 `sub_11622` → `sub_31F9E` → | `docs/research/02-other-platforms.md:291` |
| `sub_32C24` | `sub_32C24` 命令 16 → `sub_33322`。 | `docs/research/02-other-platforms.md:292` |
| `sub_33322` | `LoadRGB4` 的 thunk 往回找到 `sub_33ED2` → `A4−0x4F0` → `sub_33322`。 | `docs/research/02-other-platforms.md:193`, `docs/research/02-other-platforms.md:218`, `docs/research/02-other-platforms.md:235` 等 5 處 |
| `sub_33A18` | 填進去，`sub_33A18` 接著**只把 `0x12`–`0x1F` 從畫面現有的色表抄回來** | `docs/research/02-other-platforms.md:217`, `docs/research/02-other-platforms.md:236` |
| `sub_33ED2` | `LoadRGB4` 的 thunk 往回找到 `sub_33ED2` → `A4−0x4F0` → `sub_33322`。 | `docs/research/02-other-platforms.md:233`, `docs/research/02-other-platforms.md:251` |
| `sub_33EF2` | 解碼常式是 `mm2` 的 **`sub_33EF2`**；載入器 `sub_33322`（命令 16）。 | `docs/research/02-other-platforms.md:193` |
| `sub_34314` | → sub_312A8 → sub_34314(C open) → sub_354BE → dos.Open | `docs/research/02-other-platforms.md:307` |
| `sub_346FE` | 錨點鏈：`dos.Read` 的 stub（`jmp -$2A(a6)`）→ `sub_354D4` → `sub_346FE` | `docs/research/02-other-platforms.md:283`, `docs/research/02-other-platforms.md:308`, `docs/research/02-other-platforms.md:310` |
| `sub_354BE` | → sub_312A8 → sub_34314(C open) → sub_354BE → dos.Open | `docs/research/02-other-platforms.md:307` |
| `sub_354D4` | 錨點鏈：`dos.Read` 的 stub（`jmp -$2A(a6)`）→ `sub_354D4` → `sub_346FE` | `docs/research/02-other-platforms.md:283`, `docs/research/02-other-platforms.md:308` |
| `sub_357A8` | `sub_357A8` 是 `LoadRGB4` 的包裝。**全程沒有 `udos.library` 的痕跡** —— | `docs/research/02-other-platforms.md:311` |
| `sub_37D32` | （`sub_37D32`），解密後也不是標準容器，程式自己切割。 | `docs/research/02-other-platforms.md:256` |
| `sub_AF136` | `sub_AF136` 是 vblank 的音樂更新（遞增 `$A000E6`、每 16 次呼叫 `sub_AF378`）， | `docs/research/md-music-driver.md:53` |
| `sub_AF1C2` | `sub_AF1C2` 不是選曲，是**驅動初始化** —— 偵測 PAL/NTSC（`($A10001)` bit 6， | `docs/research/md-music-driver.md:37`, `docs/research/md-music-driver.md:46`, `docs/research/md-music-driver.md:155` |
| `sub_AF23A` | 每首曲子的前 4 bytes 是它的結束指標**（`sub_AF23A` 的 `move.l -4(a0),-$37C4(a5)`）。 | `docs/research/md-music-driver.md:34`, `docs/research/md-music-driver.md:48`, `docs/research/md-music-driver.md:78` 等 5 處 |
| `sub_AF296` | 正對照通過**：同一支掃描器對 `sub_AF296` 與 `sub_AF2C0` 各掃出 4 筆， | `docs/research/md-music-driver.md:20`, `docs/research/md-music-driver.md:70` |
| `sub_AF2C0` | 正對照通過**：同一支掃描器對 `sub_AF296` 與 `sub_AF2C0` 各掃出 4 筆， | `docs/research/md-music-driver.md:19`, `docs/research/md-music-driver.md:70` |
| `sub_AF340` | 音樂關閉時播的空曲**，`sub_AF340` 回報驅動閒置時也會重新送它。 | `docs/research/md-music-driver.md:97` |
| `sub_AF378` | `sub_AF136` 是 vblank 的音樂更新（遞增 `$A000E6`、每 16 次呼叫 `sub_AF378`）， | `docs/research/md-music-driver.md:53` |
| `sub_AF3E4` | AF252 bsr.w sub_AF3E4 ; 把 0x400 bytes 搬進 Z80 RAM $A01600 | `docs/research/md-music-driver.md:39` |
| `sub_BFC28` | `sub_BFC28`：ROM `0`–`0xBFC28` 的 32-bit 長字加總要等於 `0x3ACE1FBA`，跳過 `0x18C`（標頭 checksum 欄位）。不符就進死迴圈 | `docs/research/md-music-driver.md:194`, `docs/research/md-music-driver.md:229`, `docs/research/md-re-status.md:36` 等 5 處 |
| `sub_1C7162` | （`sub_1C7162(i)` 取第 i 隻的指標，迴圈內再擲一次）。 | `docs/formats/09-spells.md:472` |

## 只出現在程式碼註解

這些是 remake 的註解引用了原版函式，但 `docs/` 裡沒有對應筆記。
要動它們之前，先把該函式的證據補進 `docs/`。

| 函式 | 摘要 | 出處 |
|---|---|---|
| `sub_E3E` | 的動畫組，`sub_E3E` 那支查表貼圖畫的是地面條不是火焰。少的是素材 | `internal/assets/msx/scene.go:77` |
| `sub_7726` | 累加的位置（`記錄+31` 交給 `sub_7726`）還沒追出來 —— 所以這裡不動 `AC`， | `internal/game/gear.go:16` |
| `sub_15188` | OpPayGold 與 OpPayGems 是「全隊湊錢」（`sub_1A01E` → `sub_15188` | `internal/game/world.go:750` |
| `sub_15262` | 與 `sub_1A04C` → `sub_15262`，各長 3 個位元組）。 | `internal/game/world.go:751` |
| `sub_157E0` | 原版 `sub_157E0` 以 ds:5214 的十個指標讀取（音高索引、時值索引）對， | `cmd/mm2music/main.go:3` |
| `sub_1719E` | count = 4 + lv // 手冊「4 個怪物＋1 個怪物／等級」，原版走 sub_1719E | `internal/game/cast.go:437`, `internal/game/cast.go:1332` |
| `sub_1774A` | 原版用 `sub_1774A(ds:583E, 10)` 問「這個人有沒有第 10 項技能」。 | `internal/game/shop.go:34` |
| `sub_17EAF` | （`sub_17EAF` 的 `cmp ds:59C8, 0x80` 先擋掉）。 | `internal/game/session.go:403` |
| `sub_1C1BC` | 原版的兩個索引都可以按 Esc 取消（`sub_1C1BC` 回 `0x1B`）， | `internal/ui/session.go:2111` |
| `sub_1CEEE` | `2SMITH` 寫 `0x83`、`2MISC sub_1CEEE` 寫 3），開戰時 `0x1A344` | `internal/game/session.go:520` |
| `sub_1CFDE` | （`sub_1CFDE`／`1CFF6`／`1D00E`／`1D026`／`1D03E`／`1D056`）。 | `internal/game/equip.go:110` |
| `sub_1D06E` | 「任何近戰武器」是 `sub_1D06E`，它把單手與雙手兩個判斷式加起來。 | `internal/game/equip.go:156` |

