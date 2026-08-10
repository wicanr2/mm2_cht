package ui

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 建立新角色的互動流程。
//
// 原版的鍵（`sub_18F48`）：`A`–`G` 挑一項屬性再挑另一項對調、`1`–`8` 選職業、
// Enter 重擲、Esc 離開。這裡照抄，只是方向鍵也能移游標。
//
// 選完職業接著問種族、陣營、性別，最後打名字 —— 順序與原版一致
// （`sub_18B1A` 依序印 `Race (1-5)`、`Alignment (1-3)`、`Sex (1-2)`、
// `Type Name of Character and Press 'Return' to Save`）。

// CreateLines 是建角畫面要顯示的內容。
//
// 屬性與職業都排兩欄 —— 選單框只有十一列，一行一項會擠掉操作提示，
// 而提示不見了玩家就不知道怎麼對調。
//
// 補位一定要用**顯示寬度**不是位元組數：中文一個字佔兩格、UTF-8 卻是
// 三個位元組，用 `%-14s` 補出來的欄位會參差不齊，長到超出框寬就折行，
// 兩欄版面整個垮掉。
func (s *Session) CreateLines() []string {
	n := &s.New
	elig := game.EligibleClasses(n.Attr)
	out := []string{"建立新角色"}

	attr := func(i int) string {
		if i >= game.NumAttrs {
			return ""
		}
		mark := " "
		switch {
		case i == s.attrPick:
			mark = "*"
		case i == s.attrCur:
			mark = "▶"
		}
		return fmt.Sprintf("%s%c %s %2d", mark, 'A'+i, game.AttrLabels[i], n.Attr[i])
	}
	rows := (game.NumAttrs + 1) / 2
	for i := 0; i < rows; i++ {
		out = append(out, padCols(attr(i), createCol)+attr(i+rows))
	}

	out = append(out, "")
	class := func(c int) string {
		name := "－"
		if elig[c] {
			name = game.Class(c).String()
		}
		return fmt.Sprintf("%d %s", c+1, name)
	}
	for i := 0; i < 4; i++ {
		out = append(out, padCols(class(i), createCol)+class(i+4))
	}
	if s.attrPick >= 0 {
		out = append(out, "移到另一項按 Enter 換")
	} else {
		out = append(out, "E 對調　Enter 重擲　Esc")
	}
	return out
}

// createCol 是建角畫面左欄的寬度（以 8 px 為一格）。
const createCol = 12

// padCols 把字串補到指定的顯示格數。中文一個字算兩格。
func padCols(text string, cols int) string {
	w := 0
	for _, r := range text {
		if r > 0x7F {
			w += 2
		} else {
			w++
		}
	}
	for ; w < cols; w++ {
		text += " "
	}
	return text
}

// createKey 處理建角畫面的按鍵。
func (s *Session) createKey(k Key) bool {
	switch k {
	case KeyCancel:
		s.Mode = ModeExplore
		return true
	case KeyUp:
		if s.attrCur > 0 {
			s.attrCur--
			return true
		}
		return false
	case KeyDown:
		if s.attrCur < game.NumAttrs-1 {
			s.attrCur++
			return true
		}
		return false
	case KeyConfirm:
		// 原版的 Enter 是重擲；先挑了一項要對調時，Enter 換成完成對調。
		if s.attrPick >= 0 {
			s.New.Exchange(s.attrPick, s.attrCur)
			s.attrPick = -1
			return true
		}
		s.New = game.RollNewCharacter(s.Game.Rand)
		return true
	case KeyUse:
		// 挑起這一項，等下一次確認時與游標所在的那一項對調。
		s.attrPick = s.attrCur
		return true
	}
	return false
}

// PressDigit 是按下數字鍵 1–9。
//
// 原版的選單是按數字直選（建角那頁的 1–8 就是職業），所以數字鍵要
// 走到底層而不是只當成游標。選單開著時等同「選第 n 項」。
func (s *Session) PressDigit(n int) bool {
	switch {
	case s.Mode == ModeCreate:
		return s.CreatePickClass(n)
	case s.Mode == ModeMenu && s.Menu != nil:
		if n < 1 || n > len(s.Menu.Items) {
			return false
		}
		s.Menu.Cur = n - 1
		return s.choose()
	}
	return false
}

