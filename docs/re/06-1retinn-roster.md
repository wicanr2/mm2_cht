# 旅店與名冊：`1RETINN`

輸入檔：`MM2.EXE` ＋ `1RETINN.OVL`（SHA-256 見 [`inventory.md`](../inventory.md)）。
IDA 位址取自 `workplace/ida/out/1RETINN.asm`，函式邊界由
`tools/ida_funclist.py` 對 `1RETINN.img.i64` 取得。

`1RETINN` 是**旅店**，也是**名冊與隊伍編組畫面**，還是**全滅之後的回收站** ——
三件事同一個 overlay，因為它們共用同一張「二十四個角色 ＋ 二十四個雇傭兵」的清單。

| 進入點 | 位址 | 大小 | 誰呼叫 | 內容 |
|---|---|---|---|---|
| `_1retinn_e00` | `0x1C130` | 185 | `0e 01`（thunk `0x173D2`）| 旅店招呼語與登記詢問 |
| `_1retinn_e01` | `0x1C1EA` | 188 | `_1retinn_e00`、`_1retinn_e04`、thunk `0x173DE` | 編組完成後離開旅店 |
| `_1retinn_e02` | `0x1C2C0` | 496 | `_1retinn_e03` | 列出二十四格清單 |
| `_1retinn_e03` | `0x1C5C0` | 1408 | `_1retinn_e01` | 名冊與隊伍編組的主迴圈 |
| `_1retinn_e04` | `0x1CB40` | 194 | 外部 | **全滅畫面** |
| `sub_1C2A6` | `0x1C2A6` | 25 | `_1retinn_e03` | 印一行 `('ESC' to exit to DOS)` |
| `sub_1C4B0` | `0x1C4B0` | 272 | `_1retinn_e03` | 加入／移出隊伍，含人數上限 |

`_1retinn_e01` 就是結局控制室通關後跳的那一支（`ds:0395 == 2` →
thunk `0x173DE` → `1RETINN +C1EA`，見 [`05`](05-2smith-control-room.md) §1）。

---

## 1. 名冊的版面

名冊基底 `ds:7E20`，一筆 `0x82` ＝ **130 bytes**，與 `ROSTER.DAT` 的記錄大小相同
（[`../formats/02`](../formats/02-data-files.md) §5）。程式碼裡看到的 `ds:7E2B`、
`ds:7E2F`、`ds:5D29`、`ds:7DA9` 都是編譯器把常數索引摺進基底之後的結果：

| 出現的位址 | 還原 | 欄位 |
|---|---|---|
| `ds:7E20` | 記錄 `+0` | 名字 |
| `ds:7E2B` | 記錄 `+11` | **寄放在哪座城**（`& 0x7F`，1 起算）|
| `ds:7E2F` | 記錄 `+15` | 職業 |
| `ds:5D29` | `0x7E2B − 'A' × 130` | 同 `+11`，索引是按鍵 `'A'`–`'X'` |
| `ds:7DA9` | `0x7E2B − 1 × 130` | 同 `+11`，索引是 Ctrl 碼 1–24 |

**`+11` 是這個 overlay 的軸心**：清單上只有「寄放在目前這座城」的角色選得動
（`_1retinn_e03` 的兩處 `cmp ax, cx`，`cx = ds:0392 + 1`），登記入住時整隊的
`+11` 被改寫成目前城鎮編號 ＋ 1（`_1retinn_e00` 的 `loc_1C1BD` 迴圈），
全滅之後也是靠它回到最後投宿的那一家。

## 2. `_1retinn_e00`：登記入住

	ds:0430 = 6                              ; 畫面模式
	印 ds:20BA[城 × 4 + i] 四行，第 0x13–0x16 列
	loc_173AE()                              ; 收 Y／N
	ds:042F == 0 → loc_172D6()               ; 答 N，回地圖
	否則：
	    loc_1704E(-1, 0, 0)
	    每個隊員（ds:0416[i]）的記錄 +11 ← ds:0392 + 1
	    ds:03D4 = ds:0392
	    loc_16E3E()
	    _1retinn_e01()

招呼語每座城四行，五座城共二十行，指標表在 `ds:20BA`：

| 城 | 開頭 |
|---|---|
| 0 Middlegate | `A jolly old innkeeper waves his quill…` |
| 1 Atlantium | `The well-groomed whiskers of the old concierge…` |
| 2 Tundara | `Ordigon, the elderly innkeeper…` |
| 3 Vulcania | `The aging host of the sleezy inn…` |
| 4 Sansobar | `The proprietor blows a pile of dust…` |

## 3. `_1retinn_e03`：編組畫面

畫面上的字全部在 DGROUP，逐條對得上操作：

