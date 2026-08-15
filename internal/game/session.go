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
	World *World
	// Party 是目前上場的隊伍（最多六人），Roster 是名冊。
	//
	// 兩者是分開的：建好的角色先進名冊，再到旅店編進隊伍 ——
	// 原版的旅店（`1RETINN.OVL`）就是那個編組畫面
	// （`'A'-'X' to View`、`Ctrl 'A'-'X' to Add/Remove`）。
	Party    []Character
	Roster   []Character
	Rand     *Rand
	Bestiary []monsters.Monster

	// Items 是物品表。設定之後隊伍的武器數值會依已裝備的物品重算。
	Items []items.Item

	// shelf 是這一輪的商店貨架（見 ShopShelf），key 是日期／類別／城。
	shelf map[int][]int

	// casting 是正在套用效果的法術編號，−1 表示沒有。傷害計算在
	// applyDamage，那裡拿不到法術編號，而驅散類的加成需要知道。
	casting int

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

	// Difficulty 是**突襲狀態**（原版 `ds:0415`）。前排隻數依它調整：
	// 2 減半、3 加倍，其餘不動。由 rollAmbush 在開戰時擲，見那裡。
	Difficulty int

	// Target 是「選一名隊員」那批法術的對象，負值表示施法者自己。
	// 對應原版的 `sub_1CF8C` 選單。
	Target int

	// Item 是「選一件背包物品」那批法術的槽位（0–5），負值表示沒選。
	// 對應原版的 `sub_1CB48` 選單（回傳 `0x1B` 表示取消）。
	Item int

	// Choice 是數字提示的答案（原版 `sub_16EC2(下限, 上限)` 讀一個按鍵）。
	// 傳送術用它當步數 1–9、城市傳送術用它當城市 1–5。
	Choice int

	// Column 是字母提示的答案（飛行術的 A–E，0 起算）。
	Column int

	// Fight 是進行中的戰鬥，沒有就是 nil。攻擊法術要靠它找目標。
	Fight *Encounter

	// Facility 是這一步踩到的設施，沒有就是 FacilityNone。
	Facility FacilityKind

	// Device 是這一步踩到的特殊裝置（`2CAVES` 那幾支），沒有就是 DeviceNone。
	Device CaveDevice

	// Log 是最近一次動作產生的訊息。
	Log []string

	// showMap 由定位術設起來，Cast 回傳時搬進 CastResult.ShowMap。
	showMap bool
}

