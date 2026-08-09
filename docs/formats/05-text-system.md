# 文字系統：長文字不是明文，是單字索引

這一份影響中文化的整體策略，動翻譯之前要先讀。

## 1. 發現的經過

原版走進 Middlegate 的神殿會顯示：

```
A slim cleric in a cowled robe peers
at you and asks in a serene voice,
"May I aid you, travelers (y/n)?"
```

**這句話不在 `EVENTSI.DAT` 裡。** 從事件檔抽出的 1,308 條字串（見
[`02-data-files.md`](02-data-files.md) §4）全文搜尋 `cowled robe`、
`May I aid you` 都是零命中。

它在 `STR.DAT` 的單字表裡，而且是拆開的：

| 單字 | 索引 |
|---|---|
| `SLIM` | 1451 |
| `CLERIC` | 1452 |
| `COWLED` | 1455 |
| `ROBE` | 1456 |
| `PEERS` | 1457 |
| `SERENE` | … |
| `AID` | … |
| `TRAVELERS` | … |

索引連續遞增，順序與句子裡的出現順序一致 —— 這句話是**照索引把單字串起來**的。

## 2. STR.DAT 的結構

```
LZW 段 → 每個 byte −4 → 用 0x00 與 0xFD 分隔 → 1,694 個單字
```

`0xFD` 與 `0x00` 都是分隔符。只用 `0x00` 切會得到 `THOUGHTýKITES` 這種黏在
一起的條目（`ý` = 0xFD），單字數也會少掉約 400 條。

## 3. 首字母帶標記位元

把索引 1100–1135 連起來讀：

```
DIRECTLY TOWARDS THE SUN … STRANGE MACHINE PULSATES #OLLISION IMMINENT
%NTER OVERRIDE CODE TO ABORT … ) AM 3HELTEM RULER OF THE PLANET 4ERRA
```

`#OLLISION`、`%NTER`、`3HELTEM`、`4ERRA`、`)`、`!` 讀起來像亂碼，但差值一致：

| 檔案裡 | 應該是 | 差 |
|---|---|---|
| `#` (0x23) | `C` (0x43) | 0x20 |
| `%` (0x25) | `E` (0x45) | 0x20 |
| `3` (0x33) | `S` (0x53) | 0x20 |
| `4` (0x34) | `T` (0x54) | 0x20 |
| `)` (0x29) | `I` (0x49) | 0x20 |
| `!` (0x21) | `A` (0x41) | 0x20 |

固定差 `0x20` = bit 5。**某些單字的首字母在檔案裡被清掉 bit 5**，
`COLLISION`、`ENTER`、`SHELTEM`、`TERRA`、`I`、`A` 都是句子的第一個字。

推定為排版標記（句首／需要大寫／前面要斷行之類）。等級：**強推論** ——
差值一致且落點都在句首，但確切語意要看使用端的程式碼才算數。

## 4. 對中文化的影響

**中文化的戰場比原先估計的大，而且不能照抄原版的做法。**

- 事件檔的 1,308 條是**設施名與短提示**，可以逐條翻。
- 劇情、對話、旁白這些長文字**不在那裡**，是靠單字索引在執行時組出來的。
- 英文可以這樣壓縮是因為單字之間有空格、重複率高；**中文沒有對應的分詞單位**，
  把中文拆成「單字表 + 索引」既不省空間也不可讀。

所以中文化要：

1. 解出索引層 —— 找出「誰引用 STR.DAT 的索引、以什麼編碼」，
   把原版的長文字**還原成完整句子**；
2. 以完整句子為單位翻譯，進 `translations/`；
3. remake 的文字層直接吃翻譯後的完整句子，**不重建單字索引機制**。

原版的單字表與索引仍然要能原樣往返，方便對照與驗證。

## 5. 未解

- **索引層在哪。** 事件段的腳本區（`0xFF` 分隔的變長序列，見
  [`02-data-files.md`](02-data-files.md) §4）是頭號候選：索引值超過 255，
  需要兩個 byte，而腳本區正是變長的。
- **bit 5 標記的確切語意。**
- 單字表裡的 `$ID`、`0x1F` 等記號。

## 6. 可重跑指令

```bash
python3 -c "
import sys, re; sys.path.insert(0,'tools')
from mm2lzw import unpack_segment
dec = bytes((b-4)&0xFF for b in unpack_segment(open('workplace/orig/MM2/STR.DAT','rb').read(),0)[1])
words = re.split(rb'[\x00\xfd]', dec)
print(len(words), [w.decode('latin1') for w in words[1451:1458]])
"
```
