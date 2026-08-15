package game

import "fmt"

// 特殊互動裝置：事件腳本 `0x0e` 分派給 `2CAVES.OVL` 的那幾支。
//
// 它們與旅店、神殿那些設施走同一個 opcode，但不是設施 —— 原版
// `2PLAY.img` 的 `sub_19716` 依代碼直接呼叫 `2CAVES` 的進入點。
// 完整分派表與每一支的證據見 `docs/re/02-2caves-special-events.md`。

// CaveDevice 是踩到的特殊裝置。
type CaveDevice byte

const (
	DeviceNone CaveDevice = iota
	// DeviceTeleport 是 `0e 7E`：問 X／Y (0-15)，同一張圖內傳送。
	DeviceTeleport
	// DeviceGoldExp 是 `0e CB`：選一名角色，黃金全數換成經驗。
	DeviceGoldExp
	// DeviceGemExp 是 `0e CD`：選一名角色，寶石換成經驗。
	DeviceGemExp
	// DeviceEraGate 是 `0e CF`：年代之門。
	DeviceEraGate
	// DeviceMaxHP 是 `0e CE`：選一名角色，把生命上限重算到該有的值。
	DeviceMaxHP
	// DeviceCircus 是 `0e 64`：馬戲團，七個攤位。
	DeviceCircus
	// DeviceFight 是 `0e 7F`：擲一隻怪，每名隊員各一隻，當場開打。
	DeviceFight
	// DeviceRack1–3 是 `0e 81`–`0e 83`：武器架、弓架、防具架，
	// 各自從一段連續的物品編號裡擲一件出來。
	DeviceRack1
	DeviceRack2
	DeviceRack3
	// DeviceTripleGold 是 `0e CC`：全隊的黃金三倍換成經驗。
	DeviceTripleGold
)

// NeedsUI 回報這個裝置要不要玩家輸入。不要的那幾支由 `Session.Step`
// 當場做完，要的交給 UI 開畫面。
func (d CaveDevice) NeedsUI() bool {
	switch d {
	case DeviceFight, DeviceRack1, DeviceRack2, DeviceRack3, DeviceTripleGold:
		return false
	}
	return d != DeviceNone
}

// caveDeviceByCode 把 `0x0e` 的子命令換成裝置。**滑梯陷阱（`0e 80`）
// 不在這裡** —— 它不需要玩家輸入，在腳本裡當場做完。
func caveDeviceByCode(code int) CaveDevice {
	switch code {
	case 0x7E:
		return DeviceTeleport
	case 0xCB:
		return DeviceGoldExp
	case 0xCD:
		return DeviceGemExp
	case 0xCF:
		return DeviceEraGate
	case 0xCE:
		return DeviceMaxHP
	case 0x64:
		return DeviceCircus
	case 0x7F:
		return DeviceFight
	case 0x81:
		return DeviceRack1
	case 0x82:
		return DeviceRack2
	case 0x83:
		return DeviceRack3
	case 0xCC:
		return DeviceTripleGold
	}
	return DeviceNone
}

// slideCode 是魔法滑梯陷阱的子命令。
const slideCode = 0x80

// slideFrom／slideTo 是滑梯的來源與目的座標，各十組。
// 原版在 `ds:3486`（來源 X）、`ds:3490`（來源 Y）、`ds:349A`（目的 X）、
// `ds:34A4`（目的 Y），比對用的是**目前座標**，不看地圖編號。
var slideFrom = [10][2]int{
	{1, 13}, {1, 2}, {4, 5}, {4, 10}, {7, 2},
	{7, 13}, {5, 10}, {10, 10}, {13, 2}, {13, 13},
}
var slideTo = [10][2]int{
	{1, 14}, {1, 1}, {4, 4}, {4, 11}, {7, 1},
	{7, 14}, {10, 4}, {10, 11}, {13, 1}, {13, 14},
}

// eraDest 是年代之門的八個目的地：地圖、X、Y。
// 原版在 `ds:36EA`（地圖）、`ds:36DA`（X）、`ds:36E2`（Y）。
var eraDest = [8][3]int{
	{15, 0, 0},
	{5, 0, 15},
	{33, 15, 15},
	{40, 15, 0},
	{11, 7, 6},
	{37, 5, 5},
	{6, 8, 3},
	{38, 14, 4},
}