| 位址 | 內容 | 位置 |
|---|---|---|
| `ds:2138` | `'A' - 'X' to View` | (0x0C, 0x12) |
| `ds:214A` | `(Ctrl) 'A' - 'X' to Add/Remove` | (5, 0x13) |
| `ds:2169` | `Other Towns` | (2, 1) |
| `ds:2175` | `'1' - '5'` | (2, 2) |
| `ds:2180` | `PARTY` | (0x1F, 1) |
| `ds:2186` | `C=  / H=` | (0x1D, 2) |
| `ds:218F` | `'Z' to exit` | (0x1B, 0x15)，隊伍非空時才印 |
| `ds:219B` | `*** Party is Full ***` | (0x0A, 4)，總數滿 8 時才印 |
| `ds:21B1` ＋ `ds:05F4[頁]` | `'Space' for ` ＋ `Hirelings `／`Characters` | (2, 0x15) |
| `ds:21BE` | `'V' View spell book` | 檢視角色時的內圈 |
| `ds:21D2` | ` Exit to DOS (Y/N)?  ` | Esc 的確認 |

標題是 `(N-城鎮名)`，置中的算法是 `18 − strlen ÷ 2`（`repne scasb` 量長度）。
城鎮名指標表 `ds:043C`：Middlegate／Atlantium／Tundara／Vulcania／Sansobar。

### 兩個計數器

`ds:155BE` 與 `ds:155BF` 每一輪重算（`loc_1C692` 起的迴圈掃 `ds:0416`）：
索引落在 0–23 算**一般角色**、其餘算**雇傭兵**，`0xFFFF` 是空位。
畫面上就是 `C=n / H=n` 那一行。

### 按鍵

| 鍵 | 動作 |
|---|---|
| `'A'`–`'X'` | 檢視該格的角色（`loc_16E4A`），內圈可按 `V` 看法術書（`loc_17636`），Esc 回來 |
| Ctrl+`'A'`–`'X'`（碼 1–24）| `sub_1C4B0`：加入或移出隊伍 |
| `'1'`–`'5'` | 換城（`ds:0392 = 鍵 − '1'`），**`ds:0416` 八格全部填 `0xFFFF`、`ds:0426 = 0`** —— 換城等於解散隊伍 |
| 空白 | 在名冊與雇傭兵兩頁之間切換（`var_A` 在 0 與 0x18 之間跳）|
| `'Z'` | 隊伍非空時離開，回傳值 1 |
| Esc | 問 `Exit to DOS (Y/N)?`；`ds:039D == 2` 時整條跳過 |

**兩層過濾器**，兩個入口各寫一次（按鍵版在 `loc_1C88B`、Ctrl 版在 `loc_1C9EC`）：

1. 記錄 `+11` 必須等於 `ds:0392 + 1`，否則整個按鍵無效。
2. 在雇傭兵頁（`var_A == 0x18`）另外要 `ds:03F6[索引] != 0` ——
   就是那二十四個夥伴解鎖旗標（[`03`](03-character-flags.md) §3）。

### 離開時寫的四個全域

	ds:03D4 = ds:0392                ; 最後投宿的城
	ds:0393 = ds:21E8[城]            ; X
	ds:0394 = ds:21EE[城]            ; Y
	ds:03CF = ds:21F4[城]            ; 朝向（ASCII）

三張表各六個位元組，第六格是零：

| 城 | X | Y | 朝向 |
|---|---|---|---|
| 0 Middlegate | 7 | 3 | `'N'`（78）|
| 1 Atlantium | 9 | 13 | `'N'` |
| 2 Tundara | 7 | 11 | `'E'`（69）|
| 3 Vulcania | 7 | 0 | `'N'` |
| 4 Sansobar | 3 | 10 | `'W'`（87）|

**Middlegate 的 (7, 3) 面北是靜態證據**，與 `docs/playtest/01` §7 用 DOSBox
記憶體 dump 量到的起始狀態逐項相同 —— 那條原本只有動態證據，現在兩面都有。
朝向存的是 ASCII 字母，與牆位元遮罩用的 `'N'`／`'S'`／`'E'`／`'W'` 是同一套
（[`../formats/06`](../formats/06-map.md) §4）。

## 4. `_1retinn_e02`：清單怎麼排

二十四格，**兩欄各十二列**：列 ＝ `索引 mod 12 + 5`，欄 ＝ `(索引 ≤ 11 ? 0 : 1) × 20 + 1`。
每一格印「標記字元、`'A'+索引`、`-`、空白、名字、職業縮寫」。

- 標記字元預設空白；已在隊伍中（`loc_1762A`）且 `+11` 對得上時改成 `0x17`。
- 職業縮寫查 `ds:0446[職業]`，每項三個字元：
  `Kni`／`Pal`／`Arc`／`Cle`／`Sor`／`Rob`／`Nin`／`Bar`。
  **`arg_2 == 0` 時只印第一個字元再接 `/` 與城鎮號**，非零時印第二、三個字元。
