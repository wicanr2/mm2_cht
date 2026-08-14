package view

import (
	"fmt"
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// 戰鬥畫面：怪物圖疊在第一人稱視圖區上。
//
// **像素與影格已證實**（docs/formats/04 §2）：圖號 → 槽號的回退規則抄自
// `sub_6818`，RLE 逐張解出剛好 w×h，59 張基準圖 render 出來可辨識。
//
// **精靈圖的座標還沒有 oracle**，但版面的一部分從程式碼拿得到
// （見 `docs/formats/04`）：怪物訊息面板在第 22–38 欄、第 1 列起，
// 場上超過 10 隻時多留 3 列；而且同一屏的怪物共用一張圖
// （`ds:9F84` 與 `ds:9E2B` 不同時才重載），所以不會出現混種畫面。
//
// **原版沒有屍體。** 怪物一死（或逃走）就從場上的六個平行陣列裡
// 被刪掉、後面的往前搬（`2COMBAT.img` 的 `sub_18A22`，見
// `docs/formats/08-combat.md`），所以畫面上不存在「死掉的怪物」
// 這個狀態 —— 名單少一行、精靈照舊。

// MonsterSprite 是畫面上的一隻怪物。
type MonsterSprite struct {
	Pic gfx.MonsterPic
	// Anim 是要播的動畫序列編號，Step 是序列裡的第幾步。
	// 兩者都超出範圍時只畫基準圖。
	Anim, Step int
}

// MonsterListLines 是右側名單一次顯示幾行，超過就以「+N more …」收尾。
// 實機截圖數出來的（十三隻骷髏顯示十行加一行 `+3 more Skeletons`）。
const MonsterListLines = 10

// DrawMonsters 畫戰鬥中的怪物。
//
// **原版只畫一隻精靈**，不論場上有幾隻（實機截圖：十三隻骷髏、
// 一個精靈圖）。數量與種類靠右側那一欄用文字列出。這也是
// `2COMBAT` 只在 `ds:9F84 != ds:9E2B` 時才重載圖的原因 ——
// 一屏只有一張圖要顯示。
//
// 精靈水平置中於視圖區、底邊貼齊視圖底部。
func DrawMonsters(s *render.Screen, ms []MonsterSprite) {
	if len(ms) == 0 {
		return
	}
	drawMonster(s, &ms[0], FPX+FPW/2, FPY+FPH)
}

// MonsterListText 產生右側名單的行，滿 MonsterListLines 行之後
// 以「+N more <名稱>」收尾（原版的複數形是英文的，這裡交給呼叫端
// 給已經處理好的名稱）。
func MonsterListText(name string, n int) []string {
	if n <= 0 {
		return nil
	}
	if n <= MonsterListLines {
		out := make([]string, n)
		for i := range out {
			out[i] = name
		}
		return out
	}
	out := make([]string, MonsterListLines+1)
	for i := 0; i < MonsterListLines; i++ {
		out[i] = name
	}
	out[MonsterListLines] = fmt.Sprintf("+%d more %s", n-MonsterListLines, name)
	return out
}

// drawMonster 把一隻怪物畫在 (cx, bottom)：水平置中、底邊對齊。
//
// 先鋪基準圖（影格 0），再疊上目前動畫步的影格。動畫零件自己帶
// x/y 偏移，那個偏移是相對基準圖左上角的，不是相對畫面。
func drawMonster(s *render.Screen, m *MonsterSprite, cx, bottom int) {
	if len(m.Pic.Frames) == 0 {
		return
	}
	base := m.Pic.Frames[0]
	ox := cx - base.Width/2
	oy := bottom - base.Height
	blitFrame(s, base, ox, oy)

	f := m.currentFrame()
	if f == nil {
		return
	}
	blitFrame(s, *f, ox+f.X, oy+f.Y)
}

// currentFrame 回傳目前動畫步要疊的影格，沒有就回 nil。
func (m *MonsterSprite) currentFrame() *gfx.Frame {
	if m.Anim < 0 || m.Anim >= len(m.Pic.Anims) {
		return nil
	}
	seq := m.Pic.Anims[m.Anim]
	if m.Step < 0 || m.Step >= len(seq) {
		return nil
	}
	i := seq[m.Step].Frame
	if i <= 0 || i >= len(m.Pic.Frames) {
		// 0 是基準圖，已經畫過了。**超過影格數也一樣** ——
		// 原版 root `0x1578E` 比對 `ds:9F76`（影格數），超過就 `xor al,al`
		// 用影格 0，不是壞資料也不是要略過整段。
		return nil
	}
	return &m.Pic.Frames[i]
}

// blitFrame 把一個影格畫上去，跳過透明色。
func blitFrame(s *render.Screen, f gfx.Frame, x, y int) {
	for sy := 0; sy < f.Height; sy++ {
		dy := y + sy
		if dy < FPY || dy >= FPY+FPH || dy >= render.OrigH {
			continue
		}
		for sx := 0; sx < f.Width; sx++ {
			dx := x + sx
			if dx < FPX || dx >= FPX+FPW || dx >= render.OrigW {
				continue
			}
			c := f.At(sx, sy)
			if c == gfx.TransparentIndex {
				continue
			}
			s.Orig.SetColorIndex(dx, dy, c)
		}
	}
}
