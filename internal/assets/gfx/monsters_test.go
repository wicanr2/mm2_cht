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
// 59 個槽共 **240 段**（`FF` 是每一段的結束標記，第一個 `FF` 之前那段
// 就是第一段），每一段的長度都是偶數。
//
// 影格位元組的 bit 7 只出現在各槽的第一段，全檔 31 個（槽 9 那個在原廠
// 修補後消失），而且那一對的停留值一律是 0。遮掉 bit 7 之後只剩三步越界
// （槽 24／35／39，編號正好等於影格數），全部帶 bit 7 —— 所以這裡守的是
// 「越界的都帶 bit 7」，不假裝已經懂 bit 7，但擋得住解析退化
// （退化會一次多出幾十步不帶旗標的越界）。
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
					if !st.Flag {
						t.Errorf("槽 %d 有一步越界卻沒有 bit 7：影格 %d／共 %d",
							i, st.Frame, len(pic.Frames))
					}
				}
				if st.Flag {
					flagged++
					if st.Hold != 0 {
						t.Errorf("槽 %d 的 bit 7 那一步停留是 %d，全檔應該一律是 0",
							i, st.Hold)
					}
				}
			}
		}
	}
	if anims != 240 {
		t.Errorf("解出 %d 段動畫，預期 240", anims)
	}
	if out != 3 {
		t.Errorf("有 %d 步的影格編號越界，預期 3（槽 24／35／39）", out)
	}
	if flagged != 31 {
		t.Errorf("設了 bit 7 的步數是 %d，預期 31", flagged)
	}
}

// 原版對槽 9 的執行時修補要照做：檔案裡那張表是壞的。
func TestSlot9RuntimePatch(t *testing.T) {
	blob := orig(t, "MONSTERS.16")
	pic, err := gfx.ParseMonsterPic(blob, 9)
	if err != nil {
		t.Fatal(err)
	}
	if len(pic.Anims) != 4 {
		t.Fatalf("槽 9 解出 %d 段，預期 4", len(pic.Anims))
	}
	// 檔案裡的第一段是 (47,10)…，修補後是 (1,1),(2,1),(3,1)。
	want := []gfx.AnimStep{{Frame: 1, Hold: 1}, {Frame: 2, Hold: 1}, {Frame: 3, Hold: 1}}
	if len(pic.Anims[0]) != len(want) {
		t.Fatalf("第一段有 %d 步，預期 %d", len(pic.Anims[0]), len(want))
	}
	for i, w := range want {
		if pic.Anims[0][i] != w {
			t.Errorf("第一段第 %d 步是 %+v，預期 %+v", i, pic.Anims[0][i], w)
		}
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
