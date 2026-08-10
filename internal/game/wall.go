package game

// 牆在屬性層（512 bytes 的後半，見 docs/formats/06-map.md）。每格一個 byte，
// 四個方向各佔一個位元：
//
//	bit 6 = 北   bit 4 = 東   bit 2 = 南   bit 0 = 西
//
// 這正是 `sub_15E68` 裡 `and al, 55h`（`0101_0101`）取出的四個位元。
// 位元與方位的對應由三方交叉確認：
//
//   - 「牆有兩面必須一致」：格 (x,y) 的東側位元要等於格 (x+1,y) 的西側位元。
//     全 60 張地圖的自洽率 93.8%，次高的排列只有 86.7%，隨機期望 50%。
//   - `sub_1423E` 依朝向設定遮罩 `C0`/`30`/`0C`/`03` 與位移量 6/4/2/0，
//     `'N'` 對到的正是 `C0`，也就是 bit 6。
//   - 手冊的城鎮地圖 y 軸由下往上，圖上設施的位置與事件表的格編號在 y 上
//     對得起來 —— 第 0 列在南邊，所以 bit 6 是北而不是南。
var wallBit = [4]uint{
	North: 6,
	East:  4,
	South: 2,
	West:  0,
}

// 屬性層剩下的位元：**bit 7 是「這一格有事件」**（五座城的事件格 100%
// 都設了它，見 docs/formats/06）。bit 5/3/1 的用途仍未解 ——
// 它們在 7.4% 的方向格位被設起來，但對向一致率只有 86.5%
// （牆位元是 99.7%），所以**不是牆的性質**。
const AttrHasEvent = 0x80

// WallKind 是一面牆的種類，值就是原版的訊息編號。
//
// `sub_15E68` 的室內分支：
//
//	al = 屬性層 & 方向遮罩 & 0x55     ; 只留偶數位元 —— 有沒有障礙
//	若為 0            → 走得過去
//	種類 = 地形層 >> 位移 & 3         ; **兩個位元一起取**
//	種類 == 3         → 當成 1
//	拿種類去查 ds:4E4C 的訊息表
//
// `ds:4E4C`：0 `Barrier!`、1 `Solid!`、2 `Locked!`、3 `Not Locked!`、
// 4 `Success!`、5 `Impassable!`、6 `Can't swim!`。
type WallKind byte

const (
	WallBarrier WallKind = iota // 0 屏障
	WallSolid                   // 1 實牆
	WallDoor                    // 2 上鎖的門
	WallNone    = WallKind(0xFF)
)

// String 是撞上去時原版印的那一句。
func (k WallKind) String() string {
	switch k {
	case WallBarrier:
		return wallMsg("exe.4E06", "有屏障！")
	case WallDoor:
		return wallMsg("exe.4E16", "鎖住了！")
	case WallNone:
		return ""
	}
	return wallMsg("exe.4E0F", "是實牆！")
}

func wallMsg(key, fallback string) string {
	if text == nil {
		return fallback
	}
	return text.Or(key, fallback)
}

// HasWall 只回答「擋不擋路」，WallKind 進一步分屏障、實牆與門。
//
// **只適用室內圖**（城鎮與地城）。原版在 `sub_15E68` 依 `ds:039D` 分兩條路，
// 室外走的是另一套（5 位元碼查 `ds:52B2` 的 32 項表分成 5 類）。
//
// **兩個位元不同源**：擋不擋路來自屬性層的低位元，是哪一種來自地形層的
// 兩個位元。先前把兩個位元都從同一個位元組取，算出 92.8% 的牆是門 ——
// 那個荒謬的數字就是同源假設的反證。改成分開取之後，21,092 面牆裡
// 實牆 78%、屏障 14%、門 7.7%。
func (m *Map) WallKind(x, y int, f Facing) WallKind {
	c := Cell(x, y)
	if c < 0 {
		return WallSolid
	}
	bit := wallBit[f&3]
	if m.Attr[c]>>bit&1 == 0 {
		return WallNone
	}
	if !m.Indoor {
		return WallSolid
	}
	k := WallKind(m.Terrain[c] >> bit & 3)
	if k == 3 {
		// 原版把 3 當成 1（`cmp ax, 3` / `mov var_2, 1`）。
		k = WallSolid
	}
	return k
}

// HasWall 回報格 (x, y) 朝 f 那一側有沒有牆。出界一律當成有牆。
func (m *Map) HasWall(x, y int, f Facing) bool {
	c := Cell(x, y)
	if c < 0 {
		return true
	}
	return m.Attr[c]>>wallBit[f&3]&1 != 0
}

// CanMove 回報從 (x, y) 朝 f 走一步會不會被**地圖本身**擋住。
//
// 牆位元只在室內圖成立。野外圖的屬性層低 5 位元放的是地形碼，
// 而牆位元取的是 bit 6/4/2/0 —— 其中 0、2、4 正落在地形碼裡，
// 拿去當牆讀會把「森林」讀成「西邊有牆」。野外的通行條件跟隊伍有關
// （山要登山家、林要探險家），在 `Session.EnterOutdoor`。
func (m *Map) CanMove(x, y int, f Facing) bool {
	if m.Indoor && m.HasWall(x, y, f) {
		return false
	}
	dx, dy := f.Delta()
	return Cell(x+dx, y+dy) >= 0
}