// eraFirstCentury 是「開始改寫世紀」的選項編號。**選項 1–4 只傳送、
// 不改世紀**：原版是 `cmp [bp+var_8], 5 / jl` 跳過那一行寫入。
const eraFirstCentury = 5

// eraFlagOffset／eraFlagBit 是開門條件：隊上任一人的記錄 `+128` bit 1。
//
// 該位元的**寫入端還沒找到**（掃過全部 `.asm` 的 `[reg+80h]` 形式只有
// 這一處讀取），所以語意未知；這裡照原版的判斷照做，不猜它代表什麼。
const (
	eraFlagOffset = 128
	eraFlagBit    = 0x02
)

// slideHalved 是滑梯陷阱要右移一位的記錄偏移。
//
// 原版逐條 `shr byte ptr [si+N], 1`，外加 `shr word ptr [si+58h], 1`。
// 這一段全是「當前值」的鏡像：`+106` 目前陣營、`+107`–`+112` 六個屬性的
// 當前值、`+113` 戰鬥等級、`+114` 法力等級、`+115` 耐力，再加 `+88` 目前 SP。
// 沒有機率、沒有豁免判定 —— 控制流從頭到尾沒有分支。
var slideHalved = []int{106, 107, 108, 109, 110, 111, 112, 113, 114, 115}

// caveText 取譯文；沒載入翻譯層時用原文。`text` 是可以為 nil 的全域，
// 直接呼叫 `text.Or` 會在沒有翻譯的環境（單元測試、工具）panic。
func caveText(key, fallback string) string {
	if text == nil {
		return fallback
	}
	return text.Or(key, fallback)
}

// EraOption 是年代之門的一個選項。
type EraOption struct {
	Map, X, Y int
	// Century 是選了之後要寫進的世紀，0 表示不改。
	Century int
}

// EraOptions 回傳八個選項，順序就是原版畫面上的 1–8。
func EraOptions() []EraOption {
	out := make([]EraOption, 0, len(eraDest))
	for i, d := range eraDest {
		c := 0
		if i+1 >= eraFirstCentury {
			c = i + 1
		}
		out = append(out, EraOption{Map: d[0], X: d[1], Y: d[2], Century: c})
	}
	return out
}

// EraGateOpen 回報隊伍開不開得了年代之門。
func (s *Session) EraGateOpen() bool {
	for i := range s.Party {
		if s.Party[i].FieldByte(eraFlagOffset)&eraFlagBit != 0 {
			return true
		}
	}
	return false
}

// EnterEra 選一個年代（1–8）。回傳要播報的話；選項不合法就什麼都不做。
func (s *Session) EnterEra(option int) []string {
	if option < 1 || option > len(eraDest) {
		return nil
	}
	d := eraDest[option-1]
	if option >= eraFirstCentury {
		s.World.SetGlobal(globalSelCentury, byte(option))
	}
	s.World.MapIndex = d[0]
	s.World.X, s.World.Y = d[1], d[2]
	s.World.ClearTravelEffects()
	s.World.Teleported = true
	return []string{caveText("ui.era.enter", "門後的景色換了。")}
}

// MagicLocation 是座標傳送機：**不換地圖**，只把隊伍移到同一張圖的
// (x, y)。原版問兩個 0–15 的數字，直接寫 `ds:0393`／`ds:0394`。
func (s *Session) MagicLocation(x, y int) bool {
	if x < 0 || x > 15 || y < 0 || y > 15 {
		return false
	}
	s.World.X, s.World.Y = x, y
	s.World.ClearTravelEffects()
	s.World.Teleported = true
	return true
}

