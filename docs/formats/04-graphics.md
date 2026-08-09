# .16 圖形格式

26 個 `.16` 檔全部是 LZW 段（見 [`03-lzw-compression.md`](03-lzw-compression.md)），
段頭宣告長度與實際解出長度全部相符。

實作：[`tools/mm216.py`](../../tools/mm216.py)，輸出 PNG 與 contact sheet。

## 1. 一般 .16：影像集

LZW 解開之後：

```
uint16  count            影像數
uint32  offsets[count]   相對解壓緩衝開頭的絕對偏移
                         offsets[0] 恆等於 2 + count*4（即檔頭長度）

每張影像：
uint16  width            像素
uint16  height
        4bpp packed 像素，每 byte 兩個像素，高 nibble 在左
        （每張影像結尾與下一個 offset 之間固定空 4 bytes，用途未定）
```

影像邊界一律以 `offsets` 為準，不要用寬高回推 —— 那 4 bytes 會讓計算差一截。

`offsets[0] = 2 + count*4` 這條在 count = 1、2、5、36 的檔案上都成立，
是判斷檔頭解讀正確的自我驗證條件。

### 驗證

| 檔案 | 影像 | 尺寸 | 結果 |
|---|---|---|---|
| `NWCP.16` | 1 | 320×82 | 開場畫面 "New World Computing Presents..."，地球與行星正確 |
| `DISK.16` | 1 | 88×67 | 磁碟圖示，上面的 "Might & Magic II" 標題清晰 |
| `TOWNF.16` | 1 | 208×60 | — |
| `TOWNT.16` | 36 | 24×43 起 | 火炬的 36 個透視變體，藍色石牆配色 |
| `CAVET.16` | 36 | 同上 | 同一組火炬，綠色洞穴配色 |

`NWCP.16` 的字母有規律的紅白橫條，那是 EGA 16 色的 dithering，不是解析錯誤 ——
同一支解析器解出的 `DISK.16` 是乾淨的實心色塊。

調色盤目前用 EGA 標準 16 色。**原版開機是否整組換掉調色盤尚未確認**；
姊妹專案冬之魔在這件事上解錯兩次，教訓是「檔裡沒有調色盤不代表用標準表」。
等級：**假設待驗**，要找 `SETRGBPALETTE` 一類的呼叫。

## 2. MONSTERS.16：索引式，段內另一套子格式

168,257 bytes，結構與其他 `.16` 不同：

```
uint32  offsets[75]      第一筆 = 300 = 索引表長度；其中 16 筆為 0（空槽）
        59 個 LZW 段
```

檔頭 `2C 01 00 00` 的形狀與 LZW 段頭（uint16 長度 + uint16 0）**完全一樣**，
無法靠形狀分辨。判準是解開之後 `offsets[0] == 2 + count*4` 是否成立 ——
`MONSTERS.16` 直接當單段解會得到 345 bytes 的垃圾。

59 個段全部解壓成功（長度相符），段內結構：

```
uint16  count
uint16  offsets[count]   ← uint16，不是 uint32
        動畫序列表（見下）
每張影像：uint8 x, uint8 y, uint8 width, uint8 height
        RLE 資料
```

段 0（count=12）的檔頭與 `offsets[0]` 之間有 53 bytes：

```
85 00 | ff 01 07 03 07 01 07 00 07 | ff 02 07 04 07 02 07 00 07
      | ff 05 07 07 07 05 07 00 07 | ff 06 07 08 07 06 07 00 07
      | ff 09 05 0a 05 0b 05 0a 05 09 05 00 05 | ff ff
```

`0xFF` 開頭的每一組是一段動畫，組內是 (影格編號, 停留長度) 對，`FF FF` 收尾。
五組動畫分別用影格 {0,1,3}、{0,2,4}、{0,5,7}、{0,6,8}、{0,9,10,11} ——
與 12 張影像的編號範圍自洽。等級：**強推論**。

影像頭的四個位元組是 x/y/w/h：影像 1 是 `00 14 38 1F`、影像 2 是 `1C 14 38 1F`，
只有第一個位元組不同（0 與 28），寬高同為 56×31 —— 同尺寸不同水平位置的動畫影格。
未壓縮需要 868 bytes 而段長只有 582，所以像素走 RLE。等級：**強推論**。

RLE 的編碼方式**未解**。這是目前 `.16` 這條線唯一的缺口。

## 3. 可重跑指令

```bash
python3 tools/mm216.py workplace/orig/MM2/NWCP.16 workplace/gfx -v
python3 -c "
import sys; sys.path.insert(0,'tools')
from mm216 import parse, contact_sheet
raw,c,hl,offs,imgs = parse(open('workplace/orig/MM2/TOWNT.16','rb').read())
open('workplace/gfx/sheet_TOWNT.png','wb').write(contact_sheet(imgs, 9))
"
```
