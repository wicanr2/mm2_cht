package game

import (
	"fmt"
	"strings"
)

// 結局控制室：事件腳本 `0e fd` → 原版 `2SMITH` 的 `_2smith_e01`。
//
// 流程是「守門的一場仗 → 十格中止碼 → 替代加密的密碼題 → 結算」，
// 每一段的位址與證據見 `docs/re/05-2smith-control-room.md`。

// controlRoomCode 是 `0x0e` 的子命令。
const controlRoomCode = 0xFD

// controlRoomMonsters 是守在門口的五隻，原版寫進 `ds:9680`：
// `FF E1 C2 C1 E0` ＝ Sheltem 與水、風、火、土四隻元素生物。
var controlRoomMonsters = []int{255, 225, 194, 193, 224}

// controlRoomAbort 是中止碼。原版把玩家打的十個字補空白到十格，
// 再與 `ds:45FC` 的 `"WAFE      "` 做 `strcmp`。
const controlRoomAbort = "WAFE"

// ControlAbortWidth 是中止碼的輸入格數（原版 `loc_16EE6(&buf, 10)`）。
const ControlAbortWidth = 10

// controlRoomAnswer 是要編碼的字：`ds:45BA` 指向 `ds:45B0` 的 `"Preamble"`。
const controlRoomAnswer = "Preamble"

// 進門要隊伍裡有人帶著記錄 `+129` bit 5（Square Lake 的指令）。
const (
	controlChosenOffset = 129
	controlChosenBit    = 0x20
)

// 通關獎勵：`+129` bit 3 沒設就設起來，並加 `0x02FAF080` 到經驗值。
const (
	controlBonusBit = 0x08
	controlBonusExp = 0x02FAF080
)

// controlRoomMillis 是倒數的起始值（原版 `ds:0436:0438` ＝ 900,000），
// 依 controlClock 的算式顯示就是 `15:00:00`。
const controlRoomMillis = 900000

// controlRoomTick 是每前進一格扣掉的量（原版每輪輪詢 `sub 0x4B`）。
//
// **原版扣的是輪詢次數不是時間**：它每讀一次鍵盤就扣 75，中間只夾一個
// 延遲呼叫，而那支常式的單位還沒證實（見 `docs/re/05` §6）。
// remake 把它掛在火炬動畫的節拍上 —— 同樣是「跟著畫面走」，
// 而畫面上顯示的仍是原版那個 15 分鐘的鐘。
const controlRoomTick = 0x4B

// ControlPhase 是控制室的階段。
type ControlPhase byte

const (
	// ControlIdle 表示沒有在控制室裡。
	ControlIdle ControlPhase = iota
	// ControlAbortCode 在等十格中止碼。
	ControlAbortCode
	// ControlCryptogram 在等八個字元的密碼。
	ControlCryptogram
	// ControlWon 是解對了。
	ControlWon
	// ControlTimeout 是時間到（原版 `ds:0395 = 3`）。
	ControlTimeout
	// ControlRejected 是中止碼打錯，或隊伍裡沒有天選者。
	ControlRejected
)

// ControlRoom 是一次控制室的進行狀態。
//
// **不進存檔**：原版的控制室是一支跑到底的函式，中間沒有存檔點，
// 離開它只有三種結果（通關、時間到、被請出去）。
type ControlRoom struct {
	// Phase 是目前階段。
	Phase ControlPhase
	// Alphabet 是這一次的替代字母表：Alphabet[i] 是字母 i 換成的字母。
	Alphabet [26]byte
	// Expect 是要打進去的八個字元 ＝ Encode(controlRoomAnswer)。
	Expect string
	// Left 是倒數剩下的量，0 表示時間到。
	Left int
}

