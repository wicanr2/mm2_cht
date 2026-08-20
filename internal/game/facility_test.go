package game_test

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// 每級生命點數的骰子上限照手冊的職業表。
func TestHitDiceMatchManual(t *testing.T) {
	want := map[game.Class]int{
		game.Knight: 12, game.Paladin: 10, game.Archer: 10, game.Cleric: 8,
		game.Sorcerer: 6, game.Robber: 8, game.Ninja: 8, game.Barbarian: 12,
	}
	for c, n := range want {
		if got := c.HitDice(); got != n {
			t.Errorf("%v 每級生命點數上限是 %d，手冊寫 %d", c, got, n)
		}
	}
}

// 升級要擲在骰子範圍內、年齡加一、HP 補滿。
func TestTrainRaisesLevel(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	r := game.NewRand(2024)
	c := &cs[0] // 武士，每級 1–12
	if c.CanTrain() {
		t.Fatal("第一級沒經驗值就能受訓")
	}
	c.Exp = game.ExpForLevel(2, c.Class)
	if !c.CanTrain() {
		t.Fatal("經驗值夠了卻不能受訓")
	}
	lv, age, max := c.Level, c.Age, c.MaxHP
	gained, err := c.Train(r)
	if err != nil {
		t.Fatal(err)
	}
	if c.Level != lv+1 {
		t.Errorf("等級是 %d，預期 %d", c.Level, lv+1)
	}
	if c.Age != age+1 {
		t.Errorf("年齡是 %d，預期 %d", c.Age, age+1)
	}
	if gained < 1 || gained > 12 {
		t.Errorf("擲出 %d 點生命，超出武士的 1–12", gained)
	}
	if c.MaxHP != max+gained || c.HP != c.MaxHP {
		t.Errorf("HP 是 %d/%d，預期 %d/%d", c.HP, c.MaxHP, max+gained, max+gained)
	}
}

// 倒下的人不能受訓。
func TestDownedCannotTrain(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	c := &cs[0]
	c.Exp = 999999
	c.TakeDamage(c.MaxHP)
	if c.CanTrain() {
		t.Error("無意識的人可以受訓")
	}
	if _, err := c.Train(game.NewRand(1)); err == nil {
		t.Error("無意識的人受訓沒有回錯誤")
	}
}

// 法力等級照手冊的對照表：經驗等級 1、3、5… 各對應法力等級 1…9。
func TestSpellLevelThresholds(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cleric := cs[3] // 牧師
	for _, w := range []struct{ level, sl int }{
		{1, 1}, {2, 1}, {3, 2}, {5, 3}, {9, 5}, {17, 9}, {25, 9},
	} {
		cleric.Level = w.level
		if got := cleric.SpellLevel(); got != w.sl {
			t.Errorf("牧師第 %d 級的法力等級是 %d，預期 %d", w.level, got, w.sl)
		}
	}
	// 武士不會施法
	knight := cs[0]
	knight.Level = 20
	if got := knight.SpellLevel(); got != 0 {
		t.Errorf("武士的法力等級是 %d，預期 0", got)
	}
}

// 旅店休息補滿並解除無意識，但救不回死亡。
func TestRestAtInn(t *testing.T) {
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, cs, nil, 1)
	s.Party[0].TakeDamage(s.Party[0].MaxHP) // 無意識
	s.Party[1].TakeDamage(s.Party[1].MaxHP)
	s.Party[1].TakeDamage(1) // 死亡
	s.Party[2].HP = 1

	s.RestAtInn()
	if s.Party[0].Condition != game.CondGood || s.Party[0].HP != s.Party[0].MaxHP {
		t.Errorf("無意識的人休息後是 %v HP=%d", s.Party[0].Condition, s.Party[0].HP)
	}
	if s.Party[1].Condition != game.CondDead {
		t.Errorf("死亡的人被休息救回來了：%v", s.Party[1].Condition)
	}
	if s.Party[2].HP != s.Party[2].MaxHP {
		t.Errorf("受傷的人沒補滿：%d/%d", s.Party[2].HP, s.Party[2].MaxHP)
	}
}

