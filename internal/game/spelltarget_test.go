package game_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
)

// healerSession 組一個「有牧師、法力與寶石都夠、會傷痛術」的場面。
func healerSession(t *testing.T) (*game.Session, int) {
	t.Helper()
	s := session(t)
	for i := range s.Party {
		// **不會施法的職業也回得出「系」** —— `SpellSchoolOf` 沒有
		// 「不會」這個值，所以要先問 `CanCast`，否則會挑到武士。
		if game.CanCast(s.Party[i].Class) &&
			game.SpellSchoolOf(s.Party[i].Class) == game.SchoolCleric {
			// **順序有意義**：`SetFieldByte` 會改記錄再重新解析，
			// 先設好的 SP 會被檔案裡的值蓋回去。
			s.Party[i].SetFieldByte(114, 0x00, 9) // 法力等級 9
			s.Party[i].Learn(11)
			s.Party[i].SP, s.Party[i].MaxSP, s.Party[i].Gems = 99, 99, 99
			return s, i
		}
	}
	t.Skip("預設隊伍裡沒有牧師系")
	return nil, -1
}

// easyFoe 挑一隻不抗魔法、不抗屬性的怪 —— 抗性會讓「打到沒有」與
// 「打錯隻」長得一模一樣。
func easyFoe(t *testing.T, defs []monsters.Monster) monsters.Monster {
	t.Helper()
	for i := range defs {
		d := defs[i]
		if d.Name != "" && d.MagicResistIndex == 0 && d.Index <= 3 {
			clear := true
			for _, r := range d.Resists {
				if r {
					clear = false
				}
			}
			if clear {
				return d
			}
		}
	}
	t.Skip("找不到沒有抗性的怪物")
	return monsters.Monster{}
}

// 單體攻擊法術要打玩家挑的那一隻，不是陣列裡的第一隻。
//
// 原版在戰鬥中施單體法術會問 `On which (A-J)?`，範圍是場上全部
// （2026-08-17 實機量到，見 `docs/research/spell-interaction-oracle.md`）。
func TestSpellTargetHitsThatMonster(t *testing.T) {
	s, who := healerSession(t)
	defs := mons(t)
	d := easyFoe(t, defs)
	foes := []game.Combatant{game.NewMonster(d), game.NewMonster(d), game.NewMonster(d)}
	s.Fight = &game.Encounter{Party: s.Combatants(), Monsters: foes,
		Front: len(foes), Flags: map[uint16]byte{}}
	s.Fight.SpellTarget = 2

	before := make([]int, len(foes))
	for i, m := range foes {
		before[i] = m.CombatHP()
	}
	// 10 ＝ 傷痛術，`spellMonsterCount` 裡是 1 隻。
	if got := game.SpellPromptFor(10).Kind; got != game.SpellPromptMonster {
		t.Fatalf("傷痛術的提示是 %v，應該要問打哪一隻", got)
	}
	if r := s.Cast(who, 11); !r.OK { // 牧師系第 11 條 ＝ 引擎編號 10
		t.Fatalf("傷痛術施不出來：%s", r.Reason)
	}

	for i, m := range foes {
		hurt := m.CombatHP() < before[i]
		if (i == 2) != hurt {
			t.Errorf("第 %d 隻 hurt=%v（挑的是第 2 隻）", i, hurt)
		}
	}
}

// 挑中的那一隻已經倒下時退回原本的掃描順序 —— 目標在「挑」與「施」
// 之間失效是常態（怪物會死），不是特例。
func TestSpellTargetFallsBackWhenDowned(t *testing.T) {
	s, who := healerSession(t)
	d := easyFoe(t, mons(t))
	foes := []game.Combatant{game.NewMonster(d), game.NewMonster(d)}
	s.Fight = &game.Encounter{Party: s.Combatants(), Monsters: foes,
		Front: len(foes), Flags: map[uint16]byte{}}
	foes[1].TakeDamage(foes[1].CombatHP()) // 打倒第 1 隻
	s.Fight.SpellTarget = 1

	before := foes[0].CombatHP()
	if r := s.Cast(who, 11); !r.OK {
		t.Fatalf("傷痛術施不出來：%s", r.Reason)
	}
	if foes[0].CombatHP() >= before {
		t.Errorf("挑中的那隻倒了，法術應該落到還站著的第 0 隻")
	}
}

// 多目標法術不問目標 —— 原版從第 0 隻往後掃。
func TestMultiTargetSpellsDoNotAsk(t *testing.T) {
	for _, idx := range []int{20, 27, 90, 94, 0, 13} {
		if got := game.SpellPromptFor(idx).Kind; got == game.SpellPromptMonster {
			t.Errorf("法術 %d 是多目標，不該問打哪一隻", idx)
		}
	}
}

// spellMonsterCount 與 spellEffects 那邊的 count 參數必須一致。
//
// 兩張表分開放（一張是效果、一張是「打幾隻」），靠讀 `cast.go` 的語法樹
// 比對 —— 改了一邊忘了另一邊，這條會紅。**手寫的效果不列**
// （`gravity`／`prismatic`／`turnUndead`），它們都不是單體。
func TestMonsterCountsMatchEffects(t *testing.T) {
	// 建構式的 count 在第幾個引數。
	argOf := map[string]int{
		"damageSpell":      2,
		"damageSpellLo":    3,
		"levelDamageSpell": 2,
		"fixedDamageSpell": 1,
		"statusSpell":      1,
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "cast.go", nil, 0)
	if err != nil {
		t.Fatalf("讀不到 cast.go：%v", err)
	}
	want := map[int]int{}
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "spellEffects" {
			return true
		}
		lit, ok := vs.Values[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			call, ok := kv.Value.(*ast.CallExpr)
			if !ok {
				continue // 手寫的效果（識別字，不是呼叫）
			}
			fn, ok := call.Fun.(*ast.Ident)
			if !ok {
				continue
			}
			pos, ok := argOf[fn.Name]
			if !ok {
				continue // 不打怪的建構式
			}
			key, err := strconv.Atoi(kv.Key.(*ast.BasicLit).Value)
			if err != nil {
				t.Fatalf("法術編號解不開：%v", err)
			}
			cnt, err := strconv.Atoi(call.Args[pos].(*ast.BasicLit).Value)
			if err != nil {
				t.Fatalf("法術 %d 的 count 解不開：%v", key, err)
			}
			want[key] = cnt
		}
		return false
	})
	if len(want) == 0 {
		t.Fatal("語法樹裡一條打怪法術都沒找到 —— 掃法壞了，不是表空了")
	}
	got := game.MonsterCounts()
	for idx, n := range want {
		if got[idx] != n {
			t.Errorf("法術 %d：spellEffects 打 %d 隻，spellMonsterCount 寫 %d", idx, n, got[idx])
		}
	}
	for idx := range got {
		if _, ok := want[idx]; !ok {
			t.Errorf("法術 %d 在 spellMonsterCount 裡，但 spellEffects 沒有對應的建構式", idx)
		}
	}
}
