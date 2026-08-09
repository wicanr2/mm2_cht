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

// 動畫表用到的影格編號必須落在影格數內 —— 這是動畫表解對了的自洽條件。
//
// 59 個槽共 181 段，只有一步越界（槽 9 的第三段，編號 6 而它宣告 6 個影格）。
// 那一步沒有解釋，所以這裡守的是「越界不超過一步」：解析退化的話會一次
// 多出幾十步，這條擋得住；同時也不假裝那一步已經懂了。
func TestMonsterAnimsInRange(t *testing.T) {
	blob := orig(t, "MONSTERS.16")
	idx, err := gfx.MonsterIndex(blob)
	if err != nil {
		t.Fatal(err)
	}
	anims, out, flagged := 0, 0, 0
	for i, v := range idx {
		if v == 0 {
			continue
		}
		pic, err := gfx.ParseMonsterPic(blob, i)
		if err != nil {
			t.Fatal(err)
		}
		for _, seq := range pic.Anims {
			anims++
			for _, st := range seq {
				if st.Frame < 0 || st.Frame >= len(pic.Frames) {
					out++
				}
				if st.Flag {
					flagged++
				}
			}
		}
	}
	if anims != 181 {
		t.Errorf("解出 %d 段動畫，預期 181", anims)
	}
	if out > 1 {
		t.Errorf("有 %d 步的影格編號越界，預期最多 1", out)
	}
	// bit 7 只出現在未解的表頭，FF 之後一次都沒有。
	if flagged != 0 {
		t.Errorf("FF 之後有 %d 步設了 bit 7，預期 0", flagged)
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