// 打贏會給經驗，累積夠了就升得了級 —— 整條成長線要接得起來。
func TestProgressionEndToEnd(t *testing.T) {
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	ms := mons(t)
	s := game.NewSession(w, cs, ms, 555)

	// 打到經驗夠升級為止
	need := game.ExpForLevel(2, s.Party[0].Class)
	for i := 0; i < 200 && s.Party[0].Exp < need; i++ {
		e := &game.Encounter{Party: s.Combatants()}
		e.Monsters = append(e.Monsters, game.NewMonster(ms[3]))
		e.Front = len(e.Monsters)
		e.Fight(s.Rand, 50)
		if e.PartyWon() {
			e.AwardExp(s.Party)
		}
		s.RestAtInn() // 補血再打下一場
	}
	if s.Party[0].Exp < need {
		t.Fatalf("打了兩百場，經驗只有 %d，升到第二級要 %d", s.Party[0].Exp, need)
	}
	maxHP := s.Party[0].MaxHP
	log := s.TrainParty()
	if len(log) == 0 {
		t.Fatal("受訓沒有任何結果")
	}
	if s.Party[0].Level != 2 {
		t.Errorf("受訓後等級是 %d，預期 2", s.Party[0].Level)
	}
	if s.Party[0].MaxHP <= maxHP {
		t.Errorf("升級後生命上限沒增加：%d → %d", maxHP, s.Party[0].MaxHP)
	}
	t.Logf("%s", log[0])
}

