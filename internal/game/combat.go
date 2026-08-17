package game

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/items"
)

// 戰鬥的流程規則出自手冊（docs/manual/part-2.md §2.11）：
//
//   - 速度決定行動順序，最快的人物或怪物先攻。
//   - 力量影響傷害、準確度影響命中、耐力影響生命點數。
//   - 生命點數歸零時失去意識，之後再受任何傷害即死亡。
//
// 這一層只固定「流程」——誰先動、指令有哪些、狀態怎麼轉移。
// **命中與傷害的公式在 `attack.go`**，已經是原版的兩條路徑
// （怪物打隊伍 `sub_8398`、隊伍打怪物 `sub_8E81`）；怪物記錄那 12 個
// 位元組也已定位（見 docs/formats/02-data-files.md §6）。
// 數值一律由 Combatant 介面提供，這一層不自己算。

// Condition 是身體狀況。名稱用手冊的官方譯法。
type Condition byte

const (
	CondGood Condition = iota
	CondPoisoned
	CondAsleep
	CondUnconscious
	CondDead
)

var condNames = [...]string{"正常", "中毒", "沈睡", "無意識", "死亡"}

func (c Condition) String() string {
	if int(c) >= len(condNames) {
		return "未知"
	}
	return condNames[c]
}

// Acts 回報這個狀況還能不能行動。
func (c Condition) Acts() bool { return c == CondGood || c == CondPoisoned }

// Combatant 是戰鬥中的一方。角色與怪物都實作它。
type Combatant interface {
	CombatName() string
	CombatSpeed() int
	CombatHP() int
	CombatCondition() Condition
	// TakeDamage 扣血並回傳扣完之後的狀況。
	TakeDamage(n int) Condition
}

// CombatCommand 是戰鬥指令。按鍵與手冊的指令表對得上，
// 分派表在 `2COMBAT.OVL` 的 jpt_19573（`sub ax, 46h` 之後 17 個 case）。
type CombatCommand byte

const (
	CmdFight    CombatCommand = 'F' // 戰鬥（近戰）
	CmdShoot    CombatCommand = 'S' // 射擊
	CmdCast     CombatCommand = 'C' // 施法
	CmdUse      CombatCommand = 'U' // 使用物品
	CmdBlock    CombatCommand = 'B' // 抵擋
	CmdRun      CombatCommand = 'R' // 溜跑
	CmdExchange CombatCommand = 'E' // 對調
	CmdView     CombatCommand = 'V' // 檢視
	CmdProtect  CombatCommand = 'P' // 顯示防護效能
)

var cmdNames = map[CombatCommand]string{
	CmdFight: "戰鬥", CmdShoot: "射擊", CmdCast: "施法", CmdUse: "使用物品",
	CmdBlock: "抵擋", CmdRun: "溜跑", CmdExchange: "對調", CmdView: "檢視",
	CmdProtect: "防護",
}

func (c CombatCommand) String() string {
	if s, ok := cmdNames[c]; ok {
		return s
	}
	return string(rune(c))
}

// 九個指令的 handler 都已從反組譯讀出來（位址見 docs/formats/08-combat.md
// 的指令表）。
var confirmedCommands = []CombatCommand{
	CmdFight, CmdShoot, CmdCast, CmdUse, CmdBlock,
	CmdRun, CmdExchange, CmdView, CmdProtect,
}

// ConfirmedCommands 回傳已在 2COMBAT.OVL 找到 handler 的指令。
func ConfirmedCommands() []CombatCommand {
	out := make([]CombatCommand, len(confirmedCommands))
	copy(out, confirmedCommands)
	return out
}

