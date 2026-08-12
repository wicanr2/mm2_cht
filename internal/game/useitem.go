package game

import "fmt"

// 使用物品：戰鬥中的指令 `U`，戰鬥外走 `2CMDS` 的同一套。
//
// 兩支的形狀一模一樣（`2COMBAT.img` 的 `sub_1BA18` 與 `2CMDS.img` 的
// `sub_1CED8`）：問「用哪一個」、檢查該欄非空、扣一次充能、依物品記錄
// `+15` 決定發生什麼。
//
// 原版的提示是 `Use Which (A-F)/(1-6)?`（`ds:14CC`），**字母是背包、
// 數字是已裝備** —— `sub_1BA18` 拿 `'A'` 去讀 `記錄[+58]` 那一組，
// 拿 `'1'` 去讀 `記錄[+40]` 那一組。

// UseError 是使用失敗的原因，值對應 `sub_1B89E` 的訊息編號。
type UseError int

const (
	UseOK          UseError = 0
	UseNoPower     UseError = 0x0F // "No special power"：物品 +15 是 0
	UseNoCharges   UseError = 0x10 // "No charges"：充能已經是 0
	UseEmptySlot   UseError = -1   // 那一欄是空的，原版連選都不讓你選
	UseUnknownKind UseError = -2   // 損毀的法術效果碼，正常的 ITEMS.DAT 不會到這裡
)

var useErrText = map[UseError]string{
	UseNoPower:     "沒有特殊能力",
	UseNoCharges:   "次數用盡",
	UseEmptySlot:   "那一欄是空的",
	UseUnknownKind: "這件東西的效果碼無效",
}

func (e UseError) Error() string {
	if s, ok := useErrText[e]; ok {
		return s
	}
	return "不能使用"
}

// UseResult 是使用一件物品的結果。
type UseResult struct {
	Err UseError
	// Spell 是發動的法術（remake 編號 0–95）；只在 SpellUsed 時有意義。
	Spell     int
	SpellUsed bool
	// Effect 是法術或非魔法效果的播報。
	Effect string
	// Spent 是這次用掉之後還剩幾次。
	Spent int
	// UsedUp 為真表示這一次把它用光了，物品欄被填成 0xFF。
	UsedUp bool
}

// ItemSpellToEngine 把物品記錄 `+15` 的法術編號換成 remake 的編號。
//
// 原版是巫師 0–47、牧師 48–95（`SPELLS.DAT` 與兩張施法跳表的順序）；
// remake 內部把兩半對調過（見 SpellIndex 的說明），所以要換一次。
// 換錯不會報錯，只會放出另一條法術 —— 火球術變成治傷術這種。
func ItemSpellToEngine(orig int) (int, bool) {
	switch {
	case orig < 0 || orig > 95:
		return 0, false
	case orig < 48: // 原版的巫師系
		return 48 + orig, true
	default: // 原版的牧師系
		return orig - 48, true
	}
}

// UseItem 讓第 who 名隊員使用第 slot 欄的東西（0–5 已裝備、6–11 背包）。
//
// 原版的流程（`sub_1B92E` 已裝備／`sub_1B9A4` 背包）：
//
//  1. 充能欄是 0 → `No charges`，什麼都不發生
//  2. 充能減一；減到 0 → 物品編號填 `0xFF`、屬性欄清 0
//  3. 物品 `+15 >= 0x80` → 施放法術，編號 `(+15 & 0x7F) - 1`
//  4. 否則由 `sub_1BBAE` 依高 nibble 改寫角色的一個 byte
//
// **第 2 步的兩支不對稱**：背包那一支減到 0 時仍然發動效果（fall through
// 到 `loc_1B95D`），已裝備那一支減到 0 時直接 `jmp` 出去，最後一次
// 有扣沒有用。照抄。
func (s *Session) UseItem(who, slot int) UseResult {
	if who < 0 || who >= len(s.Party) || slot < 0 || slot >= itemSlots {
		return UseResult{Err: UseEmptySlot}
	}
	c := &s.Party[who]
	it := &c.Items[slot]
	if it.Empty() {
		return UseResult{Err: UseEmptySlot}
	}
	def, ok := s.itemDef(it.ID)
	if !ok || def.Use == 0 {
		return UseResult{Err: UseNoPower}
	}
	if it.Charge == 0 {
		return UseResult{Err: UseNoCharges}
	}

	it.Charge--
	res := UseResult{Spent: int(it.Charge)}
	defer c.syncItemSlot(slot)
	equipped := slot < EquippedSlots
	if it.Charge == 0 {
		res.UsedUp = true
		it.ID = 0xFF
		it.Attr = 0
		if equipped {
			// 已裝備的最後一次不發動 —— 原版就是這樣（`loc_1B9C0`）。
			return res
		}
	}

	if def.Use < 0x80 {
		res.Effect = c.applyNonSpellUse(def.Use, it.Attr)
		return res
	}
	n, _ := def.UseSpell()
	idx, ok := ItemSpellToEngine(n)
	if !ok {
		res.Err = UseUnknownKind
		return res
	}
	res.Spell, res.SpellUsed = idx, true
	res.Effect = s.applyEffect(idx, who)
	return res
}