// 升級經驗表對得對不對，用名冊自己驗：每個角色的經驗都必須 ≥ 他目前等級
// 的門檻（不然他不會是那個等級），而剛升級的人會**精確等於**門檻。
//
// 這個檢驗不需要 oracle —— 名冊是原版資料，門檻是從原版程式碼解出來的，
// 兩邊獨立。分段的等差只要有一段抄錯，高等級那幾筆就會落到門檻底下。
func TestExpTableAgainstRoster(t *testing.T) {
	cs, err := game.ParseCharacters(orig(t, "ROSTER.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	exact, checked := 0, 0
	for _, c := range cs {
		if c.Empty() || c.Level < 2 {
			continue
		}
		checked++
		need := game.ExpForLevel(c.Level, c.Class)
		if c.Exp < need {
			t.Errorf("%s（%v，等級 %d）的經驗 %d 低於門檻 %d",
				c.Name, c.Class, c.Level, c.Exp, need)
		}
		if c.Exp == need {
			exact++
		}
	}
	if checked < 15 {
		t.Fatalf("只檢到 %d 筆，名冊可能沒讀對", checked)
	}
	// 剛升級就沒再打的角色會停在門檻上。一筆都沒有的話，表大概是錯的。
	if exact < checked/2 {
		t.Errorf("只有 %d/%d 筆精確落在門檻上，表可能有偏移", exact, checked)
	}
}

// 兩組職業的門檻不同：武士那組從 1,500 起、遊俠那組從 2,000 起，每級加倍。
func TestExpTableGroups(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip(err)
	}
	for _, w := range []struct {
		class game.Class
		level int
		want  int
	}{
		{game.Knight, 2, 1500}, {game.Knight, 10, 384000},
		{game.Sorcerer, 2, 2000}, {game.Sorcerer, 10, 512000},
		{game.Barbarian, 5, 12000}, {game.Ninja, 5, 16000},
	} {
		if got := game.ExpForLevel(w.level, w.class); got != w.want {
			t.Errorf("%v 升到 %d 級要 %d，預期 %d", w.class, w.level, got, w.want)
		}
	}
}

// opcode 0x0e 的子命令與旁邊的招牌必須是同一類設施。
//
// 五座城鎮各有一個 `0e 01`…`0e 06`。這條掃真的資料，把「子命令 → 設施」
// 的對照釘在原版身上，而不是靠招牌字串猜。
func TestFacilityCodeMatchesSigns(t *testing.T) {
	segs, err := events.Parse(orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[game.FacilityKind]int{}
	agree, disagree := 0, 0
	for i := 0; i < 5 && i < len(segs); i++ { // 前五段是五座城鎮
		seg := &segs[i]
		for _, e := range seg.Events {
			idx := int(e.Index)
			if idx < 0 || idx >= len(seg.Scripts) {
				continue
			}
			sc := seg.Scripts[idx]
			for p := 0; p < len(sc); {
				n := game.OpLen(sc[p])
				if sc[p] == game.OpFacility && p+1 < len(sc) {
					k := game.FacilityByCode(int(sc[p+1]))
					if k == game.FacilityNone {
						break
					}
					counts[k]++
					// 找四周事件格的招牌，看是不是同一類。
					x, y := int(e.Cell)%16, int(e.Cell)/16
					for _, d := range [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} {
						nx, ny := x+d[0], y+d[1]
						if nx < 0 || nx > 15 || ny < 0 || ny > 15 {
							continue
						}
						for _, ne := range seg.Events {
							if int(ne.Cell) != ny*16+nx {
								continue
							}
							j := int(ne.Index)
							if j < 0 || j >= len(seg.Scripts) {
								continue
							}
							msg := game.ScriptMessageForTest(seg, seg.Scripts[j])
							switch sign := game.FacilityAt(msg); sign {
							case game.FacilityNone:
							case k:
								agree++
							default:
								disagree++
							}
						}
					}
				}
				if n < 1 {
					break
				}
				p += n
			}
		}
	}
	for _, k := range []game.FacilityKind{
		game.FacilityInn, game.FacilityTraining, game.FacilityTavern,
		game.FacilityTemple, game.FacilityMageGuild, game.FacilityBlacksmith,
	} {
		if counts[k] != 5 {
			t.Errorf("%v 在五座城鎮出現 %d 次，預期 5", k, counts[k])
		}
	}
	if agree == 0 {
		t.Fatal("一塊招牌都沒對上，測的東西不對")
	}
	// 招牌是自由文字，偶有鄰格串味；多數要一致。
	if disagree*4 > agree {
		t.Errorf("招牌與子命令對不上的有 %d 處，對得上的只有 %d 處", disagree, agree)
	}
	t.Logf("招牌與子命令一致 %d 處、不一致 %d 處", agree, disagree)
}

// 神殿的兩項恢復服務：狀態與陣營，各自抄自 sub_1C178 與 sub_1C1B2。
func TestTempleRestores(t *testing.T) {
	s := session(t)
	c := &s.Party[0]
	c.HP = 1
	c.Condition = game.CondDead
	c.CondBits = 0x81
	if !c.RestoreCondition() {
		t.Fatal("恢復狀態沒有回報變化")
	}
	if c.HP != c.MaxHP || c.Condition != game.CondGood || c.CondBits != 0 {
		t.Errorf("恢復之後：生命 %d/%d 狀況 %v 位元 %#x",
			c.HP, c.MaxHP, c.Condition, c.CondBits)
	}
	if c.RestoreCondition() {
		t.Error("已經好了還回報有變化")
	}

	// 陣營：把原始的抄回當前的
	orig := byte(c.Align)
	other := (orig + 1) % 3
	c.SetFieldByte(106, 0, other)
	if !c.RestoreAlignment() {
		t.Fatal("恢復陣營沒有回報變化")
	}
	if got := c.FieldByte(106); got != orig {
		t.Errorf("當前陣營是 %d，預期抄回原始的 %d", got, orig)
	}
	if c.RestoreAlignment() {
		t.Error("陣營已經是原始值還回報有變化")
	}
}

// 神殿的四項服務都要有回應，不能靜悄悄。
func TestTempleServeAlwaysReports(t *testing.T) {
	s := session(t)
	for k := game.TempleRestoreCond; k <= game.TempleLeave; k++ {
		if lines := s.Serve(k); len(lines) == 0 {
			t.Errorf("服務 %v 沒有任何回應", game.TempleServiceNames[k])
		}
	}
}

// 大腦淨化清掉兩項第二技能，並把技能給的加值扣回去。
//
// 加值表讀自 `sub_1C5CA` 的跳表 —— 它在清除時扣什麼，就等於當初給了什麼。
func TestDetoxRemovesSkillsAndBonuses(t *testing.T) {
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, cs, nil, 1)
	c := &s.Party[0]
	c.Gold = 500

	// 武器專家（1，準確度 +5）與戰士（15，耐力 +5）。
	c.Skills = [2]int{1, 15}
	c.Raw[80] = 1<<4 | 15
	acc0, end0 := c.Current[game.Accuracy], c.Endurance

	lines := s.Detox(0)
	if len(lines) == 0 {
		t.Fatal("沒有回報")
	}
	if c.Skills != [2]int{0, 0} || c.Raw[80] != 0 {
		t.Errorf("技能沒清掉：%v（+80 = %d）", c.Skills, c.Raw[80])
	}
	if got := c.Current[game.Accuracy]; got != acc0-5 {
		t.Errorf("武器專家的準確度加值沒扣回去：%d → %d", acc0, got)
	}
	if c.Endurance != end0-5 {
		t.Errorf("戰士的耐力加值沒扣回去：%d → %d", end0, c.Endurance)
	}

	// 黃金不足時什麼都不做（原版只驗不扣，所以驗完黃金也不會少）。
	c2 := &s.Party[1]
	c2.Gold = 99
	c2.Skills = [2]int{7, 0}
	c2.Raw[80] = 7 << 4
	before := c2.Current[game.Might]
	s.Detox(1)
	if c2.Skills[0] != 7 || c2.Current[game.Might] != before {
		t.Errorf("黃金不足卻還是動了：技能 %v，力量 %d → %d",
			c2.Skills, before, c2.Current[game.Might])
	}
}

// 競技賽要有入場券；有券就收走並排出「每名隊員各一隻」的對手。
func TestArenaNeedsTicket(t *testing.T) {
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
	s := game.NewSession(w, cs, ms, 7)

	// 背包裡沒有券
	for i := range s.Party {
		for slot := 0; slot < 6; slot++ {
			s.Party[i].SetFieldByte(58+slot, 0x00, 0)
		}
	}
	if e := s.EnterArena(); e.Ready {
		t.Error("沒有入場券卻報名成功了")
	}

	// 黑券（211）在第三個人的背包裡
	s.Party[2].SetFieldByte(58+1, 0x00, game.ArenaTicketLast)
	e := s.EnterArena()
	if !e.Ready || e.Tier != 3 {
		t.Fatalf("黑券應該是第 3 階且可以開打，得到 %+v", e)
	}
	if got := s.Party[2].Backpack()[1].ID; got == game.ArenaTicketLast {
		t.Error("券沒有被收走")
	}
	enc := s.ArenaEncounter(e.Tier)
	if enc == nil {
		t.Fatal("排不出對手")
	}
	if len(enc.Monsters) != len(s.Party) {
		t.Errorf("對手 %d 隻，隊伍 %d 人 —— 原版是一人一隻",
			len(enc.Monsters), len(s.Party))
	}
	first := enc.Monsters[0].CombatName()
	for _, m := range enc.Monsters {
		if m.CombatName() != first {
			t.Errorf("對手不是同一種：%q 與 %q", first, m.CombatName())
			break
		}
	}
}

// 獎金只給第一個人，獎章全隊都點（原版加完金額就清成 0 再繼續迴圈）。
func TestArenaRewardGoldAndBadge(t *testing.T) {
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, cs, nil, 1)
	gold0 := make([]int, len(s.Party))
	for i := range s.Party {
		gold0[i] = s.Party[i].Gold
	}
	lines := s.ArenaReward(0) // 第 0 階、地圖 0 → 200 金
	if len(lines) == 0 {
		t.Fatal("沒有回報獎金")
	}
	if s.Party[0].Gold != gold0[0]+200 {
		t.Errorf("第一個人的黃金 %d → %d，預期 +200", gold0[0], s.Party[0].Gold)
	}
	for i := 1; i < len(s.Party); i++ {
		if s.Party[i].Gold != gold0[i] {
			t.Errorf("第 %d 個人也拿到錢了：%d → %d", i+1, gold0[i], s.Party[i].Gold)
		}
	}
	// 獎章：記錄 +0x79+5 = +126 的 bit0
	for i := range s.Party {
		if s.Party[i].Raw[126]&1 == 0 {
			t.Errorf("第 %d 個人沒拿到獎章位元", i+1)
			break
		}
	}
}

