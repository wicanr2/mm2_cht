package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
}

type row struct {
	Cols []string `json:"cols"`
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
	}
}

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
	return m
}

// refSection 是第二層：某一類的內容。
func (s *Session) refSection(i int) *Menu {
	secs := s.Ref.sections()
	if i < 0 || i >= len(secs) {
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
