package ui

import (
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/view"
)

// 結局控制室的畫面（`0e fd` → 原版 2SMITH 的 `_2smith_e01`）。
// 每一段的機制與位址見 `docs/re/05-2smith-control-room.md`。

// controlStage 是控制室走到哪一頁。
type controlStage int

const (
	// stageGuard 是守門那段旁白，打完 Sheltem 才會看到。
	stageGuard controlStage = iota
	// stageAbort 在等十格中止碼。
	stageAbort
	// stageBrief 是 Sheltem 的預錄訊息。
	stageBrief
	// stageCipher 是密碼題。
	stageCipher
	// stageWin／stageScore 是通關的兩頁。
	stageWin
	stageScore
	// stageOut 是被請出去或時間到。
	stageOut
)

// controlInputMax 是密碼欄的長度上限（原版 `sub_1D11A` 的 `cmp si, 8`）。
const controlInputMax = 8

// beginControlRoom 踩到控制室那一格：先打守門的那一場。
//
// 原版是 `_2smith_e01` 把五隻寫進 `ds:9680`、設 `ds:0415 = 0x83` 之後
// 呼叫 `sub_1720A` 開打，打完才看 `ds:0509` 決定進不進控制室。
// remake 的戰鬥是逐回合推進的，所以拆成「開打」與「打完之後」兩半，
// 與競技賽（`arenaTier`）同一個做法。
func (s *Session) beginControlRoom() bool {
	enc := s.Game.ControlRoomEncounter()
	if enc == nil {
		// 沒有怪物表時直接進控制室，不要讓整格變成什麼都沒發生。
		return s.openControlRoom()
	}
	s.Game.Fight = enc
	s.controlGuard = true
	s.Mode = ModeCombat
	s.take(s.Game.Log)
	return true
}

// openControlRoom 打贏之後進控制室。
func (s *Session) openControlRoom() bool {
	s.control = game.NewControlRoom(s.Game.Rand)
	s.controlStage = stageGuard
	s.controlInput = ""
	s.Mode = ModeControl
	return true
}

// closeControlRoom 收掉這一場。
func (s *Session) closeControlRoom() bool {
	s.control = nil
	s.controlInput = ""
	s.Mode = ModeExplore
	return true
}

// controlPage 排出目前這一頁。
func (s *Session) controlPage() view.TextPage {
	cr := s.control
	anyKey := "按 Enter 繼續"
	switch s.controlStage {
	case stageGuard:
		return view.TextPage{
			Title: "控制室入口",
			Lines: s.Game.ControlGuardLines(),
			Hint:  anyKey,
		}
	case stageAbort:
		return view.TextPage{
			Title:  "控制室",
			Lines:  s.Game.ControlAbortLines(),
			Prompt: "解除碼＝",
			Input:  s.controlInput,
			Hint:   "打完按 Enter",
		}
	case stageBrief:
		return view.TextPage{
			Title: "預錄訊息",
			Lines: s.Game.ControlBriefLines(),
			Hint:  anyKey,
		}
	case stageCipher:
		return view.TextPage{
			Title:  "密碼題",
			Lines:  s.Game.ControlCipherLines(cr),
			Prompt: s.Game.ControlCodePrompt(cr),
			Input:  s.controlInput,
			Clock:  cr.Clock(),
			Hint:   "照上面那組密碼打進去，按 Enter",
		}
	case stageWin:
		return view.TextPage{
			Title: "科隆得救了",
			Lines: s.Game.ControlWinLines(),
			Hint:  anyKey,
		}
	case stageScore:
		return view.TextPage{
			Title: "結算",
			Lines: s.Game.ControlScoreLines(s.controlScore),
			Hint:  anyKey,
		}
	}
	return view.TextPage{Title: "控制室", Lines: s.controlOutLines, Hint: anyKey}
}