// Encounter 是一場遭遇戰。
type Encounter struct {
	// Flags 是戰鬥期間的一次性旗標，鍵是原版的 DGROUP 位址
	// （`ds:9FC0`–`ds:9FCD`）。開戰時是空的，戰鬥結束就丟掉。
	Flags map[uint16]byte

	Party    []Combatant
	Monsters []Combatant
	Round    int

	// Front 是這一波實際上場、打得到的怪物數（原版 `ds:9FC5`）。
	// 場上可以有上百隻，前排只有這幾隻 —— 死一隻就要重新夾。
	Front int

	// Protect 是開戰時抄下來的防護法術計數器（`ds:03E3`–`ds:03E7`）。
	// 原版是全域的，戰鬥中不會變 —— 開戰抄一份，語意相同而且不必
	// 讓戰鬥層去碰 Session。
	Protect Protection

	// Ranged 是「隊伍這一回合用射擊」（原版 `ds:54A4`）。
	//
	// 原版是逐一動作設定的：每個人下攻擊指令時 `sub_190D6` 設 0、
	// `sub_190C0` 設 1，動作結束沒有清（`sub_184FE` 尾巴才歸零）。
	// 這裡的迴圈一次跑完整隊，所以放在遭遇上，語意是「這一回合的指令」。
	Ranged bool

	// Target 是隊伍這一回合集火的目標，`Monsters` 的索引
	// （原版 `ds:9FCE`，由 `sub_18DAA` 把玩家按的字母減掉 `'A'` 得到）。
	//
	// **零值就是原版的預設**：原版兩個不問目標的指令（`0x19422` 的
	// `1` 與 `0x19439` 的 `A`）都是傳 0 進去。指到打不到、已經倒下或
	// 超出範圍的那一隻時退回「第一個站著的」——
	// 目標在解析與實際揮擊之間失效是常態不是特例（怪物死掉會整批往前搬）。
	Target int

	// SpellTarget 是這一次施法要打的那一隻，`Monsters` 的索引。
	//
	// 與 `Target` 分開存是因為**範圍不同**：近戰只問得到前排
	// （`ds:9FC5`），法術問的是場上全部（原版提示 `On which (A-J)?`，
	// 同一場戰鬥的近戰提示是 `Fight which (A - E)?`，2026-08-17 實機量到，
	// 見 `docs/research/spell-interaction-oracle.md`）。
	//
	// 負值＝這一次沒有指定，照舊打第一個站著的。每次施法前由 UI 清掉。
	SpellTarget int

	// Killed 與 Lost 是**最近一次 `Fight` 呼叫**裡倒下的敵人數與隊員數。
	// 每次 `Fight` 開頭歸零 —— 它們是給音效用的一次性訊號，不是統計。
	Killed, Lost int

	// 戰利品是戰鬥中逐隻死亡時累加的原版全域值（`ds:1695A`、
	// `ds:1695C/E`）。只在玩家獲勝後由 Session.VictoryChest 消費；
	// 競技賽由 UI 另走 ArenaReward，不會誤用這條一般戰鬥路徑。
	// 2MISC 的 sub_1C64A 將 ds:1695A（word）加到角色 +5C Gems，
	// 將 ds:1695C/E（dword）加到角色 +66 Gold；名稱依消費端而定，
	// 不依反編譯器全域符號猜測。
	lootGold      uint32
	lootGems      uint16
	lootBand      int
	lootTier      int
	lootEligible  bool
	lootGenerated bool
	defeated      map[*Monster]bool
}

// MaxFront 是前排的上限。原版 `0x196BB` 把 `ds:9FC5` 夾在 10。
// 名單面板一次也只顯示十行，兩者同源。
const MaxFront = 10

// NextActor 挑出這一輪下一個行動的人，回傳它與「是不是隊伍這一邊」。
//
// 原版的排程（`0x1A1CC` 與 `0x1A200` 兩個迴圈）：
//
//	怪物：在前十隻裡跳過 ds:5480[i] != 0（這一輪動過了）的，取 ds:9F92[i] 最大
//	角色：在全隊裡跳過 ds:548C[i] != 0 的，取記錄 +110 最大
//	兩邊的最大值再比：角色的 >= 怪物的就角色先動
//
// 兩個計數陣列在每一輪開頭清零（`0x1A123` 與 `0x1A138`）。
// acted 記的就是那兩個陣列；回傳 ok = false 表示這一輪沒人可動了。
func (e *Encounter) NextActor(actedParty, actedMonsters map[int]bool) (i int, party, ok bool) {
	best, bi := 0, -1
	for k, c := range e.Monsters {
		if k >= MaxFront || actedMonsters[k] {
			continue
		}
		if v := c.CombatSpeed(); v > best {
			best, bi = v, k
		}
	}
	pbest, pi := 0, -1
	for k, c := range e.Party {
		if actedParty[k] {
			continue
		}
		if v := c.CombatSpeed(); v > pbest {
			pbest, pi = v, k
		}
	}
	switch {
	case pi >= 0 && pbest >= best:
		return pi, true, true
	case bi >= 0:
		return bi, false, true
	case pi >= 0:
		return pi, true, true
	}
	return 0, false, false
}

// RollFront 決定這一波前排有幾隻（原版 `sub_19640`）。
//
//	室外：rand(10, 39) / 10 + 3            → 4–6
//	室內：rand(10, 69) / 10 + 隊伍人數 / 2  → 1–6 加人數的一半
//
// 接著依難度旗標 `ds:0415` 調整（2 減半、3 加倍），最後夾在
// `[0, min(場上總數, 10)]`。difficulty 給 2 或 3 以外的值就不調整。
func (e *Encounter) RollFront(r *Rand, indoor bool, difficulty int) int {
	var n int
	if indoor {
		n = r.Range(10, 69)/10 + len(e.Party)/2
	} else {
		n = r.Range(10, 39)/10 + 3
	}
	switch difficulty {
	case 2:
		n /= 2
	case 3:
		n *= 2
	}
	e.Front = n
	e.clampFront()
	return e.Front
}

