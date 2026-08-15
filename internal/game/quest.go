package game

import "fmt"

// 兩位領主的任務：Lord Hoardall 找裝備、Lord Slayer 獵怪。
//
// 原版是 `2CAVES` 的 `sub_1D3C4`（`0e C9` 是 Hoardall、`0e CA` 是 Slayer），
// 機制與每一張表的位址見 `docs/re/02-2caves-special-events.md`。
//
// 這是唯一跨系統的一支：任務狀態常駐在角色記錄裡，而「打死了指派的那隻」
// 是戰鬥那邊回報的（`2COMBAT` 的 `sub_189D2`）。

// QuestLord 是委託人。
type QuestLord byte

const (
	// LordHoardall 收集裝備。
	LordHoardall QuestLord = 0
	// LordSlayer 獵怪。
	LordSlayer QuestLord = 1
)

// 任務狀態在角色記錄裡的位置。
const (
	// offQuestTarget 是指派的目標編號：Hoardall 是物品、Slayer 是怪物。
	// 0 表示沒有單一目標（領主任務就是這種）。
	offQuestTarget = 120
	// offQuestFlags 是旗標位元組。
	offQuestFlags = 124
)

const (
	// questLordBit 是委託人：0 Hoardall、1 Slayer。
	questLordBit = 0x01
	// questKilledBit 由戰鬥點亮：指派的那隻死了。
	questKilledBit = 0x02
	// questActiveBit 表示領主任務（難度 D）進行中。
	questActiveBit = 0x04
	// questDoneHoardall／questDoneSlayer 是兩位領主的任務已完成。
	questDoneHoardall = 0x08
	questDoneSlayer   = 0x10
	// questBeastBits 是領主任務的三隻獸，湊滿才算完成。
	questBeastBits = 0xE0
)

// QuestDifficulty 是四個難度。
type QuestDifficulty int

const (
	QuestPage QuestDifficulty = iota
	QuestSquire
	QuestKnight
	// QuestFinal 是領主任務（選單上的 D）：目標固定，不是隨機挑的。
	QuestFinal
)

// QuestDifficultyNames 是選單上的四個名字。
var QuestDifficultyNames = [4]string{"侍童任務", "扈從任務", "騎士任務", "領主任務"}

// slayerRange 是三個難度的怪物編號範圍：`目標 = rand(1, 件數) + 起點`。
// 原版件數在 `ds:3E36`、起點在 `ds:3E3A`。
var slayerRange = [3][2]int{
	{48, 31},  // A：32–79
	{64, 79},  // B：80–143
	{48, 143}, // C：144–191
}

// hoardallBuckets 是三個難度的六個裝備類別（起點 `ds:3E0C`、件數 `ds:3E1E`）。
//
// 六段全部落在乾淨的類別邊界上（短兵／長柄／遠程／盾／甲／盔），
// 那是「總數與起點讀對了」的獨立佐證。
var hoardallBuckets = [3][6][2]int{
	{{1, 24}, {66, 13}, {92, 6}, {115, 3}, {127, 8}, {155, 2}},
	{{25, 29}, {79, 6}, {98, 7}, {118, 7}, {135, 15}, {157, 2}},
	{{54, 12}, {85, 7}, {105, 10}, {125, 2}, {150, 5}, {159, 1}},
}

// beastRewardStep 是 Slayer 隨機目標的獎勵：依**怪物編號**查門檻
// （`ds:3E3E`），取對應的經驗（`ds:3E48`）。
var beastRewardStep = [10]struct{ upTo, exp int }{
	{48, 2000}, {64, 4000}, {80, 5000}, {96, 7000}, {112, 10000},
	{128, 15000}, {144, 25000}, {160, 50000}, {176, 100000}, {192, 250000},
}

// lordReward 是領主任務的固定獎勵，與難度無關。
const (
	lordRewardHoardall = 100000
	lordRewardSlayer   = 1000000
)

// questSwords 是領主任務要找的三把劍。
var questSwords = [3]int{226, 227, 228} // Valor / Honor / Noble Sword

// questCondLimit 是驗收時角色的狀況上限：`+38` 要小於 0x80。
const questCondLimit = 0x80

// QuestTarget 回報這名角色被指派的目標與委託人。
func QuestTarget(c *Character) (target int, lord QuestLord) {
	return int(c.FieldByte(offQuestTarget)), QuestLord(c.FieldByte(offQuestFlags) & questLordBit)
}

// QuestActive 回報有沒有在跑領主任務。
func QuestActive(c *Character) bool {
	return c.FieldByte(offQuestFlags)&questActiveBit != 0
}

// QuestDone 回報這位領主的任務完成過沒有。
func QuestDone(c *Character, lord QuestLord) bool {
	bit := byte(questDoneHoardall)
	if lord == LordSlayer {
		bit = questDoneSlayer
	}
	return c.FieldByte(offQuestFlags)&bit != 0
}

