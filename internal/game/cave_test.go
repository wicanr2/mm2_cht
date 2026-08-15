package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// caveSession 開一場帶原版資料的遊戲；沒有原版資料就跳過。
func caveSession(t *testing.T) *game.Session {
	t.Helper()
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	ms, err := monsters.Parse(orig(t, "MONSTERS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	return game.NewSession(w, cs, ms, 7)
}

// 捐獻的換算：黃金 1:1、寶石一顆十點。原版 `2CAVES` 的 `e04`／`e06`。
func TestDonateExchangeRates(t *testing.T) {
	s := caveSession(t)
	c := &s.Party[0]
	c.SetFieldValue(102, 4, 250)
	c.SetFieldValue(92, 2, 7)
	c.SetFieldValue(98, 4, 100)

	if lines := s.TradeGoldForExp(1); len(lines) == 0 {
		t.Fatal("換黃金沒有回話")
	}
	if c.Exp != 350 || c.Gold != 0 {
		t.Errorf("黃金換經驗後 Exp=%d Gold=%d，預期 350／0", c.Exp, c.Gold)
	}
	if lines := s.DonateGemsForExp(1); len(lines) == 0 {
		t.Fatal("捐寶石沒有回話")
	}
	if c.Exp != 420 || c.Gems != 0 {
		t.Errorf("寶石換經驗後 Exp=%d Gems=%d，預期 420（+70）／0", c.Exp, c.Gems)
	}
}

// 身上沒有東西時不能換，也不能把經驗變成負的。
func TestDonateNothingToGive(t *testing.T) {
	s := caveSession(t)
	c := &s.Party[0]
	c.SetFieldValue(102, 4, 0)
	c.SetFieldValue(92, 2, 0)
	c.SetFieldValue(98, 4, 500)
	s.TradeGoldForExp(1)
	s.DonateGemsForExp(1)
	if c.Exp != 500 {
		t.Errorf("空手捐獻改了經驗：%d", c.Exp)
	}
	if lines := s.TradeGoldForExp(99); len(lines) == 0 {
		t.Error("選了不存在的位置應該要有回話")
	}
}

// 年代之門：選項 1–4 只傳送，5–8 才改世紀。
func TestEraGateCentury(t *testing.T) {
	opts := game.EraOptions()
	if len(opts) != 8 {
		t.Fatalf("年代選項有 %d 個，預期 8", len(opts))
	}
	for i, o := range opts {
		want := 0
		if i+1 >= 5 {
			want = i + 1
		}
		if o.Century != want {
			t.Errorf("選項 %d 的世紀是 %d，預期 %d", i+1, o.Century, want)
		}
	}

	s := caveSession(t)
	// 沒有旗標就開不了門。
	if s.EraGateOpen() {
		t.Error("預設隊伍不該開得了年代之門")
	}
	s.Party[0].SetFieldByte(128, 0xFF, 0x02)
	if !s.EraGateOpen() {
		t.Fatal("設了 +128 bit 1 之後應該開得了")
	}

	s.EnterEra(6)
	if got := s.World.MapIndex; got != 37 {
		t.Errorf("選第 6 個年代後地圖是 %d，預期 37", got)
	}
	if s.World.X != 5 || s.World.Y != 5 {
		t.Errorf("座標是 (%d,%d)，預期 (5,5)", s.World.X, s.World.Y)
	}
	// 世紀要讓事件 opcode `0x22` 讀得到。
	if got := s.World.Global(0x84); got != 6 {
		t.Errorf("世紀是 %d，預期 6", got)
	}

	// 選項 4 只傳送不改世紀。
	s.EnterEra(4)
	if got := s.World.Global(0x84); got != 6 {
		t.Errorf("選項 4 不該改世紀，卻變成 %d", got)
	}
	if s.World.MapIndex != 40 {
		t.Errorf("選項 4 的地圖是 %d，預期 40", s.World.MapIndex)
	}
}

// 座標傳送機不換地圖，而且擋掉範圍外的座標。
func TestMagicLocation(t *testing.T) {
	s := caveSession(t)
	s.World.MapIndex = 3
	if !s.MagicLocation(11, 4) {
		t.Fatal("(11,4) 應該是合法座標")
	}
	if s.World.MapIndex != 3 {
		t.Errorf("座標傳送機換了地圖：%d", s.World.MapIndex)
	}
	if s.World.X != 11 || s.World.Y != 4 {
		t.Errorf("座標是 (%d,%d)，預期 (11,4)", s.World.X, s.World.Y)
	}
	for _, bad := range [][2]int{{-1, 0}, {16, 0}, {0, 16}} {
		if s.MagicLocation(bad[0], bad[1]) {
			t.Errorf("(%d,%d) 不該被接受", bad[0], bad[1])
		}
	}
}

// 滑梯陷阱：座標對上才發動，發動就把「當前值」鏡像與 SP 全部砍半。
//
// 用腳本驅動而不是直接呼叫內部函式 —— 要驗的是 `0e 80` 這條路真的接上了。
func TestSlideTrapHalvesCurrentValues(t *testing.T) {
	s := caveSession(t)
	w := s.World
	// slideFrom 的第一組是 (1,13)。
	w.X, w.Y = 1, 13
	c := &s.Party[0]
	c.SetFieldValue(88, 2, 40) // 目前 SP
	c.SetFieldByte(113, 0, 12) // 戰鬥等級
	c.SetFieldByte(115, 0, 18) // 耐力
	before := c.Current[game.Might]

	w.RunScriptForTest([]byte{0x0e, 0x80})

	if w.X != 1 || w.Y != 14 {
		t.Fatalf("滑梯後座標是 (%d,%d)，預期 (1,14)", w.X, w.Y)
	}
	if !w.Teleported {
		t.Error("滑梯應該算一次傳送")
	}
	if got := c.SP; got != 20 {
		t.Errorf("SP 是 %d，預期 20", got)
	}
	if got := c.BattleLevel; got != 6 {
		t.Errorf("戰鬥等級是 %d，預期 6", got)
	}
	if got := c.Endurance; got != 9 {
		t.Errorf("耐力是 %d，預期 9", got)
	}
	if got := c.Current[game.Might]; got != before/2 {
		t.Errorf("當前力量是 %d，預期 %d", got, before/2)
	}
	// 基礎值不動 —— 砍的是當前值那一份。
	if c.Base[game.Might] == c.Current[game.Might] && before != 0 {
		t.Error("基礎力量也被砍了")
	}
}

// 座標沒對上就什麼都不做。挑一個確定不在表上的位置。
func TestSlideTrapNeedsMatchingCell(t *testing.T) {
	s := caveSession(t)
	w := s.World
	w.X, w.Y = 0, 0
	before := s.Party[0].Current[game.Might]
	w.RunScriptForTest([]byte{0x0e, 0x80})
	if w.X != 0 || w.Y != 0 {
		t.Errorf("不該移動，卻跑到 (%d,%d)", w.X, w.Y)
	}
	if got := s.Party[0].Current[game.Might]; got != before {
		t.Errorf("不該扣值，力量 %d → %d", before, got)
	}
}

// 三倍泉：全隊的黃金乘三變成經驗。
func TestTripleGold(t *testing.T) {
	s := caveSession(t)
	for i := range s.Party {
		s.Party[i].SetFieldValue(102, 4, 100)
		s.Party[i].SetFieldValue(98, 4, 5)
	}
	s.World.RunScriptForTest([]byte{0x0e, 0xCC})
	if s.World.Device != game.DeviceTripleGold {
		t.Fatalf("`0e CC` 應該是三倍泉，得到 %v", s.World.Device)
	}
	s.TripleGold()
	for i := range s.Party {
		if s.Party[i].Exp != 305 || s.Party[i].Gold != 0 {
			t.Fatalf("第 %d 人 Exp=%d Gold=%d，預期 305／0",
				i+1, s.Party[i].Exp, s.Party[i].Gold)
		}
	}
}

// 架子從對應的編號區段擲一件出來，放進第一個空背包格。
func TestRackGivesItemInRange(t *testing.T) {
	s := caveSession(t)
	for i := range s.Party {
		for slot := 0; slot < 6; slot++ {
			s.Party[i].SetFieldByte(58+slot, 0, 0)
		}
	}
	for _, tc := range []struct {
		d        game.CaveDevice
		lo, hi   int
	}{
		{game.DeviceRack1, 66, 78},
		{game.DeviceRack2, 92, 97},
		{game.DeviceRack3, 127, 133},
	} {
		for i := range s.Party {
			for slot := 0; slot < 6; slot++ {
				s.Party[i].SetFieldByte(58+slot, 0, 0)
			}
		}
		if lines := s.TakeFromRack(tc.d); len(lines) == 0 {
			t.Fatalf("%v 沒有發東西", tc.d)
		}
		got := int(s.Party[0].FieldByte(58))
		if got < tc.lo || got > tc.hi {
			t.Errorf("%v 給了物品 %d，超出 %d–%d", tc.d, got, tc.lo, tc.hi)
		}
	}
}

// 背包全滿就什麼都不發（原版找不到空位就直接返回）。
func TestRackNeedsEmptySlot(t *testing.T) {
	s := caveSession(t)
	for i := range s.Party {
		for slot := 0; slot < 6; slot++ {
			s.Party[i].SetFieldByte(58+slot, 0, 1)
		}
	}
	if lines := s.TakeFromRack(game.DeviceRack1); len(lines) != 0 {
		t.Errorf("背包全滿卻發了東西：%v", lines)
	}
}

// 馬戲團：有旗標必贏並加 10 點，旗標用掉；沒旗標就是輸。
func TestCircusWinConsumesFlag(t *testing.T) {
	s := caveSession(t)
	c := &s.Party[0]
	if s.CircusWins() {
		t.Fatal("預設隊伍不該帶著必贏旗標")
	}
	c.SetFieldByte(125, 0xFF, 0x02)
	if !s.CircusWins() {
		t.Fatal("設了 +125 bit 1 之後應該必贏")
	}
	before := int(c.FieldByte(16)) // 力量
	s.PlayCircus(0)                // 力量試驗
	if got := int(c.FieldByte(16)); got != before+10 {
		t.Errorf("力量 %d → %d，預期 +10", before, got)
	}
	if s.CircusWins() {
		t.Error("旗標沒有被用掉")
	}
}

// 上限 100：原版是「大於 90 就設成 100」，不是加到爆。
func TestCircusStatCap(t *testing.T) {
	s := caveSession(t)
	c := &s.Party[0]
	c.SetFieldByte(125, 0xFF, 0x02)
	c.SetFieldByte(16, 0, 95)
	s.PlayCircus(0)
	if got := int(c.FieldByte(16)); got != 100 {
		t.Errorf("95 玩一次之後是 %d，預期 100", got)
	}
}

// 生命上限重算：沒到頂又有錢才算，而且原版不扣錢。
func TestRecomputeMaxHP(t *testing.T) {
	s := caveSession(t)
	c := &s.Party[0]
	want := game.MaxHPTarget(c)
	if want <= 0 {
		t.Skip("沒有屬性表，跳過")
	}
	c.SetFieldValue(96, 2, 1) // 基礎上限壓低
	c.SetFieldValue(102, 4, 10)
	s.RecomputeMaxHP(1)
	if c.BaseMaxHP != 1 {
		t.Errorf("黃金不夠卻改了上限：%d", c.BaseMaxHP)
	}
	c.SetFieldValue(102, 4, 15*65536)
	s.RecomputeMaxHP(1)
	if c.BaseMaxHP != want || c.MaxHP != want {
		t.Errorf("重算後 %d／%d，預期 %d", c.BaseMaxHP, c.MaxHP, want)
	}
	if c.Gold != 15*65536 {
		t.Errorf("原版不扣錢，黃金卻變成 %d", c.Gold)
	}
	// 已經到頂就不動。
	s.RecomputeMaxHP(1)
	if c.BaseMaxHP != want {
		t.Errorf("到頂之後又被改成 %d", c.BaseMaxHP)
	}
}

// `0e 7F` 擲出來的怪，編號要落在隊伍所在那一列的十六隻裡。
func TestFightRollRow(t *testing.T) {
	s := caveSession(t)
	s.World.Y = 3
	enc := s.FightRoll()
	if enc == nil {
		t.Skip("這一列沒有可用的怪物")
	}
	if len(enc.Monsters) != len(s.Party) {
		t.Errorf("排了 %d 隻，預期與隊伍同數 %d", len(enc.Monsters), len(s.Party))
	}
}

// 每日笑話：`今天 mod 22` 挑一則，key 是那一組的第一筆。
func TestJokeOfTheDay(t *testing.T) {
	s := caveSession(t)
	if s.World.Globals == nil {
		s.World.Globals = map[uint16]byte{}
	}
	s.World.Globals[0x03CA] = 0 // 世紀索引 0 → 讀 ds:03A2
	for _, tc := range []struct {
		day  byte
		want string
	}{{0, "str.000"}, {1, "str.004"}, {21, "str.084"}, {22, "str.000"}, {23, "str.004"}} {
		s.World.Globals[0x03A2] = tc.day
		if got := s.JokeOfTheDay(); got != tc.want {
			t.Errorf("第 %d 天的笑話是 %s，預期 %s", tc.day, got, tc.want)
		}
	}
	// 走 `0e E2` 這條路要把裝置認出來。
	s.World.RunScriptForTest([]byte{0x0e, 0xE2})
	if s.World.Device != game.DeviceJoke {
		t.Errorf("`0e E2` 是 %v，預期每日笑話", s.World.Device)
	}
}
