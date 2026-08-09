// 這些測試拿原版檔案當對照，不是只驗 remake 自己的內部一致性。
// 原版資料不入版控，找不到就 skip。
package assets_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/font"
	"github.com/wicanr2/mm2_cht/internal/assets/gfx"
	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

func orig(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "workplace", "orig", "MM2", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("找不到原版檔案 %s（玩家自備合法原版，解到 workplace/orig/）", path)
	}
	return b
}

// 段頭宣告的長度與實際解出的長度必須相符 —— 這是 LZW 參數是否正確的判準，
// 任何一個參數猜錯都會在某個檔案上對不起來。
func TestLZWSegmentLengths(t *testing.T) {
	for _, tc := range []struct {
		file string
		want int
	}{
		{"MONSTERS.DAT", 6656},
		{"ATTRIB.DAT", 3840},
		{"STR.DAT", 7707},
	} {
		out, err := lzw.Segment(orig(t, tc.file), 0)
		if err != nil {
			t.Errorf("%s: %v", tc.file, err)
			continue
		}
		if len(out) != tc.want {
			t.Errorf("%s: 解出 %d bytes，預期 %d", tc.file, len(out), tc.want)
		}
	}
}

// MAP.DAT 的 60 段每段都必須解出 512 bytes（= 16×16 × 2）。
func TestMapSegments(t *testing.T) {
	blob := orig(t, "MAP.DAT")
	for i := 0; i < 60; i++ {
		off := int(binary.LittleEndian.Uint16(blob[i*2:]))
		out, err := lzw.Segment(blob, off)
		if err != nil {
			t.Fatalf("段 %d @%d: %v", i, off, err)
		}
		if len(out) != 512 {
			t.Errorf("段 %d 解出 %d bytes，預期 512", i, len(out))
		}
	}
}

// EVENTSI.DAT 第一段是 Middlegate 的地點名；解錯 LZW 就湊不出這個字串。
func TestEventsIndoorFirstSegment(t *testing.T) {
	blob := orig(t, "EVENTSI.DAT")
	off := int(binary.LittleEndian.Uint32(blob))
	if off != 0x11C {
		t.Fatalf("索引表首項 %#x，預期 0x11C", off)
	}
	out, err := lzw.Segment(blob, off)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte("Middlegate Inn"); !contains(out, want) {
		t.Errorf("第一段沒有出現 %q", want)
	}
}

func contains(hay, needle []byte) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if string(hay[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}

func TestGraphicsSets(t *testing.T) {
	for _, tc := range []struct {
		file          string
		count         int
		width, height int // 第一張
	}{
		{"NWCP.16", 1, 320, 82},
		{"DISK.16", 1, 88, 67},
		{"TOWNF.16", 1, 208, 60},
		{"TOWNT.16", 36, 24, 43},
		{"CAVET.16", 36, 24, 43},
	} {
		imgs, err := gfx.ParseSet(orig(t, tc.file))
		if err != nil {
			t.Errorf("%s: %v", tc.file, err)
			continue
		}
		if len(imgs) != tc.count {
			t.Errorf("%s: %d 張影像，預期 %d", tc.file, len(imgs), tc.count)
			continue
		}
		if imgs[0].Width != tc.width || imgs[0].Height != tc.height {
			t.Errorf("%s: 首張 %d×%d，預期 %d×%d",
				tc.file, imgs[0].Width, imgs[0].Height, tc.width, tc.height)
		}
		// 每張影像的 packed 資料至少要夠畫滿宣告的寬高
		for i, im := range imgs {
			if need := (im.Width*im.Height + 1) / 2; len(im.Pixels) < need {
				t.Errorf("%s 影像 %d: %d×%d 需要 %d bytes，只有 %d",
					tc.file, i, im.Width, im.Height, need, len(im.Pixels))
			}
		}
	}
}

// 未壓縮的檔案開頭形狀很像 LZW 段頭，必須被段頭檢查擋下來，
// 否則會解出「長度剛好對」的垃圾。
func TestNonLZWFilesRejected(t *testing.T) {
	for _, name := range []string{"SPELLS.DAT", "ITEMS.DAT", "ROSTER.DAT", "DEFAULT.DAT"} {
		if _, err := lzw.Segment(orig(t, name), 0); err == nil {
			t.Errorf("%s 不是 LZW 段，卻解成功了", name)
		}
	}
}

// MONSTERS.16 是索引式，直接當影像集解必須失敗 —— 這條防的是
// 「檔頭形狀和 LZW 段頭一樣」造成的誤判。
func TestMonstersIsNotAPlainSet(t *testing.T) {
	if _, err := gfx.ParseSet(orig(t, "MONSTERS.16")); err == nil {
		t.Error("MONSTERS.16 當一般影像集解居然成功了，判準失效")
	}
}

// 字型的 'A' 必須長得像 A：對照 render 出來的實際字形。
func TestFontASCII(t *testing.T) {
	f, err := font.Parse(orig(t, "MM2.CH"))
	if err != nil {
		t.Fatal(err)
	}
	wantA := [8]byte{
		0b00111000, // ..###...
		0b01111100, // .#####..
		0b11000110, // ##...##.
		0b11000110, // ##...##.
		0b11111110, // #######.
		0b11000110, // ##...##.
		0b11000110, // ##...##.
		0b00000000,
	}
	for y := 0; y < 8; y++ {
		if got := f.Row('A', y); got != wantA[y] {
			t.Errorf("'A' 第 %d 列 = %08b，預期 %08b", y, got, wantA[y])
		}
	}
	// 空白必須全暗，避免整份字型位移一格還測得過
	for y := 0; y < 8; y++ {
		if f.Row(' ', y) != 0 {
			t.Errorf("空白字元第 %d 列不是全暗", y)
		}
	}
}
