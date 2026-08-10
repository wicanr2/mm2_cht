package render_test

import (
	"math"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/render"
)

// 視窗怎麼拉，畫面都要等比例縮放並置中。
//
// 長寬比一變，中文疊加層與原版像素的相對位置就跑掉了 ——
// 那正是把中文做成獨立圖層最怕的事。
func TestFitKeepsAspectAndCentres(t *testing.T) {
	for _, tc := range []struct{ w, h int }{
		{render.HiW, render.HiH},         // 一比一
		{render.HiW * 2, render.HiH * 2}, // 整數倍
		{1920, 600},                      // 很寬
		{960, 1200},                      // 很高
		{1234, 777},                      // 隨便
		{1, 1},                           // 極小
		{0, 0},                           // 還沒配置
	} {
		scale, ox, oy := render.Fit(tc.w, tc.h)
		if scale <= 0 || math.IsNaN(scale) {
			t.Fatalf("%dx%d 的倍率是 %v", tc.w, tc.h, scale)
		}
		if tc.w < 1 || tc.h < 1 {
			continue
		}
		gw, gh := float64(render.HiW)*scale, float64(render.HiH)*scale
		if gw > float64(tc.w)+1e-6 || gh > float64(tc.h)+1e-6 {
			t.Errorf("%dx%d：畫面 %.1fx%.1f 超出視窗", tc.w, tc.h, gw, gh)
		}
		if math.Abs(gw-float64(tc.w)) > 1e-6 && math.Abs(gh-float64(tc.h)) > 1e-6 {
			t.Errorf("%dx%d：畫面 %.1fx%.1f 兩邊都沒貼齊", tc.w, tc.h, gw, gh)
		}
		if math.Abs((float64(tc.w)-gw)/2-ox) > 1e-6 || math.Abs((float64(tc.h)-gh)/2-oy) > 1e-6 {
			t.Errorf("%dx%d：沒有置中（%.1f, %.1f）", tc.w, tc.h, ox, oy)
		}
		want := float64(render.HiW) / float64(render.HiH)
		if math.Abs(gw/gh-want) > 1e-9 {
			t.Errorf("%dx%d：長寬比 %.6f，預期 %.6f", tc.w, tc.h, gw/gh, want)
		}
	}
}
