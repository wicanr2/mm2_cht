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

## 代價，以及還沒做的事

三件事要先講清楚，才輪得到實作：

1. **那是另一份稿，不是 DOS 的缺頁。** 接上去之後 remake 顯示的是
   Mega Drive 版的文字，DOS 原版沒有這回事。這與專案的裁決順序
   （DOS 是唯一 oracle）直接衝突，所以只能當成**明示的選項**，
   不能預設開啟，也不能混進「還原」的說法裡。
2. **對齊還沒做。** 一個區塊裡有好幾段（一間鐵匠一段），段與段之間是
   **可列印的分隔位元組**（抽出來會看到 `g`／`m`／`p`／`{` 黏在句首），
   所以現在的切法會把分隔符留在字串開頭。要對到「哪一段配哪一間設施」，
   得先把那個分隔規則解出來 —— 這一步沒做，TSV 的 `idx` 只是流水號。
3. **翻譯量會增加。** 229 段長描述加上中句，粗估兩三百條要譯，
   而目前 `translations/` 是 2,703／2,703 剛好收斂的狀態。

**目前的結論**：素材面可行、內容面要裁決。工具與 TSV 已經在了，
真的要接的時候從第 2 點（分隔規則與設施對齊）開始。
