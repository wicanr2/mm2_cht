package text_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/text"
)

func orig(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "workplace", "orig", "MM2", name))
	if err != nil {
		t.Skip("沒有原版 " + name + "，跳過")
	}
	return b
}


// STR.DAT 的取用方式是「區塊 + 順序游標」，不是逐條索引。
//
// 原版的載入常式（root `0x16750` 一帶）把 `ds:52F4[N]` 起的 0x960 bytes
// 解碼進 `ds:A06E` 並把游標 `ds:52F2` 歸零；取字串的常式回傳游標處的
// 字串再掃到 NUL。表裡的五個位移必須落在段落的開頭 —— 落在中間的話
// 遊戲第一次取字串就會拿到半句話。
func TestStrBlockTableLandsOnSegments(t *testing.T) {
	lines, err := text.Parse(orig(t, "STR.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 400 {
		t.Fatalf("解出 %d 段，預期 400", len(lines))
	}
	// 每一段都是一行顯示，最長 38 字 —— 一則訊息是連續數行。
	for i, l := range lines {
		if len(l) > 38 {
			t.Errorf("第 %d 段有 %d 字，超過一行的寬度：%q", i, len(l), l)
		}
	}
	// 區塊表的五個位移（`ds:52F4`）換算成累計長度時，必須正好在某一段的開頭。
	starts := map[int]bool{}
	pos := 0
	for _, l := range lines {
		starts[pos] = true
		pos += len(l) + 1 // 分隔符佔一個位元組
	}
	for _, off := range []int{0, 1596, 3932, 4742, 6212} {
		if !starts[off] {
			t.Errorf("區塊位移 %d 不在段落開頭", off)
		}
	}
	// 每個區塊都要塞得進一次 0x960 bytes 的載入
	blocks := []int{0, 1596, 3932, 4742, 6212, pos - 1}
	for i := 0; i+1 < len(blocks); i++ {
		if n := blocks[i+1] - blocks[i]; n > 0x960 {
			t.Errorf("第 %d 區塊有 %d bytes，超過一次載入的 %d", i, n, 0x960)
		}
	}
}
