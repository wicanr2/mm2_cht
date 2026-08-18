// 這些測試拿原版檔案當對照，不是只驗 remake 自己的內部一致性。
// 原版資料不入版控，找不到就 skip。
package assets_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
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

// B 型檔頭（兩組 uint16 偏移）。這裡驗的是實際確認過的事實：
// 解出的張數等於 count，且 MASTER.16 含一張 320×196 的標題畫面 ——
// 那張已經與原版 DOSBox 截圖逐像素比對過，62,672/62,720 相同。
func TestGraphicsTypeBHeader(t *testing.T) {
	for _, tc := range []struct {
		file  string
		count int
	}{
		{"MASTER.16", 15},
		{"DESERT.16", 20},
		{"TUNDRA.16", 20},
		{"OUTDOOR1.16", 8},
	} {
		imgs, err := gfx.ParseSet(orig(t, tc.file))
		if err != nil {
			t.Errorf("%s: %v", tc.file, err)
			continue
		}
		if len(imgs) != tc.count {
			t.Errorf("%s: 解出 %d 張，count 宣告 %d", tc.file, len(imgs), tc.count)
		}
	}

	imgs, err := gfx.ParseSet(orig(t, "MASTER.16"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, im := range imgs {
		if im.Width == 320 && im.Height == 196 {
			found = true
			if need := 320 * 196 / 2; len(im.Pixels) < need {
				t.Errorf("標題畫面只有 %d bytes，需要 %d", len(im.Pixels), need)
			}
		}
	}
	if !found {
		t.Error("MASTER.16 裡找不到 320×196 的標題畫面")
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

// 事件段的佈局讀自 2PLAY.OVL 的 sub_1A85C。這裡驗三件事：
// 事件表能正常以 00 00 00 結束、skip 指到緩衝內、字串區解得出東西。
// 三者任一算錯，段數或字串數就會整批崩掉。
func TestEventSegmentLayout(t *testing.T) {
	for _, tc := range []struct {
		file         string
		segments     int
		maxIrregular int
		lowNibble    int // Kind 低 nibble 非 0 的事件筆數
	}{
		{"EVENTSI.DAT", 44, 0, 0},
		{"EVENTSO.DAT", 27, 0, 1},
	} {
		segs, err := events.Parse(orig(t, tc.file))
		if err != nil {
			t.Errorf("%s: %v", tc.file, err)
			continue
		}
		if len(segs) != tc.segments {
			t.Errorf("%s: 解出 %d 段，預期 %d", tc.file, len(segs), tc.segments)
		}
		irregular, lowNibble := 0, 0
		for _, seg := range segs {
			if seg.Irregular {
				irregular++
				continue
			}
			// 腳本庫（編號 60 以上）本來就沒有事件表。
			if seg.Library {
				continue
			}
			if len(seg.Events) == 0 {
				t.Errorf("%s 段 %d 沒有事件", tc.file, seg.Index)
			}
			for _, e := range seg.Events {
				// 第三欄的低 nibble **幾乎**恆為 0：整份資料只有一筆
				// 例外（EVENTSO 段 37 的格 98，Kind=0xF1）。
				// 拿「恆為 0」當解析判準會丟掉那一整段，所以這裡只數，
				// 不當成錯誤 —— 真正的結構條件是 Cell 遞增。
				if e.Kind&0x0F != 0 {
					lowNibble++
				}
			}
		}
		// 曾經是 EVENTSI 8/44、EVENTSO 4/27。認出「腳本庫」佈局、
		// 並停止拿 Kind 低 nibble 當判準之後，兩邊都是 0。
		// 這是回歸基準：變多就是退步了。
		t.Logf("%s: %d/%d 段不符合事件表佈局", tc.file, irregular, len(segs))
		if irregular > tc.maxIrregular {
			t.Errorf("%s: %d 段不符合佈局，超過基準 %d", tc.file, irregular, tc.maxIrregular)
		}
		if lowNibble != tc.lowNibble {
			t.Errorf("%s: %d 筆事件的 Kind 低 nibble 非 0，預期 %d",
				tc.file, lowNibble, tc.lowNibble)
		}
	}
}

// EVENTSI 段 0 是 Middlegate：42 筆事件、34 條字串，第一筆事件在第 8 格。
func TestMiddlegateSegment(t *testing.T) {
	segs, err := events.Parse(orig(t, "EVENTSI.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	seg := segs[0]
	if len(seg.Events) != 42 || len(seg.Strings) != 34 {
		t.Fatalf("段 0 有 %d 筆事件、%d 條字串，預期 42 / 34", len(seg.Events), len(seg.Strings))
	}
	if seg.Events[0].Cell != 8 {
		t.Errorf("第一筆事件在第 %d 格，預期 8", seg.Events[0].Cell)
	}
	if seg.Strings[0] != "Middlegate Inn" {
		t.Errorf("第一條字串是 %q，預期 \"Middlegate Inn\"", seg.Strings[0])
	}
}
