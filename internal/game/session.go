package game

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/assets/items"
	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/gamedata"
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

	// Items 是物品表。設定之後隊伍的武器數值會依已裝備的物品重算。
	Items []items.Item

	// Attrs 是六十張地圖的屬性（`ATTRIB.DAT`）。撞門的難度從這裡來。
	Attrs []MapAttr

	// Names 是怪物名的譯文，空的話顯示原文。
	Names map[string]string

	// EncounterRate 是每走一步遇敵的機率分母，**每張地圖各自不同**：
	// 走進哪一張就用那一張的 `ATTRIB.DAT` `+9`。設定 `UseAttrs` 之後
	// 由 `Step` 自動跟著地圖換，只有在沒有地圖屬性時才用這裡的值。
	//
	// 判定抄自 `sub_17EB9`：`rand(1, N)` 擲出 1 才遇敵，而且**事件格
	// 不擲**（`cmp ds:59C8, 0x80` 那一行先擋掉）。
	EncounterRate int

	// Facility 是這一步踩到的設施，沒有就是 FacilityNone。
	Facility FacilityKind

	// Log 是最近一次動作產生的訊息。
	Log []string
}

// NewSession 建一次遊玩。
func NewSession(w *World, party []Character, bestiary []monsters.Monster, seed uint16) *Session {
	s := &Session{World: w, Party: party, Bestiary: bestiary,
		Rand: NewRand(seed), EncounterRate: 12}
	// 腳本要改角色欄位（opcode 0x15／0x18），所以世界那邊也要看得到隊伍。
	// 共用同一個底層陣列 —— 腳本改的就是這裡的資料。
	w.Party = s.Party
	w.Rand = s.Rand
	w.Sound = -1
	return s
}

// UseAttrs 設定地圖屬性，並把室內／室外標記寫進每張地圖。
//
// 沒設定的話所有地圖都算室外，`WallKind` 就不會回報門 ——
// 寧可少報一種牆，也不要在野外圖上把地形碼誤讀成門。
func (s *Session) UseAttrs(attrs []MapAttr) {
	s.Attrs = attrs
	s.World.Neighbor = make([][4]int, len(s.World.Maps))
	for i := range s.World.Maps {
		if i < len(attrs) {
			s.World.Maps[i].Indoor = attrs[i].Indoor()
			s.World.Neighbor[i] = attrs[i].Neighbors()
		}
	}
}

// BashDoor 撞前方的門。
//
// 公式抄自 `2MISC.img` 的 `0xC19C`：
//
//	力量 = 隊伍第一人的當前力量，隊伍超過一人再加上第二人的
//	擲   = rand(10, 109) / 10   → 1–10
//	擲 == 5 直接成功；否則 力量 + 擲 >= 這張地圖的門難度才成功
//
// 「擲出 5 直接成功」是原版真的這樣寫 —— 一條與門難度無關的保底。
//
// 撞開之後把那一面的門位元改成實牆的相反：原版怎麼記「已經開了」還沒解，
// 所以這裡只回報成敗，不改地圖。
func (s *Session) BashDoor() (bool, string) {
	m := s.World.CurrentMap()
	if m == nil {
		return false, "這裡沒有地圖。"
	}
	if m.WallKind(s.World.X, s.World.Y, s.World.Face) != WallDoor {
		return false, "前面不是門。"
	}
	might := 0
	for i, c := range s.Party {
		if i >= 2 {
			break
		}
		might += c.Current[Might]
	}
	roll := s.Rand.Range(10, 109) / 10
	if roll == 5 {
		return true, "成功！"
	}
	need := 0
	if idx := s.World.MapIndex; idx >= 0 && idx < len(s.Attrs) {
		need = s.Attrs[idx].BashDifficulty()
	}
	if might+roll >= need {
		return true, "成功！"
	}
	return false, "撞不開。"
}

