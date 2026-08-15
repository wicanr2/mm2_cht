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
)

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
	gained := c.Gold
	c.Exp += gained
	c.Gold = 0
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
	c.Exp += gained
	c.Gems = 0
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
