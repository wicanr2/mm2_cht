package game

import "fmt"

// Chest 是一個寶箱。
//
// 欄位對應原版散在 DGROUP 的那幾格（`_2misc_e02` 開頭讀的就是它們）：
//
//	ds:6953[3]  三件物品的等級（低 6 位）
//	ds:695C[3]  三件物品的編號
//	ds:695A[2]  金幣、寶石
//
// 這裡收成一個結構，語意相同而且不必讓上層去碰全域位址。
type Chest struct {
	// Kind 是箱子的種類（0–4），決定名字用哪一列。
	Kind int
	// Gold、Gems 非零表示裡面有錢。
	Gold, Gems int
	// Items 是三格物品：編號與等級。編號 0 表示沒有。
	Items [3]ChestItem
	// Magic 記三格有沒有魔法（偵測魔法看的就是這個）。
	Magic [3]bool
	// Trap 是陷阱難度。0 表示沒有陷阱，開箱不會出事。
	Trap int
	// TrapKind 是陷阱的種類（0 電、1 火、2 毒氣、3 尖刺），
	// TrapTier 是層級（0–2），一起決定播報哪一句。
	TrapKind, TrapTier int

	opened bool
}

// ChestItem 是箱子裡的一格物品。
type ChestItem struct {
	ID    int
	Level int // 原版存在 ds:6953，只取低 6 位
}

// Quality 是箱子的品質（0–7），也就是名字表的欄索引。
//
// 算式抄自 `_2misc_e02` 開頭：
//
//	每一件物品、金幣、寶石各加一分；一分都沒有就當一分
//	三件物品裡最高的等級（`& 0x3F`）大於 1 再加一分
//	再加上「最高等級 / 4」
//	夾在 8，最後減一當索引
//
// 所以「箱子看起來多高級」直接反映裡面有多少東西 —— 名字不是隨機挑的。
func (c *Chest) Quality() int {
	n := 0
	for _, it := range c.Items {
		if it.ID != 0 {
			n++
		}
	}
	if c.Gold != 0 {
		n++
	}
	if c.Gems != 0 {
		n++
	}
	if n == 0 {
		n = 1
	}
	best := 0
	for _, it := range c.Items {
		if v := it.Level & 0x3F; v > best {
			best = v
		}
	}
	if best > 1 {
		n++
	}
	n += best / 4
	if n > 8 {
		n = 8
	}
	return n - 1
}

// Name 是箱子的名字。
func (c *Chest) Name() string {
	k := c.Kind
	if k < 0 || k >= len(chestNames) {
		k = 0
	}
	q := c.Quality()
	if q < 0 || q >= len(chestNames[k]) {
		q = 0
	}
	return chestNames[k][q].String()
}

// ChestAction 是寶箱那一頁的四個選項。編號與原版的按鍵一致。
type ChestAction int

const (
	ChestOpen    ChestAction = 1 // 1) Open It
	ChestFind    ChestAction = 2 // 2) Find Trap
	ChestDetect  ChestAction = 3 // 3) Detect Magic
	ChestLeave   ChestAction = 4 // 4) Leave it
)

// ChestResult 是一次操作的結果。
type ChestResult struct {
	// Lines 是要播報的話。
	Lines []string
	// Sprung 表示陷阱被觸發了。
	Sprung bool
	// Opened 表示箱子開了，Gold／Gems／Items 已經進了隊伍。
	Opened bool
	// Done 表示這一頁結束（開了、或選了離開）。
	Done bool
}

// disarm 是開箱與拆陷阱共用的那一擲（`sub_1C824` 與 `sub_1C8AE` 逐行相同）。
//
//	r = rand(1, 100)
//	r > 96      → 觸發（4% 一定中，技能再高也擋不掉）
//	r <= 盜行   → 安全
//	否則        → 觸發
//
// 難度 0 的箱子沒有陷阱，怎麼擲都不會出事。
func (c *Chest) disarm(r *Rand, thievery int) bool {
	roll := r.Range(1, 100)
	switch {
	case roll > chestAlwaysSprings:
		return false
	case roll <= thievery:
		return true
	}
	return false
}

// chestAlwaysSprings 是「這一擲以上一定觸發」的門檻（`cmp al, 60h` / `ja`）。
const chestAlwaysSprings = 0x60

