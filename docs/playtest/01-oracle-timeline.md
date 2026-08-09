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

## 3. 走進神殿：設施觸發的實機確認

接在 `shot:fpv` 之後，面北連走十二步（每步之間 `wait:1`）：

```bash
tools/dosbox_run.sh ega "wait:3;key:Return;wait:2;key:s;wait:4;key:g;wait:5;key:z;wait:4;\
key:Up;wait:1;key:Up;wait:1;key:Up;wait:1;shot:c1;key:Up;wait:1;key:Up;wait:1;key:Up;wait:1;shot:c2;\
key:Up;wait:1;key:Up;wait:1;key:Up;wait:1;shot:c3;key:Up;wait:1;key:Up;wait:1;key:Up;wait:1;shot:c4"
```

`c4` 拍到的是**神殿的入口對話**：

	A slim cleric in a cowled robe peers
	at you and asks in a serene voice,
	"May I aid you, travelers (y/n)?"

這是 opcode `0x0e` 的實機確認 —— 走到入口格就跳訊息，
與靜態分析拿掉「招牌格也算入口」那個回退一致
（招牌在 (7,5)、入口在 (7,3)，是分開的兩格）。

## 4. 量到的畫面框（原始 320×200 座標）

截圖是 1024×768，但遊戲畫面就渲染在左上角 320×200 的區域，
**可以直接讀原始像素座標**：

| 區塊 | 大約範圍 |
|---|---|
| 第一人稱視圖 | x 5–217、y 3–131 |
| 右側面板（Protection／Light／Magic／Forces）| x 221–317、y 3–131 |
| 狀態列（`'O' Options`／Day／Year／Face）| y 133–147 |
| 隊伍列表／訊息區 | x 5–317、y 149–187 |

訊息出現時**佔用隊伍列表那一塊**，不是另開視窗。
怪物精靈畫在第一人稱視圖區裡（x 5–217、y 3–131）——
還缺一張戰鬥截圖才能定出精靈在框內的實際座標。

## 5. 兩條訊息通道

三次實機跑下來，訊息不是只有一個地方顯示：

| 訊息 | 顯示位置 | 例子 |
|---|---|---|
| 設施提示（opcode `0x0e`）| **佔用隊伍列表區**（y 149–187，多行）| 神殿「May I aid you, travelers (y/n)?」、旅店「Will you sign the registry (y/n)?」|
| 撞牆 | **佔用狀態列**（y 133–147，單行置中）| `Solid!` |

撞牆時隊伍列表**還在**，設施提示時隊伍列表**被蓋掉**。
remake 的 UI 分層要照這個分 —— 兩者不是同一個視窗。

## 6. 已驗證的設施入口（Middlegate）

從 `shot:fpv` 的位置出發：

| 方向 | 步數 | 結果 |
|---|---|---|
| 北（預設朝向）| 12 | 神殿 |
| 南（`Left` ×2 之後）| 16 | 旅店 |
| 東（`Right` ×1 之後）| 16 | 撞牆（`Solid!`）|

設施提示會擋住後續按鍵，所以走的途中要穿插 `key:n` 取消，
否則第一個設施就停住了 —— 前兩次跑都卡在這裡。

## 7. 起點：實機量出來是 (7,3)

`dump:pos` 把記憶體倒出來，用 opcode 長度表的前十二個值當 pattern
搜到 DGROUP（`命中位址 − 0x15E6`，全 117 MB 只中一處），讀出：

	ds:0392 地圖   = 0
	ds:0393 X      = 7
	ds:0394 Y      = 3
	ds:039D 室內外 = 0
	ds:0426 人數   = 6

**進城後在 (7,3)，不是 `ATTRIB` `+14` 的 (7,5)。**
`+14` 是地圖的預設進入座標（傳送帶 `0FFh` 時用），
「進城」選單走的是另一條路 —— 兩者都成立，只是用在不同場合。

這也解掉了上一節的矛盾：引擎從 (7,5) 面西算得出七步暢通，
從**實測的 (7,3)** 面西只走一步就被擋，與實機的 `Solid!` 一致。
**牆模型沒問題，錯的是起點的推定。**

事件資料說的兩個出口仍然有效：

| 格 | X, Y | opcode | 去向 |
|---|---|---|---|
| 80 | 0, 5 | `0x0c` | 地圖 11 的 (5,5) —— 往荒野 |
| 42 | 10, 2 | `0x0c` | 地圖 17 的 (15,8) |

從 (7,3) 到 (0,5) 要繞路，不是直線。下一步用引擎的牆模型做 BFS
排出按鍵序列，再拿去實機跑 —— 走得到就同時驗證了牆模型。

