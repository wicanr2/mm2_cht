# 資料檔格式

輸入檔見 [`docs/inventory.md`](../inventory.md) 的 SHA-256。
探測工具：`tools/probe_dat.py`（產生記錄長度候選，候選一律要另外驗證）。

## 總覽

| 檔案 | bytes | 熵 | 狀態 |
|---|---|---|---|
| `ITEMS.DAT` | 5,120 | 5.40 | **已解**，見 §1 |
| `MM2.CH` | 1,024 | 4.36 | **已解**，見 §2 |
| `MAP.DAT` | 18,748 | 7.50 | 索引與段頭已解，內容經壓縮，見 §3 |
| `EVENTSI.DAT` | 49,609 | 7.72 | 索引已解，內容經壓縮，見 §4 |
| `EVENTSO.DAT` | 25,797 | 7.72 | 同上 |
| `DEFAULT.DAT` | 780 | 2.55 | 6 × 130 bytes 角色記錄，見 §5 |
| `ROSTER.DAT` | 8,293 | 2.30 | 同格式，筆數未定，見 §5 |
| `SPELLS.DAT` | 192 | 4.87 | 未解 |
| `MONSTERS.DAT` | 5,702 | 7.56 | 未解，高熵 |
| `ATTRIB.DAT` | 1,768 | 7.41 | 未解，高熵 |
| `STR.DAT` | 4,700 | 7.80 | 未解，高熵 |
| `.16` × 26 | — | — | 未解 |

高熵（>7.4）的那幾個與 `MAP.DAT`、`EVENTS*.DAT` 的內容段一致，推測共用同一套壓縮。
等級：**假設待驗**。

## 1. ITEMS.DAT — 道具表

**stride 20，256 筆**，無檔頭。

| 偏移 | 長度 | 內容 |
|---|---|---|
| +0x00 | 12 | 名稱，空格填充 |
| +0x0C | 1 | 0x00（256 筆全部如此） |
| +0x0D | 7 | 屬性 |

256 筆的第 12 個位元組全是 0，名稱欄位邊界因此確定。

```
  0  BLANK          00 f0 00 00 00 00 00
  1  Small Club     00 00 00 02 00 01 00
  2  Small Knife    10 00 00 03 00 05 00
  4  Dagger         10 00 00 04 00 08 00
100  Quiet Sling    18 bf bd 05 00 dc 05
255  Useless Item   00 f0 00 00 00 01 00
```

+0x0D 觀察到 `00`/`0x10`/`0x18`（類別或裝備部位），+0x10 與 +0x12 是遞增的 word
（傷害／價格一類）。欄位語意**待驗**——要對照原版的鑑定畫面或商店價目確認，
欄位名不是證據。

## 2. MM2.CH — 8×8 點陣字型

1,024 bytes = **8 bytes/字元 × 128 字元**，每 byte 一列，MSB 在左。