// clampFront 把前排夾回上限：不超過場上總數，也不超過 MaxFront。
func (e *Encounter) clampFront() {
	if e.Front > len(e.Monsters) {
		e.Front = len(e.Monsters)
	}
	if e.Front > MaxFront {
		e.Front = MaxFront
	}
	if e.Front < 0 {
		e.Front = 0
	}
}

// Reachable 回傳這一次攻擊打得到的怪物。
//
// 原版在 `sub_18DAA` 開頭依 `ds:54A4` 挑可選目標的隻數：
// 近戰用 `ds:9FC5`（前排），射擊用 `ds:0508`（場上總數），
// 兩者都夾在 10 —— 選單只發得出 A..J 十個字母。
//
// **這是射擊與近戰唯一的目標差異**：射擊不是命中率比較好，
// 是打得到後排。
func (e *Encounter) Reachable(ranged bool) []Combatant {
	n := e.Front
	if ranged {
		n = len(e.Monsters)
	}
	if n > MaxFront {
		n = MaxFront
	}
	if n > len(e.Monsters) {
		n = len(e.Monsters)
	}
	if n < 0 {
		n = 0
	}
	return e.Monsters[:n]
}

// SpellOrder 回傳法術要掃的怪物順序：玩家挑中的那一隻排最前面。
//
// 沒指定（或指到已經倒下的）就是原本的陣列順序 —— 與原版「從第 0 隻
// 往後掃」相同。多目標法術也走這裡：指定了就從那一隻開始往後掃。
func (e *Encounter) SpellOrder() []int {
	out := make([]int, 0, len(e.Monsters))
	first := -1
	if e.SpellTarget >= 0 && e.SpellTarget < len(e.Monsters) &&
		e.Monsters[e.SpellTarget].CombatCondition().Acts() {
		first = e.SpellTarget
		out = append(out, first)
	}
	for i := range e.Monsters {
		if i == first {
			continue
		}
		out = append(out, i)
	}
	return out
}

// PartyTarget 回傳隊伍這一回合實際會打的那一隻。
//
// 先看 `Target` 指到的：**打得到（在 `Reachable` 的範圍內）而且還站著**
// 才算數，否則退回第一個站著的。兩層都不成立就回 nil ——
// 近戰在「前排清空、後排還在」時就是這個狀況，原版那一位也是白站一回合。
func (e *Encounter) PartyTarget() Combatant {
	foes := e.Reachable(e.Ranged)
	if e.Target >= 0 && e.Target < len(foes) && foes[e.Target].CombatCondition().Acts() {
		return foes[e.Target]
	}
	return firstStanding(foes)
}

// RemoveMonster 把第 i 隻怪物從場上刪掉，後面的往前搬。
//
// 原版 `sub_18A22`：`ds:0508`（場上怪物數）減一，然後從死掉那一格起
// 把六個平行陣列各往前搬一格 —— 編號 `ds:9680`、狀態 `ds:9F86`、
// `ds:9F92`、剩餘行動 `ds:9F9E`、HP `ds:9FAA`（word）、`ds:5480`。
// 搬完重新夾 `ds:9FC5`（前排數）並重畫名單。
//
// **原版沒有屍體** —— 死掉與逃走走同一條路（`sub_18AF4`），
// 差別只在印哪一句話。所以之後 `Monsters[i]` 指的是別隻怪，
// 拿著索引跨越死亡事件是錯的。
func (e *Encounter) RemoveMonster(i int) bool {
	if i < 0 || i >= len(e.Monsters) {
		return false
	}
	e.Monsters = append(e.Monsters[:i], e.Monsters[i+1:]...)
	e.clampFront()
	return true
}

// RecordDefeat 將已死亡的怪物納入原版戰利品累加器。由 UI 在每回合結束後
// 呼叫，避免把正常戰鬥、競技賽與事件獎賞混成一條路徑。
func (e *Encounter) RecordDefeat(r *Rand) {
	for _, c := range e.Monsters {
		m, ok := c.(*Monster)
		if !ok || m.CombatCondition().Acts() {
			continue
		}
		e.recordDefeat(r, m)
	}
}

// VictoryChest 依本場戰鬥已證實的怪物記錄欄位與遭遇分段產生一般寶箱。
// 沒有任何掉落就回 nil；此函式不處理競技賽與事件 0x2a。
func (e *Encounter) VictoryChest(r *Rand) *Chest {
	return e.victoryChest(r, nil)
}

// VictoryChestFromItems 是帶玩家原版 ITEMS.DAT 的正常玩家入口。物品 ID
// 仍由 MM2.EXE 的遭遇分段與玩家自備物品表生成，缺表時只結算金幣／寶石。
func (e *Encounter) VictoryChestFromItems(r *Rand, table []items.Item) *Chest {
	return e.victoryChest(r, table)
}

