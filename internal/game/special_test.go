package game_test

import (
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// specialMonster 造一隻只用來發動指定招式的怪。
//
// `Tier` 平常由解包從編號算出來（`編號 >> 4`），這裡是手工組的記錄，
// 要自己填 —— 群體骰的顆數就是它，忘了填會變成 0 顆骰子。
func specialMonster(code, index, hp int) *game.Monster {
	return game.NewMonster(monsters.Monster{
		Index: index, Name: "測試怪", HP: hp, Tier: index >> 4,
		SpecialIndex: code, SpecialChance: 99, SpecialUses: 3,
		Attacks: 3, DamageDice: 6,
	})
}

// specialParty 拿六個預設角色當靶。用真的記錄是為了讓寫回 `Raw`
// 的那一段也跑到 —— 只有結構欄位的假角色驗不出存檔會不會同步。
func specialParty(t *testing.T) []game.Combatant {
	t.Helper()
	cs, err := game.ParseCharacters(orig(t, "DEFAULT.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	out := make([]game.Combatant, 0, len(cs))
	for i := range cs {
		c := &cs[i]
		c.HP, c.MaxHP = 9999, 9999
		out = append(out, c)
	}
	return out
}

func specialEncounter(t *testing.T, m *game.Monster) *game.Encounter {
	t.Helper()
	return &game.Encounter{Party: specialParty(t), Monsters: []game.Combatant{m}, Front: 1}
}

// 三十二種全部跑得動，而且播報第一行一定帶招式名。
//
// 這一條擋的是「跳表少接一個 case」：少接的那一種在遊戲裡的症狀是
// 怪物宣告了招式卻什麼都沒發生，畫面上看不出來。
func TestSpecialAllCodesRun(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	for code := 0; code < 32; code++ {
		def, ok := game.Special(code)
		if !ok {
			t.Fatalf("碼 %d 查不到定義", code)
		}
		m := specialMonster(code, 0x80, 40)
		e := specialEncounter(t, m)
		log := e.MonsterSpecial(game.NewRand(uint16(code*7+1)), m)
		if len(log) == 0 {
			t.Errorf("碼 %d（%s）一行播報都沒有", code, def.Announce)
			continue
		}
		if !strings.Contains(log[0], def.Announce) {
			t.Errorf("碼 %d 的第一行是 %q，沒有招式名 %q", code, log[0], def.Announce)
		}
	}
}

// ` casts` 只印在碼 15–30、29 除外。吐息與凝視不念咒，狂暴夾在中間也不念。
func TestSpecialCastsPrefix(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	for code := 0; code < 32; code++ {
		def, _ := game.Special(code)
		m := specialMonster(code, 0x80, 40)
		e := specialEncounter(t, m)
		log := e.MonsterSpecial(game.NewRand(9), m)
		// 碼 2 的招式名本身就叫 `casts a curse`，所以不能只找 "casts"，
		// 要比整句：念咒的形狀是「名字 casts 招式!」。
		want := m.CombatName() + " " + def.Announce + "!"
		if code >= 15 && code <= 30 && code != 29 {
			want = m.CombatName() + " casts " + def.Announce + "!"
		}
		if log[0] != want {
			t.Errorf("碼 %d 的播報是 %q，預期 %q", code, log[0], want)
		}
	}
}

// 吐息的威力就是這隻怪目前的 HP，編號 0xB3 以下折半。
//
// 抗性會往下削（幸運通道減半、元素通道除以四），所以驗的是**上界**：
// 多擲幾次，最大值要正好等於未被抗性削過的那個數。
func TestSpecialBreathDamageIsMonsterHP(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	for _, tc := range []struct{ index, hp, want int }{
		{0x40, 60, 30}, // 編號 < 0xB3：折半
		{0xC0, 60, 60}, // 編號 >= 0xB3：不打折
		{0x40, 1, 1},   // 折半歸零要補回 1
	} {
		best := 0
		for seed := 0; seed < 200; seed++ {
			m := specialMonster(3, tc.index, tc.hp) // breathes fire
			e := specialEncounter(t, m)
			for _, line := range e.MonsterSpecial(game.NewRand(uint16(seed)), m) {
				if n, ok := damageOf(line); ok && n > best {
					best = n
				}
			}
		}
		if best != tc.want {
			t.Errorf("編號 %02X、HP %d 的吐息最大傷害 %d，預期 %d",
				tc.index, tc.hp, best, tc.want)
		}
	}
}

// 群體骰的顆數是怪物編號的高 4 位，不是另一張表。
//
// 碼 18（fireballs）是 `Tier` 顆 6 面骰、總和再乘二加一。
// **上界擲不到**（十顆全六是六千萬分之一），所以驗三件事：
// 不超過上界、Tier 0 只有 1（一顆骰都沒擲）、顆數多的一定打得更痛。
func TestSpecialDiceCountIsTier(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	best := map[int]int{}
	for _, index := range []int{0x00, 0x30, 0xA0} {
		for seed := 0; seed < 300; seed++ {
			m := specialMonster(18, index, 40)
			e := specialEncounter(t, m)
			for _, line := range e.MonsterSpecial(game.NewRand(uint16(seed)), m) {
				if n, ok := damageOf(line); ok && n > best[index] {
					best[index] = n
				}
			}
		}
		if limit := (index>>4)*6*2 + 1; best[index] > limit {
			t.Errorf("編號 %02X（%d 顆骰）最大傷害 %d，超過上界 %d",
				index, index>>4, best[index], limit)
		}
	}
	if best[0x00] != 1 {
		t.Errorf("Tier 0 的最大傷害是 %d，一顆骰都沒擲該只有 1", best[0x00])
	}
	if !(best[0x30] < best[0xA0]) {
		t.Errorf("3 顆骰打到 %d、10 顆骰打到 %d，顆數沒跟著編號走",
			best[0x30], best[0xA0])
	}
}

// 一次群體攻擊最多打到六個位置，而且同一次裡每個位置最多一次。
func TestSpecialSpreadStaysWithinParty(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	for seed := 0; seed < 100; seed++ {
		m := specialMonster(18, 0x50, 40)
		e := specialEncounter(t, m)
		hit := map[string]int{}
		for _, line := range e.MonsterSpecial(game.NewRand(uint16(seed)), m) {
			if _, ok := damageOf(line); !ok {
				continue
			}
			hit[strings.SplitN(line, " ", 2)[0]]++
		}
		if len(hit) > len(e.Party) {
			t.Fatalf("種子 %d 打到 %d 個位置，隊伍只有 %d 人", seed, len(hit), len(e.Party))
		}
		for name, n := range hit {
			if n > 1 {
				t.Fatalf("種子 %d：%s 在同一次裡挨了 %d 下", seed, name, n)
			}
		}
	}
}

// 抽魔力把全隊目前 SP 清成 0，抽法力等級每人減一（0 的不動）。
func TestSpecialDrains(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	e := specialEncounter(t, specialMonster(11, 0x50, 40))
	before := 0
	for _, p := range e.Party {
		c := p.(*game.Character)
		c.SP, c.SL = 20, 3
		before += c.SP
	}
	if before == 0 {
		t.Fatal("測試前提沒建立起來：全隊 SP 都是 0")
	}
	e.MonsterSpecial(game.NewRand(3), e.Monsters[0].(*game.Monster))
	for _, p := range e.Party {
		if c := p.(*game.Character); c.SP != 0 {
			t.Errorf("%s 的 SP 是 %d，抽魔力該清成 0", c.Name, c.SP)
		}
	}

	e2 := specialEncounter(t, specialMonster(12, 0x50, 40))
	for i, p := range e2.Party {
		p.(*game.Character).SL = i // 第 0 個是 0，驗「0 不再往下減」
	}
	e2.MonsterSpecial(game.NewRand(3), e2.Monsters[0].(*game.Monster))
	for i, p := range e2.Party {
		want := i - 1
		if i == 0 {
			want = 0
		}
		if c := p.(*game.Character); c.SL != want {
			t.Errorf("第 %d 位的法力等級是 %d，預期 %d", i, c.SL, want)
		}
	}
}

// 詛咒每發動一次加一，加到 0xFF 就不再加。
func TestSpecialCurseAccumulates(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	e := specialEncounter(t, specialMonster(2, 0x50, 40))
	m := e.Monsters[0].(*game.Monster)
	for i := 0; i < 3; i++ {
		e.MonsterSpecial(game.NewRand(uint16(i)), m)
	}
	if e.Protect.Curse != 3 {
		t.Errorf("詛咒值是 %d，發動三次該是 3", e.Protect.Curse)
	}
	e.Protect.Curse = 0xFF
	e.MonsterSpecial(game.NewRand(1), m)
	if e.Protect.Curse != 0xFF {
		t.Errorf("詛咒值溢位到 %d，上限該是 0xFF", e.Protect.Curse)
	}
}

// 自爆（碼 9）與狂暴（碼 29）打完自己就倒下。
func TestSpecialSelfDestruct(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	for _, code := range []int{9, 29} {
		m := specialMonster(code, 0x50, 40)
		e := specialEncounter(t, m)
		e.MonsterSpecial(game.NewRand(5), m)
		if m.CombatCondition() != game.CondDead {
			t.Errorf("碼 %d 打完之後怪物還活著（%v）", code, m.CombatCondition())
		}
		if e.Killed != 1 {
			t.Errorf("碼 %d 的 Killed 是 %d，預期 1", code, e.Killed)
		}
	}
}

// 上狀況那一路真的會把位元寫進狀況位元組。
//
// 抗性是逐人各擲各的，六個人不會全部擋下 —— 多擲幾個種子，
// 至少要有一次成功。
func TestSpecialStatusSetsBits(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	for _, tc := range []struct {
		code int
		bit  byte
	}{
		{16, 0x10}, // sleep
		{28, 0x02}, // silence
		{30, 0x20}, // paralyze
		{10, 0x82}, // gazes：石化
	} {
		ok := false
		for seed := 0; seed < 40 && !ok; seed++ {
			m := specialMonster(tc.code, 0x50, 40)
			e := specialEncounter(t, m)
			e.MonsterSpecial(game.NewRand(uint16(seed)), m)
			for _, p := range e.Party {
				if c := p.(*game.Character); c.CondBits&tc.bit == tc.bit {
					ok = true
				}
			}
		}
		if !ok {
			t.Errorf("碼 %d 擲了四十次都沒有人被上到 %#02x", tc.code, tc.bit)
		}
	}
}

// 蒸發財物只在擲值大於 65 時發作，發作就整隊一起沒錢。
func TestSpecialVaporizeIsAllOrNothing(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	fired, spared := 0, 0
	for seed := 0; seed < 60; seed++ {
		m := specialMonster(13, 0x50, 40)
		e := specialEncounter(t, m)
		for _, p := range e.Party {
			p.(*game.Character).Gold = 500
		}
		e.MonsterSpecial(game.NewRand(uint16(seed)), m)
		zero := 0
		for _, p := range e.Party {
			if p.(*game.Character).Gold == 0 {
				zero++
			}
		}
		switch zero {
		case 0:
			spared++
		case len(e.Party):
			fired++
		default:
			t.Fatalf("種子 %d：%d/%d 人沒錢，這一招是全隊一起", seed, zero, len(e.Party))
		}
	}
	if fired == 0 || spared == 0 {
		t.Errorf("六十次裡發作 %d 次、沒事 %d 次，機率門檻沒生效", fired, spared)
	}
}

// 戰鬥迴圈真的會發動特殊攻擊，而且那一回合不再普通攻擊。
func TestFightFiresSpecials(t *testing.T) {
	if err := game.EnsureData(); err != nil {
		t.Skip("沒有 data/：", err)
	}
	m := specialMonster(18, 0x50, 400) // fireballs，必定使用
	m.Def.Speed = 250
	e := specialEncounter(t, m)
	log := e.Fight(game.NewRand(11), 3)
	hit := false
	for _, line := range log {
		if strings.Contains(line, "fireballs") || strings.Contains(line, "火球") {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("三回合打下來一次特殊攻擊都沒有：\n%s", strings.Join(log, "\n"))
	}
}

// damageOf 從一行播報裡抓出傷害數字。抓不到就回 false ——
// 播報有三種形狀，只有「takes N pts」那一種帶數字。
func damageOf(line string) (int, bool) {
	n, seen := 0, false
	for _, ch := range line {
		switch {
		case ch >= '0' && ch <= '9':
			n = n*10 + int(ch-'0')
			seen = true
		case seen:
			return n, true
		}
	}
	return n, seen
}
