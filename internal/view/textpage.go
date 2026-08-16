package view

import "github.com/wicanr2/mm2_cht/internal/render"

// 整頁的純文字畫面：標題、內容、輸入列、提示。
//
// 原版好幾個畫面是 25 列的整頁文字（結局控制室、全滅、名冊編組），
// 而探索畫面下方那個訊息框只有三列，塞不下。這一頁與世界地圖、片頭
// 一樣，是 remake 的版面決定，不是 pixel-perfect 還原。

const (
	pageTop  = 10
	pageRowH = 9
	pageX    = 12
	// pageBottom 是這一頁畫得到的最下緣。
	pageBottom = 198
	// pageRows 是總列數，最後兩列固定留給輸入列與提示。
	pageRows    = (pageBottom - pageTop) / pageRowH
	pageContent = pageRows - 2
)

// TextPage 是要畫的一頁。
type TextPage struct {
	// Title 是最上面那一行。
	Title string
	// Lines 是內容。
	Lines []string
	// Prompt 與 Input 是輸入列；Prompt 空的時候不畫輸入列。
	Prompt string
	Input  string
	// Clock 是倒數的鐘面，空的表示這一頁沒有計時。
	Clock string
	// Hint 是最下面那一行的操作提示。
	Hint string
}

// head 是標題那一行；有鐘就接在標題後面，不另外佔一列。
func (p TextPage) head() string {
	switch {
	case p.Title != "" && p.Clock != "":
		return p.Title + "　" + p.Clock
	case p.Clock != "":
		return p.Clock
	}
	return p.Title
}

// DrawTextPage 畫一頁純文字。
//
// 內容超過放得下的列數時**會留一行標記**，不是安靜地截掉 ——
// 少掉的幾行與「這一段本來就沒有」在畫面上長得一模一樣。
func DrawTextPage(s *render.Screen, a Assets, p TextPage) {
	s.Clear(0)
	s.Flush()

	st := a.white()
	rows := make([]string, 0, pageContent+1)
	if h := p.head(); h != "" {
		rows = append(rows, h)
	}
	rows = append(rows, p.Lines...)
	if len(rows) > pageContent {
		rows = rows[:pageContent]
		rows[pageContent-1] = "……（放不下，還有 " +
			itoa(len(p.Lines)+1-pageContent+1) + " 行）"
	}
	for i, l := range rows {
		s.DrawText(st, l, pageX*render.Scale, (pageTop+i*pageRowH)*render.Scale)
	}
	if p.Prompt != "" {
		s.DrawText(st, p.Prompt+" "+p.Input+"_", pageX*render.Scale,
			(pageTop+(pageRows-2)*pageRowH)*render.Scale)
	}
	if p.Hint != "" {
		s.DrawText(st, p.Hint, pageX*render.Scale,
			(pageTop+(pageRows-1)*pageRowH)*render.Scale)
	}
}

// itoa 是給上面那一行標記用的小工具，避免為了一個數字拉進 strconv。
func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