func (e *Encounter) victoryChest(r *Rand, table []items.Item) *Chest {
	if e == nil || !e.PartyWon() || e.lootGenerated {
		return nil
	}
	e.RecordDefeat(r)
	e.lootGenerated = true
	c := &Chest{Kind: 0, Gold: int(e.lootGold), Gems: int(e.lootGems)}
	if e.lootEligible {
		// sub_19B88 的 0–3 件判定，門檻為 11／46／91。
		n := 0
		switch roll := r.Range(1, 100); {
		case roll < 11:
			n = 3
		case roll < 46:
			n = 2
		case roll < 91:
			n = 1
		}
		if n > len(c.Items) {
			n = len(c.Items)
		}
		column := e.lootBand
		for i := 0; i < n; i++ {
			// 沒有玩家自備 ITEMS.DAT 時，原始物品 ID 沒有語意；
			// 不可用 band 基數拼出一個看似合理的假物品。
			if len(table) == 0 {
				break
			}
			// sub_19B88 先把 byte_15494 減一，再傳給 sub_19A3C；
			// 因此低兩位 1、2 對應的是 0、1 欄，而非遭遇怪物的 +1 欄。
			if column > 0 {
				column--
			}
			id := rollLootItem(r, column)
			if id <= 0 || id >= len(table) {
				continue
			}
			charge := 0
			if data != nil && table[id].Raw[15] != 0 && column < len(data.Encounter.LootCharges) {
				charge = data.Encounter.LootCharges[column]
			}
			// `+0x0E == 0xF0` 的東西（鑰匙、票券、藥水這類非裝備品）
			// **照樣掉，只是不附魔**。原版 `sub_19A3C` 的順序是：
			// `0x19AC2` 先把編號寫進 `ds:6950`，`0x19AC9` 依 `+0x0F`
			// 取充能，`0x19ADF` 才檢查 `0xF0` —— 命中就跳到函式尾端，
			// 而尾端照樣把充能寫回去，只有附魔那一擲沒跑到。
			// 所以不是「跳過這件物品」，也不是「連充能都沒有」。
			attr := byte(0)
			if table[id].Raw[14] != 0xF0 {
				attr = lootAttribute(r, e.lootTier)
			}
			c.Items[i] = ChestItem{ID: id, Charge: charge, Level: int(attr)}
			c.Magic[i] = attr&0xC0 != 0
		}
	}
	if c.Gold == 0 && c.Gems == 0 {
		empty := true
		for _, it := range c.Items {
			if it.ID != 0 {
				empty = false
			}
		}
		if empty {
			return nil
		}
	}
	return c
}

func rollLootItem(r *Rand, column int) int {
	if data == nil || len(data.Encounter.Bands) == 0 {
		return 0
	}
	row := RollEncounterBand(r)
	if row < 0 || row >= len(data.Encounter.Bands) {
		return 0
	}
	return rollLootBandItem(r, data.Encounter.Bands[row], column)
}

func rollLootBandItem(r *Rand, band []int, column int) int {
	if len(band) < 2 {
		return 0
	}
	// 每列是 [基礎 ID, band0 範圍, band1 範圍, band2 範圍]。
	// IDA `sub_19A3C` 先讀 ds:10F6+row*4 當基數，再讀
	// ds:10F7+row*4+byte_15494 當擲骰上限；因此 slice 欄位要 +1。
	col := column + 1
	if col < 1 {
		col = 1
	}
	if col >= len(band) {
		col = len(band) - 1
	}
	span := band[col]
	if span < 1 {
		span = 1
	}
	return band[0] + r.Range(1, span)
}

func lootAttribute(r *Rand, tier int) byte {
	if tier < 2 {
		return 0
	}
	level := r.Range(1, 7)
	if tier <= 12 {
		level = r.Range(1, tier)
	} else if tier == 13 {
		level = r.Range(1, 21) + 11
	}
	if level < 5 {
		return byte(level)
	}
	switch roll := r.Range(1, 100); {
	case roll < 41:
		return byte(level | 0x80)
	case roll < 71:
		return byte(level | 0x40)
	default:
		return byte(level | 0xC0)
	}
}

// Reap 把已經倒下的怪物一次清掉，回傳清掉幾隻。
//
// 原版是「誰死了就當場移除」，沒有集中清理這一步；這裡提供它是
// 為了讓上層在一次群體法術之後把場面收乾淨，語意與逐隻呼叫
// RemoveMonster 相同。
func (e *Encounter) Reap() int {
	n := 0
	for i := len(e.Monsters) - 1; i >= 0; i-- {
		if !e.Monsters[i].CombatCondition().Acts() {
			e.RemoveMonster(i)
			n++
		}
	}
	return n
}

