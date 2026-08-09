package events_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// 兩個事件檔的檔頭都是**同一張 71 項的 32 位元偏移表**，用地圖號索引，
// 0 表示這張圖在這個檔裡沒有事件。
//
// 表佔 71 × 4 = 284 bytes，而最小的非零偏移正好是 284 —— 表尾接著資料。
//
// `EVENTSO` 的第 0 項是 0（地圖 0 是室內的中門，沒有室外事件），
// 所以**不能拿第 0 項去推表長** —— 那樣會算出表長 0。
func TestEventOffsetTable(t *testing.T) {
	const entries = 71
	for _, tc := range []struct {
		file  string
		want  int
		first int
	}{
		{"EVENTSI.DAT", 44, 0},
		{"EVENTSO.DAT", 27, 5},
	} {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", "workplace", "orig", "MM2", tc.file))
		if err != nil {
			t.Skipf("找不到 %s", tc.file)
		}
		var maps []int
		var offs []int
		for i := 0; i < entries; i++ {
			o := int(binary.LittleEndian.Uint32(b[i*4:]))
			if o == 0 {
				continue
			}
			maps = append(maps, i)
			offs = append(offs, o)
			if o >= len(b) {
				t.Errorf("%s 地圖 %d 的偏移 %d 超出檔案", tc.file, i, o)
			}
		}
		if len(maps) != tc.want {
			t.Errorf("%s 有 %d 個非零項，預期 %d", tc.file, len(maps), tc.want)
		}
		if len(maps) > 0 && maps[0] != tc.first {
			t.Errorf("%s 第一個有事件的地圖是 %d，預期 %d", tc.file, maps[0], tc.first)
		}
		if len(offs) > 0 && offs[0] != entries*4 {
			t.Errorf("%s 最小偏移是 %d，預期 %d（緊接在表後）", tc.file, offs[0], entries*4)
		}
		for i := 1; i < len(offs); i++ {
			if offs[i] < offs[i-1] {
				t.Errorf("%s 偏移不是遞增：地圖 %d 的 %d < 地圖 %d 的 %d",
					tc.file, maps[i], offs[i], maps[i-1], offs[i-1])
			}
		}
		t.Logf("%s：%d 個地圖有事件，%v", tc.file, len(maps), maps)
	}
}