// 神殿三項服務的價錢：基數 × 等級 × 城鎮倍率（捐獻不乘等級）。
func TestTemplePrices(t *testing.T) {
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	s := game.NewSession(w, cs, nil, 1)
	c := &s.Party[0]
	c.BattleLevel = 3

	// 完全健康 → 0（原版拿 0 當「不提供」的旗標）
	c.HP = c.MaxHP
	c.CondBits = 0
	if got := s.TemplePrice(game.TempleRestoreCond, 0); got != 0 {
		t.Errorf("完全健康卻要收 %d", got)
	}
	// 生命沒滿 → 基數 10
	c.HP = c.MaxHP - 1
	if got := s.TemplePrice(game.TempleRestoreCond, 0); got != 10*3*1 {
		t.Errorf("受傷的價錢是 %d，預期 10×等級 3×米德格特倍率 1 = 30", got)
	}
	// 死亡（≥ 0x80）→ 基數 100
	c.CondBits = 0x81
	if got := s.TemplePrice(game.TempleRestoreCond, 0); got != 100*3 {
		t.Errorf("死亡的價錢是 %d，預期 300", got)
	}
	// 最嚴重（0xFF）→ 基數 1000
	c.CondBits = 0xFF
	if got := s.TemplePrice(game.TempleRestoreCond, 0); got != 1000*3 {
		t.Errorf("0xFF 的價錢是 %d，預期 3000", got)
	}
	// 捐獻不乘等級：100 × 城鎮倍率
	if got := s.TemplePrice(game.TempleDonate, 0); got != 100 {
		t.Errorf("米德格特的捐獻是 %d，預期 100", got)
	}
	// 亞特蘭汀的倍率是 5
	s.World.MapIndex = 1
	if got := s.TemplePrice(game.TempleDonate, 0); got != 500 {
		t.Errorf("亞特蘭汀的捐獻是 %d，預期 500", got)
	}
}

