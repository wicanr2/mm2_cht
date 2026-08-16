package view

import "github.com/wicanr2/mm2_cht/internal/render"

// 結局控制室那一頁。
//
// 原版是 25 列的純文字畫面，一次攤開 Sheltem 的訊息、密文與輸入框
// （版面座標見 `docs/re/05-2smith-control-room.md` §4）。探索畫面下方
// 那個訊息框只有三列，塞不下，所以這一段獨立成整頁 —— 這與世界地圖、
// 片頭一樣，是 remake 的版面決定，不是 pixel-perfect 還原。

const (
	controlTop  = 10
	controlRowH = 9
	controlX    = 12
	// controlBottom 是這一頁畫得到的最下緣。
	controlBottom = 198
	// controlRows 是總列數，最後兩列固定留給輸入列與提示。
	controlRows    = (controlBottom - controlTop) / controlRowH
	controlContent = controlRows - 2
)

// ControlPage 是控制室要畫的一頁。
type ControlPage struct {
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
func (p ControlPage) head() string {
	switch {
	case p.Title != "" && p.Clock != "":
		return p.Title + "　" + p.Clock
	case p.Clock != "":
		return p.Clock
	}
	return p.Title
}

// DrawControlRoom 畫控制室的一頁。
//
// 內容超過放得下的列數時**會留一行標記**，不是安靜地截掉 ——
// 少掉的幾行與「這一段本來就沒有」在畫面上長得一模一樣。
func DrawControlRoom(s *render.Screen, a Assets, p ControlPage) {
	s.Clear(0)
	s.Flush()

	st := a.white()
	rows := make([]string, 0, controlContent+1)
	if h := p.head(); h != "" {
		rows = append(rows, h)
	}
	rows = append(rows, p.Lines...)
	if len(rows) > controlContent {
		rows = rows[:controlContent]
		rows[controlContent-1] = "……（放不下，還有 " +
			itoa(len(p.Lines)+1-controlContent+1) + " 行）"
	}
	for i, l := range rows {
		s.DrawText(st, l, controlX*render.Scale, (controlTop+i*controlRowH)*render.Scale)
	}
	if p.Prompt != "" {
		s.DrawText(st, p.Prompt+" "+p.Input+"_", controlX*render.Scale,
			(controlTop+(controlRows-2)*controlRowH)*render.Scale)
	}
	if p.Hint != "" {
		s.DrawText(st, p.Hint, controlX*render.Scale,
			(controlTop+(controlRows-1)*controlRowH)*render.Scale)
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
