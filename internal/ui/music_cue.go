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
)

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
		return MusicCueDungeon
	}
	return MusicCueOutside
}