// 第一級的生命與法力是查表不是擲骰（`sub_18624`）。
func TestStartingHPAndSP(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip(err)
	}
	mk := func(class game.Class, attr [game.NumAttrs]int) game.Character {
		var n game.NewCharacter
		n.Attr = attr
		n.SetClass(class)
		n.SetRace(0)
		n.SetAlign(0)
		n.SetSex(0)
		n.Name = "T"
		c, ok := n.Finish()
		if !ok {
			t.Fatalf("%v 建不起來", class)
		}
		return c
	}
	// 野蠻人耐力 21：ds:6E6[7]=12 加 ds:6F2[21]=7 → 19
	b := mk(game.Barbarian, [game.NumAttrs]int{15, 10, 10, 21, 10, 10, 10})
	if b.MaxHP != 19 {
		t.Errorf("野蠻人（耐力 21）第一級生命 %d，預期 12+7=19", b.MaxHP)
	}
	// 巫師智慧 21：ds:6E6[4]=3 加耐力 10 的 3 → 6 生命；法力 ds:71E[21]=7
	so := mk(game.Sorcerer, [game.NumAttrs]int{10, 21, 10, 10, 10, 10, 10})
	if so.MaxHP != 3+3 {
		t.Errorf("巫師第一級生命 %d，預期 3+3=6", so.MaxHP)
	}
	if so.MaxSP != 7 {
		t.Errorf("巫師（智慧 21）第一級法力 %d，預期 7", so.MaxSP)
	}
	if so.SL != 1 {
		t.Errorf("巫師第一級的法力等級是 %d，預期 1", so.SL)
	}
	if so.SpellsKnown[0] != 0x3A {
		t.Errorf("巫師起手的已學法術是 %#x，預期 0x3A", so.SpellsKnown[0])
	}
	// 武士沒有法力
	k := mk(game.Knight, [game.NumAttrs]int{18, 10, 10, 10, 10, 10, 10})
	if k.MaxSP != 0 || k.SL != 0 {
		t.Errorf("武士不該有法力：SP %d SL %d", k.MaxSP, k.SL)
	}
}

