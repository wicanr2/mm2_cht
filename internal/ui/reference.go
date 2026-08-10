package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// Reference 是紙本說明書裡那些「遊戲內查不到」的參考資料。
//
// 1988 年靠磁片出貨，第二技能表、指令一覽這類東西只印在紙本上。
// 這個 remake 把它們收進遊戲 —— 資料由 `tools/extract_manual.py`
// 從說明書轉錄抽出來（`data/reference.json`）。
type Reference struct {
	Source        string  `json:"source"`
	Skills        []row   `json:"skills"`
	TownCommands  []row   `json:"townCommands"`
	FieldCommands []row   `json:"fieldCommands"`
	SkillShops    []row   `json:"skillShops"`

	// Lore 是說明書的敘事段落（序言與科隆的歷史）。
	//
	// 那兩章只印在紙本上，遊戲裡一個字都沒有 —— 而故事的來龍去脈
	// （柯拉克為什麼失蹤、四大自然種族、卡隆國王與火龍）全在那裡。
	// 使用者的要求是「把完整的遊戲訊息都放在遊戲內」，這是其中一塊。
	Lore []loreSection `json:"lore"`
}

type row struct {
	Cols []string `json:"cols"`
}

// loreSection 是一章敘事，Items 依序是小標與段落。
type loreSection struct {
	Title string     `json:"title"`
	Items []loreItem `json:"items"`
}

type loreItem struct {
	Heading string `json:"heading,omitempty"`
	Text    string `json:"text,omitempty"`
}

// Line 是這一項要顯示的字。小標前面加符號，讀起來分得出層次。
func (l loreItem) Line() string {
	if l.Heading != "" {
		return "── " + l.Heading
	}
	return l.Text
}

// LoadReference 讀參考資料。讀不到就回 nil —— 遊戲照樣能玩，
// 只是查不到說明書的內容。
func LoadReference(dir string) *Reference {
	b, err := os.ReadFile(filepath.Join(dir, "reference.json"))
	if err != nil {
		return nil
	}
	var r Reference
	if json.Unmarshal(b, &r) != nil {
		return nil
	}
	return &r
}

// sections 是可以查的幾類。
func (r *Reference) sections() []struct {
	name string
	rows []row
} {
	return []struct {
		name string
		rows []row
	}{
		{"第二技能", r.Skills},
		{"城鎮指令", r.TownCommands},
		{"冒險畫面指令", r.FieldCommands},
		{"技能在哪學", r.SkillShops},
		{"職業", classRows()},
	}
}

// loreMenus 把敘事章節也排進「查說明書」的第一層。
func (r *Reference) loreMenus() []loreSection { return r.Lore }

// refMenu 是「查說明書」的第一層：挑一類。
func (s *Session) refMenu() *Menu {
	m := &Menu{Title: "查說明書"}
	if s.Ref == nil {
		m.Items = append(m.Items, "（沒有 data/reference.json）")
		return m
	}
	for _, sec := range s.Ref.sections() {
		m.Items = append(m.Items, fmt.Sprintf("%s（%d 條）", sec.name, len(sec.rows)))
	}
	for _, l := range s.Ref.loreMenus() {
		m.Items = append(m.Items, fmt.Sprintf("%s（%d 段）", l.Title, len(l.Items)))
	}
	return m
}

// refSection 是第二層：某一類的內容。
func (s *Session) refSection(i int) *Menu {
	secs := s.Ref.sections()
	// 敘事章節排在表格類後面。
	if i >= len(secs) {
		lore := s.Ref.loreMenus()
		j := i - len(secs)
		if j < 0 || j >= len(lore) {
			return &Menu{Title: "查說明書"}
		}
		m := &Menu{Title: lore[j].Title}
		s.refRows = s.refRows[:0]
		for _, it := range lore[j].Items {
			s.refRows = append(s.refRows, []string{it.Line()})
			m.Items = append(m.Items, it.Line())
		}
		return m
	}
	if i < 0 {
		return &Menu{Title: "查說明書"}
	}
	sec := secs[i]
	m := &Menu{Title: sec.name}
	s.refRows = s.refRows[:0]
	for _, r := range sec.rows {
		cols := trimIndex(r.Cols)
		s.refRows = append(s.refRows, cols)
		// 清單只放名稱，其餘（效果、出處）放訊息區 —— 一行塞不下
		// 「原文 + 中文 + 效果」，硬擠會被截掉，反而是效果先不見。
		m.Items = append(m.Items, nameOf(cols))
	}
	return m
}

// refDetail 是游標那一列的完整內容，顯示在訊息區。
func (s *Session) refDetail(i int) string {
	if i < 0 || i >= len(s.refRows) {
		return ""
	}
	cols := s.refRows[i]
	if len(cols) < 2 {
		return strings.Join(cols, "")
	}
	// 名稱那兩欄接在一起，其餘用換行符分開（'@' 是原版的換行）。
	head := nameOf(cols)
	rest := cols[2:]
	if len(cols) == 2 {
		return head
	}
	return head + "@" + strings.Join(rest, "　")
}

// nameOf 取一列的名稱：有中文就「原文 中文」，否則只有原文。
func nameOf(cols []string) string {
	switch len(cols) {
	case 0:
		return ""
	case 1:
		return cols[0]
	}
	return cols[0] + "　" + cols[1]
}

// trimIndex 丟掉說明書表格自己的編號欄 —— 選單已經編過號，
// 兩個號並排會變成「1) 1 Arms Master」。
func trimIndex(cols []string) []string {
	if len(cols) > 1 && isNumber(cols[0]) {
		return cols[1:]
	}
	return cols
}

// isNumber 判斷一欄是不是純數字（說明書表格的編號欄）。
func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// classRows 是職業的成長數值。
//
// **不從手冊來**：手冊那份（part-4 §3.3）只有地圖上的專屬區標示，
// 不是完整職業表。引擎自己的表反而齊全 —— 每級生命點數來自手冊的
// 職業表（`data/classes.json`），命中與揮擊除數是從原版反組譯來的
// （`ds:1012`／`ds:101A`）。
func classRows() []row {
	if !game.Loaded() {
		return nil
	}
	out := make([]row, 0, 8)
	for i := 0; i < 8; i++ {
		out = append(out, row{Cols: []string{
			game.ClassName(i),
			fmt.Sprintf("命中除數 %d", game.AttackDivisorFor(i)),
			fmt.Sprintf("揮擊除數 %d", game.SwingDivisorFor(i)),
			fmt.Sprintf("會不會施法：%s", yesNo(game.CanCast(game.Class(i)))),
		}})
	}
	return out
}

func yesNo(b bool) string {
	if b {
		return "會"
	}
	return "不會"
}
