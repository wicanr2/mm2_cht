# MAP.DAT — 地圖

```
+0x0000  uint16 = 0x0078      索引表長度（120 bytes）
+0x0000  uint16[60]           每張地圖的檔案內偏移
+0x0078  地圖段 × 60          每段是一個 LZW 段，解出固定 512 bytes
```

索引嚴格遞增，最後一段延伸到檔尾。60 段全部解壓成功，長度一律 512。

## 1. 512 bytes = 兩層 16×16

```
+0x000  地形層  16×16，每格 1 byte
+0x100  屬性層  16×16，每格 1 byte
```

格位置是 **row-major**：格 n 在第 `n/16` 列、第 `n%16` 行，與事件表的 `Cell` 欄位同一套座標。

## 2. 屬性層的 bit3 = 這格有事件

**每一個事件格都在屬性層設了 bit3（高 nibble 的 bit 3，即 `attr[cell] & 0x80`）。**

驗證方式是拿事件表的 `Cell` 欄位（見 [`02-data-files.md`](02-data-files.md) §4）
與地圖屬性層取交集：

| MAP 段 | 事件數 | 屬性層 bit3 的格數 | 交集 | 事件格有 bit3 | bit3 格有事件 |
|---|---|---|---|---|---|
| 0 Middlegate | 42 | 56 | 42 | **100%** | 75% |
| 1 Atlantium | 57 | 67 | 57 | **100%** | 85% |
| 2 Tundara | 69 | 75 | 69 | **100%** | 92% |
| 3 Vulcania | 41 | 70 | 41 | **100%** | 59% |
| 4 Sansobar | 50 | 64 | 50 | **100%** | 78% |

正向是 100%、五個段無一例外；反向不成立 —— 有些設了 bit3 的格子沒有對應的
事件記錄，那些應該是牆或其他不經事件表處理的特殊格。等級：**強推論**
（正向零例外，但 bit3 的完整語意還要看使用端的程式碼）。

## 3. MAP 段 k 對應 EVENTSI 段 k

上表同時確認了地圖與事件檔的段編號是同一套：**只有 MAP 段 0 對 EVENTSI 段 0
的事件格出現富集**（值 8 富集 5.3 倍、值 12 富集 4.4 倍），其餘 59 段都沒有。
把段編號錯開一格，富集就消失。

前五段依序是 Middlegate、Atlantium、Tundara、Vulcania、Sansobar，
與 `MM2.EXE` 尾部的城鎮列表同序。

## 4. 未解

- **地形層每個 byte 的語意。** 把高 nibble 當 tile 索引 render 出來，
  野外段有可辨識的地理結構（大片草地、土路、山脈、對角海岸線），
  城鎮段則是每格不同（合理，城鎮每格都是不同建築）。但實際的 tile 對應表未定。
- **屬性層其餘 7 個位元。** 牆、門、可通行、觸發類型都應該在這裡。
- 後 55 段對應哪些地城與野外區域。

下一步用原版 oracle 逐格移動截圖對照（見
[`docs/playtest/01`](../playtest/01-oracle-timeline.md) §5），
不要再用統計富集度硬猜其餘位元。

## 5. 可重跑指令

```bash
python3 -c "
import sys; sys.path.insert(0,'tools')
from mm2lzw import unpack_segment
m = open('workplace/orig/MM2/MAP.DAT','rb').read()
offs = [int.from_bytes(m[i*2:i*2+2],'little') for i in range(60)]
seg = unpack_segment(m, offs[0])[1]
print('屬性層設了 bit3 的格:', [i for i in range(256) if seg[256+i] & 0x80])
"
```