## 8. 路線驗證：牆模型是對的

用引擎的牆模型 BFS 排出「起點 (7,3) → 城門 (0,5)」的路線
（`TestGateRoute`）：北 2 步到 (7,5)，轉西，再西行 7 步。

實機驗證分兩段，每段用 `dump:` 讀 `ds:0393`／`ds:0394` 對答案：

| 動作 | 期望 | 實機 |
|---|---|---|
| 北 2 步 | (7,5) | **(7,5)** ✓ |
| 轉西後西行 3 步 | (4,5) | **(4,5)** ✓ |
| 走完全部九步 | (0,5) | **(0,5)** ✓ |

**牆模型與原版一致。** 先前那次 `Solid!` 是自動化的問題不是模型的問題：
三個 `key:Right` 連著送、中間沒有 `wait`，DOSBox 吃掉了其中幾個，
隊伍還朝北就往前走，撞上北邊的牆。

> **送鍵要逐個隔開。** 連續的 `key:` 之間沒有 `wait` 時會掉鍵，
> 而掉鍵的症狀（走錯方向、撞牆）與「模型算錯」長得一模一樣。
> 判斷模型對錯之前，先用 `dump:` 確認隊伍真的在你以為的位置。
>
> 這個坑踩了兩次：第一次是三個 `key:Right` 連著送，第二次是
> `key:n;key:Up` 這個組合裡兩個鍵之間沒有 `wait`。**每一個 `key:`
> 後面都要跟一個 `wait:1`**，沒有例外。

驗證用的完整 timeline（`W` ＝ `key:n;wait:1;key:Up;wait:1`）：

```bash
tools/dosbox_run.sh ega "wait:3;key:Return;wait:2;key:s;wait:4;key:g;wait:5;key:z;wait:4;\
$W;$W;key:Right;wait:1;key:Right;wait:1;key:Right;wait:1;\
$W;$W;$W;$W;$W;$W;$W;wait:2;dump:k0"
```

`key:n` 是用來取消途中的設施提示（(7,5) 是旅店招牌那一格）。

走完之後 `ds:0392` 仍是 0、座標 (0,5)。

**再往西走四步，位置仍然是地圖 0 的 (0,5)** —— 沒有移動、也沒有傳送。
所以：

1. (0,5) 是這一列的最西格，再往西是地圖邊界，擋住。
2. **站上 (0,5) 不會觸發那個 `0x0c`。** 事件表把它掛在格 80 沒錯，
   但顯然還有前提 —— 同一段腳本裡 `0x0c` 前面應該有條件 opcode，
   或者觸發時機不是「踩上去」。

把段 0 格 80 那條腳本整段印出來就看懂了 —— 全長九個位元組：

	01 18      ; 顯示字串 #18h
	09 10      ; 問 Y／N
	01 0f      ; 顯示字串 #0Fh
	0c 0b 37   ; 傳送到地圖 11 的 (7,3)

**城門會先問一句 Y／N。** 而我為了取消設施提示，在每一步之前都送
`key:n` —— 那個 `n` 同時把城門的問題也回答成「不要」。

改成走到門口之後送 `key:y`，位置立刻離開地圖 0。

**但落點對不上腳本。** 腳本寫的是「地圖 11 的 (7,3)」，
`dump` 讀到的卻是**地圖 4 的 (8,1)**，而且畫面仍是室內的石牆
（`shots/q6.png` 還是 `Solid!`，不是野外）。

兩種可能還沒分清楚：

1. 觸發的不是段 0 格 80 那條腳本，而是路上另一格的事件。
2. **「段號 ＝ 地圖號」這個對應本身有問題** ——
   目前所有事件分析都建立在這個假設上，它從來沒被獨立驗過。

第 2 點的影響範圍比第 1 點大得多。下一步是拿事件段裡的傳送目標
與 `ATTRIB` 的室內／室外旗標交叉比對：如果段號真的等於地圖號，
室內段的傳送目標分佈應該與地圖的鄰接關係吻合。

> **通用的取消鍵不是通用的。** 用一個「萬用取消」把途中所有提示掃掉，
> 會連你正要通過的那一道也一起否決。自動化送鍵要對每個提示分別決定
> 答案，不能一律 `n`。

## 9. 還沒拍到的

**戰鬥畫面。** 三次都在城內繞，沒遇敵。下一步是走出城門到野外
（Middlegate 的出口方向還沒找到），或直接走到 `docs/playtest/README.md`
列的固定遭遇格。
