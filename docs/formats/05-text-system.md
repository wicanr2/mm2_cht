# 文字系統：長文字在 STR.DAT

遊戲的長文字（劇情、對話、商店選單、結局）不在事件檔裡，在 `STR.DAT`。

## 1. 解法

```
LZW 段 → 每個 byte + 0x1C → 純文字，0x1D 是換行
```

`+0x1C` 之後整份是可讀的英文，大小寫、標點、引號全部正確，
控制碼只剩一種（`0x1D`，出現 399 次），切出 400 行。

```python
raw = lzw.Segment(open("STR.DAT","rb").read(), 0)
text = bytes((b + 0x1C) & 0xFF for b in raw).decode("latin1")
lines = text.split("\x1d")
```

## 2. 驗證

原版走進 Middlegate 的神殿，畫面顯示：

```
A slim cleric in a cowled robe peers
at you and asks in a serene voice,
"May I aid you, travelers (y/n)?"
```

解出的內容**逐字相符**，連引號、逗號、`(y/n)` 與兩處斷行位置都一樣：

```
A slim cleric in a cowled robe peers⏎at you and asks in a serene voice,⏎"May I aid you, travelers (y/n)?"⏎
```

## 3. 內容

400 行涵蓋：

| 類別 | 例 |
|---|---|
| 酒館笑話 | `Did you hear about the orc who thought / kites were made from flypaper?` |
| 商店與神殿對話 | `The burly blacksmith Svendegard busily shapes a deadly sword in the forge.` |
| 選單 | `A) Weapons / B) Today's Specials / C) Armor / D) Misc Items / E) Sell Items` |
| 提示與線索 | `Meal A, then C1 2,10`、`Castle Xabran holds all clues` |
| 結局 | `Congratulations! Because of your superior intellect and diligence you have saved CRON…` |
| 謎題 | `We, the people of Terra, in order to form a more perfect union…`（密碼謎題的明文） |
| 片尾 | `New World Computing / P. O. Box 2068 / Van Nuys, California 91404` |

## 3.5 取用方式：區塊加順序游標，沒有逐條索引

原版根本不「指定第幾條」。root `0x16750` 一帶有兩支常式：

**載入**

```
起點 = ds:52F4[區塊編號]              ; 五個 word 的區塊表
把起點起的 0x960（2,400）bytes 逐位元組 +0x1C 解碼進 ds:A06E
解出 0x1D 的就寫成 0                  ; 分隔符當場變成 NUL
ds:52F2 = 0                           ; 游標歸零
```

**取下一條**

```
回傳 ds:A06E + ds:52F2
從游標往後掃到 0，游標指向下一條的開頭
```

所以定位方式是**（區塊，第幾次呼叫）**，不是編號。要顯示某一段對話，
程式先載入那個區塊，再連續呼叫「取下一條」。

`ds:52F4` 的五個位移是 **0、1596、3932、4742、6212**，每一個都正好落在
段落開頭（前一個位元組是分隔符），而且相鄰兩個的距離都小於 0x960 ——
一個區塊剛好一次載入吃得下。內容也分得開：

| 區塊 | 起點 | 開頭 |
|---|---|---|
| 0 | 0 | `Did you hear about the orc who thought` |
| 1 | 1596 | `A low mumble emerges from the middle` |
| 2 | 3932 | `    Thank you -` |
| 3 | 4742 | `Sheltem and his Elementals guard the` |
| 4 | 6212 | `Sages in multi-hued robes congregate` |

**每一「條」是一行顯示，不是一則訊息** —— 四百段裡最長的也只有 38 個
字元。一則訊息是連續數行，程式一行一行取。等級：**已證實**。

## 4. 對中文化的影響

素材分兩批：

| 來源 | 條數 | 內容 |
|---|---|---|
| `EVENTSI/O.DAT` | 1,308 | 設施名、短提示、按鍵詢問 |
| `STR.DAT` | 400 行 | 劇情、對話、選單、結局、謎題 |

兩批都是明文，逐條翻即可 —— 不需要重建原版的任何壓縮或編碼機制。
remake 的文字層吃翻譯後的完整句子。

原版的編碼要能原樣往返，方便與原版畫面對照驗證。

## 5. 密碼謎題要特別處理

結局前的謎題是把美國憲法序言做成密文，玩家要解出來才能過關。
它同時牽涉**文字內容**與**玩法**：翻成中文之後，原本「用英文字母頻率解密」
的解法就不成立了。這一項要單獨設計，不能當成普通字串翻掉。等級：**待設計**。
