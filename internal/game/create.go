package game

// 建立新角色（`1MENU2.OVL`）。
//
// 流程在 `sub_18F48`：擲一組屬性，接著等鍵 ——
//
//	A–G      挑一項與另一項對調（`sub_18A60`）
//	1–8      用那個職業建角色（`sub_18B1A`），名冊滿了就不讓
//	Enter    重擲（`loc_189EE`）
//	Esc      離開
//
// 職業能不能選由屬性決定（`sub_18952`），不是選了才擋。

// NewCharacter 是還沒定案的新角色。
type NewCharacter struct {
	// Attr 是七項屬性，順序與畫面上的 A–G 一致（見 AttrLabels）。
	Attr [NumAttrs]int
	// 選好的職業、種族、陣營、性別；Chosen 記哪幾樣已經選了。
	Class  Class
	Race   Race
	Align  Alignment
	Sex    int
	Name   string
	chosen uint8
}

// NumAttrs 是建角畫面上的屬性項數：引擎的六項再加運氣。
//
// 運氣在角色記錄裡是獨立的一格（`+115`），不在屬性區裡，
// 所以這裡不能直接用 NumStats。
const NumAttrs = int(NumStats) + 1

// LuckIndex 是運氣在建角畫面上的位置（畫面上的 G）。
const LuckIndex = int(NumStats)

// AttrLabels 是畫面上 A–G 的標籤（原版 `ds:07A4` 起那七條）。
var AttrLabels = [NumAttrs]string{"力量", "智慧", "人格", "耐力", "速度", "準確度", "運氣"}

// 已選的位元。
const (
	choseClass uint8 = 1 << iota
	choseRace
	choseAlign
	choseSex
)

// RollNewCharacter 擲一組新屬性。
//
// 每一項是**三次 `rand(10, 79) / 10` 的和**（`loc_189EE`：外層三輪、
// 內層七項，每次擲完除以 10 再累加），所以範圍是 3–21。
func RollNewCharacter(r *Rand) NewCharacter {
	var n NewCharacter
	for round := 0; round < 3; round++ {
		for i := range n.Attr {
			n.Attr[i] += r.Range(10, 79) / 10
		}
	}
	return n
}

// SetClass、SetRace、SetAlign、SetSex 記下選擇。
func (n *NewCharacter) SetClass(c Class) { n.Class, n.chosen = c, n.chosen|choseClass }
func (n *NewCharacter) SetRace(r Race)   { n.Race, n.chosen = r, n.chosen|choseRace }
func (n *NewCharacter) SetAlign(a Alignment) {
	n.Align, n.chosen = a, n.chosen|choseAlign
}
func (n *NewCharacter) SetSex(s int) { n.Sex, n.chosen = s, n.chosen|choseSex }

// Ready 回報四樣都選了、名字也打了沒有。
func (n *NewCharacter) Ready() bool {
	return n.chosen == choseClass|choseRace|choseAlign|choseSex && n.Name != ""
}

// Exchange 把第 i 與第 j 項屬性對調（畫面上的 A–G）。
func (n *NewCharacter) Exchange(i, j int) bool {
	if i < 0 || j < 0 || i >= NumAttrs || j >= NumAttrs || i == j {
		return false
	}
	n.Attr[i], n.Attr[j] = n.Attr[j], n.Attr[i]
	return true
}

// classReq 是每個職業的屬性門檻（`sub_18952` 的八個 case）。
//
// 每一條是「這幾項都要達到門檻」。索引是 AttrOrder 的位置。
var classReq = [8]struct {
	need []int // 要看的屬性
	min  int
}{
	Knight:    {[]int{0}, 15},       // 力量
	Paladin:   {[]int{0, 2, 3}, 13}, // 力量、人格、耐力
	Archer:    {[]int{1, 5}, 13},    // 智力、準確度
	Cleric:    {[]int{2}, 13},       // 人格
	Sorcerer:  {[]int{1}, 13},       // 智力
	Robber:    {[]int{6}, 13},       // 運氣
	Ninja:     {[]int{4, 5}, 13},    // 速度、準確度
	Barbarian: {[]int{3}, 15},       // 耐力
}

// EligibleClasses 回報八個職業各自能不能選。
//
// 原版是擲完屬性當場算好一張表（`sub_18952` 填 `arg_2`），畫面上不能選的
// 就不亮 —— 不是選了才擋。對調屬性之後會重算。
func EligibleClasses(attr [NumAttrs]int) [8]bool {
	var out [8]bool
	for c, req := range classReq {
		ok := true
		for _, i := range req.need {
			if i >= len(attr) || attr[i] < req.min {
				ok = false
				break
			}
		}
		out[c] = ok
	}
	return out
}

// Eligible 回報這組屬性能不能當某個職業。
func (n *NewCharacter) Eligible(c Class) bool {
	e := EligibleClasses(n.Attr)
	if int(c) < 0 || int(c) >= len(e) {
		return false
	}
	return e[c]
}

// RosterFull 回報名冊滿了沒有。原版滿了會印 `*** Roster is Full ***`
// 而且不讓你按 1–8。
func RosterFull(roster []Character) bool { return len(roster) >= MaxRoster }

// MaxRoster 是名冊的上限。`ROSTER.DAT` 是 8,293 bytes，
// 扣掉開頭一個位元組之後剛好 130 × 62 + 32，實際筆數以解析結果為準。
const MaxRoster = 18

// Finish 把選好的東西組成一個角色。
//
// 屬性的當前值與基礎值一開始相同；生命與法力由職業與屬性算出來
// （沿用升級那一套，見 facility.go）。
func (n *NewCharacter) Finish() (Character, bool) {
	if !n.Ready() {
		return Character{}, false
	}
	if !n.Eligible(n.Class) {
		return Character{}, false
	}
	var c Character
	c.Name = n.Name
	c.Class, c.Race, c.Align = n.Class, n.Race, n.Align
	c.Level, c.BattleLevel = 1, 1
	c.Age = 18
	for i := Stat(0); i < NumStats; i++ {
		c.Base[i] = n.Attr[i]
		c.Current[i] = n.Attr[i]
	}
	c.Luck = n.Attr[LuckIndex]
	// 第一級的生命與法力沿用升級那一套（`Train`）：一顆職業的生命骰。
	// 原版第一級怎麼給還沒解，所以這裡用同一套規則，標**假設**。
	c.MaxHP = c.Class.HitDice()
	c.HP = c.MaxHP
	if c.SpellLevel() > 0 {
		c.MaxSP = 4
		c.SP = c.MaxSP
	}
	c.Condition = CondGood
	c.Food = 10
	return c, true
}
