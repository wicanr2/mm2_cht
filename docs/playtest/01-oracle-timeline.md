# 原版 oracle：從開機到第一人稱視角

環境見 [`tools/dosbox_run.sh`](../../tools/dosbox_run.sh)。截圖輸出到
`workplace/dosbox/shots/`（不入版控）。

## 1. 兩個非跑不可的前置條件

**LOADFIX。** MM2 在可用常規記憶體「太多」時會誤判成不足 —— 632 KB free
也照報 `MM2: Not enough memory for 16 color version.`，CGA 模式報的是
`4 color version`，所以與顯示模式無關。`loadfix -64` 先吃掉 64 KB
把程式推到較高位址就正常了。這是 DOSBox wiki 對這個遊戲的唯一指示。

**顯示模式參數。** MM2 靠命令列參數選顯示模式，說明字串就在 EXE 尾部：

```
Valid arguements:
  E - EGA        T - Tandy 1000 16 color
  M - MCGA/VGA   C - CGA          H - Hercules mono
NOTE: MCGA/VGA requires 448K
```

`entrypoint.sh` 把 DOSBox 的 `machine` 與 MM2 的參數一起對上
（`ega`→`machine=ega` + `mm2 E`，`mcga`→`machine=vgaonly` + `mm2 M`）。

## 2. 按鍵流程

```bash
tools/dosbox_run.sh ega "wait:3;key:Return;wait:2;key:s;wait:4;key:g;wait:5;key:z;wait:4;shot:fpv"
```

| 步驟 | 畫面 | 動作 |
|---|---|---|
| `wait:3` | 標題畫面（`MASTER.16` 的 320×196） | — |
| `key:Return` | OPTIONS 選單 | S 開始 / C 複製角色磁片 / A 關於 / **D 循環切換 Disk 2 的磁碟機字母** |
| `key:s` | Main Options | C 建角 / V 檢視 / T 轉移 / G 進城 |
| `key:g` | 角色選擇（1-Middlegate） | 打勾的是隊伍成員，`Z` 離開 |
| `key:z` | **第一人稱視角** | — |

⚠ `D` 是循環切換磁碟機字母，不是確認鍵。在 OPTIONS 畫面按 `d` 之後再按 `s`，
`s` 會被當成「選 drive S」而不是「開始遊戲」。預設就是 Drive C，直接按 `S`。

## 3. 第一人稱視角的版面

```
┌─────────────────────┬──────────────┐
│  3D 視圖 208×120    │ Protection   │  右上是防護值面板
│  （牆／地面／門）    │ Light Magic  │  與 Light/Magic/Forces
├──────┬──────┬───────┴──────┬───────┤
│ 'O'  │ Day= │ Year=        │ Face= │  狀態列
├──────┴──────┴──────────────┴───────┤
│ 1) VOODOO  /14   2) SQUATH   /26   │  隊伍六人與 HP
│ 3) EVILDUDE /9   4) BUDEY    /12   │
│ 5) HOLTMAN /23   6) GAMA     /17   │
└────────────────────────────────────┘
```

初始狀態：Middlegate、`Day=13 Year=900`、`Face=N`。

## 4. 順帶驗證到的事

角色選擇畫面列出 `Sir Felgar`、`Terwin III`、`Sure Valla`、`Gene Eric`、
`Cassandra`、`The Hermit` —— 正是 `DEFAULT.DAT` 解出的六個預設角色
（見 [`docs/formats/02`](../formats/02-data-files.md) §5），順序也一致。
`ROSTER.DAT` 的角色（VOODOO、SQUATH、GAMA、EVILDUDE、BUDEY、HOLTMAN、
Hellfire…）接在後面。

## 5. 接下來要用 oracle 裁決的事

1. **地圖語意。** `MAP.DAT` 每段 512 bytes = 兩個 16×16 的位元組層，
   哪一層是地形、哪些位元是牆／門／事件旗標尚未確定。做法是從已知位置
   逐格移動並截圖，把視野內的牆與地圖資料對照。
2. **`MONSTERS.16` 的 RLE。** 遇敵畫面有怪物圖可以當已知輸出，
   照 [`docs/formats/04`](../formats/04-graphics.md) §3 的截圖反推法做。
3. **戰鬥與升級規則。** 固定隊伍、固定位置的重複戰鬥可以當回歸測試。
