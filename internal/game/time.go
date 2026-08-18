package game

// 走一步過多少時間。
//
// 出自 root `0x150CE`（引數是步數）：
//
//	if 步數 == 1 && (ds:59C8 & 0x20) && ds:03D5 != 0: dec ds:03D5   ; 吃照明的格
//	ds:03CC += 步數
//	if ds:03CC >= 0x100:
//	    day = ++日期表[索引]                      ; ds:03A2 + ds:03CA×2
//	    ds:03CC %= 0x100
//	    sub_15092()                               ; 每個隊員的年齡加一天
//	    if day 是 60／120／180: ds:03F4 = ds:03F5 = 0
//	    if 日期表[索引] > 180:
//	        日期表[索引] = 1
//	        年份表[索引]++（到 999 為止）          ; ds:03B6 + ds:03CA×2
//	        ds:03EA = 0
//
// 所以原版的時間單位就是**步**：256 步一天、180 天一年。`ds:03CC` 是
// 「今天走到哪裡」——它是一個真的日內時鐘，只是原版的繪圖路徑一處都
// 沒讀它（掃過 `ds:03CA`–`ds:03CE` 的全部使用點，只有這一支在動它）。
const (
	// clockAddr 是今天走了幾步（0–255）。
	clockAddr = 0x03CC
	// yearTable 與 dayTable 同索引（`ds:03CA`）。索引 9 指到的是
	// `ds:03B4`（日）與 `ds:03C8`（年），與畫面上的 `Day=`／`Year=` 一致。
	yearTable = 0x03B6

	stepsPerDay = 256
	daysPerYear = 180
	maxYear     = 999

	// dayResetA／dayResetB 是那兩個在 60／120／180 天被清掉的位元組。
	// 語意未解 —— 只知道被清，所以照清不猜用途。
	dayResetA = 0x03F4
	dayResetB = 0x03F5
	// yearResetAddr 是換年時被清掉的那一個，語意同樣未解。
	yearResetAddr = 0x03EA
)

func (w *World) word(addr uint16) int {
	return int(w.Globals[addr]) | int(w.Globals[addr+1])<<8
}

func (w *World) setWord(addr uint16, v int) {
	w.Globals[addr] = byte(v)
	w.Globals[addr+1] = byte(v >> 8)
}

// Clock 回傳今天走了幾步（0–255）。
func (w *World) Clock() int {
	if w.Globals == nil {
		return 0
	}
	return w.word(clockAddr)
}

// StepTime 推進 n 步的時間。走一步的照明消耗也在這裡付。
func (w *World) StepTime(n int) {
	if w.Globals == nil || n <= 0 {
		return
	}
	if n == 1 {
		w.burnLight()
	}
	clock := w.Clock() + n
	if clock < stepsPerDay {
		w.setWord(clockAddr, clock)
		return
	}
	w.setWord(clockAddr, clock%stepsPerDay)

	idx := uint16(w.Globals[dayIndex]) * 2
	day := w.word(dayTable+idx) + 1
	w.setWord(dayTable+idx, day)
	w.ageParty()
	if day == 60 || day == 120 || day == daysPerYear {
		w.Globals[dayResetA], w.Globals[dayResetB] = 0, 0
	}
	if day > daysPerYear {
		w.setWord(dayTable+idx, 1)
		if y := w.word(yearTable + idx); y != maxYear {
			w.setWord(yearTable+idx, y+1)
		}
		w.Globals[yearResetAddr] = 0
	}
}

// ageParty 是每天一次的年齡推進（`sub_15092`）：每個隊員的「今年第幾天」
// 加一，滿 181 天就換一歲。
func (w *World) ageParty() {
	for i := range w.Party {
		c := &w.Party[i]
		c.AgeDays++
		if c.AgeDays >= daysPerYear+1 {
			c.Age = clampByte(c.Age + 1)
			c.AgeDays = 1
		}
	}
}

// NightFrom 是「入夜」的起點（今天走到第幾步）。
//
// **這條線是 remake 訂的，原版沒有。** 時鐘本身是 DOS 的（`ds:03CC`，
// 256 步一天），但 DOS 的繪圖路徑一處都沒讀它 —— 沒有日夜這回事。
// 會有這個功能是因為 Mega Drive 版有：它把整條調色盤按
// `分量 × 亮度 ÷ 8` 調暗，晝 8、夜 6。remake 借的是那個做法，
// 而「一天的後半算夜晚」是這裡挑的，不是量到的。
//
// 所以它預設**關閉**，在 `F2` 裡開，見 `docs/polish-spec.md`。
const NightFrom = stepsPerDay / 2

// Night 回報現在算不算夜晚（只看時鐘，不看設定）。
func (w *World) Night() bool { return w.Clock() >= NightFrom }