// TryFlee 判定第 i 隻怪物這一輪要不要逃走，要的話當場移除。
//
// 原版 `0x1858C`：`ds:9FC4`（禁逃旗標）為 0 時，拿這隻怪的士氣門檻
// 與 `ds:0FC2`（隊伍最高等級的一半）比，門檻**小於**它才可能逃 ——
// 隊伍越強，弱怪越留不住。過了這關再擲 `rand(1,100) <= 50`。
//
// 士氣層 3 的門檻是 255，`ds:0FC2` 是等級的一半、上限 127，
// 所以那一層的怪物永遠不逃。
func (e *Encounter) TryFlee(r *Rand, i int, blocked bool) bool {
	if blocked || i < 0 || i >= len(e.Monsters) {
		return false
	}
	m, ok := e.Monsters[i].(*Monster)
	if !ok {
		return false
	}
	threshold := 255
	if data != nil {
		threshold = data.FleeThreshold(m.Def.MoraleTier)
	}
	if threshold >= e.partyPower() {
		return false
	}
	if r.Range(1, 100) > 50 {
		return false
	}
	e.recordDefeat(r, m)
	e.RemoveMonster(i)
	return true
}

// recordDefeat 是原版 `sub_188FC` 的最小 typed 版本。它必須在怪物死亡／
// 逃走當下呼叫，不能等整場戰鬥結束才猜掉落，否則共用亂數序列會偏移。
func (e *Encounter) recordDefeat(r *Rand, m *Monster) {
	if m == nil {
		return
	}
	if e.defeated == nil {
		e.defeated = make(map[*Monster]bool)
	}
	if e.defeated[m] {
		return
	}
	e.defeated[m] = true
	// 領主任務：怪物死掉時比對每個隊員被指派的目標（`2COMBAT sub_189D2`）。
	markQuestKill(e.Party, m.Def.Index)
	if m.Def.GemDrop {
		e.lootGems += uint16(r.Range(1, 10))
	}
	if m.Def.GoldMode != 0 {
		// byte_19E21=1 不把怪物層級帶進高位；2 取層級，3 再除二。
		v := m.Def.Index >> 4
		if m.Def.GoldMode == 1 {
			v = 0
		} else if m.Def.GoldMode >= 3 {
			v >>= 1
		}
		low := r.Range(1, v)
		amount := uint32(r.Range(1, 50) + 6 + (low << 8))
		e.lootGold += amount
	}
	// `sub_19BF8` 只有 byte_15494／byte_15497 非零才呼叫
	// `sub_19B88`；前者由怪物 b16 低兩位的最大值更新。
	// 因此純金幣／寶石的戰鬥不會憑空生成物品。
	if m.Def.DropBand > 0 && (e.lootBand == 0 || m.Def.DropBand >= e.lootBand) {
		e.lootEligible = true
		e.lootBand = m.Def.DropBand
		e.lootTier = m.Def.Tier
	}
}

// TryRun 是戰鬥指令 `R`（溜跑）：第 i 名角色擲一次，成功就脫離戰鬥。
//
// `sub_1914A`：擲 `rand(1, 100)`，**小於**這張地圖的 `ATTRIB +13`
// 才算成功。跑掉的是**下指令的那一個人**，不是整隊 —— 其他人留在
// 場上繼續打。chance 由 `MapAttr.RunChance()` 提供。
func (e *Encounter) TryRun(r *Rand, i, chance int) bool {
	if i < 0 || i >= len(e.Party) {
		return false
	}
	if r.Range(1, 100) >= chance {
		return false
	}
	e.RemoveMember(i)
	return true
}

// RemoveMember 把第 i 名角色移出戰鬥。
//
// 原版用的是**把最後一格搬進空出來的那一格**（`0x1917C`），不是像
// 怪物那樣整批往前搬 —— 所以隊伍在戰鬥中的順序會變。同一支 overlay
// 裡兩種移除法並存，照抄才會與原版的後續判定一致。
func (e *Encounter) RemoveMember(i int) bool {
	if i < 0 || i >= len(e.Party) {
		return false
	}
	last := len(e.Party) - 1
	if i != last {
		e.Party[i] = e.Party[last]
	}
	e.Party = e.Party[:last]
	return true
}

// partyPower 是 `ds:0FC2`：隊伍裡最高等級的一半（`sub_1974C`）。
func (e *Encounter) partyPower() int {
	best := 0
	for _, c := range e.Party {
		ch, ok := c.(*Character)
		if !ok {
			continue
		}
		if v := ch.Level / 2; v > best {
			best = v
		}
	}
	return best
}

// 死亡與逃走的播報。兩句都在 EXE 尾部的字串表裡，
// 由 `sub_18AB8` 依 `ds:54A6` 二選一，前面接怪物名。
const (
	msgGoesDown = "exe.1197" // " goes down!"
	msgRunsAway = "exe.118B" // " runs away!"
)

