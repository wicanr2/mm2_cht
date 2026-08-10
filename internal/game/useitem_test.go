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