// TradeGoldForExp 是 `0e CB`：把一名角色的黃金全數換成經驗，**1 金 1 點**。
// member 是 1 起算的隊伍位置。
func (s *Session) TradeGoldForExp(member int) []string {
	c := s.member(member)
	if c == nil {
		return []string{caveText("ui.cave.nobody", "那個位置沒有人。")}
	}
	if c.Gold == 0 {
		return []string{caveText("ui.cave.nogold", "他身上沒有黃金。")}
	}
	// **要寫回 Raw**：解析出來的欄位只是工作副本，下一次 SetField*
	// 會從 Raw 重新解析，只改結構欄位的話等於沒改。
	gained := c.Gold
	c.SetFieldValue(offExp, 4, uint32(c.Exp+gained))
	c.SetFieldValue(offGold, 4, 0)
	return []string{fmt.Sprintf(caveText("ui.cave.goldexp", "%s 交出 %d 金，換得 %d 點經驗。"),
		c.Name, gained, gained)}
}

// DonateGemsForExp 是 `0e CD`：寶石換經驗，**一顆十點**。
//
// 原版把 `((g×4)+g)×2` 拆成移位加法算，等價於 ×10。
func (s *Session) DonateGemsForExp(member int) []string {
	c := s.member(member)
	if c == nil {
		return []string{caveText("ui.cave.nobody", "那個位置沒有人。")}
	}
	if c.Gems == 0 {
		return []string{caveText("ui.cave.nogems", "他身上沒有寶石。")}
	}
	gems := c.Gems
	gained := gems * 10
	c.SetFieldValue(offExp, 4, uint32(c.Exp+gained))
	c.SetFieldValue(offGems, 2, 0)
	return []string{fmt.Sprintf(caveText("ui.cave.gemexp", "%s 捐出 %d 顆寶石，換得 %d 點經驗。"),
		c.Name, gems, gained)}
}

// member 取 1 起算的隊員；位置空著回 nil。
func (s *Session) member(i int) *Character {
	if i < 1 || i > len(s.Party) {
		return nil
	}
	return &s.Party[i-1]
}

