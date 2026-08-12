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

func TestMonsterAnimationFallsBackForInvalidOrZeroHold(t *testing.T) {
	pic := gfx.MonsterPic{
		Frames: []gfx.Frame{{}, {}},
		Anims: [][]gfx.AnimStep{
			{{Frame: 9, Hold: 5}},
			{{Frame: 1, Hold: 0}},
		},
	}
	s, _ := testMonsterSession(pic, 1)
	if got := s.sprites()[0]; got.Anim != 1 || got.Step != 0 {
		t.Fatalf("應跳過非法段，得到 (%d,%d)", got.Anim, got.Step)
	}
	s.Tick()
	if got := s.sprites()[0].Step; got != 0 {
		t.Fatalf("hold<=0 的安全 fallback 不應越過單步序列：%d", got)
	}
}

func TestMonsterAnimationResetsOnFightChangeAndHandlesNoAnimation(t *testing.T) {
	s, first := testMonsterSession(gfx.MonsterPic{Frames: []gfx.Frame{{}, {}}, Anims: [][]gfx.AnimStep{{{Frame: 1, Hold: 3}}}}, 1)
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