// `0e 08` 是競技賽自己的入口 —— 三座城的地圖格上各有一個。
// 先前只認 1–7，那三格會被當成「這格沒有設施」。
func TestFacilityCodeEightIsArena(t *testing.T) {
	if got := game.FacilityByCode(8); got != game.FacilityArena {
		t.Errorf("0e 08 → %v，預期競技場", got)
	}
	// 9 以上仍然不是一般設施。
	if got := game.FacilityByCode(9); got != game.FacilityNone {
		t.Errorf("0e 09 → %v，預期不是一般設施", got)
	}
}

// 休息一次做七件事（`2MISC sub_1CD8A`）。這一份逐條驗。
//
// 為什麼要逐條：先前 remake 的休息只做了「清旅行效果」與「補滿生命法力」
// 兩件半，而**少做的每一件都讓旅店變便宜** —— 食物不扣、疾病照樣回血、
// 中毒不罰、狀況一次清乾淨。缺的東西在畫面上看不出來，只有對著原版的
// 流程逐條問才問得出來。
func restSession(t *testing.T) *game.Session {
	t.Helper()
	w, err := game.NewWorld(orig(t, "MAP.DAT"), orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	return game.NewSession(w, cs, nil, 1)
}

// 食物：每次休息扣一格；**沒有食物就整個不回復**。
func TestRestEatsFood(t *testing.T) {
	s := restSession(t)
	c := &s.Party[0]
	c.Food, c.HP = 5, 1
	s.RestAtInn()
	if c.Food != 4 {
		t.Errorf("休息後食物 %d，該是 4", c.Food)
	}
	if c.HP != c.MaxHP {
		t.Errorf("有食物卻沒補滿：%d/%d", c.HP, c.MaxHP)
	}

	c.Food, c.HP = 0, 1
	s.RestAtInn()
	if c.Food != 0 {
		t.Errorf("沒有食物卻扣成 %d", c.Food)
	}
	if c.HP != 1 {
		t.Errorf("沒有食物卻補到 %d", c.HP)
	}
}

// 疾病：食物照扣，生命不回。
func TestRestDiseasedDoesNotHeal(t *testing.T) {
	s := restSession(t)
	c := &s.Party[0]
	c.CondBits = game.CondBitDiseased
	c.Food, c.HP = 5, 1
	s.RestAtInn()
	if c.Food != 4 {
		t.Errorf("生病的人食物 %d，該照扣成 4", c.Food)
	}
	if c.HP != 1 {
		t.Errorf("生病的人生命補到 %d，該留在 1", c.HP)
	}
	// 疾病本身留著 —— `and 0Dh` 收的三個位元之一。
	if c.CondBits&game.CondBitDiseased == 0 {
		t.Error("休息把疾病清掉了")
	}
}

// 中毒：**有效上限直接減半，而且不先抄基礎值** —— 連續休息會一路砍下去。
func TestRestPoisonedHalvesMaxHP(t *testing.T) {
	s := restSession(t)
	c := &s.Party[0]
	c.CondBits = game.CondBitPoisoned
	c.Food = 9
	before := c.MaxHP
	s.RestAtInn()
	if c.MaxHP != before/2 {
		t.Errorf("中毒休息一次上限 %d → %d，該減半成 %d", before, c.MaxHP, before/2)
	}
	half := c.MaxHP
	s.RestAtInn()
	if c.MaxHP != half/2 {
		t.Errorf("再休息一次上限 %d → %d，該再減半", half, c.MaxHP)
	}
	// 解毒之後才抄回基礎值。
	c.CondBits = 0
	s.RestAtInn()
	if c.MaxHP != c.BaseMaxHP {
		t.Errorf("解毒後休息上限是 %d，該抄回基礎值 %d", c.MaxHP, c.BaseMaxHP)
	}
}

// 狀況位元 `and 0Dh`：**只留詛咒、疾病、中毒**。
func TestRestClearsAllButThreeConditions(t *testing.T) {
	s := restSession(t)
	c := &s.Party[0]
	c.Food = 9
	c.CondBits = game.CondBitCursed | game.CondBitSilenced | game.CondBitDiseased |
		game.CondBitPoisoned | game.CondBitAsleep | game.CondBitParalyzed |
		game.CondBitUnconscious
	s.RestAtInn()
	want := byte(game.CondBitCursed | game.CondBitDiseased | game.CondBitPoisoned)
	if c.CondBits != want {
		t.Errorf("休息後狀況是 %#02x，該只剩 %#02x", c.CondBits, want)
	}
}

// 死亡與石化（`>= 0x80`）整個跳過 —— 手冊也是這樣寫的。
func TestRestSkipsSevereConditions(t *testing.T) {
	s := restSession(t)
	c := &s.Party[0]
	c.CondBits = 0x81
	c.Food, c.HP = 5, 0
	s.RestAtInn()
	if c.CondBits != 0x81 {
		t.Errorf("死亡的人狀況變成 %#02x", c.CondBits)
	}
	if c.Food != 5 {
		t.Errorf("死亡的人也被扣食物：%d", c.Food)
	}
	if c.HP != 0 {
		t.Errorf("死亡的人生命被補成 %d", c.HP)
	}
}

// 八十歲之後每次休息都有近一半的機率老死；七十九歲不會。
func TestRestOldAgeKills(t *testing.T) {
	s := restSession(t)
	c := &s.Party[0]
	c.Food = 200
	c.Age = 79
	for i := 0; i < 50; i++ {
		s.RestAtInn()
	}
	if c.CondBits&game.CondBitSevere != 0 {
		t.Fatalf("七十九歲休息 50 次就死了：%#02x", c.CondBits)
	}
	c.Age = 80
	died := false
	for i := 0; i < 50 && !died; i++ {
		s.RestAtInn()
		died = c.CondBits&game.CondBitSevere != 0
	}
	if !died {
		t.Error("八十歲休息 50 次一次都沒死 —— 老死那一擲沒接上")
	}
}

// 法力上限 = `等級 × (屬性修正 + 3)`，人格為主、巫師與弓箭手看智慧。
// **只有基礎法力等級非零的人才重算。**
func TestRestRecomputesMaxSP(t *testing.T) {
	s := restSession(t)
	var caster, plain *game.Character
	for i := range s.Party {
		c := &s.Party[i]
		if c.Empty() {
			continue
		}
		if c.FieldByte(35) != 0 && caster == nil {
			caster = c
		}
		if c.FieldByte(35) == 0 && plain == nil {
			plain = c
		}
	}
	if caster == nil || plain == nil {
		t.Skip("預設隊伍裡沒有同時有施法者與非施法者")
	}
	caster.Food, plain.Food = 9, 9
	plainBefore := plain.MaxSP
	caster.MaxSP = 1
	s.RestAtInn()

	stat := caster.Base[game.Personality]
	if caster.Class == game.Sorcerer || caster.Class == game.Archer {
		stat = caster.Base[game.Intellect]
	}
	bonus := testData(t).StatBonus(stat)
	if bonus < 0 {
		bonus = 0
	}
	if want := caster.Level * (bonus + 3); caster.MaxSP != want {
		t.Errorf("%s 的法力上限是 %d，該是 %d（等級 %d × (修正 %d + 3)）",
			caster.Name, caster.MaxSP, want, caster.Level, bonus)
	}
	if caster.SP != caster.MaxSP {
		t.Errorf("休息後目前法力 %d 沒有補到上限 %d", caster.SP, caster.MaxSP)
	}
	if plain.MaxSP != plainBefore {
		t.Errorf("不會施法的人法力上限被改成 %d（原本 %d）", plain.MaxSP, plainBefore)
	}
}

// 休息會把**當前值抄回基礎值** —— 增減益一起洗掉，
// 包括魔法滑梯陷阱砍掉的那一半（`sub_13572` 加呼叫端補的耐力與運氣）。
func TestRestResetsCurrentStats(t *testing.T) {
	s := restSession(t)
	c := &s.Party[0]
	c.Food = 9
	for i := game.Stat(0); i < game.NumStats; i++ {
		c.Current[i] = 1
	}
	c.BattleLevel = 1
	s.RestAtInn()
	for i := game.Stat(0); i < game.NumStats; i++ {
		if c.Current[i] != c.Base[i] {
			t.Errorf("屬性 %d 當前 %d 沒抄回基礎 %d", i, c.Current[i], c.Base[i])
		}
	}
	if c.BattleLevel != c.Level {
		t.Errorf("戰鬥等級 %d 沒抄回 %d", c.BattleLevel, c.Level)
	}
}