// NewControlRoom 抽一張字母表、算出答案、把鐘上緊。
//
// 抽法照 `sub_1D19C`：對每個位置重擲 `rand(1, 26) - 1` 直到抽到還沒用過的，
// 所以 Alphabet 是 0..25 的隨機排列，**每次進控制室重抽**。
func NewControlRoom(r *Rand) *ControlRoom {
	cr := &ControlRoom{Phase: ControlAbortCode, Left: controlRoomMillis}
	var used [26]bool
	for i := range cr.Alphabet {
		v := i
		if r != nil {
			for {
				v = r.Range(1, 26) - 1
				if v >= 0 && v < 26 && !used[v] {
					break
				}
			}
		}
		used[v] = true
		cr.Alphabet[i] = byte(v)
	}
	cr.Expect = cr.Encode(controlRoomAnswer)
	return cr
}

// Encode 把一段字照 `sub_1D1FC` 編碼：大小寫各自代換、其餘原樣。
//
// 原版查的是同一張表（`ds:580D[c]` 與 `ds:57ED[c]` 都落在 `ds:584E`），
// 差別只在還原時加 `'A'` 還是 `'a'` —— 所以**大小寫會被保留**。
func (cr *ControlRoom) Encode(s string) string {
	if cr == nil {
		return s
	}
	out := []byte(s)
	for i, b := range out {
		switch {
		case b >= 'A' && b <= 'Z':
			out[i] = 'A' + cr.Alphabet[b-'A']
		case b >= 'a' && b <= 'z':
			out[i] = 'a' + cr.Alphabet[b-'a']
		}
	}
	return string(out)
}

// Answer 回傳要編碼的那個字（畫面上原版就寫著它）。
func (cr *ControlRoom) Answer() string { return controlRoomAnswer }

// Clock 把剩下的量排成原版的鐘面。
//
// 算式照 `sub_1CF78`：÷60000 是分、餘 ÷1000 是秒、餘 ÷10 是百分秒，
// 三段都補零到兩位。
func (cr *ControlRoom) Clock() string {
	v := 0
	if cr != nil && cr.Left > 0 {
		v = cr.Left
	}
	m := v / 60000
	v -= m * 60000
	s := v / 1000
	v -= s * 1000
	return fmt.Sprintf("%02d:%02d:%02d", m, s, v/10)
}

// Tick 前進一格。時間到時回 true，呼叫端要收掉這一場。
func (cr *ControlRoom) Tick() bool {
	if cr == nil || cr.Phase != ControlCryptogram {
		return false
	}
	if cr.Left <= controlRoomTick {
		cr.Left = 0
		cr.Phase = ControlTimeout
		return true
	}
	cr.Left -= controlRoomTick
	return false
}

// SubmitAbort 收下中止碼。chosen 是隊伍裡有沒有人帶著天選者的旗標。
//
// 原版兩道關卡任一沒過都跳到同一個失敗分支，畫面只有一行 `Incorrect!`
// —— 分不出來是碼錯還是沒有旗標，remake 照樣不分。
func (cr *ControlRoom) SubmitAbort(input string, chosen bool) ControlPhase {
	if cr == nil {
		return ControlIdle
	}
	if cr.Phase != ControlAbortCode {
		return cr.Phase
	}
	if !abortCodeMatches(input) || !chosen {
		cr.Phase = ControlRejected
		return cr.Phase
	}
	cr.Phase = ControlCryptogram
	return cr.Phase
}

// abortCodeMatches 比對十格中止碼。
//
// 原版讀滿十格、不足補空白，再與 `"WAFE      "` 逐位元組比 —— 等價於
// 「去掉尾端空白之後正好是 WAFE」。**大小寫要一樣**，`strcmp` 不折大小寫。
func abortCodeMatches(input string) bool {
	if len(input) > ControlAbortWidth {
		return false
	}
	return strings.TrimRight(input, " ") == controlRoomAbort
}

