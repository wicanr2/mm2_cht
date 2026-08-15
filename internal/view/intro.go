package view

import (
	"image"

	"github.com/wicanr2/mm2_cht/internal/render"
)

// 片頭畫面：原版開機那張「Might and Magic Book Two」，草地上一隻獨角獸低頭吃草。
//
// `MASTER.16` 有 15 張圖，320×196 的那一張（第 14 張）是底圖，其餘 14 張是
// 疊在底圖上的動畫。位置與透空色是拿 DOSBox 實機截圖定出來的：把每張疊圖
// 在整個 320×196 上滑動，找「非透空像素與截圖逐格相同」的落點。
//
// 透空色是 **7**（淺灰）：把第 0 張放在 (160, 161)，只有 key=7 能讓那一塊
// 與截圖 **完全相同**（1271 個非透空像素零誤差），其餘 15 個候選色都不行。
//
// 疊圖不是「一對會動、其他不動」—— 27 秒的實機取樣（60 張截圖，1 秒一張）
// 裡 13 張疊圖全部出現過，每一張都落在下面表列的位置上。
const (
	introW, introH = 320, 196
	// introKey 是疊圖的透空色。
	introKey = 7
	// hintH 是底部提示條的高度（原版像素）。
	hintH = 13
	// introLoopHold／introPopHold 是每一格動畫撐幾個 tick。tick 約 7.5 Hz。
	//
	// **原版的週期未知**：取樣間隔 1 秒，而馬頭馬尾幾乎每一張都換了一次，
	// 只能定出「比 1 秒快」。這兩個數字是看起來對的呈現值，不是量到的。
	introLoopHold = 4
	introPopHold  = 6
)

// IntroSpotDef 是一個動畫熱點：位置，加上 `MASTER.16` 裡的圖號。
type IntroSpotDef struct {
	X, Y int
	Pics []int
}

// IntroLoopSpots 是一直在動的兩處：獨角獸的頭與尾。
//
// 兩處的第一張都與底圖相同（第 1 張零誤差、第 3 張只差 1 個像素），
// 也就是原版拿來「擦掉上一格」的還原圖。
var IntroLoopSpots = []IntroSpotDef{
	{X: 160, Y: 161, Pics: []int{1, 0}}, // 32×12，低頭咀嚼
	{X: 16, Y: 126, Pics: []int{3, 2}},  // 40×42，尾巴甩動
}

// IntroPopSpots 是偶爾冒出來的：樹叢與草地裡藏著幾張臉，一次探一個出來。
//
// 27 秒的取樣裡每一處都只出現一到兩次，**觸發規則未知**（沒去追原版是
// 亂數還是排程）。remake 用固定輪替，不宣稱與原版同步。
// 第 7 張（72×89 @ (232, 62)）是整片樹叢的還原圖，與底圖完全相同，
// 原版用它一次擦掉樹上的臉；remake 每格重畫底圖，用不到。
var IntroPopSpots = []IntroSpotDef{
	{X: 96, Y: 95, Pics: []int{12}},   // 40×21，左邊樹叢
	{X: 240, Y: 61, Pics: []int{9}},   // 24×13
	{X: 256, Y: 74, Pics: []int{5}},   // 48×31
	{X: 232, Y: 125, Pics: []int{10}}, // 32×23
	{X: 160, Y: 154, Pics: []int{4}},  // 24×5，馬頭上方
	{X: 240, Y: 113, Pics: []int{8}},  // 24×11
	{X: 256, Y: 74, Pics: []int{6}},
	{X: 232, Y: 125, Pics: []int{11}},
}

// IntroSpot 是熱點載進來之後的樣子。
type IntroSpot struct {
	X, Y   int
	Frames []*image.Paletted
}

// Intro 是片頭畫面的素材。Title 為 nil 表示載不到，呼叫端就該跳過片頭。
type Intro struct {
	Title *image.Paletted
	Loop  []IntroSpot
	Pop   []IntroSpot
}

// Ready 回報片頭畫得出來。
func (in *Intro) Ready() bool { return in != nil && in.Title != nil }

// DrawIntro 畫片頭。tick 是單調遞增的動畫計數。
func DrawIntro(s *render.Screen, in *Intro, tick int, a Assets, hint string) {
	if !in.Ready() {
		return
	}
	if tick < 0 {
		tick = -tick
	}
	s.Clear(0)
	s.Blit(in.Title, 0, 0)
	for _, sp := range in.Loop {
		if n := len(sp.Frames); n > 0 {
			s.BlitKey(sp.Frames[(tick/introLoopHold)%n], sp.X, sp.Y, introKey)
		}
	}
	// 冒頭的臉一次只出一個，而且中間空一拍 —— 排成連續的話會像跑馬燈。
	if n := len(in.Pop); n > 0 {
		if slot := (tick / introPopHold) % (2 * n); slot%2 == 0 {
			sp := in.Pop[slot/2]
			if m := len(sp.Frames); m > 0 {
				s.BlitKey(sp.Frames[0], sp.X, sp.Y, introKey)
			}
		}
	}
	if hint != "" {
		// 提示壓一條暗底再寫字。圖是 196 列、畫面 200 列，下面那 4 列
		// 放不下 24 px 的中文，所以只能疊在圖上 —— 壓底是為了讓白字在
		// 黃色地面上也看得見。
		fill(s, 0, introH-hintH, render.OrigW, hintH, 0)
	}
	s.Flush()
	if hint != "" {
		s.DrawText(a.white(), hint, 8*render.Scale, (introH-hintH+1)*render.Scale)
	}
}