// 怪物攻擊的動詞是**隨機八選一**：`sub_17FB2` 擲 `rand(1, 8)`，
// 拿結果去查 `ds:1058` 那張字串指標表（索引 1 起算，所以第一個
// 指標在 `ds:105A`）。
//
// 隊伍那一側不擲 —— `sub_18C78` 固定用 ` attacks ` 或 ` shoots `。
// 花樣在怪物身上，不在玩家身上。
var monsterVerbs = [8]struct{ key, fallback string }{
	{"exe.0C44", "attacks"},
	{"exe.0C4C", "fights"},
	{"exe.0C53", "charges"},
	{"exe.0C5B", "battles"},
	{"exe.0C63", "thrusts at"},
	{"exe.0C6E", "slashes at"},
	{"exe.0C79", "strikes at"},
	{"exe.0C84", "engages"},
}

const (
	msgPartyMelee = "exe.11D5" // " attacks "
	msgPartyShoot = "exe.11CC" // " shoots "
)

// MonsterVerb 擲一個怪物攻擊的動詞。
func MonsterVerb(r *Rand) string {
	v := monsterVerbs[r.Range(1, len(monsterVerbs))-1]
	if text == nil {
		return v.fallback
	}
	return text.Or(v.key, v.fallback)
}

// PartyVerb 是隊伍攻擊的動詞，近戰與射擊各一句，不擲。
func PartyVerb(ranged bool) string {
	key, fallback := msgPartyMelee, " attacks "
	if ranged {
		key, fallback = msgPartyShoot, " shoots "
	}
	if text == nil {
		return strings.TrimSpace(fallback)
	}
	return strings.TrimSpace(text.Or(key, fallback))
}

// LeaveMessage 組出某隻怪物離場的播報：逃走是 fled，倒下是 !fled。
func LeaveMessage(name string, fled bool) string {
	key, fallback := msgGoesDown, " goes down!"
	if fled {
		key, fallback = msgRunsAway, " runs away!"
	}
	if text == nil {
		return name + fallback
	}
	return name + text.Or(key, fallback)
}

// Order 回傳這一回合的行動順序：行動鍵大的先動，平手時隊伍優先。
//
// 原版**不預先排序**：每次要挑下一個行動者時，在「這一輪還沒動過」的
// 裡面各自取最大（怪物 `0x1A1CC`、角色 `0x1A200`），兩邊的最大值再比一次，
// **平手時角色先動**（`0x1A243` 的 `jb`）。NextActor 照那個做法；
// Order 是它的整輪展開，只在同鍵時的先後上與原版一致。
func (e *Encounter) Order() []Combatant {
	type slot struct {
		c     Combatant
		party bool
	}
	all := make([]slot, 0, len(e.Party)+len(e.Monsters))
	for _, c := range e.Party {
		all = append(all, slot{c, true})
	}
	for _, c := range e.Monsters {
		all = append(all, slot{c, false})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].c.CombatSpeed() != all[j].c.CombatSpeed() {
			return all[i].c.CombatSpeed() > all[j].c.CombatSpeed()
		}
		return all[i].party && !all[j].party
	})
	out := make([]Combatant, 0, len(all))
	for _, s := range all {
		if s.c.CombatCondition().Acts() {
			out = append(out, s.c)
		}
	}
	return out
}

// Over 回報戰鬥是否結束：一方全倒就結束。
func (e *Encounter) Over() bool {
	return countActive(e.Party) == 0 || countActive(e.Monsters) == 0
}

// PartyWon 只在怪物全倒、而隊伍還有人站著時為真。
func (e *Encounter) PartyWon() bool {
	return countActive(e.Monsters) == 0 && countActive(e.Party) > 0
}

func countActive(cs []Combatant) int {
	n := 0
	for _, c := range cs {
		if c.CombatCondition().Acts() {
			n++
		}
	}
	return n
}

// CombatName 等方法讓 Character 直接參戰。
func (c *Character) CombatName() string { return c.Name }

// CombatSpeed 是排行動順序用的值。
//
// 原版讀的是記錄 `+110`（`0x1A216` 的 `[bx+0x6e]`），也就是**屬性區
// 第四格的當前值**，而不是第五格。同一格也是防護等級加成的來源
// （root `sub_14F3A` 讀基礎那一份的 `+19`）。
//
// 第四格是**速度**（見 `Stat` 的三條證據），所以原版拿速度排行動順序，
// 與手冊寫的一致。
func (c *Character) CombatSpeed() int           { return c.Current[Speed] }
func (c *Character) CombatHP() int              { return c.HP }
func (c *Character) CombatCondition() Condition { return c.Condition }