// SubmitCode 收下密碼。打錯只是回到原地重打 —— 原版的比對函式回 0，
// 迴圈就再問一次，**沒有罰則**，只能一直試到時間用完。
func (cr *ControlRoom) SubmitCode(input string) ControlPhase {
	if cr == nil {
		return ControlIdle
	}
	if cr.Phase != ControlCryptogram {
		return cr.Phase
	}
	if cr.Left == 0 {
		cr.Phase = ControlTimeout
		return cr.Phase
	}
	if len([]byte(input)) == len(cr.Expect) && input == cr.Expect {
		cr.Phase = ControlWon
	}
	return cr.Phase
}

// ControlRoomEncounter 擺出守門的那一場。
func (s *Session) ControlRoomEncounter() *Encounter {
	return s.fixedEncounter(controlRoomMonsters)
}

// ControlRoomChosen 回報隊伍裡有沒有人帶著 `+129` bit 5。
func (s *Session) ControlRoomChosen() bool {
	for i := range s.Party {
		if s.Party[i].FieldByte(controlChosenOffset)&controlChosenBit != 0 {
			return true
		}
	}
	return false
}

// ControlRoomReward 發通關獎勵，回傳播報與最終分數。
//
// 每個人的 `+129` bit 3 沒設才設起來並加五千萬經驗（旗標擋重複領），
// 最終分數是**全隊經驗值總和**，含剛加上去的那一筆。
func (s *Session) ControlRoomReward() ([]string, uint32) {
	var score uint32
	given := 0
	for i := range s.Party {
		c := &s.Party[i]
		if c.FieldByte(controlChosenOffset)&controlBonusBit == 0 {
			c.SetFieldByte(controlChosenOffset, 0xFF, controlBonusBit)
			c.SetFieldValue(offExp, 4, uint32(c.Exp)+controlBonusExp)
			given++
		}
		score += uint32(c.Exp)
	}
	lines := []string{fmt.Sprintf(
		caveText("ui.control.bonus", "%d 名隊員各獲得 %d 點經驗。"), given, controlBonusExp)}
	return lines, score
}

// ControlRoomBattles 回傳戰鬥的勝場與敗場，填進原版那一行
// `Battles:   won /   lost.`（`ds:0410`／`ds:0412`，由戰鬥模組各自累加）。
func (s *Session) ControlRoomBattles() (won, lost int) {
	return s.BattlesWon, s.BattlesLost
}

// UseStrings 交進玩家那份 `STR.DAT` 解出來的行。
func (s *Session) UseStrings(lines []string) { s.Strings = lines }

// 六張表在 `STR.DAT` 裡的起點與條數，由 `sub_1D2A4` 的讀取順序算出來
// （`ds:52F4[3]` ＝ 第 280 行，之後依序 4／4／14／4／11／10 條）。
// 譯文的 key 就是各段的起點，見 `docs/re/05-2smith-control-room.md` §3.1。
const (
	strControlGuard  = 280 // ds:58B8 守門旁白
	strControlAbort  = 284 // ds:58C0 中止碼提問
	strControlBrief  = 288 // ds:5892 Sheltem 的預錄訊息
	strControlCipher = 302 // ds:5846 要被加密的四行
	strControlWin    = 306 // ds:5868 通關賀詞
	strControlScore  = 317 // ds:587E 戰績、分數與通訊地址
	cipherLines      = 4
)

// controlText 取某一段的譯文並拆成行。沒有譯文就回空的 —— 原文不放進
// 程式碼（版權材料不入版控），要英文的地方一律從玩家的 `STR.DAT` 讀。
func controlText(start int) []string {
	t := caveText(fmt.Sprintf("str.%03d", start), "")
	if t == "" {
		return nil
	}
	return strings.Split(t, "\n")
}

// origLines 取玩家那份 `STR.DAT` 的第 from 行起 n 行。
func (s *Session) origLines(from, n int) []string {
	if from < 0 || from+n > len(s.Strings) {
		return nil
	}
	return append([]string(nil), s.Strings[from:from+n]...)
}

