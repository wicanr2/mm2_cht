package game

import (
	"fmt"
	"strings"
)

// 升級與城鎮設施。
//
// 手冊寫明的部分照做，沒寫的標暫定：
//
//   - 每級生命點數的骰子範圍（手冊第 18–22 頁的職業表）—— 照抄
//   - 經驗等級 → 法力等級的對照（1/3/5/7/9/11/13/15/17）—— 照抄
//   - 年齡由 18 歲開始，八十歲後過夜可能死亡 —— 照抄
//   - **升級所需的經驗值**：手冊沒給，從 `2MISC2.img` 的 `sub_CC8C` 解出來，
//     放在 data/experience.json
//
// 數值全部放在 data/*.json，這裡只有規則。
//
// 擲骰一律走 `Rand`，也就是原版那顆 RNG。

// HitDice 回傳這個職業每級的生命點數上限（data/classes.json）。
func (c Class) HitDice() int {
	if data == nil {
		return 8
	}
	return data.HitDice(int(c))
}

// SpellLevel 回傳這個角色能施放的最高咒語等級。不會施法的職業回 0。
func (c *Character) SpellLevel() int {
	if !c.Class.Caster() && c.Class != Paladin && c.Class != Archer {
		return 0
	}
	if data == nil {
		return 0
	}
	lv := data.SpellLevelFor(c.Level)
	// 遊俠與弓箭手要到高經驗等級才有法力，起步比純施法職業晚。
	if c.Class == Paladin || c.Class == Archer {
		lv -= 2
	}
	if lv < 0 {
		lv = 0
	}
	return lv
}

// ExpForLevel 是某個職業升到指定等級所需的累計經驗值。
//
// 原版的表（`data/experience.json`）：等級 2–10 直接查，武士／牧師／賊／
// 野蠻人從 1,500 起每級加倍，遊俠／弓箭手／巫師／忍者從 2,000 起。
// 11 級以上改成分段的等差累加。
func ExpForLevel(level int, class Class) int {
	if data == nil {
		return 0
	}
	return data.ExpForLevel(level, int(class))
}

// CanTrain 回報這個角色能不能在訓練所升級。
func (c *Character) CanTrain() bool {
	return c.Condition.Acts() && c.Exp >= ExpForLevel(c.Level+1, c.Class)
}

// Train 升一級：擲生命點數、法力等級跟著經驗等級走、年齡加一。
//
// 手冊：訓練基地「提昇技巧」。年齡隨遊戲進行增加，這裡讓它跟著升級走 ——
// 名冊那四十個角色的年齡與等級相關性是 +1.00，與這個做法一致。
func (c *Character) Train(r *Rand) (gained int, err error) {
	if !c.Condition.Acts() {
		return 0, fmt.Errorf("%s 目前是%v，不能受訓", c.Name, c.Condition)
	}
	if need := ExpForLevel(c.Level+1, c.Class); c.Exp < need {
		return 0, fmt.Errorf("%s 的經驗值 %d 不足，升到第 %d 級需要 %d",
			c.Name, c.Exp, c.Level+1, need)
	}
	c.Level++
	c.Age++
	gained = r.Range(1, c.Class.HitDice())
	c.MaxHP += gained
	c.HP = c.MaxHP
	if c.SpellLevel() > 0 {
		c.MaxSP += r.Range(1, 4)
		c.SP = c.MaxSP
	}
	return gained, nil
}

// RestAtInn 是旅店的休息：補滿生命與法力，解除無意識。
//
// 手冊：旅店的功能是「儲存遊戲」，登記後才能存檔。死亡不會因休息復原
// （手冊列的「休息無法恢復的狀況」有死亡、中毒、痲痺、石化、根除）。
// CanRestHere 回報目前這一格能不能休息（屬性層 bit 3）。
//
// 原版在 `_2misc` 的休息入口 `test byte_159C8, 8`，成立就印
// `Too dangerous!` 而不是問 `Rest here? (Y/N)`。旅店那條路不受影響 ——
// 旅店是設施，不是野地休息。
func (s *Session) CanRestHere() bool {
	m := s.World.CurrentMap()
	return m == nil || !m.NoRest(s.World.X, s.World.Y)
}

