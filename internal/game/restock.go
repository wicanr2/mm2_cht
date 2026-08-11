package game

// 商店進貨的洗牌。
//
// 原版 `2SMITH.img` 的 `sub_1D19C` 每次進貨都跑一次：
//
//	清空 26 格的「抽過了」標記
//	重複 26 次：
//	    n = rand(1, 26) - 1
//	    n 抽過了就重擲
//	    標記[n] = 1
//	    ds:584E[次序] = n
//
// 產出的是 **0–25 的一個排列**，不是「從清單裡挑幾件」—— 二十六格全都會
// 被填滿，差別只在順序。
//
// **這個排列原版沒有人讀。** 位元組序列 `4E 58`（小端序的 `584Eh`）在十四個
// overlay 映像裡總共只出現一次，就是那條寫入指令本身；DGROUP 的初值段裡
// 也沒有任何指標指向它。算完就丟，是被砍掉的功能留下的死碼。
//
// **remake 讓它有用途：貨色輪替。** 這是我們加的，原版沒有這回事。
// 形狀是現成的（每次進貨產生一個排列），缺的只是有人去讀它。

// restockSlots 是洗牌的長度，原版寫死 26。
const restockSlots = 26

// RestockPermutation 照原版的做法擲出一個 0–25 的排列。
//
// **不要改成「洗牌演算法」**（Fisher–Yates 之類）。原版是「重擲到沒抽過為止」，
// 兩者產生的排列分佈相同但**消耗的亂數不同** —— 而亂數序列是共用的，
// 換掉會讓後面的遭遇、寶箱、戰鬥判定全部跟著偏掉。
func RestockPermutation(r *Rand) [restockSlots]int {
	var out [restockSlots]int
	var taken [restockSlots]bool
	for i := 0; i < restockSlots; i++ {
		var n int
		for {
			n = r.Range(1, restockSlots) - 1
			if !taken[n] {
				break
			}
		}
		taken[n] = true
		out[i] = n
	}
	return out
}

// shopShelfSize 是一家店同時上架幾件（原版每座城每一類固定六件）。
const shopShelfSize = 6

// shopGroupSize 是一類商店的貨底總數：五座城 × 六件。
const shopGroupSize = 30

// RestockShop 用洗牌決定這一輪某座城的某一類商店賣哪六件。
//
// 原版的貨架是死的：`ds:43C8` 起那張表按「類別 × 城 × 6」查出來就是全部，
// 走多少次都一樣。這裡改成每輪從**同一類的三十件貨底**裡抽六件：
//
//	索引 = (城 × 6 + 排列[i]) mod 30
//
// 加上「城 × 6」是讓每座城仍有自己的重心 —— 米德格特抽到的東西還是
// 以米德格特原本那六件為起點，不會變成五座城賣一樣的貨。排列的值域是
// 0–25、三十件貨底，所以取模之後三十件都輪得到，而且六個索引互不相同。
func RestockShop(r *Rand, group, town int) []int {
	ids, _ := ShopGoods(group, town)
	if len(ids) == 0 {
		return nil
	}
	all := make([]int, 0, shopGroupSize)
	for t := 0; t < 5; t++ {
		g, _ := ShopGoods(group, t)
		all = append(all, g...)
	}
	if len(all) < shopGroupSize {
		return ids
	}
	// **同一個物品編號在三十件貨底裡本來就會重複**（好幾座城賣同一把劍），
	// 所以互不相同的索引不保證互不相同的貨。挑到重複的就往排列後面順延，
	// 排列走完還湊不到六件才用原本那一組補。
	perm := RestockPermutation(r)
	out := make([]int, 0, shopShelfSize)
	seen := map[int]bool{}
	for _, k := range perm {
		if len(out) == shopShelfSize {
			break
		}
		id := all[(town*shopShelfSize+k)%shopGroupSize]
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, id := range ids {
		if len(out) == shopShelfSize {
			break
		}
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// ShopShelf 回傳這座城這一類商店**這一輪**的貨。
//
// 一天進一次貨：同一天重複進店看到的是同一批，睡過一晚才換。
// 判準是遊戲內的日期（`World.Today`），不是實際時間 ——
// 存檔讀回來看到的必須與存檔當下一致。
func (s *Session) ShopShelf(group, town int) []int {
	if s.shelf == nil {
		s.shelf = map[int][]int{}
	}
	day := 0
	if s.World != nil {
		day = int(s.World.Today())
	}
	key := day<<8 | group<<4 | town
	if v, ok := s.shelf[key]; ok {
		return v
	}
	// 進貨用獨立的亂數源，種子由「日期 × 類別 × 城」散開 —— 不能動主亂數，
	// 那條序列與遭遇、寶箱、戰鬥判定綁在一起，多擲幾次會讓整場偏掉。
	r := NewRand(uint16(key*40503) | 1)
	v := RestockShop(r, group, town)
	s.shelf = map[int][]int{key: v} // 只留當輪，換日就整批換掉
	return v
}
