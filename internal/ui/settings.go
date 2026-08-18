package ui

// 設定畫面（`F2`）。
//
// remake 有幾個地方**刻意與原版不同**（理由記在 `docs/polish-spec.md` §4）。
// 那些差異不是缺陷，但也不該由我們替玩家決定死 —— 想照原版玩的人
// 在這裡切回去，想順順玩的人維持預設。
//
// **這是 remake 自己的畫面，原版沒有。** 原版的 `O` 指令視窗放的是
// 存檔與離開，那兩件事在 remake 是 `F4` 與 `F10`。

// settingsMenu 組出設定選單。每一項都印出目前的值 ——
// 開起來看不到現在是哪一邊的設定畫面等於沒有設定畫面。
func (s *Session) settingsMenu() *Menu {
	claim := "自動入袋（remake 預設）"
	if !s.Game.AutoClaimReward {
		claim = "按 S 領取（照原版）"
	}
	night := "關（照原版）"
	if s.Game.NightDimming {
		night = "開（借 Mega Drive 的做法）"
	}
	flavor := "關（照原版）"
	if s.Game.MDFlavor {
		flavor = "開（Mega Drive 版的稿）"
	}
	return listMenu("設定", []string{
		"事件獎賞：" + claim,
		"入夜視野變暗：" + night,
		"設施場景描述：" + flavor,
		"存檔",
		"離開",
	})
}

// settingsChoose 接住設定選單。
func (s *Session) settingsChoose(i int) bool {
	switch i {
	case 0:
		s.Game.AutoClaimReward = !s.Game.AutoClaimReward
		// 切回「照原版」時，已經自動入袋的那些拿不回來，也不必拿回來 ——
		// 這個開關只影響**之後**踩到的事件。
		return s.open(menuSettings, s.settingsMenu())
	case 1:
		// DOS 沒有日夜 —— 時鐘是它的（256 步一天），變暗是借 Mega Drive 的。
		s.Game.NightDimming = !s.Game.NightDimming
		return s.open(menuSettings, s.settingsMenu())
	case 2:
		// 進設施時先講一段場景。**那是 Mega Drive 版的稿**，DOS 沒有。
		s.Game.MDFlavor = !s.Game.MDFlavor
		return s.open(menuSettings, s.settingsMenu())
	case 3:
		line := s.Save()
		s.closeMenu()
		s.Lines = append(s.Lines, line)
		s.Mode = ModeMessage
		return true
	}
	return s.closeMenu()
}

// quickFight 一路打到分出結果（`F` 快速戰鬥）。
//
// 每一輪走的是與按 Enter 完全相同的 `fightRound`，所以戰利品、經驗、
// 音效與全滅處理都是同一條路 —— **不是另寫一套結算**。
//
// 上限 `quickFightRounds` 是給「兩邊都打不死對方」準備的：原版沒有這個
// 情形（總有一邊會倒），但 remake 的目標選擇還沒接完（見 `docs/todo.md`
// G1），打不到後排時可能真的僵住。到上限就停下來讓玩家自己決定，
// 而不是把畫面掛住。
const quickFightRounds = 60

func (s *Session) quickFight() bool {
	if s.Game.Fight == nil {
		s.Mode = ModeExplore
		return true
	}
	n := 0
	for i := 0; i < quickFightRounds; i++ {
		if s.Game.Fight == nil || s.Mode != ModeCombat {
			break
		}
		if !s.fightRound() {
			break
		}
		n++
		if s.Game.Fight == nil || s.Game.Fight.Over() {
			break
		}
	}
	if n == 0 {
		return false
	}
	if s.Mode == ModeCombat && s.Game.Fight != nil && !s.Game.Fight.Over() {
		s.Lines = append(s.Lines,
			"打了很久還沒分出結果，先停下來。")
	}
	return true
}
