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
	case FacilityTraining:
		return append([]string{"進入訓練基地。"}, s.TrainParty()...)
	case FacilityBlacksmith, FacilityMageGuild, FacilityTavern, FacilityBrainDetox:
		return []string{fmt.Sprintf("進入%s。（功能未實作）", k)}
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

// TempleServiceNames 是選單上的四項。
var TempleServiceNames = [4]string{"恢復狀態", "恢復陣營", "捐獻", "離開"}

// Serve 對整隊執行一項神殿服務。
func (s *Session) Serve(k TempleService) []string {
	var log []string
	switch k {
	case TempleRestoreCond:
		for i := range s.Party {
			c := &s.Party[i]
			if c.RestoreCondition() {
				log = append(log, c.Name+" 恢復了。")
			}
		}
		if len(log) == 0 {
			log = append(log, "隊伍沒有需要治療的人。")
		}
	case TempleRestoreAlign:
		for i := range s.Party {
			c := &s.Party[i]
			if c.RestoreAlignment() {
				log = append(log, fmt.Sprintf("%s 的陣營回到%v。", c.Name, c.Align))
			}
		}
		if len(log) == 0 {
			log = append(log, "沒有人的陣營需要恢復。")
		}
	case TempleDonate:
		log = append(log, donationThanks())
	case TempleLeave:
		log = append(log, "隊伍離開神殿。")
	}
	return log
}

// donationThanks 是捐獻的答謝（`STR.DAT` 神殿那一段）。
func donationThanks() string {
	if text == nil {
		return "  Your generosity is greatly appreciated."
	}
	return text.Or("temple.thanks", "  非常感謝 你的慷慨。")
}