// ControlGuardLines 是守門那一段的旁白（原版在第 19–22 列印 `ds:58B8`）。
func (s *Session) ControlGuardLines() []string { return controlText(strControlGuard) }

// ControlAbortLines 是中止碼的提問畫面（`ds:58C0`）。
func (s *Session) ControlAbortLines() []string { return controlText(strControlAbort) }

// ControlBriefLines 是 Sheltem 的預錄訊息（`ds:5892`）。
func (s *Session) ControlBriefLines() []string { return controlText(strControlBrief) }

// ControlCipherLines 排出密碼題那一頁。
//
// **中英並陳是 remake 的裁決，不是原版行為。** 原版只印密文，玩家要自己
// 認出那是憲法序言的改寫、推出字母對應、再把 `Preamble` 編碼打進去；
// 翻成中文之後那條線索斷了（中文沒有字母可以代換），與 `KEYS`／`DRUIDS`
// 那幾題同一類。這裡把三份一起攤開 —— 密文、英文明文、中文明文 ——
// 密文與英文明文對照就看得出字母怎麼換，中文負責讓劇情讀得懂。
func (s *Session) ControlCipherLines(cr *ControlRoom) []string {
	plain := s.origLines(strControlCipher, cipherLines)
	var out []string
	if len(plain) > 0 {
		out = append(out, caveText("ui.control.cipher", "密文"))
		for _, l := range plain {
			out = append(out, "  "+cr.Encode(l))
		}
		out = append(out, caveText("ui.control.plain", "原文"))
		for _, l := range plain {
			out = append(out, "  "+l)
		}
	}
	if zh := controlText(strControlCipher); len(zh) > 0 {
		out = append(out, caveText("ui.control.zh", "譯文"))
		for _, l := range zh {
			out = append(out, "  "+l)
		}
	}
	return out
}

// ControlCodePrompt 是輸入格左邊那一行。
//
// 原版是 `ds:4681` 的 `Answer= Preamble     Code=` —— 它本來就把「要編碼
// 的字」印在畫面上，玩家缺的是編碼後那八個字元。**這裡把那八個字元也
// 附上去**，玩家照打即可（與事件謎題的 `World.TextExpect` 同一套做法）。
func (s *Session) ControlCodePrompt(cr *ControlRoom) string {
	head := caveText("exe.4681", "")
	if head == "" {
		head = "Answer= " + controlRoomAnswer + "  Code="
	}
	if cr == nil {
		return head
	}
	return fmt.Sprintf("%s %s", head, cr.Expect)
}

// ControlWinLines 是通關賀詞（`ds:5868`）。
func (s *Session) ControlWinLines() []string { return controlText(strControlWin) }

// ControlScoreLines 是結算那一頁：原版的十行加上填進去的三個數字。
//
// 原版把勝場填在第 13 列第 13 欄、敗場第 25 欄、最終分數第 14 列第 21 欄
// —— 也就是填進 `Battles:   won /   lost.` 與 `Your final score is` 那兩行的
// 空白處。remake 的版面重排過，改成各自獨立成行。
func (s *Session) ControlScoreLines(score uint32) []string {
	out := controlText(strControlScore)
	won, lost := s.ControlRoomBattles()
	battles := fmt.Sprintf(caveText("ui.control.battles", "戰鬥：%d 勝 / %d 敗"), won, lost)
	if len(out) == 0 {
		return []string{battles, fmt.Sprintf(caveText("ui.control.score", "最終分數：%d"), score)}
	}
	// 第一行是勝敗那一行的樣板（原版把數字填進它的空白），第二行是
	// 「你的最終分數是」。照原版把數字填進去，不要另外加兩行重複的。
	out[0] = battles
	if len(out) > 1 {
		out[1] = fmt.Sprintf("%s %d", strings.TrimRight(out[1], " "), score)
	}
	return out
}