// NewSession 建一次遊玩。
func NewSession(w *World, party []Character, bestiary []monsters.Monster, seed uint16) *Session {
	s := &Session{World: w, Party: party, Bestiary: bestiary,
		Rand: NewRand(seed), EncounterRate: 12, Target: -1, Item: -1, casting: -1}
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
			for y := 0; y < MapH; y++ {
				for x := 0; x < MapW; x++ {
					s.World.Maps[i].Ceiling[y*MapW+x] = attrs[i].Ceiling(x, y)
				}
			}
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
// **力氣過了還不一定開得成**：`0x1C1FA` 再擲一次 `rand(1,100)`，
//
//	>= 51 → `loc_1765A`（root `sub_13A64`）把門打開
//	<  51 → `sub_1C41E`／`sub_1C390` 觸發陷阱，`ds:0430 = 3` 重繪
//
// 兩條路都不印 `Success!` —— 那句只有開鎖才有。門開了由畫面自己表現。
//
// 開門由 `World.OpenDoor` 翻掉屬性層的牆位元。那一層在離開地圖時會還原，
// 所以門的「開著」只活到走出這張圖為止（見 docs/formats/06-map.md）。
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
	need := 0
	if idx := s.World.MapIndex; idx >= 0 && idx < len(s.Attrs) {
		need = s.Attrs[idx].BashDifficulty()
	}
	if roll != 5 && might+roll < need {
		return false, "撞不開。"
	}
	if s.Rand.Range(1, 100) < 51 {
		return false, s.Trap()
	}
	s.World.OpenDoor(s.World.X, s.World.Y, s.World.Face)
	return true, "成功！"
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
		s.World.OpenDoor(s.World.X, s.World.Y, s.World.Face)
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
// **賊與忍者會多吃一次**：`sub_1C390` 先對職業 5／6 單獨結算一次，
// 然後**沒有跳過**全隊那個迴圈（`loc_1C3C2` 之後緊接 `loc_1C3D1`，
// 中間沒有 `jmp`）。這裡的 `Trap` 是地圖陷阱，沒有「誰去碰」這個概念，
// 所以全隊一視同仁；開箱那條走 `Chest.springTrap`。
// 抗性有沒有減免仍未解。
// trapDamage 是陷阱傷害：`ds:2946[場景] << ds:599A`（`sub_1C338`）。
// 地圖陷阱與寶箱陷阱共用同一條。
func (s *Session) trapDamage() int {
	if data == nil {
		return 0
	}
	scene, shift := 0, 0
	if i := s.World.MapIndex; i >= 0 && i < len(s.Attrs) {
		scene = gamedata.TrapScene(s.Attrs[i].Scene())
		shift = s.Attrs[i].TrapShift()
	}
	return data.Traps.Damage(scene, shift)
}

func (s *Session) Trap() string {
	if data == nil {
		return "陷阱！"
	}
	scene := 0
	if i := s.World.MapIndex; i >= 0 && i < len(s.Attrs) {
		scene = gamedata.TrapScene(s.Attrs[i].Scene())
	}
	kind := s.Rand.Range(1, 100) & 3
	dmg := s.trapDamage()
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
	// 突襲狀態只活一場：原版 `_2play_e00` 在移動時把 `ds:0415` 清成 0。
	s.Difficulty = 0
	// 原版事件正在等鍵／Y/N／選人／文字時，不會讓移動把續跑點覆蓋掉。
	if s.World.Pending != nil {
		return false, nil
	}
	wasRoom := s.inRoom()
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
	return true, s.finishEvent(wasRoom, true)
}

// ResumeEventKey 讓 0x07 收到確認鍵並執行後半段腳本。
func (s *Session) ResumeEventKey() (*Encounter, bool) {
	return s.resumeEvent(s.World.ResumeKey)
}

// ResumeEventYesNo 交出事件的 Y/N 回答。
func (s *Session) ResumeEventYesNo(yes bool) (*Encounter, bool) {
	return s.resumeEvent(func() bool { return s.World.ResumeYesNo(yes) })
}

// ResumeEventMember 交出 1 起算的隊員編號，0 表示取消。
func (s *Session) ResumeEventMember(member int) (*Encounter, bool) {
	return s.resumeEvent(func() bool { return s.World.ResumeMember(member) })
}

// ResumeEventText 交出事件的文字答案。
func (s *Session) ResumeEventText(answer string) (*Encounter, bool) {
	return s.resumeEvent(func() bool { return s.World.ResumeText(answer) })
}

func (s *Session) resumeEvent(resume func() bool) (*Encounter, bool) {
	s.Log = nil
	if !resume() {
		return nil, false
	}
	// 回答事件本身不是走路，但原本的同步直譯器會在同一次 Step 結算
	// 設施、獎賞與固定／隨機遭遇；續跑完成後沿用同一條結算規則。
	return s.finishEvent(false, false), true
}

// finishEvent 收下已經執行完的事件效果。若腳本又停在輸入點，後面的
// 設施、獎賞與遭遇必須延後，否則玩家尚未回答就會看見結果。
func (s *Session) finishEvent(wasRoom, reportRoom bool) *Encounter {
	if s.World.Message != "" {
		s.Log = append(s.Log, s.World.Message)
	}
	if reportRoom && s.World.Pending == nil && s.inRoom() != wasRoom {
		now := s.inRoom()
		if now {
			s.Log = append(s.Log, "你走進一間石室。")
		} else {
			s.Log = append(s.Log, "你離開了石室。")
		}
	}
	if s.World.Pending != nil {
		return nil
	}
	// 踩到設施就進去。判準**只有**腳本的 opcode `0x0e`（`FacilityByCode`）。
	//
	// 招牌字串不能拿來判：招牌格與入口格是分開的兩格。Middlegate 的
	// 旅店招牌在 (7,5)、入口（`0e 01`）在 (7,3)；神殿招牌在 (7,6)、
	// 入口（`0e 04`）在 (7,7)。用招牌判會在招牌格就把人送進設施，
	// 等於多開一次門。
	if k := s.World.Facility; k != FacilityNone {
		s.Facility = k
		s.Log = append(s.Log, s.EnterFacility(k)...)
		return nil // 在設施裡不會遇敵
	}
	s.Facility = FacilityNone
	if d := s.World.Device; d != DeviceNone {
		// 要玩家輸入的交給 UI 開畫面；不要的當場做完。
		if d.NeedsUI() {
			s.Device = d
			return nil // 這一步不遇敵
		}
		s.Device = DeviceNone
		switch d {
		case DeviceFight:
			enc := s.FightRoll()
			s.Fight = enc
			if enc != nil {
				s.World.Flag = true
			}
			return enc
		case DeviceRack1, DeviceRack2, DeviceRack3:
			s.Log = append(s.Log, s.TakeFromRack(d)...)
		case DeviceTripleGold:
			s.Log = append(s.Log, s.TripleGold()...)
		}
		return nil
	}
	s.Device = DeviceNone
	// 腳本擺好的獎賞當場領走（原版 `ds:0434` 那條路）。
	if lines := s.ClaimReward(); len(lines) > 0 {
		s.Log = append(s.Log, lines...)
	}
	// 腳本擺出來的固定遭遇優先於隨機遭遇。
	if len(s.World.Encounter) > 0 {
		enc := s.fixedEncounter(s.World.Encounter)
		s.World.Encounter = nil
		s.Fight = enc
		if enc != nil {
			s.World.Flag = true // 打過架，條件旗標成立到下次移動為止
		}
		return enc
	}
	if rate := s.encounterRate(); rate > 0 && !s.onEventCell() &&
		s.Rand.Range(1, rate) == 1 {
		enc := s.rollEncounter()
		s.Fight = enc
		if enc != nil {
			s.World.Flag = true
		}
		return enc
	}
	return nil
}

// inRoom 回報隊伍站的那一格是不是房間輪廓的一部分（`game.AttrRoom`）。
func (s *Session) inRoom() bool {
	m := s.World.CurrentMap()
	return m != nil && m.Indoor && m.InRoom(s.World.X, s.World.Y)
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
	return m.WallKind(s.World.X, s.World.Y, f).String()
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
	s.rollAmbush()
	s.rollFront(e)
	e.Protect = s.protection()
	s.Log = append(s.Log, encounterLine(e))
	return e
}

// encounterLine 是遭遇的播報。異界生物會另外點名 —— 玩家要知道
// 驅散類法術對這一場特別有效（見 game.dispelSpells）。
func encounterLine(e *Encounter) string {
	n := 0
	for _, m := range e.Monsters {
		if otherworldly(m) {
			n++
		}
	}
	if n == 0 {
		return fmt.Sprintf("遭遇 %d 隻敵人！", len(e.Monsters))
	}
	return fmt.Sprintf("遭遇 %d 隻敵人，其中 %d 隻是異界生物！",
		len(e.Monsters), n)
}

// rollFront 決定這一場的前排隻數。
//
// **開戰一定要擲** —— 前排是 0 就代表近戰打不到任何人，而場上明明有怪。
// 原版在 `sub_19640` 擲，室內／室外用不同的式子，所以要先知道在哪。
func (s *Session) rollFront(e *Encounter) {
	indoor := false
	if a := s.CurrentAttr(); a != nil {
		indoor = a.Indoor()
	}
	e.RollFront(s.Rand, indoor, s.Difficulty)
}

// rollAmbush 擲這一場的突襲狀態（原版 `ds:0415`）。
//
// `2COMBAT _2combat_e03` 的 `0x1A4E7`：
//
//	if ds:0415 == 0:                                  ; 還沒被腳本指定
//	    roll = rand(1, 100)
//	    if roll <= 40 且 roll <= ds:549E → ds:0415 = 2   ; 隊伍先手
//	    else if ds:03DA == 0 且 roll >= 90 → ds:0415 = 3 ; 被突襲
//
// `ds:549E` 是**隊伍的平均盜行**（root `sub_13A9E` 逐人取記錄 `+0x1E`
// 再除以人數，上限 255），所以盜行越高越容易先手 —— 不是難度設定。
// `ds:03DA` 是守衛術（見 docs/formats/09 §計數型），**開著就不會被突襲**。
//
// 已經非零時不擲：那是腳本指定的遭遇（`2PLAY sub_19912` 寫 `0x80`、
// `2SMITH` 寫 `0x83`、`2MISC sub_1CEEE` 寫 3），開戰時 `0x1A344`
// 先把 `0x80` 減掉。
func (s *Session) rollAmbush() {
	if s.Difficulty != 0 {
		return
	}
	roll := s.Rand.Range(1, 100)
	switch {
	case roll <= 40 && roll <= s.avgThievery():
		s.Difficulty = 2
	case s.World != nil && s.World.Globals[globalGuard] == 0 && roll >= 90:
		s.Difficulty = 3
	}
}

// RollAmbushForTest 讓測試直接擲一次突襲狀態。正式路徑走 rollAmbush，
// 它在建立遭遇時、`rollFront` 之前被呼叫。
func (s *Session) RollAmbushForTest() { s.rollAmbush() }

// globalGuard 是守衛術的計數器（`ds:03DA`）。
const globalGuard = 0x03DA

// avgThievery 是隊伍的平均盜行（原版 `sub_13A9E`）。整數除法，上限 255。
func (s *Session) avgThievery() int {
	n := len(s.Party)
	if n == 0 {
		return 0
	}
	sum := 0
	for i := range s.Party {
		sum += s.Party[i].Thievery
	}
	if v := sum / n; v <= 255 {
		return v
	}
	return 255
}

// rollEncounter 依目前所在的地圖決定遇到什麼。
//
// 怪物的挑選走**原版的門檻表**：`rand(1,100)` 落在 `ds:10EA` 的哪一段
// 決定類別，再由 `ds:10F6` 的基礎編號加上難度對應的範圍。
// 兩張表都是從執行時的記憶體 dump 出來的（見 internal/game/tables.go）。
//
// **什麼時候擲這一把**由 `Session.step` 決定，也是原版的：每走一步擲
// `rand(1, ATTRIB+9)`，擲出 1 才進來，事件格不擲（`sub_17EB9`）。
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
	s.rollAmbush()
	s.rollFront(e)
	e.Protect = s.protection()
	s.Log = append(s.Log, encounterLine(e))
	return e
}

// protection 把五個防護法術的全域計數器抄成一份。
func (s *Session) protection() Protection {
	g := func(addr uint16) int {
		if s.World == nil {
			return 0
		}
		return int(s.World.Globals[addr])
	}
	return Protection{
		Curse:       g(0x03DB),
		Bless:       g(0x03E3),
		Invisible:   g(0x03E4),
		Shield:      g(0x03E5),
		PowerShield: g(0x03E6),
		HolyBonus:   g(0x03E7),
	}
}

// EndCombat 結束戰鬥並復原只在戰鬥期間有效的東西。
//
// 目前是戰鬥用的等級：勇氣術把記錄 `+113` 加了 6，而 `+32` 沒動 ——
// 「維持到戰鬥結束」就是靠這個復原步驟做出來的。
func (s *Session) EndCombat() {
	s.Fight = nil
	for i := range s.Party {
		s.Party[i].ResetBattleLevel()
	}
}

// CurrentAttr 是目前這張地圖的屬性記錄，沒載入或超出範圍回 nil。
func (s *Session) CurrentAttr() *MapAttr {
	i := s.World.MapIndex
	if i < 0 || i >= len(s.Attrs) {
		return nil
	}
	return &s.Attrs[i]
}
