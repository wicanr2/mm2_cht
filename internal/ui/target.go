package ui

// 攻擊前挑目標（原版 `2COMBAT` 的 `sub_18DAA` 開頭那一段）。
//
// 原版的流程：
//
//	可選隻數 = 射擊 ? ds:0508（場上全部）: ds:9FC5（前排），夾在 10
//	只有一隻（或零隻）→ 不問，直接打第 0 隻
//	提示字串是「Fight／Shoot」＋「 which (A - J)?」，最後那個字母
//	由可選隻數算出來（`var_C + 0x40` 寫進字串的第 12 個位元組）
//	讀鍵直到讀到 A..該字母 或 Esc；其他鍵一律忽略，不播報
//	Esc → 這次攻擊取消，**行動不消耗**（`var_E--` 之後跳過整段攻擊）
//
// remake 照這條規則，兩處刻意不同：
//
//   - **選單用數字不用字母。** 這個 remake 的選單一律是數字（隊伍名單、
//     設定、商店都是），為了一個畫面改成字母反而不一致。可選隻數的
//     上限 10 照留 —— 那是規則不是版面，原版夾在 10 之後打不到第 11 隻。
//   - **只列站著的。** 原版怪物一倒下就整批往前搬（`sub_18A22`），
//     陣列裡不存在倒下的怪；remake 是留在陣列裡等結算，所以這裡要自己濾。
//
// 不問的時機也照原版：可選目標剩一隻就直接打，不要為了「一致」多問一次。

// targetMenu 排出可以打的目標。回傳 nil 表示不必問（沒得選或只有一隻）。
//
// `s.pickers` 存的是 `Monsters` 的索引 —— 選單上的第 n 項與陣列的第 n 隻
// **不是同一個號碼**，中間隔著「倒下的不列」。
func (s *Session) targetMenu(ranged bool) *Menu {
	enc := s.Game.Fight
	if enc == nil {
		return nil
	}
	foes := enc.Reachable(ranged)
	s.pickers = s.pickers[:0]
	var items []string
	for i, m := range foes {
		if !m.CombatCondition().Acts() {
			continue
		}
		// 編號由選單框自己加（`1)`／`2)`…），這裡只放名字 ——
		// 兩邊都編一次會變成「1) 1. 狗頭人」。
		s.pickers = append(s.pickers, i)
		items = append(items, m.CombatName())
	}
	if len(items) < 2 {
		return nil
	}
	verb := s.text("exe.1206", "Fight")
	if ranged {
		verb = s.text("exe.120C", "Shoot")
	}
	return listMenu(verb+"　打哪一隻？", items)
}

// attackCommand 是攻擊與射擊的共同入口：先問目標，再打一回合。
//
// 只有一個目標時直接打 —— 原版的 `var_C <= 1` 那一行就是這件事。
func (s *Session) attackCommand(ranged bool) bool {
	enc := s.Game.Fight
	if enc == nil {
		s.Mode = ModeExplore
		return true
	}
	if m := s.targetMenu(ranged); m != nil {
		s.targetRanged = ranged
		return s.open(menuTarget, m)
	}
	enc.Target = 0
	if ranged {
		return s.shootRound()
	}
	return s.fightRound()
}

// targetChoose 接住目標選單。選中就照那一隻打一回合。
func (s *Session) targetChoose(i int) bool {
	enc := s.Game.Fight
	if enc == nil {
		return s.closeMenu()
	}
	if i < 0 || i >= len(s.pickers) {
		return s.cancelTarget()
	}
	enc.Target = s.pickers[i]
	ranged := s.targetRanged
	s.closeMenu()
	if ranged {
		return s.shootRound()
	}
	return s.fightRound()
}

// cancelTarget 是提示裡按 Esc：**這一回合不打，也不消耗行動**。
//
// 原版 `sub_18DAA` 收到 `0x1B` 就 `var_E--`，之後那一段攻擊整個跳過，
// 回合照樣留給玩家重下指令。這裡對應的就是「回到戰鬥畫面，不推進回合」。
func (s *Session) cancelTarget() bool {
	s.targetRanged = false
	return s.closeMenu()
}