// AssignQuest 指派任務。回傳要播報的話；已經在任務中就回 nil。
//
// A–C 擲一個目標寫進全隊每個人的 `+120`；D 不寫目標，只點亮
// `questActiveBit`（原版 `sub_1CEB2`：已經完成過這位領主的人跳過）。
func (s *Session) AssignQuest(lord QuestLord, d QuestDifficulty) []string {
	if len(s.Party) == 0 || s.Rand == nil {
		return nil
	}
	if s.QuestPending(lord) != "" {
		return nil
	}
	if d == QuestFinal {
		n := 0
		for i := range s.Party {
			c := &s.Party[i]
			if QuestDone(c, lord) {
				continue
			}
			f := c.FieldByte(offQuestFlags)&^byte(questLordBit) | questActiveBit | byte(lord)
			c.SetFieldByte(offQuestFlags, 0, f)
			n++
		}
		if n == 0 {
			return nil
		}
		what := caveText("exe.3F21", "三把劍。")
		if lord == LordSlayer {
			what = caveText("exe.3F13", "三隻猛獸。")
		}
		return []string{caveText("exe.3E7C", "我為你的隊伍決定的任務，") +
			caveText("exe.3EA3", "是去尋找") + what}
	}

	target := s.rollQuestTarget(lord, d)
	if target == 0 {
		return nil
	}
	for i := range s.Party {
		c := &s.Party[i]
		c.SetFieldByte(offQuestTarget, 0, byte(target))
		c.SetFieldByte(offQuestFlags, ^byte(questLordBit), byte(lord))
	}
	return []string{caveText("exe.3E7C", "我為你的隊伍決定的任務，") +
		caveText("exe.3EA3", "是去尋找") + s.questTargetName(target, lord)}
}

// rollQuestTarget 擲一個目標。
//
// Slayer 直接 `rand(1, 件數) + 起點`；Hoardall 是六個類別的加權挑選 ——
// 擲一個落在總數內的數，逐段扣，落在哪一段就從那一段的起點往後數。
func (s *Session) rollQuestTarget(lord QuestLord, d QuestDifficulty) int {
	if d < 0 || int(d) >= len(slayerRange) {
		return 0
	}
	if lord == LordSlayer {
		r := slayerRange[d]
		return s.Rand.Range(1, r[0]) + r[1]
	}
	buckets := hoardallBuckets[d]
	total := 0
	for _, b := range buckets {
		total += b[1]
	}
	r := s.Rand.Range(1, total) - 1
	for _, b := range buckets {
		if r < b[1] {
			return b[0] + r
		}
		r -= b[1]
	}
	return buckets[len(buckets)-1][0]
}

// questTargetName 把目標編號換成看得懂的名字。
func (s *Session) questTargetName(target int, lord QuestLord) string {
	if lord == LordSlayer {
		if target >= 0 && target < len(s.Bestiary) {
			return s.Bestiary[target].Name
		}
		return fmt.Sprintf("#%d", target)
	}
	return s.itemName(target)
}

// QuestPending 回報這位領主的任務還在進行中的敘述，沒有就回空字串。
func (s *Session) QuestPending(lord QuestLord) string {
	for i := range s.Party {
		c := &s.Party[i]
		flags := c.FieldByte(offQuestFlags)
		if QuestLord(flags&questLordBit) != lord {
			continue
		}
		if t := int(c.FieldByte(offQuestTarget)); t != 0 {
			return caveText("exe.3E08", "你的隊伍已經接下了任務，") +
				caveText("exe.3E0A", "要去找") + s.questTargetName(t, lord)
		}
		if flags&questActiveBit != 0 {
			what := caveText("exe.3F21", "三把劍。")
			if lord == LordSlayer {
				what = caveText("exe.3F13", "三隻猛獸。")
			}
			return caveText("exe.3E08", "你的隊伍已經接下了任務，") +
				caveText("exe.3E0A", "要去找") + what
		}
	}
	return ""
}

// TurnInQuest 結算這位領主的任務。回傳要播報的話。
//
// 順序照原版 `sub_1D094`：先看領主任務（`questActiveBit`），再逐人驗收
// A–C 的隨機目標。兩者都要角色的狀況 `+38 < 0x80`。
func (s *Session) TurnInQuest(lord QuestLord) []string {
	var lines []string
	if l := s.turnInLordQuest(lord); len(l) > 0 {
		lines = append(lines, l...)
	}
	for i := range s.Party {
		if l := s.turnInTarget(&s.Party[i], lord); len(l) > 0 {
			lines = append(lines, l...)
		}
	}
	return lines
}

