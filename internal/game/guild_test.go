package game_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 價格的壓縮碼：低 5 位是基數，bit5/6/7 各乘 10／100／1000。
func TestGuildPriceDecode(t *testing.T) {
	for _, tc := range []struct {
		code byte
		want int
	}{
		{10, 10},       // 0x0A
		{0x25, 50},     // 5 × 10
		{0x41, 100},    // 1 × 100
		{0x81, 1000},   // 1 × 1000
		{0xA5, 50000},  // 5 × 10 × 1000
		{0xAA, 100000}, // 10 × 10 × 1000
	} {
		if got := game.DecodeGuildPrice(tc.code); got != tc.want {
			t.Errorf("%#x 解出 %d，預期 %d", tc.code, got, tc.want)
		}
	}
}

// 五座城的貨色要隨城鎮階梯漲價 —— 這是編號與價格都解對了的旁證。
func TestGuildStockTiers(t *testing.T) {
	cheapest := func(town int) int {
		lo := 1 << 30
		for _, it := range game.GuildStockOf(town) {
			if it.Price < lo {
				lo = it.Price
			}
		}
		return lo
	}
	// 米德格特（起始城）最便宜，亞特蘭汀最貴
	if cheapest(0) >= cheapest(3) || cheapest(3) >= cheapest(1) {
		t.Errorf("價格階梯不對：米德格特 %d、佛卡尼亞 %d、亞特蘭汀 %d",
			cheapest(0), cheapest(3), cheapest(1))
	}
	for town := 0; town < 5; town++ {
		st := game.GuildStockOf(town)
		if len(st) != 4 {
			t.Fatalf("第 %d 座城賣 %d 條，預期 4 條", town, len(st))
		}
		for _, it := range st {
			if it.Spell < 0 || it.Spell > 47 {
				t.Errorf("法術編號 %d 超出巫師系的 0–47", it.Spell)
			}
			if it.Price <= 0 {
				t.Errorf("價格 %d 不合理", it.Price)
			}
		}
	}
}

// 買法術：職業、法力等級、已學、金幣四道關卡。
func TestGuildBuyGates(t *testing.T) {
	s := session(t)
	// 找一個巫師與一個非施法職業
	sorc, other := -1, -1
	for i := range s.Party {
		switch s.Party[i].Class {
		case game.Sorcerer:
			sorc = i
		case game.Knight:
			other = i
		}
	}
	if sorc < 0 || other < 0 {
		t.Skip("預設隊伍裡沒有巫師或武士")
	}
	if msg, ok := s.GuildBuy(0, other, 0); ok || !strings.Contains(msg, "guild") && !strings.Contains(msg, "公會") {
		t.Errorf("武士買得到法術：%q", msg)
	}

	c := &s.Party[sorc]
	c.Gold = 0
	if msg, ok := s.GuildBuy(0, sorc, 0); ok || !strings.Contains(msg, "錢不夠") {
		t.Errorf("沒錢也買得到：%q", msg)
	}
	c.Gold = 1000000
	msg, ok := s.GuildBuy(0, sorc, 0)
	if !ok {
		t.Fatalf("巫師有錢還買不到：%q", msg)
	}
	if !c.Knows(game.GuildStockOf(0)[0].Spell + 1) {
		t.Error("買了卻沒學會")
	}
	if _, ok := s.GuildBuy(0, sorc, 0); ok {
		t.Error("同一條法術買了兩次")
	}
}

// 神殿賣牧師系法術，只有牧師與聖騎士能買 —— 與法師公會逐條對稱。
func TestTempleSellsClericSpells(t *testing.T) {
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, cs, nil, 1)

	for town := 0; town < 5; town++ {
		st := game.TempleStockOf(town)
		if len(st) != 3 {
			t.Fatalf("第 %d 座城的神殿有 %d 條法術，原版是三條（D/E/F）", town, len(st))
		}
		for _, it := range st {
			if it.Spell < 0 || it.Spell > 47 {
				t.Errorf("法術序號 %d 超出牧師系的 0–47", it.Spell)
			}
			if it.Price <= 0 {
				t.Errorf("價格 %d 不合理", it.Price)
			}
		}
	}

	// 找一個牧師與一個非牧師非聖騎士
	cleric, other := -1, -1
	for i := range s.Party {
		switch s.Party[i].Class {
		case game.Cleric, game.Paladin:
			if cleric < 0 {
				cleric = i
			}
		default:
			if other < 0 {
				other = i
			}
		}
	}
	if cleric < 0 || other < 0 {
		t.Skip("預設隊伍裡湊不齊牧師與非牧師")
	}
	if _, ok := s.TempleBuy(0, other, 0); ok {
		t.Error("非牧師非聖騎士也買得到神殿的法術")
	}
	// 法力等級由經驗等級推（`SpellLevel`），一級的聖騎士是 0 —— 那是對的
	// 行為，但這條要驗的是職業閘門，所以先把經驗等級撐起來。
	s.Party[cleric].Gold = 1000000
	s.Party[cleric].Level = 20
	if _, ok := s.TempleBuy(0, cleric, 0); !ok {
		line, _ := s.TempleBuy(0, cleric, 0)
		t.Errorf("牧師買不到神殿的法術：%s", line)
	}
}