// controlKey 處理控制室的按鍵。字元由 TypeRune 進來。
func (s *Session) controlKey(k Key) bool {
	cr := s.control
	if cr == nil {
		return s.closeControlRoom()
	}
	switch s.controlStage {
	case stageGuard:
		s.controlStage = stageAbort
		return true
	case stageAbort:
		if k != KeyConfirm {
			return false
		}
		switch cr.SubmitAbort(s.controlInput, s.Game.ControlRoomChosen()) {
		case game.ControlCryptogram:
			s.controlInput = ""
			s.controlStage = stageBrief
		default:
			// 原版兩道關卡沒過都只印一行 `Incorrect!`，然後把隊伍請出去。
			s.controlOutLines = []string{s.text("exe.469C", "Incorrect!")}
			s.controlStage = stageOut
		}
		return true
	case stageBrief:
		s.controlStage = stageCipher
		return true
	case stageCipher:
		if k != KeyConfirm {
			return false
		}
		switch cr.SubmitCode(s.controlInput) {
		case game.ControlWon:
			lines, score := s.Game.ControlRoomReward()
			s.controlScore = score
			s.controlReward = lines
			s.controlStage = stageWin
		case game.ControlTimeout:
			s.controlOutLines = []string{"時間到了。科隆撞上太陽。"}
			s.controlStage = stageOut
		default:
			// 打錯不罰，回到原地重打 —— 原版的比對回 0 就再問一次。
			s.controlInput = ""
		}
		return true
	case stageWin:
		s.controlStage = stageScore
		return true
	case stageScore:
		s.Lines = append(s.Lines, s.controlReward...)
		s.controlReward = nil
		s.closeControlRoom()
		if len(s.Lines) > 0 {
			s.Mode = ModeMessage
		}
		return true
	}
	// stageOut：把結果推回訊息列再離開。
	s.Lines = append(s.Lines, s.controlOutLines...)
	s.controlOutLines = nil
	s.closeControlRoom()
	if len(s.Lines) > 0 {
		s.Mode = ModeMessage
	}
	return true
}

// controlTick 讓密碼題的鐘走一格。時間到就當場結束這一場。
func (s *Session) controlTick() {
	if s.Mode != ModeControl || s.control == nil {
		return
	}
	if s.control.Tick() {
		s.controlOutLines = []string{"時間到了。科隆撞上太陽。"}
		s.controlStage = stageOut
	}
}

// typeControlRune 把一個字元打進控制室的輸入欄。退格傳 '\b'。
//
// 兩個欄位的長度上限不同，照原版：中止碼十格（`loc_16EE6(&buf, 10)`）、
// 密碼八格（`sub_1D11A` 的 `cmp si, 8`）。可打的字元也照原版限在
// `0x20`–`0x7E`，中文打不進去 —— 兩個答案都是拉丁字母。
func (s *Session) typeControlRune(r rune) bool {
	max := controlInputMax
	switch s.controlStage {
	case stageAbort:
		max = game.ControlAbortWidth
	case stageCipher:
	default:
		return false
	}
	if r == '\b' {
		if n := []rune(s.controlInput); len(n) > 0 {
			s.controlInput = string(n[:len(n)-1])
			return true
		}
		return false
	}
	if r < 0x20 || r > 0x7E || len([]rune(s.controlInput)) >= max {
		return false
	}
	s.controlInput += string(r)
	return true
}

// text 取譯文，沒有譯文就用 fallback。
func (s *Session) text(key, fallback string) string {
	if s.cat == nil {
		return fallback
	}
	return s.cat.Or(key, fallback)
}

// deadPage 是全滅那一頁。
//
// 十行的內容與位置照原版 `_1retinn_e04`（`ds:22A6` 的十個指標，第 1–10 列），
// 譯文已經在 `exe.*` 裡。機制見 `docs/re/06-1retinn-roster.md` §6。
func (s *Session) deadPage() view.TextPage {
	keys := []string{"exe.21FA", "exe.220E", "exe.220F", "exe.2227", "exe.223E",
		"exe.2252", "exe.2253", "exe.226A", "exe.2280", "exe.2294"}
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, s.text(k, ""))
	}
	return view.TextPage{Lines: lines, Hint: "按 Enter 回到最後投宿的旅店"}
}
