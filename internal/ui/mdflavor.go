package ui

import (
	"encoding/json"
	"os"

	"github.com/wicanr2/mm2_cht/internal/game"
)

// 進設施時先講一段場景 —— **那是 Mega Drive 版的稿，DOS 版沒有**。
//
// DOS 走進鐵匠鋪只有一個光禿禿的選單；Mega Drive 每座城的鐵匠都有名字
// 與一句描寫。五種設施（旅店、鐵匠、酒館、神殿、訓練所）× 五座城 ＝ 25 段，
// 場景與 DOS 完全對得上，所以放得進同一個位置。
//
// 原文由 `tools/mdflavor.py` 從玩家自備的 ROM 抽到 `workplace/`（不入版控），
// 譯文在 `translations/md-flavor.json`。兩個都沒有就什麼都不顯示。
// 開關在 `F2`，**預設關** —— 顯示另一個版本的文字是內容選擇，不是還原。
//
// 對應規則與那一個對不上的例外見 `docs/research/03-megadrive-text.md`。

// mdFlavorKind 把設施種類換成 key 裡的那一段。沒有描述的設施回空字串。
func mdFlavorKind(k game.FacilityKind) string {
	switch k {
	case game.FacilityInn:
		return "inn"
	case game.FacilityBlacksmith:
		return "blacksmith"
	case game.FacilityTavern:
		return "tavern"
	case game.FacilityTemple:
		return "temple"
	case game.FacilityTraining:
		return "training"
	}
	return ""
}

// loadMDFlavor 讀原文。檔案不在就回 nil —— 玩家沒抽過 ROM 是常態。
func loadMDFlavor(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var rows []struct {
		Key  string `json:"key"`
		Text string `json:"text"`
	}
	if json.Unmarshal(b, &rows) != nil {
		return nil
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Key] = r.Text
	}
	return out
}

// mdFlavorLine 回傳這一次進設施要先講的那一段，沒有就回空字串。
func (s *Session) mdFlavorLine() string {
	if !s.Game.MDFlavor {
		return ""
	}
	kind := mdFlavorKind(s.Game.Facility)
	if kind == "" {
		return ""
	}
	town := s.Game.World.MapIndex
	if town < 0 || town >= mdFlavorTowns {
		return ""
	}
	key := "md." + kind + "." + string(rune('0'+town))
	// 有譯文就用譯文，沒有就用原文；兩個都沒有就不講。
	return s.text(key, s.mdFlavor[key])
}

// mdFlavorTowns 是有描述的城鎮數。五座城，index 就是地圖編號。
const mdFlavorTowns = 5
