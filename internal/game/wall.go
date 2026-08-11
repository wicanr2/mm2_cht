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

// 屬性層的奇數位元是**整格的旗標**，不是方向的性質 —— 這也是為什麼
// 它們的對向一致率只有 86.5%（牆位元是 99.7%）。
//
// 讀它們的地方不是平面本身，而是**當前格的快取**（`ds:59C8`，由
// `sub_1B4E0` 每步從平面抄一個位元組進去）。這正是先前只掃平面會找不到
// 讀取點的原因。
//
//	bit 7  這一格有事件（五座城的事件格 100% 都設了它）
//	bit 5  原版沒有任何一處讀它；remake 拿它當房間輪廓，見 AttrRoom
//	bit 3  這一格不能休息 → `Too dangerous!`
//	bit 1  這一格不能施法 → `*** Spell Failed ***`
const (
	AttrHasEvent = 0x80
	// AttrNoRest：`_2misc` 的休息入口 `test byte_159C8, 8`，
	// 成立就印 `ds:2A5C`（`Too dangerous!`）而不是問 `Rest here? (Y/N)`。
	// 950 格、12 張圖。
	AttrNoRest = 0x08
	// AttrNoMagic：`test byte_159C8, 2` 出現在 `2CAST1`（3 處）、
	// `2CAST2`（3 處）與 `2COMBAT`（1 處），全部導向施法失敗。
	// 206 格、7 張圖。
	AttrNoMagic = 0x02
	// AttrRoom 是 bit 5。**原版沒有任何一處讀它** —— 兩個平面基底的全部
	// 20 處參照，加上快取 `ds:59C8` 的 8 條指令，跨十五個映像掃過，零命中
	// （掃描前拿 `ds:03E3` 做過正對照）。
	//
	// 它標的是什麼可以從資料的形狀看出來：125 格分佈在 6 張圖，
	// **每一格都有牆**（125/125），而且排成封閉的輪廓 ——
	// 地圖 59 的 26 格是一個完整的 4×5 矩形外框、地圖 22 的 8 格是三面牆的
	// 房間、地圖 58 的 12 格是中間留缺口的一道長牆。
	//
	// **remake 給它一個用途：房間。** 這是我們加的，原版沒有這回事。
	// 自動地圖把這些格的牆畫成另一種顏色，進出時也報一句。
	AttrRoom = 0x20
)

// InRoom 回報這一格是不是房間輪廓的一部分（屬性層 bit 5）。
func (m *Map) InRoom(x, y int) bool {
	c := Cell(x, y)
	return c >= 0 && m.Attr[c]&AttrRoom != 0
}

// NoMagic 回報這一格禁不禁止施法。
func (m *Map) NoMagic(x, y int) bool {
	c := Cell(x, y)
	return c >= 0 && m.Attr[c]&AttrNoMagic != 0
}

// NoRest 回報這一格能不能休息。
func (m *Map) NoRest(x, y int) bool {
	c := Cell(x, y)
	return c >= 0 && m.Attr[c]&AttrNoRest != 0
}

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

// DrawKind 回報這一面**畫出來長什麼樣**，`WallNone` 表示什麼都不畫。
//
// **畫不畫與擋不擋路是兩件事，而且會不一致。** 擋路看屬性層的位元
// （`HasWall`），畫什麼看地形層的兩個位元 —— 兩邊各自獨立：
//
//	屬性 0、地形 2   看得到門，走得過去   ← 城鎮設施的入口就是這樣
//	屬性 1、地形 0   看不到東西，走不過去 ← 撞上去印 `Barrier!`
//
// 種類 0 之所以叫「屏障」而不是「沒有牆」就是這個意思：它是**看不見的**
// 屏障。拿 `HasWall` 當繪圖條件會同時錯兩邊 —— 該畫的門不畫、
// 該看穿的屏障擋住視線。
//
// 證據是 (7,5) 面北的實機截圖（`cmd/mm2diff`）：屬性層說 (7,6) 北向沒有牆，
// 原版卻在深度 1 畫了 `TOWN.16` 的影格 17（門版的正牆），而 (7,6) 的地形
// 位元組是 `0x80`，北向取出來正好是 2。
func (m *Map) DrawKind(x, y int, f Facing) WallKind {
	c := Cell(x, y)
	if c < 0 {
		return WallSolid // 出界一律當實牆，不然視線會穿出地圖
	}
	if !m.Indoor {
		if m.Attr[c]>>wallBit[f&3]&1 != 0 {
			return WallSolid
		}
		return WallNone
	}
	k := WallKind(m.Terrain[c] >> wallBit[f&3] & 3)
	if k == WallBarrier {
		return WallNone // 屏障看不見
	}
	return k
}

// HasTorch 回報這一面牆上有沒有火炬。**假設待驗**（見下）。
//
// 火炬不是每一面牆都有：`shots/p5.png` 的左邊三個深度都是牆，只有深度 2
// 那一面點著火炬；`shots/22-fpv2.png` 則是深度 0 左右各一盞。所以觸發條件
// 存在，而且是**每面牆**的性質，不是每一格的。
//
// 種類 3 是目前唯一形狀吻合的候選：
//
//   - 它是**逐方向**存的（地形層每個方向兩個位元），與火炬同一個粒度。
//   - 撞牆訊息表把 3 折成 1（`cmp ax, 3` / `mov var_2, 1`），也就是它與
//     實牆的差別**只在畫面上**，這正是裝飾性差異的樣子。
//   - 數量對得上：五張城鎮圖裡種類 3 佔會擋路的牆兩到三成，不是零星幾面
//     也不是滿場都是。
//   - 兩張截圖觀察到的火炬排列在五張城鎮圖裡都排得出來
//     （p5 那一種 4 個視角、22-fpv2 那一種 23 個）。
//
// 要推翻它只需要一件事：一張知道座標與朝向的原版截圖，畫面上的火炬與
// 這裡算出來的不一致。目前手上的 126 張截圖都沒有座標，所以還推不動。
func (m *Map) HasTorch(x, y int, f Facing) bool {
	c := Cell(x, y)
	if c < 0 || !m.Indoor {
		return false
	}
	bit := wallBit[f&3]
	return m.Attr[c]>>bit&1 != 0 && m.Terrain[c]>>bit&3 == 3
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