// CreatePickClass 用職業編號（1–8）往下走。
func (s *Session) CreatePickClass(n int) bool {
	c := game.Class(n - 1)
	if n < 1 || n > 8 || !s.New.Eligible(c) {
		s.Lines = append(s.Lines, "這組屬性當不了那個職業。")
		return true
	}
	if game.RosterFull(s.Roster) {
		// 原版滿了會印 `*** Roster is Full ***` 而且根本不讓你按。
		s.Lines = append(s.Lines, "名冊已滿。")
		s.Mode = ModeMessage
		return true
	}
	s.New.SetClass(c)
	return s.open(menuCreateRace, listMenu("種族", names(game.RaceName, 5)))
}

// names 把 game 那邊的查名函式攤成清單。
func names(f func(int) string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = f(i)
	}
	return out
}

// listMenu 是簡單的字串清單選單。
func listMenu(title string, items []string) *Menu {
	return &Menu{Title: title, Items: append([]string{}, items...)}
}

// createChoose 接住種族／陣營／性別三個選單。
func (s *Session) createChoose(kind menuKind, i int) bool {
	switch kind {
	case menuCreateRace:
		if i < 0 || i >= 5 {
			return s.closeMenu()
		}
		s.New.SetRace(game.Race(i))
		return s.open(menuCreateAlign, listMenu("陣營", names(game.AlignName, 3)))
	case menuCreateAlign:
		if i < 0 || i >= 3 {
			return s.closeMenu()
		}
		s.New.SetAlign(game.Alignment(i))
		return s.open(menuCreateSex, listMenu("性別", names(game.SexName, 2)))
	case menuCreateSex:
		if i < 0 || i >= 2 {
			return s.closeMenu()
		}
		s.New.SetSex(i)
		s.closeMenu()
		s.Mode = ModeName
		s.New.Name = ""
		return true
	}
	return s.closeMenu()
}



// nameKey 處理輸入姓名那一頁的控制鍵；字元由 TypeRune 進來。
func (s *Session) nameKey(k Key) bool {
	switch k {
	case KeyCancel:
		s.Mode = ModeExplore
		return true
	case KeyConfirm:
		c, ok := s.New.Finish()
		if !ok {
			s.Lines = append(s.Lines, "還不能存檔：名字是空的。")
			return true
		}
		s.Roster = append(s.Roster, c)
		s.Lines = append(s.Lines,
			fmt.Sprintf("%s（%v %v）加入名冊，共 %d 人。",
				c.Name, c.Race, c.Class, len(s.Roster)))
		s.Mode = ModeMessage
		return true
	}
	return false
}

// maxNameLen 是姓名的長度上限。角色記錄的名字欄是 10 bytes
// （`offName`，空格填充，第 11 個位元組是 0）。
const maxNameLen = 10

// TypeRune 把一個字元打進姓名欄。退格傳入 '\b'。
func (s *Session) TypeRune(r rune) bool {
	if s.Mode != ModeName {
		return false
	}
	if r == '\b' {
		if n := []rune(s.New.Name); len(n) > 0 {
			s.New.Name = string(n[:len(n)-1])
			return true
		}
		return false
	}
	if r < 0x20 || len([]rune(s.New.Name)) >= maxNameLen {
		return false
	}
	s.New.Name += string(r)
	return true
}

// NameLines 是輸入姓名那一頁。
func (s *Session) NameLines() []string {
	return []string{
		"輸入角色姓名，然後按 Enter 存檔",
		"",
		"姓名：" + s.New.Name + "_",
		"",
		fmt.Sprintf("%v　%v　%v　%s",
			s.New.Class, s.New.Race, s.New.Align, game.SexName(clampSex(s.New.Sex))),
	}
}

func clampSex(i int) int {
	if i < 0 || i > 1 {
		return 0
	}
	return i
}
