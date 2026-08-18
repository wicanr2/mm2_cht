package game

// 旅店的落點與全滅之後的回收。機制與位址見 `docs/re/06-1retinn-roster.md`。

// TownStart 是離開旅店時隊伍出現的位置，索引即城鎮編號（＝地圖編號）。
//
// 三張表在 DGROUP：`ds:21E8`（X）、`ds:21EE`（Y）、`ds:21F4`（朝向），
// 由 `_1retinn_e03` 收尾時寫進 `ds:0393`／`ds:0394`／`ds:03CF`。
// **朝向存的是 ASCII 字母**，與牆位元遮罩用的那一套相同。
var TownStart = [5]struct {
	X, Y int
	Face Facing
}{
	{7, 3, North},  // 0 Middlegate
	{9, 13, North}, // 1 Atlantium
	{7, 11, East},  // 2 Tundara
	{7, 0, North},  // 3 Vulcania
	{3, 10, West},  // 4 Sansobar
}

// LastInnTown 回傳最後投宿的城，沒住過就是 Middlegate。
//
// 對應原版的 `ds:03D4`：`_1retinn_e00` 登記入住時寫、`_1retinn_e03` 離開時
// 再寫一次，`_1retinn_e04`（全滅）拿它決定回哪一家。
func (s *Session) LastInnTown() int {
	if s.LastInn < 0 || s.LastInn >= len(TownStart) {
		return 0
	}
	return s.LastInn
}

// CheckInAtInn 記下這一家旅店。踩進旅店那一格時呼叫。
//
// 原版順便把整隊的記錄 `+11` 改寫成城鎮編號 ＋ 1（`_1retinn_e00` 的
// `loc_1C1BD` 迴圈）—— 那是「這個角色寄放在哪座城」，名冊畫面只讓
// 寄放在當地的角色選得動。remake 的隊伍不走名冊分頁，但這個欄位要跟著寫，
// 存檔才與原版一致。
func (s *Session) CheckInAtInn() {
	town := s.World.MapIndex
	if town < 0 || town >= len(TownStart) {
		return
	}
	s.LastInn = town
	for i := range s.Party {
		c := &s.Party[i]
		c.SetFieldByte(offInnTown, 0x80, byte(town+1))
	}
}

// ReviveAtInn 是全滅之後的回收：回到最後投宿的城，隊伍原地重整。
//
// 原版 `_1retinn_e04` 印完 `Death Strikes!` 那一頁就把 `ds:0392` 設回
// `ds:03D4` 並跳進 `_1retinn_e01`，也就是「從那家旅店重新出發」。
// 倒下的人在原版是留在名冊裡等神殿救；remake 沒有名冊分頁，所以
// **狀況不清除** —— 走得到神殿就治得好，這條路不代替神殿。
func (s *Session) ReviveAtInn() []string {
	town := s.LastInnTown()
	st := TownStart[town]
	s.World.MapIndex, s.World.X, s.World.Y = town, st.X, st.Y
	s.World.Face = st.Face
	s.World.ClearTravelEffects()
	s.World.Teleported = true
	s.World.MarkExplored()
	s.EndCombat()
	return []string{caveText("ui.inn.revive", "隊伍回到最後投宿的旅店。")}
}