// TooDangerous 是不能休息時印的那一句（`ds:2A5C`）。
func TooDangerous() string {
	if text == nil {
		return "Too dangerous!"
	}
	return text.Or("exe.2A5C", "太危險了！")
}

func (s *Session) RestAtInn() []string {
	var log []string
	for i := range s.Party {
		c := &s.Party[i]
		if c.Empty() {
			continue
		}
		if c.Condition == CondDead {
			log = append(log, fmt.Sprintf("%s 已經死亡，休息救不回來。", c.Name))
			continue
		}
		if c.Condition == CondPoisoned {
			log = append(log, fmt.Sprintf("%s 仍在中毒，需要神殿。", c.Name))
		}
		if c.Condition == CondUnconscious {
			c.Condition = CondGood
			c.CondBits &^= CondBitUnconscious
		}
		c.HP, c.SP = c.MaxHP, c.MaxSP
	}
	log = append(log, "隊伍在旅店休息，體力與法力已恢復。")
	return log
}

// HealAtTemple 是神殿的治療：解除中毒等狀況，死亡另計。
//
// 手冊沒給價目，這裡不收費 —— 收費規則未解，寧可不收也不要編一個數字。
func (s *Session) HealAtTemple() []string {
	var log []string
	for i := range s.Party {
		c := &s.Party[i]
		if c.Empty() || c.Condition == CondGood {
			continue
		}
		switch c.Condition {
		case CondDead:
			log = append(log, fmt.Sprintf("%s 已經死亡，神殿的治療不及於此。", c.Name))
		default:
			log = append(log, fmt.Sprintf("%s 的%v已解除。", c.Name, c.Condition))
			c.Condition = CondGood
			c.CondBits = 0
			c.HP = c.MaxHP
		}
	}
	if len(log) == 0 {
		log = append(log, "隊伍沒有需要治療的人。")
	}
	return log
}

// TrainParty 讓夠格的人在訓練所升級。
func (s *Session) TrainParty() []string {
	var log []string
	for i := range s.Party {
		c := &s.Party[i]
		if c.Empty() {
			continue
		}
		gained, err := c.Train(s.Rand)
		if err != nil {
			continue
		}
		log = append(log, fmt.Sprintf("%s 升到第 %d 級，生命上限 +%d（共 %d），法力等級 %d。",
			c.Name, c.Level, gained, c.MaxHP, c.SpellLevel()))
	}
	if len(log) == 0 {
		log = append(log, "沒有人的經驗值足夠受訓。")
	}
	return log
}

// AwardExp 把戰鬥的經驗值分給還站著的人。
//
// 經驗值是**原版的**：怪物記錄第 15 個位元組的位元欄位
// （基數 × 10 的冪次，見 `internal/assets/monsters`）。
// 隊伍裡怎麼分還沒從原版解出來，這裡在還站著的人之間均分。
func (e *Encounter) AwardExp(party []Character) int {
	total := 0
	for _, m := range e.Monsters {
		if mm, ok := m.(*Monster); ok {
			total += mm.Def.Exp
		}
	}
	n := 0
	for i := range party {
		if party[i].Condition.Acts() {
			n++
		}
	}
	if n == 0 || total == 0 {
		return 0
	}
	each := total / n
	for i := range party {
		if party[i].Condition.Acts() {
			party[i].Exp += each
		}
	}
	return each
}

// FacilityKind 是城鎮設施的種類。
type FacilityKind byte

const (
	FacilityNone FacilityKind = iota
	FacilityInn
	FacilityTemple
	FacilityTraining
	FacilityBlacksmith
	FacilityMageGuild
	FacilityTavern
	// FacilityBrainDetox 是 `0e 07`（`2BRAIN` 的另一個入口），
	// 招牌寫 `Brain Detoxification`。功能未實作。
	FacilityBrainDetox
)

