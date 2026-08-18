# Mega Drive 版的英文文字：能不能拿來擴充 DOS 版的同一個場景

Mega Drive ROM 裡有 90 個英文區塊（`tools/mdassets.py` 抽出，落在
`workplace/gfx/md-all/text/*.txt`，不入版控）。這一份回答一個問題：
**它們與 DOS 版是同一份稿嗎？不是的部分，能不能放進 remake 的同一個場景？**

整理工具是 [`tools/mdtext.py`](../../tools/mdtext.py)，產物是一張 TSV
（`block / idx / chars / match / dos_key / text`），同樣不入版控 ——
它帶原版英文全文，與 `translations/strings.json` 同一類。

```
tools/mdtext.py workplace/gfx/md-all/text workplace/md-text.tsv
```

## 比對的分母要對

DOS 那一側**兩個來源缺一不可**：`translations/strings.json`（事件、
`STR.DAT`、EXE 內嵌、物品名、怪物名）加上 `data/*.json`（從原版表格萃出來
的名字：法術的 `Origin`、職業、種族、狀況、標籤）。

只取第一個的話，96 條法術名會整批被判成「Mega Drive 專有」—— 它們在 DOS
也有，只是住在資料表不住在字串區。**分母錯了結論就整個歪掉**：那一版的
重疊率算出來是 28%，補上資料表之後是 37%。

## 結果

1,670 條可比對的字串：

| 判定 | 條數 | 比例 |
|---|---|---|
| 逐字相同 | 511 | 30% |
| 去掉大小寫、標點與空白之後相同 | 116 | 7% |
| DOS 那側找不到 | 1,043 | 63% |

**三分之一是同一句，三分之二是 Mega Drive 自己的稿。** 找不到的那些不是
「DOS 漏轉錄」——`No room to equip!`、`Wrong alignment to equip!`、
`Already have a missile weapon!` 這一整組裝備錯誤訊息，DOS 整份文字裡一條
都沒有，那是移植時新寫的 UI 文案。

MD 專有的 1,043 條粗分：

| 類型 | 條數 | 是什麼 |
|---|---|---|
| 短詞（名字／選項）| 437 | 選單項、人名、地名 |
| 中句 | 352 | 提示與回應 |
| **長段落（80 字以上）** | **229** | **設施的氣氛描述** |
| 短 UI 訊息 | 23 | `…!` 那一類 |
| 工具鏈殘留 | 4 | `Copyright 1990`／`Symantec` —— 編譯器的字串，不是遊戲文字 |

## 可以擴充的是哪一塊

那 229 條長段落集中在十幾個區塊，內容是**進設施時的場景描述**，而 DOS
在同一個位置只有一個光禿禿的選單。舉例（只引片段）：

- 鐵匠（`080BEA`）：Mega Drive 給每一間鐵匠一個名字與一句描寫 ——
  `the burly blacksmith Svendegard`、`the famous blacksmith Morgan Drewnhald`、
  `A friendly smith wipes his hands on a worn, leather apron`。
- 酒館（`08166C`）：每一間有自己的女侍與場面 —— `Amber, a sultry waitress`、
  `Belinthra, the bawdy proprietress`、`Gabrielle, the gorgeous barmaid`、
  `Amidst bloodthirsty brawling stands luscious Lucindra`。
- 訓練所（`081D42`）：`A page rolls rusty armor in a barrel of sand…`、
  `Banners flutter loudly in the damp sea air…`。

**場景是對得上的**：同一批設施、同一批城鎮，DOS 有的它都有。所以「放進
DOS 版的同一個場景」在技術上成立 —— 一個設施一段描述，接在現有選單前面。

## 區塊的版面

每個區塊是 **`uint8 長度 + 內容`** 一路接下去，沒有終止符，字元是
Mac Roman（引號是 `0xD2`／`0xD3`）。九十個區塊全部照這個版面解得開 ——
先前看到的 `g`／`m`／`p`／`{` 黏在句首，那不是分隔符，是**長度位元組**
被當成了字。（`mdassets.py` 的區塊邊界會切掉最後一兩個位元組，
所以解析要容錯，不是版面有問題。）

## 段對到哪一座城

**index 就是城的編號**（0 中門格特、1 亞特蘭提姆、2 通達拉、3 瓦肯尼亞、
4 桑德索巴）。決定性的證據在訓練所那一塊：五段描述之後**緊接著五個
DOS 的設施名**，順序正是

```
Turkov's Training / Island Training / Enhancement Center /
Training Academy / Sheik Training Arena
```

—— 與 DOS 那五座城的訓練所逐項相同。四個獨立佐證：

| 段 | 內容 | 對上 |
|---|---|---|
| 鐵匠 [1] | `Morgan Drewnhald` | DOS 城 1 的 `Drewnhald Ironworks` |
| 旅店 [2] | `cozy, warm beds`、`Ordigon` | 城 2 是雪地城（`Tundaran Arms Inn`）|
| 神殿 [3] | `the smoke of the lava fire` | 城 3 是火山城瓦肯尼亞 |
| 訓練 [1] | `the damp sea air` | 城 1 是島城亞特蘭提姆 |

**一個例外**：酒館 [2] 是 `Belinthra`，而 DOS 的 `Belinthra's Bar` 在城 3。
Mega Drive 把她挪了一座城，還是酒館那一塊另有規則，**沒有查證**——
照 index 放，並在這裡記著。

## 接進 remake 的範圍

不是 229 段全接。**前五段都是長描述**的區塊只有七個，其中五個是設施
（旅店、鐵匠、酒館、神殿、訓練所），另外兩個是城鎮入口的描述。
所以實際要譯的是 **5 種設施 × 5 座城 ＝ 25 段**，不是兩三百條。

抽取工具 [`tools/mdflavor.py`](../../tools/mdflavor.py) 把那 25 段寫成
`workplace/md-flavor/flavor.json`（原文，不入版控），譯文在
`translations/md-flavor.json`（只有 key、原文雜湊與譯文）。remake 在
進設施時把它排在 `進入神殿。` 之前 —— 它是「你看到了什麼」，
那一句是之後的動作。

**開關在 `F2`，預設關。** 顯示另一個版本的文字是內容選擇，不是還原：
DOS 走進鐵匠鋪就只有一個選單，沒有這些句子。
