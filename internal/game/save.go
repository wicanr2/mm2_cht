package game

import (
	"encoding/binary"
	"fmt"
)

// Encode 把角色寫回 130 bytes 的記錄格式。
//
// 關鍵是 Raw：解析時整筆原樣留著，寫回時只覆蓋**已經定位的欄位**，
// 其餘二十幾個還沒解的位元組原封不動帶回去。這樣存檔不會因為
// 「remake 還沒解完」就把原版的資料洗掉 —— 也讓存檔能拿回原版讀。
func (c *Character) Encode() []byte {
	out := make([]byte, RecordSize)
	if len(c.Raw) == RecordSize {
		copy(out, c.Raw)
	}
	// 空槽原樣送回去。名冊裡刪掉的角色留著半截舊資料，
	// 名稱欄是全 0 而不是空格填充 —— 照有效角色的規則寫回會改動它們。
	if c.Empty() {
		return out
	}
	// 名稱：10 bytes，空格填充，第 11 個位元組歸零
	for i := 0; i < 10; i++ {
		if i < len(c.Name) {
			out[offName+i] = c.Name[i]
		} else {
			out[offName+i] = ' '
		}
	}
	out[offName+10] = 0

	out[offSex] = byte(c.Sex)
	out[offAlign] = byte(c.Align)
	out[offRace] = byte(c.Race)
	out[offClass] = byte(c.Class)
	out[offLevel] = byte(c.Level)
	out[offAge] = byte(c.Age)
	out[offAgeDays] = byte(c.AgeDays)
	out[offFood] = byte(c.Food)
	out[offAC] = byte(c.AC)
	out[offGems] = byte(c.Gems)
	out[offGems+1] = byte(c.Gems >> 8)
	writeU32(out, offExp, c.Exp)
	writeU32(out, offGold, c.Gold)
	for i := 0; i < slotsPerSet; i++ {
		e, b := c.Items[i], c.Items[slotsPerSet+i]
		out[offEquipID+i], out[offEquipCharge+i], out[offEquipAttr+i] = byte(e.ID), e.Charge, e.Attr
		out[offPackID+i], out[offPackCharge+i], out[offPackAttr+i] = byte(b.ID), b.Charge, b.Attr
	}
	out[offSkills] = byte(c.Skills[0]<<4 | c.Skills[1]&0x0F)
	copy(out[offSpells:offSpells+6], c.SpellsKnown[:])
	for i := 0; i < NumResists; i++ {
		out[offResist+i] = byte(c.Resist[i])
	}
	out[offSL] = byte(c.SL)
	battleLevel := c.BattleLevel
	if battleLevel == 0 {
		battleLevel = c.Level
	}
	out[offBattleLevel] = byte(battleLevel)
	out[offEnd] = byte(c.Endurance)
	out[offThief] = byte(c.Thievery)
	out[offEndB] = byte(c.Endurance)
	out[offCond] = c.CondBits
	binary.LittleEndian.PutUint16(out[offHP:], uint16(c.HP))
	binary.LittleEndian.PutUint16(out[offBaseMaxHP:], uint16(c.baseMaxHP()))
	binary.LittleEndian.PutUint16(out[offMaxHP:], uint16(c.MaxHP))
	binary.LittleEndian.PutUint16(out[offSP:], uint16(c.SP))
	binary.LittleEndian.PutUint16(out[offMaxSP:], uint16(c.MaxSP))
	for i := Stat(0); i < NumStats; i++ {
		out[offStats+int(i)] = byte(c.Base[i])
		out[offCur+int(i)] = byte(c.Current[i])
	}
	return out
}

// EncodeRoster 把一份名冊寫回去。
//
// 要帶原檔進來，因為尾端有不成一筆的殘餘要原樣保留：
// ROSTER.DAT 是 8,293 bytes = 130 × 63 + **103**，那 103 bytes 不是零，
// 用零填會改動檔案。
func EncodeRoster(cs []Character, orig []byte) ([]byte, error) {
	need := len(cs) * RecordSize
	if len(orig) < need {
		return nil, fmt.Errorf("原檔 %d bytes 放不下 %d 筆記錄", len(orig), len(cs))
	}
	out := make([]byte, len(orig))
	copy(out, orig)
	for i := range cs {
		copy(out[i*RecordSize:], cs[i].Encode())
	}
	return out, nil
}

