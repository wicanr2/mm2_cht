package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/items"
	"github.com/wicanr2/mm2_cht/internal/game"
)

func itemTable(t *testing.T) []items.Item {
	t.Helper()
	tbl, err := items.Parse(orig(t, "ITEMS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	return tbl
}

// session 組一個帶地圖、隊伍與物品表的 Session。
func session(t *testing.T) *game.Session {
	t.Helper()
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, cs, mons(t), 99)
	s.Items = itemTable(t)
	return s
}

// 物品的 +15 欄要對得上物品的名字 —— 這是這個欄位是「附帶法術」的證據。
func TestItemUseSpellMatchesNames(t *testing.T) {
	items := itemTable(t)
	want := map[string]string{
		"Torch":        "照明術",
		"Lantern":      "照明術",
		"Compass":      "定位術",
		"Sextant":      "定位術",
		"Wakeup Horn":  "喚醒術",
		"Slumber Club": "催眠術",
		"Antidote Ale": "解毒術",
		"Water Talon":  "洪水陣",
		"Fire Talon":   "烈火陣",
		"Earth Talon":  "后土陣",
		"Air Talon":    "狂風陣",
	}
	for _, it := range items {
		w, ok := want[it.Name]
		if !ok {
			continue
		}
		n, ok := it.UseSpell()
		if !ok {
			t.Errorf("%s 的 +15 是 %#x，不是法術型", it.Name, it.Use)
			continue
		}
		idx, ok := game.ItemSpellToEngine(n)
		if !ok {
			t.Errorf("%s 的法術編號 %d 超出範圍", it.Name, n)
			continue
		}
		sp, ok := game.SpellByEngineIndex(idx)
		if !ok {
			t.Errorf("%s 查不到編號 %d 的法術", it.Name, idx)
			continue
		}
		if sp.Name != w {
			t.Errorf("%s 放的是「%s」，預期「%s」", it.Name, sp.Name, w)
		}
	}
}

// 使用物品要扣充能，用光的那一次把欄位填成 0xFF。
func TestUseItemSpendsCharges(t *testing.T) {
	s := session(t)
	c := &s.Party[0]
	// 找一件法術型的東西塞進背包第一格
	var id int
	for i, it := range s.Items {
		if _, ok := it.UseSpell(); ok {
			id = i
			break
		}
	}
	if id == 0 {
		t.Fatal("物品表裡沒有法術型的東西")
	}
	slot := game.EquippedSlots // 背包第一格
	c.Items[slot] = game.ItemSlot{ID: id, Charge: 2}

	r := s.UseItem(0, slot)
	if r.Err != game.UseOK {
		t.Fatalf("第一次就失敗了：%v", r.Err)
	}
	if c.Items[slot].Charge != 1 {
		t.Errorf("充能剩 %d，預期 1", c.Items[slot].Charge)
	}

	r = s.UseItem(0, slot)
	if !r.UsedUp {
		t.Error("用光了卻沒回報")
	}
	if c.Items[slot].ID != 0xFF {
		t.Errorf("用光的物品編號是 %d，原版填 0xFF", c.Items[slot].ID)
	}
	// 背包那一支用光的那一次仍然發動（原版 loc_1B95D fall through）
	if r.Err != game.UseOK {
		t.Errorf("背包的最後一次不該失敗：%v", r.Err)
	}

	// 0xFF 是物品表的第 255 號，名字就叫 Useless Item（Use = 0），
	// 所以用光之後再選它得到的是「沒有特殊能力」而不是「次數用盡」——
	// 原版在 sub_1BA18 也是先擋 +15 再看充能。
	if got := s.Items[c.Items[slot].ID].Name; got != "Useless Item" {
		t.Errorf("用光的欄位指到「%s」，預期 Useless Item", got)
	}
	if r := s.UseItem(0, slot); r.Err != game.UseNoPower {
		t.Errorf("用光後再用回 %v，預期 UseNoPower", r.Err)
	}
}

// 已裝備的最後一次有扣沒有用 —— 原版 loc_1B9C0 直接跳出去。
func TestUseItemEquippedLastChargeWasted(t *testing.T) {
	s := session(t)
	c := &s.Party[0]
	var id int
	for i, it := range s.Items {
		if _, ok := it.UseSpell(); ok {
			id = i
			break
		}
	}
	c.Items[0] = game.ItemSlot{ID: id, Charge: 1}
	r := s.UseItem(0, 0)
	if !r.UsedUp {
		t.Fatal("沒回報用光")
	}
	if r.Spell != 0 || r.Effect != "" {
		t.Errorf("已裝備的最後一次不該發動效果：法術 %d「%s」", r.Spell, r.Effect)
	}
}

// 沒有特殊能力的東西不能用。
func TestUseItemNoSpecialPower(t *testing.T) {
	s := session(t)
	var id int
	for i, it := range s.Items {
		if i > 0 && it.Use == 0 && it.Name != "" && it.Name != "BLANK" {
			id = i
			break
		}
	}
	s.Party[0].Items[0] = game.ItemSlot{ID: id, Charge: 5}
	if r := s.UseItem(0, 0); r.Err != game.UseNoPower {
		t.Errorf("回 %v，預期 UseNoPower", r.Err)
	}
}

// 28 件非魔法型物品都要走到原版 sub_1BBAE 的其中一格，不可再掉進未解分支。
func TestEveryNonSpellItemUseEffect(t *testing.T) {
	wantCount := map[byte]int{
		0x02: 1,
		0x1A: 2, 0x1F: 4,
		0x2F: 2,
		0x3A: 1, 0x3F: 2,
		0x55: 2, 0x5A: 4, 0x5C: 1, 0x5F: 8,
		0x61: 1,
	}
	seen := map[byte]int{}
	total := 0
	for id, def := range itemTable(t) {
		if def.Use == 0 || def.Use >= 0x80 {
			continue
		}
		total++
		if _, ok := wantCount[def.Use]; !ok {
			t.Errorf("物品 %d %q 的非魔法效果碼 %#02x 不在已驗證集合", id, def.Name, def.Use)
			continue
		}
		seen[def.Use]++

		s := session(t)
		c := &s.Party[0]
		slot := game.EquippedSlots
		beforeMax := c.MaxHP
		beforeMight := c.Current[game.Might]
		beforeSpeed := c.Current[game.Speed]
		beforeAccuracy := c.Current[game.Accuracy]
		beforeBattle := c.BattleLevel
		beforeSL := c.SL
		c.Items[slot] = game.ItemSlot{ID: id, Charge: 2}

		r := s.UseItem(0, slot)
		if r.Err != game.UseOK {
			t.Errorf("%q 使用失敗：%v", def.Name, r.Err)
			continue
		}
		if r.SpellUsed {
			t.Errorf("%q 是非魔法效果卻標成施法", def.Name)
		}
		if r.Effect == "" {
			t.Errorf("%q 沒有非魔法效果播報", def.Name)
		}
		if c.Items[slot].Charge != 1 {
			t.Errorf("%q 使用後充能是 %d，預期 1", def.Name, c.Items[slot].Charge)
		}

		amount := int(def.Use & 0x0F)
		switch def.Use >> 4 {
		case 0:
			want := beforeMax&0x00FF | ((beforeMax>>8)+amount)<<8
			if c.MaxHP != want {
				t.Errorf("%q 生命上限 %d，預期 %d", def.Name, c.MaxHP, want)
			}
		case 1:
			if c.Current[game.Might] != beforeMight+amount {
				t.Errorf("%q 力量 %d，預期 %d", def.Name, c.Current[game.Might], beforeMight+amount)
			}
		case 2:
			if c.Current[game.Speed] != beforeSpeed+amount {
				t.Errorf("%q 速度 %d，預期 %d", def.Name, c.Current[game.Speed], beforeSpeed+amount)
			}
		case 3:
			if c.Current[game.Accuracy] != beforeAccuracy+amount {
				t.Errorf("%q 準確度 %d，預期 %d", def.Name, c.Current[game.Accuracy], beforeAccuracy+amount)
			}
		case 5:
			if c.BattleLevel != beforeBattle+amount {
				t.Errorf("%q 戰鬥等級 %d，預期 %d", def.Name, c.BattleLevel, beforeBattle+amount)
			}
		case 6:
			if c.SL != beforeSL+amount {
				t.Errorf("%q 法力等級 %d，預期 %d", def.Name, c.SL, beforeSL+amount)
			}
		default:
			t.Errorf("%q 的效果高 nibble %#x 沒有實際資料對照", def.Name, def.Use>>4)
		}
	}
	if total != 28 {
		t.Errorf("非魔法型物品共 %d 件，預期 28", total)
	}
	for code, want := range wantCount {
		if got := seen[code]; got != want {
			t.Errorf("效果碼 %#02x 有 %d 件，預期 %d", code, got, want)
		}
	}
}

func itemIDWithUse(t *testing.T, s *game.Session, use byte) int {
	t.Helper()
	for id, def := range s.Items {
		if def.Use == use {
			return id
		}
	}
	t.Fatalf("找不到效果碼 %#02x 的物品", use)
	return -1
}

// 原版的效果量先以 byte 回捲，再作飽和加法；背包最後一次使用時 Attr 已清零。
func TestNonSpellUseByteRulesAndSaveRoundTrip(t *testing.T) {
	s := session(t)
	c := &s.Party[0]
	slot := game.EquippedSlots
	force := itemIDWithUse(t, s, 0x1A)

	c.Current[game.Might] = 15
	c.Items[slot] = game.ItemSlot{ID: force, Charge: 2, Attr: 3}
	if r := s.UseItem(0, slot); r.Err != game.UseOK {
		t.Fatalf("Force Potion 使用失敗：%v", r.Err)
	}
	if got := c.Current[game.Might]; got != 28 {
		t.Errorf("Force Potion +3 後力量是 %d，預期 28", got)
	}
	if got := c.FieldByte(107); got != 28 {
		t.Errorf("Force Potion 沒同步當前力量原始欄位：%d", got)
	}
	if got := c.FieldByte(64); got != 1 {
		t.Errorf("Force Potion 沒同步背包充能原始欄位：%d", got)
	}

	c.Current[game.Might] = 250
	c.Items[slot] = game.ItemSlot{ID: force, Charge: 2, Attr: 0xFF}
	if r := s.UseItem(0, slot); r.Err != game.UseOK {
		t.Fatalf("回捲測試使用失敗：%v", r.Err)
	}
	if got := c.Current[game.Might]; got != 255 {
		t.Errorf("0xff + 0x0a 應先回捲成 9 再飽和，力量是 %d", got)
	}

	maxPotion := itemIDWithUse(t, s, 0x02)
	base, before := c.BaseMaxHP, c.MaxHP
	c.Items[slot] = game.ItemSlot{ID: maxPotion, Charge: 1, Attr: 0x7F}
	r := s.UseItem(0, slot)
	if r.Err != game.UseOK || !r.UsedUp {
		t.Fatalf("MaxHP Potion 最後一次結果：%+v", r)
	}
	if got, want := c.MaxHP, before&0x00FF|((before>>8)+2)<<8; got != want {
		t.Errorf("最後一次背包使用應先清 Attr，生命上限 %d，預期 %d", got, want)
	}
	if c.BaseMaxHP != base {
		t.Errorf("MaxHP Potion 改了基礎上限：%d → %d", base, c.BaseMaxHP)
	}
	if got := c.FieldByte(58); got != 0xFF {
		t.Errorf("用光的物品沒有同步回原始欄位：%#02x", got)
	}
	back, err := game.ParseCharacters(c.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if back[0].BaseMaxHP != base || back[0].MaxHP != c.MaxHP {
		t.Errorf("存檔往返後基礎／有效上限是 %d／%d，預期 %d／%d",
			back[0].BaseMaxHP, back[0].MaxHP, base, c.MaxHP)
	}
	if got := back[0].Items[slot]; got.ID != 0xFF || got.Attr != 0 {
		t.Errorf("用光物品的存檔欄位是 %+v，預期 Useless Item 且 Attr=0", got)
	}
}

func TestMaxHPPotionLifecycleAtFacilities(t *testing.T) {
	s := session(t)
	c := &s.Party[0]
	slot := game.EquippedSlots
	maxPotion := itemIDWithUse(t, s, 0x02)
	base := c.BaseMaxHP
	c.Items[slot] = game.ItemSlot{ID: maxPotion, Charge: 2}
	if r := s.UseItem(0, slot); r.Err != game.UseOK {
		t.Fatal(r.Err)
	}
	boosted := c.MaxHP
	if boosted == base {
		t.Fatal("MaxHP Potion 沒有建立有效上限")
	}

	// 神殿只有在目前生命低於基礎上限時才清掉暫時上限。
	c.CondBits = game.CondBitPoisoned
	c.Condition = game.CondPoisoned
	c.HP = base
	c.RestoreCondition()
	if c.MaxHP != boosted {
		t.Errorf("生命未低於基礎值時神殿清掉有效上限：%d → %d", boosted, c.MaxHP)
	}
	c.HP = 1
	c.RestoreCondition()
	if c.HP != base || c.MaxHP != base {
		t.Errorf("受傷後神殿結果是 HP=%d/%d，預期 %d/%d", c.HP, c.MaxHP, base, base)
	}

	c.Items[slot] = game.ItemSlot{ID: maxPotion, Charge: 2}
	if r := s.UseItem(0, slot); r.Err != game.UseOK {
		t.Fatal(r.Err)
	}
	c.HP = 1
	s.RestAtInn()
	if c.HP != base || c.MaxHP != base {
		t.Errorf("休息後結果是 HP=%d/%d，預期 %d/%d", c.HP, c.MaxHP, base, base)
	}

	c.Items[slot] = game.ItemSlot{ID: maxPotion, Charge: 2}
	if r := s.UseItem(0, slot); r.Err != game.UseOK {
		t.Fatal(r.Err)
	}
	baseBefore, maxBefore, hpBefore := c.BaseMaxHP, c.MaxHP, c.HP
	c.Exp = game.ExpForLevel(c.Level+1, c.Class)
	gained, err := c.Train(game.NewRand(42))
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseMaxHP != baseBefore+gained || c.MaxHP != maxBefore+gained || c.HP != hpBefore+gained {
		t.Errorf("受訓後基礎／有效／目前生命是 %d／%d／%d，預期 %d／%d／%d",
			c.BaseMaxHP, c.MaxHP, c.HP,
			baseBefore+gained, maxBefore+gained, hpBefore+gained)
	}
}

func TestSkillPotionResetsAtEndCombat(t *testing.T) {
	s := session(t)
	c := &s.Party[0]
	slot := game.EquippedSlots
	skillPotion := itemIDWithUse(t, s, 0x55)
	c.Items[slot] = game.ItemSlot{ID: skillPotion, Charge: 2}
	if r := s.UseItem(0, slot); r.Err != game.UseOK {
		t.Fatal(r.Err)
	}
	if c.BattleLevel != c.Level+5 {
		t.Fatalf("Skill Potion 後戰鬥等級是 %d，預期 %d", c.BattleLevel, c.Level+5)
	}
	s.EndCombat()
	if c.BattleLevel != c.Level || c.FieldByte(113) != byte(c.Level) {
		t.Errorf("戰鬥結束後等級是 %d／原始欄位 %d，預期都回到 %d",
			c.BattleLevel, c.FieldByte(113), c.Level)
	}
}