// Do 執行一個選項。who 是下手的隊員。
func (s *Session) Do(c *Chest, act ChestAction, who int) ChestResult {
	if c == nil || who < 0 || who >= len(s.Party) {
		return ChestResult{Done: true}
	}
	ch := &s.Party[who]
	switch act {
	case ChestLeave:
		return ChestResult{Done: true, Lines: []string{"隊伍沒有動那個" + c.Name() + "。"}}

	case ChestDetect:
		// 偵測魔法不擲骰、沒有風險（`sub_1C91A` 只是數與印）。
		n := 0
		for i, it := range c.Items {
			if it.ID != 0 || c.Magic[i] {
				n++
			}
		}
		line := c.Name() + "："
		if n > 0 {
			line += "有魔法"
		} else {
			line += "沒有魔法"
		}
		if c.Trap > 0 {
			line += "、有陷阱"
		} else {
			line += "、沒有陷阱"
		}
		return ChestResult{Lines: []string{line}}

	case ChestFind:
		res := ChestResult{}
		if c.Trap > 0 && !c.disarm(s.Rand, ch.Thievery) {
			res.Sprung = true
			res.Lines = append(res.Lines, c.springTrap(s)...)
		} else if c.Trap > 0 {
			res.Lines = append(res.Lines, ch.Name+" 拆掉了陷阱。")
			c.Trap = 0
		} else {
			res.Lines = append(res.Lines, "沒有陷阱。")
		}
		// 原版找完陷阱**一定接著開箱**（`sub_1C824(0FFh)`：跳過判定直接開）。
		open := s.openChest(c, who)
		res.Lines = append(res.Lines, open.Lines...)
		res.Opened, res.Done = open.Opened, true
		return res

	case ChestOpen:
		res := ChestResult{}
		if c.Trap > 0 && !c.disarm(s.Rand, ch.Thievery) {
			res.Sprung = true
			res.Lines = append(res.Lines, c.springTrap(s)...)
		}
		open := s.openChest(c, who)
		res.Lines = append(res.Lines, open.Lines...)
		res.Opened, res.Done = open.Opened, true
		return res
	}
	return ChestResult{Done: true}
}

// openChest 把裡面的東西給隊伍。
func (s *Session) openChest(c *Chest, who int) ChestResult {
	if c.opened {
		return ChestResult{}
	}
	c.opened = true
	res := ChestResult{Opened: true}
	res.Lines = append(res.Lines, c.Name()+"打開了！")
	ch := &s.Party[who]
	if c.Gold > 0 {
		ch.Gold += c.Gold
		res.Lines = append(res.Lines, fmt.Sprintf("獲得 %d 金幣", c.Gold))
	}
	if c.Gems > 0 {
		ch.Gems += c.Gems
		res.Lines = append(res.Lines, fmt.Sprintf("獲得 %d 顆寶石", c.Gems))
	}
	for _, it := range c.Items {
		if it.ID == 0 {
			continue
		}
		name := fmt.Sprintf("物品 %d", it.ID)
		if it.ID < len(s.Items) {
			name = s.Items[it.ID].Name
		}
		if slot := ch.FreeBackpackSlot(); slot >= 0 {
			ch.Items[slot] = ItemSlot{ID: it.ID, Charge: byte(it.Level & 0x3F)}
			res.Lines = append(res.Lines, "獲得 "+name)
			continue
		}
		res.Lines = append(res.Lines, ch.Name+" 的背包滿了，"+name+" 拿不走")
	}
	return res
}

// springTrap 觸發陷阱：播報那兩行，然後對全隊造成傷害。
//
// 傷害公式還沒從 `sub_1C4A6` 解出來（那一段先做畫面閃爍再結算），
// 這裡用箱子的難度當基準，標**假設**。播報的兩行是照原版的表挑的。
func (c *Chest) springTrap(s *Session) []string {
	t, k := c.TrapTier, c.TrapKind
	if t < 0 || t >= len(trapLines) {
		t = 0
	}
	if k < 0 || k >= len(trapLines[t]) {
		k = 0
	}
	var out []string
	for _, l := range trapLines[t][k] {
		if v := l.String(); v != "" {
			out = append(out, v)
		}
	}
	dmg := s.Rand.Range(1, c.Trap*4+4)
	hit := 0
	for i := range s.Party {
		p := &s.Party[i]
		if !p.Condition.Acts() {
			continue
		}
		p.TakeDamage(dmg)
		hit++
	}
	if hit > 0 {
		out = append(out, fmt.Sprintf("全隊各受到 %d 點傷害", dmg))
	}
	return out
}

// ChestFromReward 把 opcode `0x2a` 擺好的獎賞換成一個箱子。
//
// 三件物品的三個位元組依原版的讀取順序是「編號、`ds:6956`、`ds:6953`」，
// 而品質算式看的是 `ds:6953`（等級），所以取第三個位元組。
func ChestFromReward(r Reward) *Chest {
	c := &Chest{Gold: int(r.Gold), Gems: int(r.Gems)}
	for i, it := range r.Items {
		c.Items[i] = ChestItem{ID: int(it[0]), Level: int(it[2]) & 0x3F}
		c.Magic[i] = it[1] != 0
	}
	return c
}

// ClaimReward 領走 opcode `0x2a` 擺好的獎賞。
//
// 原版走的是 `_2misc_e02` 開頭那個分支：`ds:0434` 非零時**不開選單**，
// 直接印 `Treasure!`（`ds:2A2B`）把東西給隊伍 —— 箱子那一頁是別的觸發，
// 那個觸發點還沒找到。
func (s *Session) ClaimReward() []string {
	if !s.World.Reward.Pending || len(s.Party) == 0 {
		return nil
	}
	c := ChestFromReward(s.World.Reward)
	s.World.Reward = Reward{}
	who := 0
	for i := range s.Party {
		if s.Party[i].Condition.Acts() {
			who = i
			break
		}
	}
	out := []string{treasureMsg()}
	return append(out, s.openChest(c, who).Lines[1:]...)
}

// treasureMsg 是 `ds:2A2B`。
func treasureMsg() string {
	if text == nil {
		return "Treasure!"
	}
	return text.Or("exe.2A2B", "Treasure!")
}