// turnInLordQuest 驗收難度 D。
func (s *Session) turnInLordQuest(lord QuestLord) []string {
	done := false
	for i := range s.Party {
		c := &s.Party[i]
		flags := c.FieldByte(offQuestFlags)
		if flags&questActiveBit == 0 || QuestLord(flags&questLordBit) != lord {
			continue
		}
		if lord == LordSlayer {
			if flags&questBeastBits != questBeastBits {
				continue
			}
			c.SetFieldByte(offQuestFlags, ^byte(questBeastBits), 0)
		} else if !s.takeQuestSwords() {
			continue
		}
		done = true
	}
	if !done {
		return nil
	}
	reward, bit := lordRewardHoardall, byte(questDoneHoardall)
	if lord == LordSlayer {
		reward, bit = lordRewardSlayer, questDoneSlayer
	}
	for i := range s.Party {
		c := &s.Party[i]
		flags := c.FieldByte(offQuestFlags)
		if flags&questActiveBit == 0 || QuestLord(flags&questLordBit) != lord {
			continue
		}
		c.SetFieldByte(offQuestFlags, ^byte(questActiveBit), 0)
		c.SetFieldByte(offQuestFlags, 0xFF, bit)
		c.SetFieldValue(offExp, 4, uint32(c.Exp+reward))
	}
	return []string{fmt.Sprintf(caveText("exe.3EBB", "你們立了大功，")+
		caveText("exe.3EE2", "應當受賞。")+"%d"+caveText("exe.3EFE", " 點經驗值！"), reward)}
}

// takeQuestSwords 檢查三把劍在不在隊伍身上，在就一併收走。
func (s *Session) takeQuestSwords() bool {
	for _, id := range questSwords {
		if !s.hasQuestItem(id) {
			return false
		}
	}
	for _, id := range questSwords {
		s.takeQuestItem(id)
	}
	return true
}

func (s *Session) hasQuestItem(id int) bool {
	for i := range s.Party {
		c := &s.Party[i]
		for slot := 0; slot < 6; slot++ {
			if int(c.FieldByte(offPackID+slot)) == id {
				return true
			}
		}
	}
	return false
}

func (s *Session) takeQuestItem(id int) bool {
	for i := range s.Party {
		c := &s.Party[i]
		for slot := 0; slot < 6; slot++ {
			if int(c.FieldByte(offPackID+slot)) != id {
				continue
			}
			c.SetFieldByte(offPackID+slot, 0, 0)
			c.SetFieldByte(offPackCharge+slot, 0, 0)
			c.SetFieldByte(offPackAttr+slot, 0, 0)
			return true
		}
	}
	return false
}

// turnInTarget 驗收 A–C 的隨機目標。
func (s *Session) turnInTarget(c *Character, lord QuestLord) []string {
	target := int(c.FieldByte(offQuestTarget))
	flags := c.FieldByte(offQuestFlags)
	if target == 0 || QuestLord(flags&questLordBit) != lord {
		return nil
	}
	if c.FieldByte(offCond) >= questCondLimit {
		return nil
	}
	var reward int
	if lord == LordSlayer {
		if flags&questKilledBit == 0 {
			return nil
		}
		reward = beastReward(target)
		c.SetFieldByte(offQuestFlags, ^byte(questKilledBit), 0)
	} else {
		if !s.hasQuestItem(target) {
			return nil
		}
		s.takeQuestItem(target)
		reward = s.itemValue(target)
	}
	c.SetFieldByte(offQuestTarget, 0, 0)
	c.SetFieldValue(offExp, 4, uint32(c.Exp+reward))
	return []string{fmt.Sprintf(caveText("exe.3EBB", "你們立了大功，")+
		caveText("exe.3EE2", "應當受賞。")+"%d"+caveText("exe.3EFE", " 點經驗值！"), reward)}
}

// beastReward 依怪物編號查獎勵。
func beastReward(target int) int {
	for _, step := range beastRewardStep {
		if target <= step.upTo {
			return step.exp
		}
	}
	return beastRewardStep[len(beastRewardStep)-1].exp
}

// itemValue 取物品的價值（原版讀物品記錄 `+18` 的 word）。
func (s *Session) itemValue(id int) int {
	if id < 0 || id >= len(s.Items) {
		return 0
	}
	return s.Items[id].Price
}

// markQuestKill 是 `2COMBAT` 的 `sub_189D2`：怪物死掉時逐一比對每個隊員
// 指派的目標，型別也要是 Slayer 才點亮 `questKilledBit`。
func markQuestKill(party []Combatant, monster int) {
	if monster == 0 {
		return
	}
	for _, cb := range party {
		c, ok := cb.(*Character)
		if !ok {
			continue
		}
		flags := c.FieldByte(offQuestFlags)
		if flags&questLordBit == 0 {
			continue
		}
		if int(c.FieldByte(offQuestTarget)) != monster {
			continue
		}
		c.SetFieldByte(offQuestFlags, 0xFF, questKilledBit)
	}
}

// MarkQuestKillForTest 讓測試不必真的打一場戰鬥就能回報擊殺。
// 正式路徑是 `Encounter.recordDefeat`。
func (s *Session) MarkQuestKillForTest(monster int) {
	party := make([]Combatant, 0, len(s.Party))
	for i := range s.Party {
		party = append(party, &s.Party[i])
	}
	markQuestKill(party, monster)
}