// slideTrap 是 `0e 80`：踩到就跑，沒有判定。
//
// 目前座標要落在 slideFrom 才會發動 —— 原版拿 `ds:0393`／`ds:0394` 去查
// 那十組，沒中就直接返回。發動時清掉目前格屬性層的 bit 7、換到對應的
// 目的座標，然後把全隊的「當前值」鏡像與目前 SP 全部右移一位。
func (w *World) slideTrap() []string {
	idx := -1
	for i, f := range slideFrom {
		if f[0] == w.X && f[1] == w.Y {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	w.clearAttrTop(w.X, w.Y)
	w.X, w.Y = slideTo[idx][0], slideTo[idx][1]
	w.ClearTravelEffects()
	w.Teleported = true
	for i := range w.Party {
		c := &w.Party[i]
		for _, off := range slideHalved {
			c.SetFieldByte(off, 0, c.FieldByte(off)>>1)
		}
		c.SetFieldValue(offSP, 2, uint32(c.SP)>>1)
	}
	return []string{caveText("exe.3472", "Magical slide trap!")}
}

// ── 不需要玩家輸入的那幾支 ────────────────────────────────────────────

// rackRange 是三個架子的物品編號範圍：`ds:34C0` 是件數、`ds:34C4` 是起點。
//
// 三段各自成類，這是「起點與件數讀對了」的獨立佐證：
// 66–78 十三種長柄與鈍器（Staff…Flamberge）、92–97 六種遠程（Blowpipe…
// Great Bow）、127–133 七種護甲（Padded Armor…Plate Mail）。
var rackRange = map[CaveDevice][2]int{
	DeviceRack1: {66, 13},
	DeviceRack2: {92, 6},
	DeviceRack3: {127, 7},
}

// TakeFromRack 從架子上擲一件放進隊伍的第一個空背包格。
//
// **是擲不是選**：原版呼叫的 `rand(1, 件數)`（root `sub_11C88`），
// 不是輸入函式。找不到空位就什麼都不發生，那一格也不會被用掉。
func (s *Session) TakeFromRack(d CaveDevice) []string {
	r, ok := rackRange[d]
	if !ok || s.Rand == nil {
		return nil
	}
	id := r[0] + s.Rand.Range(1, r[1]) - 1
	for i := range s.Party {
		c := &s.Party[i]
		for slot := 0; slot < 6; slot++ {
			if c.FieldByte(offPackID+slot) != 0 {
				continue
			}
			c.SetFieldByte(offPackID+slot, 0, byte(id))
			c.SetFieldByte(offPackCharge+slot, 0, 0)
			c.SetFieldByte(offPackAttr+slot, 0, 0)
			s.World.clearAttrTop(s.World.X, s.World.Y)
			return []string{caveText("exe.34AE", "你找到了 ") + s.itemName(id)}
		}
	}
	return nil
}


// TripleGold 是 `0e CC`：全隊每個人的黃金乘三變成經驗，黃金歸零。
//
// 原版的訊息是 `Your experience has multiplied three-fold.`，但算的是黃金
// ——`[+102] × 3` 加進 `[+98]` 之後把 `[+102]` 清成 0，控制流沒有分支。
// 雇傭兵（`ds:0416[i] >= 24`）跳過。
func (s *Session) TripleGold() []string {
	for i := range s.Party {
		c := &s.Party[i]
		c.SetFieldValue(offExp, 4, uint32(c.Exp+c.Gold*3))
		c.SetFieldValue(offGold, 4, 0)
	}
	return []string{caveText("exe.3EBB.dup", "你的經驗變成了三倍。")}
}

// FightRoll 是 `0e 7F`：擲一隻怪，每名隊員各一隻。
//
// 怪物編號 = `rand(1, 16) − 1 + Y × 16`，也就是**用隊伍所在的列決定
// 那一段十六隻**。原版把編號填進場上位置陣列 `ds:9680`，筆數是隊伍人數。
func (s *Session) FightRoll() *Encounter {
	if s.Rand == nil {
		return nil
	}
	id := s.Rand.Range(1, 16) - 1 + s.World.Y*16
	ids := make([]int, 0, len(s.Party))
	for range s.Party {
		ids = append(ids, id)
	}
	return s.fixedEncounter(ids)
}

// ── 生命上限重算（`0e CE`）────────────────────────────────────────────

// hpPerLevel 是職業的每級生命（`ds:36B4`，索引 0–7）。
var hpPerLevel = [8]int{12, 10, 10, 8, 6, 8, 8, 15}

// MaxHPTarget 算出這名角色「該有的」生命上限：
//
//	(耐力的屬性修正 + 職業每級生命) × 經驗等級
//
// 耐力修正走的是全遊戲共用的那張門檻表（`ds:4D84`，root `sub_1354A`）。
func MaxHPTarget(c *Character) int {
	if data == nil {
		return 0
	}
	per := data.StatBonus(int(c.FieldByte(offEndB)))
	if cls := int(c.Class); cls >= 0 && cls < len(hpPerLevel) {
		per += hpPerLevel[cls]
	}
	v := per * c.Level
	if v < 0 {
		v = 0
	}
	return v
}

// maxHPGoldGate 是重算生命上限要的黃金。原版比的是**黃金的高位 word**
// （`cmp word ptr [bx+68h], 0Fh`），所以門檻是 15 × 65536。
const maxHPGoldGate = 15 * 65536

// RecomputeMaxHP 是 `0e CE`：把一名角色的生命上限重算成該有的值。
//
// 兩道關卡照原版：目前的基礎上限已經到了就回「You're maxxed out.」，
// 黃金不夠就回「Not enough gold.」。**原版不扣錢**，只驗。
func (s *Session) RecomputeMaxHP(member int) []string {
	c := s.member(member)
	if c == nil {
		return []string{caveText("exe.3610", "這座泉水不理會雇傭兵。")}
	}
	want := MaxHPTarget(c)
	if c.BaseMaxHP >= want {
		return []string{caveText("exe.363E", "你已經到頂了。")}
	}
	if c.Gold < maxHPGoldGate {
		return []string{caveText("exe.3652", "黃金不夠。")}
	}
	c.SetFieldValue(offBaseMaxHP, 2, uint32(want))
	c.SetFieldValue(offMaxHP, 2, uint32(want))
	return []string{fmt.Sprintf(caveText("ui.cave.maxhp", "%s 的生命上限提高到 %d。"), c.Name, want)}
}

// ── 馬戲團（`0e 64`）─────────────────────────────────────────────────

// CircusBooth 是一個攤位：名字、贏了加哪個欄位、贏與輸的台詞。
type CircusBooth struct {
	Name    string
	Offset  int
	WinKey  string
	Win     string
	LoseKey string
	Lose    string
}

// circusBooths 是七個攤位。欄位偏移出自 `sub_1C83E` 的分派
// （arg 0 → `+16`、1 → `+18`、2 → `+20`、3 → `+39`、4 → `+19`、
// 5 → `+21`、其餘 → `+17`），台詞出自 `ds:3A18` 的十六項表：
// 前七個是贏、接著七個是輸、最後一個是安慰獎。
var circusBooths = [7]CircusBooth{
	{"力量試驗", 16, "exe.3811", "你敲響了鈴！力量 +10。", "exe.390C", "你使盡了力氣，還是不夠。"},
	{"接吻亭", 18, "exe.3836", "你得手了！人格 +10。", "exe.392E", "你被拒絕了！"},
	{"擲蹄鐵", 20, "exe.385A", "正中木樁！準確度 +10。", "exe.394A", "你沒中。"},
	{"浸水桶", 39, "exe.3880", "你撐過來了！耐力 +10。", "exe.3964", "你昏了過去。"},
	{"布袋賽跑", 19, "exe.38A4", "你贏了！速度 +10。", "exe.3980", "你摔了個狗吃屎。"},
	{"幸運骰", 21, "exe.38C2", "你擲出了七點！運氣 +10。", "exe.399F", "你沒擲出那個七點。"},
	{"猜殼遊戲", 17, "exe.38E5", "記性驚人！智慧 +10。", "exe.39C3", "你挑錯了殼。"},
}

// CircusBooths 回傳七個攤位，順序就是原版選單上的 1–7。
func CircusBooths() []CircusBooth { return circusBooths[:] }

// circusWinFlag 是「下一攤必贏」的旗標：角色記錄 `+125` 的 bit 1。
// 用掉就清掉（原版 `and byte ptr [si+7Dh], 0FDh`）。
const (
	circusWinOffset = 125
	circusWinBit    = 0x02
)

// circusPrize 是安慰獎的物品編號（`Cupie Doll`）與中獎機率的分母。
// 原版擲 `rand(1, 254)`，`<= 127` 才給 —— 也就是一半。
const (
	circusPrizeItem = 218
	circusPrizeRoll = 254
	circusPrizeMax  = 127
)

// CircusWins 回報隊伍手上有沒有「必贏」的旗標。
func (s *Session) CircusWins() bool {
	for i := range s.Party {
		if s.Party[i].FieldByte(circusWinOffset)&circusWinBit != 0 {
			return true
		}
	}
	return false
}

// PlayCircus 玩一個攤位（0–6）。
//
// 有旗標就必贏：全隊每個帶旗標的人各加 10 點那一項（上限 100，原版是
// 「大於 90 就設成 100」），旗標用掉。沒旗標就是輸，另擲一半的機率拿
// 安慰獎 Cupie Doll。
func (s *Session) PlayCircus(booth int) []string {
	if booth < 0 || booth >= len(circusBooths) {
		return nil
	}
	b := circusBooths[booth]
	if s.CircusWins() {
		for i := range s.Party {
			c := &s.Party[i]
			if c.FieldByte(circusWinOffset)&circusWinBit == 0 {
				continue
			}
			c.SetFieldByte(circusWinOffset, ^byte(circusWinBit), 0)
			v := int(c.FieldByte(b.Offset))
			if v > 90 {
				v = 100
			} else {
				v += 10
			}
			c.SetFieldByte(b.Offset, 0, byte(v))
		}
		return []string{caveText(b.WinKey, b.Win)}
	}
	lines := []string{caveText(b.LoseKey, b.Lose)}
	if s.Rand != nil && s.Rand.Range(1, circusPrizeRoll) <= circusPrizeMax {
		for i := range s.Party {
			c := &s.Party[i]
			for slot := 0; slot < 6; slot++ {
				if c.FieldByte(offPackID+slot) != 0 {
					continue
				}
				c.SetFieldByte(offPackID+slot, 0, circusPrizeItem)
				c.SetFieldByte(offPackCharge+slot, 0, 0)
				c.SetFieldByte(offPackAttr+slot, 0, 0)
				return []string{caveText("exe.39E4", "你贏到一個娃娃！")}
			}
		}
	}
	return lines
}