// TakeDamage 扣血，回傳扣完之後的狀況。
//
// 抄自 root 的 `sub_13928` 尾巴（`0x13A19`–`0x13A5C`），那是隊伍受傷的
// 唯一收口：
//
//	狀況 &= 0xEF                       ; 挨打會醒
//	狀況 & 0x40（已經無意識）→ 狀況 = 0x81（死亡）、生命 = 0
//	生命 <= 傷害             → 狀況 |= 0x40、生命 = 0
//	否則                     → 生命 -= 傷害
//
// **狀況位元組跟著改**：`CondBits` 與 `Condition` 是同一個位元組的兩種
// 看法，只改其中一份會讓下一次目標挑選看到相反的答案。
func (c *Character) TakeDamage(n int) Condition {
	if c.Condition == CondDead || c.CondBits&CondBitSevere != 0 {
		return c.Condition
	}
	bits := c.CondBits &^ CondBitAsleep
	switch {
	case c.Condition == CondUnconscious || bits&CondBitUnconscious != 0:
		c.setCond(CondDeadBits)
		c.setHP(0)
	case c.HP <= n:
		c.setCond(bits | CondBitUnconscious)
		c.setHP(0)
	default:
		c.setCond(bits)
		c.setHP(c.HP - n)
	}
	return c.Condition
}

// Fight 打完一整場，回傳每一回合的過程。
//
// 每回合照速度順序輪一次。隊伍打 `Target` 指到的那一隻（打不到或它已經
// 倒下就退回第一個站著的，見 `PartyTarget`），怪物打隊伍第一個站著的。
//
// **與原版的差異在顆粒度不在規則**：原版逐一角色下指令，這裡一次跑完
// 整隊，所以「這一回合打哪一隻」是整隊共用的。可選目標的範圍
// （近戰前排、射擊全場、都夾在 10）走的是同一條規則。
func (e *Encounter) Fight(r *Rand, maxRounds int) []string {
	var log []string
	e.Killed, e.Lost = 0, 0
	for e.Round = 1; e.Round <= maxRounds && !e.Over(); e.Round++ {
		// 每輪開頭把怪物的特殊攻擊額度補回去（原版重設 ds:9F9E）。
		for _, m := range e.Monsters {
			if mm, ok := m.(*Monster); ok {
				mm.ResetRound()
			}
		}
		for _, c := range e.Order() {
			if !c.CombatCondition().Acts() {
				continue
			}
			// 怪物每次輪到都先擲一次「這次用不用特殊攻擊」。擲中就
			// 改發遠程／法術攻擊，這一回合不再普通攻擊；擲不中才
			// 照原路近身 —— 那一擲決定的是攻擊種類，不是行不行動。
			if m, ok := c.(*Monster); ok && m.UseSpecial(r) {
				for _, line := range e.MonsterSpecial(r, m) {
					log = append(log, fmt.Sprintf("第 %d 回合　%s", e.Round, line))
				}
				if e.Over() {
					break
				}
				continue
			}
			// 隊伍打玩家挑的那一隻（`Target`），怪物打隊伍第一個站著的。
			target := firstStanding(e.Party)
			if e.isParty(c) {
				target = e.PartyTarget()
			}
			if target == nil {
				// 前排清空但後排還在：近戰打不到，這一位這回合白站。
				if e.isParty(c) && len(e.Monsters) > 0 {
					log = append(log, fmt.Sprintf("第 %d 回合　%s 打不到後排的敵人",
						e.Round, c.CombatName()))
					continue
				}
				break
			}
			a, okA := c.(Attacker)
			d, okD := target.(Defender)
			if !okA || !okD {
				continue
			}
			party := e.isParty(c)
			if ch, ok := c.(*Character); ok && e.Ranged {
				a = NewShooter(r, ch)
			}
			res := ResolveMod(r, a, d, e.Mods(party))
			verb := PartyVerb(e.Ranged)
			if _, ok := c.(*Monster); ok {
				verb = MonsterVerb(r)
			}
			line := fmt.Sprintf("第 %d 回合　%s %s %s：%s",
				e.Round, c.CombatName(), verb, target.CombatName(), res)
			if res.Hit && res.Target == CondDead {
				line += "（倒下）"
				if m, ok := target.(*Monster); ok {
					e.recordDefeat(r, m)
					e.Killed++
				} else {
					e.Lost++
				}
			}
			log = append(log, line)
			if e.Over() {
				break
			}
		}
	}
	return log
}

func (e *Encounter) isParty(c Combatant) bool {
	for _, p := range e.Party {
		if p == c {
			return true
		}
	}
	return false
}

func firstStanding(cs []Combatant) Combatant {
	for _, c := range cs {
		if c.CombatCondition().Acts() {
			return c
		}
	}
	return nil
}

