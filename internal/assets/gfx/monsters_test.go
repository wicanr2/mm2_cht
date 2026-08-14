package gfx_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
)

func orig(t *testing.T, name string) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "..", "workplace", "orig", "MM2", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("沒有原版檔案 %s，跳過", name)
	}
	return b
}

// 索引表要是 75 項、第 0 項等於表長、剛好 16 個空槽。
func TestMonsterIndex(t *testing.T) {
	idx, err := gfx.MonsterIndex(orig(t, "MONSTERS.16"))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) != gfx.MonsterPicCount {
		t.Fatalf("索引表 %d 項，預期 %d", len(idx), gfx.MonsterPicCount)
	}
	empty := 0
	for _, v := range idx {
		if v == 0 {
			empty++
		}
	}
	if empty != 16 {
		t.Errorf("空槽 %d 個，預期 16", empty)
	}
}

// 圖號指到空槽時要往後借，掃到底回繞。
//
// 圖號 7（老守財奴／隱士）的索引 6 是空的，原版的 sub_6818 會往後掃到 7；
// 圖號 8（哥布林）本來就是 7。兩者共用同一張圖是原版的行為，不是解析錯誤。
func TestResolveMonsterPicFallsForward(t *testing.T) {
	idx, err := gfx.MonsterIndex(orig(t, "MONSTERS.16"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ pic, want int }{
		{1, 0}, {6, 5}, {7, 7}, {8, 7}, {23, 23}, {60, 59},
	} {
		if got := gfx.ResolveMonsterPic(idx, tc.pic); got != tc.want {
			t.Errorf("圖號 %d 解到槽 %d，預期 %d", tc.pic, got, tc.want)
		}
	}
	// 每個空槽都要能解到某個非空槽。
	for i, v := range idx {
		if v != 0 {
			continue
		}
		got := gfx.ResolveMonsterPic(idx, i+1)
		if got < 0 || idx[got] == 0 {
			t.Errorf("空槽 %d 沒解到有效的槽（得到 %d）", i, got)
		}
	}
}

// 每個槽的每個影格都要解出剛好 w×h 個像素，而且基準圖是 84×86、
// 左上角是透明色。這條是像素編碼的驗收條件：解錯了長度不會對。
func TestMonsterFramesDecode(t *testing.T) {
	blob := orig(t, "MONSTERS.16")
	idx, err := gfx.MonsterIndex(blob)
	if err != nil {
		t.Fatal(err)
	}
	slots, frames := 0, 0
	for i, v := range idx {
		if v == 0 {
			continue
		}
		pic, err := gfx.ParseMonsterPic(blob, i)
		if err != nil {
			t.Fatalf("槽 %d：%v", i, err)
		}
		slots++
		if len(pic.Frames) == 0 {
			t.Fatalf("槽 %d 一個影格都沒有", i)
		}
		base := pic.Frames[0]
		if base.Width != 84 || base.Height != 86 {
			t.Errorf("槽 %d 的基準圖是 %d×%d，預期 84×86", i, base.Width, base.Height)
		}
		if base.At(0, 0) != gfx.TransparentIndex {
			t.Errorf("槽 %d 的基準圖左上角是 %d，預期透明色 %d",
				i, base.At(0, 0), gfx.TransparentIndex)
		}
		for j, f := range pic.Frames {
			if len(f.Pixels) != f.Width*f.Height {
				t.Fatalf("槽 %d 影格 %d 解出 %d 個像素，預期 %d×%d",
					i, j, len(f.Pixels), f.Width, f.Height)
			}
			for _, p := range f.Pixels {
				if p > 15 {
					t.Fatalf("槽 %d 影格 %d 有顏色索引 %d", i, j, p)
				}
			}
			frames++
		}
	}
	if slots != 59 {
		t.Errorf("解出 %d 個槽，預期 59", slots)
	}
	if frames != 433 {
		t.Errorf("解出 %d 個影格，預期 433", frames)
	}
}

// 播放腳本的段編號一定指得到段，動畫序列的影格編號不必 —— 這兩條是
// 動畫表解對了的自洽條件。
//
// 59 個槽共 240 段，第一段是**播放腳本**（段編號, 停留），其餘是
// （影格, 停留）。腳本共 136 項，其中 31 項帶 bit 7（隨機挑段）。
// 段編號一律落在 1..段數-1，零例外 —— 這條是關鍵：先前把第一段當動畫讀，
// 就是這 136 個位元組冒充出 47、131、134 這種「越界影格」。
//
// 影格編號反而可以越界：原版在 root `0x1578E` 比對影格數，超過就畫影格 0。
func TestMonsterAnimScripts(t *testing.T) {
	blob := orig(t, "MONSTERS.16")
	idx, err := gfx.MonsterIndex(blob)
	if err != nil {
		t.Fatal(err)
	}
	segs, entries, random := 0, 0, 0
	for i, v := range idx {
		if v == 0 {
			continue
		}
		pic, err := gfx.ParseMonsterPic(blob, i)
		if err != nil {
			t.Fatal(err)
		}
		segs += 1 + len(pic.Anims) // 腳本段 + 各動畫段
		for _, st := range pic.Script {
			entries++
			if st.Random {
				random++
				if st.Hold != 0 {
					t.Errorf("槽 %d 的隨機腳本項停留是 %d，全檔應該一律是 0", i, st.Hold)
				}
			}
			if st.Seq < 1 || st.Seq > len(pic.Anims) {
				t.Errorf("槽 %d 的腳本項指到第 %d 段，但只有 %d 段", i, st.Seq, len(pic.Anims))
			}
		}
	}
	if segs != 240 {
		t.Errorf("解出 %d 段，預期 240（含每個槽的腳本段）", segs)
	}
	if entries != 136 {
		t.Errorf("腳本項共 %d，預期 136", entries)
	}
	if random != 31 {
		t.Errorf("帶 bit 7 的腳本項 %d，預期 31", random)
	}
}

// 原版對槽 9 的執行時修補要照做：檔案裡那張表是壞的。
func TestSlot9RuntimePatch(t *testing.T) {
	blob := orig(t, "MONSTERS.16")
	pic, err := gfx.ParseMonsterPic(blob, 9)
	if err != nil {
		t.Fatal(err)
	}
	// 檔案裡的腳本是 (47,10),(16,59),(6,4),(131,0) —— 段編號全部指不到。
	// 修補後是 (1,1),(2,1),(3,1)，三段都存在。
	want := []gfx.ScriptStep{{Seq: 1, Hold: 1}, {Seq: 2, Hold: 1}, {Seq: 3, Hold: 1}}
	if len(pic.Script) != len(want) {
		t.Fatalf("槽 9 的腳本有 %d 項，預期 %d", len(pic.Script), len(want))
	}
	for i, w := range want {
		if pic.Script[i] != w {
			t.Errorf("腳本第 %d 項是 %+v，預期 %+v", i, pic.Script[i], w)
		}
	}
	if len(pic.Anims) != 3 {
		t.Fatalf("槽 9 解出 %d 段動畫，預期 3", len(pic.Anims))
	}
	// 第三段那個越界的 6 要變成 2。
	for _, st := range pic.Anims[2] {
		if st.Frame >= len(pic.Frames) {
			t.Errorf("第三段仍有越界影格 %d（共 %d）", st.Frame, len(pic.Frames))
		}
	}
}

// 每一隻有名字的怪物都要解得到圖。這條把 MONSTERS.DAT 的圖號欄位
// 與 monsters.16 的索引表綁在一起：任何一邊解錯都會炸。
func TestEveryMonsterHasPicture(t *testing.T) {
	blob := orig(t, "MONSTERS.16")
	idx, err := gfx.MonsterIndex(blob)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := monsters.Parse(orig(t, "MONSTERS.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		if m.Name == "" {
			continue
		}
		slot := gfx.ResolveMonsterPic(idx, m.Sprite)
		if slot < 0 {
			t.Fatalf("%s 的圖號 %d 解不到槽", m.Name, m.Sprite)
		}
		if idx[slot] == 0 {
			t.Fatalf("%s 解到空槽 %d", m.Name, slot)
		}
	}
}