var facilityNames = [...]string{"", "旅店", "神殿", "訓練基地", "物品店", "法師公會", "酒館", "大腦淨化"}

func (f FacilityKind) String() string {
	if int(f) >= len(facilityNames) {
		return "未知設施"
	}
	return facilityNames[f]
}

// 招牌字串 → 設施種類。
//
// 設施的招牌格腳本只有 `04 NN`（顯示名稱），真正的入口在相鄰格用
// `0b XX 00 0e YY`（見 docs/formats/07 §7）。`0x0b` 的參數要經 DGROUP 裡
// 的換算表，那張表在 BSS 讀不到 —— 所以這裡改用**招牌名稱**認設施。
//
// 這不是原版的機制，但招牌字串本身是原版資料，比自己編一份座標表可靠。
var facilitySigns = []struct {
	Contains string
	Kind     FacilityKind
}{
	{"Inn", FacilityInn},
	{"Temple", FacilityTemple},
	{"Training", FacilityTraining},
	{"Blacksmith", FacilityBlacksmith},
	{"Mage Guild", FacilityMageGuild},
	{"Tavern", FacilityTavern},
}

// facilityByCode 是 opcode `0x0e` 的子命令 → 設施種類。
//
// 出自原版：`sub_19716` 依子命令跳到不同 overlay 的入口
// （1 → `1RETINN`、2 → `2MISC2`、3 → `2BRAIN`、4／5 → `2TEMPLE` 的兩個
// 入口、6 → `2SMITH`），而資料本身把對應關係釘死了 —— 五座城鎮各有
// 一個 `0e 01`…`0e 06`，每一格旁邊的招牌都同一類：
//
//	01  Carriage Inn、Tundaran Arms Inn                    旅店
//	02  Turkov's Training、Island Training、Training Academy 訓練基地
//	03  Slaughtered Lamb、Boar's Tongue Tavern              酒館
//	04  Gateway Temple、Eleusinian Temple、White Dove Temple 神殿
//	05  Sleepy's Mage Guild、Blackrock Mage Guild            法師公會
//	06  Drewnhald Ironworks、Thundrax Weaponry、Bestway Blacksmith 鐵匠
//
// 這比看招牌字串可靠：招牌是自由文字，設施是資料裡的編號。
var facilityByCode = [...]FacilityKind{
	1: FacilityInn,
	2: FacilityTraining,
	3: FacilityTavern,
	4: FacilityTemple,
	5: FacilityMageGuild,
	6: FacilityBlacksmith,
	7: FacilityBrainDetox,
}

// FacilityByCode 把 opcode `0x0e` 的子命令換成設施種類。
// 子命令 8 以上還沒解（三處，旁邊沒有招牌），一律回 FacilityNone。
func FacilityByCode(code int) FacilityKind {
	if code < 0 || code >= len(facilityByCode) {
		return FacilityNone
	}
	return facilityByCode[code]
}

// FacilityAt 依這一格的事件訊息判斷是哪種設施。
//
// **這是後備**：正式的判準是 opcode `0x0e` 的子命令（見 FacilityByCode）。
// 招牌字串留著是因為它在沒跑腳本的場合（例如只看訊息的工具）仍然管用。
func FacilityAt(message string) FacilityKind {
	for _, s := range facilitySigns {
		if strings.Contains(message, s.Contains) {
			return s.Kind
		}
	}
	return FacilityNone
}

