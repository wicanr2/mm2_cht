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

空行（連續的 `0x1D`）看起來是訊息之間的分隔，但訊息的**索引方式**尚未確定 ——
程式怎麼指定「顯示第幾條」還沒追到。等級：**未知**。

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
