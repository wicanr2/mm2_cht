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

// 剩下的 bit 1/3/5/7 是另一個平面。原版用 `shr ..., 1` 把它們移到低位元
// 再照同一套遮罩取用，所以形狀與牆一樣是「每個方向一個位元」，語意未定。
// 已知 bit 7（南向那一個）被 `cmp ds:59C8h, 80h` 單獨測試，
// 而且五座城的事件格 100% 都設了它 —— 現行程式用它判斷有沒有事件。
const AttrHasEvent = 0x80

// WallKind 是一面牆的種類。
type WallKind byte

const (
	WallNone  WallKind = iota // 沒有牆，走得過去
	WallSolid                 // 實牆
	WallDoor                  // 門，擋住但可以撞開或開鎖
)

// HasWall 只回答「擋不擋路」，WallKind 進一步分實牆與門。
//
// **這是強推論，不是已證實。** 原版的 `sub_5E68` 確認了「每個方向 2 位元、
// 值 3 是實牆、值 2 是門」，但那兩個位元各自取自哪個平面還沒定：
// 照「同一個位元組的相鄰兩位元」去算，21,092 面牆裡有 19,572 面會變成門，
// 荒謬。`and al, ds:59C6h` 那一行把兩個東西相 AND 才得到牆，
// 形狀與「兩個平面各出一個位元」一致，也就是下面這個讀法。
//
// 分辨的依據是**地形層同一個位元**：有牆的面裡 78.2% 那個位元是 1，
// 剩下的少數是門。判準是對向一致性 —— 只看有牆的面，地形層位元在相鄰兩格
// 的一致率是 91.7%（全體只有 85.4%），與牆規則自己的 93.8% 同一個量級。
//
// 佐證：中門（地圖 0）379 面牆裡只有 7 面沒標記，而且成對或連成一排；
// 地圖 2 那 20 面排成 y=1 每隔一格一個的規則圖樣，形狀就是一排店門。
// 原版的碰撞訊息也是分開的兩句：`Solid!`（實牆）與 `Locked!`（鎖住了）。
func (m *Map) WallKind(x, y int, f Facing) WallKind {
	c := Cell(x, y)
	if c < 0 {
		return WallSolid
	}
	bit := wallBit[f&3]
	if m.Attr[c]>>bit&1 == 0 {
		return WallNone
	}
	if m.Terrain[c]>>bit&1 == 0 {
		return WallDoor
	}
	return WallSolid
}

// HasWall 回報格 (x, y) 朝 f 那一側有沒有牆。出界一律當成有牆。
func (m *Map) HasWall(x, y int, f Facing) bool {
	c := Cell(x, y)
	if c < 0 {
		return true
	}
	return m.Attr[c]>>wallBit[f&3]&1 != 0
}

// CanMove 回報從 (x, y) 朝 f 走一步能不能成功。
func (m *Map) CanMove(x, y int, f Facing) bool {
	if m.HasWall(x, y, f) {
		return false
	}
	dx, dy := f.Delta()
	return Cell(x+dx, y+dy) >= 0
}