// EnterFacility 進入設施並執行它的功能。
func (s *Session) EnterFacility(k FacilityKind) []string {
	switch k {
	case FacilityInn:
		return append([]string{"進入旅店。"}, s.RestAtInn()...)
	case FacilityTemple:
		// 神殿有選單（`2TEMPLE.OVL` 的四項），由上層開；這裡只報進門。
		return []string{"進入神殿。"}
	case FacilityTraining, FacilityBlacksmith, FacilityMageGuild, FacilityTavern:
		// 這幾家都有選單，由上層開；這裡只報進門。
		return []string{fmt.Sprintf("進入%s。", k)}
	case FacilityBrainDetox:
		// 有選單（挑人），由上層開。
		return []string{fmt.Sprintf("進入%s。", k)}
	}
	return nil
}

// 神殿的服務（`2TEMPLE.OVL`）。
//
// 原版的選單有四項（字串在 `STR.DAT`）：
//
//	A) Restore Cond   sub_1C178
//	B) Restore Algn   sub_1C1B2
//	C) Donations
//	D/E/F) Spell      每座城賣的法術不同（`ds:46DA`）
//
// 每一項都要付錢，價格在 `ds:58E2`／`ds:58E6` —— 那兩格**是執行時填的**，
// 填它的那一段還沒追到，所以金額未知。這裡先不收費，不編一個數字。

// RestoreCondition 是神殿的「恢復狀態」。
//
// `sub_1C178`：付錢之後 `sub_1C698`（生命補到上限，順便寫記錄 `+116`）
// 再把狀況位元組 `+38` 清成 0。**死亡也在這一條裡** —— 原版沒有另外擋，
// 清掉狀況位元組就等於復活。
func (c *Character) RestoreCondition() bool {
	if c.Empty() {
		return false
	}
	changed := c.CondBits != 0 || c.Condition != CondGood || c.HP < c.MaxHP
	if c.HP < c.MaxHP {
		c.HP = c.MaxHP
	}
	c.CondBits = 0
	c.Condition = CondGood
	return changed
}

// RestoreAlignment 是神殿的「恢復陣營」。
//
// `sub_1C1B2` 只做一件事：`[記錄+106] = [記錄+13]` —— 把**原始陣營**抄回
// **當前陣營**。陣營與等級一樣在記錄裡有兩份，一份是本來的、一份是現在的，
// 所謂「恢復」就是把本來的抄回去。
func (c *Character) RestoreAlignment() bool {
	if c.Empty() {
		return false
	}
	if c.FieldByte(offCurAlign) == byte(c.Align) {
		return false
	}
	// SetFieldByte 的參數是 (偏移, 保留遮罩, 新值) —— 遮罩給 0 表示整格換掉。
	c.SetFieldByte(offCurAlign, 0, byte(c.Align))
	return true
}

// TempleService 是神殿的一項服務。
type TempleService int

const (
	TempleRestoreCond TempleService = iota
	TempleRestoreAlign
	TempleDonate
	TempleLeave
)

// TempleServiceNames 是選單上的四項服務。原版的選單還有 D／E／F 三格，
// 那是賣法術（見 TempleStockOf），由上層另外開一個清單。
var TempleServiceNames = [4]string{"恢復狀態", "恢復陣營", "捐獻", "離開"}

