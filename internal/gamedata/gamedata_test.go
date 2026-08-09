package gamedata_test

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/gamedata"
)

func load(t *testing.T) *gamedata.Data {
	t.Helper()
	d, err := gamedata.Load(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Skipf("資料還沒產生：%v", err)
	}
	return d
}

// opcode 長度表有兩個獨立來源：EXE 尾部的 DGROUP 影像，以及執行時的
// 記憶體 dump。這裡釘住幾個關鍵值，資料重產時位移飄掉就會被抓到。
func TestOpcodeLengths(t *testing.T) {
	d := load(t)
	if n := len(d.Opcodes.Lengths); n != 51 {
		t.Fatalf("長度表有 %d 項，預期 51", n)
	}
	for op, want := range map[byte]int{
		0x00: 0,  // 沒有 handler
		0x01: 2,  // 顯示字串（靠左）
		0x04: 2,  // 顯示字串
		0x0b: 3,  // 進入設施畫面
		0x12: 13, // 最長的幾個之一
	} {
		if got := d.OpLen(op); got != want {
			t.Errorf("opcode %#02x 長度 = %d，預期 %d", op, got, want)
		}
	}
}

// 攻擊次數除數依職業索引：武士、遊俠、弓箭手、野蠻人每級都增加（除數 1），
// 巫師最慢（除數 4）。
func TestAttackDivisor(t *testing.T) {
	d := load(t)
	const knight, sorcerer = 0, 4
	if got := d.AttackDivisorFor(knight); got != 1 {
		t.Errorf("武士的除數 = %d，預期 1", got)
	}
	if got := d.AttackDivisorFor(sorcerer); got != 4 {
		t.Errorf("巫師的除數 = %d，預期 4", got)
	}
	if got := d.AttackDivisorFor(99); got != 1 {
		t.Errorf("職業越界時應該回 1，實際 %d", got)
	}
}

// 遭遇門檻必須遞增，最後一段到 100 —— 否則 rand(1,100) 會有落不進任何
// 一段的值。
func TestEncounterThresholds(t *testing.T) {
	d := load(t)
	th := d.Encounter.Thresholds
	for i := 1; i < len(th); i++ {
		if th[i] <= th[i-1] {
			t.Fatalf("門檻表不是遞增的：%v", th)
		}
	}
	if th[len(th)-1] != 100 {
		t.Errorf("最後一段的門檻是 %d，預期 100", th[len(th)-1])
	}
}

// 三十種特殊攻擊：效果編號依元素分群，火系三種必須同號。
func TestSpecialAttacks(t *testing.T) {
	d := load(t)
	if n := len(d.Specials); n != 30 {
		t.Fatalf("特殊攻擊有 %d 種，預期 30", n)
	}
	byName := map[string]gamedata.SpecialAttack{}
	for _, s := range d.Specials {
		byName[s.Announce] = s
	}
	for _, name := range []string{"breathes fire", "fireballs", "incinerate", "inferno"} {
		s, ok := byName[name]
		if !ok {
			t.Fatalf("找不到 %q", name)
		}
		if s.Effect != gamedata.EffectFire {
			t.Errorf("%q 的效果是 %v，預期火", name, s.Effect)
		}
	}
	// 四種標成 99 的不走共用路徑。
	for _, name := range []string{"casts a curse", "drains magic", "drains spell level", "vaporizes valuables"} {
		if byName[name].Handled() {
			t.Errorf("%q 應該是另有處理（旗標 99）", name)
		}
	}
}

// 法術表 96 條，牧師巫師各 48。
func TestSpells(t *testing.T) {
	d := load(t)
	if n := len(d.Spells); n != 96 {
		t.Fatalf("法術有 %d 條，預期 96", n)
	}
	cleric := 0
	for _, s := range d.Spells {
		if s.School == gamedata.SchoolCleric {
			cleric++
		}
	}
	if cleric != 48 {
		t.Errorf("牧師法術 %d 條，預期 48", cleric)
	}
}

// 缺檔時的錯誤要講得清楚，指向產生資料的工具。
func TestMissingDataMessage(t *testing.T) {
	_, err := gamedata.Load(t.TempDir())
	if err == nil {
		t.Fatal("空目錄應該要報錯")
	}
	if !contains(err.Error(), "cmd/mm2data") {
		t.Errorf("錯誤訊息沒有指向產生工具：%v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
