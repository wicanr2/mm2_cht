package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 特殊裝置的畫面：事件腳本 `0x0e` 分派給 `2CAVES` 的那幾支。
// 機制與證據見 `docs/re/02-2caves-special-events.md`。

// openDevice 依裝置開對應的畫面。回傳 false 表示這個裝置還沒有畫面。
func (s *Session) openDevice(d game.CaveDevice) bool {
	switch d {
	case game.DeviceTeleport:
		s.PromptText = ""
		s.textFor = textLocation
		s.Notice = "魔法座標：輸入 X Y（各 0–15）"
		s.Mode = ModeText
		return true
	case game.DeviceGoldExp:
		return s.open(menuGoldExp, s.donateMenu("誰要拿黃金換經驗？", func(c *game.Character) string {
			return fmt.Sprintf("%d 金", c.Gold)
		}))
	case game.DeviceGemExp:
		return s.open(menuGemExp, s.donateMenu("誰要捐出寶石？（一顆十點經驗）", func(c *game.Character) string {
			return fmt.Sprintf("%d 顆", c.Gems)
		}))
	case game.DeviceEraGate:
		if !s.Game.EraGateOpen() {
			s.Lines = append(s.Lines, "門一動也不動。隊伍裡沒有人帶著開門的東西。")
			s.Mode = ModeMessage
			return true
		}
		return s.open(menuEraGate, eraMenu())
	}
	return false
}

// donateMenu 是「選一名隊員」的清單，右邊附上他身上的數量。
func (s *Session) donateMenu(title string, amount func(*game.Character) string) *Menu {
	m := &Menu{Title: title}
	s.pickers = s.pickers[:0]
	for i := range s.Game.Party {
		c := &s.Game.Party[i]
		s.pickers = append(s.pickers, i)
		m.Items = append(m.Items, fmt.Sprintf("%s　%s", c.Name, amount(c)))
	}
	m.Items = append(m.Items, "離開")
	return m
}

// eraMenu 列出八個年代。選項 5–8 才改世紀，這裡標出來 ——
// 原版畫面只有 `What era do you desire (1-8)?` 一行，看不出差別。
func eraMenu() *Menu {
	m := &Menu{Title: "你想去哪一個年代？"}
	for i, o := range game.EraOptions() {
		mark := ""
		if o.Century != 0 {
			mark = fmt.Sprintf("　第 %d 世紀", o.Century)
		}
		m.Items = append(m.Items, fmt.Sprintf("%d%s", i+1, mark))
	}
	m.Items = append(m.Items, "離開")
	return m
}

// deviceChoice 處理三個裝置選單的選擇。
func (s *Session) deviceChoice(kind menuKind, i int) bool {
	switch kind {
	case menuGoldExp, menuGemExp:
		if i >= 0 && i < len(s.pickers) {
			member := s.pickers[i] + 1
			if kind == menuGoldExp {
				s.Lines = append(s.Lines, s.Game.TradeGoldForExp(member)...)
			} else {
				s.Lines = append(s.Lines, s.Game.DonateGemsForExp(member)...)
			}
		}
	case menuEraGate:
		if i >= 0 && i < len(game.EraOptions()) {
			s.Lines = append(s.Lines, s.Game.EnterEra(i+1)...)
		}
	}
	s.closeMenu()
	if len(s.Lines) > 0 {
		s.Mode = ModeMessage
	} else {
		s.Mode = ModeExplore
	}
	return true
}

// applyLocation 收下座標傳送機的輸入。格式是兩個 0–15 的數字，
// 中間空白或逗號都可以 —— 原版分兩次問，這裡合成一行。
func (s *Session) applyLocation() bool {
	f := strings.FieldsFunc(s.PromptText, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t'
	})
	if len(f) != 2 {
		return false
	}
	x, err1 := strconv.Atoi(f[0])
	y, err2 := strconv.Atoi(f[1])
	if err1 != nil || err2 != nil {
		return false
	}
	if !s.Game.MagicLocation(x, y) {
		return false
	}
	s.PromptText = ""
	s.textFor = textEvent
	s.Notice = ""
	s.Lines = append(s.Lines, fmt.Sprintf("隊伍出現在 (%d, %d)。", x, y))
	s.Mode = ModeMessage
	return true
}