// Serve 對整隊執行一項神殿服務。
func (s *Session) Serve(k TempleService) []string {
	var log []string
	switch k {
	case TempleRestoreCond:
		for i := range s.Party {
			c := &s.Party[i]
			price := s.TemplePrice(k, i)
			if price == 0 {
				continue // 完全健康的人不算一項服務
			}
			if !c.pay(price) {
				log = append(log, fmt.Sprintf("%s 付不起 %d 金幣。", c.Name, price))
				continue
			}
			if c.RestoreCondition() {
				log = append(log, fmt.Sprintf("%s 恢復了，花了 %d 金幣。", c.Name, price))
			}
		}
		if len(log) == 0 {
			log = append(log, "隊伍沒有需要治療的人。")
		}
	case TempleRestoreAlign:
		for i := range s.Party {
			c := &s.Party[i]
			price := s.TemplePrice(k, i)
			if price == 0 {
				continue
			}
			if !c.pay(price) {
				log = append(log, fmt.Sprintf("%s 付不起 %d 金幣。", c.Name, price))
				continue
			}
			if c.RestoreAlignment() {
				log = append(log, fmt.Sprintf("%s 的陣營回到%v，花了 %d 金幣。",
					c.Name, c.Align, price))
			}
		}
		if len(log) == 0 {
			log = append(log, "沒有人的陣營需要恢復。")
		}
	case TempleDonate:
		// 捐獻**會清掉詛咒**（`sub_1C1EA`：`mov byte_103DB, 0`）——
		// 那是它真正的作用，不只是一句道謝。
		price := s.TemplePrice(k, 0)
		if len(s.Party) > 0 && s.Party[0].pay(price) {
			cursed := s.World.Globals[globalCurse] > 0
			s.setGlobalAddr(globalCurse, 0)
			log = append(log, fmt.Sprintf("%s（%d 金幣）", donationThanks(), price))
			if cursed {
				log = append(log, "隊伍身上的詛咒消失了。")
			}
		} else {
			log = append(log, fmt.Sprintf("捐獻要 %d 金幣。", price))
		}
	case TempleLeave:
		log = append(log, "隊伍離開神殿。")
	}
	return log
}

// globalCurse 是詛咒計數器（`ds:03DB`）。
const globalCurse = 0x03DB

// TemplePrice 是神殿服務的價錢（`sub_1C6CC` 進來時一次算好六格，
// 存在 `ds:58E2 + i*4`）。三支算式：
//
//	恢復狀態  sub_1C616  基數 × 等級 × 城鎮倍率
//	          基數：狀況 0xFF → 1000；≥ 0x80 → 100；
//	                其餘非零、或生命沒滿 → 10；完全健康 → 0
//	恢復陣營  sub_1C5B8  目前陣營與原始陣營不同才收，100 × 等級 × 城鎮倍率
//	捐獻      sub_1C5A6  100 × 城鎮倍率（不乘等級）
//
// 「等級」讀的是 `+113`（戰鬥等級那一份），城鎮倍率是 `ds:46A8`
// （米德格特 1、亞特蘭汀 5、桑達拉 2、佛卡尼亞 3、桑德索巴 2）。
//
// **價錢 0 表示這一項對這個人不必做** —— 原版就是拿 0 當「不提供」的旗標
// （`sub_1C2B4` 開頭 `or ax,[bx+58E4h]; jnz` 之後才往下走）。
func (s *Session) TemplePrice(k TempleService, who int) int {
	if data == nil || who < 0 || who >= len(s.Party) {
		return 0
	}
	c := &s.Party[who]
	mul := data.Creation.TempleMultiplier(s.World.MapIndex)
	switch k {
	case TempleDonate:
		return 100 * mul
	case TempleRestoreAlign:
		// `Align` 是原始陣營（`+13`），`+106` 是目前的。
		if c.FieldByte(offCurAlign) == byte(c.Align) {
			return 0
		}
		return 100 * c.BattleLevel * mul
	case TempleRestoreCond:
		base := 0
		switch {
		case c.CondBits == 0xFF:
			base = 1000
		case c.CondBits >= 0x80:
			base = 100
		case c.CondBits != 0 || c.HP < c.MaxHP:
			base = 10
		}
		return base * c.BattleLevel * mul
	}
	return 0
}

// donationThanks 是捐獻的答謝（`STR.DAT` 神殿那一段）。
func donationThanks() string {
	if text == nil {
		return "  Your generosity is greatly appreciated."
	}
	return text.Or("temple.thanks", "  非常感謝 你的慷慨。")
}

// 旅店（`1RETINN.OVL`）是**名冊與隊伍的編組畫面**，不只是休息。
//
// 畫面上的鍵：`A`–`X` 檢視某個角色、`Ctrl` + `A`–`X` 加入／移出隊伍、
// 空白鍵在「角色」與「傭兵」兩張清單之間切換、`1`–`5` 去別的城鎮的旅店、
// `V` 看法術書、`Z` 離開。隊伍滿了會印 `*** Party is Full ***`。