// Unlock 讓指定的隊員開前方的鎖。
//
// 公式抄自 `2MISC.img` 的 `0xC2B2`：
//
//	擲 = rand(1, 100)
//	擲 < 96 且 盜行 >= 擲  → 成功（原版印 `Success!`）
//
// 「擲 < 96」是原版的上限保護：擲到 96 以上一律失敗，
// 盜行再高也一樣。盜行取自記錄 `+0x1E`（= +30）。
//
// 失敗時再擲一次 `rand(1,100)`：地圖屬性 `+19` **小於**那一擲就觸發陷阱
// （`0xC2FF` 的 `jb`）。陷阱走 `Trap`。
func (s *Session) Unlock(who int) (bool, string) {
	m := s.World.CurrentMap()
	if m == nil {
		return false, "這裡沒有地圖。"
	}
	if m.WallKind(s.World.X, s.World.Y, s.World.Face) != WallDoor {
		return false, "沒有鎖！"
	}
	if who < 0 || who >= len(s.Party) {
		return false, "沒有這個人。"
	}
	roll := s.Rand.Range(1, 100)
	if roll < 96 && s.Party[who].Thievery >= roll {
		return true, "成功！"
	}
	if s.lockDifficulty() < s.Rand.Range(1, 100) {
		return false, s.Trap()
	}
	return false, "開不開。"
}

func (s *Session) lockDifficulty() int {
	if i := s.World.MapIndex; i >= 0 && i < len(s.Attrs) {
		return s.Attrs[i].LockDifficulty()
	}
	return 0
}

// Trap 觸發一個陷阱：擲出種類、算傷害、對全隊套用，回傳原版的播報文字。
//
// 抄自 `2MISC.OVL`：
//
//	種類 = rand(1,100) & 3        ; 0 電擊 1 火焰 2 毒氣 3 尖刺
//	訊息 = 訊息表[場景][種類]      ; ds:28F2，場景 × 16 + 種類 × 4
//	傷害 = 基礎[場景] << ATTRIB+20 ; 基礎表 ds:2946 = 3,4,4,5,6
//	逐一對隊伍套用（sub_1C390 → sub_1C338）
//
// **賊與忍者另有處理**（`sub_1C390` 對職業 5／6 先呼叫一次 `sub_1C338`），
// 那一段還沒解，所以這裡全隊一視同仁。抗性有沒有減免也未解。
func (s *Session) Trap() string {
	if data == nil {
		return "陷阱！"
	}
	scene, shift := 0, 0
	if i := s.World.MapIndex; i >= 0 && i < len(s.Attrs) {
		scene = gamedata.TrapScene(s.Attrs[i].Scene())
		shift = s.Attrs[i].TrapShift()
	}
	kind := s.Rand.Range(1, 100) & 3
	dmg := data.Traps.Damage(scene, shift)
	for i := range s.Party {
		if s.Party[i].Empty() {
			continue
		}
		s.Party[i].TakeDamage(dmg)
	}
	l := data.Traps.TrapText(scene, kind)
	if text != nil {
		return text.Or(l.Key, l.Text)
	}
	return l.Text
}

// UseItems 設定物品表，並依已裝備的物品重算全隊的戰鬥數值。
func (s *Session) UseItems(table []items.Item) {
	s.Items = table
	for i := range s.Party {
		s.Party[i].RecomputeGear(table)
		s.Party[i].RecomputeAC()
	}
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
	// 野外的地形檢查要在移動之前做 —— 它跟隊伍有關（山要登山家、
	// 林要探險家），`World` 看不到隊伍。
	if ok, msg := s.canEnter(step); !ok {
		s.Log = append(s.Log, msg)
		return false, nil
	}
	if !s.World.Move(step) {
		// 原版對實牆與門有分開的訊息（`Solid!` 與 `Locked!`）。
		s.Log = append(s.Log, s.blockedMessage(step))
		return false, nil
	}
	if s.World.Message != "" {
		s.Log = append(s.Log, s.World.Message)
	}
	// 踩到設施就進去。判準是腳本的 opcode `0x0e`（`FacilityByCode`）；
	// 腳本沒說的話才退回看招牌字串。
	k := s.World.Facility
	if k == FacilityNone {
		k = FacilityAt(s.World.Message)
	}
	if k != FacilityNone {
		s.Facility = k
		s.Log = append(s.Log, s.EnterFacility(k)...)
		return true, nil // 在設施裡不會遇敵
	}
	s.Facility = FacilityNone
	// 腳本擺出來的固定遭遇優先於隨機遭遇。
	if len(s.World.Encounter) > 0 {
		enc := s.fixedEncounter(s.World.Encounter)
		s.World.Encounter = nil
		if enc != nil {
			s.World.Flag = true // 打過架，條件旗標成立到下次移動為止
		}
		return true, enc
	}
	if rate := s.encounterRate(); rate > 0 && !s.onEventCell() &&
		s.Rand.Range(1, rate) == 1 {
		enc := s.rollEncounter()
		if enc != nil {
			s.World.Flag = true
		}
		return true, enc
	}
	return true, nil
}