func writeU32(b []byte, off, v int) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
	b[off+2] = byte(v >> 16)
	b[off+3] = byte(v >> 24)
}

// ── 遊玩狀態 ────────────────────────────────────────────────────────────

// State 是遊玩狀態裡**不在角色記錄裡**的那一半：位置、朝向、亂數種子、
// 全域旗標。
//
// 原版把這些放在哪還沒解，所以這是 remake 自己的格式（JSON），
// 與名冊分開存。名冊那一份仍是原版格式、位元組完全一致往返。
//
// 少了它，劇情旗標（`ds:03F6` 起的 24 個位元組）與世紀（`ds:03CA`）
// 一離開行程就消失 —— 那等於全遊戲的進度沒有存。
type State struct {
	Version int    `json:"version"`
	Map     int    `json:"map"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Facing  int    `json:"facing"`
	Seed    uint16 `json:"seed"`

	// Globals 的 key 是 DGROUP 位址（十進位）。用位址不用選擇器，
	// 因為多個選擇器可能指到同一個位址。
	Globals map[uint16]byte `json:"globals,omitempty"`

	// Explored 是走過哪些格，key 是地圖編號，值是 256 個位元壓成的
	// 32 個位元組。原版沒有這一份（見 explored.go），所以它只在
	// remake 自己的存檔裡，不會寫進 ROSTER.DAT。
	Explored map[int][]byte `json:"explored,omitempty"`

	// EventUsed 是哪些格的事件用掉了，壓法同 Explored。
	//
	// 原版沒有這一份 —— 它的「用過了」只寫在屬性層，換圖就重讀。
	// 這裡存下來是為了 `World.KeepConsumedEvents` 這個可選的修正；
	// 開關本身**不進存檔**（與其他三個設定一樣，每次啟動回到預設）。
	// 舊存檔沒有這一欄，讀回來是空的。
	EventUsed map[int][]byte `json:"event_used,omitempty"`

	// Pending 是尚未收完玩家輸入的事件續跑點。它只在腳本停在
	// 0x07／0x09／0x0a／0x26／0x2f 時存在；讀檔後從 Offset 繼續，
	// 絕不重跑已付款、已傳送或已 ConsumeEvent 的前半段。
	Pending *EventPrompt `json:"pending,omitempty"`

	// BattlesWon／BattlesLost 是勝敗場數（原版 `ds:0410`／`ds:0412`）。
	// 只有結局畫面讀它們，但那是一整場遊戲累積下來的數字，
	// 不存就等於結局永遠印 0。舊存檔沒有這兩欄，讀回來就是 0。
	BattlesWon  int `json:"battles_won,omitempty"`
	BattlesLost int `json:"battles_lost,omitempty"`

	// LastInn 是最後投宿的城（原版 `ds:03D4`）。舊存檔沒有這一欄，
	// 讀回來是 0 ＝ Middlegate，與原版沒住過任何旅店時的行為相同。
	LastInn int `json:"last_inn,omitempty"`
}

// packExplored 把走過的格壓成位元。
func packExplored(e Explored) map[int][]byte {
	if len(e) == 0 {
		return nil
	}
	out := make(map[int][]byte, len(e))
	for m, cells := range e {
		b := make([]byte, (MapCells+7)/8)
		any := false
		for i, v := range cells {
			if v {
				b[i/8] |= 1 << (i % 8)
				any = true
			}
		}
		if any {
			out[m] = b
		}
	}
	return out
}

// unpackExplored 是 packExplored 的反向。
func unpackExplored(in map[int][]byte) Explored {
	out := Explored{}
	for m, b := range in {
		cells := make([]bool, MapCells)
		for i := range cells {
			if i/8 < len(b) && b[i/8]&(1<<(i%8)) != 0 {
				cells[i] = true
			}
		}
		out[m] = cells
	}
	return out
}

// StateVersion 是存檔格式的版本。第 2 版加入事件中途輸入的續跑點；
// LoadState 仍接受第 1 版，讓既有存檔可無痛升級。
const StateVersion = 2

// State 取出目前的遊玩狀態。
func (s *Session) State() State {
	st := State{
		Version: StateVersion,
		Map:     s.World.MapIndex,
		X:       s.World.X,
		Y:       s.World.Y,
		Facing:  int(s.World.Face),
		Seed:    s.Rand.Seedof(),

		BattlesWon:  s.BattlesWon,
		BattlesLost: s.BattlesLost,
		LastInn:     s.LastInn,
	}
	if len(s.World.Globals) > 0 {
		st.Globals = make(map[uint16]byte, len(s.World.Globals))
		for k, v := range s.World.Globals {
			st.Globals[k] = v
		}
	}
	st.Explored = packExplored(s.World.Explored)
	st.EventUsed = packExplored(s.World.EventUsed)
	if p := s.World.Pending; p != nil {
		copy := *p
		copy.Message = s.World.Message
		copy.Result = s.World.Result
		copy.Selected = s.World.Selected
		copy.TextExpect = s.World.TextExpect
		copy.Encounter = append([]int(nil), s.World.Encounter...)
		copy.Reward = s.World.Reward
		copy.Facility = s.World.Facility
		copy.Sound = s.World.Sound
		copy.Picture = s.World.Picture
		copy.Teleported = s.World.Teleported
		copy.Time = s.World.Time
		st.Pending = &copy
	}
	return st
}

// LoadState 把狀態套回這一場遊玩。
func (s *Session) LoadState(st State) error {
	if st.Version != 1 && st.Version != StateVersion {
		return fmt.Errorf("存檔格式版本 %d，這一版讀 1 或 %d", st.Version, StateVersion)
	}
	if st.Map < 0 || st.Map >= len(s.World.Maps) {
		return fmt.Errorf("存檔指向第 %d 張地圖，但只有 %d 張", st.Map, len(s.World.Maps))
	}
	if st.X < 0 || st.X >= MapW || st.Y < 0 || st.Y >= MapH {
		return fmt.Errorf("存檔的座標 (%d,%d) 出界", st.X, st.Y)
	}
	if st.Pending != nil && !s.World.validPrompt(st.Pending) {
		return fmt.Errorf("存檔的事件續跑點無效")
	}
	s.World.MapIndex, s.World.X, s.World.Y = st.Map, st.X, st.Y
	s.World.Face = Facing(st.Facing & 3)
	s.Rand.Seed(st.Seed)
	s.World.Globals = map[uint16]byte{}
	for k, v := range st.Globals {
		s.World.Globals[k] = v
	}
	s.World.Explored = unpackExplored(st.Explored)
	s.World.EventUsed = unpackExplored(st.EventUsed)
	s.World.MarkExplored()
	s.BattlesWon, s.BattlesLost = st.BattlesWon, st.BattlesLost
	s.LastInn = st.LastInn

	// 先清掉目前執行中事件留下的暫時輸出。舊版存檔沒有這些欄位，
	// 讀回後就維持舊版語意：從正常探索狀態接續。
	s.World.Message = ""
	s.World.MessageSegment = -1
	s.World.MessageWait = false
	s.World.Pending = nil
	s.World.Result = 0
	s.World.Selected = 0
	s.World.TextExpect = ""
	s.World.Encounter = nil
	s.World.Reward = Reward{}
	s.World.Facility = FacilityNone
	s.World.Device = DeviceNone
	s.World.Sound = -1
	s.World.Picture = 0
	s.World.Teleported = false
	s.World.Time = 0
	if st.Pending != nil {
		p := *st.Pending
		p.Encounter = append([]int(nil), st.Pending.Encounter...)
		s.World.Pending = &p
		s.World.Message = p.Message
		s.World.MessageSegment = p.Segment
		s.World.MessageWait = true
		s.World.Result = p.Result
		s.World.Selected = p.Selected
		s.World.TextExpect = p.TextExpect
		s.World.Encounter = append([]int(nil), p.Encounter...)
		s.World.Reward = p.Reward
		s.World.Facility = p.Facility
		s.World.Sound = p.Sound
		s.World.Picture = p.Picture
		s.World.Teleported = p.Teleported
		s.World.Time = p.Time
	}
	return nil
}