// MaxParty 之外的上限：名冊一次顯示 A–X 共 24 個。
const RosterPageSize = 24

// AddToParty 把名冊第 i 個編進隊伍。
//
// 隊伍滿了回 false —— 原版印 `*** Party is Full ***`（`ds:219B`）。
func (s *Session) AddToParty(i int) (string, bool) {
	if i < 0 || i >= len(s.Roster) {
		return "沒有這個人。", false
	}
	if len(s.Party) >= MaxParty {
		return partyFullMsg(), false
	}
	c := s.Roster[i]
	for _, p := range s.Party {
		if p.Name == c.Name {
			return c.Name + " 已經在隊伍裡了。", false
		}
	}
	s.Party = append(s.Party, c)
	return fmt.Sprintf("%s 加入隊伍，目前 %d 人。", c.Name, len(s.Party)), true
}

// RemoveFromParty 把隊伍第 i 個移回名冊。
func (s *Session) RemoveFromParty(i int) (string, bool) {
	if i < 0 || i >= len(s.Party) {
		return "沒有這個人。", false
	}
	c := s.Party[i]
	// 移出時把當前狀態寫回名冊 —— 隊伍裡受的傷、升的級都要留著。
	for j := range s.Roster {
		if s.Roster[j].Name == c.Name {
			s.Roster[j] = c
			break
		}
	}
	s.Party = append(s.Party[:i], s.Party[i+1:]...)
	return fmt.Sprintf("%s 離開隊伍，目前 %d 人。", c.Name, len(s.Party)), true
}

// partyFullMsg 是 `ds:219B`。
func partyFullMsg() string {
	if text == nil {
		return "*** Party is Full ***"
	}
	return text.Or("exe.219B", "*** 隊伍已滿 ***")
}

// 大腦淨化（`2BRAIN.OVL` 的 `_2brain_e01`，執行時偏移 `0xC7E2`）。
//
// 招牌是 `Brain Detoxification`，畫面上的四行是：
//
//	The surgically garbed Cerebral
//	Detoxification Specialist will cleanse
//	a party member of all secondary skills
//	for 100 gold.  Pay (y/n)?
//
// 流程（`_2brain_e01`）：問 Y／N → 問挑誰（`1`–隊伍人數）→ 檢查黃金
// 是否 ≥ 100（`cmp [bx+66h], 64h`，不足就印 `ds:41C8`）→ 對兩項第二技能
// 各呼叫一次 `sub_1C5CA` 把技能給的加值扣回去 → `+80` 清成 0。
//
// **原版只檢查黃金，沒有扣**（`_2brain_e01` 整支沒有寫回 `+0x66`）。
// remake 照做 —— 這是原版的行為，不是漏實作。

// DetoxPrice 是大腦淨化的收費（原版只驗、不扣，見上）。
const DetoxPrice = 100

// skillBonus 是每個第二技能給的加值：技能編號 → (屬性, 加值)。
//
// 讀自 `sub_1C5CA` 的 15 路跳表 —— 它在清除技能時把加值扣回去，
// 所以「扣了什麼」就等於「當初給了什麼」。七個有屬性效果的技能與
// 手冊寫的**七比七全中**，這也是屬性順序的三條證據之一（見 Stat）。
//
// 沒列出來的技能（製圖家、宗教家、商人、登山家、領航員、探險家）
// 不動屬性，它們的效果在別處：登山家與探險家是野外通行條件、
// 製圖家對應遊戲裡的地圖畫面。
var skillBonus = map[int]struct {
	Stat  Stat
	Delta int
}{
	1: {Accuracy, 5},    // 武器專家
	2: {Speed, 5},       // 運動家
	5: {Personality, 5}, // 外交家
	6: {Luck, 5},        // 賭徒
	7: {Might, 5},       // 鬥士
	9: {Intellect, 5},   // 語言家
}