- 該格不可選時整格印 `ds:2129`（十四個空白）蓋掉。
- 標題兩種：`Characters` ＋ 十個 `0x05`（底線字模），或 `Hirelings ` ＋ 同樣的底線。

## 5. `sub_1C4B0`：加入與移出

	已在隊伍中（loc_1762A）→ loc_17642 移出，標記改回空白
	否則檢查上限：
	    名冊那一頁：ds:155BE < 6 且 ds:155BE + ds:155BF != 8
	    雇傭兵那一頁：ds:155BE + ds:155BF != 8 且 ds:03F6[索引] != 0
	通過 → ds:0416[ds:0426++] = 索引，標記改成 0x17

**一般角色最多 6 人，連同雇傭兵最多 8 人**。這兩個上限是分開判的：
六個一般角色湊滿之後還能再加兩名雇傭兵，反過來則不行。

## 6. `_1retinn_e04`：全滅

	ds:1040E++                                ; 全滅次數
	畫框 (7, 6)–(0x21, 0x11)
	印 ds:22A6 的十行，第 1–10 列第 2 欄
	loc_17096(7)；loc_16EB5+1(0x0D)           ; 等 Enter
	ds:0392 = ds:03D4                         ; 回到最後投宿的城
	loc_16E26()
	_1retinn_e01()

十行的內容是：

	     Death Strikes!

	Unfortunately, you were
	not successful in your
	     last endeavor.

	 To resume adventuring
	  at the inn in which
	    you last stayed
	      press ENTER

所以**全滅不是重來，是回到最後投宿的旅店重新編組** —— `ds:03D4` 由
`_1retinn_e00` 在登記入住時寫下，`_1retinn_e03` 離開時再寫一次。

## 7. 用到的 thunk

`1RETINN` 自己只有七支函式，其餘全部經 thunk 出去
（`tools/ovl_thunks.py` 反查）：

| thunk | 目標 | 用在哪 |
|---|---|---|
| `sub_16DD2` | root `0x10B0E` | 畫框（`_1retinn_e04` 的全滅框）|
| `sub_16E92` | root `0x1142A` | 切換顯示狀態，成對呼叫 |
| `sub_16F5E` | root `0x135A8` | Esc 確認前後各一次 |
| `loc_16E02` | root `0x13FC4` | Esc 答 Y：離開回 DOS |
| `loc_16E0E` | root `0x149E2` | 離開旅店前的收尾 |
| `loc_16E26` | root `0x1276C` | 全滅收尾 |
| `loc_16E3E` | root `0x12792` | 登記完成 |
| `loc_16E4A` | root `0x12A6A` | 檢視某個角色 |
| `loc_16F22` | root `0x100E8` | 轉大寫 |
| `loc_16F46` | root `0x15440` | `ds:039D == 2` 時取代提示行 |
| `loc_16F6A` | root `0x1165C` | 換顏色 |
| `loc_16FB2` | root `0x12DFE` | 讀一個鍵 |
| `loc_1711A` | root `0x1163E` | 反白開關（`*** Party is Full ***` 用）|
| `loc_1761E` | root `0x1423E` | 離開前寫回名冊 |
| `loc_1762A` | root `0x127E4` | 這個索引在不在隊伍裡 |
| `loc_17636` | root `0x14EA6` | 看法術書 |
| `loc_17642` | root `0x12852` | 從隊伍移除 |
| `loc_173AE` | 2PLAY `+946E` | 收 Y／N |
| `loc_174F2` | 2PLAY `+B0F6` | 進編組畫面前的準備 |
| `loc_172D6` | 2PLAY `+A580` | 答 N，回地圖 |

## 8. 未解

- `ds:20E2` 在 DGROUP 初值段是空字串，`_1retinn_e01` 卻把它交給
  `loc_16E79+1` 印。與 [`05`](05-2smith-control-room.md) §6 的 `ds:58B8`
  同一類形狀（印在填之前），但這一支沒有對應的填入端，語意未知。
- `ds:039D`（Esc 的兩處判斷都比 2）、`ds:0399`、`ds:039B`、`ds:1040E`
  的完整語意還沒追。
- `loc_16E02`（Esc 答 Y 走的那一支）與 `loc_16E26` 是 root 的常式，沒讀。

---

等級：**已證實**（七支逐條讀完；字串、城鎮名、招呼語、落點、職業縮寫
五張表都從 `MM2.EXE` 的 DGROUP 初值段直接讀出並與程式碼的索引算式對上）。
§8 各項標未解。