// Protection 是五條防護法術的全域計數器（`ds:03E3`–`ds:03E7`）。
//
// 清單與順序抄自原版的「Protection Spells」畫面（`sub_1A882`：五個旗標
// 指標在 `ds:136C`、五個名稱在 `ds:1376`）。前四條是「非零就生效」，
// 第五條連數值一起顯示。
type Protection struct {
	// Curse 是詛咒（ds:03DB）：**從命中值裡扣掉**，與祝福術反向。
	//
	// 怪物的特殊攻擊 case 2 每次 +1（上限 0xFF，`2COMBAT` 的 `sub_1B70C`），
	// `sub_18DAA` 的命中判定 `sub ax, cx` 扣掉它，狀態畫面印
	// `Cursed - N to Attacks!`。**神殿的捐獻會清成 0**（`sub_1C1EA`）——
	// 那就是「捐獻」真正的作用。
	Curse       int
	Bless       int // ds:03E3 祝福術：加在隊伍的命中值上
	Invisible   int // ds:03E4 隱身術：原版沒有實作效果，只計數與顯示
	Shield      int // ds:03E5 防護罩：受到的**近戰**傷害減半
	PowerShield int // ds:03E6 強力護罩：受到的傷害一律減半
	HolyBonus   int // ds:03E7 聖光加值：隊伍命中過至少一次就加進總傷害

	// MagicBonus（`ds:03D6`）與 ElementBonus（`ds:03D7`）不是防護法術，
	// 是**全隊的抗性加成**，直接加在角色自己的抗性百分比上。初值都是 1。
	// 放在這裡是因為它們與上面五個一樣，開戰時抄一份就不會再變。
	// `Values()` 不收它們 —— 防護畫面上沒有這兩項。
	MagicBonus   int
	ElementBonus int
}

// ProtectionNames 是五條的顯示名稱，順序與原版畫面一致。
var ProtectionNames = [6]string{"詛咒", "祝福術", "隱身術", "防護罩", "強力護罩", "聖光加值"}

// Values 依原版畫面的順序回傳五個值。
func (p Protection) Values() [6]int {
	return [6]int{p.Curse, p.Bless, p.Invisible, p.Shield, p.PowerShield, p.HolyBonus}
}

// Lines 是「顯示防護效能」指令要印的內容（原版指令 `P`，`sub_1A882`）。
//
// 原版只列出計數器非零的那幾條，最後一條連數值一起印。
// 詛咒排在最前面 —— 它是唯一的負面項，混在後面看不出來。
func (p Protection) Lines() []string {
	out := []string{"防護法術"}
	v := p.Values()
	for i, n := range v {
		if n == 0 {
			continue
		}
		if i == len(v)-1 {
			out = append(out, fmt.Sprintf("%s %d", ProtectionNames[i], n))
			continue
		}
		out = append(out, ProtectionNames[i])
	}
	if len(out) == 1 {
		out = append(out, "（一條都沒有）")
	}
	return out
}

// Mods 組出這一次攻擊要套的全域修正。
//
// 攻方是隊伍就吃祝福術與聖光加值；攻方是怪物就換成守方（隊伍）的
// 兩道護罩。原版是同一批全域值分別在兩條路徑上被讀，這裡只是把
// 「誰讀哪幾個」寫清楚。
func (e *Encounter) Mods(party bool) Mods {
	if party {
		// 詛咒與祝福術在同一條命中式上，方向相反
		// （`sub_18DAA`：`add ax,cx` 之後 `sub ax,cx`）。
		return Mods{
			Hit:    e.Protect.Bless - e.Protect.Curse,
			Damage: e.Protect.HolyBonus,
		}
	}
	return Mods{
		Halve:      e.Protect.PowerShield > 0,
		HalveMelee: e.Protect.Shield > 0,
		Melee:      true,
	}
}

// Exchange 是戰鬥指令 `E`（對調）：把兩名隊員在隊伍裡的位置互換。
//
// 原版走 `sub_1705A` → `2MISC2` 的 `_2misc2_e02`（thunk 的 overlay 編號 7、
// 執行時偏移 `0xC370`，減掉載入段 `0x0C13` × 16 得 overlay 內偏移 `0x240`）。
// 它問兩個索引，任一個回 `0x1B` 就整個取消；兩個都給了才動手，
// **同時搬兩個平行陣列**：
//
//	ds:0416   word  隊伍成員的記錄指標
//	ds:548C   byte  這一輪動過沒有
//
// 兩個一起搬，換過位置的人不會憑空多一次或少一次行動。回傳有沒有真的換。
//
// 換的是**戰鬥中的隊形**，不是名冊順序 —— 前面的人先挨打，所以這是
// 「把快倒的人換到後面」用的。
func (e *Encounter) Exchange(i, j int) bool {
	n := len(e.Party)
	if i < 0 || j < 0 || i >= n || j >= n || i == j {
		return false
	}
	e.Party[i], e.Party[j] = e.Party[j], e.Party[i]
	return true
}