// skillThievery 是扒手加的盜行（`sub_1C5CA` case 14：`+0x1E` 減 15）。
const skillThievery = 15

// skillEndurance 是戰士加的耐力（case 15：`+0x27`／`+0x73` 各減 5）。
const skillEndurance = 5

// heroBonus 是勇士（case 8）加在**每一項**上的值：六項屬性、盜行與耐力
// 各 1（`sub_1C5CA` 對十四個欄位各呼叫一次，量都是 1）。
const heroBonus = 1

// Detox 清掉一名隊員的兩項第二技能，並把技能給的加值扣回去。
func (s *Session) Detox(who int) []string {
	if who < 0 || who >= len(s.Party) {
		return []string{"沒有這個人。"}
	}
	c := &s.Party[who]
	if c.Gold < DetoxPrice {
		return []string{detoxMsg("exe.41C8", "抱歉，你必須有 100 金幣。")}
	}
	if c.Skills[0] == 0 && c.Skills[1] == 0 {
		return []string{c.Name + " 沒有次要技能。"}
	}
	for _, sk := range c.Skills {
		s.removeSkill(c, sk)
	}
	c.Skills = [2]int{0, 0}
	if len(c.Raw) == RecordSize {
		c.Raw[offSkills] = 0
	}
	c.RecomputeAC()
	return []string{detoxMsg("exe.41E7", "他們的次要技能都消失了。")}
}

// removeSkill 把一個第二技能給的加值扣回去（`sub_1C5CA`）。
func (s *Session) removeSkill(c *Character, skill int) {
	switch skill {
	case 0:
		return
	case 8: // 勇士：所有基本屬性、盜行與耐力各減一
		for st := Stat(0); st < NumStats; st++ {
			c.addStat(st, -heroBonus)
		}
		c.addField(offThief, -heroBonus)
		c.addField(offEndB, -heroBonus)
		c.addField(offEnd, -heroBonus)
		return
	case 14: // 扒手
		c.addField(offThief, -skillThievery)
		return
	case 15: // 戰士
		c.addField(offEndB, -skillEndurance)
		c.addField(offEnd, -skillEndurance)
		return
	}
	if b, ok := skillBonus[skill]; ok {
		c.addStat(b.Stat, -b.Delta)
	}
}

// addStat 同時調整屬性的基礎值與當前值（原版兩份都改，見 `sub_1C5CA`）。
func (c *Character) addStat(st Stat, d int) {
	if st < 0 || st >= NumStats {
		return
	}
	c.Base[st] = clampByte(c.Base[st] + d)
	c.Current[st] = clampByte(c.Current[st] + d)
	if len(c.Raw) == RecordSize {
		c.Raw[offStats+int(st)] = byte(c.Base[st])
		c.Raw[offCur+int(st)] = byte(c.Current[st])
	}
}

// addField 調整記錄裡的一個位元組欄位，夾在 0–255。
func (c *Character) addField(off, d int) {
	if len(c.Raw) != RecordSize {
		return
	}
	v := clampByte(int(c.Raw[off]) + d)
	c.Raw[off] = byte(v)
	switch off {
	case offThief:
		c.Thievery = v
	case offEnd:
		c.Endurance = v
	}
}

