package ui

// MusicCue 是目前正常玩家畫面的語意音樂角色。
//
// 這是穩定的字串型別；播放端只需依字串選擇自己的音訊資產，不必依賴
// Ebiten 或知道 UI 的私有 menuKind。
type MusicCue string

const (
	MusicCueUnknown    MusicCue = "unknown"
	MusicCueBattle     MusicCue = "battle"
	MusicCueTemple     MusicCue = "temple"
	MusicCueInn        MusicCue = "inn"
	MusicCueBlacksmith MusicCue = "blacksmith"
	MusicCueTavern     MusicCue = "tavern"
	MusicCueTraining   MusicCue = "training"
	MusicCueTown       MusicCue = "town"
	MusicCueDungeon    MusicCue = "dungeon"
	MusicCueOutside    MusicCue = "outside"
	MusicCueCastle     MusicCue = "castle"
)

// castleScenes 是走 `castle*.16` 那組牆面的場景碼。
//
// **已證實**：`2PLAY _2play_e10` 是 7 個 case 的 switch（跳表
// `jpt_1B2E0`），case 0 推 `town*.16`、case 1 推 `cave*.16`、
// **cases 2 與 5 推 `castle*.16`**、cases 3/4/6 推 `out*.16`。
// 檔名是從 DGROUP 初值段把指標解出來讀的，不是從反組譯的 `dw` 猜的
// （那些 `dw` 是程式碼被當成資料的誤讀）。
//
// 配上場景碼的區間表（`World.Scene`），城堡就是地圖 45–59。
var castleScenes = map[int]bool{2: true, 5: true}

// 一次性音效（stinger）。與上面那些不一樣：它們**不是背景音樂**，
// 是事件發生當下放一次就結束，放完仍回到原本的背景樂。
const (
	MusicCueVictory      MusicCue = "victory"       // 打贏
	MusicCueDefeat       MusicCue = "defeat"        // 全隊倒下
	MusicCueEnemyKilled  MusicCue = "enemy_killed"  // 戰鬥中消滅一隻敵人
	MusicCueMemberKilled MusicCue = "member_killed" // 戰鬥中隊員陣亡
	MusicCueTreasure     MusicCue = "treasure"      // 拿到戰利品
)

// Stinger 取走待播的一次性音效，取過就沒有了。
//
// **一幀只給一個**：好幾件事同時發生時（打贏的同一回合也死了人）
// 依重要性挑一個，疊著播會變成噪音。
func (s *Session) Stinger() (MusicCue, bool) {
	if s == nil || s.stinger == "" {
		return "", false
	}
	c := s.stinger
	s.stinger = ""
	return c, true
}

// stingerRank 是同一幀撞在一起時的優先序，數字大的贏。
func stingerRank(c MusicCue) int {
	switch c {
	case MusicCueDefeat:
		return 5
	case MusicCueVictory:
		return 4
	case MusicCueTreasure:
		return 3
	case MusicCueMemberKilled:
		return 2
	case MusicCueEnemyKilled:
		return 1
	}
	return 0
}

// queueStinger 排一個一次性音效，已經有更重要的就不蓋掉。
func (s *Session) queueStinger(c MusicCue) {
	if stingerRank(c) > stingerRank(s.stinger) {
		s.stinger = c
	}
}

// MusicCue 回傳目前 UI 狀態的音樂角色。
//
// 優先序是戰鬥、設施選單、地圖分類。城鎮只依已證實的地圖 0–4 判定；
// 其他地圖必須有該張 ATTRIB 記錄，才能安全區分室內地城與戶外。資料不足
// 時回傳 unknown，不把未知室內圖猜成城堡或其他設施。
func (s *Session) MusicCue() MusicCue {
	if s == nil || s.Game == nil || s.Game.World == nil {
		return MusicCueUnknown
	}
	if s.Game.Fight != nil || s.Mode == ModeCombat {
		return MusicCueBattle
	}
	if s.Mode == ModeMenu {
		switch s.menuKind {
		case menuTemple, menuTempleBuy:
			return MusicCueTemple
		case menuInn, menuInnAdd, menuInnDrop:
			return MusicCueInn
		case menuSmith, menuSmithSell, menuSmithIdent:
			return MusicCueBlacksmith
		case menuTavern:
			return MusicCueTavern
		case menuTrain:
			return MusicCueTraining
		}
	}

	mapIndex := s.Game.World.MapIndex
	if mapIndex >= 0 && mapIndex <= 4 {
		return MusicCueTown
	}
	attr := s.Game.CurrentAttr()
	if attr == nil {
		return MusicCueUnknown
	}
	if attr.Indoor() {
		if castleScenes[s.Game.World.Scene()] {
			return MusicCueCastle
		}
		return MusicCueDungeon
	}
	return MusicCueOutside
}