字碼 32–127 是標準 ASCII 字形（已 render 驗證 `!"#$%&'`、`@ABCDEFG`、`` `abcdefg`` 全部對位）。
字碼 0–31 放的是自訂繪圖符號（框線、箭頭、方塊、菱形），不是控制字元。

中文化要換掉的就是這條渲染路徑：原版每個字元固定 8×8，中文需要獨立的高解析點陣層
（見 `CLAUDE.md` §6）。

## 3. MAP.DAT — 地圖

```
+0x0000  uint16 = 0x0078      索引表長度（120 bytes）
+0x0000  uint16[60]           每張地圖的檔案內偏移
+0x0078  地圖段 × 60
```

索引嚴格遞增，最後一段延伸到檔尾。段長 121–476 bytes，平均 310。

每段開頭是固定的 5 bytes `00 02 00 00 00`，其中 `uint16 = 0x0200 = 512`。
16×16 格 × 2 bytes = 512，與 MM2 的地圖尺寸吻合 —— 推定為**解壓後長度**。
段長不固定且內容高熵，說明段體經過壓縮。等級：**強推論**（尺寸吻合 + 變長）。

段體的壓縮格式尚未確定。**不要用檔頭位元組去猜 LZSS/RLE** —— 解壓常式在 root code 裡，
從那裡讀出來才算數。

## 4. EVENTSI.DAT / EVENTSO.DAT — 事件表

兩個檔都以 **uint32 偏移表**開頭：

```
EVENTSI.DAT  +0x00: 0000011C 000005DD 00000AA5 00000F92 00001428 0000...
EVENTSO.DAT  +0x00: 00000000 ×4, 0000011C 0000049D 00000933 00000DC5 ...
```

`EVENTSO.DAT` 前四筆是 0（空槽）。第一個非零偏移都是 0x011C = 284 = 71 × 4，
與索引表長度自洽。內容段熵 7.72，同樣經過壓縮。

檔名的 I/O 推測是 indoor / outdoor。等級：**假設待驗**。

## 5. DEFAULT.DAT / ROSTER.DAT — 角色記錄

**130 bytes/筆**。`DEFAULT.DAT` = 780 = 6 × 130，六個預設角色：
`Sir Felgar`、`Terwin III`、`Sure Valla`、`Gene Eric`、`Cassandra`、`The Hermit`。
名稱在記錄開頭，NUL 結尾。

`ROSTER.DAT` 8,293 bytes 不是 130 的整數倍（130 × 63 + 103），推測有檔頭或檔尾。
記錄長度 130 與社群工具的說法一致。等級：**強推論**。

## 6. MM2.EXE 尾部資料區

`MM2.EXE` 檔案 `0x8610` 起 43,504 bytes 不在 MZ image 內，執行時載入到偏移 `0x0D850`
（見 [`01-overlay-and-memory-layout.md`](01-overlay-and-memory-layout.md) §1）。
950 段可見字串，占 34%。

已定位的內容：

| 尾部偏移 | linear | 內容 |
|---|---|---|
| +0x0022 | 0x0D872 | `Version 1.01` |
| +0x002F | 0x0D87F | 五座城鎮：Middlegate、Atlantium、Tundara、Vulcania、Sansobar |
| +0x005E | 0x0D8AE | 八種職業：Knight、Paladin、Archer、Cleric、Sorcerer、Robber、Ninja、Barbarian |
| +0x009B | 0x0D8EB | 種族：Human、（略）、Dwarf、Gnome、H-Orc |
| +0x00B7 | 0x0D907 | 陣營 Good/Neutral/Evil、性別 Male/Female |
| +0x00E2 | 0x0D932 | 稱號：Arms Master、Athlete、Cartographer、Crusader、… |
| +0x01BD | 0x0DA0D | 狀態與提示：`Dead`、`Stone`、`Eradicated`、`('ESC' to go back)`、`('Space' to continue)` |
| +0x01FD | 0x0DA4D | **資料檔名表**（見下） |

檔名表（NUL 分隔，小寫）：

```
monsters.16 globe.16 disk.16 book.16 throw.16 xfer.16 sky.16 eventsi.dat
nwcp.16 master.16 townf.16 townt.16 townb.16 town.16 cavef.16 cavet.16
caveb.16 cave.16 castlef.16 castlet.16 castleb.16 castle.16
outdoor1.16 outdoor2.16 outdoor3.16 outb.16 outf.16
desert.16 ocean.16 tundra.16 swamp.16 endgame.16 str.dat  map.dat …
```

檔名全小寫而實體檔案全大寫，DOS 不分大小寫所以原版無事；remake 跑在 Linux/macOS 上
必須做大小寫無關解析。

## 7. 下一步

1. **找出壓縮／解壓常式。** 解開它，`MAP.DAT`、`EVENTS*.DAT`、`STR.DAT`、
   `MONSTERS.DAT`、`ATTRIB.DAT` 一起打開，這是目前投報最高的一項。
2. `.16` 圖形格式與 EGA 調色盤。
3. `ITEMS.DAT` 的 7 個屬性位元組語意，需要原版畫面當對照。
