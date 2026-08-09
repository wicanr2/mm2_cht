package game

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
)

// Session 把地圖、隊伍與亂數綁在一起，是遊戲迴圈的狀態。
//
// 存檔要存的就是這裡的東西：隊伍（`Character.Encode`）、位置、朝向、
// 亂數種子。種子一起存才能重播 —— 同一顆種子必然給出同一串數列。
type Session struct {
	World    *World
	Party    []Character
	Rand     *Rand
	Bestiary []monsters.Monster

	// Names 是怪物名的譯文，空的話顯示原文。
	Names map[string]string

	// EncounterRate 是每走一步遇敵的機率分母：值為 N 表示大約 1/N。
	// 原版的遭遇判定還沒解出來（`sub_19A3C` 是怪物**選擇**，不是觸發時機），
	// 這裡先用固定機率。
	EncounterRate int

	// Log 是最近一次動作產生的訊息。
	Log []string
}

// NewSession 建一次遊玩。
func NewSession(w *World, party []Character, bestiary []monsters.Monster, seed uint16) *Session {
	return &Session{World: w, Party: party, Bestiary: bestiary,
		Rand: NewRand(seed), EncounterRate: 12}
}

// Combatants 回傳隊伍裡還能行動的人。
func (s *Session) Combatants() []Combatant {
	out := make([]Combatant, 0, len(s.Party))
	for i := range s.Party {
		out = append(out, &s.Party[i])
	}
	return out
}

// Alive 回報隊伍裡還有沒有人站著。
func (s *Session) Alive() bool {
	for i := range s.Party {
		if s.Party[i].Condition.Acts() {
			return true
		}
	}
	return false
}

// Step 走一步：移動、觸發事件、判定遭遇。
//
// 回傳這一步有沒有進入戰鬥。戰鬥本身由呼叫端決定怎麼打
// （自動打完用 `Encounter.Fight`，逐指令則自己驅動）。
func (s *Session) Step(step int) (moved bool, enc *Encounter) {
	s.Log = nil
	if !s.World.Move(step) {
		s.Log = append(s.Log, "走不過去。")
		return false, nil
	}
	if s.World.Message != "" {
		s.Log = append(s.Log, s.World.Message)
	}
	if s.EncounterRate > 0 && s.Rand.Range(1, s.EncounterRate) == 1 {
		return true, s.rollEncounter()
	}
	return true, nil
}

// Turn 轉向。也要清訊息 —— 不清的話上一步的「遭遇」會跟著轉向那一格
// 再顯示一次，看起來像轉個身又遇到一次。
func (s *Session) Turn(dir int) {
	s.Log = nil
	s.World.Turn(dir)
}

// rollEncounter 依目前所在的地圖決定遇到什麼。
//
// 怪物的挑選方式：地圖編號越後面越難，用它決定怪物表的取樣區間。
// **這不是原版的遭遇表** —— 原版查的是 DGROUP 裡的難度門檻表
// （`sub_19A3C` 讀 `[bx+10EAh]`），那張表在 BSS，檔案裡讀不到。
func (s *Session) rollEncounter() *Encounter {
	if len(s.Bestiary) == 0 {
		return nil
	}
	base := s.World.MapIndex * 4
	if base > 200 {
		base = 200
	}
	n := s.Rand.Range(1, 3)
	e := &Encounter{Party: s.Combatants()}
	for i := 0; i < n; i++ {
		idx := base + s.Rand.Range(0, 15)
		if idx >= len(s.Bestiary) {
			idx = len(s.Bestiary) - 1
		}
		m := NewMonster(s.Bestiary[idx])
		m.Display = s.Names[m.Def.Name]
		e.Monsters = append(e.Monsters, m)
	}
	s.Log = append(s.Log, fmt.Sprintf("遭遇 %d 隻敵人！", n))
	return e
}
