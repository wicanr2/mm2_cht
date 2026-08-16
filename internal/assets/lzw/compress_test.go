package lzw_test

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/lzw"
)

// 壓縮的驗收條件只有一條：**原版的解壓常式解得回去**。
//
// 壓縮端的選擇（何時送 CLEAR、字典滿了怎麼辦）在解壓端看不出來，
// 所以「與原版的壓縮輸出逐位元組相同」既做不到也不是需求。
func roundTrip(t *testing.T, src []byte) {
	t.Helper()
	packed := lzw.PackSegment(src)
	got, err := lzw.Segment(packed, 0)
	if err != nil {
		t.Fatalf("解不回來（%d bytes）：%v", len(src), err)
	}
	if !bytes.Equal(got, src) {
		t.Fatalf("往返之後不一樣：原 %d bytes、回 %d bytes", len(src), len(got))
	}
}

func TestCompressRoundTripSynthetic(t *testing.T) {
	cases := map[string][]byte{
		"空的":      {},
		"一個位元組":   {0x41},
		"全同":      bytes.Repeat([]byte{0xAA}, 5000),
		"遞增":      seq(4096),
		"兩個位元組交替": bytes.Repeat([]byte{1, 2}, 3000),
	}
	// 亂數那一份要夠長，逼碼寬一路長到 12 並把字典塞滿 ——
	// 碼寬增長與「字典滿了不再新增」是兩個最容易與解壓端不對稱的地方。
	r := rand.New(rand.NewSource(1))
	big := make([]byte, 40000)
	for i := range big {
		big[i] = byte(r.Intn(4)) // 變化少，才長得出長字串把字典撐滿
	}
	cases["低變化長資料"] = big
	for name, src := range cases {
		t.Run(name, func(t *testing.T) { roundTrip(t, src) })
	}
}

// 真資料才算數：拿原版兩個事件檔的每一段往返一次。
func TestCompressRoundTripOriginal(t *testing.T) {
	for _, name := range []string{"EVENTSI.DAT", "EVENTSO.DAT"} {
		path := filepath.Join("..", "..", "..", "workplace", "orig", "MM2", name)
		blob, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("找不到原版檔案 %s（玩家自備合法原版）", path)
		}
		n := 0
		for i := 0; i < 71; i++ {
			off := int(binary.LittleEndian.Uint32(blob[i*4:]))
			if off == 0 {
				continue
			}
			raw, err := lzw.Segment(blob, off)
			if err != nil {
				t.Fatalf("%s 段 %d 解不開：%v", name, i, err)
			}
			roundTrip(t, raw)
			n++
		}
		if n == 0 {
			t.Fatalf("%s 一段都沒解到", name)
		}
		t.Logf("%s：%d 段全部往返成功", name, n)
	}
}

func seq(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(i)
	}
	return out
}
