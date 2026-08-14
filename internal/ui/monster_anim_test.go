package ui

import (
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
)

func testMonsterSession(pic gfx.MonsterPic, sprite int) (*Session, *game.Encounter) {
	m := game.NewMonster(monsters.Monster{Sprite: sprite})
	f := &game.Encounter{Monsters: []game.Combatant{m}}
	s := &Session{
		Game:     &game.Session{Fight: f},
		monBlob:  []byte{1},
		monIndex: []int{1},
		monCache: map[int]gfx.MonsterPic{0: pic},
	}
	return s, f
}

func TestMonsterAnimationUsesHoldAndLoops(t *testing.T) {
	pic := gfx.MonsterPic{
		Frames: []gfx.Frame{{}, {}, {}},
		Script: []gfx.ScriptStep{{Seq: 1}},
		Anims:  [][]gfx.AnimStep{{{Frame: 1, Hold: 2}, {Frame: 2, Hold: 1}}},
	}
	s, _ := testMonsterSession(pic, 1)
	if got := s.sprites()[0]; got.Anim != 0 || got.Step != 0 {
		t.Fatalf("初始動畫 = (%d,%d)，預期 (0,0)", got.Anim, got.Step)
	}
	s.Tick()
	if got := s.sprites()[0].Step; got != 0 {
		t.Fatalf("第一個 hold tick 後 step = %d，預期 0", got)
	}
	s.Tick()
	if got := s.sprites()[0].Step; got != 1 {
		t.Fatalf("第二個 hold tick 後 step = %d，預期 1", got)
	}
	s.Tick()
	if got := s.sprites()[0].Step; got != 0 {
		t.Fatalf("循環後 step = %d，預期 0", got)
	}
}

// 影格編號越界不是「非法段」——原版比對影格數，超過就畫影格 0
// （root `0x1578E`）。所以照播，畫面上只剩基準圖。
func TestMonsterAnimationPlaysOutOfRangeFrame(t *testing.T) {
	pic := gfx.MonsterPic{
		Frames: []gfx.Frame{{}, {}},
		Script: []gfx.ScriptStep{{Seq: 1}},
		Anims:  [][]gfx.AnimStep{{{Frame: 9, Hold: 5}}},
	}
	s, _ := testMonsterSession(pic, 1)
	if got := s.sprites()[0]; got.Anim != 0 || got.Step != 0 {
		t.Fatalf("越界影格的段仍該照播，得到 (%d,%d)", got.Anim, got.Step)
	}
}

// 停留 0 的單步序列不能把游標推出去 —— 那會變成無窮前進。
func TestMonsterAnimationHandlesZeroHold(t *testing.T) {
	pic := gfx.MonsterPic{
		Frames: []gfx.Frame{{}, {}},
		Script: []gfx.ScriptStep{{Seq: 1}},
		Anims:  [][]gfx.AnimStep{{{Frame: 1, Hold: 0}}},
	}
	s, _ := testMonsterSession(pic, 1)
	s.Tick()
	if got := s.sprites()[0].Step; got != 0 {
		t.Fatalf("hold<=0 的安全 fallback 不應越過單步序列：%d", got)
	}
}

// 腳本走完一段就換下一項；帶 bit 7 的項要隨機挑。
func TestMonsterAnimationFollowsScript(t *testing.T) {
	pic := gfx.MonsterPic{
		Frames: []gfx.Frame{{}, {}, {}},
		Script: []gfx.ScriptStep{{Seq: 1}, {Seq: 2}},
		Anims: [][]gfx.AnimStep{
			{{Frame: 1, Hold: 1}},
			{{Frame: 2, Hold: 1}},
		},
	}
	s, _ := testMonsterSession(pic, 1)
	if got := s.sprites()[0].Anim; got != 0 {
		t.Fatalf("腳本第 1 項該播第 1 段（Anims[0]），得到 %d", got)
	}
	s.Tick()
	if got := s.sprites()[0].Anim; got != 1 {
		t.Fatalf("一段播完該換腳本第 2 項（Anims[1]），得到 %d", got)
	}
	s.Tick()
	if got := s.sprites()[0].Anim; got != 0 {
		t.Fatalf("腳本走完該繞回第 1 項，得到 %d", got)
	}
}

func TestMonsterAnimationResetsOnFightChangeAndHandlesNoAnimation(t *testing.T) {
	s, first := testMonsterSession(gfx.MonsterPic{Frames: []gfx.Frame{{}, {}}, Script: []gfx.ScriptStep{{Seq: 1}}, Anims: [][]gfx.AnimStep{{{Frame: 1, Hold: 3}}}}, 1)
	s.sprites()
	s.Tick()
	second := &game.Encounter{Monsters: []game.Combatant{game.NewMonster(monsters.Monster{Sprite: 1})}}
	s.Game.Fight = second
	if got := s.sprites()[0].Step; got != 0 {
		t.Fatalf("重新進戰鬥未重設 step：%d", got)
	}
	s.Game.Fight = nil
	s.Tick()
	if s.monsterAnimFight != nil || s.monsterAnimStates != nil {
		t.Fatal("離開戰鬥後仍保留動畫狀態")
	}
	s.Game.Fight = first
	s.monCache[0] = gfx.MonsterPic{Frames: []gfx.Frame{{}}, Anims: nil}
	if got := s.sprites()[0]; got.Anim != -1 || got.Step != 0 {
		t.Fatalf("無動畫圖片未安全退回基準圖：(%d,%d)", got.Anim, got.Step)
	}
}