// applyNonSpellUse 實作原版的 sub_1BBAE（戰鬥）與 sub_1CF34（戰鬥外）。
//
// 兩支先算 amount = byte(物品欄屬性 + (Use & 0x0f))，所以低位元的相加
// 會在 8 位元處回捲；再以 root 的 +0x3608 飽和加到由高三位選出的目標。
// 物品欄最後一次使用時已先清掉 Attr，呼叫端傳入的正是那個清零後的值。
func (c *Character) applyNonSpellUse(use, attr byte) string {
	amount := int(byte(int(attr) + int(use&0x0F)))
	switch (use >> 4) & 7 {
	case 0: // +0x75：有效生命上限 +0x74 的高位元組
		before := c.MaxHP
		high := clampByte((before >> 8) + amount)
		c.MaxHP = before&0x00FF | high<<8
		if len(c.Raw) == RecordSize {
			c.Raw[offMaxHP] = byte(c.MaxHP)
			c.Raw[offMaxHP+1] = byte(c.MaxHP >> 8)
		}
		return fmt.Sprintf("生命上限 %d → %d", before, c.MaxHP)
	case 1:
		return c.addNonSpellStat(Might, amount)
	case 2:
		return c.addNonSpellStat(Speed, amount)
	case 3:
		return c.addNonSpellStat(Accuracy, amount)
	case 4: // +0x6a：目前陣營
		before := 0
		if len(c.Raw) == RecordSize {
			before = int(c.Raw[offCurAlign])
			c.Raw[offCurAlign] = byte(clampByte(before + amount))
		}
		return fmt.Sprintf("目前陣營值 %d → %d", before, clampByte(before+amount))
	case 5: // +0x71：戰鬥等級
		before := c.BattleLevel
		c.BattleLevel = clampByte(before + amount)
		if len(c.Raw) == RecordSize {
			c.Raw[offBattleLevel] = byte(c.BattleLevel)
		}
		return fmt.Sprintf("戰鬥等級 %d → %d", before, c.BattleLevel)
	case 6: // +0x72：法力等級
		before := c.SL
		c.SL = clampByte(before + amount)
		if len(c.Raw) == RecordSize {
			c.Raw[offSL] = byte(c.SL)
		}
		return fmt.Sprintf("法力等級 %d → %d", before, c.SL)
	case 7: // +0x58：目前法力的低位元組
		before := c.SP
		low := clampByte((before & 0x00FF) + amount)
		c.SP = before&0xFF00 | low
		if len(c.Raw) == RecordSize {
			c.Raw[offSP] = byte(c.SP)
		}
		return fmt.Sprintf("法力 %d → %d", before, c.SP)
	}
	return ""
}

func (c *Character) addNonSpellStat(st Stat, amount int) string {
	before := c.Current[st]
	c.Current[st] = clampByte(before + amount)
	if len(c.Raw) == RecordSize {
		c.Raw[offCur+int(st)] = byte(c.Current[st])
	}
	return fmt.Sprintf("%s %d → %d", st, before, c.Current[st])
}

// syncItemSlot 把一次使用後的三個平行欄位同步到原始記錄，避免下一個
// 事件直接改 Raw 時把剛扣掉的充能重新解析回來。
func (c *Character) syncItemSlot(slot int) {
	if slot < 0 || slot >= itemSlots || len(c.Raw) != RecordSize {
		return
	}
	it := c.Items[slot]
	if slot < slotsPerSet {
		c.Raw[offEquipID+slot] = byte(it.ID)
		c.Raw[offEquipCharge+slot] = it.Charge
		c.Raw[offEquipAttr+slot] = it.Attr
		return
	}
	i := slot - slotsPerSet
	c.Raw[offPackID+i] = byte(it.ID)
	c.Raw[offPackCharge+i] = it.Charge
	c.Raw[offPackAttr+i] = it.Attr
}

// itemDef 查物品表。
func (s *Session) itemDef(id int) (def itemDefinition, ok bool) {
	if id < 0 || id >= len(s.Items) {
		return def, false
	}
	it := s.Items[id]
	return itemDefinition{Use: it.Use, Name: it.Name}, true
}

type itemDefinition struct {
	Use  byte
	Name string
}

// UseSpell 回傳這件東西附帶的法術編號（原版編號，0 起算）。
func (d itemDefinition) UseSpell() (int, bool) {
	if d.Use < 0x80 {
		return 0, false
	}
	return int(d.Use&0x7F) - 1, true
}

// SpellByEngineIndex 依引擎編號（0–95）取法術。
func SpellByEngineIndex(i int) (Spell, bool) {
	if data == nil || i < 0 || i >= len(data.Spells) {
		return Spell{}, false
	}
	return data.Spells[i], true
}
