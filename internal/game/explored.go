package game

// 探索記錄。
//
// **原版沒有這個東西** —— 1988 年的玩法是拿方格紙自己畫，遊戲裡沒有
// 任何地圖畫面。這裡加它是為了把紙本的東西收進遊戲（使用者要求：
// 「把完整的遊戲訊息都放在遊戲內，解決當年要翻閱紙本的問題」），
// 所以它只記錄「玩家親自走過哪些格」，不揭露沒去過的地方。
//
// 手冊印了五座城鎮的完整地圖，那是紙本本來就給的；城鎮因此整張直接看得到
// （見 MapAttr.Mapped）。地城與野外只顯示走過的格子。

// Explored 是每張地圖走過哪些格。key 是地圖編號。
type Explored map[int][]bool

// Mark 記下走過這一格。
func (e Explored) Mark(mapIndex, x, y int) {
	if e == nil || mapIndex < 0 || x < 0 || x >= MapW || y < 0 || y >= MapH {
		return
	}
	cells, ok := e[mapIndex]
	if !ok {
		cells = make([]bool, MapCells)
		e[mapIndex] = cells
	}
	cells[y*MapW+x] = true
}

// MarkMap 把整張圖標成看過。
//
// 用途是定位術（`2CAST1 sub_1C1D2`）：手冊寫「顯示目前 16×16 區域之地圖，
// 標示你所在位置及方向」，而原版是把整張 16×16 直接畫出來
// （`_2play_e14` 用 `.16` 的 `B` 圖磚，見 docs/formats/04-graphics.md）。
// remake 的地圖畫面是持久的，所以「畫出整張」對應成「整張標成看過」。
func (e Explored) MarkMap(mapIndex int) {
	if e == nil || mapIndex < 0 {
		return
	}
	cells, ok := e[mapIndex]
	if !ok {
		cells = make([]bool, MapCells)
		e[mapIndex] = cells
	}
	for i := range cells {
		cells[i] = true
	}
}

// Seen 回報這一格看過沒有。
func (e Explored) Seen(mapIndex, x, y int) bool {
	if e == nil || x < 0 || x >= MapW || y < 0 || y >= MapH {
		return false
	}
	cells, ok := e[mapIndex]
	if !ok {
		return false
	}
	return cells[y*MapW+x]
}

// Count 回傳某張地圖走過幾格。
func (e Explored) Count(mapIndex int) int {
	n := 0
	for _, v := range e[mapIndex] {
		if v {
			n++
		}
	}
	return n
}

// TownMaps 是五座城鎮的地圖編號，順序與 `MM2.EXE` 尾部的城鎮列表一致
// （見 docs/formats/06-map.md §3）：Middlegate、Atlantium、Tundara、
// Vulcania、Sansobar。
var TownMaps = [5]int{0, 1, 2, 3, 4}

// Mapped 回報這張地圖是不是「紙本本來就給了整張」。
//
// 只有那五座城鎮 —— 手冊印了它們的完整地圖（docs/manual/part-2）。
// 地城與野外紙本沒給，玩家當年拿方格紙自己畫，所以這裡也只顯示走過的格。
//
// **不要拿場景碼當判準**：六十張裡室內（含地城）全是 0，用它會把整個
// 地下城攤開來。
func Mapped(mapIndex int) bool {
	for _, t := range TownMaps {
		if t == mapIndex {
			return true
		}
	}
	return false
}