// onEventCell 回報隊伍現在踩在事件格上 —— 那種格子不擲隨機遭遇
// （`sub_17EAF` 的 `cmp ds:59C8, 0x80` 先擋掉）。
func (s *Session) onEventCell() bool {
	m := s.World.CurrentMap()
	return m != nil && m.HasEvent(s.World.X, s.World.Y)
}

// encounterRate 回傳目前這張地圖的遭遇機率分母。
func (s *Session) encounterRate() int {
	if i := s.World.MapIndex; i >= 0 && i < len(s.Attrs) {
		return s.Attrs[i].EncounterRate()
	}
	return s.EncounterRate
}

// canEnter 做野外的地形檢查。室內圖直接放行，交給牆位元。
func (s *Session) canEnter(step int) (bool, string) {
	m := s.World.CurrentMap()
	if m == nil || m.Indoor {
		return true, ""
	}
	f := s.World.Face
	if step < 0 {
		f = Facing((int(f) + 2) & 3)
	}
	dx, dy := f.Delta()
	return s.EnterOutdoor(s.World.X+dx, s.World.Y+dy)
}

// blockedMessage 回傳撞牆時的訊息，分實牆與門。
func (s *Session) blockedMessage(step int) string {
	m := s.World.CurrentMap()
	if m == nil {
		return "走不過去。"
	}
	f := s.World.Face
	if step < 0 {
		f = (f + 2) & 3
	}
	if m.WallKind(s.World.X, s.World.Y, f) == WallDoor {
		return "鎖住了！"
	}
	return "是實牆！"
}

// Turn 轉向。也要清訊息 —— 不清的話上一步的「遭遇」會跟著轉向那一格
// 再顯示一次，看起來像轉個身又遇到一次。
func (s *Session) Turn(dir int) {
	s.Log = nil
	s.World.Turn(dir)
}

// fixedEncounter 依腳本給的怪物編號組一場遭遇。
//
// 原版的 opcode `0x12`／`0x13` 把十個編號寫進 `ds:9680`，戰鬥模組直接
// 拿那個陣列當怪物來源 —— 這不是隨機的，是設計好的。
func (s *Session) fixedEncounter(ids []int) *Encounter {
	if len(s.Bestiary) == 0 {
		return nil
	}
	e := &Encounter{Party: s.Combatants()}
	for _, id := range ids {
		if id >= len(s.Bestiary) {
			continue
		}
		m := NewMonster(s.Bestiary[id])
		m.Display = s.Names[m.Def.Name]
		e.Monsters = append(e.Monsters, m)
	}
	if len(e.Monsters) == 0 {
		return nil
	}
	s.Log = append(s.Log, fmt.Sprintf("遭遇 %d 隻敵人！", len(e.Monsters)))
	return e
}

// rollEncounter 依目前所在的地圖決定遇到什麼。
//
// 怪物的挑選走**原版的門檻表**：`rand(1,100)` 落在 `ds:10EA` 的哪一段
// 決定類別，再由 `ds:10F6` 的基礎編號加上難度對應的範圍。
// 兩張表都是從執行時的記憶體 dump 出來的（見 internal/game/tables.go）。
//
// 還是暫定的：**遭遇的觸發時機**。原版什麼時候擲這一把還沒解出來，
// 這裡用固定機率。
func (s *Session) rollEncounter() *Encounter {
	if len(s.Bestiary) == 0 {
		return nil
	}
	// 難度由所在地圖決定，怪物則走原版的門檻表（見 tables.go）。
	diff := s.World.MapIndex/20 + 1
	n := s.Rand.Range(1, 3)
	e := &Encounter{Party: s.Combatants()}
	for i := 0; i < n; i++ {
		idx := RollMonsterIndex(s.Rand, diff)
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
