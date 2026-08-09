package game

// 從原版執行時的記憶體讀出來的表。
//
// 這些表在 DGROUP，而 DGROUP 整段是 BSS —— 檔案裡沒有初值。取得方式是
// DOSBox 起遊戲、走到第一人稱視角、dump 行程記憶體（`tools/dosbox_run.sh`
// 的 `dump` 步驟），再用已知的值當 pattern 定出 DGROUP 基底。
//
// 定位方法見 docs/formats/01 §2.8。

// encounterThresholds 是遭遇時決定怪物類別的門檻，`ds:10EA`。
//
// `sub_19A3C` 擲 `rand(1,100)`，從頭掃這張表找出落在哪一段，
// 段號就是怪物類別（0–6）。
var encounterThresholds = [7]int{25, 40, 50, 55, 70, 75, 100}

// encounterBands 是每個類別的怪物編號起點與範圍，`ds:10F6` 起。
//
// 原版每四個位元組一組：`sub_19A3C` 取 `[si+10F6h]` 當基礎編號、
// `[bx+si+10F7h]` 當範圍（bx 是當前的難度等級），
// 最終編號是 `基礎 + rand(1, 範圍)`。
var encounterBands = [4][4]byte{
	{0, 24, 53, 65},
	{65, 13, 19, 26},
	{91, 6, 13, 19},
	{114, 3, 10, 12},
}

// attackDivisor 是算每回合攻擊次數的除數，`ds:1012`，依職業索引。
//
// `sub_18DAA`：`[bx+71h] / [si+1012h]`，si 是職業。
// 武士、遊俠、弓箭手、野蠻人是 1（每級都增加），巫師是 4（最慢）。
var attackDivisor = [8]int{1, 1, 1, 3, 4, 2, 2, 1}

// levelDivisor 是同一段程式碼裡的第二個除數，`ds:101A`。用途待確認。
var levelDivisor = [8]int{4, 4, 5, 7, 10, 5, 5, 4}

// AttacksPerRound 回傳這個角色每回合的攻擊次數。
//
// 形狀取自 `sub_18DAA`（等級除以職業除數再加一），實際的被除數是記錄的
// `+0x71`，那個欄位還沒定位 —— 這裡用經驗等級代替。
func (c *Character) AttacksPerRound() int {
	d := 1
	if int(c.Class) < len(attackDivisor) {
		d = attackDivisor[c.Class]
	}
	if d < 1 {
		d = 1
	}
	n := c.Level/d + 1
	if n < 1 {
		n = 1
	}
	return n
}

// RollEncounterBand 依原版的門檻表決定遭遇的怪物類別。
func RollEncounterBand(r *Rand) int {
	roll := r.Range(1, 100)
	for i, t := range encounterThresholds {
		if roll <= t {
			return i
		}
	}
	return len(encounterThresholds) - 1
}

// RollMonsterIndex 依原版的方式挑一隻怪物：類別決定基礎編號，
// 難度等級決定範圍。
func RollMonsterIndex(r *Rand, difficulty int) int {
	band := RollEncounterBand(r)
	row := band
	if row >= len(encounterBands) {
		row = len(encounterBands) - 1
	}
	base := int(encounterBands[row][0])
	col := difficulty
	if col < 1 {
		col = 1
	}
	if col > 3 {
		col = 3
	}
	span := int(encounterBands[row][col])
	if span < 1 {
		span = 1
	}
	idx := base + r.Range(1, span)
	if idx > 255 {
		idx = 255
	}
	return idx
}
