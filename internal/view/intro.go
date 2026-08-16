package view

import (
	"image"

	"github.com/wicanr2/mm2_cht/internal/render"
)

// 片頭畫面：原版開機那張「Might and Magic Book Two」，草地上一隻獨角獸低頭吃草。
//
// `MASTER.16` 有 15 張圖，320×196 的那一張（第 14 張）是底圖，其餘 14 張是
// 疊在底圖上的動畫。**播放順序、落點與週期都取自原版**（`1MENU1` 的
// `sub_1C1FA`），見底下三個表與 `docs/formats/04-graphics.md`。
//
// 透空色是 **7**（淺灰）：把第 0 張放在 (160, 161)，只有 key=7 能讓那一塊
// 與 DOSBox 截圖 **完全相同**（1271 個非透空像素零誤差），其餘 15 個候選色
// 都不行。
const (
	introW, introH = 320, 196
	// introKey 是疊圖的透空色。
	introKey = 7
	// hintH 是底部提示條的高度（原版像素）。
	hintH = 13
)

// IntroPlaylist 是原版的播放清單：`1MENU1` 的 `sub_1C1FA` 從 `ds:18D8`
// 逐格取出的 47 個圖號。**不是亂數也不是輪替** —— 每一次開機的順序都一樣。
//
// 第 0 步是底圖（第 14 張），其餘每一步在原地疊一張圖。清單裡的
// 1、3、7 是與底圖相同的還原圖，原版靠它們擦掉上一格 ——
// 所以整串是累加的，不能只畫目前這一張。
var IntroPlaylist = []int{
	14, 0, 1, 0, 2, 5, 1, 3, 6, 0, 5, 1, 7, 0, 1, 0,
	2, 8, 1, 7, 0, 3, 1, 0, 9, 1, 7, 0, 2, 1, 0, 3,
	10, 1, 11, 0, 7, 1, 0, 2, 12, 1, 0, 3, 13, 1, 4,
}

// IntroSpotAt 是十五張圖各自的落點，取自 `ds:1936`（x）與 `ds:1954`（y）。
//
// 這兩張表與先前用 60 張 DOSBox 截圖逐像素反推出來的位置**逐項相同**，
// 是同一件事的第二條獨立證據。
var IntroSpotAt = [15]image.Point{
	{X: 160, Y: 161}, {X: 160, Y: 161}, // 0/1 獨角獸低頭咀嚼（1 是還原圖）
	{X: 16, Y: 126}, {X: 16, Y: 126}, // 2/3 尾巴甩動（3 是還原圖）
	{X: 160, Y: 154},                 // 4 馬頭上方
	{X: 256, Y: 74}, {X: 256, Y: 74}, // 5/6 樹上的臉
	{X: 232, Y: 62},                    // 7 整片樹叢的還原圖
	{X: 240, Y: 113},                   // 8
	{X: 240, Y: 61},                    // 9
	{X: 232, Y: 125}, {X: 232, Y: 125}, // 10/11 樹下
	{X: 96, Y: 95}, {X: 96, Y: 95}, // 12/13 左邊樹叢
	{X: 0, Y: 0}, // 14 底圖
}

// 每一步撐多久：原版是 `sub_14EFE(0x46)`，也就是 140 次「查一次鍵盤 ＋
// 等 5 個計時器 tick」，共 **700 個 tick**。`TIMER.DRV` 把 PIT 的除數設成
// `0x0400`（`mov al,36h; out 43h,al` 之後送 `00 04`），所以一個 tick 是
// 1,193,182 ÷ 1024 = 1,165.2 Hz —— 700 個 tick ＝ **0.601 秒**，
// 47 步合計 28.2 秒，與實機量到的「標題畫面約 27 秒」對得上。
//
// remake 的 tick 是 60 fps ÷ 8 ＝ 7.5 Hz，一步 0.601 秒剛好是 4.5 個 tick，
// 所以用 `tick × 2 ÷ 9` 取步數 —— 整數運算，長期平均正好 4.5。
const (
	introStepNum = 2
	introStepDen = 9
)

// IntroStep 回傳第 tick 個更新該走到播放清單的第幾步。
//
// 原版跑完 47 步就關檔進主選單；remake 的片頭是等按鍵，所以跑完從頭再來
// （原版不會循環，這是 remake 為了「等待時畫面還活著」加的）。
func IntroStep(tick int) int {
	if tick < 0 {
		tick = -tick
	}
	return (tick * introStepNum / introStepDen) % len(IntroPlaylist)
}

// Intro 是片頭畫面的素材。Title 為 nil 表示載不到，呼叫端就該跳過片頭。
type Intro struct {
	Title *image.Paletted
	// Frames 是 `MASTER.16` 的十五張，索引就是播放清單裡的圖號。
	Frames []*image.Paletted
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
	// 原版是把整串疊圖往同一張畫面上累加，還原圖負責擦掉上一格；
	// remake 每一格都從底圖重畫，所以要把 1..step 依序補回去。
	step := IntroStep(tick)
	for i := 1; i <= step && i < len(IntroPlaylist); i++ {
		n := IntroPlaylist[i]
		if n < 0 || n >= len(in.Frames) || in.Frames[n] == nil {
			continue
		}
		p := IntroSpotAt[n]
		s.BlitKey(in.Frames[n], p.X, p.Y, introKey)
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