func clampByte(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func detoxMsg(key, fallback string) string {
	if text == nil {
		return fallback
	}
	return text.Or(key, fallback)
}

// 酒館的競技賽（`2BRAIN.OVL` 的 `_2brain_e00`，`0e 03` 進來）。
//
// 原版的酒館只有這一件事：**要有入場券才打得成**。四張券是物品 208–211
// （綠、黃、紅、黑），階層就是券號減 208 —— 券越黑對手越強、獎金越高。
//
// 流程：掃全隊背包找券（`cmp dl,0D0h` / `cmp dl,0D3h`）→ 沒有就印
// 「抱歉，要有入場券才能參加這些競技。」→ 有就收走那張券，
// 開一場**每名隊員各一隻**的戰鬥，對手編號是
//
//	((階層×3 + 城鎮基數) << 4) + rand(1, 16)
//
// 打贏之後照「階層 × 城鎮」發獎金（200–50,000），並在記錄 `+126`／`+127`
// 點一個獎章位元 —— 十二種組合各一個位元，所以那兩個位元組記的是
// 「這一階、這座城的競技賽贏過了」。

// ArenaTicket 是入場券的物品編號範圍。
const (
	ArenaTicketFirst = 208 // Green Ticket
	ArenaTicketLast  = 211 // Black Ticket
)

// arenaBadgeBase 是獎章位元在角色記錄裡的基底（`lea ax,[bx+di+79h]`）。
const arenaBadgeBase = 0x79

// ArenaEntry 是一次競技賽的入場結果。
type ArenaEntry struct {
	// Lines 是要播報的話。
	Lines []string
	// Tier 是券的階層（0–3），Ready 為 false 時無意義。
	Tier int
	// Ready 表示券收走了、可以開打。
	Ready bool
}

// EnterArena 檢查入場券並收走它。回傳的 Ready 為 true 時由呼叫端開戰。
func (s *Session) EnterArena() ArenaEntry {
	for i := range s.Party {
		c := &s.Party[i]
		for slot, it := range c.Backpack() {
			if it.ID < ArenaTicketFirst || it.ID > ArenaTicketLast {
				continue
			}
			tier := it.ID - ArenaTicketFirst
			c.RemovePackItem(slot)
			return ArenaEntry{Tier: tier, Ready: true, Lines: []string{
				arenaMsg("exe.409D", "競技主持人收下你的入場券。") +
					arenaMsg("exe.40C3", " 戰鬥開始！"),
			}}
		}
	}
	return ArenaEntry{Lines: []string{
		arenaMsg("exe.4060", "抱歉，要有入場券才能參加") +
			arenaMsg("exe.4085", "這些競技。"),
	}}
}

// ArenaEncounter 排出競技賽的對手：每名隊員各一隻，全部同一種。
func (s *Session) ArenaEncounter(tier int) *Encounter {
	if data == nil {
		return nil
	}
	base := data.ArenaMonster(tier, s.World.MapIndex)
	id := base + s.Rand.Range(1, 16)
	ids := make([]int, 0, len(s.Party))
	for range s.Party {
		ids = append(ids, id)
	}
	return s.fixedEncounter(ids)
}

// ArenaReward 發競技賽的獎金與獎章。打贏之後呼叫。
//
// **獎金只給第一個人**（原版加完就把金額清成 0 再繼續迴圈），
// 獎章則是全隊每個人都點。
func (s *Session) ArenaReward(tier int) []string {
	if data == nil || len(s.Party) == 0 {
		return nil
	}
	gold, off, bit := data.ArenaReward(tier, s.World.MapIndex)
	if off == 0 && bit == 0 {
		return nil
	}
	paid := false
	for i := range s.Party {
		c := &s.Party[i]
		if !paid {
			c.Gold += gold
			c.SetFieldValue(offGold, 4, uint32(c.Gold))
			paid = true
		}
		if len(c.Raw) == RecordSize {
			c.Raw[arenaBadgeBase+off] |= byte(bit)
		}
	}
	return []string{fmt.Sprintf("%s%d%s",
		arenaMsg("exe.40DA", "優勝者，你獲得 "), gold,
		arenaMsg("exe.40EF", " 金幣"))}
}

func arenaMsg(key, fallback string) string {
	if text == nil {
		return fallback
	}
	return text.Or(key, fallback)
}


// pay 扣錢，不夠就一毛不動（`sub_1C326`：先比大小再減）。
func (c *Character) pay(n int) bool {
	if n <= 0 {
		return true
	}
	if c.Gold < n {
		return false
	}
	c.Gold -= n
	c.SetFieldValue(offGold, 4, uint32(c.Gold))
	return true
}
